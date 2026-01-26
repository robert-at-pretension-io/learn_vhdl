# Synthesis-Related Rules
# Rules for detecting potential synthesis issues
package vhdl.synthesis

import data.vhdl.helpers

# Rule: Multiple clock domains in same architecture without synchronizers
# This is a heuristic - if multiple different clocks are used, CDC might be needed
multiple_clock_domains[violation] {
    arch := input.architectures[_]
    procs := [p | p := input.processes[_]; p.file == arch.file; p.clock_signal != ""]
    clocks := {lower(p.clock_signal) | p := procs[_]}
    count(clocks) > 1
    clock_list := [c | c := clocks[_]]
    violation := {
        "rule": "multiple_clock_domains",
        "severity": "warning",
        "file": arch.file,
        "line": arch.line,
        "message": sprintf("Architecture '%s' uses multiple clocks %v - ensure proper CDC synchronization", [arch.name, clock_list])
    }
}

# Rule: Signal used in multiple clock domains (CDC risk)
signal_crosses_clock_domain[violation] {
    proc1 := input.processes[_]
    proc2 := input.processes[_]
    proc1.is_sequential == true
    proc2.is_sequential == true
    proc1.clock_signal != ""
    proc2.clock_signal != ""
    lower(proc1.clock_signal) != lower(proc2.clock_signal)
    proc1.file == proc2.file  # Same architecture
    # Signal assigned in one domain, read in another
    assigned := proc1.assigned_signals[_]
    read := proc2.read_signals[_]
    lower(assigned) == lower(read)
    violation := {
        "rule": "signal_crosses_clock_domain",
        "severity": "error",
        "file": proc1.file,
        "line": proc1.line,
        "message": sprintf("Signal '%s' written in '%s' domain, read in '%s' domain - needs synchronizer", [assigned, proc1.clock_signal, proc2.clock_signal])
    }
}

# Rule: Very wide bus without pipelining hint
# Wide buses (>64 bits) may have timing issues
very_wide_bus[violation] {
    sig := input.signals[_]
    width := extract_width(sig.type)
    width > 64
    violation := {
        "rule": "very_wide_bus",
        "severity": "info",
        "file": sig.file,
        "line": sig.line,
        "message": sprintf("Signal '%s' is %d bits wide - consider pipelining for timing closure", [sig.name, width])
    }
}

# Helper: Extract width from type
extract_width(type_str) = width {
    re_match("\\([0-9]+ downto 0\\)", lower(type_str))
    width := to_number(regex.find_all_string_submatch_n("([0-9]+) downto 0", lower(type_str), 1)[0][1]) + 1
} else = width {
    re_match("\\(0 to [0-9]+\\)", lower(type_str))
    width := to_number(regex.find_all_string_submatch_n("0 to ([0-9]+)", lower(type_str), 1)[0][1]) + 1
} else = 0

# Rule: Reset-less sequential logic for critical signals
# Signals with certain names should always have reset
critical_signal_no_reset[violation] {
    proc := input.processes[_]
    proc.is_sequential == true
    proc.has_reset == false
    assigned := proc.assigned_signals[_]
    is_critical_signal_name(assigned)
    violation := {
        "rule": "critical_signal_no_reset",
        "severity": "warning",
        "file": proc.file,
        "line": proc.line,
        "message": sprintf("Critical signal '%s' in process '%s' has no reset initialization", [assigned, proc.label])
    }
}

# Helper: Names that suggest critical signals
is_critical_signal_name(name) {
    contains(lower(name), "valid")
}
is_critical_signal_name(name) {
    contains(lower(name), "enable")
}
is_critical_signal_name(name) {
    contains(lower(name), "ready")
}
is_critical_signal_name(name) {
    contains(lower(name), "error")
}
is_critical_signal_name(name) {
    contains(lower(name), "state")
}
is_critical_signal_name(name) {
    contains(lower(name), "count")
}

# Rule: Potential gated clock
# If a clock-like signal is assigned in combinational logic, it might be a gated clock
gated_clock_detection[violation] {
    ca := input.concurrent_assignments[_]
    helpers.is_clock_name(ca.target)
    not is_testbench_arch(ca.in_arch)
    violation := {
        "rule": "gated_clock_detection",
        "severity": "warning",
        "file": ca.file,
        "line": ca.line,
        "message": sprintf("Clock signal '%s' assigned in concurrent statement - potential gated clock (use clock enable instead)", [ca.target])
    }
}

gated_clock_detection[violation] {
    proc := input.processes[_]
    proc.is_combinational == true
    assigned := proc.assigned_signals[_]
    helpers.is_clock_name(assigned)
    not helpers.process_in_testbench(proc)
    violation := {
        "rule": "gated_clock_detection",
        "severity": "warning",
        "file": proc.file,
        "line": proc.line,
        "message": sprintf("Clock signal '%s' assigned in combinational process - potential gated clock", [assigned])
    }
}

is_testbench_arch(arch_name) {
    arch := input.architectures[_]
    lower(arch.name) == lower(arch_name)
    helpers.is_testbench_name(arch.entity_name)
}

# Rule: Async reset generated combinationally
# Reset should typically come from a dedicated reset controller
combinational_reset[violation] {
    ca := input.concurrent_assignments[_]
    helpers.is_reset_name(ca.target)
    count(ca.read_signals) > 0  # Not a constant assignment
    violation := {
        "rule": "combinational_reset",
        "severity": "info",
        "file": ca.file,
        "line": ca.line,
        "message": sprintf("Reset signal '%s' generated combinationally - consider dedicated reset controller", [ca.target])
    }
}

# Rule: Potential inference of RAM/ROM
# Large array types might infer memory blocks
potential_memory_inference[violation] {
    sig := input.signals[_]
    # Check for array-like type names
    is_array_type(sig.type)
    violation := {
        "rule": "potential_memory_inference",
        "severity": "info",
        "file": sig.file,
        "line": sig.line,
        "message": sprintf("Signal '%s' with type '%s' may infer memory block - verify synthesis results", [sig.name, sig.type])
    }
}

# Helper: Check for array types
is_array_type(t) {
    contains(lower(t), "array")
}
is_array_type(t) {
    # Pattern: type(N downto 0)(M downto 0) - 2D array
    regex.match(".*\\)\\s*\\(.*\\)", t)
}

# Rule: Output port driven by combinational logic (timing risk)
# Unregistered outputs can cause timing closure issues
unregistered_output[violation] {
    port := input.ports[_]
    port.direction == "out"
    # Check if this output is driven by a sequential process
    not output_driven_by_sequential(port.name)
    # Must be driven by something (not just floating)
    output_is_driven(port.name)
    violation := {
        "rule": "unregistered_output",
        "severity": "warning",
        "file": get_entity_file(port.in_entity),
        "line": port.line,
        "message": sprintf("Output port '%s' is driven by combinational logic - consider registering for timing closure", [port.name])
    }
}

# Helper: Check if output is driven by sequential process
output_driven_by_sequential(port_name) {
    proc := input.processes[_]
    proc.is_sequential == true
    assigned := proc.assigned_signals[_]
    lower(assigned) == lower(port_name)
}

# Helper: Check if output is driven at all
output_is_driven(port_name) {
    proc := input.processes[_]
    assigned := proc.assigned_signals[_]
    lower(assigned) == lower(port_name)
}
output_is_driven(port_name) {
    ca := input.concurrent_assignments[_]
    lower(ca.target) == lower(port_name)
}

# Helper: Get file for entity
get_entity_file(entity_name) = file {
    entity := input.entities[_]
    lower(entity.name) == lower(entity_name)
    file := entity.file
} else = "unknown"

# Aggregate synthesis violations
violations := signal_crosses_clock_domain | gated_clock_detection | unregistered_output

# Optional violations
optional_violations := multiple_clock_domains | very_wide_bus | critical_signal_no_reset | combinational_reset | potential_memory_inference
