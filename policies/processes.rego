# Process Analysis Rules
# Rules for process complexity and latch inference detection
package vhdl.processes

import data.vhdl.helpers

# Rule: Process assigns too many signals (complexity)
complex_process[violation] {
    proc := input.processes[_]
    count(proc.assigned_signals) > 20
    violation := {
        "rule": "complex_process",
        "severity": "info",
        "file": proc.file,
        "line": proc.line,
        "message": sprintf("Process '%s' assigns %d signals - consider splitting into smaller processes", [proc.label, count(proc.assigned_signals)])
    }
}

# Rule: Combinational process without default assignments (latch risk)
# A combinational process that assigns signals conditionally without defaults
# Note: This is a heuristic - proper check requires analyzing control flow
comb_process_no_default[violation] {
    proc := input.processes[_]
    proc.is_combinational == true
    count(proc.assigned_signals) > 0
    # If there are case statements without others, flag it
    cs := input.case_statements[_]
    cs.in_process == proc.label
    cs.has_others == false
    violation := {
        "rule": "comb_process_no_default",
        "severity": "warning",
        "file": proc.file,
        "line": proc.line,
        "message": sprintf("Combinational process '%s' has incomplete case statement - may infer latch", [proc.label])
    }
}

# Aggregate process violations
violations := complex_process | comb_process_no_default
