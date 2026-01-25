# FSM (Finite State Machine) Analysis Rules
# Rules for detecting state machine issues
package vhdl.fsm

import data.vhdl.helpers

# Rule: State signal with suspicious type (should be enumerated)
# If a signal named "state" or "*_state" is std_logic_vector, it's likely a poorly designed FSM
state_signal_not_enum[violation] {
    sig := input.signals[_]
    is_state_signal_name(sig.name)
    is_vector_type(sig.type)
    violation := {
        "rule": "state_signal_not_enum",
        "severity": "warning",
        "file": sig.file,
        "line": sig.line,
        "message": sprintf("State signal '%s' uses vector type '%s' - consider using enumerated type for clarity", [sig.name, sig.type])
    }
}

# Helper: Check if signal name suggests FSM state
is_state_signal_name(name) {
    lower(name) == "state"
}
is_state_signal_name(name) {
    endswith(lower(name), "_state")
}
is_state_signal_name(name) {
    startswith(lower(name), "state_")
}
is_state_signal_name(name) {
    lower(name) == "current_state"
}
is_state_signal_name(name) {
    lower(name) == "next_state"
}
is_state_signal_name(name) {
    lower(name) == "present_state"
}

# Helper: Check if type is a vector
is_vector_type(t) {
    contains(lower(t), "vector")
}
is_vector_type(t) {
    contains(lower(t), "unsigned")
}
is_vector_type(t) {
    contains(lower(t), "signed")
}

# Rule: FSM with only one state signal (missing next_state)
# Good FSM design has separate current_state and next_state signals
single_state_signal[violation] {
    sig := input.signals[_]
    lower(sig.name) == "state"
    not has_next_state_signal(sig.in_entity)
    violation := {
        "rule": "single_state_signal",
        "severity": "info",
        "file": sig.file,
        "line": sig.line,
        "message": sprintf("Signal 'state' found without 'next_state' - consider two-process FSM style", [])
    }
}

# Helper: Check if entity has a next_state signal
has_next_state_signal(entity_name) {
    sig := input.signals[_]
    sig.in_entity == entity_name
    is_next_state_name(sig.name)
}

is_next_state_name(name) {
    lower(name) == "next_state"
}
is_next_state_name(name) {
    lower(name) == "nextstate"
}
is_next_state_name(name) {
    lower(name) == "nxt_state"
}

# Rule: State machine case statement without when others (covered by core, but specific message)
fsm_missing_default_state[violation] {
    cs := input.case_statements[_]
    is_state_expression(cs.expression)
    cs.has_others == false
    violation := {
        "rule": "fsm_missing_default_state",
        "severity": "error",
        "file": cs.file,
        "line": cs.line,
        "message": sprintf("FSM case statement on '%s' missing 'when others' - undefined behavior for invalid states", [cs.expression])
    }
}

# Helper: Check if expression looks like a state variable
is_state_expression(expr) {
    is_state_signal_name(expr)
}

# Aggregate FSM violations
violations := state_signal_not_enum | fsm_missing_default_state

# Optional violations
optional_violations := single_state_signal
