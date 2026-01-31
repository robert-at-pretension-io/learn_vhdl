// Relational fact tables schema (Datalog-friendly)
// This is the CONTRACT between Go and the fact-table output.

package facts_schema

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
    target: string & !=""
    kind:   "use" | "library" | "instantiation" | "context" | string
    line:   int & >=1
}

#UseClauseRow: {
    file: string & =~".+\\.(vhd|vhdl)$"
    item: #QualifiedIdentifier | string
    line: int & >=1
}

#LibraryClauseRow: {
    file:    string & =~".+\\.(vhd|vhdl)$"
    library: #Identifier
    line:    int & >=1
}

#ContextClauseRow: {
    file: string & =~".+\\.(vhd|vhdl)$"
    // Context references can appear in negative tests with non-standard identifiers.
    // Keep non-empty constraint so invalid user input doesn't crash validation.
    name: string & !=""
    line: int & >=1
}

#ProcessRow: {
    label:           string
    file:            string & =~".+\\.(vhd|vhdl)$"
    line:            int & >=1
    in_arch:         string
    is_sequential:   bool
    is_combinational: bool
}

#GenerateRow: {
    label:   string
    kind:    string
    file:    string & =~".+\\.(vhd|vhdl)$"
    line:    int & >=1
    in_arch: string
    can_elaborate: bool
}

#TypeRow: {
    name:      #Identifier
    kind:      string
    file:      string & =~".+\\.(vhd|vhdl)$"
    line:      int & >=1
    in_package: string
    in_arch:    string
}

#SubtypeRow: {
    name:      #Identifier
    base_type: string & !=""
    file:      string & =~".+\\.(vhd|vhdl)$"
    line:      int & >=1
    in_package: string
    in_arch:    string
}

#FunctionRow: {
    name:        #Identifier | ""
    // Some negative tests omit return types; keep non-empty constraint out of facts schema
    // to avoid crashing on invalid user code.
    return_type: string
    file:        string & =~".+\\.(vhd|vhdl)$"
    line:        int & >=1
    in_package:  string
    in_arch:     string
    is_pure:     bool
    has_body:    bool
}

#ProcedureRow: {
    name:       #Identifier | ""
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
    kind: string
    file: string & =~".+\\.(vhd|vhdl)$"
    line: int & >=1
}
