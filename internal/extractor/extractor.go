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
	"regexp"
	"strconv"
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
	Instances      []Instance           // Component/entity instantiations
	CaseStatements []CaseStatement      // Case statements for latch detection
	Generates      []GenerateStatement  // Generate statements (for-generate, if-generate, case-generate)
	// Type system
	Types         []TypeDeclaration        // Type declarations (enum, record, array, etc.)
	Subtypes      []SubtypeDeclaration     // Subtype declarations
	Functions     []FunctionDeclaration    // Function declarations/bodies
	Procedures    []ProcedureDeclaration   // Procedure declarations/bodies
	ConstantDecls []ConstantDeclaration    // Constant declarations with full info
	// Type system information (for filtering false positives) - LEGACY, use Types/ConstantDecls instead
	EnumLiterals []string          // Enum literals from type declarations (e.g., S_IDLE, S_RUN)
	Constants    []string          // Constants from constant declarations (names only)
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
	CDCCrossings     []CDCCrossing     // Clock domain crossing detection
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
	Signal       string
	IsRead       bool   // Appears on RHS of assignment
	IsWritten    bool   // Appears on LHS of assignment
	InProcess    string // Which process (empty if concurrent)
	InPortMap    bool   // Appears as actual in component port map (may be driven by output)
	InstanceName string // If InPortMap, which instance
	Line         int
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

// CDCCrossing represents a potential clock domain crossing
// Detected when a signal written in one clock domain is read in another
type CDCCrossing struct {
	Signal       string // Signal crossing domains
	SourceClock  string // Clock domain where signal is written
	SourceProc   string // Process that writes the signal
	DestClock    string // Clock domain where signal is read
	DestProc     string // Process that reads the signal
	IsSynchronized bool // True if synchronizer detected
	SyncStages   int    // Number of synchronizer stages (0 if not sync'd)
	IsMultiBit   bool   // True if signal is wider than 1 bit (needs special handling)
	Line         int    // Line of the reading process
	File         string
	InArch       string
}

// GenerateStatement represents a VHDL generate statement
// Generate statements create conditional or iterative scopes with their own declarations
// Types: for-generate (iteration), if-generate (conditional), case-generate (selection)
type GenerateStatement struct {
	Label     string   // Generate block label (required in VHDL)
	Kind      string   // "for", "if", "case"
	Line      int
	InArch    string   // Which architecture contains this generate
	// For-generate specific
	LoopVar   string   // Loop variable name (for-generate)
	RangeLow  string   // Range low bound (for-generate)
	RangeHigh string   // Range high bound (for-generate)
	RangeDir  string   // "to" or "downto" (for-generate)
	// Elaboration results (for-generate)
	IterationCount int  // Number of iterations (-1 if cannot evaluate)
	CanElaborate   bool // True if range was successfully evaluated
	// If-generate specific
	Condition string   // Condition expression (if-generate)
	// Nested declarations (scoped to this generate block)
	Signals               []Signal               // Signals declared inside
	Instances             []Instance             // Component instances inside
	Processes             []Process              // Processes inside
	ConcurrentAssignments []ConcurrentAssignment // Concurrent signal assignments inside
	SignalUsages          []SignalUsage          // Signal reads/writes tracked
	Generates             []GenerateStatement    // Nested generate statements
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

// =============================================================================
// TYPE SYSTEM TYPES
// =============================================================================
// These types enable type-aware analysis: overload resolution, width checking,
// latch detection, and more.

// TypeDeclaration represents a VHDL type declaration
// Captures: type name is (enum_literals) | record ... | array ... | range ...
type TypeDeclaration struct {
	Name       string        // Type name (e.g., "state_t")
	Kind       string        // "enum", "record", "array", "physical", "access", "file", "incomplete", "protected"
	Line       int
	InPackage  string        // Package containing this type (empty if in architecture)
	InArch     string        // Architecture containing this type (empty if in package)
	// Enum-specific
	EnumLiterals []string    // For enums: ["IDLE", "RUN", "STOP"]
	// Record-specific
	Fields []RecordField     // For records: field definitions
	// Array-specific
	ElementType  string      // For arrays: element type
	IndexTypes   []string    // For arrays: index type(s) or range(s)
	Unconstrained bool       // For arrays: true if "range <>"
	// Physical-specific (time, etc.)
	BaseUnit     string      // For physical: base unit name
	// Range-specific
	RangeLow     string      // For integer/real subtypes: low bound
	RangeHigh    string      // For integer/real subtypes: high bound
	RangeDir     string      // "to" or "downto"
}

// RecordField represents a field in a record type
type RecordField struct {
	Name string
	Type string
	Line int
}

// SubtypeDeclaration represents a VHDL subtype declaration
// Captures: subtype name is [resolution] base_type [constraint]
type SubtypeDeclaration struct {
	Name       string
	BaseType   string   // The parent type
	Constraint string   // Range or index constraint (if any)
	Resolution string   // Resolution function (if any)
	Line       int
	InPackage  string
	InArch     string
}

// FunctionDeclaration represents a VHDL function declaration or body
type FunctionDeclaration struct {
	Name       string
	ReturnType string
	Parameters []SubprogramParameter
	IsPure     bool    // true for pure functions (default), false for impure
	HasBody    bool    // true if this is a function body (not just declaration)
	Line       int
	InPackage  string  // Package containing this function
	InArch     string  // Architecture if local function
}

// ProcedureDeclaration represents a VHDL procedure declaration or body
type ProcedureDeclaration struct {
	Name       string
	Parameters []SubprogramParameter
	HasBody    bool    // true if this is a procedure body (not just declaration)
	Line       int
	InPackage  string  // Package containing this procedure
	InArch     string  // Architecture if local procedure
}

// SubprogramParameter represents a parameter in a function or procedure
type SubprogramParameter struct {
	Name      string
	Direction string  // "in", "out", "inout" (empty defaults to "in")
	Type      string
	Class     string  // "signal", "variable", "constant", "file" (empty defaults based on direction)
	Default   string  // Default value expression (if any)
	Line      int
}

// ConstantDeclaration represents a VHDL constant declaration
// Captures: constant name : type := value;
type ConstantDeclaration struct {
	Name      string
	Type      string
	Value     string  // The expression value (may be empty for deferred constants)
	Line      int
	InPackage string  // Package containing this constant
	InArch    string  // Architecture if local constant
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

	// Detect clock domain crossings
	facts.CDCCrossings = DetectCDCCrossings(&facts)

	return facts, nil
}

// walkTree traverses the syntax tree and extracts relevant nodes
// context is the current architecture name (for scoping signals, processes, etc.)
// We also need to track package context separately for type declarations
func (e *Extractor) walkTree(node *sitter.Node, source []byte, facts *FileFacts, context string) {
	e.walkTreeWithPkg(node, source, facts, "", context)
}

// walkTreeWithPkg traverses with both package and architecture context
func (e *Extractor) walkTreeWithPkg(node *sitter.Node, source []byte, facts *FileFacts, pkgContext, archContext string) {
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
		archContext = entity.Name

	case "architecture_body":
		arch := e.extractArchitecture(node, source)
		facts.Architectures = append(facts.Architectures, arch)
		archContext = arch.Name

	case "package_declaration":
		pkg := e.extractPackage(node, source)
		facts.Packages = append(facts.Packages, pkg)
		pkgContext = pkg.Name

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
		inst := e.extractInstance(node, source, archContext)
		facts.Instances = append(facts.Instances, inst)
		// Track signals used in port maps - they may be driven by component outputs
		for _, actual := range inst.PortMap {
			if actual != "" && actual != "open" {
				// Extract base signal name (handle indexed like sig(0))
				baseSig := extractBaseSignalName(actual)
				if baseSig != "" {
					facts.SignalUsages = append(facts.SignalUsages, SignalUsage{
						Signal:       baseSig,
						InPortMap:    true,
						InstanceName: inst.Name,
						Line:         inst.Line,
					})
				}
			}
		}

	case "signal_declaration":
		signals := e.extractSignals(node, source, archContext)
		facts.Signals = append(facts.Signals, signals...)

	case "type_declaration":
		// Extract full type declaration
		td := e.extractTypeDeclaration(node, source, pkgContext, archContext)
		facts.Types = append(facts.Types, td)
		// Also populate legacy EnumLiterals for backward compatibility
		if td.Kind == "enum" {
			facts.EnumLiterals = append(facts.EnumLiterals, td.EnumLiterals...)
		}

	case "subtype_declaration":
		// Extract subtype declaration
		st := e.extractSubtypeDeclaration(node, source, pkgContext, archContext)
		facts.Subtypes = append(facts.Subtypes, st)

	case "function_declaration":
		// Extract function declaration/body
		fd := e.extractFunctionDeclaration(node, source, pkgContext, archContext)
		facts.Functions = append(facts.Functions, fd)

	case "procedure_declaration":
		// Extract procedure declaration/body
		pd := e.extractProcedureDeclaration(node, source, pkgContext, archContext)
		facts.Procedures = append(facts.Procedures, pd)

	case "constant_declaration":
		// Extract full constant declarations with context
		constDecls := e.extractConstantDeclarations(node, source, pkgContext, archContext)
		facts.ConstantDecls = append(facts.ConstantDecls, constDecls...)
		// Also extract names for legacy filtering
		constNames := e.extractConstantNames(node, source)
		facts.Constants = append(facts.Constants, constNames...)

	case "component_declaration":
		comp := e.extractComponentDecl(node, source)
		facts.Components = append(facts.Components, comp)

	case "signal_assignment":
		// Concurrent signal assignment (outside processes)
		// Note: Sequential assignments inside processes are "sequential_signal_assignment"
		ca := e.extractConcurrentAssignment(node, source, archContext)
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
		deps := e.extractSignalDepsFromConcurrent(ca, archContext)
		facts.SignalDeps = append(facts.SignalDeps, deps...)

	case "process_statement":
		proc := e.extractProcess(node, source, archContext)
		facts.Processes = append(facts.Processes, proc)
		// Extract case statements within the process for latch detection
		e.extractCaseStatementsFromProcess(node, source, archContext, proc.Label, facts)
		// Extract comparisons for trojan/trigger detection
		e.extractComparisonsFromProcess(node, source, archContext, proc.Label, facts)
		// Extract arithmetic operations for power analysis
		e.extractArithmeticOpsFromProcess(node, source, archContext, proc.Label, facts)
		// Extract signal dependencies for loop detection
		e.extractSignalDepsFromProcess(node, source, archContext, proc.Label, proc.IsSequential, facts)

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

	case "generate_statement":
		// Extract generate statement with its nested declarations
		gen := e.extractGenerateStatement(node, source, archContext)
		facts.Generates = append(facts.Generates, gen)
		// Recursively flatten all nested generate contents into facts
		e.flattenGenerateToFacts(&gen, archContext, facts)
		// Don't recurse manually - extractGenerateStatement handles nested content
		return
	}

	// Recurse into children
	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkTreeWithPkg(node.Child(i), source, facts, pkgContext, archContext)
	}
}

// flattenGenerateToFacts recursively extracts all contents from a generate statement
// (and its nested generates) into the main facts structure. This ensures that signals,
// instances, processes, and signal usages inside generate blocks are visible to policies.
func (e *Extractor) flattenGenerateToFacts(gen *GenerateStatement, archContext string, facts *FileFacts) {
	scope := archContext + "." + gen.Label

	// Add signals with generate scope
	for _, sig := range gen.Signals {
		sig.InEntity = scope
		facts.Signals = append(facts.Signals, sig)
	}

	// Add instances with generate scope and track port map signals
	for _, inst := range gen.Instances {
		inst.InArch = scope
		facts.Instances = append(facts.Instances, inst)
		// Track signals used in port maps
		for _, actual := range inst.PortMap {
			if actual != "" && actual != "open" {
				baseSig := extractBaseSignalName(actual)
				if baseSig != "" {
					facts.SignalUsages = append(facts.SignalUsages, SignalUsage{
						Signal:       baseSig,
						InPortMap:    true,
						InstanceName: inst.Name,
						Line:         inst.Line,
					})
				}
			}
		}
	}

	// Add processes with generate scope
	for _, proc := range gen.Processes {
		proc.InArch = scope
		facts.Processes = append(facts.Processes, proc)
	}

	// Add concurrent assignments with generate scope
	for _, ca := range gen.ConcurrentAssignments {
		ca.InArch = scope
		facts.ConcurrentAssignments = append(facts.ConcurrentAssignments, ca)
	}

	// Add signal usages
	facts.SignalUsages = append(facts.SignalUsages, gen.SignalUsages...)

	// Recursively process nested generates
	for i := range gen.Generates {
		e.flattenGenerateToFacts(&gen.Generates[i], scope, facts)
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
	// Structure: identifier "=>" (identifier | indexed_name | expression | "open")
	// Handle complex actuals like cpu_trace(i), sig(7 downto 0), etc.
	sawArrow := false
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		childType := child.Type()

		if childType == "=>" {
			sawArrow = true
			continue
		}

		// For formal (before =>), usually just an identifier
		if !sawArrow {
			if childType == "identifier" {
				formal = child.Content(source)
			}
			continue
		}

		// For actual (after =>), capture the full content including indexed_name, selected_name, etc.
		// This ensures cpu_trace(i) is captured as "cpu_trace(i)" for signal usage tracking
		switch childType {
		case "identifier", "number", "character_literal":
			actual = child.Content(source)
		case "indexed_name", "selected_name", "attribute_name":
			// Get full content for complex expressions
			actual = child.Content(source)
		case "open":
			actual = "open"
		default:
			// For any other expression type, get the full content
			// This handles things like (others => '0'), type conversions, etc.
			if actual == "" && childType != "(" && childType != ")" && childType != "," {
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
	// VHDL selected assignment: "with expr select target <= value when choice, ..."
	// VHDL conditional assignment: "target <= value when condition else other"
	content := strings.ToLower(node.Content(source))
	// Selected assignments must start with "with" keyword (not just contain "select" in a signal name)
	isSelected := strings.HasPrefix(strings.TrimSpace(content), "with ") && strings.Contains(content, " select ")
	if strings.Contains(content, " when ") && strings.Contains(content, " else ") && !isSelected {
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
		} else if childType == "selected_name" || childType == "indexed_name" {
			// Handle record field access (e.g., rec.field) or array access (e.g., arr(i))
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

// extractBaseSignal extracts the base signal from selected_name or indexed_name
// For "rec.field" returns "rec"
// For "arr(i)" returns "arr"
// For "a.b.c" returns "a"
func (e *Extractor) extractBaseSignal(node *sitter.Node, source []byte) string {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" {
			return child.Content(source)
		}
		if child.Type() == "selected_name" || child.Type() == "indexed_name" {
			return e.extractBaseSignal(child, source)
		}
	}
	return ""
}

// extractEnumLiterals extracts enum literal names from a type_declaration with enumeration_type_definition
// Example: type state_t is (S_IDLE, S_RUN, S_STOP); -> returns ["S_IDLE", "S_RUN", "S_STOP"]
func (e *Extractor) extractEnumLiterals(node *sitter.Node, source []byte) []string {
	var literals []string

	// Find enumeration_type_definition child
	var walkForEnum func(n *sitter.Node)
	walkForEnum = func(n *sitter.Node) {
		if n.Type() == "enumeration_type_definition" {
			// Extract all identifier children as enum literals
			for i := 0; i < int(n.ChildCount()); i++ {
				child := n.Child(i)
				if child.Type() == "identifier" {
					literals = append(literals, child.Content(source))
				}
				// Also handle character literals in enums (like std_ulogic: '0', '1', etc.)
				if child.Type() == "character_literal" {
					literals = append(literals, child.Content(source))
				}
			}
			return
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walkForEnum(n.Child(i))
		}
	}
	walkForEnum(node)

	return literals
}

// extractConstantNames extracts constant names from a constant_declaration
// Example: constant WIDTH : integer := 8; -> returns ["WIDTH"]
func (e *Extractor) extractConstantNames(node *sitter.Node, source []byte) []string {
	var names []string

	// Constant declaration structure: constant name1, name2 : type := value;
	sawColon := false
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		childType := child.Type()

		if childType == ":" {
			sawColon = true
			continue
		}

		// Names come before the colon
		if !sawColon && childType == "identifier" {
			names = append(names, child.Content(source))
		}
	}

	return names
}

// extractConstantDeclarations extracts full constant declarations with type and context
// Example: constant WIDTH : integer := 8; -> returns [{Name: "WIDTH", Type: "integer", Value: "8"}]
func (e *Extractor) extractConstantDeclarations(node *sitter.Node, source []byte, pkgContext, archContext string) []ConstantDeclaration {
	var decls []ConstantDeclaration
	line := int(node.StartPoint().Row) + 1

	// Constant declaration structure: constant name1, name2 : type := value;
	var names []string
	var typeStr string
	var valueStr string

	sawColon := false
	sawAssign := false
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		childType := child.Type()

		if childType == ":" {
			sawColon = true
			continue
		}
		if childType == ":=" {
			sawAssign = true
			continue
		}

		// Names come before the colon
		if !sawColon && childType == "identifier" {
			names = append(names, child.Content(source))
		}

		// Type comes after colon, before :=
		if sawColon && !sawAssign {
			// The type is usually in a subtype_indication or identifier
			if typeStr == "" {
				typeStr = e.extractTypeName(child, source)
			}
		}

		// Value comes after :=
		if sawAssign && valueStr == "" {
			valueStr = strings.TrimSpace(child.Content(source))
		}
	}

	// Create a declaration for each name
	for _, name := range names {
		decls = append(decls, ConstantDeclaration{
			Name:      name,
			Type:      typeStr,
			Value:     valueStr,
			Line:      line,
			InPackage: pkgContext,
			InArch:    archContext,
		})
	}

	return decls
}

// extractTypeName extracts a type name from a type indication node
func (e *Extractor) extractTypeName(node *sitter.Node, source []byte) string {
	if node == nil {
		return ""
	}

	nodeType := node.Type()

	// Direct identifier
	if nodeType == "identifier" || nodeType == "simple_name" {
		return node.Content(source)
	}

	// Selected name (e.g., ieee.std_logic_1164.std_logic)
	if nodeType == "selected_name" {
		return node.Content(source)
	}

	// Type mark or subtype indication - recurse to find the actual type name
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		childType := child.Type()
		if childType == "identifier" || childType == "simple_name" || childType == "selected_name" {
			return child.Content(source)
		}
		// Recurse into type_mark, subtype_indication, etc.
		if childType == "type_mark" || childType == "subtype_indication" || childType == "_type_mark" {
			result := e.extractTypeName(child, source)
			if result != "" {
				return result
			}
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
			// LHS can be identifier, selected_name (record.field), or indexed_name (arr(i))
			for i := 0; i < int(n.ChildCount()); i++ {
				child := n.Child(i)
				childType := child.Type()
				// Stop at the assignment operator - everything after is RHS
				if childType == "<=" || childType == ":=" {
					break
				}
				if childType == "identifier" {
					// Simple signal assignment
					sig := child.Content(source)
					assignedSet[sig] = true
					break
				}
				if childType == "selected_name" || childType == "indexed_name" {
					// Record field or array element assignment
					// Extract the base signal (first identifier in the chain)
					sig := e.extractBaseSignal(child, source)
					if sig != "" {
						assignedSet[sig] = true
					}
					break
				}
			}
			// Walk RHS for reads
			e.extractReadsFromNode(n, source, readSet, true)

		case "assignment_statement":
			// Variable/generic assignments (tmp := expr) don't assign signals,
			// but the RHS may read signals that we need to track
			e.extractReadsFromNode(n, source, readSet, true)

		case "if_statement":
			// Check for clock edge pattern in elsif clause
			// Grammar parses "elsif rising_edge(clk)" with prefix/content fields
			// NOTE: ChildByFieldName returns the first match, but there may be many
			// prefix/content fields from record assignments (ctrl.enable, etc.)
			// We need to look for rising_edge/falling_edge specifically
			if proc.ClockSignal == "" {
				// Iterate through children looking for clock edge pattern
				for i := 0; i < int(n.ChildCount()); i++ {
					child := n.Child(i)
					if child.Type() == "identifier" {
						funcName := strings.ToLower(child.Content(source))
						if funcName == "rising_edge" || funcName == "falling_edge" {
							// Found clock edge function, look for next identifier (clock signal)
							for j := i + 1; j < int(n.ChildCount()); j++ {
								nextChild := n.Child(j)
								if nextChild.Type() == "identifier" {
									argName := nextChild.Content(source)
									if funcName == "rising_edge" {
										proc.ClockSignal = argName
										proc.ClockEdge = "rising"
									} else {
										proc.ClockSignal = argName
										proc.ClockEdge = "falling"
									}
									break
								}
								// Skip non-identifier nodes but stop at statements
								if strings.Contains(nextChild.Type(), "statement") ||
									strings.Contains(nextChild.Type(), "assignment") {
									break
								}
							}
							break
						}
					}
				}
			}
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
			childType == "assignment_statement" {
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

		// For selected_name (e.g., a_req_i.stb) or indexed_name (e.g., arr(i)),
		// only extract the first identifier (base signal)
		// The field name (.stb) or index (i) are not signals - the base signal is
		if n.Type() == "selected_name" || n.Type() == "indexed_name" {
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
				// If the first child is another selected_name or indexed_name, recurse
				if child.Type() == "selected_name" || child.Type() == "indexed_name" {
					walk(child)
					break
				}
			}
			return // Don't recurse into children - we've handled it
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

// extractGenerateStatement extracts a generate statement with its nested declarations
// Handles for-generate, if-generate, and case-generate (VHDL-2008)
// Requires grammar to expose visible nodes: for_generate, if_generate, case_generate
func (e *Extractor) extractGenerateStatement(node *sitter.Node, source []byte, context string) GenerateStatement {
	gen := GenerateStatement{
		Line:      int(node.StartPoint().Row) + 1,
		InArch:    context,
		Signals:   []Signal{},
		Instances: []Instance{},
		Processes: []Process{},
		Generates: []GenerateStatement{},
	}

	// Extract label (required for generate statements)
	if labelNode := node.ChildByFieldName("label"); labelNode != nil {
		gen.Label = labelNode.Content(source)
	}

	// Determine the generate kind by walking children
	// Grammar must expose for_generate, if_generate, case_generate as visible nodes
	var bodyNode *sitter.Node
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		childType := child.Type()

		switch childType {
		case "for_generate":
			gen.Kind = "for"
			e.extractForGenerateDetails(child, source, &gen)
			bodyNode = child // The body is inside for_generate

		case "if_generate":
			gen.Kind = "if"
			e.extractIfGenerateDetails(child, source, &gen)
			bodyNode = child // The body is inside if_generate

		case "case_generate":
			gen.Kind = "case"
			// Case generate: case expr generate ... end generate
			// Extract expression using grammar field
			if exprNode := child.ChildByFieldName("expression"); exprNode != nil {
				gen.Condition = exprNode.Content(source)
			}
			bodyNode = child // The body is inside case_generate
		}
	}

	// Extract nested declarations from the generate body
	// Pass the for_generate/if_generate/case_generate node, NOT the generate_statement
	if bodyNode != nil {
		e.extractGenerateBody(bodyNode, source, &gen)
	}

	return gen
}

// extractForGenerateDetails extracts for-generate specific info (loop var, range)
// Uses grammar fields: loop_var, range
func (e *Extractor) extractForGenerateDetails(node *sitter.Node, source []byte, gen *GenerateStatement) {
	// Use grammar field for loop variable
	if loopVarNode := node.ChildByFieldName("loop_var"); loopVarNode != nil {
		gen.LoopVar = loopVarNode.Content(source)
	}

	// Extract range info from the full for_generate content
	// The grammar hides "to"/"downto" keywords, so we parse from text
	// Pattern: for LOOP_VAR in LOW to/downto HIGH generate
	content := node.Content(source)
	e.parseForGenerateRange(content, gen)

	// Initialize elaboration fields
	gen.IterationCount = -1
	gen.CanElaborate = false
}

// parseForGenerateRange extracts low/high/dir from for-generate text
// Handles patterns like: "for i in 0 to 7 generate", "for i in WIDTH-1 downto 0 generate"
func (e *Extractor) parseForGenerateRange(content string, gen *GenerateStatement) {
	contentLower := strings.ToLower(content)

	// Find the "in" keyword to get the start of the range
	inIdx := strings.Index(contentLower, " in ")
	if inIdx == -1 {
		return
	}
	rangeStart := inIdx + 4 // Skip " in "

	// Find "generate" to get the end of the range
	genIdx := strings.Index(contentLower[rangeStart:], " generate")
	if genIdx == -1 {
		genIdx = strings.Index(contentLower[rangeStart:], "\ngenerate")
	}
	if genIdx == -1 {
		return
	}

	// Extract the range expression
	rangeExpr := strings.TrimSpace(content[rangeStart : rangeStart+genIdx])

	// Find "to" or "downto" (case insensitive)
	rangeLower := strings.ToLower(rangeExpr)

	// Check for "downto" first (it contains "to")
	if downtoIdx := strings.Index(rangeLower, " downto "); downtoIdx != -1 {
		gen.RangeLow = strings.TrimSpace(rangeExpr[:downtoIdx])
		gen.RangeHigh = strings.TrimSpace(rangeExpr[downtoIdx+8:])
		gen.RangeDir = "downto"
		return
	}

	// Check for "to"
	if toIdx := strings.Index(rangeLower, " to "); toIdx != -1 {
		gen.RangeLow = strings.TrimSpace(rangeExpr[:toIdx])
		gen.RangeHigh = strings.TrimSpace(rangeExpr[toIdx+4:])
		gen.RangeDir = "to"
		return
	}

	// No direction found - might be an attribute like vec'range
	gen.RangeLow = rangeExpr
}

// extractIfGenerateDetails extracts if-generate specific info (condition)
// Uses grammar field: condition
func (e *Extractor) extractIfGenerateDetails(node *sitter.Node, source []byte, gen *GenerateStatement) {
	// Use grammar field for condition
	if condNode := node.ChildByFieldName("condition"); condNode != nil {
		gen.Condition = condNode.Content(source)
	}
}

// extractGenerateBody extracts signals, instances, and processes from generate body
func (e *Extractor) extractGenerateBody(node *sitter.Node, source []byte, gen *GenerateStatement) {
	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}

		nodeType := n.Type()

		switch nodeType {
		case "signal_declaration":
			signals := e.extractSignals(n, source, gen.Label)
			gen.Signals = append(gen.Signals, signals...)
			return // Don't recurse into signal declaration

		case "component_instantiation":
			inst := e.extractInstance(n, source, gen.Label)
			gen.Instances = append(gen.Instances, inst)
			return // Don't recurse into instance

		case "process_statement":
			proc := e.extractProcess(n, source, gen.Label)
			gen.Processes = append(gen.Processes, proc)
			return // Don't recurse into process

		case "signal_assignment":
			// Concurrent signal assignment inside generate block
			ca := e.extractConcurrentAssignment(n, source, gen.InArch+"."+gen.Label)
			gen.ConcurrentAssignments = append(gen.ConcurrentAssignments, ca)
			// Track signal usages
			gen.SignalUsages = append(gen.SignalUsages, SignalUsage{
				Signal:    ca.Target,
				IsWritten: true,
				InProcess: "", // Empty = concurrent
				Line:      ca.Line,
			})
			for _, sig := range ca.ReadSignals {
				gen.SignalUsages = append(gen.SignalUsages, SignalUsage{
					Signal:    sig,
					IsRead:    true,
					InProcess: "", // Empty = concurrent
					Line:      ca.Line,
				})
			}
			return // Don't recurse into assignment

		case "generate_statement":
			// Nested generate - extract recursively
			nested := e.extractGenerateStatement(n, source, gen.InArch+"."+gen.Label)
			gen.Generates = append(gen.Generates, nested)
			return // Don't recurse - already handled
		}

		// Recurse into children
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(node)
}

// =============================================================================
// TYPE SYSTEM EXTRACTION
// =============================================================================

// extractTypeDeclaration extracts a type declaration (enum, record, array, etc.)
// Grammar: type name is definition;
func (e *Extractor) extractTypeDeclaration(node *sitter.Node, source []byte, pkgContext, archContext string) TypeDeclaration {
	td := TypeDeclaration{
		Line:      int(node.StartPoint().Row) + 1,
		InPackage: pkgContext,
		InArch:    archContext,
	}

	// Extract type name
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		td.Name = nameNode.Content(source)
	}

	// Extract type definition
	if defNode := node.ChildByFieldName("definition"); defNode != nil {
		defType := defNode.Type()

		switch defType {
		case "enumeration_type_definition":
			td.Kind = "enum"
			td.EnumLiterals = e.extractEnumLiteralsFromDef(defNode, source)

		case "record_type_definition":
			td.Kind = "record"
			td.Fields = e.extractRecordFields(defNode, source)

		case "array_type_definition":
			td.Kind = "array"
			e.extractArrayTypeDetails(defNode, source, &td)

		case "physical_type_definition":
			td.Kind = "physical"
			e.extractPhysicalTypeDetails(defNode, source, &td)

		default:
			// Could be a simple type (integer range), access type, file type, etc.
			content := strings.ToLower(defNode.Content(source))
			if strings.HasPrefix(content, "access") {
				td.Kind = "access"
			} else if strings.HasPrefix(content, "file") {
				td.Kind = "file"
			} else if strings.Contains(content, "range") {
				td.Kind = "range"
				// Try to extract range details
				e.extractRangeDetails(defNode, source, &td)
			} else {
				td.Kind = "alias" // Simple type alias
			}
		}
	} else {
		// Incomplete type declaration: type name;
		td.Kind = "incomplete"
	}

	return td
}

// extractEnumLiteralsFromDef extracts enum literals from an enumeration_type_definition
func (e *Extractor) extractEnumLiteralsFromDef(node *sitter.Node, source []byte) []string {
	var literals []string
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "identifier":
			literals = append(literals, child.Content(source))
		case "character_literal":
			// Character enums like '0', '1', 'Z'
			literals = append(literals, child.Content(source))
		}
	}
	return literals
}

// extractRecordFields extracts fields from a record_type_definition
func (e *Extractor) extractRecordFields(node *sitter.Node, source []byte) []RecordField {
	var fields []RecordField

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "element_declaration" {
			// element_declaration: name[, name...] : type;
			fieldLine := int(child.StartPoint().Row) + 1
			var names []string
			var fieldType string
			sawColon := false

			for j := 0; j < int(child.ChildCount()); j++ {
				elem := child.Child(j)
				if elem.Content(source) == ":" {
					sawColon = true
					continue
				}
				if elem.Type() == "identifier" {
					if !sawColon {
						names = append(names, elem.Content(source))
					} else if fieldType == "" {
						fieldType = elem.Content(source)
					}
				}
			}

			// Also check for field name via grammar field
			if nameNode := child.ChildByFieldName("name"); nameNode != nil && len(names) == 0 {
				names = append(names, nameNode.Content(source))
			}
			if typeNode := child.ChildByFieldName("type"); typeNode != nil {
				fieldType = typeNode.Content(source)
			}

			// Create a field for each name
			for _, name := range names {
				fields = append(fields, RecordField{
					Name: name,
					Type: fieldType,
					Line: fieldLine,
				})
			}
		}
	}

	return fields
}

// extractArrayTypeDetails extracts array type information
func (e *Extractor) extractArrayTypeDetails(node *sitter.Node, source []byte, td *TypeDeclaration) {
	// array (index_constraint, ...) of element_type
	content := node.Content(source)

	// Check for unconstrained array (range <>)
	if strings.Contains(content, "<>") {
		td.Unconstrained = true
	}

	// Walk children to find index types and element type
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		childType := child.Type()

		// Index constraints come before 'of' keyword
		if childType == "identifier" {
			// Could be index type or element type
			// Element type comes after 'of'
			if td.ElementType == "" {
				// Check if previous sibling was 'of' keyword
				if i > 0 {
					prev := node.Child(i - 1)
					if strings.ToLower(prev.Content(source)) == "of" {
						td.ElementType = child.Content(source)
						continue
					}
				}
				// Otherwise it's an index type
				td.IndexTypes = append(td.IndexTypes, child.Content(source))
			}
		}
	}

	// If we couldn't extract element type, try getting from content
	if td.ElementType == "" {
		parts := strings.Split(strings.ToLower(content), " of ")
		if len(parts) > 1 {
			// Element type is after "of"
			elemPart := strings.TrimSpace(parts[1])
			elemPart = strings.TrimRight(elemPart, ";")
			td.ElementType = elemPart
		}
	}
}

// extractPhysicalTypeDetails extracts physical type information (time, etc.)
func (e *Extractor) extractPhysicalTypeDetails(node *sitter.Node, source []byte, td *TypeDeclaration) {
	// Physical type: range X to Y units base_unit; secondary_units... end units;
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" && td.BaseUnit == "" {
			td.BaseUnit = child.Content(source)
			break
		}
	}
}

// extractRangeDetails extracts range constraint details
func (e *Extractor) extractRangeDetails(node *sitter.Node, source []byte, td *TypeDeclaration) {
	content := node.Content(source)
	lowerContent := strings.ToLower(content)

	// Look for "to" or "downto"
	if strings.Contains(lowerContent, " downto ") {
		td.RangeDir = "downto"
		parts := strings.Split(lowerContent, " downto ")
		if len(parts) == 2 {
			// Extract the actual values (strip "range" keyword if present)
			low := strings.TrimPrefix(parts[0], "range ")
			td.RangeLow = strings.TrimSpace(low)
			td.RangeHigh = strings.TrimSpace(parts[1])
		}
	} else if strings.Contains(lowerContent, " to ") {
		td.RangeDir = "to"
		parts := strings.Split(lowerContent, " to ")
		if len(parts) == 2 {
			low := strings.TrimPrefix(parts[0], "range ")
			td.RangeLow = strings.TrimSpace(low)
			td.RangeHigh = strings.TrimSpace(parts[1])
		}
	}
}

// extractSubtypeDeclaration extracts a subtype declaration
// Grammar: subtype name is [resolution] type_mark [constraint];
func (e *Extractor) extractSubtypeDeclaration(node *sitter.Node, source []byte, pkgContext, archContext string) SubtypeDeclaration {
	st := SubtypeDeclaration{
		Line:      int(node.StartPoint().Row) + 1,
		InPackage: pkgContext,
		InArch:    archContext,
	}

	// Extract subtype name
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		st.Name = nameNode.Content(source)
	}

	// Extract base type from 'indication' field (grammar: field('indication', $._type_mark))
	if indicationNode := node.ChildByFieldName("indication"); indicationNode != nil {
		st.BaseType = indicationNode.Content(source)
	}

	// Also try 'prefix' field (some grammar variants)
	if st.BaseType == "" {
		if prefixNode := node.ChildByFieldName("prefix"); prefixNode != nil {
			st.BaseType = prefixNode.Content(source)
		}
	}

	// Fallback: walk children to find base type after "is"
	if st.BaseType == "" {
		sawIs := false
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			childContent := child.Content(source)

			if strings.ToLower(childContent) == "is" {
				sawIs = true
				continue
			}

			if sawIs && child.Type() == "identifier" && st.BaseType == "" {
				st.BaseType = childContent
				break
			}
		}
	}

	// Extract resolution function if present
	if resNode := node.ChildByFieldName("resolution"); resNode != nil {
		st.Resolution = resNode.Content(source)
	}

	// Try to extract constraint from content
	content := node.Content(source)
	if idx := strings.Index(strings.ToLower(content), "range"); idx != -1 {
		// Has range constraint
		st.Constraint = strings.TrimSpace(content[idx:])
		st.Constraint = strings.TrimRight(st.Constraint, ";")
	} else if idx := strings.Index(content, "("); idx != -1 {
		// Has index constraint like (0 to 7)
		endIdx := strings.LastIndex(content, ")")
		if endIdx > idx {
			st.Constraint = content[idx : endIdx+1]
		}
	}

	return st
}

// extractFunctionDeclaration extracts a function declaration or body
// Grammar: [pure|impure] function name [parameters] return type [is ... end];
func (e *Extractor) extractFunctionDeclaration(node *sitter.Node, source []byte, pkgContext, archContext string) FunctionDeclaration {
	fd := FunctionDeclaration{
		Line:      int(node.StartPoint().Row) + 1,
		InPackage: pkgContext,
		InArch:    archContext,
		IsPure:    true, // Default is pure
	}

	// Check for pure/impure
	content := strings.ToLower(node.Content(source))
	if strings.HasPrefix(content, "impure") {
		fd.IsPure = false
	}

	// Check if this has a body (contains "is" followed by "begin")
	if strings.Contains(content, " is ") && strings.Contains(content, "begin") {
		fd.HasBody = true
	}

	// Extract function name
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		fd.Name = nameNode.Content(source)
	}

	// Extract return type
	if retNode := node.ChildByFieldName("return_type"); retNode != nil {
		fd.ReturnType = retNode.Content(source)
	}

	// Extract parameters
	fd.Parameters = e.extractSubprogramParameters(node, source)

	return fd
}

// extractProcedureDeclaration extracts a procedure declaration or body
// Grammar: procedure name [parameters] [is ... end];
func (e *Extractor) extractProcedureDeclaration(node *sitter.Node, source []byte, pkgContext, archContext string) ProcedureDeclaration {
	pd := ProcedureDeclaration{
		Line:      int(node.StartPoint().Row) + 1,
		InPackage: pkgContext,
		InArch:    archContext,
	}

	// Check if this has a body
	content := strings.ToLower(node.Content(source))
	if strings.Contains(content, " is ") && strings.Contains(content, "begin") {
		pd.HasBody = true
	}

	// Extract procedure name
	if nameNode := node.ChildByFieldName("name"); nameNode != nil {
		pd.Name = nameNode.Content(source)
	}

	// Extract parameters
	pd.Parameters = e.extractSubprogramParameters(node, source)

	return pd
}

// extractSubprogramParameters extracts parameters from a function/procedure
func (e *Extractor) extractSubprogramParameters(node *sitter.Node, source []byte) []SubprogramParameter {
	var params []SubprogramParameter

	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}

		if n.Type() == "parameter" {
			// Extract parameter details
			paramLine := int(n.StartPoint().Row) + 1
			var names []string
			var paramType string
			var direction string
			var class string
			var defaultVal string
			sawColon := false

			// Check for class (signal/variable/constant)
			if classNode := n.ChildByFieldName("class"); classNode != nil {
				class = strings.ToLower(classNode.Content(source))
			}

			// Check for direction
			if dirNode := n.ChildByFieldName("direction"); dirNode != nil {
				direction = strings.ToLower(dirNode.Content(source))
			}

			// Check for default
			if defNode := n.ChildByFieldName("default"); defNode != nil {
				defaultVal = defNode.Content(source)
			}

			// Walk children for names and type
			for i := 0; i < int(n.ChildCount()); i++ {
				child := n.Child(i)
				childContent := child.Content(source)

				if childContent == ":" {
					sawColon = true
					continue
				}

				if child.Type() == "identifier" {
					if !sawColon {
						names = append(names, childContent)
					} else if paramType == "" {
						paramType = childContent
					}
				}

				// Handle parameter class
				if child.Type() == "parameter_class" {
					class = strings.ToLower(childContent)
				}

				// Handle port_direction
				if child.Type() == "port_direction" {
					direction = strings.ToLower(childContent)
				}
			}

			// Create a parameter for each name
			for _, name := range names {
				params = append(params, SubprogramParameter{
					Name:      name,
					Direction: direction,
					Type:      paramType,
					Class:     class,
					Default:   defaultVal,
					Line:      paramLine,
				})
			}
		}

		// Recurse into children
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(node)

	return params
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

// =============================================================================
// GENERATE ELABORATION
// =============================================================================
// Evaluates for-generate ranges to determine iteration counts.
// This is called after extraction when constants are available.

// ElaborateGenerates evaluates for-generate ranges using available constants
// Returns the number of generates that were successfully elaborated
func ElaborateGenerates(generates []GenerateStatement, constants map[string]int) int {
	count := 0
	for i := range generates {
		if elaborateGenerate(&generates[i], constants) {
			count++
		}
		// Recursively elaborate nested generates
		count += ElaborateGenerates(generates[i].Generates, constants)
	}
	return count
}

// elaborateGenerate evaluates a single for-generate's range
func elaborateGenerate(gen *GenerateStatement, constants map[string]int) bool {
	if gen.Kind != "for" {
		return false
	}

	// Try to evaluate low and high bounds
	low, okLow := evaluateRangeExpr(gen.RangeLow, constants)
	high, okHigh := evaluateRangeExpr(gen.RangeHigh, constants)

	if !okLow || !okHigh {
		gen.IterationCount = -1
		gen.CanElaborate = false
		return false
	}

	// Calculate iteration count based on direction
	switch gen.RangeDir {
	case "to":
		gen.IterationCount = high - low + 1
	case "downto":
		gen.IterationCount = low - high + 1
	default:
		gen.IterationCount = -1
		gen.CanElaborate = false
		return false
	}

	// Sanity check - iteration count should be positive
	if gen.IterationCount < 0 {
		gen.IterationCount = -1
		gen.CanElaborate = false
		return false
	}

	gen.CanElaborate = true
	return true
}

// evaluateRangeExpr evaluates a simple range expression
// Handles: integer literals, identifiers (from constants), simple arithmetic
func evaluateRangeExpr(expr string, constants map[string]int) (int, bool) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return 0, false
	}

	// Try to parse as integer literal
	if val, err := parseIntLiteral(expr); err == nil {
		return val, true
	}

	// Try to look up as constant (case-insensitive)
	if val, ok := constants[strings.ToLower(expr)]; ok {
		return val, true
	}

	// Try to evaluate simple arithmetic expressions
	// Handle: CONST - 1, CONST + 1, CONST * 2, CONST / 2
	return evaluateSimpleArithmetic(expr, constants)
}

// parseIntLiteral parses an integer literal (decimal, hex, binary, octal)
func parseIntLiteral(s string) (int, error) {
	s = strings.TrimSpace(s)

	// Handle VHDL-style based literals: 16#FF#, 2#1010#, 8#77#
	if strings.Contains(s, "#") {
		return parseBasedLiteral(s)
	}

	// Standard Go parsing handles decimal
	var val int
	_, err := fmt.Sscanf(s, "%d", &val)
	return val, err
}

// parseBasedLiteral parses VHDL based literals like 16#FF#, 2#1010#
func parseBasedLiteral(s string) (int, error) {
	parts := strings.Split(s, "#")
	if len(parts) < 2 {
		return 0, fmt.Errorf("invalid based literal: %s", s)
	}

	base, err := fmt.Sscanf(parts[0], "%d", new(int))
	if err != nil || base == 0 {
		return 0, fmt.Errorf("invalid base: %s", parts[0])
	}

	var baseVal int
	fmt.Sscanf(parts[0], "%d", &baseVal)

	// Parse the value part in the given base
	valueStr := strings.ToLower(parts[1])
	var result int
	for _, c := range valueStr {
		var digit int
		if c >= '0' && c <= '9' {
			digit = int(c - '0')
		} else if c >= 'a' && c <= 'f' {
			digit = int(c - 'a' + 10)
		} else {
			continue // Skip underscores
		}
		result = result*baseVal + digit
	}

	return result, nil
}

// evaluateSimpleArithmetic handles simple expressions like "WIDTH - 1"
func evaluateSimpleArithmetic(expr string, constants map[string]int) (int, bool) {
	// Try common patterns: X - N, X + N, X * N, X / N
	operators := []string{" - ", " + ", " * ", " / ", "-", "+", "*", "/"}

	for _, op := range operators {
		if idx := strings.Index(expr, op); idx > 0 {
			left := strings.TrimSpace(expr[:idx])
			right := strings.TrimSpace(expr[idx+len(op):])

			leftVal, okLeft := evaluateRangeExpr(left, constants)
			rightVal, okRight := evaluateRangeExpr(right, constants)

			if okLeft && okRight {
				opChar := strings.TrimSpace(op)
				switch opChar {
				case "-":
					return leftVal - rightVal, true
				case "+":
					return leftVal + rightVal, true
				case "*":
					return leftVal * rightVal, true
				case "/":
					if rightVal != 0 {
						return leftVal / rightVal, true
					}
				}
			}
		}
	}

	return 0, false
}

// BuildConstantMap creates a map of constant names to integer values
// Used for generate elaboration
func BuildConstantMap(constants []ConstantDeclaration) map[string]int {
	result := make(map[string]int)
	for _, c := range constants {
		if val, err := parseIntLiteral(c.Value); err == nil {
			result[strings.ToLower(c.Name)] = val
		}
	}
	return result
}

// DetectCDCCrossings analyzes processes to find signals crossing clock domains
// A CDC crossing occurs when a signal written in one clock domain is read in another
func DetectCDCCrossings(facts *FileFacts) []CDCCrossing {
	var crossings []CDCCrossing

	// Build map: signal -> list of (process, clock) that write it
	type writeInfo struct {
		process string
		clock   string
		arch    string
	}
	signalWriters := make(map[string][]writeInfo)

	// Build map: signal -> bit width estimate (for multi-bit detection)
	signalWidths := make(map[string]int)
	for _, sig := range facts.Signals {
		signalWidths[strings.ToLower(sig.Name)] = estimateSignalWidth(sig.Type)
	}

	// Collect all sequential processes and their writes
	for _, proc := range facts.Processes {
		if !proc.IsSequential || proc.ClockSignal == "" {
			continue // Only care about clocked processes
		}
		for _, sig := range proc.AssignedSignals {
			sigLower := strings.ToLower(sig)
			signalWriters[sigLower] = append(signalWriters[sigLower], writeInfo{
				process: proc.Label,
				clock:   proc.ClockSignal,
				arch:    proc.InArch,
			})
		}
	}

	// Build synchronizer detection map
	// Pattern: signal_meta -> signal_sync (2-stage) or signal_meta1 -> signal_meta2 -> signal_sync (3-stage)
	syncStages := detectSynchronizers(facts.Processes)

	// Check each sequential process for reads from different clock domains
	for _, proc := range facts.Processes {
		if !proc.IsSequential || proc.ClockSignal == "" {
			continue
		}
		destClock := strings.ToLower(proc.ClockSignal)

		for _, readSig := range proc.ReadSignals {
			readLower := strings.ToLower(readSig)
			writers := signalWriters[readLower]

			for _, w := range writers {
				srcClock := strings.ToLower(w.clock)
				// Skip same clock domain
				if srcClock == destClock {
					continue
				}

				// Found a CDC crossing
				crossing := CDCCrossing{
					Signal:      readSig,
					SourceClock: w.clock,
					SourceProc:  w.process,
					DestClock:   proc.ClockSignal,
					DestProc:    proc.Label,
					Line:        proc.Line,
					File:        facts.File,
					InArch:      proc.InArch,
					IsMultiBit:  signalWidths[readLower] > 1,
				}

				// Check if this signal goes through a synchronizer
				if stages, ok := syncStages[readLower]; ok && stages >= 2 {
					crossing.IsSynchronized = true
					crossing.SyncStages = stages
				}

				crossings = append(crossings, crossing)
			}
		}
	}

	return crossings
}

// detectSynchronizers looks for common synchronizer patterns
// Returns map of signal name -> number of synchronizer stages detected
func detectSynchronizers(processes []Process) map[string]int {
	result := make(map[string]int)

	// Look for patterns like:
	// sig_meta <= async_sig;
	// sig_sync <= sig_meta;
	// This indicates async_sig has 2 sync stages

	// Build assignment chains: what does each signal get assigned from?
	signalSource := make(map[string]string) // signal -> its direct source

	for _, proc := range processes {
		if !proc.IsSequential {
			continue
		}
		// If process assigns exactly one signal from one source, track it
		if len(proc.AssignedSignals) == 1 && len(proc.ReadSignals) == 1 {
			assigned := strings.ToLower(proc.AssignedSignals[0])
			read := strings.ToLower(proc.ReadSignals[0])
			signalSource[assigned] = read
		}
	}

	// Trace chains: for each signal, count how many synchronizer stages
	for sig := range signalSource {
		stages := 0
		current := sig
		visited := make(map[string]bool)

		// Walk backwards through the chain
		for {
			source, exists := signalSource[current]
			if !exists || visited[source] {
				break
			}
			visited[current] = true
			stages++
			current = source
		}

		// The original source signal has this many sync stages
		if stages >= 2 && current != sig {
			result[current] = stages
		}
	}

	return result
}

// estimateSignalWidth guesses the bit width from a VHDL type string
func estimateSignalWidth(typeStr string) int {
	typeLower := strings.ToLower(typeStr)

	// Single-bit types
	if typeLower == "std_logic" || typeLower == "std_ulogic" ||
		typeLower == "bit" || typeLower == "boolean" {
		return 1
	}

	// Vector types - try to extract width
	// std_logic_vector(7 downto 0) -> 8 bits
	// std_logic_vector(WIDTH-1 downto 0) -> unknown, assume multi-bit
	if strings.Contains(typeLower, "vector") ||
		strings.Contains(typeLower, "unsigned") ||
		strings.Contains(typeLower, "signed") {
		// Try to find numeric range
		if match := regexp.MustCompile(`(\d+)\s+(?:downto|to)\s+(\d+)`).FindStringSubmatch(typeLower); match != nil {
			high, _ := strconv.Atoi(match[1])
			low, _ := strconv.Atoi(match[2])
			width := high - low
			if width < 0 {
				width = -width
			}
			return width + 1
		}
		// Has a range but couldn't parse - assume multi-bit
		return 8 // Conservative guess
	}

	// Integer types
	if typeLower == "integer" || typeLower == "natural" || typeLower == "positive" {
		return 32
	}

	// Unknown type - assume single bit to avoid false positives
	return 1
}

// extractBaseSignalName extracts the base signal name from an expression
// Examples: "sig" -> "sig", "sig(0)" -> "sig", "sig(7 downto 0)" -> "sig"
// Returns empty string for complex expressions or literals
func extractBaseSignalName(expr string) string {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return ""
	}

	// Skip literals and keywords
	if strings.HasPrefix(expr, "'") || strings.HasPrefix(expr, "\"") {
		return ""
	}
	exprLower := strings.ToLower(expr)
	if exprLower == "open" || exprLower == "others" {
		return ""
	}

	// Handle indexed/sliced: sig(index) or sig(high downto low)
	if parenIdx := strings.Index(expr, "("); parenIdx > 0 {
		return expr[:parenIdx]
	}

	// Handle selected: sig.field - return full path as it may be the signal
	// But if it contains multiple dots, it's likely a qualified name
	if strings.Count(expr, ".") > 1 {
		return ""
	}

	// Simple identifier or record.field
	return expr
}
