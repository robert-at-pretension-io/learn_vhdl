# Signal Analysis Rules
# Rules for signal usage: unused, undriven, multi-driven signals
# Now includes concurrent assignment tracking for accurate analysis
package vhdl.signals

import data.vhdl.helpers

# Rule: Signal declared but never used (dead code)
unused_signal[violation] {
    sig := input.signals[_]
    not signal_is_used(sig.name, sig.in_entity)
    violation := {
        "rule": "unused_signal",
        "severity": "warning",
        "file": sig.file,
        "line": sig.line,
        "message": sprintf("Signal '%s' is declared but never used", [sig.name])
    }
}

# Helper: Check if signal is used in any process or concurrent statement
signal_is_used(sig_name, entity_name) {
    proc := input.processes[_]
    sig := proc.read_signals[_]
    lower(sig) == lower(sig_name)
}
signal_is_used(sig_name, entity_name) {
    proc := input.processes[_]
    sig := proc.assigned_signals[_]
    lower(sig) == lower(sig_name)
}
signal_is_used(sig_name, entity_name) {
    # Used in instance port map
    inst := input.instances[_]
    actual := inst.port_map[_]
    contains(lower(actual), lower(sig_name))
}
signal_is_used(sig_name, entity_name) {
    # Read in concurrent assignment
    ca := input.concurrent_assignments[_]
    sig := ca.read_signals[_]
    lower(sig) == lower(sig_name)
}
signal_is_used(sig_name, entity_name) {
    # Assigned in concurrent assignment
    ca := input.concurrent_assignments[_]
    lower(ca.target) == lower(sig_name)
}

# Rule: Signal read but never assigned (undriven)
undriven_signal[violation] {
    sig := input.signals[_]
    signal_is_read(sig.name)
    not signal_is_assigned(sig.name)
    violation := {
        "rule": "undriven_signal",
        "severity": "error",
        "file": sig.file,
        "line": sig.line,
        "message": sprintf("Signal '%s' is read but never assigned (undriven)", [sig.name])
    }
}

# Helper: Check if signal is read (in process or concurrent)
signal_is_read(sig_name) {
    proc := input.processes[_]
    sig := proc.read_signals[_]
    lower(sig) == lower(sig_name)
}
signal_is_read(sig_name) {
    ca := input.concurrent_assignments[_]
    sig := ca.read_signals[_]
    lower(sig) == lower(sig_name)
}

# Helper: Check if signal is assigned (in process or concurrent)
signal_is_assigned(sig_name) {
    proc := input.processes[_]
    sig := proc.assigned_signals[_]
    lower(sig) == lower(sig_name)
}
signal_is_assigned(sig_name) {
    ca := input.concurrent_assignments[_]
    lower(ca.target) == lower(sig_name)
}

# Rule: Signal assigned in multiple places (potential multi-driver)
# Now checks both processes and concurrent assignments
multi_driven_signal[violation] {
    sig := input.signals[_]
    driver_count := count_drivers(sig.name)
    driver_count > 1
    violation := {
        "rule": "multi_driven_signal",
        "severity": "error",
        "file": sig.file,
        "line": sig.line,
        "message": sprintf("Signal '%s' is assigned in %d places (potential multi-driver)", [sig.name, driver_count])
    }
}

# Helper: Count total drivers for a signal
count_drivers(sig_name) = total {
    proc_drivers := count([p | p := input.processes[_]; sig_assigned_in_process(sig_name, p)])
    conc_drivers := count([ca | ca := input.concurrent_assignments[_]; lower(ca.target) == lower(sig_name)])
    total := proc_drivers + conc_drivers
}

# Helper: Check if signal is assigned in a specific process
sig_assigned_in_process(sig_name, proc) {
    assigned := proc.assigned_signals[_]
    lower(assigned) == lower(sig_name)
}

# Rule: Very wide signal (potential performance issue)
wide_signal[violation] {
    sig := input.signals[_]
    width := extract_vector_width(sig.type)
    width > 128
    violation := {
        "rule": "wide_signal",
        "severity": "info",
        "file": sig.file,
        "line": sig.line,
        "message": sprintf("Signal '%s' is %d bits wide - consider if this width is necessary", [sig.name, width])
    }
}

# Helper: Extract width from std_logic_vector type
# This is a simplified check - looks for common patterns
extract_vector_width(type_str) = width {
    # Pattern: std_logic_vector(N downto 0) -> width = N+1
    re_match("\\([0-9]+ downto 0\\)", lower(type_str))
    matches := regex.find_n("([0-9]+) downto 0", lower(type_str), 1)
    count(matches) > 0
    width := to_number(regex.find_all_string_submatch_n("([0-9]+) downto 0", lower(type_str), 1)[0][1]) + 1
} else = width {
    # Pattern: std_logic_vector(0 to N) -> width = N+1
    re_match("\\(0 to [0-9]+\\)", lower(type_str))
    width := to_number(regex.find_all_string_submatch_n("0 to ([0-9]+)", lower(type_str), 1)[0][1]) + 1
} else = 0

# Duplicate signal names across entities (potential confusion)
duplicate_signal_name[violation] {
    sig1 := input.signals[i]
    sig2 := input.signals[j]
    i < j
    lower(sig1.name) == lower(sig2.name)
    sig1.in_entity != sig2.in_entity
    # Common names like 'clk', 'rst', 'data' are expected - skip them
    not helpers.is_common_signal_name(sig1.name)
    violation := {
        "rule": "duplicate_signal_name",
        "severity": "info",
        "file": sig1.file,
        "line": sig1.line,
        "message": sprintf("Signal '%s' also exists in entity '%s' - verify intentional", [sig1.name, sig2.in_entity])
    }
}

# NOW ENABLED - concurrent assignment extraction makes these accurate
violations := unused_signal | undriven_signal | multi_driven_signal

# Additional optional rules
optional_violations := wide_signal | duplicate_signal_name
