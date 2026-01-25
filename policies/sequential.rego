# Sequential Logic Rules
# Rules for detecting issues in clocked processes
#
# =============================================================================
# POLICY PHILOSOPHY: THESE RULES CHECK REAL HARDWARE BUGS
# =============================================================================
#
# Sequential logic rules catch actual FPGA/ASIC problems:
# - Missing clock in sensitivity list = simulation/synthesis mismatch
# - Missing reset = undefined initial state
# - Signal in both sequential and combinational = potential race condition
#
# If these rules fire false positives, the problem is upstream:
# - Grammar can't parse the process correctly (ERROR nodes)
# - Extractor misidentifies clock/reset patterns
# - Signal read/write analysis is incomplete
#
# DON'T weaken these rules to reduce false positives.
# DO fix the grammar and extractor so extraction is accurate.
#
# See: AGENTS.md "The Grammar Improvement Cycle"
# =============================================================================
package vhdl.sequential

import data.vhdl.helpers

# Rule: Sequential process without clock in sensitivity list
# This is almost always a bug
missing_clock_sensitivity[violation] {
    proc := input.processes[_]
    proc.is_sequential == true
    proc.clock_signal != ""
    not helpers.has_all_sensitivity(proc.sensitivity_list)
    not clock_in_sensitivity(proc.clock_signal, proc.sensitivity_list)
    violation := {
        "rule": "missing_clock_sensitivity",
        "severity": "error",
        "file": proc.file,
        "line": proc.line,
        "message": sprintf("Sequential process '%s' uses clock '%s' but it's not in sensitivity list", [proc.label, proc.clock_signal])
    }
}

# Helper: Check if clock is in sensitivity list
clock_in_sensitivity(clk, sens) {
    s := sens[_]
    lower(s) == lower(clk)
}

# Rule: Async reset process without reset in sensitivity list
missing_reset_sensitivity[violation] {
    proc := input.processes[_]
    proc.is_sequential == true
    proc.has_reset == true
    proc.reset_signal != ""
    not helpers.has_all_sensitivity(proc.sensitivity_list)
    not reset_in_sensitivity(proc.reset_signal, proc.sensitivity_list)
    violation := {
        "rule": "missing_reset_sensitivity",
        "severity": "warning",
        "file": proc.file,
        "line": proc.line,
        "message": sprintf("Process '%s' uses reset '%s' but it's not in sensitivity list (sync reset?)", [proc.label, proc.reset_signal])
    }
}

# Helper: Check if reset is in sensitivity list
reset_in_sensitivity(rst, sens) {
    s := sens[_]
    lower(s) == lower(rst)
}

# Rule: Sequential process assigns to too many signals
# This often indicates a process that should be split
very_wide_register[violation] {
    proc := input.processes[_]
    proc.is_sequential == true
    count(proc.assigned_signals) > 15
    violation := {
        "rule": "very_wide_register",
        "severity": "info",
        "file": proc.file,
        "line": proc.line,
        "message": sprintf("Sequential process '%s' assigns %d signals - consider splitting for clarity", [proc.label, count(proc.assigned_signals)])
    }
}

# Rule: Different signals clocked on different edges in same entity
# This can cause timing issues
mixed_edge_clocking[violation] {
    proc1 := input.processes[i]
    proc2 := input.processes[j]
    i < j
    proc1.is_sequential == true
    proc2.is_sequential == true
    proc1.clock_signal != ""
    proc2.clock_signal != ""
    lower(proc1.clock_signal) == lower(proc2.clock_signal)
    proc1.clock_edge != proc2.clock_edge
    proc1.file == proc2.file  # Same file
    violation := {
        "rule": "mixed_edge_clocking",
        "severity": "warning",
        "file": proc1.file,
        "line": proc1.line,
        "message": sprintf("Processes '%s' (%s edge) and '%s' (%s edge) use same clock '%s' with different edges", [proc1.label, proc1.clock_edge, proc2.label, proc2.clock_edge, proc1.clock_signal])
    }
}

# Rule: Signal assigned in both sequential and combinational process
# This is a common mistake that causes simulation/synthesis mismatch
signal_in_seq_and_comb[violation] {
    proc_seq := input.processes[_]
    proc_comb := input.processes[_]
    proc_seq.is_sequential == true
    proc_comb.is_combinational == true
    proc_seq.file == proc_comb.file  # Same architecture
    assigned_seq := proc_seq.assigned_signals[_]
    assigned_comb := proc_comb.assigned_signals[_]
    lower(assigned_seq) == lower(assigned_comb)
    # Filter out false positives: enum literals, constants, functions
    helpers.is_actual_signal(assigned_seq)
    violation := {
        "rule": "signal_in_seq_and_comb",
        "severity": "error",
        "file": proc_seq.file,
        "line": proc_seq.line,
        "message": sprintf("Signal '%s' assigned in both sequential process '%s' and combinational process '%s'", [assigned_seq, proc_seq.label, proc_comb.label])
    }
}

# Rule: Async reset without active-low naming convention
# Convention: active-low resets should be named *_n, *n, rstn
async_reset_naming[violation] {
    proc := input.processes[_]
    proc.is_sequential == true
    proc.has_reset == true
    proc.reset_signal != ""
    not is_active_low_reset_name(proc.reset_signal)
    violation := {
        "rule": "async_reset_naming",
        "severity": "info",
        "file": proc.file,
        "line": proc.line,
        "message": sprintf("Reset signal '%s' doesn't follow active-low naming convention (*_n, *n)", [proc.reset_signal])
    }
}

# Helper: Check for active-low reset naming
is_active_low_reset_name(name) {
    endswith(lower(name), "_n")
}
is_active_low_reset_name(name) {
    endswith(lower(name), "n")
    helpers.is_reset_name(name)
}

# Aggregate sequential violations
violations := missing_clock_sensitivity | signal_in_seq_and_comb

# Optional violations
optional_violations := missing_reset_sensitivity | very_wide_register | mixed_edge_clocking | async_reset_naming
