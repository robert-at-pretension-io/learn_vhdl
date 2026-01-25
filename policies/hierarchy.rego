# Hierarchy and Instantiation Rules
# Rules for detecting issues with component instantiation and hierarchy
package vhdl.hierarchy

import data.vhdl.helpers

# Rule: Instance with very few port connections
# If an instance has significantly fewer connections than expected, might be missing some
sparse_port_map[violation] {
    inst := input.instances[_]
    count(inst.port_map) > 0
    count(inst.port_map) < 3
    violation := {
        "rule": "sparse_port_map",
        "severity": "info",
        "file": inst.file,
        "line": inst.line,
        "message": sprintf("Instance '%s' has only %d port connections - verify all required ports are connected", [inst.name, count(inst.port_map)])
    }
}

# Rule: Instance without any port map (might be positional or error)
empty_port_map[violation] {
    inst := input.instances[_]
    count(inst.port_map) == 0
    violation := {
        "rule": "empty_port_map",
        "severity": "warning",
        "file": inst.file,
        "line": inst.line,
        "message": sprintf("Instance '%s' has no named port map - using positional mapping or no connections", [inst.name])
    }
}

# Rule: Instance name same as component name (confusion risk)
instance_name_matches_component[violation] {
    inst := input.instances[_]
    # Extract component name from target (might be lib.component)
    parts := split(inst.target, ".")
    comp_name := parts[count(parts) - 1]
    lower(inst.name) == lower(comp_name)
    violation := {
        "rule": "instance_name_matches_component",
        "severity": "info",
        "file": inst.file,
        "line": inst.line,
        "message": sprintf("Instance name '%s' matches component name - consider a unique instance name", [inst.name])
    }
}

# Rule: Very deep hierarchy (heuristic: same component instantiated multiple times in same file)
# This might indicate overly complex design
repeated_component_instantiation[violation] {
    inst1 := input.instances[i]
    inst2 := input.instances[j]
    i < j
    inst1.file == inst2.file
    lower(inst1.target) == lower(inst2.target)
    inst1.target != ""
    count_same_component := count([x | x := input.instances[_]; x.file == inst1.file; lower(x.target) == lower(inst1.target)])
    count_same_component > 5
    i == 0  # Only report once per component type
    violation := {
        "rule": "repeated_component_instantiation",
        "severity": "info",
        "file": inst1.file,
        "line": inst1.line,
        "message": sprintf("Component '%s' instantiated %d times - consider generate statement or hierarchical design", [inst1.target, count_same_component])
    }
}

# Rule: Very many instances in single architecture
many_instances[violation] {
    arch := input.architectures[_]
    instances_in_arch := [i | i := input.instances[_]; i.in_arch == arch.name; i.file == arch.file]
    count(instances_in_arch) > 20
    violation := {
        "rule": "many_instances",
        "severity": "info",
        "file": arch.file,
        "line": arch.line,
        "message": sprintf("Architecture '%s' has %d instances - consider hierarchical decomposition", [arch.name, count(instances_in_arch)])
    }
}

# Rule: Port connected to literal instead of signal
# Example: port_map(enable => '1') - hardcoded value
hardcoded_port_value[violation] {
    inst := input.instances[_]
    formal := inst.port_map[port_name]
    is_literal_value(formal)
    violation := {
        "rule": "hardcoded_port_value",
        "severity": "info",
        "file": inst.file,
        "line": inst.line,
        "message": sprintf("Instance '%s' has hardcoded value '%s' on port '%s' - consider using a constant/signal", [inst.name, formal, port_name])
    }
}

# Helper: Check if value is a literal
is_literal_value(val) {
    startswith(val, "'")  # Character literal
}
is_literal_value(val) {
    startswith(val, "\"")  # String literal
}
is_literal_value(val) {
    regex.match("^[0-9]+$", val)  # Number
}
is_literal_value(val) {
    lower(val) == "open"  # Open port
}

# Rule: Open port connection (might be intentional, but worth noting)
open_port_connection[violation] {
    inst := input.instances[_]
    formal := inst.port_map[port_name]
    lower(formal) == "open"
    violation := {
        "rule": "open_port_connection",
        "severity": "info",
        "file": inst.file,
        "line": inst.line,
        "message": sprintf("Instance '%s' has 'open' connection on port '%s'", [inst.name, port_name])
    }
}

# Rule: Input port of instantiated entity not connected
# This catches missing connections that will cause synthesis errors or undefined behavior
floating_instance_input[violation] {
    inst := input.instances[_]
    # Find the entity being instantiated
    entity := input.entities[_]
    inst_target_lower := lower(inst.target)
    # Match either "entity_name" or "lib.entity_name"
    target_matches_entity(inst_target_lower, lower(entity.name))
    # Find input ports of that entity
    port := entity.ports[_]
    port.direction == "in"
    # Check if this port is connected in the instance
    not port_connected_in_instance(inst, port.name)
    # Exclude clock and reset (often connected at top level or generated)
    not helpers.is_clock_name(port.name)
    not helpers.is_reset_name(port.name)
    violation := {
        "rule": "floating_instance_input",
        "severity": "error",
        "file": inst.file,
        "line": inst.line,
        "message": sprintf("Instance '%s' has unconnected input port '%s' from entity '%s'", [inst.name, port.name, entity.name])
    }
}

# Helper: Check if instance target matches entity name
target_matches_entity(target, entity_name) {
    target == entity_name
}
target_matches_entity(target, entity_name) {
    endswith(target, concat(".", [entity_name]))
}

# Helper: Check if port is connected in instance port map
port_connected_in_instance(inst, port_name) {
    _ = inst.port_map[port_name]
}
port_connected_in_instance(inst, port_name) {
    # Also check case-insensitive
    key := inst.port_map[k]
    lower(k) == lower(port_name)
}

# Aggregate hierarchy violations
violations := empty_port_map | floating_instance_input

# Optional violations (mostly informational)
optional_violations := sparse_port_map | instance_name_matches_component | repeated_component_instantiation | many_instances | hardcoded_port_value | open_port_connection
