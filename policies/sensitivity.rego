# Sensitivity List Rules
# Rules for detecting simulation/synthesis mismatches due to incomplete sensitivity lists
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
