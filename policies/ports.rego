# Port Analysis Rules
# Rules for port usage: unused inputs, undriven outputs
# Now includes concurrent assignment tracking for accurate analysis
package vhdl.ports

import data.vhdl.helpers

# Rule: Input port never read (unused input)
unused_input_port[violation] {
    port := input.ports[_]
    port.direction == "in"
    not helpers.is_clock_name(port.name)
    not helpers.is_reset_name(port.name)
    not port_is_read(port.name, port.in_entity)
    violation := {
        "rule": "unused_input_port",
        "severity": "warning",
        "file": input.entities[_].file,
        "line": port.line,
        "message": sprintf("Input port '%s' is never read", [port.name])
    }
}

# Helper: Check if port is read (in process, sensitivity list, instance, or concurrent)
port_is_read(port_name, entity_name) {
    proc := input.processes[_]
    sig := proc.read_signals[_]
    lower(sig) == lower(port_name)
}
port_is_read(port_name, entity_name) {
    proc := input.processes[_]
    sig := proc.sensitivity_list[_]
    lower(sig) == lower(port_name)
}
port_is_read(port_name, entity_name) {
    # Read via instance port map actual
    inst := input.instances[_]
    actual := inst.port_map[_]
    contains(lower(actual), lower(port_name))
}
port_is_read(port_name, entity_name) {
    # Read in concurrent assignment
    ca := input.concurrent_assignments[_]
    sig := ca.read_signals[_]
    lower(sig) == lower(port_name)
}

# Rule: Output port never assigned (undriven output)
undriven_output_port[violation] {
    port := input.ports[_]
    port.direction == "out"
    not port_is_assigned(port.name, port.in_entity)
    violation := {
        "rule": "undriven_output_port",
        "severity": "error",
        "file": input.entities[_].file,
        "line": port.line,
        "message": sprintf("Output port '%s' is never assigned (floating output)", [port.name])
    }
}

# Helper: Check if port is assigned (in process, instance, or concurrent)
port_is_assigned(port_name, entity_name) {
    proc := input.processes[_]
    sig := proc.assigned_signals[_]
    lower(sig) == lower(port_name)
}
port_is_assigned(port_name, entity_name) {
    # Assigned via instance port map formal
    inst := input.instances[_]
    formal := inst.port_map[key]
    lower(formal) == lower(port_name)
}
port_is_assigned(port_name, entity_name) {
    # Assigned in concurrent assignment
    ca := input.concurrent_assignments[_]
    lower(ca.target) == lower(port_name)
}

# NOW ENABLED - concurrent assignment extraction makes these accurate
violations := unused_input_port | undriven_output_port
