use crate::policy::input::Input;
use crate::policy::result::Violation;

pub fn violations(_input: &Input) -> Vec<Violation> {
    Vec::new()
}

pub fn optional_violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(state_signal_not_enum(input));
    out.extend(single_state_signal(input));
    out.extend(fsm_unreachable_state(input));
    out.extend(fsm_missing_default_state(input));
    out.extend(fsm_unhandled_state(input));
    out
}

fn state_signal_not_enum(input: &Input) -> Vec<Violation> {
    input
        .signals
        .iter()
        .filter(|sig| is_state_signal_name(&sig.name))
        .filter(|sig| is_vector_type(&sig.r#type))
        .map(|sig| Violation {
            rule: "state_signal_not_enum".to_string(),
            severity: "warning".to_string(),
            file: sig.file.clone(),
            line: sig.line,
            message: format!(
                "State signal '{}' uses vector type '{}' - consider using enumerated type for clarity",
                sig.name, sig.r#type
            ),
        })
        .collect()
}

fn single_state_signal(input: &Input) -> Vec<Violation> {
    input
        .signals
        .iter()
        .filter(|sig| sig.name.eq_ignore_ascii_case("state"))
        .filter(|sig| !has_next_state_signal(input, &sig.in_entity))
        .map(|sig| Violation {
            rule: "single_state_signal".to_string(),
            severity: "info".to_string(),
            file: sig.file.clone(),
            line: sig.line,
            message: "Signal 'state' found without 'next_state' - consider two-process FSM style"
                .to_string(),
        })
        .collect()
}

fn fsm_missing_default_state(input: &Input) -> Vec<Violation> {
    input
        .case_statements
        .iter()
        .filter(|cs| is_state_expression(&cs.expression))
        .filter(|cs| !cs.has_others)
        .map(|cs| Violation {
            rule: "fsm_missing_default_state".to_string(),
            severity: "error".to_string(),
            file: cs.file.clone(),
            line: cs.line,
            message: format!(
                "FSM case statement on '{}' missing 'when others' - undefined behavior for invalid states",
                cs.expression
            ),
        })
        .collect()
}

fn fsm_unhandled_state(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for type_decl in &input.types {
        if type_decl.kind != "enum" {
            continue;
        }
        if !is_state_type_name(&type_decl.name) {
            continue;
        }
        for literal in &type_decl.enum_literals {
            for cs in &input.case_statements {
                if !is_state_expression(&cs.expression) {
                    continue;
                }
                if !case_uses_this_type(cs, &type_decl.enum_literals) {
                    continue;
                }
                if cs.has_others {
                    continue;
                }
                if !state_in_choices(literal, &cs.choices) {
                    out.push(Violation {
                        rule: "fsm_unhandled_state".to_string(),
                        severity: "warning".to_string(),
                        file: cs.file.clone(),
                        line: cs.line,
                        message: format!(
                            "FSM state '{}' from type '{}' not explicitly handled in case statement",
                            literal, type_decl.name
                        ),
                    });
                }
            }
        }
    }
    out
}

fn fsm_unreachable_state(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for type_decl in &input.types {
        if type_decl.kind != "enum" {
            continue;
        }
        if !is_state_type_name(&type_decl.name) {
            continue;
        }
        if type_decl.enum_literals.is_empty() {
            continue;
        }
        for literal in &type_decl.enum_literals {
            if literal == &type_decl.enum_literals[0] {
                continue;
            }
            for sig in &input.signals {
                if !is_state_signal_name(&sig.name) {
                    continue;
                }
                if !state_ever_assigned(input, &sig.name, literal) {
                    out.push(Violation {
                        rule: "fsm_unreachable_state".to_string(),
                        severity: "warning".to_string(),
                        file: sig.file.clone(),
                        line: sig.line,
                        message: format!(
                            "FSM state '{}' is never assigned to '{}' - potentially unreachable",
                            sig.name, literal
                        ),
                    });
                }
            }
        }
    }
    out
}

fn is_state_signal_name(name: &str) -> bool {
    let lower = name.to_ascii_lowercase();
    lower == "state"
        || lower.ends_with("_state")
        || lower.starts_with("state_")
        || lower == "current_state"
        || lower == "next_state"
        || lower == "present_state"
}

fn is_next_state_name(name: &str) -> bool {
    matches!(
        name.to_ascii_lowercase().as_str(),
        "next_state" | "nextstate" | "nxt_state"
    )
}

fn is_vector_type(t: &str) -> bool {
    let lower = t.to_ascii_lowercase();
    lower.contains("vector") || lower.contains("unsigned") || lower.contains("signed")
}

fn is_state_expression(expr: &str) -> bool {
    is_state_signal_name(expr)
}

fn is_state_type_name(name: &str) -> bool {
    let lower = name.to_ascii_lowercase();
    lower.contains("state")
        || (lower.ends_with("_t") && lower.contains("st"))
        || lower.ends_with("_type")
}

fn has_next_state_signal(input: &Input, entity_name: &str) -> bool {
    input
        .signals
        .iter()
        .any(|sig| sig.in_entity == entity_name && is_next_state_name(&sig.name))
}

fn case_uses_this_type(cs: &crate::policy::input::CaseStatement, literals: &[String]) -> bool {
    cs.choices
        .iter()
        .any(|choice| literals.iter().any(|lit| choice.eq_ignore_ascii_case(lit)))
}

fn state_in_choices(state: &str, choices: &[String]) -> bool {
    choices.iter().any(|c| c.eq_ignore_ascii_case(state))
}

fn state_ever_assigned(input: &Input, sig_name: &str, state_literal: &str) -> bool {
    input.processes.iter().any(|proc| {
        proc.assigned_signals
            .iter()
            .any(|assigned| assigned.eq_ignore_ascii_case(sig_name))
            && proc
                .read_signals
                .iter()
                .any(|read| read.eq_ignore_ascii_case(state_literal))
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::policy::input::{CaseStatement, Input, Process, Signal, TypeDeclaration};

    #[test]
    fn state_signal_not_enum_flags_vector() {
        let mut input = Input::default();
        input.signals.push(Signal {
            name: "state".to_string(),
            r#type: "std_logic_vector".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        let violations = state_signal_not_enum(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "state_signal_not_enum");
    }

    #[test]
    fn single_state_signal_flags_missing_next_state() {
        let mut input = Input::default();
        input.signals.push(Signal {
            name: "state".to_string(),
            in_entity: "core".to_string(),
            file: "a.vhd".to_string(),
            line: 2,
            ..Default::default()
        });
        let violations = single_state_signal(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "single_state_signal");
    }

    #[test]
    fn fsm_missing_default_state_flags() {
        let mut input = Input::default();
        input.case_statements.push(CaseStatement {
            expression: "state".to_string(),
            has_others: false,
            file: "a.vhd".to_string(),
            line: 3,
            ..Default::default()
        });
        let violations = fsm_missing_default_state(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "fsm_missing_default_state");
    }

    #[test]
    fn fsm_unhandled_state_flags_missing_literal() {
        let mut input = Input::default();
        input.types.push(TypeDeclaration {
            name: "state_t".to_string(),
            kind: "enum".to_string(),
            enum_literals: vec!["IDLE".to_string(), "RUN".to_string()],
            ..Default::default()
        });
        input.case_statements.push(CaseStatement {
            expression: "state".to_string(),
            choices: vec!["IDLE".to_string()],
            has_others: false,
            file: "a.vhd".to_string(),
            line: 4,
            ..Default::default()
        });
        let violations = fsm_unhandled_state(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "fsm_unhandled_state");
    }

    #[test]
    fn fsm_unreachable_state_flags_missing_assignment() {
        let mut input = Input::default();
        input.types.push(TypeDeclaration {
            name: "state_t".to_string(),
            kind: "enum".to_string(),
            enum_literals: vec!["IDLE".to_string(), "RUN".to_string()],
            ..Default::default()
        });
        input.signals.push(Signal {
            name: "state".to_string(),
            file: "a.vhd".to_string(),
            line: 5,
            ..Default::default()
        });
        let violations = fsm_unreachable_state(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "fsm_unreachable_state");
    }

    #[test]
    fn fsm_unreachable_state_not_flagged_when_assigned() {
        let mut input = Input::default();
        input.types.push(TypeDeclaration {
            name: "state_t".to_string(),
            kind: "enum".to_string(),
            enum_literals: vec!["IDLE".to_string(), "RUN".to_string()],
            ..Default::default()
        });
        input.signals.push(Signal {
            name: "state".to_string(),
            file: "a.vhd".to_string(),
            line: 5,
            ..Default::default()
        });
        input.processes.push(Process {
            assigned_signals: vec!["state".to_string()],
            read_signals: vec!["RUN".to_string()],
            ..Default::default()
        });
        let violations = fsm_unreachable_state(&input);
        assert!(violations.is_empty());
    }
}
