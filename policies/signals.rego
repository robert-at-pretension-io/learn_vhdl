# Signal Analysis Rules
# Rules for signal usage: unused, undriven, multi-driven signals
# Now includes concurrent assignment tracking for accurate analysis
#
# =============================================================================
# POLICY PHILOSOPHY: RULES SHOULD BE SIMPLE, GRAMMAR SHOULD BE RICH
# =============================================================================
#
# If you're adding complex filtering logic here (skip lists, keyword checks,
# pattern matching to work around bad data), STOP and ask:
#
# "Is this a GRAMMAR or EXTRACTOR bug?"
#
# FALSE POSITIVES usually mean:
# - Grammar has ERROR nodes → keywords leak into signal lists
# - Extractor misses a construct → signals appear unread/unassigned
#
# DON'T: Add "downto", "others", "when" to skip lists here
# DO:    Fix grammar.js so these never appear as signal names
#
# The helpers in helpers.rego (is_enum_literal, is_constant, is_skip_name)
# are for LEGITIMATE filtering (enum literals, constants, loop variables),
# NOT for working around parsing bugs.
#
# See: AGENTS.md "The Grammar Improvement Cycle"
# =============================================================================
package vhdl.signals

import data.vhdl.helpers

# Helper: Check if name is an enum literal (not a signal)
is_enum_literal(name) {
    lit := input.enum_literals[_]
    lower(lit) == lower(name)
}

# Helper: Check if name is a constant (not a signal)
is_constant(name) {
    c := input.constants[_]
    lower(c) == lower(name)
}

# Helper: Check if name is actually a signal (not enum/constant)
is_actual_signal(name) {
    not is_enum_literal(name)
    not is_constant(name)
}

# Helper: Check if name is declared anywhere (signal/port/type/subprogram/etc)
is_declared_identifier(name) {
    sig := input.signals[_]
    lower(sig.name) == lower(name)
}
is_declared_identifier(name) {
    port := input.ports[_]
    lower(port.name) == lower(name)
}
is_declared_identifier(name) {
    c := input.constants[_]
    lower(c) == lower(name)
}
is_declared_identifier(name) {
    lit := input.enum_literals[_]
    lower(lit) == lower(name)
}
is_declared_identifier(name) {
    t := input.types[_]
    lower(t.name) == lower(name)
}
is_declared_identifier(name) {
    st := input.subtypes[_]
    lower(st.name) == lower(name)
}
is_declared_identifier(name) {
    fn := input.functions[_]
    lower(fn.name) == lower(name)
}
is_declared_identifier(name) {
    pr := input.procedures[_]
    lower(pr.name) == lower(name)
}

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
# Excludes enum literals and constants from being treated as signals
signal_is_used(sig_name, entity_name) {
    proc := input.processes[_]
    sig := proc.read_signals[_]
    lower(sig) == lower(sig_name)
    is_actual_signal(sig)
}
signal_is_used(sig_name, entity_name) {
    proc := input.processes[_]
    sig := proc.assigned_signals[_]
    lower(sig) == lower(sig_name)
    is_actual_signal(sig)
}
signal_is_used(sig_name, entity_name) {
    # Used in instance port map
    inst := input.instances[_]
    actual := inst.port_map[_]
    contains(lower(actual), lower(sig_name))
    is_actual_signal(sig_name)
}
signal_is_used(sig_name, entity_name) {
    # Read in concurrent assignment
    ca := input.concurrent_assignments[_]
    sig := ca.read_signals[_]
    lower(sig) == lower(sig_name)
    is_actual_signal(sig)
}
signal_is_used(sig_name, entity_name) {
    # Assigned in concurrent assignment
    ca := input.concurrent_assignments[_]
    lower(ca.target) == lower(sig_name)
    is_actual_signal(ca.target)
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
# Excludes enum literals and constants from being treated as signals
signal_is_read(sig_name) {
    proc := input.processes[_]
    sig := proc.read_signals[_]
    lower(sig) == lower(sig_name)
    is_actual_signal(sig)
}
signal_is_read(sig_name) {
    ca := input.concurrent_assignments[_]
    sig := ca.read_signals[_]
    lower(sig) == lower(sig_name)
    is_actual_signal(sig)
}

# Helper: Check if signal is assigned (in process or concurrent)
# Excludes enum literals and constants from being treated as signals
signal_is_assigned(sig_name) {
    proc := input.processes[_]
    sig := proc.assigned_signals[_]
    lower(sig) == lower(sig_name)
    is_actual_signal(sig)
}
signal_is_assigned(sig_name) {
    ca := input.concurrent_assignments[_]
    lower(ca.target) == lower(sig_name)
    is_actual_signal(ca.target)
}
signal_is_assigned(sig_name) {
    # Signal may be driven by component output port in port map
    usage := input.signal_usages[_]
    usage.in_port_map == true
    lower(usage.signal) == lower(sig_name)
}

# Rule: Signal assigned in multiple places (potential multi-driver)
# Now checks both processes and concurrent assignments
# Severity: warning (not error) because record field assignments create false positives
multi_driven_signal[violation] {
    sig := input.signals[_]
    driver_count := count_drivers(sig.name)
    driver_count > 1
    violation := {
        "rule": "multi_driven_signal",
        "severity": "warning",
        "file": sig.file,
        "line": sig.line,
        "message": sprintf("Signal '%s' is assigned in %d places (review for multi-driver)", [sig.name, driver_count])
    }
}

# Rule: Signal or port used but not declared
undeclared_signal_usage[violation] {
    proc := input.processes[_]
    name := proc.read_signals[_]
    not helpers.is_skip_name(name)
    not is_declared_identifier(name)
    violation := {
        "rule": "undeclared_signal_usage",
        "severity": "warning",
        "file": proc.file,
        "line": proc.line,
        "message": sprintf("Signal '%s' is read but not declared in this design unit", [name])
    }
}
undeclared_signal_usage[violation] {
    proc := input.processes[_]
    name := proc.assigned_signals[_]
    not helpers.is_skip_name(name)
    not is_declared_identifier(name)
    violation := {
        "rule": "undeclared_signal_usage",
        "severity": "warning",
        "file": proc.file,
        "line": proc.line,
        "message": sprintf("Signal '%s' is assigned but not declared in this design unit", [name])
    }
}
undeclared_signal_usage[violation] {
    ca := input.concurrent_assignments[_]
    name := ca.read_signals[_]
    not helpers.is_skip_name(name)
    not is_declared_identifier(name)
    violation := {
        "rule": "undeclared_signal_usage",
        "severity": "warning",
        "file": ca.file,
        "line": ca.line,
        "message": sprintf("Signal '%s' is read but not declared in this design unit", [name])
    }
}
undeclared_signal_usage[violation] {
    ca := input.concurrent_assignments[_]
    name := ca.target
    not helpers.is_skip_name(name)
    not is_declared_identifier(name)
    violation := {
        "rule": "undeclared_signal_usage",
        "severity": "warning",
        "file": ca.file,
        "line": ca.line,
        "message": sprintf("Signal '%s' is assigned but not declared in this design unit", [name])
    }
}

# Rule: Input port driven inside architecture (illegal)
input_port_driven[violation] {
    port := input.ports[_]
    lower(port.direction) == "in"
    proc := input.processes[_]
    assigned := proc.assigned_signals[_]
    lower(assigned) == lower(port.name)
    violation := {
        "rule": "input_port_driven",
        "severity": "error",
        "file": proc.file,
        "line": proc.line,
        "message": sprintf("Input port '%s' is assigned in process '%s' (illegal driver)", [port.name, proc.label])
    }
}
input_port_driven[violation] {
    port := input.ports[_]
    lower(port.direction) == "in"
    ca := input.concurrent_assignments[_]
    lower(ca.target) == lower(port.name)
    violation := {
        "rule": "input_port_driven",
        "severity": "error",
        "file": ca.file,
        "line": ca.line,
        "message": sprintf("Input port '%s' is driven by concurrent assignment (illegal driver)", [port.name])
    }
}

# Helper: Count total drivers for a signal
# Assignments inside the same generate block count as 1 driver (not N)
count_drivers(sig_name) = total {
    proc_drivers := count([p | p := input.processes[_]; sig_assigned_in_process(sig_name, p)])
    # Count non-generate concurrent assignments
    non_gen_drivers := count([ca |
        ca := input.concurrent_assignments[_]
        lower(ca.target) == lower(sig_name)
        not ca.in_generate
    ])
    # Count distinct generate blocks that assign this signal (each block = 1 driver)
    gen_drivers := count({label |
        ca := input.concurrent_assignments[_]
        lower(ca.target) == lower(sig_name)
        ca.in_generate
        label := ca.generate_label
    })
    total := proc_drivers + non_gen_drivers + gen_drivers
}

# Helper: Check if signal is assigned in a specific process
# Excludes enum literals and constants
sig_assigned_in_process(sig_name, proc) {
    assigned := proc.assigned_signals[_]
    lower(assigned) == lower(sig_name)
    is_actual_signal(assigned)
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
violations := unused_signal | undriven_signal | multi_driven_signal | undeclared_signal_usage | input_port_driven

# Additional optional rules
optional_violations := wide_signal | duplicate_signal_name
