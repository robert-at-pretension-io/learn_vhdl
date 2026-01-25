// VHDL Linter Input Schema
// This is the CONTRACT between Go (extractor/indexer) and OPA (policy engine)
// Any field mismatch will cause immediate validation failure
// "Silent failures" are impossible - if OPA gets bad data, we crash first

package schema

// Input is the root structure passed to OPA
// This MUST match policy.Input in Go exactly
#Input: {
    entities:               [...#Entity]
    architectures:          [...#Architecture]
    packages:               [...#Package]
    components:             [...#Component]
    signals:                [...#Signal]
    ports:                  [...#Port]
    dependencies:           [...#Dependency]
    symbols:                [...#Symbol]
    instances:              [...#Instance]
    case_statements:        [...#CaseStatement]
    processes:              [...#Process]
    concurrent_assignments: [...#ConcurrentAssignment]
    // Advanced analysis for security/power/correctness
    comparisons:            [...#Comparison]
    arithmetic_ops:         [...#ArithmeticOp]
    signal_deps:            [...#SignalDep]
}

// Entity declaration
#Entity: {
    name:  string & =~"^[a-zA-Z_][a-zA-Z0-9_]*$"  // Valid VHDL identifier
    file:  string & =~".+\\.(vhd|vhdl)$"          // Must be VHDL file
    line:  int & >=1                               // Line numbers start at 1
    ports: [...#Port]
}

// Architecture body
#Architecture: {
    name:        string & =~"^[a-zA-Z_][a-zA-Z0-9_]*$"
    entity_name: string & =~"^[a-zA-Z_][a-zA-Z0-9_]*$"
    file:        string & =~".+\\.(vhd|vhdl)$"
    line:        int & >=1
}

// Package declaration
#Package: {
    name: string & =~"^[a-zA-Z_][a-zA-Z0-9_]*$"
    file: string & =~".+\\.(vhd|vhdl)$"
    line: int & >=1
}

// Component declaration or instantiation
#Component: {
    name:        string & =~"^[a-zA-Z_][a-zA-Z0-9_]*$"
    entity_ref:  string  // Can be empty for forward declarations
    file:        string & =~".+\\.(vhd|vhdl)$"
    line:        int & >=1
    is_instance: bool
}

// Signal declaration
#Signal: {
    name:      string & =~"^[a-zA-Z_][a-zA-Z0-9_]*$"
    type:      string & !=""  // Type must not be empty
    file:      string & =~".+\\.(vhd|vhdl)$"
    line:      int & >=1
    in_entity: string  // Which entity/architecture this signal belongs to
}

// Port declaration
// Note: direction can be empty for generics (which are parsed similarly to ports)
#Port: {
    name:      string & =~"^[a-zA-Z_][a-zA-Z0-9_]*$"
    direction: "in" | "out" | "inout" | "buffer" | "linkage" | ""
    type:      string & !=""  // Type must not be empty
    line:      int & >=1
    in_entity: string  // Which entity this port belongs to
}

// Dependency between files/entities
#Dependency: {
    source:   string & !=""  // Source file or entity
    target:   string & !=""  // Target (e.g., "ieee.std_logic_1164")
    kind:     "use" | "library" | "instantiation" | "component"
    line:     int & >=1
    resolved: bool  // Was the target found in the symbol table?
}

// Global symbol in the cross-file symbol table
#Symbol: {
    name: string & =~"^[a-zA-Z_][a-zA-Z0-9_.]*$"  // Qualified: work.my_entity
    kind: "entity" | "package" | "component" | "architecture"
    file: string & =~".+\\.(vhd|vhdl)$"
    line: int & >=1
}

// Instance represents a component/entity instantiation with port/generic mappings
// Enables system-level analysis (cross-module signal tracing)
#Instance: {
    name:        string & =~"^[a-zA-Z_][a-zA-Z0-9_]*$"  // Instance label
    target:      string & !=""                          // Target entity/component
    port_map:    {[string]: string}                     // Formal -> actual signal
    generic_map: {[string]: string}                     // Formal -> actual value
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
    has_reset:        bool                              // Has reset logic
    reset_signal:     string                            // Reset signal name
    assigned_signals: [...string]                       // Signals written
    read_signals:     [...string]                       // Signals read
    file:             string & =~".+\\.(vhd|vhdl)$"
    line:             int & >=1
    in_arch:          string                            // Containing architecture
}

// ConcurrentAssignment represents a concurrent signal assignment (outside processes)
// Enables detection of undriven/multi-driven signals that were previously missed
#ConcurrentAssignment: {
    target:       string & =~"^[a-zA-Z_][a-zA-Z0-9_]*$"  // Signal being assigned
    read_signals: [...string]                            // Signals being read
    file:         string & =~".+\\.(vhd|vhdl)$"
    line:         int & >=1
    in_arch:      string                                 // Containing architecture
    kind:         "simple" | "conditional" | "selected"  // Assignment type
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
    source:        string                               // Signal being read
    target:        string                               // Signal being assigned
    in_process:    string                               // Which process (empty if concurrent)
    is_sequential: bool                                 // True if crosses clock boundary
    file:          string & =~".+\\.(vhd|vhdl)$"
    line:          int & >=1
    in_arch:       string                               // Which architecture
}
