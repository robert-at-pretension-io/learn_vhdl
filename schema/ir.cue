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
    entities:        [...#Entity]
    architectures:   [...#Architecture]
    packages:        [...#Package]
    components:      [...#Component]
    use_clauses:     [...#UseClause]
    library_clauses: [...#LibraryClause]
    context_clauses: [...#ContextClause]
    signals:         [...#Signal]
    ports:           [...#Port]
    dependencies:    [...#Dependency]
    symbols:         [...#Symbol]
    scopes:          [...#Scope]
    symbol_defs:     [...#SymbolDef]
    name_uses:       [...#NameUse]
    verification_blocks:    [...#VerificationBlock]
    verification_tags:      [...#VerificationTag]
    verification_tag_errors:[...#VerificationTagError]
    files:           [...#FileInfo]
    instances:       [...#Instance]
    case_statements: [...#CaseStatement]
    processes:       [...#Process]
    configurations:  [...#Configuration]
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
}

#FileInfo: {
    path:          string
    library:       string
    is_third_party: bool
}

#Scope: {
    name:   string
    kind:   string
    file:   string & =~".+\\.(vhd|vhdl)$"
    line:   int & >=1
    parent: string
    path:   [...string]
}

#SymbolDef: {
    name:  string
    kind:  string
    file:  string & =~".+\\.(vhd|vhdl)$"
    line:  int & >=1
    scope: string
}

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
    name: #QualifiedIdentifier  // Qualified: work.my_entity
    kind: "entity" | "package" | "component" | "architecture"
    file: string & =~".+\\.(vhd|vhdl)$"
    line: int & >=1
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

// Configuration declaration
#Configuration: {
    name:        #Identifier
    entity_name: #Identifier
    file:        string & =~".+\\.(vhd|vhdl)$"
    line:        int & >=1
}
