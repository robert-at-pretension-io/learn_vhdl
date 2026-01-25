# Clock and Reset Rules
# Rules for clock domain crossing, reset patterns, and clock/reset signal integrity
package vhdl.clocks_resets

import data.vhdl.helpers

# Rule: Clock signal should be std_logic (not std_logic_vector)
clock_not_std_logic[violation] {
    port := input.ports[_]
    helpers.is_clock_name(port.name)
    port.direction == "in"
    not helpers.is_single_bit_type(port.type)
    violation := {
        "rule": "clock_not_std_logic",
        "severity": "error",
        "file": input.entities[_].file,
        "line": port.line,
        "message": sprintf("Clock signal '%s' should be std_logic, not '%s'", [port.name, port.type])
    }
}

# Rule: Reset signal should be std_logic (not std_logic_vector)
reset_not_std_logic[violation] {
    port := input.ports[_]
    helpers.is_reset_name(port.name)
    port.direction == "in"
    not helpers.is_single_bit_type(port.type)
    violation := {
        "rule": "reset_not_std_logic",
        "severity": "error",
        "file": input.entities[_].file,
        "line": port.line,
        "message": sprintf("Reset signal '%s' should be std_logic, not '%s'", [port.name, port.type])
    }
}

# Rule: Multiple clock domains in same process (CDC issue)
# If a process references multiple different clock signals, it's suspicious
multiple_clocks_in_process[violation] {
    proc := input.processes[_]
    proc.is_sequential == true
    # Find all clock-like signals in sensitivity list
    clocks := [s | s := proc.sensitivity_list[_]; helpers.is_clock_name(s)]
    count(clocks) > 1
    violation := {
        "rule": "multiple_clocks_in_process",
        "severity": "error",
        "file": proc.file,
        "line": proc.line,
        "message": sprintf("Process '%s' appears to use multiple clocks %v - potential CDC issue", [proc.label, clocks])
    }
}

# Rule: Missing reset in sequential process (power-on state unknown)
missing_reset[violation] {
    proc := input.processes[_]
    proc.is_sequential == true
    proc.has_reset == false
    count(proc.assigned_signals) > 0  # Only flag if it assigns something
    violation := {
        "rule": "missing_reset",
        "severity": "warning",
        "file": proc.file,
        "line": proc.line,
        "message": sprintf("Sequential process '%s' has no reset - power-on state will be unknown", [proc.label])
    }
}

# Rule: Async reset should be active-low (convention: rstn, rst_n)
# Disabled by default - project-specific convention
async_reset_active_high[violation] {
    proc := input.processes[_]
    proc.is_sequential == true
    proc.has_reset == true
    proc.reset_signal != ""
    # Check if reset name suggests active-high (no 'n' suffix)
    not endswith(lower(proc.reset_signal), "n")
    not endswith(lower(proc.reset_signal), "_n")
    not contains(lower(proc.reset_signal), "_n_")
    violation := {
        "rule": "async_reset_active_high",
        "severity": "info",
        "file": proc.file,
        "line": proc.line,
        "message": sprintf("Reset '%s' in process '%s' may be active-high - consider using active-low reset (rstn, rst_n)", [proc.reset_signal, proc.label])
    }
}

# Aggregate clock/reset violations (excluding optional rules)
violations := clock_not_std_logic | reset_not_std_logic | multiple_clocks_in_process | missing_reset

# Optional violations (project-specific)
optional_violations := async_reset_active_high
