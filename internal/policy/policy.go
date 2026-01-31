package policy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Engine evaluates Rust policy rules against VHDL facts
type Engine struct {
	binaryPath string
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
	Violations          []Violation          `json:"violations"`
	Summary             Summary              `json:"summary"`
	MissingChecks       []MissingCheckTask   `json:"missing_checks,omitempty"`
	AmbiguousConstructs []AmbiguousConstruct `json:"ambiguous_constructs,omitempty"`
}

// Summary provides aggregate counts
type Summary struct {
	TotalViolations int `json:"total_violations"`
	Errors          int `json:"errors"`
	Warnings        int `json:"warnings"`
	Info            int `json:"info"`
}

// VerificationAnchor identifies where to insert verification tags.
type VerificationAnchor struct {
	Label     string `json:"label"`
	LineStart int    `json:"line_start"`
	LineEnd   int    `json:"line_end"`
	Exists    bool   `json:"exists"`
}

// MissingCheckTask describes a structured missing verification task.
type MissingCheckTask struct {
	File       string             `json:"file"`
	Scope      string             `json:"scope"`
	Anchor     VerificationAnchor `json:"anchor"`
	MissingIDs []string           `json:"missing_ids"`
	Bindings   map[string]string  `json:"bindings,omitempty"`
	Notes      []string           `json:"notes,omitempty"`
}

// AmbiguousConstruct reports uncertain detection candidates.
type AmbiguousConstruct struct {
	Kind       string              `json:"kind"`
	Scope      string              `json:"scope"`
	File       string              `json:"file"`
	Line       int                 `json:"line"`
	Candidates map[string][]string `json:"candidates,omitempty"`
}

// Input is the data structure passed to the Rust policy engine
type Input struct {
	Standard              string                 `json:"standard"`
	FileCount             int                    `json:"file_count"`
	Entities              []Entity               `json:"entities"`
	Architectures         []Architecture         `json:"architectures"`
	Packages              []Package              `json:"packages"`
	Components            []Component            `json:"components"`
	UseClauses            []UseClause            `json:"use_clauses"`
	LibraryClauses        []LibraryClause        `json:"library_clauses"`
	ContextClauses        []ContextClause        `json:"context_clauses"`
	Signals               []Signal               `json:"signals"`
	Ports                 []Port                 `json:"ports"`
	Dependencies          []Dependency           `json:"dependencies"`
	Symbols               []Symbol               `json:"symbols"`
	Scopes                []Scope                `json:"scopes"`
	SymbolDefs            []SymbolDef            `json:"symbol_defs"`
	NameUses              []NameUse              `json:"name_uses"`
	Files                 []FileInfo             `json:"files"`
	VerificationBlocks    []VerificationBlock    `json:"verification_blocks"`
	VerificationTags      []VerificationTag      `json:"verification_tags"`
	VerificationTagErrors []VerificationTagError `json:"verification_tag_errors"`
	Instances             []Instance             `json:"instances"`              // Component/entity instantiations with port maps
	CaseStatements        []CaseStatement        `json:"case_statements"`        // Case statements for latch detection
	Processes             []Process              `json:"processes"`              // Process statements for sensitivity/clock analysis
	ConcurrentAssignments []ConcurrentAssignment `json:"concurrent_assignments"` // Concurrent signal assignments (outside processes)
	Generates             []GenerateStatement    `json:"generates"`              // Generate statements (for/if/case generate)
	Configurations        []Configuration        `json:"configurations"`         // Configuration declarations
	// Type system
	Types         []TypeDeclaration      `json:"types"`          // Type declarations (enum, record, array, etc.)
	Subtypes      []SubtypeDeclaration   `json:"subtypes"`       // Subtype declarations
	Functions     []FunctionDeclaration  `json:"functions"`      // Function declarations/bodies
	Procedures    []ProcedureDeclaration `json:"procedures"`     // Procedure declarations/bodies
	ConstantDecls []ConstantDeclaration  `json:"constant_decls"` // Constant declarations with full info
	// Type system info for filtering false positives (LEGACY - use Types/ConstantDecls instead)
	EnumLiterals    []string `json:"enum_literals"`    // Enum literals from type declarations
	Constants       []string `json:"constants"`        // Constants from constant declarations (names only)
	SharedVariables []string `json:"shared_variables"` // Shared variable names (not signals)
	// Advanced analysis for security/power/correctness
	Comparisons   []Comparison   `json:"comparisons"`    // Comparisons for trojan/trigger detection
	ArithmeticOps []ArithmeticOp `json:"arithmetic_ops"` // Expensive operations for power analysis
	SignalDeps    []SignalDep    `json:"signal_deps"`    // Signal dependencies for loop detection
	CDCCrossings  []CDCCrossing  `json:"cdc_crossings"`  // Clock domain crossings
	SignalUsages  []SignalUsage  `json:"signal_usages"`  // Signal read/write/port-map tracking
	// Configuration for lint rules
	LintConfig LintRuleConfig `json:"lint_config"` // Rule severities and enabled/disabled
	// Third-party file tracking
	ThirdPartyFiles []string `json:"third_party_files"` // Files from third-party libraries (suppress warnings)
}

// LintRuleConfig contains rule configuration passed to the Rust policy engine
type LintRuleConfig struct {
	Rules map[string]string `json:"rules"` // rule name -> "off", "warning", "error"
}

// Process represents a VHDL process for policy analysis
type Process struct {
	Label           string          `json:"label"`
	SensitivityList []string        `json:"sensitivity_list"`
	IsSequential    bool            `json:"is_sequential"`
	IsCombinational bool            `json:"is_combinational"`
	ClockSignal     string          `json:"clock_signal"`
	ClockEdge       string          `json:"clock_edge"`
	HasReset        bool            `json:"has_reset"`
	ResetSignal     string          `json:"reset_signal"`
	ResetAsync      bool            `json:"reset_async"`
	AssignedSignals []string        `json:"assigned_signals"`
	ReadSignals     []string        `json:"read_signals"`
	Variables       []VariableDecl  `json:"variables"`
	ProcedureCalls  []ProcedureCall `json:"procedure_calls"`
	FunctionCalls   []FunctionCall  `json:"function_calls"`
	WaitStatements  []WaitStatement `json:"wait_statements"`
	File            string          `json:"file"`
	Line            int             `json:"line"`
	InArch          string          `json:"in_arch"`
}

// Simplified types for policy input (mirrors extractor types)
type Entity struct {
	Name     string        `json:"name"`
	File     string        `json:"file"`
	Line     int           `json:"line"`
	Ports    []Port        `json:"ports"`
	Generics []GenericDecl `json:"generics"`
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
	Name       string        `json:"name"`
	EntityRef  string        `json:"entity_ref"`
	File       string        `json:"file"`
	Line       int           `json:"line"`
	IsInstance bool          `json:"is_instance"`
	Ports      []Port        `json:"ports"`
	Generics   []GenericDecl `json:"generics"`
}

type Signal struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	InEntity string `json:"in_entity"`
	Width    int    `json:"width"` // Estimated bit width (0 if unknown)
}

type Port struct {
	Name      string `json:"name"`
	Direction string `json:"direction"`
	Type      string `json:"type"`
	Default   string `json:"default"`
	Line      int    `json:"line"`
	InEntity  string `json:"in_entity"`
	Width     int    `json:"width"` // Estimated bit width (0 if unknown)
}

type GenericDecl struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Type        string `json:"type"`
	Class       string `json:"class"`
	Default     string `json:"default"`
	Line        int    `json:"line"`
	InEntity    string `json:"in_entity"`
	InComponent string `json:"in_component"`
}

type UseClause struct {
	Items []string `json:"items"`
	File  string   `json:"file"`
	Line  int      `json:"line"`
}

type LibraryClause struct {
	Libraries []string `json:"libraries"`
	File      string   `json:"file"`
	Line      int      `json:"line"`
}

type ContextClause struct {
	Name string `json:"name"`
	File string `json:"file"`
	Line int    `json:"line"`
}

type Association struct {
	Kind          string `json:"kind"`
	Formal        string `json:"formal"`
	Actual        string `json:"actual"`
	IsPositional  bool   `json:"is_positional"`
	ActualKind    string `json:"actual_kind"`
	ActualBase    string `json:"actual_base"`
	ActualFull    string `json:"actual_full"`
	Line          int    `json:"line"`
	PositionIndex int    `json:"position_index"`
}

type VariableDecl struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Line int    `json:"line"`
}

type ProcedureCall struct {
	Name      string   `json:"name"`
	FullName  string   `json:"full_name"`
	Args      []string `json:"args"`
	Line      int      `json:"line"`
	InProcess string   `json:"in_process"`
	InArch    string   `json:"in_arch"`
}

type FunctionCall struct {
	Name      string   `json:"name"`
	Args      []string `json:"args"`
	Line      int      `json:"line"`
	InProcess string   `json:"in_process"`
	InArch    string   `json:"in_arch"`
}

type WaitStatement struct {
	OnSignals []string `json:"on_signals"`
	UntilExpr string   `json:"until_expr"`
	ForExpr   string   `json:"for_expr"`
	Line      int      `json:"line"`
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

// FileInfo describes a parsed VHDL file and its library assignment.
type FileInfo struct {
	Path         string `json:"path"`
	Library      string `json:"library"`
	IsThirdParty bool   `json:"is_third_party"`
}

// Scope represents a lexical or generate scope for name resolution.
type Scope struct {
	Name   string   `json:"name"`
	Kind   string   `json:"kind"`
	File   string   `json:"file"`
	Line   int      `json:"line"`
	Parent string   `json:"parent"`
	Path   []string `json:"path"`
}

// SymbolDef represents a symbol definition with scope information.
type SymbolDef struct {
	Name  string `json:"name"`
	Kind  string `json:"kind"`
	File  string `json:"file"`
	Line  int    `json:"line"`
	Scope string `json:"scope"`
}

// NameUse represents a usage of a name in a given scope.
type NameUse struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Scope   string `json:"scope"`
	Context string `json:"context"`
}

// VerificationBlock represents a verification anchor block within an architecture.
type VerificationBlock struct {
	Label     string `json:"label"`
	LineStart int    `json:"line_start"`
	LineEnd   int    `json:"line_end"`
	File      string `json:"file"`
	InArch    string `json:"in_arch"`
}

// VerificationTag represents a parsed --@check tag line.
type VerificationTag struct {
	ID       string            `json:"id"`
	Scope    string            `json:"scope"`
	Bindings map[string]string `json:"bindings"`
	File     string            `json:"file"`
	Line     int               `json:"line"`
	Raw      string            `json:"raw"`
	InArch   string            `json:"in_arch"`
}

// VerificationTagError represents a malformed verification tag line.
type VerificationTagError struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Raw     string `json:"raw"`
	Message string `json:"message"`
	InArch  string `json:"in_arch"`
}

// Instance represents a component/entity instantiation with port/generic mappings
// Enables system-level analysis (cross-module signal tracing, clock mismatch detection)
type Instance struct {
	Name         string            `json:"name"`        // Instance label (e.g., "u_cpu")
	Target       string            `json:"target"`      // Target entity/component (e.g., "work.cpu")
	PortMap      map[string]string `json:"port_map"`    // Formal port -> actual signal
	GenericMap   map[string]string `json:"generic_map"` // Formal generic -> actual value
	Associations []Association     `json:"associations"`
	File         string            `json:"file"`
	Line         int               `json:"line"`
	InArch       string            `json:"in_arch"` // Which architecture contains this instance
}

// CaseStatement represents a VHDL case statement for latch detection
// A case statement without "others" can infer a latch in combinational logic
type CaseStatement struct {
	Expression string   `json:"expression"` // The case expression (e.g., "state")
	Choices    []string `json:"choices"`    // All explicit choices
	HasOthers  bool     `json:"has_others"` // true if "when others =>" is present
	File       string   `json:"file"`
	Line       int      `json:"line"`
	InProcess  string   `json:"in_process"`  // Which process contains this case statement
	InArch     string   `json:"in_arch"`     // Which architecture
	IsComplete bool     `json:"is_complete"` // true if HasOthers or all values covered
}

// ConcurrentAssignment represents a concurrent signal assignment (outside processes)
// Enables detection of undriven/multi-driven signals that were previously missed
type ConcurrentAssignment struct {
	Target        string   `json:"target"`       // Signal being assigned (LHS)
	ReadSignals   []string `json:"read_signals"` // Signals being read (RHS)
	File          string   `json:"file"`
	Line          int      `json:"line"`
	InArch        string   `json:"in_arch"`        // Which architecture contains this assignment
	Kind          string   `json:"kind"`           // "simple", "conditional", "selected"
	InGenerate    bool     `json:"in_generate"`    // True if inside a generate block
	GenerateLabel string   `json:"generate_label"` // Label of containing generate block
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

// SignalUsage represents a signal read/write/port-map usage
// Used to track where signals are used for accurate dead code detection
type SignalUsage struct {
	Signal       string `json:"signal"`
	IsRead       bool   `json:"is_read"`
	IsWritten    bool   `json:"is_written"`
	InProcess    string `json:"in_process"`
	InPortMap    bool   `json:"in_port_map"`   // True if signal is in a component port map
	InstanceName string `json:"instance_name"` // Instance name if InPortMap
	InPSL        bool   `json:"in_psl"`        // True if usage appears in PSL property/sequence/assert
	Line         int    `json:"line"`
}

// CDCCrossing represents a potential clock domain crossing
// Detected when a signal written in one clock domain is read in another
type CDCCrossing struct {
	Signal         string `json:"signal"`          // Signal crossing domains
	SourceClock    string `json:"source_clock"`    // Clock domain where signal is written
	SourceProc     string `json:"source_proc"`     // Process that writes the signal
	DestClock      string `json:"dest_clock"`      // Clock domain where signal is read
	DestProc       string `json:"dest_proc"`       // Process that reads the signal
	IsSynchronized bool   `json:"is_synchronized"` // True if synchronizer detected
	SyncStages     int    `json:"sync_stages"`     // Number of synchronizer stages (0 if not sync'd)
	IsMultiBit     bool   `json:"is_multi_bit"`    // True if signal is wider than 1 bit
	File           string `json:"file"`
	Line           int    `json:"line"`
	InArch         string `json:"in_arch"`
}

// GenerateStatement represents a VHDL generate statement
// Generate statements create conditional or iterative scopes with their own declarations
type GenerateStatement struct {
	Label  string `json:"label"` // Generate block label (required in VHDL)
	Kind   string `json:"kind"`  // "for", "if", "case"
	File   string `json:"file"`
	Line   int    `json:"line"`
	InArch string `json:"in_arch"` // Which architecture contains this generate
	// For-generate specific
	LoopVar   string `json:"loop_var,omitempty"`   // Loop variable name (for-generate)
	RangeLow  string `json:"range_low,omitempty"`  // Range low bound (for-generate)
	RangeHigh string `json:"range_high,omitempty"` // Range high bound (for-generate)
	RangeDir  string `json:"range_dir,omitempty"`  // "to" or "downto" (for-generate)
	// Elaboration results (for-generate)
	IterationCount int  `json:"iteration_count"`         // Number of iterations (-1 if cannot evaluate)
	CanElaborate   bool `json:"can_elaborate,omitempty"` // True if range was successfully evaluated
	// If-generate specific
	Condition string `json:"condition,omitempty"` // Condition expression (if-generate)
	// Nested content counts (actual content is flattened to main lists)
	SignalCount   int `json:"signal_count"`   // Number of signals declared inside
	InstanceCount int `json:"instance_count"` // Number of instances inside
	ProcessCount  int `json:"process_count"`  // Number of processes inside
}

// Configuration represents a VHDL configuration declaration
type Configuration struct {
	Name       string `json:"name"`
	EntityName string `json:"entity_name"`
	File       string `json:"file"`
	Line       int    `json:"line"`
}

// =============================================================================
// TYPE SYSTEM TYPES
// =============================================================================

// TypeDeclaration represents a VHDL type declaration
type TypeDeclaration struct {
	Name          string        `json:"name"` // Type name
	Kind          string        `json:"kind"` // "enum", "record", "array", "physical", "access", "file", "incomplete", "protected"
	File          string        `json:"file"`
	Line          int           `json:"line"`
	InPackage     string        `json:"in_package,omitempty"`    // Package containing this type
	InArch        string        `json:"in_arch,omitempty"`       // Architecture if local type
	EnumLiterals  []string      `json:"enum_literals,omitempty"` // For enums
	Fields        []RecordField `json:"fields,omitempty"`        // For records
	ElementType   string        `json:"element_type,omitempty"`  // For arrays
	IndexTypes    []string      `json:"index_types,omitempty"`   // For arrays
	Unconstrained bool          `json:"unconstrained,omitempty"` // For arrays
	BaseUnit      string        `json:"base_unit,omitempty"`     // For physical types
	RangeLow      string        `json:"range_low,omitempty"`     // For range types
	RangeHigh     string        `json:"range_high,omitempty"`    // For range types
	RangeDir      string        `json:"range_dir,omitempty"`     // "to" or "downto"
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
	Value     string `json:"value,omitempty"` // May be empty for deferred constants
	File      string `json:"file"`
	Line      int    `json:"line"`
	InPackage string `json:"in_package,omitempty"` // Package containing this constant
	InArch    string `json:"in_arch,omitempty"`    // Architecture if local constant
}

// New creates a new policy engine, loading policies from the given directory
func New(policyDir string) (*Engine, error) {
	binaryPath, err := ensurePolicyBinary(policyDir)
	if err != nil {
		return nil, err
	}
	return &Engine{binaryPath: binaryPath}, nil
}

// Evaluate runs the policies against the input data
func (e *Engine) Evaluate(input Input) (*Result, error) {
	ctx := context.Background()
	payload, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal input: %w", err)
	}

	cmd := exec.CommandContext(ctx, e.binaryPath)
	cmd.Stdin = bytes.NewReader(payload)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	if policyTimingEnabled() || policyStreamEnabled() {
		cmd.Stderr = io.MultiWriter(&stderr, os.Stderr)
	} else {
		cmd.Stderr = &stderr
	}

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("rust policy engine failed: %w (%s)", err, stderr.String())
	}

	var result Result
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("parse policy output: %w", err)
	}

	return &result, nil
}

func ensurePolicyBinary(policyDir string) (string, error) {
	if env := os.Getenv("VHDL_POLICY_BIN"); env != "" {
		if existsExecutable(env) {
			return env, nil
		}
		return "", fmt.Errorf("VHDL_POLICY_BIN is set but not executable: %s", env)
	}

	base := filepath.Dir(policyDir)
	profile := os.Getenv("VHDL_POLICY_PROFILE")
	if profile == "" {
		profile = "release"
	}
	candidates := []string{filepath.Join(base, "vhdl_policy")}
	if profile == "debug" {
		candidates = append([]string{
			filepath.Join(base, "target", "debug", "vhdl_policy"),
			filepath.Join(base, "target", "release", "vhdl_policy"),
		}, candidates...)
	} else {
		candidates = append([]string{
			filepath.Join(base, "target", "release", "vhdl_policy"),
			filepath.Join(base, "target", "debug", "vhdl_policy"),
		}, candidates...)
	}
	for _, candidate := range candidates {
		if existsExecutable(candidate) {
			return candidate, nil
		}
	}
	if path, err := exec.LookPath("vhdl_policy"); err == nil {
		return path, nil
	}

	if err := buildPolicyBinary(base, profile); err != nil {
		return "", err
	}
	for _, candidate := range candidates {
		if existsExecutable(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("vhdl_policy binary not found after build")
}

func buildPolicyBinary(base, profile string) error {
	args := []string{"build", "--quiet", "--bin", "vhdl_policy"}
	if profile == "release" {
		args = append(args, "--release")
	}
	cmd := exec.Command("cargo", args...)
	cmd.Dir = base
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("building vhdl_policy: %w (%s)", err, stderr.String())
	}
	return nil
}

func existsExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode()&0111 != 0
}

func policyTimingEnabled() bool {
	val := strings.ToLower(strings.TrimSpace(os.Getenv("VHDL_POLICY_TRACE_TIMING")))
	return val == "1" || val == "true" || val == "yes" || val == "on"
}

func policyStreamEnabled() bool {
	val := strings.ToLower(strings.TrimSpace(os.Getenv("VHDL_POLICY_STREAM")))
	return val == "1" || val == "true" || val == "yes" || val == "on"
}
