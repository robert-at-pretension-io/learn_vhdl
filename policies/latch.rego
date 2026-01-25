# Latch Inference Detection Rules
# Rules for detecting unintentional latch inference in combinational logic
#
# =============================================================================
# LATCH INFERENCE PRIMER
# =============================================================================
#
# A latch is inferred when a signal is not assigned a value in all branches
# of combinational logic. Unlike registers (flip-flops), latches are:
# - Level-sensitive (not edge-triggered)
# - Prone to timing issues (transparent when enabled)
# - Often unintentional in synthesized designs
#
# Common causes:
# 1. Incomplete case statements (missing 'when others =>')
# 2. Incomplete if-else chains (missing final 'else')
# 3. Signal assigned in some branches but not all
#
# This policy detects these patterns and warns the designer.
#
# =============================================================================
package vhdl.latch

import data.vhdl.helpers

# =============================================================================
# Rule 1: Incomplete Case Statement in Combinational Process
# =============================================================================
# A case statement without 'when others =>' in combinational logic will
# infer a latch for any signal assigned within it.

incomplete_case_latch[violation] {
    cs := input.case_statements[_]
    cs.has_others == false

    # Find the containing process
    proc := input.processes[_]
    proc.file == cs.file
    proc.label == cs.in_process
    proc.is_combinational == true

    violation := {
        "rule": "incomplete_case_latch",
        "severity": "warning",
        "file": cs.file,
        "line": cs.line,
        "message": sprintf("Case statement on '%s' in combinational process '%s' missing 'when others =>' - will infer latch", [cs.expression, cs.in_process])
    }
}

# Also catch case statements where we can't find the process (might be in architecture body)
incomplete_case_latch[violation] {
    cs := input.case_statements[_]
    cs.has_others == false
    cs.in_process == ""  # No containing process = concurrent statement area

    violation := {
        "rule": "incomplete_case_latch",
        "severity": "warning",
        "file": cs.file,
        "line": cs.line,
        "message": sprintf("Case statement on '%s' missing 'when others =>' - may infer latch", [cs.expression])
    }
}

# =============================================================================
# Rule 2: Enum Case Without Full Coverage
# =============================================================================
# If we have type information, we can check if all enum values are covered.
# This is more precise than just checking for 'others'.

enum_case_incomplete[violation] {
    cs := input.case_statements[_]
    cs.has_others == false

    # Find if the expression matches an enum type
    # The expression is typically a signal name - find its type
    sig := input.signals[_]
    lower(sig.name) == lower(cs.expression)

    # Find enum type definition
    enum_type := input.types[_]
    enum_type.kind == "enum"
    lower(enum_type.name) == lower(sig.type)

    # Check if all enum literals are covered
    covered := {lower(c) | c := cs.choices[_]}
    required := {lower(lit) | lit := enum_type.enum_literals[_]}
    missing := required - covered
    count(missing) > 0

    # Find the containing process to check if combinational
    proc := input.processes[_]
    proc.file == cs.file
    proc.label == cs.in_process
    proc.is_combinational == true

    violation := {
        "rule": "enum_case_incomplete",
        "severity": "error",
        "file": cs.file,
        "line": cs.line,
        "message": sprintf("Case statement on enum '%s' missing values %v in combinational process - will infer latch", [cs.expression, missing])
    }
}

# =============================================================================
# Rule 3: Combinational Process with Incomplete Assignments
# =============================================================================
# If a signal is assigned in some branches but not all, a latch is inferred.
# This is detected when a process assigns a signal AND reads it (feedback),
# which can indicate the signal holds its value in some paths.
#
# Note: This is a heuristic. For perfect detection, we'd need full control
# flow analysis of if-else branches, which requires deeper extraction.

combinational_incomplete_assignment[violation] {
    proc := input.processes[_]
    proc.is_combinational == true

    # Signal is both read and written in the process
    assigned := proc.assigned_signals[_]
    read := proc.read_signals[_]
    lower(assigned) == lower(read)

    # Exclude known patterns
    not helpers.is_clock_name(assigned)
    not helpers.is_reset_name(assigned)
    not is_next_state_pattern(assigned)
    helpers.is_actual_signal(assigned)

    violation := {
        "rule": "combinational_incomplete_assignment",
        "severity": "info",
        "file": proc.file,
        "line": proc.line,
        "message": sprintf("Signal '%s' in combinational process '%s' is read as well as written - verify all code paths assign it to avoid latch", [assigned, proc.label])
    }
}

# Helper: Detect "next_state" pattern where feedback is intentional
is_next_state_pattern(name) {
    contains(lower(name), "next")
}

# =============================================================================
# Rule 4: Concurrent Conditional Assignment Without Else
# =============================================================================
# Pattern: sig <= a when cond else <missing>
# This is harder to detect without deeper grammar support, but we can flag
# conditional assignments as potential latch sources.

conditional_assignment_check[violation] {
    ca := input.concurrent_assignments[_]
    ca.kind == "conditional"

    # Conditional assignments need an else clause
    # Without deep parsing, we flag all conditional assignments for review
    # This is a low-severity informational warning

    violation := {
        "rule": "conditional_assignment_review",
        "severity": "info",
        "file": ca.file,
        "line": ca.line,
        "message": sprintf("Conditional assignment to '%s' - verify all conditions have an 'else' clause to avoid latch inference", [ca.target])
    }
}

# =============================================================================
# Rule 5: Selected Assignment Without Others
# =============================================================================
# Pattern: with sel select sig <= a when "00", b when "01";
# Missing 'when others' will infer a latch.

selected_assignment_check[violation] {
    ca := input.concurrent_assignments[_]
    ca.kind == "selected"

    violation := {
        "rule": "selected_assignment_review",
        "severity": "info",
        "file": ca.file,
        "line": ca.line,
        "message": sprintf("Selected assignment to '%s' - verify 'when others' is present to avoid latch inference", [ca.target])
    }
}

# =============================================================================
# Rule 6: Combinational Process Without Default Assignments
# =============================================================================
# Best practice: Assign default values at the start of a combinational process
# to prevent latches. If a process has many assigned signals but no apparent
# pattern of default assignment, flag it.

many_signals_no_default[violation] {
    proc := input.processes[_]
    proc.is_combinational == true
    count(proc.assigned_signals) > 3

    # Check if any case statements in this process lack others
    cs := input.case_statements[_]
    cs.file == proc.file
    cs.in_process == proc.label
    cs.has_others == false

    violation := {
        "rule": "combinational_default_values",
        "severity": "info",
        "file": proc.file,
        "line": proc.line,
        "message": sprintf("Combinational process '%s' assigns %d signals - consider adding default values at process start to prevent latches", [proc.label, count(proc.assigned_signals)])
    }
}

# =============================================================================
# Rule 7: State Machine Without Reset (Related to Latch-like Behavior)
# =============================================================================
# A state machine signal that isn't reset will have undefined initial state,
# which is similar to latch-like uncertainty.

fsm_no_reset[violation] {
    proc := input.processes[_]
    proc.is_sequential == true
    proc.has_reset == false

    # Check if any assigned signal looks like a state signal
    assigned := proc.assigned_signals[_]
    helpers.is_state_name(assigned)

    violation := {
        "rule": "fsm_no_reset_state",
        "severity": "warning",
        "file": proc.file,
        "line": proc.line,
        "message": sprintf("State signal '%s' in process '%s' has no reset - initial state undefined", [assigned, proc.label])
    }
}

# =============================================================================
# Aggregate Violations
# =============================================================================

# High priority violations (actual latch inference)
violations := incomplete_case_latch | enum_case_incomplete

# Optional/informational violations
optional_violations := combinational_incomplete_assignment | conditional_assignment_check | selected_assignment_check | many_signals_no_default | fsm_no_reset
