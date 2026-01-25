# Sensitivity List Rules
# Rules for detecting simulation/synthesis mismatches due to incomplete sensitivity lists
#
# =============================================================================
# POLICY PHILOSOPHY: SENSITIVITY LIST ACCURACY DEPENDS ON EXTRACTION
# =============================================================================
#
# These rules compare process sensitivity lists against extracted read_signals.
# False positives here usually mean EXTRACTION is wrong, not the rule.
#
# COMMON FALSE POSITIVE PATTERNS:
#
# 1. "Signal 'downto' read but missing from sensitivity list"
#    → Grammar ERROR node; "downto" keyword leaked into signal list
#    → FIX: Improve grammar.js to parse the failing construct
#
# 2. "Signal 'ctrl_i' read but missing from sensitivity list"
#    → Grammar can't parse `ctrl_i.field`, creates ERROR, misses the read
#    → FIX: Improve grammar.js to handle selected_name in that context
#
# 3. "Signal 'x' in sensitivity but never read"
#    → Extractor misses read in complex expression (aggregate, generate)
#    → FIX: Update extractor to handle that expression type
#
# The helpers.is_skip_name() function filters LEGITIMATE false positives
# (constants, loop vars, types). Don't abuse it to work around parsing bugs!
#
# See: AGENTS.md "The Grammar Improvement Cycle"
# =============================================================================
package vhdl.sensitivity

import data.vhdl.helpers

# Rule: Sensitivity list incomplete (sim/synth mismatch)
# Signal read in combinational process but missing from sensitivity list
sensitivity_list_incomplete[violation] {
    proc := input.processes[_]
    proc.is_combinational == true
    not helpers.has_all_sensitivity(proc.sensitivity_list)  # VHDL-2008 'all' covers everything
    read_sig := proc.read_signals[_]
    not helpers.sig_in_sensitivity(read_sig, proc.sensitivity_list)
    not helpers.is_skip_name(read_sig)  # Skip constants, loop vars, types
    violation := {
        "rule": "sensitivity_list_incomplete",
        "severity": "error",
        "file": proc.file,
        "line": proc.line,
        "message": sprintf("Signal '%s' read in combinational process '%s' but missing from sensitivity list", [read_sig, proc.label])
    }
}

# Rule: Sensitivity list superfluous (simulation slowdown)
# Signal in sensitivity list but never read
sensitivity_list_superfluous[violation] {
    proc := input.processes[_]
    proc.is_combinational == true
    sens_sig := proc.sensitivity_list[_]
    sens_sig != "all"
    not helpers.sig_in_reads(sens_sig, proc.read_signals)
    violation := {
        "rule": "sensitivity_list_superfluous",
        "severity": "info",
        "file": proc.file,
        "line": proc.line,
        "message": sprintf("Signal '%s' in sensitivity list but never read in process '%s'", [sens_sig, proc.label])
    }
}

# Aggregate sensitivity violations
violations := sensitivity_list_incomplete | sensitivity_list_superfluous
