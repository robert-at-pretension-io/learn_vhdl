// VHDL Linter Input Schema
// This is the CONTRACT between Go (extractor/indexer) and the Rust policy engine.
// Any field mismatch will cause immediate validation failure.
// "Silent failures" are impossible - if the policy engine gets bad data, we crash first.

package schema

// Identifier patterns (standard or extended identifier)
#Identifier: string & =~"^(?:[a-zA-Z_][a-zA-Z0-9_]*|\\\\.+\\\\)$"
#QualifiedIdentifier: string & =~"^(?:[a-zA-Z_][a-zA-Z0-9_]*|\\\\.+\\\\)(?:\\.(?:[a-zA-Z_][a-zA-Z0-9_]*|\\\\.+\\\\))*$"

// Input is the root structure passed to the policy engine
// This MUST match policy.Input in Go exactly
#Input: {
    standard:              "1993" | "2002" | "2008" | "2019"
    file_count:            int & >=1
    entities:               [...#Entity]
    architectures:          [...#Architecture]
    packages:               [...#Package]
    components:             [...#Component]
    use_clauses:            [...#UseClause]
    library_clauses:        [...#LibraryClause]
    context_clauses:        [...#ContextClause]
    signals:                [...#Signal]
    ports:                  [...#Port]
    dependencies:           [...#Dependency]
    symbols:                [...#Symbol]
    scopes:                 [...#Scope]
    symbol_defs:            [...#SymbolDef]
    name_uses:              [...#NameUse]
    verification_blocks:    [...#VerificationBlock]
    verification_tags:      [...#VerificationTag]
    verification_tag_errors:[...#VerificationTagError]
    files:                  [...#FileInfo]
    instances:              [...#Instance]
    case_statements:        [...#CaseStatement]
    processes:              [...#Process]
    concurrent_assignments: [...#ConcurrentAssignment]
    generates:              [...#GenerateStatement]
    configurations:         [...#Configuration]
    signal_usages:          [...#SignalUsage]
    // Type system
    types:                  [...#TypeDeclaration]
    subtypes:               [...#SubtypeDeclaration]
    functions:              [...#FunctionDeclaration]
    procedures:             [...#ProcedureDeclaration]
    constant_decls:         [...#ConstantDeclaration]
    // Type system info for filtering false positives (LEGACY - use types/constant_decls instead)
    enum_literals:          [...string]  // Enum literals from type declarations (e.g., S_IDLE, S_RUN)
    constants:              [...string]  // Constants from constant declarations (names only)
    shared_variables:       [...string]  // Shared variable names (not signals)
    // Advanced analysis for security/power/correctness
    comparisons:            [...#Comparison]
    arithmetic_ops:         [...#ArithmeticOp]
    signal_deps:            [...#SignalDep]
    cdc_crossings:          [...#CDCCrossing]
    // Configuration
    lint_config:            #LintConfig  // Rule severities from vhdl_lint.json
    third_party_files:      [...string]  // Files from third-party libraries (suppress warnings)
}

// Configuration represents a VHDL configuration declaration
#Configuration: {
    name:        #Identifier
    entity_name: #Identifier
    file:        string & =~".+\\.(vhd|vhdl)$"
    line:        int & >=1
}

// LintConfig contains rule configuration passed to the policy engine
#LintConfig: {
    rules: {[string]: "off" | "info" | "warning" | "error"}  // rule name -> severity
}

// Entity declaration
#Entity: {
    name:  #Identifier  // Valid VHDL identifier
    file:  string & =~".+\\.(vhd|vhdl)$"          // Must be VHDL file
    line:  int & >=1                               // Line numbers start at 1
    ports: [...#Port]
    generics: [...#GenericDecl]
}

// Architecture body
#Architecture: {
    name:        #Identifier
    entity_name: #Identifier
    file:        string & =~".+\\.(vhd|vhdl)$"
    line:        int & >=1
}

// Package declaration
#Package: {
    name: #Identifier
    file: string & =~".+\\.(vhd|vhdl)$"
    line: int & >=1
}

// Component declaration or instantiation
#Component: {
    name:        #Identifier
    entity_ref:  string  // Can be empty for forward declarations
    file:        string & =~".+\\.(vhd|vhdl)$"
    line:        int & >=1
    is_instance: bool
    ports:       [...#Port]
    generics:    [...#GenericDecl]
}

// Signal declaration
#Signal: {
    name:      #Identifier
    type:      string & !=""  // Type must not be empty
    file:      string & =~".+\\.(vhd|vhdl)$"
    line:      int & >=1
    in_entity: string  // Which entity/architecture this signal belongs to
    width:     int & >=0  // Estimated bit width (0 if unknown)
}

// Port declaration
// Note: direction can be empty for generics (which are parsed similarly to ports)
#Port: {
    name:      #Identifier
    direction: "in" | "out" | "inout" | "buffer" | "linkage" | ""
    type:      string & !=""  // Type must not be empty
    default:   string
    line:      int & >=1
    in_entity: string  // Which entity this port belongs to
    width:     int & >=0  // Estimated bit width (0 if unknown)
}

#GenericDecl: {
    name:         #Identifier
    kind:         "constant" | "type" | "function" | "procedure" | "package" | string
    type:         string
    class:        string
    default:      string
    line:         int & >=1
    in_entity:    string
    in_component: string
}

#UseClause: {
    items: [...string]
    file:  string & =~".+\\.(vhd|vhdl)$"
    line:  int & >=1
}

#LibraryClause: {
    libraries: [...string]
    file:      string & =~".+\\.(vhd|vhdl)$"
    line:      int & >=1
}

#ContextClause: {
    name: string
    file: string & =~".+\\.(vhd|vhdl)$"
    line: int & >=1
}

#Association: {
    kind:           "port" | "generic" | string
    formal:         string
    actual:         string
    is_positional:  bool
    actual_kind:    string
    actual_base:    string
    actual_full:    string
    line:           int & >=1
    position_index: int & >=0
}

#VariableDecl: {
    name: #Identifier
    type: string
    line: int & >=1
}

#ProcedureCall: {
    name:       string
    full_name:  string
    args:       [...string]
    line:       int & >=1
    in_process: string
    in_arch:    string
}

#FunctionCall: {
    name:       string
    args:       [...string]
    line:       int & >=1
    in_process: string
    in_arch:    string
}

#WaitStatement: {
    on_signals: [...string]
    until_expr: string
    for_expr:   string
    line:       int & >=1
}

// Dependency between files/entities
#Dependency: {
    source:   string & !=""  // Source file or entity
    target:   string & !=""  // Target (e.g., "ieee.std_logic_1164")
    kind:     "use" | "library" | "instantiation" | "component" | "context" | "package_instantiation" | "configuration_specification" | "subprogram_instantiation"
    line:     int & >=1
    resolved: bool  // Was the target found in the symbol table?
}

// Global symbol in the cross-file symbol table
#Symbol: {
    name: #QualifiedIdentifier  // Qualified: work.my_entity or work.my_pkg.my_type
    kind: "entity" | "package" | "component" | "architecture" | "type" | "constant" | "function" | "procedure"
    file: string & =~".+\\.(vhd|vhdl)$"
    line: int & >=1
}

// FileInfo describes a parsed VHDL file and its library assignment.
#FileInfo: {
    path:          string
    library:       string
    is_third_party: bool
}

// Scope represents a lexical or generate scope for name resolution.
#Scope: {
    name:   string
    kind:   string
    file:   string & =~".+\\.(vhd|vhdl)$"
    line:   int & >=1
    parent: string
    path:   [...string]
}

// SymbolDef represents a symbol definition with scope information.
#SymbolDef: {
    name:  string
    kind:  string
    file:  string & =~".+\\.(vhd|vhdl)$"
    line:  int & >=1
    scope: string
}

// NameUse represents a usage of a name in a given scope.
#NameUse: {
    name:    string
    kind:    string
    file:    string & =~".+\\.(vhd|vhdl)$"
    line:    int & >=1
    scope:   string
    context: string
}

#VerificationBlock: {
    label:      string
    line_start: int & >=1
    line_end:   int & >=1
    file:       string & =~".+\\.(vhd|vhdl)$"
    in_arch:    string
}

#VerificationTag: {
    id:       string
    scope:    string & =~"^(entity|arch):.+$"
    bindings: {[string]: string}
    file:     string & =~".+\\.(vhd|vhdl)$"
    line:     int & >=1
    raw:      string
    in_arch:  string
}

#VerificationTagError: {
    file:    string & =~".+\\.(vhd|vhdl)$"
    line:    int & >=1
    raw:     string
    message: string & !=""
    in_arch: string
}

// Instance represents a component/entity instantiation with port/generic mappings
// Enables system-level analysis (cross-module signal tracing)
#Instance: {
    name:        #Identifier  // Instance label
    target:      string & !=""                          // Target entity/component
    port_map:    {[string]: string}                     // Formal -> actual signal
    generic_map: {[string]: string}                     // Formal -> actual value
    associations: [...#Association]
    file:        string & =~".+\\.(vhd|vhdl)$"
    line:        int & >=1
    in_arch:     string                                 // Containing architecture
}

// CaseStatement represents a VHDL case statement for latch detection
// A case statement without "others" can infer a latch in combinational logic
#CaseStatement: {
    expression:  string                                 // The case expression
    choices:     [...string]                            // All explicit choices
    has_others:  bool                                   // true if "when others =>" present
    file:        string & =~".+\\.(vhd|vhdl)$"
    line:        int & >=1
    in_process:  string                                 // Which process contains this
    in_arch:     string                                 // Which architecture
    is_complete: bool                                   // true if complete coverage
}

// Process represents a VHDL process for sensitivity/clock/reset analysis
#Process: {
    label:            string                            // Process label (can be empty)
    sensitivity_list: [...string]                       // Signals in sensitivity list
    is_sequential:    bool                              // Has clock edge
    is_combinational: bool                              // No clock edge
    clock_signal:     string                            // Clock signal if sequential
    clock_edge:       string                            // "rising" or "falling" if sequential
    has_reset:        bool                              // Has reset logic
    reset_signal:     string                            // Reset signal name
    reset_async:      bool                              // Async reset if checked before clock
    assigned_signals: [...string]                       // Signals written
    read_signals:     [...string]                       // Signals read
    variables:        [...#VariableDecl]
    procedure_calls:  [...#ProcedureCall]
    function_calls:   [...#FunctionCall]
    wait_statements:  [...#WaitStatement]
    file:             string & =~".+\\.(vhd|vhdl)$"
    line:             int & >=1
    in_arch:          string                            // Containing architecture
}

// ConcurrentAssignment represents a concurrent signal assignment (outside processes)
// Enables detection of undriven/multi-driven signals that were previously missed
#ConcurrentAssignment: {
    target:         string & =~"^[a-zA-Z_][a-zA-Z0-9_.]*$"  // Signal path (may include dots for record fields)
    read_signals:   [...string]                             // Signals being read
    file:           string & =~".+\\.(vhd|vhdl)$"
    line:           int & >=1
    in_arch:        string                                  // Containing architecture
    kind:           "simple" | "conditional" | "selected"   // Assignment type
    in_generate:    bool                                    // True if inside generate block
    generate_label: string                                  // Label of containing generate
}

// Comparison represents a comparison operation for trojan/trigger detection
// Tracks comparisons against literals, especially large "magic" values
#Comparison: {
    left_operand:  string                               // Signal or expression on left
    operator:      string                               // =, /=, <, >, <=, >=
    right_operand: string                               // Signal, literal, or expression
    is_literal:    bool                                 // True if right operand is a literal
    literal_value: string                               // The literal value if is_literal
    literal_bits:  int & >=0                            // Estimated bit width of literal
    result_drives: string                               // What signal this comparison drives
    file:          string & =~".+\\.(vhd|vhdl)$"
    line:          int & >=1
    in_process:    string                               // Which process contains this
    in_arch:       string                               // Which architecture
}

// ArithmeticOp represents an expensive arithmetic operation for power analysis
#ArithmeticOp: {
    operator:     string                                // *, /, mod, rem, **
    operands:     [...string]                           // Input signals/expressions
    result:       string                                // Output signal
    is_guarded:   bool                                  // True if gated by enable
    guard_signal: string                                // The enable/valid signal
    file:         string & =~".+\\.(vhd|vhdl)$"
    line:         int & >=1
    in_process:   string                                // Which process
    in_arch:      string                                // Which architecture
}

// SignalDep represents a signal dependency for combinational loop detection
#SignalDep: {
    // Allow empty/invalid names so malformed inputs don't crash validation.
    source:        string                                  // Signal path being read (dots for record fields)
    target:        string                                  // Signal path being assigned
    in_process:    string                                  // Which process (empty if concurrent)
    is_sequential: bool                                    // True if crosses clock boundary
    file:          string & =~".+\\.(vhd|vhdl)$"
    line:          int & >=1
    in_arch:       string                                  // Which architecture
}

// CDCCrossing represents a potential clock domain crossing
// Detected when a signal written in one clock domain is read in another
#CDCCrossing: {
    signal:          string                             // Signal crossing domains
    source_clock:    string                             // Clock domain where signal is written
    source_proc:     string                             // Process that writes the signal
    dest_clock:      string                             // Clock domain where signal is read
    dest_proc:       string                             // Process that reads the signal
    is_synchronized: bool                               // True if synchronizer detected
    sync_stages:     int & >=0                          // Number of synchronizer stages
    is_multi_bit:    bool                               // True if signal is wider than 1 bit
    file:            string & =~".+\\.(vhd|vhdl)$"
    line:            int & >=1
    in_arch:         string                             // Which architecture
}

// GenerateStatement represents a VHDL generate statement (for/if/case generate)
// Generate statements create conditional or iterative scopes with their own declarations
#GenerateStatement: {
    label:          string                              // Generate block label (required)
    kind:           "for" | "if" | "case" | ""          // Generate type (empty if not yet determined)
    file:           string & =~".+\\.(vhd|vhdl)$"
    line:           int & >=1
    in_arch:        string                              // Containing architecture
    // For-generate specific (optional)
    loop_var?:      string                              // Loop variable name
    range_low?:     string                              // Range low bound
    range_high?:    string                              // Range high bound
    range_dir?:     "to" | "downto" | ""                // Range direction
    // Elaboration results (for-generate)
    iteration_count: int                                // Number of iterations (-1 if cannot evaluate)
    can_elaborate?:  bool                               // True if range was successfully evaluated
    // If-generate specific (optional)
    condition?:     string                              // Condition expression
    // Nested content counts
    signal_count:   int & >=0                           // Signals declared inside
    instance_count: int & >=0                           // Instances inside
    process_count:  int & >=0                           // Processes inside
}

// SignalUsage tracks where signals are read, written, or used in port maps
// Enables accurate detection of undriven signals (driven by component outputs)
#SignalUsage: {
    signal:        string                               // Signal name
    is_read:       bool                                 // Signal is read
    is_written:    bool                                 // Signal is written
    in_process:    string                               // Process where usage occurs (empty if concurrent)
    in_port_map:   bool                                 // Used as actual in component port map
    instance_name: string                               // Instance name if in port map
    in_psl:        bool                                 // True if usage appears in PSL property/sequence/assert
    line:          int & >=1                            // Line number
}

// =========================================================================
// TYPE SYSTEM DEFINITIONS
// =========================================================================

// TypeDeclaration represents a VHDL type declaration
#TypeDeclaration: {
    name:           #Identifier
    kind:           "enum" | "record" | "array" | "physical" | "access" | "file" | "incomplete" | "protected" | "range" | "alias"
    file:           string & =~".+\\.(vhd|vhdl)$"
    line:           int & >=1
    in_package?:    string                              // Package containing this type
    in_arch?:       string                              // Architecture if local type
    // Enum-specific
    enum_literals?: [...string]                         // For enums: ["IDLE", "RUN", "STOP"]
    // Record-specific
    fields?:        [...#RecordField]                   // For records: field definitions
    // Array-specific
    element_type?:  string                              // For arrays: element type
    index_types?:   [...string]                         // For arrays: index type(s)
    unconstrained?: bool                                // For arrays: true if "range <>"
    // Physical-specific
    base_unit?:     string                              // For physical: base unit name
    // Range-specific
    range_low?:     string                              // For range types: low bound
    range_high?:    string                              // For range types: high bound
    range_dir?:     "to" | "downto" | ""                // Range direction
}

// RecordField represents a field in a record type
#RecordField: {
    name: #Identifier
    type: string & !=""
    line: int & >=1
}

// SubtypeDeclaration represents a VHDL subtype declaration
#SubtypeDeclaration: {
    name:        #Identifier
    base_type:   string & !=""                          // The parent type
    constraint?: string                                 // Range or index constraint
    resolution?: string                                 // Resolution function
    file:        string & =~".+\\.(vhd|vhdl)$"
    line:        int & >=1
    in_package?: string
    in_arch?:    string
}

// FunctionDeclaration represents a VHDL function declaration or body
#FunctionDeclaration: {
    name:        string                                 // Can be identifier or operator symbol
    // Some negative tests omit return types; allow empty to avoid crashing on invalid inputs.
    return_type: string
    parameters?: [...#SubprogramParameter]
    is_pure:     bool                                   // true for pure (default), false for impure
    has_body:    bool                                   // true if function body, not just declaration
    file:        string & =~".+\\.(vhd|vhdl)$"
    line:        int & >=1
    in_package?: string
    in_arch?:    string
}

// ProcedureDeclaration represents a VHDL procedure declaration or body
#ProcedureDeclaration: {
    name:        #Identifier
    parameters?: [...#SubprogramParameter]
    has_body:    bool                                   // true if procedure body
    file:        string & =~".+\\.(vhd|vhdl)$"
    line:        int & >=1
    in_package?: string
    in_arch?:    string
}

// SubprogramParameter represents a parameter in a function or procedure
#SubprogramParameter: {
    name:       #Identifier
    direction?: "in" | "out" | "inout" | "buffer" | "linkage" | ""  // Empty defaults to "in"
    // Some negative tests omit parameter types; allow empty to avoid crashing on invalid inputs.
    type:       string
    class?:     "signal" | "variable" | "constant" | "file" | ""
    default?:   string                                  // Default value expression
    line:       int & >=1
}

// ConstantDeclaration represents a VHDL constant declaration
#ConstantDeclaration: {
    name:       #Identifier
    type:       string & !=""
    value?:     string                                  // May be empty for deferred constants
    file:       string & =~".+\\.(vhd|vhdl)$"
    line:       int & >=1
    in_package?: string                                 // Package containing this constant
    in_arch?:    string                                 // Architecture if local constant
}
