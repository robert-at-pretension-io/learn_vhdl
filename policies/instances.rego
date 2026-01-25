# Instance Analysis Rules
# Rules for component instantiation: naming, port mapping
package vhdl.instances

import data.vhdl.helpers

# Rule: Positional port mapping (safety risk if ports reordered)
# Named mapping: port_map(clk => sys_clk) vs positional: port_map(sys_clk)
positional_mapping[violation] {
    inst := input.instances[_]
    count(inst.port_map) == 0  # Empty map means positional or no connections
    violation := {
        "rule": "positional_mapping",
        "severity": "warning",
        "file": inst.file,
        "line": inst.line,
        "message": sprintf("Instance '%s' uses positional port mapping - use named mapping for safety", [inst.name])
    }
}

# Rule: Instance naming convention (should start with u_, i_, or inst_)
# Disabled by default - project-specific convention
instance_naming_convention[violation] {
    inst := input.instances[_]
    not helpers.valid_instance_prefix(inst.name)
    violation := {
        "rule": "instance_naming_convention",
        "severity": "info",
        "file": inst.file,
        "line": inst.line,
        "message": sprintf("Instance '%s' should use a standard prefix (u_, i_, or inst_)", [inst.name])
    }
}

# Aggregate instance violations
violations := positional_mapping

# Optional violations (project-specific)
optional_violations := instance_naming_convention
