# Testbench-Specific Rules
# Rules for detecting issues in testbench code
package vhdl.testbench

import data.vhdl.helpers

# Rule: Entity looks like testbench (name contains tb/test) but has ports
# Testbenches typically have no ports
testbench_with_ports[violation] {
    entity := input.entities[_]
    is_testbench_name(entity.name)
    count(entity.ports) > 0
    violation := {
        "rule": "testbench_with_ports",
        "severity": "info",
        "file": entity.file,
        "line": entity.line,
        "message": sprintf("Entity '%s' looks like a testbench but has %d ports - testbenches typically have no ports", [entity.name, count(entity.ports)])
    }
}

# Helper: Check if name suggests testbench
is_testbench_name(name) {
    contains(lower(name), "_tb")
}
is_testbench_name(name) {
    contains(lower(name), "tb_")
}
is_testbench_name(name) {
    endswith(lower(name), "tb")
}
is_testbench_name(name) {
    contains(lower(name), "test")
}
is_testbench_name(name) {
    contains(lower(name), "bench")
}

# Rule: Non-testbench entity without ports (might be incomplete or error)
# Unless it's a package body or configuration
entity_no_ports_not_tb[violation] {
    entity := input.entities[_]
    count(entity.ports) == 0
    not is_testbench_name(entity.name)
    not is_package_name(entity.name)
    violation := {
        "rule": "entity_no_ports_not_tb",
        "severity": "warning",
        "file": entity.file,
        "line": entity.line,
        "message": sprintf("Entity '%s' has no ports but doesn't look like a testbench", [entity.name])
    }
}

# Helper: Check if name suggests package
is_package_name(name) {
    contains(lower(name), "_pkg")
}
is_package_name(name) {
    endswith(lower(name), "pkg")
}
is_package_name(name) {
    contains(lower(name), "package")
}

# Rule: Architecture named "testbench" or "tb" for non-testbench entity
mismatched_tb_architecture[violation] {
    arch := input.architectures[_]
    is_testbench_arch_name(arch.name)
    entity := input.entities[_]
    lower(entity.name) == lower(arch.entity_name)
    not is_testbench_name(entity.name)
    violation := {
        "rule": "mismatched_tb_architecture",
        "severity": "info",
        "file": arch.file,
        "line": arch.line,
        "message": sprintf("Architecture '%s' has testbench name but entity '%s' doesn't", [arch.name, arch.entity_name])
    }
}

# Helper: Check if architecture name suggests testbench
is_testbench_arch_name(name) {
    lower(name) == "tb"
}
is_testbench_arch_name(name) {
    lower(name) == "testbench"
}
is_testbench_arch_name(name) {
    lower(name) == "test"
}
is_testbench_arch_name(name) {
    lower(name) == "sim"
}

# Rule: Synthesis architecture name for testbench
# If entity is testbench but arch is "rtl" or "synth", might be wrong
tb_with_synth_arch[violation] {
    arch := input.architectures[_]
    entity := input.entities[_]
    lower(entity.name) == lower(arch.entity_name)
    is_testbench_name(entity.name)
    is_synthesis_arch_name(arch.name)
    violation := {
        "rule": "tb_with_synth_arch",
        "severity": "info",
        "file": arch.file,
        "line": arch.line,
        "message": sprintf("Testbench entity '%s' has synthesis-style architecture name '%s'", [entity.name, arch.name])
    }
}

# Helper: Check if architecture name suggests synthesis
is_synthesis_arch_name(name) {
    lower(name) == "rtl"
}
is_synthesis_arch_name(name) {
    lower(name) == "synth"
}
is_synthesis_arch_name(name) {
    lower(name) == "synthesis"
}

# Aggregate testbench violations
violations := entity_no_ports_not_tb

# Optional violations (mostly informational)
optional_violations := testbench_with_ports | mismatched_tb_architecture | tb_with_synth_arch
