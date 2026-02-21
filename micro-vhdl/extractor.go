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

func (e *Extractor) ExtractAll(root *sitter.Node) ([]*Module, error) {
	var modules []*Module
	
	entities := make(map[string]*sitter.Node)
	arches := make(map[string]*sitter.Node)

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
			}
			if !cursor.GotoNextSibling() {
				break
			}
		}
	}

	for name, entityNode := range entities {
		e.module = NewModule(name)
		e.extractInterface(entityNode)
		if archNode, ok := arches[name]; ok {
			e.extractArchitecture(archNode)
		}
		modules = append(modules, e.module)
	}

	return modules, nil
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
			}
			if !cursor.GotoNextSibling() {
				break
			}
		}
	}
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
	if text == "std_logic" {
		return Type{Name: "std_logic", Width: 1}
	} else if text == "integer" {
		return Type{Name: "integer", Width: 32}
	}

	// Simple check for std_logic_vector
	if strings.Contains(text, "downto") {
		startIdx := strings.Index(text, "(")
		downtoIdx := strings.Index(text, "downto")
		endIdx := strings.Index(text, ")")
		if startIdx != -1 && downtoIdx != -1 && endIdx != -1 {
			base := strings.TrimSpace(text[:startIdx])
			leftStr := strings.TrimSpace(text[startIdx+1 : downtoIdx])
			rightStr := strings.TrimSpace(text[downtoIdx+6 : endIdx])

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
	case "binary_expression":
		return BinaryExpr{
			Op:    strings.TrimSpace(e.text(node.ChildByFieldName("operator"))),
			Left:  e.extractExpression(node.ChildByFieldName("left")),
			Right: e.extractExpression(node.ChildByFieldName("right")),
		}
	case "unary_expression":
		return UnaryExpr{
			Op:    strings.TrimSpace(e.text(node.ChildByFieldName("operator"))),
			Right: e.extractExpression(node.ChildByFieldName("argument")),
		}
	case "psl_implication":
		return PslImplicationExpr{
			Left:  e.extractExpression(node.ChildByFieldName("left")),
			Right: e.extractExpression(node.ChildByFieldName("right")),
		}
	case "psl_eventually":
		return PslEventuallyExpr{
			Left:  e.extractExpression(node.ChildByFieldName("left")),
			Right: e.extractExpression(node.ChildByFieldName("right")),
		}
	case "psl_stable":
		return PslStableExpr{
			Signal: e.extractExpression(node.ChildByFieldName("signal")),
		}
	}

	return LiteralExpr{Value: e.text(node)}
}
