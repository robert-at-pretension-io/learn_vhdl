# Combinational Logic Rules
# Rules for detecting issues in combinational processes
#
# =============================================================================
# POLICY PHILOSOPHY: FIX AT THE SOURCE
# =============================================================================
#
# If these rules produce false positives, the problem is almost never the rule.
#
# WRONG: "combinational_feedback fires on 'downto' - add it to skip list"
#        This means the grammar couldn't parse something, created an ERROR
#        node, and "downto" leaked through as a signal name.
#
# RIGHT: Fix grammar.js so the construct parses cleanly, no ERROR nodes,
#        and "downto" is recognized as a keyword, not a signal.
#
# Real combinational feedback IS a real problem. False positives mean our
# extraction is broken, not that we need to weaken the rule.
#
# See: AGENTS.md "The Grammar Improvement Cycle"
# =============================================================================
package vhdl.combinational

import data.vhdl.helpers

# Rule: Combinational process reads signal it assigns (potential combinational loop)
# This can cause simulation issues and synthesis problems
combinational_feedback[violation] {
    proc := input.processes[_]
    proc.is_combinational == true
    assigned := proc.assigned_signals[_]
    read := proc.read_signals[_]
    lower(assigned) == lower(read)
    not helpers.is_clock_name(assigned)
    not helpers.is_reset_name(assigned)
    # Filter out false positives: enum literals, constants, functions
    helpers.is_actual_signal(assigned)
    violation := {
        "rule": "combinational_feedback",
        "severity": "warning",
        "file": proc.file,
        "line": proc.line,
        "message": sprintf("Combinational process '%s' reads signal '%s' that it assigns - potential combinational loop", [proc.label, assigned])
    }
}

# Rule: Very large combinational process (complexity/timing risk)
large_combinational_process[violation] {
    proc := input.processes[_]
    proc.is_combinational == true
    read_count := count(proc.read_signals)
    assigned_count := count(proc.assigned_signals)
    total := read_count + assigned_count
    total > 30
    violation := {
        "rule": "large_combinational_process",
        "severity": "info",
        "file": proc.file,
        "line": proc.line,
        "message": sprintf("Large combinational process '%s' (%d signals) - may cause timing issues", [proc.label, total])
    }
}

# Rule: Combinational process with empty sensitivity list
# This is almost always a bug - the process will only execute once
empty_sensitivity_combinational[violation] {
    proc := input.processes[_]
    proc.is_combinational == true
    count(proc.sensitivity_list) == 0
    count(proc.assigned_signals) > 0
    violation := {
        "rule": "empty_sensitivity_combinational",
        "severity": "error",
        "file": proc.file,
        "line": proc.line,
        "message": sprintf("Combinational process '%s' has empty sensitivity list - will only execute once!", [proc.label])
    }
}

# Rule: Using 'all' in sensitivity list (VHDL-2008) - informational, actually good practice
vhdl2008_sensitivity_all[violation] {
    proc := input.processes[_]
    helpers.has_all_sensitivity(proc.sensitivity_list)
    violation := {
        "rule": "vhdl2008_sensitivity_all",
        "severity": "info",
        "file": proc.file,
        "line": proc.line,
        "message": sprintf("Process '%s' uses VHDL-2008 'all' sensitivity - good practice but requires VHDL-2008 support", [proc.label])
    }
}

# Rule: Long sensitivity list (should probably use 'all')
long_sensitivity_list[violation] {
    proc := input.processes[_]
    proc.is_combinational == true
    not helpers.has_all_sensitivity(proc.sensitivity_list)
    count(proc.sensitivity_list) > 8
    violation := {
        "rule": "long_sensitivity_list",
        "severity": "info",
        "file": proc.file,
        "line": proc.line,
        "message": sprintf("Process '%s' has %d signals in sensitivity list - consider using 'all' (VHDL-2008)", [proc.label, count(proc.sensitivity_list)])
    }
}

# =============================================================================
# Combinational Loop Detection - "Delta Cycle Death Spiral"
# =============================================================================
# Theory: A combinational loop exists when a signal depends on itself through
# a chain of combinational logic (no register in the path).
#
# Pattern: a <= b; b <= c; c <= a;  -- LOOP!
#
# Consequences:
# - Simulation: Infinite delta cycles, simulator hang/crash
# - Synthesis: Oscillation, unpredictable behavior, tool errors

# Rule: Direct combinational loop (signal depends on itself)
direct_combinational_loop[violation] {
    dep := input.signal_deps[_]
    dep.is_sequential == false
    dep.source == dep.target
    violation := {
        "rule": "direct_combinational_loop",
        "severity": "error",
        "file": dep.file,
        "line": dep.line,
        "message": sprintf("Direct combinational loop: signal '%s' depends on itself", [dep.source])
    }
}

# Rule: Two-stage combinational loop (A -> B -> A)
two_stage_loop[violation] {
    dep1 := input.signal_deps[_]
    dep1.is_sequential == false
    dep2 := input.signal_deps[_]
    dep2.is_sequential == false

    # A -> B and B -> A
    dep1.source == dep2.target
    dep1.target == dep2.source
    dep1.source != dep1.target  # Not a direct loop (caught above)

    # Only report once (ordering to avoid duplicates)
    dep1.line <= dep2.line
    dep1.source < dep2.source

    violation := {
        "rule": "two_stage_combinational_loop",
        "severity": "error",
        "file": dep1.file,
        "line": dep1.line,
        "message": sprintf("Combinational loop detected: '%s' -> '%s' -> '%s'", [dep1.source, dep1.target, dep1.source])
    }
}

# Rule: Three-stage combinational loop (A -> B -> C -> A)
three_stage_loop[violation] {
    dep1 := input.signal_deps[_]
    dep1.is_sequential == false
    dep2 := input.signal_deps[_]
    dep2.is_sequential == false
    dep3 := input.signal_deps[_]
    dep3.is_sequential == false

    # A -> B, B -> C, C -> A
    dep1.target == dep2.source
    dep2.target == dep3.source
    dep3.target == dep1.source

    # All different signals (not a shorter loop)
    dep1.source != dep2.source
    dep2.source != dep3.source
    dep1.source != dep3.source

    # Only report once
    dep1.source < dep2.source
    dep2.source < dep3.source

    violation := {
        "rule": "three_stage_combinational_loop",
        "severity": "error",
        "file": dep1.file,
        "line": dep1.line,
        "message": sprintf("Combinational loop detected: '%s' -> '%s' -> '%s' -> '%s'", [dep1.source, dep1.target, dep2.target, dep1.source])
    }
}

# Rule: Potential loop (signal read and written in same combinational process)
# This is a simpler heuristic that catches many loops
potential_comb_loop[violation] {
    proc := input.processes[_]
    proc.is_combinational == true
    assigned := proc.assigned_signals[_]
    read := proc.read_signals[_]
    lower(assigned) == lower(read)
    # Exclude common false positives
    not is_loop_false_positive(assigned)
    # Filter out enum literals, constants, functions
    helpers.is_actual_signal(assigned)
    violation := {
        "rule": "potential_combinational_loop",
        "severity": "warning",
        "file": proc.file,
        "line": proc.line,
        "message": sprintf("Potential combinational loop in process '%s': signal '%s' is both read and written", [proc.label, assigned])
    }
}

# Helper: Filter out common false positives
is_loop_false_positive(sig) {
    # Mux-style feedback is often intentional
    contains(lower(sig), "next")
}

is_loop_false_positive(sig) {
    # State machine feedback
    contains(lower(sig), "state")
}

# Rule: Cross-process combinational loop
# Signal written in one comb process, read in another that writes a signal read by the first
cross_process_loop[violation] {
    proc1 := input.processes[_]
    proc1.is_combinational == true
    proc2 := input.processes[_]
    proc2.is_combinational == true
    proc1.label != proc2.label

    # proc1 writes sig_a, reads sig_b
    # proc2 writes sig_b, reads sig_a
    sig_a := proc1.assigned_signals[_]
    sig_b := proc1.read_signals[_]
    sig_a2 := proc2.assigned_signals[_]
    sig_b2 := proc2.read_signals[_]

    lower(sig_a) == lower(sig_b2)  # proc1's output is proc2's input
    lower(sig_b) == lower(sig_a2)  # proc2's output is proc1's input

    # Filter out enum literals, constants, functions
    helpers.is_actual_signal(sig_a)
    helpers.is_actual_signal(sig_b)

    # Only report once
    proc1.line < proc2.line

    violation := {
        "rule": "cross_process_combinational_loop",
        "severity": "error",
        "file": proc1.file,
        "line": proc1.line,
        "message": sprintf("Cross-process combinational loop between '%s' and '%s' via signals '%s' and '%s'", [proc1.label, proc2.label, sig_a, sig_b])
    }
}

# Aggregate combinational violations
violations := combinational_feedback | empty_sensitivity_combinational | direct_combinational_loop | two_stage_loop | three_stage_loop | cross_process_loop

# Optional violations (informational)
optional_violations := large_combinational_process | vhdl2008_sensitivity_all | long_sensitivity_list | potential_comb_loop
