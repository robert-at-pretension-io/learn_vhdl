package extractor

// =============================================================================
// EXTRACTION PHILOSOPHY: GRAMMAR-FIRST, EXTRACTOR-SIMPLE
// =============================================================================
//
// The extractor should be DUMB. The grammar should be SMART.
//
// WRONG APPROACH (fight the grammar, complex extractor):
//   - Grammar produces flat token stream
//   - Extractor walks tokens, tries to reconstruct structure
//   - Extractor has complex state machines, lookahead, backtracking
//   - Bugs are hard to find, edge cases multiply
//
// RIGHT APPROACH (invest in grammar, simple extractor):
//   - Grammar produces rich, structured AST with named nodes
//   - Extractor does simple pattern matching on node types
//   - Each extraction function is a few lines: match node, extract fields
//   - New analysis = new grammar rule + simple extraction
//
// WHEN YOU NEED NEW EXTRACTION:
//   1. FIRST: Can the grammar expose this structure directly?
//      - Add a visible node type (not underscore-prefixed)
//      - Use field() to name important children
//      - Run `tree-sitter generate` and test
//
//   2. ONLY IF GRAMMAR CAN'T HELP: Write extraction logic here
//      - Keep it simple: walk children, match types
//      - No complex state machines or lookahead
//      - If it's getting complex, go back to step 1
//
// EXAMPLES:
//   - Need to find comparisons? Grammar has `relational_expression` with
//     field('left'), field('operator'), field('right')
//   - Need to find multiplications? Grammar has `multiplicative_expression`
//   - Need to find case statements? Grammar has `case_statement` with
//     `case_alternative` children and `others_choice` nodes
//
// The grammar is your friend. Make it do the heavy lifting.
// =============================================================================

import (
	"context"
	"fmt"
	"os"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	tree_sitter_vhdl "github.com/tree-sitter/tree-sitter-vhdl"
)

// Extractor uses Tree-sitter to parse VHDL files and extract facts
type Extractor struct {
	lang *sitter.Language
}

// FileFacts contains all extracted information from a single VHDL file
type FileFacts struct {
	File           string
	Entities       []Entity
	Architectures  []Architecture
	Packages       []Package
	Components     []Component
	Dependencies   []Dependency
	Signals        []Signal
	Ports          []Port
	Processes      []Process
	Instances      []Instance      // Component/entity instantiations
	CaseStatements []CaseStatement // Case statements for latch detection
	// Concurrent statements (outside processes)
	ConcurrentAssignments []ConcurrentAssignment // Concurrent signal assignments
	// Semantic analysis
	ClockDomains  []ClockDomain
	SignalUsages  []SignalUsage
	ResetInfos    []ResetInfo
	// Advanced analysis for security/power/correctness
	Comparisons      []Comparison      // Comparisons for trojan detection
	ArithmeticOps    []ArithmeticOp    // Expensive operations for power analysis
	SignalDeps       []SignalDep       // Signal dependencies for loop detection
}

// ClockDomain represents a clock and the signals it drives
type ClockDomain struct {
	Clock     string   // Clock signal name
	Edge      string   // "rising" or "falling"
	Registers []string // Signals assigned under this clock
	Process   string   // Which process
	Line      int
}

// SignalUsage tracks where a signal is read or written
type SignalUsage struct {
	Signal    string
	IsRead    bool   // Appears on RHS of assignment
	IsWritten bool   // Appears on LHS of assignment
	InProcess string // Which process (empty if concurrent)
	Line      int
}

// ResetInfo represents reset signal detection
type ResetInfo struct {
	Signal    string // Reset signal name
	Polarity  string // "active_high" or "active_low"
	IsAsync   bool   // true if async (checked before clock edge)
	Registers []string
	Process   string
	Line      int
}

// Process represents a VHDL process statement
type Process struct {
	Label           string   // Optional label
	SensitivityList []string // Signals in sensitivity list (or "all" for VHDL-2008)
	Line            int
	InArch          string // Which architecture this process belongs to
	// Semantic info
	IsSequential    bool     // Has clock edge (rising_edge/falling_edge)
	IsCombinational bool     // No clock edge
	ClockSignal     string   // Clock signal if sequential
	ClockEdge       string   // "rising" or "falling"
	HasReset        bool     // Has reset logic
	ResetSignal     string   // Reset signal name
	ResetAsync      bool     // Is reset asynchronous
	AssignedSignals []string // Signals assigned in this process
	ReadSignals     []string // Signals read in this process
}

// ConcurrentAssignment represents a concurrent signal assignment (outside processes)
// Examples:
//   - Simple: sig <= a and b;
//   - Conditional: sig <= a when sel = '1' else b;
//   - Selected: with sel select sig <= a when "00", b when others;
type ConcurrentAssignment struct {
	Target      string   // Signal being assigned (LHS)
	ReadSignals []string // Signals being read (RHS)
	Line        int
	InArch      string // Which architecture contains this assignment
	Kind        string // "simple", "conditional", "selected"
}

// Comparison represents a comparison operation for trojan/trigger detection
// Tracks comparisons against literals, especially large "magic" values
type Comparison struct {
	LeftOperand  string // Signal or expression on left
	Operator     string // =, /=, <, >, <=, >=
	RightOperand string // Signal, literal, or expression on right
	IsLiteral    bool   // True if right operand is a literal value
	LiteralValue string // The literal value if IsLiteral
	LiteralBits  int    // Estimated bit width of literal
	ResultDrives string // What signal does this comparison result drive
	Line         int
	InProcess    string
	InArch       string
}

// ArithmeticOp represents an expensive arithmetic operation for power analysis
type ArithmeticOp struct {
	Operator     string   // *, /, mod, rem, **
	Operands     []string // Input signals/expressions
	Result       string   // Output signal
	IsGuarded    bool     // True if inputs are gated by enable
	GuardSignal  string   // The enable/valid signal if guarded
	Line         int
	InProcess    string
	InArch       string
}

// SignalDep represents a signal dependency for combinational loop detection
type SignalDep struct {
	Source       string // Signal being read
	Target       string // Signal being assigned
	InProcess    string // Which process (empty if concurrent)
	IsSequential bool   // True if crosses a clock boundary
	Line         int
	InArch       string
}

// Entity represents a VHDL entity declaration
type Entity struct {
	Name  string
	Line  int
	Ports []Port
}

// Architecture represents a VHDL architecture body
type Architecture struct {
	Name       string
	EntityName string
	Line       int
}

// Package represents a VHDL package declaration
type Package struct {
	Name string
	Line int
}

// Component represents a component declaration or instantiation
type Component struct {
	Name       string
	EntityRef  string // The entity it references
	Line       int
	IsInstance bool
}

// Instance represents a component/entity instantiation with port mapping
// This enables system-level analysis (cross-module signal tracing)
type Instance struct {
	Name       string            // Instance label (e.g., "u_cpu")
	Target     string            // Target entity/component (e.g., "work.cpu")
	PortMap    map[string]string // Formal port -> actual signal mapping
	GenericMap map[string]string // Formal generic -> actual value mapping
	Line       int
	InArch     string // Which architecture contains this instance
}

// CaseStatement represents a VHDL case statement for latch detection
// A case statement without "others" can infer a latch in combinational logic
type CaseStatement struct {
	Expression   string   // The case expression (e.g., "state")
	Choices      []string // All explicit choices (e.g., ["0", "1", "idle"])
	HasOthers    bool     // true if "when others =>" is present
	Line         int
	InProcess    string // Which process contains this case statement
	InArch       string // Which architecture
	IsComplete   bool   // true if HasOthers or all possible values covered
}

// Dependency represents a use/library clause or instantiation
type Dependency struct {
	Source string // The file/entity that has the dependency
	Target string // What it depends on (e.g., "work.my_pkg")
	Kind   string // "use", "library", "instantiation"
	Line   int
}

// Signal represents a signal declaration
type Signal struct {
	Name     string
	Type     string
	Line     int
	InEntity string // Which entity/arch it belongs to
}

// Port represents an entity port
type Port struct {
	Name      string
	Direction string // in, out, inout, buffer
	Type      string
	Line      int
	InEntity  string // Which entity this port belongs to
}

// New creates a new Extractor with VHDL language loaded
func New() *Extractor {
	// Load the VHDL language (thread-safe, can be shared)
	lang := sitter.NewLanguage(tree_sitter_vhdl.Language())

	return &Extractor{
		lang: lang,
	}
}

// Extract parses a VHDL file and extracts facts
// Creates a new parser per call for thread safety
func (e *Extractor) Extract(filePath string) (FileFacts, error) {
	facts := FileFacts{File: filePath}

	// Read file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return facts, fmt.Errorf("reading file: %w", err)
	}

	// If no language set, use simple regex-based extraction as fallback
	if e.lang == nil {
		return e.extractSimple(filePath, content)
	}

	// Create a new parser for this extraction (thread-safe)
	parser := sitter.NewParser()
	parser.SetLanguage(e.lang)

	// Parse with Tree-sitter
	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return facts, fmt.Errorf("parsing: %w", err)
	}
	defer tree.Close()

	// Walk the tree and extract facts
	e.walkTree(tree.RootNode(), content, &facts, "")

	return facts, nil
}

// walkTree traverses the syntax tree and extracts relevant nodes
func (e *Extractor) walkTree(node *sitter.Node, source []byte, facts *FileFacts, context string) {
	if node == nil {
		return
	}

	nodeType := node.Type()

	switch nodeType {
	case "entity_declaration":
		entity := e.extractEntity(node, source)
		facts.Entities = append(facts.Entities, entity)
		// Extract ports from entity
		e.extractPortsFromEntity(node, source, entity.Name, facts)
		context = entity.Name

	case "architecture_body":
		arch := e.extractArchitecture(node, source)
		facts.Architectures = append(facts.Architectures, arch)
		context = arch.Name

	case "package_declaration":
		pkg := e.extractPackage(node, source)
		facts.Packages = append(facts.Packages, pkg)
		context = pkg.Name

	case "use_clause":
		dep := e.extractUseClause(node, source, facts.File)
		facts.Dependencies = append(facts.Dependencies, dep)

	case "library_clause":
		dep := e.extractLibraryClause(node, source, facts.File)
		if dep.Target != "" {
			facts.Dependencies = append(facts.Dependencies, dep)
		}

	case "component_instantiation":
		comp := e.extractComponentInst(node, source)
		facts.Components = append(facts.Components, comp)
		// Add as dependency
		if comp.EntityRef != "" {
			facts.Dependencies = append(facts.Dependencies, Dependency{
				Source: facts.File,
				Target: comp.EntityRef,
				Kind:   "instantiation",
				Line:   comp.Line,
			})
		}
		// Also extract as Instance with port/generic maps for system-level analysis
		inst := e.extractInstance(node, source, context)
		facts.Instances = append(facts.Instances, inst)

	case "signal_declaration":
		signals := e.extractSignals(node, source, context)
		facts.Signals = append(facts.Signals, signals...)

	case "component_declaration":
		comp := e.extractComponentDecl(node, source)
		facts.Components = append(facts.Components, comp)

	case "signal_assignment":
		// Concurrent signal assignment (outside processes)
		// Note: Sequential assignments inside processes are "sequential_signal_assignment"
		ca := e.extractConcurrentAssignment(node, source, context)
		facts.ConcurrentAssignments = append(facts.ConcurrentAssignments, ca)
		// Add to signal usages
		facts.SignalUsages = append(facts.SignalUsages, SignalUsage{
			Signal:    ca.Target,
			IsWritten: true,
			InProcess: "", // Empty = concurrent
			Line:      ca.Line,
		})
		for _, sig := range ca.ReadSignals {
			facts.SignalUsages = append(facts.SignalUsages, SignalUsage{
				Signal:    sig,
				IsRead:    true,
				InProcess: "", // Empty = concurrent
				Line:      ca.Line,
			})
		}
		// Extract signal dependencies for loop detection
		deps := e.extractSignalDepsFromConcurrent(ca, context)
		facts.SignalDeps = append(facts.SignalDeps, deps...)

	case "process_statement":
		proc := e.extractProcess(node, source, context)
		facts.Processes = append(facts.Processes, proc)
		// Extract case statements within the process for latch detection
		e.extractCaseStatementsFromProcess(node, source, context, proc.Label, facts)
		// Extract comparisons for trojan/trigger detection
		e.extractComparisonsFromProcess(node, source, context, proc.Label, facts)
		// Extract arithmetic operations for power analysis
		e.extractArithmeticOpsFromProcess(node, source, context, proc.Label, facts)
		// Extract signal dependencies for loop detection
		e.extractSignalDepsFromProcess(node, source, context, proc.Label, proc.IsSequential, facts)

		// Add to semantic collections
		if proc.ClockSignal != "" {
			facts.ClockDomains = append(facts.ClockDomains, ClockDomain{
				Clock:     proc.ClockSignal,
				Edge:      proc.ClockEdge,
				Registers: proc.AssignedSignals,
				Process:   proc.Label,
				Line:      proc.Line,
			})
		}

		if proc.HasReset {
			facts.ResetInfos = append(facts.ResetInfos, ResetInfo{
				Signal:    proc.ResetSignal,
				Polarity:  "active_high", // TODO: detect from comparison value
				IsAsync:   proc.ResetAsync,
				Registers: proc.AssignedSignals,
				Process:   proc.Label,
				Line:      proc.Line,
			})
		}

		// Add signal usages
		for _, sig := range proc.AssignedSignals {
			facts.SignalUsages = append(facts.SignalUsages, SignalUsage{
				Signal:    sig,
				IsWritten: true,
				InProcess: proc.Label,
				Line:      proc.Line,
			})
		}
		for _, sig := range proc.ReadSignals {
			facts.SignalUsages = append(facts.SignalUsages, SignalUsage{
				Signal:    sig,
				IsRead:    true,
				InProcess: proc.Label,
				Line:      proc.Line,
			})
		}
	}

	// Recurse into children
	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkTree(node.Child(i), source, facts, context)
	}
}

func (e *Extractor) extractEntity(node *sitter.Node, source []byte) Entity {
	entity := Entity{
		Line: int(node.StartPoint().Row) + 1,
	}

	// Find the name field
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		entity.Name = nameNode.Content(source)
	}

	return entity
}

func (e *Extractor) extractArchitecture(node *sitter.Node, source []byte) Architecture {
	arch := Architecture{
		Line: int(node.StartPoint().Row) + 1,
	}

	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		arch.Name = nameNode.Content(source)
	}
	if entityNode := node.ChildByFieldName("entity"); entityNode != nil {
		arch.EntityName = entityNode.Content(source)
	}

	return arch
}

func (e *Extractor) extractPackage(node *sitter.Node, source []byte) Package {
	pkg := Package{
		Line: int(node.StartPoint().Row) + 1,
	}

	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		pkg.Name = nameNode.Content(source)
	}

	return pkg
}

func (e *Extractor) extractUseClause(node *sitter.Node, source []byte, file string) Dependency {
	// Extract the library.package path from use clause
	target := ""
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" || child.Type() == "selector_clause" {
			target += child.Content(source)
		}
	}
	return Dependency{
		Source: file,
		Target: target,
		Kind:   "use",
		Line:   int(node.StartPoint().Row) + 1,
	}
}

func (e *Extractor) extractLibraryClause(node *sitter.Node, source []byte, file string) Dependency {
	// Extract library name
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" {
			return Dependency{
				Source: file,
				Target: child.Content(source),
				Kind:   "library",
				Line:   int(node.StartPoint().Row) + 1,
			}
		}
	}
	return Dependency{}
}

func (e *Extractor) extractComponentInst(node *sitter.Node, source []byte) Component {
	comp := Component{
		Line:       int(node.StartPoint().Row) + 1,
		IsInstance: true,
	}

	if labelNode := node.ChildByFieldName("label"); labelNode != nil {
		comp.Name = labelNode.Content(source)
	}
	if compNode := node.ChildByFieldName("component"); compNode != nil {
		comp.EntityRef = compNode.Content(source)
	}

	return comp
}

func (e *Extractor) extractComponentDecl(node *sitter.Node, source []byte) Component {
	comp := Component{
		Line:       int(node.StartPoint().Row) + 1,
		IsInstance: false,
	}

	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		comp.Name = nameNode.Content(source)
	}

	return comp
}

// extractInstance extracts a component/entity instantiation with port/generic mappings
// This enables system-level analysis (tracing signals through the hierarchy)
func (e *Extractor) extractInstance(node *sitter.Node, source []byte, context string) Instance {
	inst := Instance{
		Line:       int(node.StartPoint().Row) + 1,
		InArch:     context,
		PortMap:    make(map[string]string),
		GenericMap: make(map[string]string),
	}

	// Extract using field names and node types (clean declarative approach)

	// Instance label
	if labelNode := node.ChildByFieldName("label"); labelNode != nil {
		inst.Name = labelNode.Content(source)
	}

	// Target - either component name or entity reference
	if compNode := node.ChildByFieldName("component"); compNode != nil {
		inst.Target = compNode.Content(source)
	} else {
		// Direct entity instantiation: entity lib.entity(arch)
		libNode := node.ChildByFieldName("library")
		entityNode := node.ChildByFieldName("entity")
		if libNode != nil && entityNode != nil {
			inst.Target = libNode.Content(source) + "." + entityNode.Content(source)
		}
	}

	// Extract generic_map_aspect and port_map_aspect
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "generic_map_aspect":
			e.extractMapAspect(child, source, inst.GenericMap)
		case "port_map_aspect":
			e.extractMapAspect(child, source, inst.PortMap)
		}
	}

	return inst
}

// extractMapAspect extracts associations from a generic_map_aspect or port_map_aspect node
func (e *Extractor) extractMapAspect(node *sitter.Node, source []byte, result map[string]string) {
	// Find the association_list child
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "association_list" {
			e.extractAssociationList(child, source, result)
			return
		}
	}
}

// extractAssociationList extracts named associations from an association_list node
func (e *Extractor) extractAssociationList(node *sitter.Node, source []byte, result map[string]string) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "association_element" {
			formal, actual := e.extractAssociationElement(child, source)
			if formal != "" && actual != "" {
				result[formal] = actual
			}
		}
	}
}

// extractAssociationElement extracts formal => actual from an association_element
func (e *Extractor) extractAssociationElement(node *sitter.Node, source []byte) (formal, actual string) {
	// Structure: identifier "=>" (identifier | number | expression | "open")
	sawArrow := false
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		childType := child.Type()

		if childType == "=>" {
			sawArrow = true
			continue
		}

		if childType == "identifier" || childType == "number" || childType == "character_literal" {
			if !sawArrow {
				formal = child.Content(source)
			} else {
				actual = child.Content(source)
			}
		}
	}
	return
}

// extractConcurrentAssignment extracts a concurrent signal assignment
// Handles: simple (sig <= expr), conditional (sig <= a when c else b), selected (with s select sig <= ...)
func (e *Extractor) extractConcurrentAssignment(node *sitter.Node, source []byte, context string) ConcurrentAssignment {
	ca := ConcurrentAssignment{
		Line:   int(node.StartPoint().Row) + 1,
		InArch: context,
		Kind:   "simple",
	}

	// Determine kind based on content
	content := strings.ToLower(node.Content(source))
	isSelected := strings.Contains(content, "select")
	if strings.Contains(content, " when ") && strings.Contains(content, " else ") {
		ca.Kind = "conditional"
	} else if isSelected {
		ca.Kind = "selected"
	}

	readSet := make(map[string]bool)
	foundTarget := false
	foundArrow := false // <=
	identifierCount := 0

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		childType := child.Type()
		childContent := child.Content(source)

		// Track when we pass the assignment operator
		if childContent == "<=" {
			foundArrow = true
			continue
		}

		if childType == "identifier" {
			if !foundArrow {
				identifierCount++
				// For selected assignment: with sel select target <= ...
				// First identifier is selector (read), second is target
				if isSelected {
					if identifierCount == 1 {
						// First identifier is the selector - it's a read signal
						readSet[childContent] = true
					} else if identifierCount == 2 && !foundTarget {
						// Second identifier is the target
						ca.Target = childContent
						foundTarget = true
					}
				} else {
					// For simple/conditional: target <= expr
					// First identifier before <= is the target
					if !foundTarget {
						ca.Target = childContent
						foundTarget = true
					}
				}
			} else {
				// After <=, all identifiers are read signals
				readSet[childContent] = true
			}
		} else if childType == "selected_name" {
			// Handle record field access (e.g., rec.field)
			// Extract base signal name
			baseSig := e.extractBaseSignal(child, source)
			if baseSig != "" {
				if !foundArrow && !foundTarget {
					ca.Target = baseSig
					foundTarget = true
				} else if foundArrow {
					readSet[baseSig] = true
				}
			}
		}
	}

	// Convert read set to slice
	for sig := range readSet {
		ca.ReadSignals = append(ca.ReadSignals, sig)
	}

	return ca
}

// extractBaseSignal extracts the base signal from a selected_name (e.g., "rec" from "rec.field")
func (e *Extractor) extractBaseSignal(node *sitter.Node, source []byte) string {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" {
			return child.Content(source)
		}
		if child.Type() == "selected_name" {
			return e.extractBaseSignal(child, source)
		}
	}
	return ""
}

func (e *Extractor) extractSignals(node *sitter.Node, source []byte, context string) []Signal {
	var signals []Signal
	line := int(node.StartPoint().Row) + 1

	// signal_declaration structure:
	// (signal_declaration
	//   "signal"
	//   (identifier name)      <- first name (has field)
	//   ","
	//   (identifier)           <- additional names (no field)
	//   ":"
	//   (identifier)           <- type (in _signal_type_indication -> _type_mark)
	//   ";")

	var names []string
	var sigType string
	foundColon := false

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		childType := child.Type()
		content := child.Content(source)

		if content == ":" {
			foundColon = true
			continue
		}

		if childType == "identifier" {
			if !foundColon {
				// Before colon = signal names
				names = append(names, content)
			} else {
				// After colon = type (take the last identifier as the main type)
				sigType = content
			}
		}
	}

	for _, name := range names {
		signals = append(signals, Signal{
			Name:     name,
			Type:     sigType,
			Line:     line,
			InEntity: context,
		})
	}

	return signals
}

func (e *Extractor) extractPortsFromEntity(node *sitter.Node, source []byte, entityName string, facts *FileFacts) {
	// Walk through entity looking for parameters (ports)
	var walkForPorts func(n *sitter.Node)
	walkForPorts = func(n *sitter.Node) {
		if n == nil {
			return
		}
		if n.Type() == "parameter" {
			ports := e.extractPorts(n, source)
			for i := range ports {
				ports[i].InEntity = entityName
			}
			facts.Ports = append(facts.Ports, ports...)
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walkForPorts(n.Child(i))
		}
	}
	walkForPorts(node)
}

// extractPorts extracts one or more ports from a parameter node
// A single parameter can declare multiple ports: a, b : in std_logic
func (e *Extractor) extractPorts(node *sitter.Node, source []byte) []Port {
	line := int(node.StartPoint().Row) + 1
	direction := ""

	// Extract direction from field (now visible in grammar as port_direction)
	if dirNode := node.ChildByFieldName("direction"); dirNode != nil {
		direction = strings.ToLower(dirNode.Content(source))
	}

	// Collect all identifiers and find where direction appears
	// Structure: [name1, name2, ...] direction type
	var identifiers []string
	directionIndex := -1

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		childType := child.Type()

		if childType == "port_direction" {
			directionIndex = len(identifiers)
		} else if childType == "identifier" {
			identifiers = append(identifiers, child.Content(source))
		}
	}

	// Determine which identifiers are names vs type
	// Names come before direction, type comes after
	var names []string
	var portType string

	if directionIndex >= 0 && len(identifiers) > directionIndex {
		// Direction was found - names are before direction, type is after
		names = identifiers[:directionIndex]
		if directionIndex < len(identifiers) {
			portType = identifiers[directionIndex] // First identifier after direction is the type
		}
	} else if len(identifiers) >= 2 {
		// No direction - all but last are names, last is type
		names = identifiers[:len(identifiers)-1]
		portType = identifiers[len(identifiers)-1]
	} else if len(identifiers) == 1 {
		// Just one identifier, treat as name
		names = identifiers
	}

	// Create a port for each name
	var ports []Port
	for _, name := range names {
		ports = append(ports, Port{
			Name:      name,
			Direction: direction,
			Type:      portType,
			Line:      line,
		})
	}

	return ports
}

func (e *Extractor) extractProcess(node *sitter.Node, source []byte, context string) Process {
	proc := Process{
		Line:   int(node.StartPoint().Row) + 1,
		InArch: context,
	}

	// Walk through children to find label and sensitivity list
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		childType := child.Type()

		switch childType {
		case "identifier":
			// First identifier before ':' is the label
			if proc.Label == "" {
				// Check if this is followed by ':'
				if i+1 < int(node.ChildCount()) {
					next := node.Child(i + 1)
					if next.Content(source) == ":" {
						proc.Label = child.Content(source)
					}
				}
			}
		case "sensitivity_list":
			// Extract signals from sensitivity list
			proc.SensitivityList = e.extractSensitivityList(child, source)
		}
	}

	// Semantic analysis: walk the process body for clock edges, resets, and signal usage
	e.analyzeProcessSemantics(node, source, &proc)

	// Determine if combinational or sequential
	if proc.ClockSignal != "" {
		proc.IsSequential = true
	} else {
		proc.IsCombinational = true
	}

	return proc
}

// analyzeProcessSemantics walks the process body to extract semantic information
func (e *Extractor) analyzeProcessSemantics(node *sitter.Node, source []byte, proc *Process) {
	assignedSet := make(map[string]bool)
	readSet := make(map[string]bool)

	var walk func(n *sitter.Node, inCondition bool)
	walk = func(n *sitter.Node, inCondition bool) {
		if n == nil {
			return
		}

		nodeType := n.Type()

		switch nodeType {
		case "indexed_name":
			// Check for rising_edge(clk) or falling_edge(clk)
			if n.ChildCount() >= 2 {
				funcName := ""
				argName := ""
				for i := 0; i < int(n.ChildCount()); i++ {
					child := n.Child(i)
					if child.Type() == "identifier" {
						if funcName == "" {
							funcName = strings.ToLower(child.Content(source))
						} else if argName == "" {
							argName = child.Content(source)
						}
					}
				}
				if funcName == "rising_edge" && argName != "" {
					proc.ClockSignal = argName
					proc.ClockEdge = "rising"
				} else if funcName == "falling_edge" && argName != "" {
					proc.ClockSignal = argName
					proc.ClockEdge = "falling"
				}
			}

		case "sequential_signal_assignment":
			// Extract LHS (assigned signal) and RHS (read signals)
			for i := 0; i < int(n.ChildCount()); i++ {
				child := n.Child(i)
				if child.Type() == "identifier" {
					// First identifier is typically the target
					sig := child.Content(source)
					assignedSet[sig] = true
					break
				}
			}
			// Walk RHS for reads
			e.extractReadsFromNode(n, source, readSet, true)

		case "if_statement":
			// Check for reset pattern in if condition
			// Pattern: if reset = '1' then (before clock edge = async)
			// Pattern: elsif reset = '1' then (after clock edge = sync)
			e.checkResetPattern(n, source, proc)
			// Extract reads from condition (identifiers before first statement)
			e.extractIfConditionReads(n, source, readSet)
			// Continue walking children for nested statements
			for i := 0; i < int(n.ChildCount()); i++ {
				walk(n.Child(i), false)
			}
			return

		case "case_statement":
			// Extract reads from case expression
			e.extractCaseExpressionReads(n, source, readSet)
			// Continue walking children for case alternatives
			for i := 0; i < int(n.ChildCount()); i++ {
				walk(n.Child(i), false)
			}
			return

		case "identifier":
			// In expression context, this is a read
			if inCondition {
				readSet[n.Content(source)] = true
			}
		}

		// Recurse into children
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i), inCondition)
		}
	}

	walk(node, false)

	// Convert sets to slices
	for sig := range assignedSet {
		proc.AssignedSignals = append(proc.AssignedSignals, sig)
	}
	for sig := range readSet {
		proc.ReadSignals = append(proc.ReadSignals, sig)
	}
}

// extractCaseExpressionReads extracts signal reads from case expression
func (e *Extractor) extractCaseExpressionReads(node *sitter.Node, source []byte, readSet map[string]bool) {
	// Case expression is typically the first identifier/expression after "case" keyword
	// Structure: case expr is when choice => stmts... end case;
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		childType := child.Type()

		// Stop when we hit case_alternative (the "when" clauses)
		if childType == "case_alternative" {
			break
		}

		// Extract identifiers from expression
		if childType == "identifier" {
			readSet[child.Content(source)] = true
		} else if childType == "indexed_name" || childType == "selected_name" {
			e.extractReadsFromNode(child, source, readSet, false)
		}
	}
}

// extractIfConditionReads extracts signal reads from if/elsif conditions
// In tree-sitter, condition elements appear as direct children before any statement nodes
func (e *Extractor) extractIfConditionReads(node *sitter.Node, source []byte, readSet map[string]bool) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		childType := child.Type()

		// Stop when we hit a statement or another if_statement (elsif/else body)
		if childType == "sequential_signal_assignment" ||
			childType == "if_statement" ||
			childType == "case_statement" ||
			childType == "loop_statement" ||
			childType == "return_statement" ||
			childType == "null_statement" ||
			childType == "wait_statement" ||
			childType == "assertion_statement" ||
			childType == "report_statement" ||
			childType == "variable_assignment" {
			break
		}

		// Extract identifiers from condition
		if childType == "identifier" {
			readSet[child.Content(source)] = true
		} else if childType == "indexed_name" || childType == "selected_name" {
			// Handle function calls and record access in conditions
			e.extractReadsFromNode(child, source, readSet, false)
		}
	}
}

// extractReadsFromNode finds all identifiers read in an expression
// It handles selected_name (record field access) by only extracting the base signal
func (e *Extractor) extractReadsFromNode(node *sitter.Node, source []byte, readSet map[string]bool, skipFirst bool) {
	first := true
	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}

		// For selected_name (e.g., a_req_i.stb), only extract the first identifier (base signal)
		// The field name (.stb) is not a signal read - the base signal (a_req_i) is
		if n.Type() == "selected_name" {
			for i := 0; i < int(n.ChildCount()); i++ {
				child := n.Child(i)
				if child.Type() == "identifier" {
					if skipFirst && first {
						first = false
					} else {
						readSet[child.Content(source)] = true
					}
					break // Only take the first identifier (base signal)
				}
				// If the first child is another selected_name, recurse
				if child.Type() == "selected_name" {
					walk(child)
					break
				}
			}
			return // Don't recurse into children - we've handled the selected_name
		}

		if n.Type() == "identifier" {
			if skipFirst && first {
				first = false
			} else {
				readSet[n.Content(source)] = true
			}
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(node)
}

// checkResetPattern looks for reset patterns in if statements
// Pattern: if reset = '1' then (async if first condition before clock edge)
func (e *Extractor) checkResetPattern(node *sitter.Node, source []byte, proc *Process) {
	// node is an if_statement - look at immediate children
	// The tree structure is: identifier, relational_op, literal, [statements...], indexed_name (elsif cond), [statements...]
	// We want to detect the FIRST condition (before any clock edge check)

	var firstIdentifier string
	var sawRelOp bool
	var firstValue string
	var sawClockEdgeFirst bool
	foundFirstCondition := false

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		childType := child.Type()

		// Stop collecting the first condition when we hit a statement or another condition marker
		if childType == "sequential_signal_assignment" || childType == "if_statement" {
			foundFirstCondition = true
		}

		if !foundFirstCondition {
			switch childType {
			case "identifier":
				if firstIdentifier == "" {
					firstIdentifier = child.Content(source)
				}
			case "relational_operator":
				sawRelOp = true
			case "character_literal":
				if firstValue == "" {
					firstValue = child.Content(source)
				}
			case "indexed_name":
				// If the FIRST condition is a clock edge, not a reset
				content := strings.ToLower(child.Content(source))
				if strings.Contains(content, "rising_edge") || strings.Contains(content, "falling_edge") {
					sawClockEdgeFirst = true
				}
			}
		}
	}

	// If the first condition is a simple comparison (not a clock edge), it's likely reset
	if firstIdentifier != "" && sawRelOp && firstValue != "" && !sawClockEdgeFirst {
		if !proc.HasReset {
			proc.HasReset = true
			proc.ResetSignal = firstIdentifier
			// Async reset pattern: reset check is FIRST (before clock edge in elsif)
			proc.ResetAsync = true
		}
	}
}

func (e *Extractor) extractSensitivityList(node *sitter.Node, source []byte) []string {
	var signals []string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		childType := child.Type()

		switch childType {
		case "identifier":
			signals = append(signals, child.Content(source))
		case "selected_name", "indexed_name":
			// For complex signal names like rec.field or arr(i)
			signals = append(signals, child.Content(source))
		}
	}

	// Check for VHDL-2008 "all" - represented as the raw content
	content := strings.TrimSpace(node.Content(source))
	if strings.ToLower(content) == "all" {
		return []string{"all"}
	}

	return signals
}

// extractCaseStatement extracts a case statement for latch detection analysis
func (e *Extractor) extractCaseStatement(node *sitter.Node, source []byte, archContext, processContext string) CaseStatement {
	caseStmt := CaseStatement{
		Line:      int(node.StartPoint().Row) + 1,
		InArch:    archContext,
		InProcess: processContext,
		Choices:   []string{},
	}

	// Extract the case expression
	if exprNode := node.ChildByFieldName("expression"); exprNode != nil {
		caseStmt.Expression = exprNode.Content(source)
	}

	// Walk children to find case_alternative nodes
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "case_alternative" {
			// Extract choices from this alternative
			for j := 0; j < int(child.ChildCount()); j++ {
				choiceChild := child.Child(j)
				if choiceChild.Type() == "case_choice" {
					// Check if this choice contains others_choice
					isOthers := false
					for k := 0; k < int(choiceChild.ChildCount()); k++ {
						if choiceChild.Child(k).Type() == "others_choice" {
							caseStmt.HasOthers = true
							caseStmt.Choices = append(caseStmt.Choices, "others")
							isOthers = true
							break
						}
					}
					// If not others, extract the choice value
					if !isOthers {
						choiceText := choiceChild.Content(source)
						caseStmt.Choices = append(caseStmt.Choices, choiceText)
					}
				}
			}
		}
	}

	// IsComplete if it has "others" (conservative check - full coverage analysis would need type info)
	caseStmt.IsComplete = caseStmt.HasOthers

	return caseStmt
}

// extractCaseStatementsFromProcess walks a process body to find all case statements
func (e *Extractor) extractCaseStatementsFromProcess(node *sitter.Node, source []byte, archContext, processLabel string, facts *FileFacts) {
	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}
		if n.Type() == "case_statement" {
			caseStmt := e.extractCaseStatement(n, source, archContext, processLabel)
			facts.CaseStatements = append(facts.CaseStatements, caseStmt)
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(node)
}

// extractComparisonsFromProcess extracts comparison operations for trojan/trigger detection
// Strategy: Look for relational_operator nodes and extract their sibling operands
// Note: When the grammar produces proper relational_expression nodes with fields,
// this extraction becomes trivial. Until then, we work with what we have.
func (e *Extractor) extractComparisonsFromProcess(node *sitter.Node, source []byte, archContext, processLabel string, facts *FileFacts) {
	var walk func(n *sitter.Node, currentAssignment string, parent *sitter.Node)
	walk = func(n *sitter.Node, currentAssignment string, parent *sitter.Node) {
		if n == nil {
			return
		}

		nodeType := n.Type()

		// Track what signal is being assigned (for ResultDrives field)
		if nodeType == "sequential_signal_assignment" {
			// First identifier is typically the target
			for i := 0; i < int(n.ChildCount()); i++ {
				child := n.Child(i)
				if child.Type() == "identifier" {
					currentAssignment = child.Content(source)
					break
				}
			}
		}

		// First try: structured relational_expression node (preferred if grammar provides it)
		if nodeType == "relational_expression" {
			comp := e.extractComparisonStructured(n, source, archContext, processLabel, currentAssignment)
			if comp.LeftOperand != "" {
				facts.Comparisons = append(facts.Comparisons, comp)
			}
		}

		// Fallback: Find relational_operator nodes and look at siblings
		// This handles the flat structure: parent(identifier, relational_operator, identifier)
		if nodeType == "relational_operator" && parent != nil {
			comp := e.extractComparisonFromSiblings(n, parent, source, archContext, processLabel, currentAssignment)
			if comp.LeftOperand != "" && comp.Operator != "" {
				facts.Comparisons = append(facts.Comparisons, comp)
			}
		}

		// Recurse into children
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i), currentAssignment, n)
		}
	}
	walk(node, "", nil)
}

// extractComparisonStructured extracts from a structured relational_expression node
func (e *Extractor) extractComparisonStructured(node *sitter.Node, source []byte, archContext, processLabel, resultDrives string) Comparison {
	comp := Comparison{
		Line:         int(node.StartPoint().Row) + 1,
		InProcess:    processLabel,
		InArch:       archContext,
		ResultDrives: resultDrives,
	}

	// Use grammar fields for clean extraction
	if leftNode := node.ChildByFieldName("left"); leftNode != nil {
		comp.LeftOperand = e.extractExpressionSignal(leftNode, source)
	}

	if opNode := node.ChildByFieldName("operator"); opNode != nil {
		comp.Operator = opNode.Content(source)
	}

	if rightNode := node.ChildByFieldName("right"); rightNode != nil {
		comp.RightOperand = e.extractExpressionSignal(rightNode, source)
		// Check if right side is a literal
		if isLiteralNode(rightNode) {
			comp.IsLiteral = true
			comp.LiteralValue = rightNode.Content(source)
			comp.LiteralBits = estimateBitWidth(comp.LiteralValue)
		}
	}

	return comp
}

// extractComparisonFromSiblings extracts a comparison by looking at siblings of relational_operator
// Handles flat structure: parent(left_operand, relational_operator, right_operand)
func (e *Extractor) extractComparisonFromSiblings(opNode, parent *sitter.Node, source []byte, archContext, processLabel, resultDrives string) Comparison {
	comp := Comparison{
		Line:         int(opNode.StartPoint().Row) + 1,
		InProcess:    processLabel,
		InArch:       archContext,
		ResultDrives: resultDrives,
		Operator:     opNode.Content(source),
	}

	// Find the operator's position among siblings
	opIndex := -1
	for i := 0; i < int(parent.ChildCount()); i++ {
		if parent.Child(i) == opNode {
			opIndex = i
			break
		}
	}

	if opIndex < 0 {
		return comp
	}

	// Look backwards for left operand (skip non-value nodes)
	for i := opIndex - 1; i >= 0; i-- {
		child := parent.Child(i)
		if isValueNode(child) {
			comp.LeftOperand = e.extractExpressionSignal(child, source)
			break
		}
	}

	// Look forwards for right operand
	for i := opIndex + 1; i < int(parent.ChildCount()); i++ {
		child := parent.Child(i)
		if isValueNode(child) {
			comp.RightOperand = e.extractExpressionSignal(child, source)
			// Check for literal (hex string, number, etc.)
			childContent := child.Content(source)
			if isLiteralContent(childContent) || isLiteralNode(child) {
				comp.IsLiteral = true
				comp.LiteralValue = childContent
				comp.LiteralBits = estimateBitWidth(childContent)
			}
			break
		}
	}

	return comp
}

// isValueNode checks if a node represents a value (operand in an expression)
func isValueNode(node *sitter.Node) bool {
	if node == nil {
		return false
	}
	valueTypes := map[string]bool{
		"identifier":         true,
		"number":             true,
		"character_literal":  true,
		"bit_string_literal": true,
		"string_literal":     true,
		"indexed_name":       true,
		"selected_name":      true,
		"attribute_name":     true,
	}
	return valueTypes[node.Type()]
}

// isLiteralContent checks if content looks like a literal value
func isLiteralContent(content string) bool {
	if len(content) == 0 {
		return false
	}
	// Hex/binary/octal string literals: X"...", B"...", O"..."
	if len(content) >= 3 {
		prefix := strings.ToUpper(string(content[0]))
		if (prefix == "X" || prefix == "B" || prefix == "O") && content[1] == '"' {
			return true
		}
	}
	// Numbers
	if content[0] >= '0' && content[0] <= '9' {
		return true
	}
	// Character literals: 'x'
	if content[0] == '\'' && len(content) >= 2 {
		return true
	}
	// String literals: "..."
	if content[0] == '"' {
		return true
	}
	return false
}

// extractExpressionSignal extracts the primary signal/identifier from an expression node
func (e *Extractor) extractExpressionSignal(node *sitter.Node, source []byte) string {
	if node == nil {
		return ""
	}

	nodeType := node.Type()

	// Direct identifier
	if nodeType == "identifier" {
		return node.Content(source)
	}

	// For indexed/selected names, get the base signal
	if nodeType == "indexed_name" || nodeType == "selected_name" {
		return e.extractBaseSignal(node, source)
	}

	// For literals, return the content
	if nodeType == "number" || nodeType == "character_literal" || strings.HasSuffix(nodeType, "_literal") {
		return node.Content(source)
	}

	// Walk children to find first identifier
	for i := 0; i < int(node.ChildCount()); i++ {
		result := e.extractExpressionSignal(node.Child(i), source)
		if result != "" {
			return result
		}
	}

	return ""
}

// isLiteralNode checks if a node represents a literal value
// Note: bit_string_literal is handled by the external scanner (src/scanner.c)
// so it appears as a proper node type, no need to check content
func isLiteralNode(node *sitter.Node) bool {
	if node == nil {
		return false
	}

	nodeType := node.Type()

	// Direct literal types (bit_string_literal from external scanner)
	literalTypes := []string{"number", "character_literal", "bit_string_literal", "string_literal"}
	for _, lt := range literalTypes {
		if nodeType == lt {
			return true
		}
	}

	// Check first child (for wrapped literals)
	if node.ChildCount() > 0 {
		return isLiteralNode(node.Child(0))
	}

	return false
}

// estimateBitWidth estimates the bit width of a literal value
func estimateBitWidth(literal string) int {
	// Handle VHDL bit string literals: X"FFFF", B"1010", O"777"
	if len(literal) >= 3 {
		prefix := strings.ToUpper(string(literal[0]))
		if (prefix == "X" || prefix == "B" || prefix == "O") && literal[1] == '"' {
			// Remove prefix, quotes
			content := strings.Trim(literal[2:], "\"")
			switch prefix {
			case "X":
				return len(content) * 4 // Each hex digit = 4 bits
			case "B":
				return len(content) // Each binary digit = 1 bit
			case "O":
				return len(content) * 3 // Each octal digit = 3 bits
			}
		}
	}

	// Handle decimal numbers
	if len(literal) > 0 && (literal[0] >= '0' && literal[0] <= '9') {
		// Rough estimate: number of decimal digits * 3.32 bits
		return len(literal) * 4
	}

	return 0
}

// extractArithmeticOpsFromProcess extracts expensive arithmetic operations for power analysis
// Uses grammar's visible `multiplicative_expression` and `exponential_expression` nodes
func (e *Extractor) extractArithmeticOpsFromProcess(node *sitter.Node, source []byte, archContext, processLabel string, facts *FileFacts) {
	// Track enable signals from if conditions
	var guardStack []string

	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}

		nodeType := n.Type()

		// Track if conditions as potential guards
		if nodeType == "if_statement" {
			// Extract first identifier from condition as potential guard
			for i := 0; i < int(n.ChildCount()); i++ {
				child := n.Child(i)
				if child.Type() == "identifier" {
					guardStack = append(guardStack, child.Content(source))
					break
				}
			}
		}

		// Grammar provides structured nodes for expensive operations:
		// - multiplicative_expression: field('left'), field('operator'), field('right')
		// - exponential_expression: field('base'), '**', field('exponent')
		if nodeType == "multiplicative_expression" {
			op := e.extractMultiplicativeOp(n, source, archContext, processLabel, guardStack)
			if op.Operator != "" {
				facts.ArithmeticOps = append(facts.ArithmeticOps, op)
			}
		}

		if nodeType == "exponential_expression" {
			op := e.extractExponentialOp(n, source, archContext, processLabel, guardStack)
			if op.Operator != "" {
				facts.ArithmeticOps = append(facts.ArithmeticOps, op)
			}
		}

		// Recurse into children
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}

		// Pop guard when exiting if statement
		if nodeType == "if_statement" && len(guardStack) > 0 {
			guardStack = guardStack[:len(guardStack)-1]
		}
	}
	walk(node)
}

// extractMultiplicativeOp extracts a multiplicative operation (*, /, mod, rem)
// Grammar structure: multiplicative_expression { left: expr, operator: multiplicative_operator, right: expr }
func (e *Extractor) extractMultiplicativeOp(node *sitter.Node, source []byte, archContext, processLabel string, guards []string) ArithmeticOp {
	op := ArithmeticOp{
		Line:      int(node.StartPoint().Row) + 1,
		InProcess: processLabel,
		InArch:    archContext,
		Operands:  []string{},
	}

	// Set guard if we're inside a conditional
	if len(guards) > 0 {
		op.IsGuarded = true
		op.GuardSignal = guards[len(guards)-1]
	}

	// Use grammar fields for clean extraction
	if leftNode := node.ChildByFieldName("left"); leftNode != nil {
		if sig := e.extractExpressionSignal(leftNode, source); sig != "" {
			op.Operands = append(op.Operands, sig)
		}
	}

	if opNode := node.ChildByFieldName("operator"); opNode != nil {
		op.Operator = opNode.Content(source)
	}

	if rightNode := node.ChildByFieldName("right"); rightNode != nil {
		if sig := e.extractExpressionSignal(rightNode, source); sig != "" {
			op.Operands = append(op.Operands, sig)
		}
	}

	return op
}

// extractExponentialOp extracts an exponential operation (**)
// Grammar structure: exponential_expression { base: expr, '**', exponent: expr }
func (e *Extractor) extractExponentialOp(node *sitter.Node, source []byte, archContext, processLabel string, guards []string) ArithmeticOp {
	op := ArithmeticOp{
		Line:      int(node.StartPoint().Row) + 1,
		InProcess: processLabel,
		InArch:    archContext,
		Operator:  "**",
		Operands:  []string{},
	}

	// Set guard if we're inside a conditional
	if len(guards) > 0 {
		op.IsGuarded = true
		op.GuardSignal = guards[len(guards)-1]
	}

	// Use grammar fields for clean extraction
	if baseNode := node.ChildByFieldName("base"); baseNode != nil {
		if sig := e.extractExpressionSignal(baseNode, source); sig != "" {
			op.Operands = append(op.Operands, sig)
		}
	}

	if expNode := node.ChildByFieldName("exponent"); expNode != nil {
		if sig := e.extractExpressionSignal(expNode, source); sig != "" {
			op.Operands = append(op.Operands, sig)
		}
	}

	return op
}

// extractSignalDepsFromProcess extracts signal dependencies for loop detection
func (e *Extractor) extractSignalDepsFromProcess(node *sitter.Node, source []byte, archContext, processLabel string, isSequential bool, facts *FileFacts) {
	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}

		if n.Type() == "sequential_signal_assignment" {
			deps := e.extractSignalDepsFromAssignment(n, source, archContext, processLabel, isSequential)
			facts.SignalDeps = append(facts.SignalDeps, deps...)
		}

		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(node)
}

// extractSignalDepsFromAssignment extracts signal dependencies from an assignment
func (e *Extractor) extractSignalDepsFromAssignment(node *sitter.Node, source []byte, archContext, processLabel string, isSequential bool) []SignalDep {
	var deps []SignalDep
	var target string
	readSet := make(map[string]bool)

	// First pass: find target
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" {
			target = child.Content(source)
			break
		}
		if child.Type() == "selected_name" || child.Type() == "indexed_name" {
			target = e.extractBaseSignal(child, source)
			break
		}
	}

	// Second pass: extract reads (skip the target)
	e.extractReadsFromNode(node, source, readSet, true)

	// Create dependencies
	for source := range readSet {
		deps = append(deps, SignalDep{
			Source:       source,
			Target:       target,
			InProcess:    processLabel,
			IsSequential: isSequential,
			Line:         int(node.StartPoint().Row) + 1,
			InArch:       archContext,
		})
	}

	return deps
}

// extractSignalDepsFromConcurrent extracts signal dependencies from concurrent assignments
func (e *Extractor) extractSignalDepsFromConcurrent(ca ConcurrentAssignment, archContext string) []SignalDep {
	var deps []SignalDep
	for _, source := range ca.ReadSignals {
		deps = append(deps, SignalDep{
			Source:       source,
			Target:       ca.Target,
			InProcess:    "", // Concurrent = no process
			IsSequential: false,
			Line:         ca.Line,
			InArch:       archContext,
		})
	}
	return deps
}

// extractSimple is a fallback regex-based extractor when Tree-sitter isn't available
func (e *Extractor) extractSimple(filePath string, content []byte) (FileFacts, error) {
	facts := FileFacts{File: filePath}

	// Simple line-by-line parsing for basic fact extraction
	// This is a placeholder - real implementation would use regex patterns

	text := string(content)
	lines := splitLines(text)

	for i, line := range lines {
		lineNum := i + 1

		// Very basic pattern matching
		if matches := matchEntity(line); matches != nil {
			facts.Entities = append(facts.Entities, Entity{
				Name: matches[0],
				Line: lineNum,
			})
		}

		if matches := matchArchitecture(line); matches != nil {
			facts.Architectures = append(facts.Architectures, Architecture{
				Name:       matches[0],
				EntityName: matches[1],
				Line:       lineNum,
			})
		}

		if matches := matchPackage(line); matches != nil {
			facts.Packages = append(facts.Packages, Package{
				Name: matches[0],
				Line: lineNum,
			})
		}
	}

	return facts, nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
