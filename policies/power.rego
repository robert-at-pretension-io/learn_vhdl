# Power Analysis Rules - "Power Vampire"
# Detects expensive operations that lack operand isolation
package vhdl.power

import data.vhdl.helpers

# =============================================================================
# Operand Isolation Analysis
# =============================================================================
# Theory: In hardware, multipliers and dividers consume power EVERY clock cycle,
# even when their outputs aren't used. Operand isolation means gating the inputs
# with an enable signal so they don't toggle when the result isn't needed.
#
# Bad pattern (power vampire):
#   result <= a * b;  -- Multiplier runs every cycle!
#
# Good pattern (operand isolation):
#   if enable = '1' then
#     result <= a * b;  -- Only runs when needed
#   end if;
#
# Or better (proper gating):
#   a_gated <= a when enable = '1' else (others => '0');
#   b_gated <= b when enable = '1' else (others => '0');
#   result <= a_gated * b_gated;

# Rule: Unguarded multiplication (power vampire)
unguarded_multiplication[violation] {
    arith := input.arithmetic_ops[_]
    arith.operator == "*"
    arith.is_guarded == false
    count(arith.operands) >= 2
    violation := {
        "rule": "unguarded_multiplication",
        "severity": "warning",
        "file": arith.file,
        "line": arith.line,
        "message": sprintf("Multiplier without operand isolation - runs every cycle even when unused. Guard with enable signal.", [])
    }
}

# Rule: Unguarded division (power vampire - EXPENSIVE)
# Division is extremely expensive in hardware
unguarded_division[violation] {
    arith := input.arithmetic_ops[_]
    is_division_op(arith.operator)
    arith.is_guarded == false
    violation := {
        "rule": "unguarded_division",
        "severity": "error",
        "file": arith.file,
        "line": arith.line,
        "message": sprintf("Division/modulo operator '%s' without operand isolation - VERY expensive, runs every cycle!", [arith.operator])
    }
}

# Helper: Check for division-type operators
is_division_op(op) {
    division_ops := ["/", "mod", "rem"]
    lower(op) == division_ops[_]
}

# Rule: Exponentiation without guard (can be multiplier chain)
unguarded_exponent[violation] {
    arith := input.arithmetic_ops[_]
    arith.operator == "**"
    arith.is_guarded == false
    violation := {
        "rule": "unguarded_exponent",
        "severity": "warning",
        "file": arith.file,
        "line": arith.line,
        "message": "Exponentiation '**' without operand isolation - implement with proper enable gating"
    }
}

# Rule: Multiple expensive operations in same process
# Concentrations of expensive ops = power hotspot
power_hotspot[violation] {
    proc := input.processes[_]

    # Count expensive operations in this process
    expensive_ops := [op |
        op := input.arithmetic_ops[_]
        op.in_process == proc.label
        is_expensive_op(op.operator)
    ]
    total := count(expensive_ops)
    total > 3

    violation := {
        "rule": "power_hotspot",
        "severity": "warning",
        "file": proc.file,
        "line": proc.line,
        "message": sprintf("Process '%s' contains %d expensive operations - power hotspot, consider operand isolation", [proc.label, total])
    }
}

# Helper: Check for expensive operations
is_expensive_op(op) {
    expensive := ["*", "/", "mod", "rem", "**"]
    lower(op) == expensive[_]
}

# Rule: Multiplier in combinational process (always active)
# Sequential processes at least only compute on clock edges
combinational_multiplier[violation] {
    arith := input.arithmetic_ops[_]
    arith.operator == "*"

    # Find the process
    proc := input.processes[_]
    proc.label == arith.in_process
    proc.is_combinational == true

    violation := {
        "rule": "combinational_multiplier",
        "severity": "warning",
        "file": arith.file,
        "line": arith.line,
        "message": "Multiplier in combinational process - active continuously, consider clocked implementation with enable"
    }
}

# Rule: Guarded operation with weak guard
# Pattern: if valid then result <= a * b;
# But 'valid' might not be properly gating the inputs
weak_guard[violation] {
    arith := input.arithmetic_ops[_]
    arith.is_guarded == true
    arith.guard_signal != ""
    is_expensive_op(arith.operator)

    # Check if guard signal is a common "always true" pattern
    is_weak_guard(arith.guard_signal)

    violation := {
        "rule": "weak_guard",
        "severity": "info",
        "file": arith.file,
        "line": arith.line,
        "message": sprintf("Expensive operation guarded by '%s' - verify this actually gates operand toggling", [arith.guard_signal])
    }
}

# Helper: Common weak guards
is_weak_guard(sig) {
    weak_patterns := ["enable", "en", "valid", "vld", "ready", "rdy"]
    # These COULD be weak if they're mostly asserted
    # This is just informational - real analysis needs runtime data
    pattern := weak_patterns[_]
    lower(sig) == pattern
}

# Rule: DSP block candidate without proper control
# Large multiplications should use DSP blocks with enable
dsp_candidate_no_control[violation] {
    arith := input.arithmetic_ops[_]
    arith.operator == "*"
    count(arith.operands) == 2
    arith.is_guarded == false

    # Check if operands are likely wide signals (from signal declarations)
    op := arith.operands[0]
    sig := input.signals[_]
    lower(sig.name) == lower(op)
    is_wide_type(sig.type)

    violation := {
        "rule": "dsp_candidate_no_control",
        "severity": "info",
        "file": arith.file,
        "line": arith.line,
        "message": sprintf("Wide signal '%s' multiplication - likely DSP block, add clock enable for power savings", [op])
    }
}

# Helper: Check for wide types
is_wide_type(t) {
    wide_patterns := ["unsigned", "signed", "std_logic_vector"]
    pattern := wide_patterns[_]
    contains(lower(t), pattern)
}

# =============================================================================
# Clock Gating Opportunities
# =============================================================================

# Rule: Sequential process with enable that could use clock gating
clock_gating_opportunity[violation] {
    proc := input.processes[_]
    proc.is_sequential == true
    count(proc.assigned_signals) > 5  # Multiple registers

    # Has an enable-like signal in read list
    read := proc.read_signals[_]
    is_enable_signal(read)

    violation := {
        "rule": "clock_gating_opportunity",
        "severity": "info",
        "file": proc.file,
        "line": proc.line,
        "message": sprintf("Process '%s' has %d registers with enable '%s' - candidate for clock gating", [proc.label, count(proc.assigned_signals), read])
    }
}

# Helper: Check for enable-like signals
is_enable_signal(sig) {
    enable_patterns := ["enable", "en", "ce", "clken", "clk_en", "clock_enable"]
    pattern := enable_patterns[_]
    contains(lower(sig), pattern)
}

# =============================================================================
# Aggregate violations
# =============================================================================

violations := unguarded_division

# Optional violations (design-dependent, may have false positives)
optional_violations := unguarded_multiplication | unguarded_exponent | power_hotspot | combinational_multiplier | weak_guard | dsp_candidate_no_control | clock_gating_opportunity
