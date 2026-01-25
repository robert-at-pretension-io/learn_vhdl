package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/open-policy-agent/opa/rego"
)

// Engine evaluates OPA policies against VHDL facts
type Engine struct {
	queries map[string]rego.PreparedEvalQuery
}

// Violation represents a policy violation
type Violation struct {
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Message  string `json:"message"`
}

// Result contains the evaluation results
type Result struct {
	Violations []Violation
	Summary    Summary
}

// Summary provides aggregate counts
type Summary struct {
	TotalViolations int `json:"total_violations"`
	Errors          int `json:"errors"`
	Warnings        int `json:"warnings"`
	Info            int `json:"info"`
}

// Input is the data structure passed to OPA
type Input struct {
	Entities              []Entity               `json:"entities"`
	Architectures         []Architecture         `json:"architectures"`
	Packages              []Package              `json:"packages"`
	Components            []Component            `json:"components"`
	Signals               []Signal               `json:"signals"`
	Ports                 []Port                 `json:"ports"`
	Dependencies          []Dependency           `json:"dependencies"`
	Symbols               []Symbol               `json:"symbols"`
	Instances             []Instance             `json:"instances"`              // Component/entity instantiations with port maps
	CaseStatements        []CaseStatement        `json:"case_statements"`        // Case statements for latch detection
	Processes             []Process              `json:"processes"`              // Process statements for sensitivity/clock analysis
	ConcurrentAssignments []ConcurrentAssignment `json:"concurrent_assignments"` // Concurrent signal assignments (outside processes)
	Generates             []GenerateStatement    `json:"generates"`              // Generate statements (for/if/case generate)
	// Type system
	Types         []TypeDeclaration        `json:"types"`          // Type declarations (enum, record, array, etc.)
	Subtypes      []SubtypeDeclaration     `json:"subtypes"`       // Subtype declarations
	Functions     []FunctionDeclaration    `json:"functions"`      // Function declarations/bodies
	Procedures    []ProcedureDeclaration   `json:"procedures"`     // Procedure declarations/bodies
	ConstantDecls []ConstantDeclaration    `json:"constant_decls"` // Constant declarations with full info
	// Type system info for filtering false positives (LEGACY - use Types/ConstantDecls instead)
	EnumLiterals []string `json:"enum_literals"` // Enum literals from type declarations
	Constants    []string `json:"constants"`     // Constants from constant declarations (names only)
	// Advanced analysis for security/power/correctness
	Comparisons   []Comparison   `json:"comparisons"`    // Comparisons for trojan/trigger detection
	ArithmeticOps []ArithmeticOp `json:"arithmetic_ops"` // Expensive operations for power analysis
	SignalDeps    []SignalDep    `json:"signal_deps"`    // Signal dependencies for loop detection
	// Configuration for lint rules
	LintConfig LintRuleConfig `json:"lint_config"` // Rule severities and enabled/disabled
	// Third-party file tracking
	ThirdPartyFiles []string `json:"third_party_files"` // Files from third-party libraries (suppress warnings)
}

// LintRuleConfig contains rule configuration passed to OPA
type LintRuleConfig struct {
	Rules map[string]string `json:"rules"` // rule name -> "off", "warning", "error"
}

// Process represents a VHDL process for policy analysis
type Process struct {
	Label           string   `json:"label"`
	SensitivityList []string `json:"sensitivity_list"`
	IsSequential    bool     `json:"is_sequential"`
	IsCombinational bool     `json:"is_combinational"`
	ClockSignal     string   `json:"clock_signal"`
	HasReset        bool     `json:"has_reset"`
	ResetSignal     string   `json:"reset_signal"`
	AssignedSignals []string `json:"assigned_signals"`
	ReadSignals     []string `json:"read_signals"`
	File            string   `json:"file"`
	Line            int      `json:"line"`
	InArch          string   `json:"in_arch"`
}

// Simplified types for OPA input (mirrors extractor types)
type Entity struct {
	Name  string `json:"name"`
	File  string `json:"file"`
	Line  int    `json:"line"`
	Ports []Port `json:"ports"`
}

type Architecture struct {
	Name       string `json:"name"`
	EntityName string `json:"entity_name"`
	File       string `json:"file"`
	Line       int    `json:"line"`
}

type Package struct {
	Name string `json:"name"`
	File string `json:"file"`
	Line int    `json:"line"`
}

type Component struct {
	Name       string `json:"name"`
	EntityRef  string `json:"entity_ref"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	IsInstance bool   `json:"is_instance"`
}

type Signal struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	InEntity string `json:"in_entity"`
}

type Port struct {
	Name      string `json:"name"`
	Direction string `json:"direction"`
	Type      string `json:"type"`
	Line      int    `json:"line"`
	InEntity  string `json:"in_entity"`
}

type Dependency struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	Kind     string `json:"kind"`
	Line     int    `json:"line"`
	Resolved bool   `json:"resolved"`
}

type Symbol struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	File string `json:"file"`
	Line int    `json:"line"`
}

// Instance represents a component/entity instantiation with port/generic mappings
// Enables system-level analysis (cross-module signal tracing, clock mismatch detection)
type Instance struct {
	Name       string            `json:"name"`        // Instance label (e.g., "u_cpu")
	Target     string            `json:"target"`      // Target entity/component (e.g., "work.cpu")
	PortMap    map[string]string `json:"port_map"`    // Formal port -> actual signal
	GenericMap map[string]string `json:"generic_map"` // Formal generic -> actual value
	File       string            `json:"file"`
	Line       int               `json:"line"`
	InArch     string            `json:"in_arch"` // Which architecture contains this instance
}

// CaseStatement represents a VHDL case statement for latch detection
// A case statement without "others" can infer a latch in combinational logic
type CaseStatement struct {
	Expression string   `json:"expression"`  // The case expression (e.g., "state")
	Choices    []string `json:"choices"`     // All explicit choices
	HasOthers  bool     `json:"has_others"`  // true if "when others =>" is present
	File       string   `json:"file"`
	Line       int      `json:"line"`
	InProcess  string   `json:"in_process"`  // Which process contains this case statement
	InArch     string   `json:"in_arch"`     // Which architecture
	IsComplete bool     `json:"is_complete"` // true if HasOthers or all values covered
}

// ConcurrentAssignment represents a concurrent signal assignment (outside processes)
// Enables detection of undriven/multi-driven signals that were previously missed
type ConcurrentAssignment struct {
	Target      string   `json:"target"`       // Signal being assigned (LHS)
	ReadSignals []string `json:"read_signals"` // Signals being read (RHS)
	File        string   `json:"file"`
	Line        int      `json:"line"`
	InArch      string   `json:"in_arch"` // Which architecture contains this assignment
	Kind        string   `json:"kind"`    // "simple", "conditional", "selected"
}

// Comparison represents a comparison operation for trojan/trigger detection
// Tracks comparisons against literals, especially large "magic" values
type Comparison struct {
	LeftOperand  string `json:"left_operand"`  // Signal or expression on left
	Operator     string `json:"operator"`      // =, /=, <, >, <=, >=
	RightOperand string `json:"right_operand"` // Signal, literal, or expression on right
	IsLiteral    bool   `json:"is_literal"`    // True if right operand is a literal
	LiteralValue string `json:"literal_value"` // The literal value if IsLiteral
	LiteralBits  int    `json:"literal_bits"`  // Estimated bit width of literal
	ResultDrives string `json:"result_drives"` // What signal does this comparison drive
	File         string `json:"file"`
	Line         int    `json:"line"`
	InProcess    string `json:"in_process"`
	InArch       string `json:"in_arch"`
}

// ArithmeticOp represents an expensive arithmetic operation for power analysis
type ArithmeticOp struct {
	Operator    string   `json:"operator"`     // *, /, mod, rem, **
	Operands    []string `json:"operands"`     // Input signals/expressions
	Result      string   `json:"result"`       // Output signal
	IsGuarded   bool     `json:"is_guarded"`   // True if inputs are gated by enable
	GuardSignal string   `json:"guard_signal"` // The enable/valid signal if guarded
	File        string   `json:"file"`
	Line        int      `json:"line"`
	InProcess   string   `json:"in_process"`
	InArch      string   `json:"in_arch"`
}

// SignalDep represents a signal dependency for combinational loop detection
type SignalDep struct {
	Source       string `json:"source"`        // Signal being read
	Target       string `json:"target"`        // Signal being assigned
	InProcess    string `json:"in_process"`    // Which process (empty if concurrent)
	IsSequential bool   `json:"is_sequential"` // True if crosses a clock boundary
	File         string `json:"file"`
	Line         int    `json:"line"`
	InArch       string `json:"in_arch"`
}

// GenerateStatement represents a VHDL generate statement
// Generate statements create conditional or iterative scopes with their own declarations
type GenerateStatement struct {
	Label     string `json:"label"`     // Generate block label (required in VHDL)
	Kind      string `json:"kind"`      // "for", "if", "case"
	File      string `json:"file"`
	Line      int    `json:"line"`
	InArch    string `json:"in_arch"`   // Which architecture contains this generate
	// For-generate specific
	LoopVar   string `json:"loop_var,omitempty"`   // Loop variable name (for-generate)
	RangeLow  string `json:"range_low,omitempty"`  // Range low bound (for-generate)
	RangeHigh string `json:"range_high,omitempty"` // Range high bound (for-generate)
	RangeDir  string `json:"range_dir,omitempty"`  // "to" or "downto" (for-generate)
	// Elaboration results (for-generate)
	IterationCount int  `json:"iteration_count"`           // Number of iterations (-1 if cannot evaluate)
	CanElaborate   bool `json:"can_elaborate,omitempty"`   // True if range was successfully evaluated
	// If-generate specific
	Condition string `json:"condition,omitempty"` // Condition expression (if-generate)
	// Nested content counts (actual content is flattened to main lists)
	SignalCount   int `json:"signal_count"`   // Number of signals declared inside
	InstanceCount int `json:"instance_count"` // Number of instances inside
	ProcessCount  int `json:"process_count"`  // Number of processes inside
}

// =============================================================================
// TYPE SYSTEM TYPES
// =============================================================================

// TypeDeclaration represents a VHDL type declaration
type TypeDeclaration struct {
	Name         string        `json:"name"`                     // Type name
	Kind         string        `json:"kind"`                     // "enum", "record", "array", "physical", "access", "file", "incomplete", "protected"
	File         string        `json:"file"`
	Line         int           `json:"line"`
	InPackage    string        `json:"in_package,omitempty"`     // Package containing this type
	InArch       string        `json:"in_arch,omitempty"`        // Architecture if local type
	EnumLiterals []string      `json:"enum_literals,omitempty"`  // For enums
	Fields       []RecordField `json:"fields,omitempty"`         // For records
	ElementType  string        `json:"element_type,omitempty"`   // For arrays
	IndexTypes   []string      `json:"index_types,omitempty"`    // For arrays
	Unconstrained bool         `json:"unconstrained,omitempty"`  // For arrays
	BaseUnit     string        `json:"base_unit,omitempty"`      // For physical types
	RangeLow     string        `json:"range_low,omitempty"`      // For range types
	RangeHigh    string        `json:"range_high,omitempty"`     // For range types
	RangeDir     string        `json:"range_dir,omitempty"`      // "to" or "downto"
}

// RecordField represents a field in a record type
type RecordField struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Line int    `json:"line"`
}

// SubtypeDeclaration represents a VHDL subtype declaration
type SubtypeDeclaration struct {
	Name       string `json:"name"`
	BaseType   string `json:"base_type"`
	Constraint string `json:"constraint,omitempty"`
	Resolution string `json:"resolution,omitempty"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	InPackage  string `json:"in_package,omitempty"`
	InArch     string `json:"in_arch,omitempty"`
}

// FunctionDeclaration represents a VHDL function declaration or body
type FunctionDeclaration struct {
	Name       string                `json:"name"`
	ReturnType string                `json:"return_type"`
	Parameters []SubprogramParameter `json:"parameters,omitempty"`
	IsPure     bool                  `json:"is_pure"`
	HasBody    bool                  `json:"has_body"`
	File       string                `json:"file"`
	Line       int                   `json:"line"`
	InPackage  string                `json:"in_package,omitempty"`
	InArch     string                `json:"in_arch,omitempty"`
}

// ProcedureDeclaration represents a VHDL procedure declaration or body
type ProcedureDeclaration struct {
	Name       string                `json:"name"`
	Parameters []SubprogramParameter `json:"parameters,omitempty"`
	HasBody    bool                  `json:"has_body"`
	File       string                `json:"file"`
	Line       int                   `json:"line"`
	InPackage  string                `json:"in_package,omitempty"`
	InArch     string                `json:"in_arch,omitempty"`
}

// SubprogramParameter represents a parameter in a function or procedure
type SubprogramParameter struct {
	Name      string `json:"name"`
	Direction string `json:"direction,omitempty"` // "in", "out", "inOut"
	Type      string `json:"type"`
	Class     string `json:"class,omitempty"`   // "signal", "variable", "constant", "file"
	Default   string `json:"default,omitempty"` // Default value expression
	Line      int    `json:"line"`
}

// ConstantDeclaration represents a VHDL constant declaration
type ConstantDeclaration struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Value     string `json:"value,omitempty"`      // May be empty for deferred constants
	File      string `json:"file"`
	Line      int    `json:"line"`
	InPackage string `json:"in_package,omitempty"` // Package containing this constant
	InArch    string `json:"in_arch,omitempty"`    // Architecture if local constant
}

// New creates a new policy engine, loading policies from the given directory
func New(policyDir string) (*Engine, error) {
	engine := &Engine{
		queries: make(map[string]rego.PreparedEvalQuery),
	}

	// Find all .rego files
	files, err := filepath.Glob(filepath.Join(policyDir, "*.rego"))
	if err != nil {
		return nil, fmt.Errorf("finding policy files: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no policy files found in %s", policyDir)
	}

	// Load all policy files
	var modules []func(*rego.Rego)
	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", f, err)
		}
		modules = append(modules, rego.Module(f, string(content)))
	}

	// Prepare query for all_violations
	opts := append(modules, rego.Query("data.vhdl.compliance.all_violations"))
	query, err := rego.New(opts...).PrepareForEval(context.Background())
	if err != nil {
		return nil, fmt.Errorf("preparing violations query: %w", err)
	}
	engine.queries["violations"] = query

	// Prepare query for summary
	opts = append(modules, rego.Query("data.vhdl.compliance.summary"))
	query, err = rego.New(opts...).PrepareForEval(context.Background())
	if err != nil {
		return nil, fmt.Errorf("preparing summary query: %w", err)
	}
	engine.queries["summary"] = query

	return engine, nil
}

// Evaluate runs the policies against the input data
func (e *Engine) Evaluate(input Input) (*Result, error) {
	ctx := context.Background()

	// Convert input to map for OPA
	inputMap, err := structToMap(input)
	if err != nil {
		return nil, fmt.Errorf("converting input: %w", err)
	}

	result := &Result{}

	// Get violations
	rs, err := e.queries["violations"].Eval(ctx, rego.EvalInput(inputMap))
	if err != nil {
		return nil, fmt.Errorf("evaluating violations: %w", err)
	}

	if len(rs) > 0 && len(rs[0].Expressions) > 0 {
		violations, ok := rs[0].Expressions[0].Value.([]interface{})
		if ok {
			for _, v := range violations {
				vmap, ok := v.(map[string]interface{})
				if !ok {
					continue
				}
				violation := Violation{
					Rule:     getString(vmap, "rule"),
					Severity: getString(vmap, "severity"),
					File:     getString(vmap, "file"),
					Line:     getInt(vmap, "line"),
					Message:  getString(vmap, "message"),
				}
				result.Violations = append(result.Violations, violation)
			}
		}
	}

	// Get summary
	rs, err = e.queries["summary"].Eval(ctx, rego.EvalInput(inputMap))
	if err != nil {
		return nil, fmt.Errorf("evaluating summary: %w", err)
	}

	if len(rs) > 0 && len(rs[0].Expressions) > 0 {
		smap, ok := rs[0].Expressions[0].Value.(map[string]interface{})
		if ok {
			result.Summary = Summary{
				TotalViolations: getInt(smap, "total_violations"),
				Errors:          getInt(smap, "errors"),
				Warnings:        getInt(smap, "warnings"),
				Info:            getInt(smap, "info"),
			}
		}
	}

	return result, nil
}

// Helper functions
func structToMap(v interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	return result, err
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getInt(m map[string]interface{}, key string) int {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case float64:
			return int(n)
		case json.Number:
			i, _ := n.Int64()
			return int(i)
		}
	}
	return 0
}
