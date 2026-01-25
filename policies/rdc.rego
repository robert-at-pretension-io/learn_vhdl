# Reset Domain Crossing (RDC) Analysis - "The Silent Killer"
# Detects asynchronous reset synchronization issues
package vhdl.rdc

import data.vhdl.helpers

# =============================================================================
# Reset Domain Crossing Detection
# =============================================================================
# Theory: Just like Clock Domain Crossings (CDC), Reset Domain Crossings (RDC)
# require proper synchronization. An unsynchronized async reset release can
# cause metastability and partial state corruption.
#
# The dangerous pattern:
#   1. Async reset goes low (deasserted)
#   2. Some flip-flops see the release on this clock edge
#   3. Other flip-flops see it on the next edge
#   4. State machine corruption, data corruption, system failure
#
# Solution: Reset must be synchronized to the clock domain before distribution

# Rule: Async reset not synchronized
# Pattern: Async reset directly used without going through a synchronizer
async_reset_unsynchronized[violation] {
    proc := input.processes[_]
    proc.is_sequential == true
    proc.has_reset == true
    proc.reset_signal != ""

    # Check if reset is async (in sensitivity list along with clock)
    is_async := helpers.signal_in_list(proc.reset_signal, proc.sensitivity_list)
    is_async == true

    # Check if there's a synchronizer for this reset
    not has_reset_synchronizer(proc.reset_signal, proc.clock_signal)

    violation := {
        "rule": "async_reset_unsynchronized",
        "severity": "warning",
        "file": proc.file,
        "line": proc.line,
        "message": sprintf("Async reset '%s' used directly in process '%s' - needs synchronization to '%s' clock domain", [proc.reset_signal, proc.label, proc.clock_signal])
    }
}

# Helper: Check if there's a synchronizer for the reset
# A synchronizer is a sequential process that:
# 1. Has the same clock
# 2. Has the reset as an input
# 3. Has a 2+ stage pipeline output
has_reset_synchronizer(reset_sig, clock_sig) {
    proc := input.processes[_]
    proc.is_sequential == true
    proc.clock_signal == clock_sig
    # Process reads the reset signal
    sig := proc.read_signals[_]
    lower(sig) == lower(reset_sig)
    # And produces a synchronized version (common patterns)
    assigned := proc.assigned_signals[_]
    is_sync_name(assigned, reset_sig)
}

# Helper: Check if signal name suggests it's a synchronized version
is_sync_name(assigned, reset_sig) {
    patterns := ["_sync", "_synced", "_meta", "_d1", "_d2", "_ff1", "_ff2"]
    pattern := patterns[_]
    contains(lower(assigned), lower(reset_sig))
    contains(lower(assigned), pattern)
}

is_sync_name(assigned, reset_sig) {
    # Or the assigned signal is "reset_sync", "rst_sync_n", etc.
    contains(lower(assigned), "sync")
    helpers.is_reset_name(assigned)
}

# Rule: Reset used across multiple clock domains
# Each clock domain should have its own synchronized reset
reset_crosses_domains[violation] {
    # Find a reset signal
    proc1 := input.processes[_]
    proc1.has_reset == true
    proc1.reset_signal != ""
    reset_sig := proc1.reset_signal

    # Find another process with the same reset but different clock
    proc2 := input.processes[_]
    proc2.has_reset == true
    proc2.reset_signal == reset_sig
    proc1.clock_signal != proc2.clock_signal
    proc1.clock_signal != ""
    proc2.clock_signal != ""

    # Only report once (proc1 < proc2 by some ordering)
    proc1.line < proc2.line

    violation := {
        "rule": "reset_crosses_domains",
        "severity": "error",
        "file": proc1.file,
        "line": proc1.line,
        "message": sprintf("Reset '%s' used in multiple clock domains ('%s' and '%s') - each domain needs synchronized reset", [reset_sig, proc1.clock_signal, proc2.clock_signal])
    }
}

# Rule: Async reset on some registers but not others in same domain
# Partial reset can leave state machine in inconsistent state
partial_reset_domain[violation] {
    proc1 := input.processes[_]
    proc1.is_sequential == true
    proc1.has_reset == true
    proc1.clock_signal != ""

    proc2 := input.processes[_]
    proc2.is_sequential == true
    proc2.has_reset == false
    proc2.clock_signal == proc1.clock_signal
    proc1.file == proc2.file  # Same file (likely same design unit)

    count(proc2.assigned_signals) > 0

    violation := {
        "rule": "partial_reset_domain",
        "severity": "warning",
        "file": proc2.file,
        "line": proc2.line,
        "message": sprintf("Process '%s' in clock domain '%s' has no reset, but other processes in same domain do - potential state inconsistency", [proc2.label, proc2.clock_signal])
    }
}

# Rule: Reset glitch potential
# Combinational logic generating reset signal
combinational_reset_gen[violation] {
    # Find a signal that's used as a reset
    proc := input.processes[_]
    proc.has_reset == true
    reset_sig := proc.reset_signal

    # Check if it's driven by combinational logic
    ca := input.concurrent_assignments[_]
    lower(ca.target) == lower(reset_sig)
    count(ca.read_signals) > 1  # Multiple inputs = combinational logic

    violation := {
        "rule": "combinational_reset_gen",
        "severity": "error",
        "file": ca.file,
        "line": ca.line,
        "message": sprintf("Reset signal '%s' generated by combinational logic - prone to glitches", [reset_sig])
    }
}

# Rule: Very short reset synchronizer (less than 2 stages)
# Standard practice is 2-3 flip-flop stages for proper metastability resolution
short_reset_sync[violation] {
    # Look for signals that appear to be reset synchronizers
    proc := input.processes[_]
    proc.is_sequential == true

    # Check for 1-stage synchronizer pattern
    assigned := proc.assigned_signals[_]
    contains(lower(assigned), "sync")
    helpers.is_reset_name(assigned)

    # Check it's a single flip-flop (only one _sync stage, no _sync2/_sync_d2)
    not has_second_stage(proc.assigned_signals, assigned)

    violation := {
        "rule": "short_reset_sync",
        "severity": "warning",
        "file": proc.file,
        "line": proc.line,
        "message": sprintf("Reset synchronizer '%s' appears to be single-stage - use 2+ stages for metastability", [assigned])
    }
}

# Helper: Check for second synchronizer stage
has_second_stage(signals, first_stage) {
    sig := signals[_]
    sig != first_stage
    contains(lower(sig), "sync")
    # Pattern: signal_sync2, signal_sync_d2, etc.
    contains(lower(sig), "2")
}

has_second_stage(signals, first_stage) {
    sig := signals[_]
    sig != first_stage
    # Pattern: signal_sync, signal_sync_meta (two stages)
    contains(lower(sig), "meta")
}

# =============================================================================
# Aggregate violations
# =============================================================================

violations := reset_crosses_domains | combinational_reset_gen

# Optional violations (may have false positives)
optional_violations := async_reset_unsynchronized | partial_reset_domain | short_reset_sync
