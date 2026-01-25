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

# Rule: State enum literal never appears in FSM case statement
# If an enum state is defined but never handled in the case statement, it's likely unreachable or missing
fsm_unhandled_state[violation] {
    # Find enum type declarations that look like state types
    type_decl := input.types[_]
    type_decl.kind == "enum"
    is_state_type_name(type_decl.name)

    # Get all enum literals for this type
    state_literal := type_decl.enum_literals[_]

    # Find case statements that use this state type (by checking if any choice matches an enum literal)
    cs := input.case_statements[_]
    is_state_expression(cs.expression)
    case_uses_this_type(cs, type_decl.enum_literals)

    # Check if this particular state literal appears in the choices
    not state_in_choices(state_literal, cs.choices)
    not cs.has_others  # If there's "when others", all states are technically handled

    violation := {
        "rule": "fsm_unhandled_state",
        "severity": "warning",
        "file": cs.file,
        "line": cs.line,
        "message": sprintf("FSM state '%s' from type '%s' not explicitly handled in case statement", [state_literal, type_decl.name])
    }
}

# Helper: Check if type name looks like a state type
is_state_type_name(name) {
    contains(lower(name), "state")
}
is_state_type_name(name) {
    endswith(lower(name), "_t")
    contains(lower(name), "st")
}
is_state_type_name(name) {
    endswith(lower(name), "_type")
}

# Helper: Check if case statement uses this enum type (at least one choice matches)
case_uses_this_type(cs, enum_literals) {
    choice := cs.choices[_]
    literal := enum_literals[_]
    lower(choice) == lower(literal)
}

# Helper: Check if state literal appears in choices
state_in_choices(state, choices) {
    choice := choices[_]
    lower(choice) == lower(state)
}

# Rule: FSM state signal never assigned to certain state value
# If a state value is never assigned, it's unreachable
fsm_unreachable_state[violation] {
    # Find state type
    type_decl := input.types[_]
    type_decl.kind == "enum"
    is_state_type_name(type_decl.name)

    # Get all enum literals
    state_literal := type_decl.enum_literals[_]

    # Find state signals of this type
    sig := input.signals[_]
    is_state_signal_name(sig.name)

    # Check if any process assigns this state value
    not state_ever_assigned(sig.name, state_literal)

    # Exclude first state (often default/reset state that might only be set in reset)
    state_literal != type_decl.enum_literals[0]

    violation := {
        "rule": "fsm_unreachable_state",
        "severity": "warning",
        "file": sig.file,
        "line": sig.line,
        "message": sprintf("FSM state '%s' is never assigned to '%s' - potentially unreachable", [sig.name, state_literal])
    }
}

# Helper: Check if state signal is ever assigned to a specific state value
# This is a heuristic - we check if the literal appears in process assignments
state_ever_assigned(sig_name, state_literal) {
    proc := input.processes[_]
    assigned := proc.assigned_signals[_]
    lower(assigned) == lower(sig_name)
    # Check if state literal appears in reads (as it would be on RHS of assignment)
    read := proc.read_signals[_]
    lower(read) == lower(state_literal)
}

# Note: For proper transition analysis, we would need to extract state transitions
# from case alternatives. This is a simplified heuristic based on current data.

# Aggregate FSM violations
violations := state_signal_not_enum | fsm_missing_default_state | fsm_unhandled_state

# Optional violations
optional_violations := single_state_signal | fsm_unreachable_state
