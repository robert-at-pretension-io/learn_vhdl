# Security Analysis Rules - "Ghost in the Machine"
# Detects potential hardware trojans and suspicious trigger patterns
package vhdl.security

import data.vhdl.helpers

# =============================================================================
# Hardware Trojan Detection
# =============================================================================
# Theory: Hardware trojans often use "magic numbers" - comparisons against
# specific large literals that act as kill switches or backdoors.
#
# Pattern: if counter = X"DEADBEEF" then secret_output <= '1';
#
# The trigger probability is often astronomically low (1 in 2^32) making
# it nearly impossible to hit during testing, but trivial for an attacker
# who knows the magic value.

# Rule: Large literal comparison (potential trojan trigger)
# Comparisons against literals > 16 bits are suspicious in most designs
large_literal_comparison[violation] {
    comp := input.comparisons[_]
    comp.is_literal == true
    comp.literal_bits > 16
    violation := {
        "rule": "large_literal_comparison",
        "severity": "warning",
        "file": comp.file,
        "line": comp.line,
        "message": sprintf("Suspicious comparison: '%s' %s literal '%s' (%d bits) - potential trojan trigger", [comp.left_operand, comp.operator, comp.literal_value, comp.literal_bits])
    }
}

# Rule: Magic number comparison (known suspicious patterns)
# Common "signature" values used in trojans
magic_number_comparison[violation] {
    comp := input.comparisons[_]
    comp.is_literal == true
    is_magic_number(comp.literal_value)
    violation := {
        "rule": "magic_number_comparison",
        "severity": "error",
        "file": comp.file,
        "line": comp.line,
        "message": sprintf("CRITICAL: Comparison against known magic number '%s' - HIGH PROBABILITY TROJAN TRIGGER", [comp.literal_value])
    }
}

# Helper: Check for known magic numbers
is_magic_number(lit) {
    magic_patterns := [
        "DEADBEEF", "CAFEBABE", "FEEDFACE", "BAADF00D",
        "DEADC0DE", "C0FFEE00", "BADC0DE0", "0BADF00D",
        "DEAD", "BEEF", "CAFE", "BABE", "FEED", "FACE"
    ]
    upper_lit := upper(lit)
    pattern := magic_patterns[_]
    contains(upper_lit, pattern)
}

# Rule: Equality comparison that drives output
# Pattern: if input = X"..." then output_signal <= value;
# This is the classic trojan pattern - a trigger condition driving a payload
trigger_drives_output[violation] {
    comp := input.comparisons[_]
    comp.is_literal == true
    comp.literal_bits > 8
    comp.operator == "="
    comp.result_drives != ""
    # Check if result_drives is an output port
    port := input.ports[_]
    lower(port.name) == lower(comp.result_drives)
    port.direction == "out"
    violation := {
        "rule": "trigger_drives_output",
        "severity": "error",
        "file": comp.file,
        "line": comp.line,
        "message": sprintf("ALERT: Literal comparison '%s' = '%s' drives output port '%s' - classic trojan pattern", [comp.left_operand, comp.literal_value, comp.result_drives])
    }
}

# Rule: Counter comparison (potential time bomb)
# Counters compared against specific values can be time bombs
counter_trigger[violation] {
    comp := input.comparisons[_]
    comp.is_literal == true
    comp.literal_bits >= 16
    helpers.is_counter_name(comp.left_operand)
    violation := {
        "rule": "counter_trigger",
        "severity": "warning",
        "file": comp.file,
        "line": comp.line,
        "message": sprintf("Counter '%s' compared against large literal '%s' - potential time bomb trigger", [comp.left_operand, comp.literal_value])
    }
}

# Rule: Rare comparison operators that could hide triggers
# Using /= (not equal) with magic numbers: "do everything except when X"
inverted_trigger[violation] {
    comp := input.comparisons[_]
    comp.is_literal == true
    comp.literal_bits > 16
    comp.operator == "/="
    violation := {
        "rule": "inverted_trigger",
        "severity": "warning",
        "file": comp.file,
        "line": comp.line,
        "message": sprintf("Inverted comparison '/=' against large literal '%s' - could hide trojan by inverting trigger logic", [comp.literal_value])
    }
}

# Rule: Multiple magic comparisons in same process
# Multiple triggers in one place is very suspicious
multi_trigger_process[violation] {
    proc := input.processes[_]
    comps := [c | c := input.comparisons[_]; c.in_process == proc.label; c.is_literal == true; c.literal_bits > 12]
    count(comps) > 2
    violation := {
        "rule": "multi_trigger_process",
        "severity": "error",
        "file": proc.file,
        "line": proc.line,
        "message": sprintf("Process '%s' contains %d large literal comparisons - suspicious concentration of potential triggers", [proc.label, count(comps)])
    }
}

# =============================================================================
# Aggregate violations
# =============================================================================

violations := magic_number_comparison | trigger_drives_output | multi_trigger_process

# Optional/informational violations
optional_violations := large_literal_comparison | counter_trigger | inverted_trigger
