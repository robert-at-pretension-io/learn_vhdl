// Policy daemon protocol schema (Go <-> Rust vhdl_policyd).
// Ensures commands/responses are well-formed before transmission.

package daemon_schema

#Identifier: string & =~"^(?:[a-zA-Z_][a-zA-Z0-9_]*|\\\\.+\\\\)$"
#QualifiedIdentifier: string & =~"^(?:[a-zA-Z_][a-zA-Z0-9_]*|\\\\.+\\\\)(?:\\.(?:[a-zA-Z_][a-zA-Z0-9_]*|\\\\.+\\\\))*$"

#FactTables: {
    files:           [...#FileRow]
    entities:        [...#EntityRow]
    architectures:   [...#ArchitectureRow]
    packages:        [...#PackageRow]
    ports:           [...#PortRow]
    signals:         [...#SignalRow]
    instances:       [...#InstanceRow]
    dependencies:    [...#DependencyRow]
    use_clauses:     [...#UseClauseRow]
    library_clauses: [...#LibraryClauseRow]
    context_clauses: [...#ContextClauseRow]
    processes:       [...#ProcessRow]
    generates:       [...#GenerateRow]
    types:           [...#TypeRow]
    subtypes:        [...#SubtypeRow]
    functions:       [...#FunctionRow]
    procedures:      [...#ProcedureRow]
    constants:       [...#ConstantRow]
    symbols:         [...#SymbolRow]
}

#FileRow: {
    path:           string & =~".+\\.(vhd|vhdl)$"
    library:        string
    is_third_party: bool
}

#EntityRow: {
    name: #Identifier
    file: string & =~".+\\.(vhd|vhdl)$"
    line: int & >=1
}

#ArchitectureRow: {
    name:        #Identifier
    entity_name: #Identifier
    file:        string & =~".+\\.(vhd|vhdl)$"
    line:        int & >=1
}

#PackageRow: {
    name: #Identifier
    file: string & =~".+\\.(vhd|vhdl)$"
    line: int & >=1
}

#PortRow: {
    entity:    #Identifier
    name:      #Identifier
    direction: "in" | "out" | "inout" | "buffer" | "linkage" | ""
    type:      string & !=""
    file:      string & =~".+\\.(vhd|vhdl)$"
    line:      int & >=1
}

#SignalRow: {
    name:  #Identifier
    type:  string & !=""
    file:  string & =~".+\\.(vhd|vhdl)$"
    line:  int & >=1
    scope: string
}

#InstanceRow: {
    name:   #Identifier
    target: #QualifiedIdentifier | #Identifier
    file:   string & =~".+\\.(vhd|vhdl)$"
    line:   int & >=1
    in_arch: string
}

#DependencyRow: {
    file:   string & =~".+\\.(vhd|vhdl)$"
    target: #QualifiedIdentifier | #Identifier
    kind:   string & !=""
    line:   int & >=1
}

#UseClauseRow: {
    file: string & =~".+\\.(vhd|vhdl)$"
    item: #QualifiedIdentifier
    line: int & >=1
}

#LibraryClauseRow: {
    file:    string & =~".+\\.(vhd|vhdl)$"
    library: #Identifier
    line:    int & >=1
}

#ContextClauseRow: {
    file: string & =~".+\\.(vhd|vhdl)$"
    name: #QualifiedIdentifier
    line: int & >=1
}

#ProcessRow: {
    label:            string
    file:             string & =~".+\\.(vhd|vhdl)$"
    line:             int & >=1
    in_arch:          string
    is_sequential:    bool
    is_combinational: bool
}

#GenerateRow: {
    label:        string
    kind:         string & !=""
    file:         string & =~".+\\.(vhd|vhdl)$"
    line:         int & >=1
    in_arch:      string
    can_elaborate: bool
}

#TypeRow: {
    name:       #Identifier
    kind:       string & !=""
    file:       string & =~".+\\.(vhd|vhdl)$"
    line:       int & >=1
    in_package: string
    in_arch:    string
}

#SubtypeRow: {
    name:       #Identifier
    base_type:  #Identifier
    file:       string & =~".+\\.(vhd|vhdl)$"
    line:       int & >=1
    in_package: string
    in_arch:    string
}

#FunctionRow: {
    name:        #Identifier
    return_type: string & !=""
    file:        string & =~".+\\.(vhd|vhdl)$"
    line:        int & >=1
    in_package:  string
    in_arch:     string
    is_pure:     bool
    has_body:    bool
}

#ProcedureRow: {
    name:       #Identifier
    file:       string & =~".+\\.(vhd|vhdl)$"
    line:       int & >=1
    in_package: string
    in_arch:    string
    has_body:   bool
}

#ConstantRow: {
    name:       #Identifier
    type:       string & !=""
    value:      string
    file:       string & =~".+\\.(vhd|vhdl)$"
    line:       int & >=1
    in_package: string
    in_arch:    string
}

#SymbolRow: {
    name: #QualifiedIdentifier | #Identifier
    kind: string & !=""
    file: string & =~".+\\.(vhd|vhdl)$"
    line: int & >=1
}

#Violation: {
    rule:     string & !=""
    severity: "error" | "warning" | "info"
    file:     string & =~".+\\.(vhd|vhdl)$"
    line:     int & >=1
    message:  string & !=""
}

#Summary: {
    total_violations: int & >=0
    errors:           int & >=0
    warnings:         int & >=0
    info:             int & >=0
}

#PolicyDaemonCommand: {
    kind: "init"
    tables: #FactTables
} | {
    kind: "delta"
    added:   #FactTables
    removed: #FactTables
} | {
    kind: "snapshot"
}

#PolicyDaemonResponse: {
    kind: "snapshot"
    summary:    #Summary
    violations: [...#Violation]
} | {
    kind:    "error"
    message: string & !=""
}
