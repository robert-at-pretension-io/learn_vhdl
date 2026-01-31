// VHDL Linter Output Schema
// This validates the JSON output format from the linter
// Use: cue vet output_schema.cue output.json
//
// This schema ensures consistent, machine-parseable output for:
// - CI/CD integration
// - IDE plugins
// - Automated analysis pipelines

package schema

// LintOutput is the root structure of JSON output
#LintOutput: {
    violations:   [...#Violation]
    missing_checks: [...#MissingCheck] | *[]
    ambiguous_constructs: [...#AmbiguousConstruct] | *[]
    summary:      #Summary
    stats:        #Stats
    files:        [...#FileResult]
    parse_errors: [...#ParseError] | *[]
}

// Violation represents a policy violation found by the linter
#Violation: {
    rule:     string & =~"^[a-z_]+$"        // Snake-case rule identifier
    severity: "error" | "warning" | "info"
    file:     string & =~".+\\.(vhd|vhdl)$" // Must be VHDL file
    line:     int & >=1                      // Line numbers start at 1
    message:  string & !=""                  // Human-readable description
}

// Summary provides aggregate counts
#Summary: {
    total_violations: int & >=0
    errors:           int & >=0
    warnings:         int & >=0
    info:             int & >=0
}

// Stats provides extraction statistics
#Stats: {
    files:     int & >=0
    symbols:   int & >=0
    entities:  int & >=0
    packages:  int & >=0
    signals:   int & >=0
    ports:     int & >=0
    processes: int & >=0
    instances: int & >=0
    generates: int & >=0
}

// FileResult provides per-file violation breakdown
#FileResult: {
    path:     string & =~".+\\.(vhd|vhdl)$"
    errors:   int & >=0
    warnings: int & >=0
    info:     int & >=0
}

// ParseError represents a parsing failure
#ParseError: {
    file:    string
    message: string & !=""
}

// MissingCheckTask is a structured task for the verification agent.
#MissingCheck: {
    file:        string & =~".+\\.(vhd|vhdl)$"
    scope:       string & =~"^(arch|entity):.+$"
    anchor:      #VerificationAnchor
    missing_ids: [...string]
    bindings:    {[string]: string} | *{}
    notes:       [...string] | *[]
}

// VerificationAnchor identifies the insertion location for tags.
#VerificationAnchor: {
    label:      string
    line_start: int & >=1
    line_end:   int & >=1
    exists:     bool
}

// AmbiguousConstruct reports uncertain detection candidates.
#AmbiguousConstruct: {
    kind:       string & !=""
    scope:      string & =~"^(arch|entity):.+$"
    file:       string & =~".+\\.(vhd|vhdl)$"
    line:       int & >=1
    candidates: {[string]: [...string]}
}
