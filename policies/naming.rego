# Naming Convention Rules
# Rules for signal, port, entity, and variable naming conventions
# NOTE: Most rules are disabled by default as they are project-specific
package vhdl.naming

import data.vhdl.helpers

# Rule: Naming convention - entities should use lowercase (info level)
entity_naming[violation] {
    entity := input.entities[_]
    entity.name != lower(entity.name)
    violation := {
        "rule": "naming_convention",
        "severity": "info",
        "file": entity.file,
        "line": entity.line,
        "message": sprintf("Entity '%s' should use lowercase naming", [entity.name])
    }
}

# Rule: Signal naming - inputs should end with _i
signal_input_naming[violation] {
    port := input.ports[_]
    port.direction == "in"
    not endswith(lower(port.name), "_i")
    not helpers.is_clock_name(port.name)
    not helpers.is_reset_name(port.name)
    violation := {
        "rule": "signal_input_naming",
        "severity": "info",
        "file": input.entities[_].file,
        "line": port.line,
        "message": sprintf("Input port '%s' should end with '_i' suffix", [port.name])
    }
}

# Rule: Signal naming - outputs should end with _o
signal_output_naming[violation] {
    port := input.ports[_]
    port.direction == "out"
    not endswith(lower(port.name), "_o")
    violation := {
        "rule": "signal_output_naming",
        "severity": "info",
        "file": input.entities[_].file,
        "line": port.line,
        "message": sprintf("Output port '%s' should end with '_o' suffix", [port.name])
    }
}

# Rule: Active-low signal naming (should end with _n)
active_low_naming[violation] {
    sig := input.signals[_]
    is_active_low_name(sig.name)
    not endswith(lower(sig.name), "_n")
    not endswith(lower(sig.name), "n")
    violation := {
        "rule": "active_low_naming",
        "severity": "info",
        "file": sig.file,
        "line": sig.line,
        "message": sprintf("Active-low signal '%s' should end with '_n' suffix", [sig.name])
    }
}

# Helper: Check if name suggests active-low
is_active_low_name(name) {
    contains(lower(name), "not_")
}
is_active_low_name(name) {
    startswith(lower(name), "n_")
}

# All naming violations disabled by default (project-specific)
violations := set()

# Optional violations (for projects that follow these conventions)
optional_violations := entity_naming | signal_input_naming | signal_output_naming | active_low_naming
