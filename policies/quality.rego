# Code Quality Rules
# Rules for general code quality, maintainability, and best practices
package vhdl.quality

import data.vhdl.helpers

# Rule: Very long file (many entities/architectures)
very_long_file[violation] {
    file := input.entities[_].file
    entities_in_file := count([e | e := input.entities[_]; e.file == file])
    archs_in_file := count([a | a := input.architectures[_]; a.file == file])
    total := entities_in_file + archs_in_file
    total > 5
    # Only report once per file
    entity := input.entities[_]
    entity.file == file
    entity.line == min([e.line | e := input.entities[_]; e.file == file])
    violation := {
        "rule": "very_long_file",
        "severity": "info",
        "file": file,
        "line": 1,
        "message": sprintf("File contains %d design units - consider splitting into separate files", [total])
    }
}

# Rule: Package with too many items
large_package[violation] {
    pkg := input.packages[_]
    # Heuristic: count signals as proxy for package size
    signals_in_pkg := count([s | s := input.signals[_]; s.in_entity == pkg.name])
    signals_in_pkg > 50
    violation := {
        "rule": "large_package",
        "severity": "info",
        "file": pkg.file,
        "line": pkg.line,
        "message": sprintf("Package '%s' is very large (%d items) - consider splitting", [pkg.name, signals_in_pkg])
    }
}

# Rule: Signal name too short (single character except i,j,k for loops)
short_signal_name[violation] {
    sig := input.signals[_]
    count(sig.name) == 1
    not is_loop_variable(sig.name)
    violation := {
        "rule": "short_signal_name",
        "severity": "info",
        "file": sig.file,
        "line": sig.line,
        "message": sprintf("Signal '%s' has very short name - consider a more descriptive name", [sig.name])
    }
}

# Helper: Check if name is typical loop variable
is_loop_variable(name) {
    loop_vars := {"i", "j", "k", "n", "x", "y"}
    loop_vars[lower(name)]
}

# Rule: Signal name too long (hard to read)
long_signal_name[violation] {
    sig := input.signals[_]
    count(sig.name) > 40
    violation := {
        "rule": "long_signal_name",
        "severity": "info",
        "file": sig.file,
        "line": sig.line,
        "message": sprintf("Signal '%s' has very long name (%d chars) - consider abbreviating", [sig.name, count(sig.name)])
    }
}

# Rule: Port name too short
short_port_name[violation] {
    port := input.ports[_]
    count(port.name) == 1
    not is_loop_variable(port.name)
    violation := {
        "rule": "short_port_name",
        "severity": "info",
        "file": input.entities[_].file,
        "line": port.line,
        "message": sprintf("Port '%s' has very short name - consider a more descriptive name", [port.name])
    }
}

# Rule: Entity name with numbers (often indicates poor naming)
entity_name_with_numbers[violation] {
    entity := input.entities[_]
    regex.match(".*[0-9].*", entity.name)
    not is_versioned_name(entity.name)
    violation := {
        "rule": "entity_name_with_numbers",
        "severity": "info",
        "file": entity.file,
        "line": entity.line,
        "message": sprintf("Entity '%s' contains numbers - consider a more descriptive name", [entity.name])
    }
}

# Helper: Check for version-like naming
is_versioned_name(name) {
    regex.match(".*_v[0-9]+$", lower(name))
}
is_versioned_name(name) {
    regex.match(".*_rev[0-9]+$", lower(name))
}

# Rule: Inconsistent port direction grouping
# Good practice: group all inputs together, then outputs
# This is hard to check without knowing port order, so we check for alternating directions
mixed_port_directions[violation] {
    entity := input.entities[_]
    ports := entity.ports
    count(ports) > 4
    has_direction_alternation(ports)
    violation := {
        "rule": "mixed_port_directions",
        "severity": "info",
        "file": entity.file,
        "line": entity.line,
        "message": sprintf("Entity '%s' has mixed port directions - consider grouping inputs and outputs together", [entity.name])
    }
}

# Helper: Check for direction alternation
has_direction_alternation(ports) {
    count(ports) > 2
    p1 := ports[i]
    p2 := ports[i+1]
    p3 := ports[i+2]
    p1.direction == "in"
    p2.direction == "out"
    p3.direction == "in"
}
has_direction_alternation(ports) {
    count(ports) > 2
    p1 := ports[i]
    p2 := ports[i+1]
    p3 := ports[i+2]
    p1.direction == "out"
    p2.direction == "in"
    p3.direction == "out"
}

# Rule: Bidirectional port (inout) - often problematic in modern designs
bidirectional_port[violation] {
    port := input.ports[_]
    lower(port.direction) == "inout"
    violation := {
        "rule": "bidirectional_port",
        "severity": "info",
        "file": input.entities[_].file,
        "line": port.line,
        "message": sprintf("Port '%s' is bidirectional (inout) - consider separate in/out ports unless truly needed", [port.name])
    }
}

# Rule: Buffer port (deprecated)
buffer_port[violation] {
    port := input.ports[_]
    lower(port.direction) == "buffer"
    violation := {
        "rule": "buffer_port",
        "severity": "warning",
        "file": input.entities[_].file,
        "line": port.line,
        "message": sprintf("Port '%s' uses deprecated 'buffer' direction - use 'out' with internal signal instead", [port.name])
    }
}

# Rule: Architecture without any concurrent statements or processes
trivial_architecture[violation] {
    arch := input.architectures[_]
    procs := count([p | p := input.processes[_]; p.in_arch == arch.name; p.file == arch.file])
    concurrents := count([c | c := input.concurrent_assignments[_]; c.in_arch == arch.name; c.file == arch.file])
    instances := count([i | i := input.instances[_]; i.in_arch == arch.name; i.file == arch.file])
    procs == 0
    concurrents == 0
    instances == 0
    violation := {
        "rule": "trivial_architecture",
        "severity": "warning",
        "file": arch.file,
        "line": arch.line,
        "message": sprintf("Architecture '%s' has no processes, concurrent statements, or instances", [arch.name])
    }
}

# Aggregate quality violations
violations := buffer_port | trivial_architecture

# Optional violations (style preferences)
optional_violations := very_long_file | large_package | short_signal_name | long_signal_name | short_port_name | entity_name_with_numbers | mixed_port_directions | bidirectional_port
