package main

import (
	"strconv"
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

type Extractor struct {
	source []byte
	module *Module
}

func NewExtractor(source []byte) *Extractor {
	return &Extractor{source: source}
}

func (e *Extractor) text(n *sitter.Node) string {
	if n == nil {
		return ""
	}
	return string(e.source[n.StartByte():n.EndByte()])
}

func (e *Extractor) parseNumber(n *sitter.Node) int {
	if n == nil {
		return -1
	}
	val, err := strconv.Atoi(e.text(n))
	if err != nil {
		return -1
	}
	return val
}

func (e *Extractor) ExtractAll(root *sitter.Node) ([]*Module, error) {
	var modules []*Module
	
	entities := make(map[string]*sitter.Node)
	arches := make(map[string]*sitter.Node)
	formals := make(map[string][]*sitter.Node) // Map entity name to its formal blocks

	cursor := root.Walk()
	if cursor.GotoFirstChild() {
		for {
			node := cursor.Node()
			if node.Kind() == "entity_declaration" {
				if nameNode := node.ChildByFieldName("name"); nameNode != nil {
					entities[e.text(nameNode)] = node
				}
			} else if node.Kind() == "architecture_body" {
				if entityNode := node.ChildByFieldName("entity"); entityNode != nil {
					arches[e.text(entityNode)] = node
				}
			} else if node.Kind() == "psl_formal_verification" {
				// Standalone verification blocks. For now, we'll associate them
				// with the first entity found or create a dummy module.
				// In a full implementation, we might want a separate "Verification" design unit.
				// For now, let's just collect them globally.
				formals["global"] = append(formals["global"], node)
			}
			if !cursor.GotoNextSibling() {
				break
			}
		}
	}

	for name, entityNode := range entities {
		e.module = NewModule(name)
		e.extractInterface(entityNode)
		
		// If there's a port named 'clk', assume it's the module clock.
		for _, p := range e.module.Ports {
			if p.Name == "clk" {
				e.module.ClockPort = "clk"
				break
			}
		}

		if archNode, ok := arches[name]; ok {
			e.extractArchitecture(archNode)
		}
		
		// Add global formal blocks to every module for now, or just the first one.
		if len(formals["global"]) > 0 {
			for _, fn := range formals["global"] {
				e.module.Formals = append(e.module.Formals, e.extractPslFormalVerification(fn))
			}
			formals["global"] = nil // only add to the first module
		}

		modules = append(modules, e.module)
	}

	return modules, nil
}

func (e *Extractor) extractPslFormalVerification(node *sitter.Node) *PslFormalBlock {
	fb := &PslFormalBlock{
		Symbols: make(map[string]Type),
	}
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		fb.Name = e.text(nameNode)
	}

	cursor := node.Walk()
	if cursor.GotoFirstChild() {
		for {
			n := cursor.Node()
			if n.Kind() == "psl_assertion" {
				fb.Statements = append(fb.Statements, e.extractPslAssertion(n))
			} else if n.Kind() == "psl_assumption" {
				fb.Statements = append(fb.Statements, e.extractPslAssumption(n))
			} else if n.Kind() == "psl_cover" {
				fb.Statements = append(fb.Statements, e.extractPslCover(n))
			} else if n.Kind() == "psl_assert_never" {
				fb.Statements = append(fb.Statements, e.extractPslAssertNever(n))
			} else if n.Kind() == "symbolic_declaration" {
				// Handle symbolic declaration
				symNamesNode := n.ChildByFieldName("sym_names")
				var typeNode *sitter.Node
				for i := uint(0); i < n.ChildCount(); i++ {
					if n.Child(i).Kind() == "type_mark" {
						typeNode = n.Child(i)
						break
					}
				}
				typ := e.extractType(typeNode)
				
				symDecl := &SymbolicDeclaration{
					Type:    typ,
					LineNum: uint32(n.StartPosition().Row + 1),
				}
				
				namesCursor := symNamesNode.Walk()
				if namesCursor.GotoFirstChild() {
					for {
						idNode := namesCursor.Node()
						if idNode.Kind() == "identifier" {
							name := e.text(idNode)
							symDecl.Names = append(symDecl.Names, name)
							fb.Symbols[name] = typ
						}
						if !namesCursor.GotoNextSibling() {
							break
						}
					}
				}
				fb.Statements = append(fb.Statements, symDecl)
			}
			if !cursor.GotoNextSibling() {
				break
			}
		}
	}
	return fb
}


func (e *Extractor) extractInterface(entityNode *sitter.Node) {
	cursor := entityNode.Walk()
	if cursor.GotoFirstChild() {
		for {
			node := cursor.Node()
			if node.Kind() == "port_clause" {
				pCursor := node.Walk()
				if pCursor.GotoFirstChild() {
					for {
						pNode := pCursor.Node()
						if pNode.Kind() == "port_declaration" {
							e.extractPortDeclaration(pNode)
						}
						if !pCursor.GotoNextSibling() {
							break
						}
					}
				}
			} else if node.Kind() == "generic_clause" {
				gCursor := node.Walk()
				if gCursor.GotoFirstChild() {
					for {
						gNode := gCursor.Node()
						if gNode.Kind() == "generic_declaration" {
							e.extractGenericDeclaration(gNode)
						}
						if !gCursor.GotoNextSibling() {
							break
						}
					}
				}
			} else if node.Kind() == "contract_declaration" {
				e.extractContractDeclaration(node)
			}
			if !cursor.GotoNextSibling() {
				break
			}
		}
	}
}

func (e *Extractor) extractContractDeclaration(node *sitter.Node) {
	contract := &Contract{
		LineNum: uint32(node.StartPosition().Row + 1),
	}
	
	cursor := node.Walk()
	if cursor.GotoFirstChild() {
		for {
			n := cursor.Node()
			fieldName := cursor.FieldName()
			if fieldName == "require_expr" {
				expr := e.extractExpression(n)
				if expr != nil {
					contract.Requires = append(contract.Requires, expr)
				}
			} else if fieldName == "ensure_expr" {
				expr := e.extractExpression(n)
				if expr != nil {
					contract.Ensures = append(contract.Ensures, expr)
				}
			}
			if !cursor.GotoNextSibling() {
				break
			}
		}
	}
	
	e.module.Contract = contract
}

func (e *Extractor) extractGenericDeclaration(node *sitter.Node) {
	nameNode := node.ChildByFieldName("name")
	defaultNode := node.ChildByFieldName("default")
	
	if nameNode != nil {
		gen := &Generic{
			Name: e.text(nameNode),
		}
		if defaultNode != nil {
			gen.Default = e.text(defaultNode)
		}
		e.module.Generics = append(e.module.Generics, gen)
		e.module.Symbols[gen.Name] = Type{Name: "integer", Width: 32} // Generics are 32-bit integers in our subset
	}
}

func (e *Extractor) extractPortDeclaration(node *sitter.Node) {
	namesNode := node.ChildByFieldName("names")
	dirNode := node.ChildByFieldName("direction")

	var typeNode *sitter.Node
	for i := uint(0); i < node.ChildCount(); i++ {
		if node.Child(i).Kind() == "type_mark" {
			typeNode = node.Child(i)
			break
		}
	}

	direction := e.text(dirNode)
	typ := e.extractType(typeNode)

	if namesNode != nil {
		cursor := namesNode.Walk()
		if cursor.GotoFirstChild() {
			for {
				n := cursor.Node()
				if n.Kind() == "identifier" {
					name := e.text(n)
					port := &Port{
						Name:      name,
						Direction: direction,
						Type:      typ,
						LineNum:   uint32(n.StartPosition().Row + 1),
					}
					e.module.Ports = append(e.module.Ports, port)
					e.module.Symbols[name] = typ
				}
				if !cursor.GotoNextSibling() {
					break
				}
			}
		}
	}
}

func (e *Extractor) extractType(node *sitter.Node) Type {
	if node == nil {
		return Type{Name: "unknown", Width: 0}
	}
	text := e.text(node)
	if strings.HasPrefix(text, "std_logic") && !strings.Contains(text, "(") {
		return Type{Name: "std_logic", Width: 1}
	} else if text == "integer" {
		return Type{Name: "integer", Width: 32}
	}

	// Simple check for std_logic_vector/unsigned
	if strings.Contains(text, "downto") {
		// Because we use a tiered grammar, the indices are now nested logical_expressions.
		// Instead of manual string parsing, let's find the numeric literals if possible,
		// or just use e.extractExpression if we had a full evaluator.
		// For now, let's try a more robust string extraction.
		startIdx := strings.Index(text, "(")
		downtoIdx := strings.Index(text, "downto")
		endIdx := strings.Index(text, ")")
		if startIdx != -1 && downtoIdx != -1 && endIdx != -1 {
			base := strings.TrimSpace(text[:startIdx])
			leftStr := strings.TrimSpace(text[startIdx+1 : downtoIdx])
			rightStr := strings.TrimSpace(text[downtoIdx+6 : endIdx])

			// Clean up any remaining parentheses from nested expression nodes
			leftStr = strings.Trim(leftStr, "() ")
			rightStr = strings.Trim(rightStr, "() ")

			left, err1 := strconv.Atoi(leftStr)
			right, err2 := strconv.Atoi(rightStr)
			if err1 == nil && err2 == nil {
				width := left - right + 1
				return Type{Name: base, Width: width}
			}
		}
	}

	return Type{Name: text, Width: 0}
}


func (e *Extractor) extractArchitecture(archNode *sitter.Node) {
	cursor := archNode.Walk()
	if cursor.GotoFirstChild() {
		for {
			node := cursor.Node()
			if node.Kind() == "type_declaration" {
				e.extractTypeDeclaration(node)
			} else if node.Kind() == "signal_declaration" {
				e.extractSignalDeclaration(node)
			} else if node.Kind() == "concurrent_assignment" {
				e.module.Statements = append(e.module.Statements, e.extractConcurrentAssignment(node))
			} else if node.Kind() == "selected_assignment" {
				e.module.Statements = append(e.module.Statements, e.extractSelectedAssignment(node))
			} else if node.Kind() == "synchronous_process" {
				e.module.Statements = append(e.module.Statements, e.extractSynchronousProcess(node))
			} else if node.Kind() == "entity_instantiation" {
				e.module.Statements = append(e.module.Statements, e.extractEntityInstantiation(node))
			} else if node.Kind() == "generate_statement" {
				e.module.Statements = append(e.module.Statements, e.extractGenerateStatement(node))
			} else if node.Kind() == "psl_assertion" {
				e.module.Statements = append(e.module.Statements, e.extractPslAssertion(node))
			} else if node.Kind() == "psl_assumption" {
				e.module.Statements = append(e.module.Statements, e.extractPslAssumption(node))
			} else if node.Kind() == "psl_cover" {
				e.module.Statements = append(e.module.Statements, e.extractPslCover(node))
			} else if node.Kind() == "psl_assert_never" {
				e.module.Statements = append(e.module.Statements, e.extractPslAssertNever(node))
			} else if node.Kind() == "psl_after_reset_assertion" {
				e.module.Statements = append(e.module.Statements, e.extractPslAfterResetAssertion(node))
			}
			if !cursor.GotoNextSibling() {
				break
			}
		}
	}
}

func (e *Extractor) extractTypeDeclaration(node *sitter.Node) {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	name := e.text(nameNode)

	var t Type
	t.Name = name

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		switch child.Kind() {
		case "enum_type_definition":
			t.Width = 32
		case "array_type_definition":
			t.IsArray = true
			var bounds []int
			for j := uint(0); j < child.ChildCount(); j++ {
				if child.Child(j).Kind() == "number" {
					if val, err := strconv.Atoi(e.text(child.Child(j))); err == nil {
						bounds = append(bounds, val)
					}
				}
			}
			if len(bounds) >= 2 {
				t.ArraySize = bounds[1] - bounds[0] + 1
			}
			
			var elTypeNode *sitter.Node
			for j := uint(0); j < child.ChildCount(); j++ {
				if child.Child(j).Kind() == "type_mark" {
					elTypeNode = child.Child(j)
					break
				}
			}
			if elTypeNode != nil {
				elType := e.extractType(elTypeNode)
				t.ElementType = &elType
			}
		case "record_type_definition":
			t.IsRecord = true
			cursor := child.Walk()
			if cursor.GotoFirstChild() {
				for {
					n := cursor.Node()
					if n.Kind() == "record_field" {
						namesNode := n.ChildByFieldName("names")
						
						var fieldTypeNode *sitter.Node
						for j := uint(0); j < n.ChildCount(); j++ {
							if n.Child(j).Kind() == "type_mark" {
								fieldTypeNode = n.Child(j)
								break
							}
						}
						
						fType := e.extractType(fieldTypeNode)

						if namesNode != nil {
							nCursor := namesNode.Walk()
							if nCursor.GotoFirstChild() {
								for {
									idNode := nCursor.Node()
									if idNode.Kind() == "identifier" {
										t.Fields = append(t.Fields, RecordField{
											Name: e.text(idNode),
											Type: fType,
										})
									}
									if !nCursor.GotoNextSibling() {
										break
									}
								}
							}
						}
					}
					if !cursor.GotoNextSibling() {
						break
					}
				}
			}
		}
	}

	e.module.Symbols[name] = t
}

func (e *Extractor) extractSignalDeclaration(node *sitter.Node) {
	namesNode := node.ChildByFieldName("names")
	var typeNode *sitter.Node
	for i := uint(0); i < node.ChildCount(); i++ {
		if node.Child(i).Kind() == "type_mark" {
			typeNode = node.Child(i)
			break
		}
	}

	typ := e.extractType(typeNode)

	if namesNode != nil {
		cursor := namesNode.Walk()
		if cursor.GotoFirstChild() {
			for {
				n := cursor.Node()
				if n.Kind() == "identifier" {
					name := e.text(n)
					sig := &Signal{
						Name: name,
						Type: typ,
					}
					e.module.Signals = append(e.module.Signals, sig)
					e.module.Symbols[name] = typ
				}
				if !cursor.GotoNextSibling() {
					break
				}
			}
		}
	}
}

func (e *Extractor) extractConcurrentAssignment(node *sitter.Node) *ConcurrentAssignment {
	stmt := &ConcurrentAssignment{
		LineNum: uint32(node.StartPosition().Row + 1),
	}

	if targetNode := node.ChildByFieldName("target"); targetNode != nil {
		stmt.Target = e.text(targetNode)
	}
	if valueNode := node.ChildByFieldName("value"); valueNode != nil {
		stmt.Value = e.extractExpression(valueNode)
	}
	if condNode := node.ChildByFieldName("condition"); condNode != nil {
		stmt.Condition = e.extractExpression(condNode)
	}
	if altNode := node.ChildByFieldName("alt_value"); altNode != nil {
		stmt.AltValue = e.extractExpression(altNode)
	}

	return stmt
}

func (e *Extractor) extractSelectedAssignment(node *sitter.Node) *SelectedAssignment {
	stmt := &SelectedAssignment{
		LineNum: uint32(node.StartPosition().Row + 1),
	}

	if selectorNode := node.ChildByFieldName("selector"); selectorNode != nil {
		stmt.Selector = e.extractExpression(selectorNode)
	}
	if targetNode := node.ChildByFieldName("target"); targetNode != nil {
		stmt.Target = e.text(targetNode)
	}

	var currentValue Expression
	parsingValue := true

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		kind := child.Kind()
		if kind == "with" || kind == "select" || kind == "when" || kind == "," || kind == "<=" || kind == ";" {
			continue
		}
		
		if node.FieldNameForChild(uint32(i)) == "selector" || node.FieldNameForChild(uint32(i)) == "target" {
			continue
		}

		if parsingValue {
			currentValue = e.extractExpression(child)
			parsingValue = false
		} else {
			if kind == "others" {
				stmt.Choices = append(stmt.Choices, Choice{
					Value:    currentValue,
					IsOthers: true,
				})
			} else {
				stmt.Choices = append(stmt.Choices, Choice{
					Value:     currentValue,
					Condition: e.extractExpression(child),
				})
			}
			parsingValue = true
		}
	}

	return stmt
}

func (e *Extractor) extractSynchronousProcess(node *sitter.Node) *SynchronousProcess {
	proc := &SynchronousProcess{
		LineNum: uint32(node.StartPosition().Row + 1),
	}
	// The micro-vhdl grammar hardcodes 'clk' as the clock signal in every
	// synchronous_process.  Record it on the module so the MLIR emitter can
	// emit the port as !seq.clock instead of i1.
	e.module.ClockPort = "clk"

	cursor := node.Walk()
	if cursor.GotoFirstChild() {
		for {
			n := cursor.Node()
			if n.Kind() == "sequential_assignment" {
				proc.Statements = append(proc.Statements, e.extractSequentialAssignment(n))
			} else if n.Kind() == "sequential_if" {
				proc.Statements = append(proc.Statements, e.extractSequentialIf(n))
			}
			if !cursor.GotoNextSibling() {
				break
			}
		}
	}

	return proc
}

func (e *Extractor) extractSequentialAssignment(node *sitter.Node) *SequentialAssignment {
	stmt := &SequentialAssignment{
		LineNum: uint32(node.StartPosition().Row + 1),
	}
	if targetNode := node.ChildByFieldName("target"); targetNode != nil {
		stmt.Target = e.text(targetNode)
	}
	if valueNode := node.ChildByFieldName("value"); valueNode != nil {
		stmt.Value = e.extractExpression(valueNode)
	}
	return stmt
}

func (e *Extractor) extractSequentialIf(node *sitter.Node) *SequentialIf {
	stmt := &SequentialIf{
		LineNum: uint32(node.StartPosition().Row + 1),
	}

	if condNode := node.ChildByFieldName("condition"); condNode != nil {
		stmt.Condition = e.extractExpression(condNode)
	}

	cursor := node.Walk()
	if cursor.GotoFirstChild() {
		for {
			n := cursor.Node()
			switch n.Kind() {
			case "sequential_assignment":
				stmt.Then = append(stmt.Then, e.extractSequentialAssignment(n))
			case "sequential_if":
				stmt.Then = append(stmt.Then, e.extractSequentialIf(n))
			case "elsif_clause":
				stmt.Elsifs = append(stmt.Elsifs, e.extractElsif(n))
			case "else_clause":
				stmt.Else = e.extractElse(n)
			}
			if !cursor.GotoNextSibling() {
				break
			}
		}
	}
	return stmt
}

func (e *Extractor) extractElsif(node *sitter.Node) Elsif {
	elsif := Elsif{}
	if condNode := node.ChildByFieldName("condition"); condNode != nil {
		elsif.Condition = e.extractExpression(condNode)
	}
	cursor := node.Walk()
	if cursor.GotoFirstChild() {
		for {
			n := cursor.Node()
			switch n.Kind() {
			case "sequential_assignment":
				elsif.Then = append(elsif.Then, e.extractSequentialAssignment(n))
			case "sequential_if":
				elsif.Then = append(elsif.Then, e.extractSequentialIf(n))
			}
			if !cursor.GotoNextSibling() {
				break
			}
		}
	}
	return elsif
}

func (e *Extractor) extractElse(node *sitter.Node) []SequentialStatement {
	var stmts []SequentialStatement
	cursor := node.Walk()
	if cursor.GotoFirstChild() {
		for {
			n := cursor.Node()
			switch n.Kind() {
			case "sequential_assignment":
				stmts = append(stmts, e.extractSequentialAssignment(n))
			case "sequential_if":
				stmts = append(stmts, e.extractSequentialIf(n))
			}
			if !cursor.GotoNextSibling() {
				break
			}
		}
	}
	return stmts
}

func (e *Extractor) extractGenerateStatement(node *sitter.Node) *GenerateStatement {
	stmt := &GenerateStatement{
		LineNum: uint32(node.StartPosition().Row + 1),
	}

	if labelNode := node.ChildByFieldName("label"); labelNode != nil {
		stmt.Label = e.text(labelNode)
	}
	if iterNode := node.ChildByFieldName("iterator"); iterNode != nil {
		stmt.Iterator = e.text(iterNode)
	}
	if startNode := node.ChildByFieldName("start"); startNode != nil {
		if start, err := strconv.Atoi(e.text(startNode)); err == nil {
			stmt.Start = start
		}
	}
	if endNode := node.ChildByFieldName("end"); endNode != nil {
		if end, err := strconv.Atoi(e.text(endNode)); err == nil {
			stmt.End = end
		}
	}

	cursor := node.Walk()
	if cursor.GotoFirstChild() {
		for {
			n := cursor.Node()
			if n.Kind() == "concurrent_assignment" {
				stmt.Statements = append(stmt.Statements, e.extractConcurrentAssignment(n))
			} else if n.Kind() == "selected_assignment" {
				stmt.Statements = append(stmt.Statements, e.extractSelectedAssignment(n))
			} else if n.Kind() == "synchronous_process" {
				stmt.Statements = append(stmt.Statements, e.extractSynchronousProcess(n))
			} else if n.Kind() == "entity_instantiation" {
				stmt.Statements = append(stmt.Statements, e.extractEntityInstantiation(n))
			} else if n.Kind() == "generate_statement" {
				stmt.Statements = append(stmt.Statements, e.extractGenerateStatement(n))
			}
			if !cursor.GotoNextSibling() {
				break
			}
		}
	}

	return stmt
}

func (e *Extractor) extractPslAssertion(node *sitter.Node) *PslAssertion {
	stmt := &PslAssertion{
		LineNum: uint32(node.StartPosition().Row + 1),
	}

	if propNode := node.ChildByFieldName("property"); propNode != nil {
		stmt.Property = e.extractExpression(propNode)
	}

	return stmt
}

func (e *Extractor) extractPslAssumption(node *sitter.Node) *PslAssumption {
	stmt := &PslAssumption{
		LineNum: uint32(node.StartPosition().Row + 1),
	}

	if propNode := node.ChildByFieldName("property"); propNode != nil {
		stmt.Property = e.extractExpression(propNode)
	}

	return stmt
}

func (e *Extractor) extractPslCover(node *sitter.Node) *PslCover {
	stmt := &PslCover{
		LineNum: uint32(node.StartPosition().Row + 1),
	}

	if propNode := node.ChildByFieldName("property"); propNode != nil {
		stmt.Property = e.extractExpression(propNode)
	}

	return stmt
}

func (e *Extractor) extractPslAssertNever(node *sitter.Node) *PslAssertNever {
	stmt := &PslAssertNever{
		LineNum: uint32(node.StartPosition().Row + 1),
	}

	if propNode := node.ChildByFieldName("property"); propNode != nil {
		stmt.Property = e.extractExpression(propNode)
	}

	return stmt
}

func (e *Extractor) extractPslAfterResetAssertion(node *sitter.Node) *PslAfterResetAssertion {
	stmt := &PslAfterResetAssertion{
		LineNum: uint32(node.StartPosition().Row + 1),
	}

	if resetNode := node.ChildByFieldName("reset"); resetNode != nil {
		stmt.Reset = e.extractExpression(resetNode)
	}
	if propNode := node.ChildByFieldName("property"); propNode != nil {
		stmt.Property = e.extractExpression(propNode)
	}

	return stmt
}

func (e *Extractor) extractEntityInstantiation(node *sitter.Node) *EntityInstantiation {
	stmt := &EntityInstantiation{
		LineNum: uint32(node.StartPosition().Row + 1),
	}
	if labelNode := node.ChildByFieldName("label"); labelNode != nil {
		stmt.Label = e.text(labelNode)
	}
	if nameNode := node.ChildByFieldName("entity_name"); nameNode != nil {
		stmt.EntityName = e.text(nameNode)
	}
	if genericMapNode := node.ChildByFieldName("generic_map"); genericMapNode != nil {
		cursor := genericMapNode.Walk()
		if cursor.GotoFirstChild() {
			for {
				n := cursor.Node()
				if n.Kind() == "association_element" {
					if formalNode := n.ChildByFieldName("formal"); formalNode != nil {
						if actualNode := n.ChildByFieldName("actual"); actualNode != nil {
							stmt.GenericMap = append(stmt.GenericMap, PortMapEntry{
								Formal: e.text(formalNode),
								Actual: e.extractExpression(actualNode),
							})
						}
					}
				}
				if !cursor.GotoNextSibling() {
					break
				}
			}
		}
	}
	if portMapNode := node.ChildByFieldName("port_map"); portMapNode != nil {
		cursor := portMapNode.Walk()
		if cursor.GotoFirstChild() {
			for {
				n := cursor.Node()
				if n.Kind() == "association_element" {
					if formalNode := n.ChildByFieldName("formal"); formalNode != nil {
						if actualNode := n.ChildByFieldName("actual"); actualNode != nil {
							stmt.PortMap = append(stmt.PortMap, PortMapEntry{
								Formal: e.text(formalNode),
								Actual: e.extractExpression(actualNode),
							})
						}
					}
				}
				if !cursor.GotoNextSibling() {
					break
				}
			}
		}
	}
	return stmt
}

func (e *Extractor) extractExpression(node *sitter.Node) Expression {
	if node == nil {
		return nil
	}
	kind := node.Kind()

	// Handle tiered grammar nodes (e.g., logical_expression -> relational_expression)
	// by unwrapping if there is only one named child.
	if (kind == "logical_expression" || kind == "relational_expression" ||
		kind == "additive_expression" || kind == "multiplicative_expression") &&
		node.NamedChildCount() == 1 {
		return e.extractExpression(node.NamedChild(0))
	}

	// Check for binary expression patterns in the tiered grammar
	if node.ChildByFieldName("left") != nil && node.ChildByFieldName("operator") != nil && node.ChildByFieldName("right") != nil {
		return BinaryExpr{
			Op:    strings.TrimSpace(e.text(node.ChildByFieldName("operator"))),
			Left:  e.extractExpression(node.ChildByFieldName("left")),
			Right: e.extractExpression(node.ChildByFieldName("right")),
		}
	}

	switch kind {
	case "identifier":
		return IdentifierExpr{Name: e.text(node)}
	case "number", "string_literal", "char_literal":
		return LiteralExpr{Value: e.text(node)}
	case "indexed_name":
		return IndexedNameExpr{
			Prefix: e.text(node.ChildByFieldName("prefix")),
			Index:  e.extractExpression(node.ChildByFieldName("index")),
		}
	case "selected_name":
		return SelectedNameExpr{
			Prefix: e.text(node.ChildByFieldName("prefix")),
			Suffix: e.text(node.ChildByFieldName("suffix")),
		}
	case "paren_expression":
		// The expression is the middle child (index 1), between '(' and ')'
		return ParenExpr{
			Expr: e.extractExpression(node.Child(1)),
		}
	case "unary_expression":
		return UnaryExpr{
			Op:    strings.TrimSpace(e.text(node.ChildByFieldName("operator"))),
			Right: e.extractExpression(node.ChildByFieldName("argument")),
		}
	case "psl_implication":
		return PslImplicationExpr{
			Op:    strings.TrimSpace(e.text(node.ChildByFieldName("operator"))),
			Left:  e.extractExpression(node.ChildByFieldName("left")),
			Right: e.extractExpression(node.ChildByFieldName("right")),
		}
	case "psl_sequence":
		var elements []PslSequenceElement
		cursor := node.Walk()
		if cursor.GotoFirstChild() {
			for {
				n := cursor.Node()
				if n.Kind() == "psl_sequence_element" {
					elements = append(elements, e.extractPslSequenceElement(n))
				}
				if !cursor.GotoNextSibling() {
					break
				}
			}
		}
		return PslSequenceExpr{Elements: elements}
	case "psl_eventually":
		return PslEventuallyExpr{
			Left:  e.extractExpression(node.ChildByFieldName("left")),
			Right: e.extractExpression(node.ChildByFieldName("right")),
		}
	case "psl_stable":
		return PslStableExpr{
			Signal: e.extractExpression(node.ChildByFieldName("signal")),
		}
	case "psl_next":
		return PslNextExpr{
			Delay: e.parseNumber(node.ChildByFieldName("delay")),
			Arg:   e.extractExpression(node.ChildByFieldName("arg")),
		}
	case "psl_next_range":
		return PslNextRangeExpr{
			Start: e.parseNumber(node.ChildByFieldName("start")),
			End:   e.parseNumber(node.ChildByFieldName("end")),
			Arg:   e.extractExpression(node.ChildByFieldName("arg")),
		}
	}

	return LiteralExpr{Value: e.text(node)}
}

func (e *Extractor) extractPslSequenceElement(node *sitter.Node) PslSequenceElement {
	element := PslSequenceElement{}
	// The first child is the expression (_psl_expression)
	element.Expr = e.extractExpression(node.Child(0))

	// Check for optional repetition
	// Tree-sitter includes named fields and anonymous children like ';' or '{'
	// but psl_sequence_element rule is just seq($._psl_expression, optional($.psl_repetition))
	// However, sometimes there might be extra hidden children.
	// It's safer to iterate children.
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "psl_repetition" {
			rep := &PslRepetition{Count: -1}
			if countNode := child.ChildByFieldName("count"); countNode != nil {
				rep.Count = e.parseNumber(countNode)
			} else {
				rep.Start = e.parseNumber(child.ChildByFieldName("start"))
				rep.End = e.parseNumber(child.ChildByFieldName("end"))
			}
			element.Repetition = rep
		}
	}
	return element
}

