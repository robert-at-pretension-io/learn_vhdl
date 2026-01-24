// VHDL Intermediate Representation Schema
// This schema validates the data passed from the Go indexer to OPA

package schema

// Root structure for the IR
#IR: {
    files: [...#FileFacts]
    symbols: [...#Symbol]
    dependencies: [...#Dependency]
    errors: [...#Error]
}

// Facts extracted from a single file
#FileFacts: {
    file: string
    entities: [...#Entity]
    architectures: [...#Architecture]
    packages: [...#Package]
    components: [...#Component]
    signals: [...#Signal]
    ports: [...#Port]
}

// Entity declaration
#Entity: {
    name: string
    line: int
    ports: [...#Port]
}

// Architecture body
#Architecture: {
    name: string
    entity_name: string
    line: int
}

// Package declaration
#Package: {
    name: string
    line: int
}

// Component declaration or instantiation
#Component: {
    name: string
    entity_ref: string
    line: int
    is_instance: bool
}

// Signal declaration
#Signal: {
    name: string
    type: string
    line: int
    in_entity: string
}

// Port declaration
#Port: {
    name: string
    direction: "in" | "out" | "inout" | "buffer" | "linkage"
    type: string
    line: int
}

// Global symbol
#Symbol: {
    name: string  // Qualified: work.my_entity
    kind: "entity" | "package" | "component" | "architecture"
    file: string
    line: int
}

// Dependency between files/entities
#Dependency: {
    source: string
    target: string
    kind: "use" | "library" | "instantiation" | "component"
    line: int
    resolved: bool  // Was the target found?
}

// Parse or validation error
#Error: {
    file: string
    line: int
    message: string
    severity: "error" | "warning" | "info"
}
