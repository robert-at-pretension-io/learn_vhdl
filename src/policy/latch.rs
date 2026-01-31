use crate::policy::helpers;
use crate::policy::input::Input;
use crate::policy::result::Violation;

pub fn violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(incomplete_case_latch(input));
    out.extend(enum_case_incomplete(input));
    out
}

pub fn optional_violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(combinational_incomplete_assignment(input));
    out.extend(conditional_assignment_check(input));
    out.extend(selected_assignment_check(input));
    out.extend(many_signals_no_default(input));
    out.extend(fsm_no_reset(input));
    out
}

fn incomplete_case_latch(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for cs in &input.case_statements {
        if cs.has_others {
            continue;
        }
        if cs.in_process.is_empty() {
            out.push(Violation {
                rule: "incomplete_case_latch".to_string(),
                severity: "warning".to_string(),
                file: cs.file.clone(),
                line: cs.line,
                message: format!(
                    "Case statement on '{}' missing 'when others =>' - may infer latch",
                    cs.expression
                ),
            });
            continue;
        }
        if let Some(proc) = input
            .processes
            .iter()
            .find(|p| p.file == cs.file && p.label == cs.in_process && p.is_combinational)
        {
            out.push(Violation {
                rule: "incomplete_case_latch".to_string(),
                severity: "warning".to_string(),
                file: cs.file.clone(),
                line: cs.line,
                message: format!(
                    "Case statement on '{}' in combinational process '{}' missing 'when others =>' - will infer latch",
                    cs.expression, proc.label
                ),
            });
        }
    }
    out
}

fn enum_case_incomplete(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for cs in &input.case_statements {
        if cs.has_others {
            continue;
        }
        let sig = match input
            .signals
            .iter()
            .find(|sig| sig.name.eq_ignore_ascii_case(&cs.expression))
        {
            Some(sig) => sig,
            None => continue,
        };
        let enum_type = match input
            .types
            .iter()
            .find(|t| t.kind == "enum" && t.name.eq_ignore_ascii_case(&sig.r#type))
        {
            Some(t) => t,
            None => continue,
        };
        let covered: Vec<String> = cs.choices.iter().map(|c| c.to_ascii_lowercase()).collect();
        let mut missing: Vec<String> = enum_type
            .enum_literals
            .iter()
            .filter(|lit| !covered.iter().any(|c| c.eq_ignore_ascii_case(lit)))
            .cloned()
            .collect();
        if missing.is_empty() {
            continue;
        }
        if let Some(_proc) = input
            .processes
            .iter()
            .find(|p| p.file == cs.file && p.label == cs.in_process && p.is_combinational)
        {
            missing.sort();
            out.push(Violation {
                rule: "enum_case_incomplete".to_string(),
                severity: "error".to_string(),
                file: cs.file.clone(),
                line: cs.line,
                message: format!(
                    "Case statement on enum '{}' missing values {:?} in combinational process - will infer latch",
                    cs.expression, missing
                ),
            });
        }
    }
    out
}

fn combinational_incomplete_assignment(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for proc in &input.processes {
        if !proc.is_combinational {
            continue;
        }
        for assigned in &proc.assigned_signals {
            for read in &proc.read_signals {
                if !assigned.eq_ignore_ascii_case(read) {
                    continue;
                }
                if helpers::is_clock_name(assigned) || helpers::is_reset_name(assigned) {
                    continue;
                }
                if is_next_state_pattern(assigned) {
                    continue;
                }
                if !helpers::is_actual_signal(input, assigned) {
                    continue;
                }
                out.push(Violation {
                    rule: "combinational_incomplete_assignment".to_string(),
                    severity: "info".to_string(),
                    file: proc.file.clone(),
                    line: proc.line,
                    message: format!(
                        "Signal '{}' in combinational process '{}' is read as well as written - verify all code paths assign it to avoid latch",
                        assigned, proc.label
                    ),
                });
            }
        }
    }
    out
}

fn is_next_state_pattern(name: &str) -> bool {
    name.to_ascii_lowercase().contains("next")
}

fn conditional_assignment_check(input: &Input) -> Vec<Violation> {
    input
        .concurrent_assignments
        .iter()
        .filter(|ca| ca.kind == "conditional")
        .map(|ca| Violation {
            rule: "conditional_assignment_review".to_string(),
            severity: "info".to_string(),
            file: ca.file.clone(),
            line: ca.line,
            message: format!(
                "Conditional assignment to '{}' - verify all conditions have an 'else' clause to avoid latch inference",
                ca.target
            ),
        })
        .collect()
}

fn selected_assignment_check(input: &Input) -> Vec<Violation> {
    input
        .concurrent_assignments
        .iter()
        .filter(|ca| ca.kind == "selected")
        .map(|ca| Violation {
            rule: "selected_assignment_review".to_string(),
            severity: "info".to_string(),
            file: ca.file.clone(),
            line: ca.line,
            message: format!(
                "Selected assignment to '{}' - verify 'when others' is present to avoid latch inference",
                ca.target
            ),
        })
        .collect()
}

fn many_signals_no_default(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for proc in &input.processes {
        if !proc.is_combinational || proc.assigned_signals.len() <= 3 {
            continue;
        }
        let has_incomplete_case = input
            .case_statements
            .iter()
            .any(|cs| cs.file == proc.file && cs.in_process == proc.label && !cs.has_others);
        if !has_incomplete_case {
            continue;
        }
        out.push(Violation {
            rule: "combinational_default_values".to_string(),
            severity: "info".to_string(),
            file: proc.file.clone(),
            line: proc.line,
            message: format!(
                "Combinational process '{}' assigns {} signals - consider adding default values at process start to prevent latches",
                proc.label,
                proc.assigned_signals.len()
            ),
        });
    }
    out
}

fn fsm_no_reset(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for proc in &input.processes {
        if !proc.is_sequential || proc.has_reset {
            continue;
        }
        for assigned in &proc.assigned_signals {
            if helpers::is_state_name(assigned) {
                out.push(Violation {
                    rule: "fsm_no_reset_state".to_string(),
                    severity: "warning".to_string(),
                    file: proc.file.clone(),
                    line: proc.line,
                    message: format!(
                        "State signal '{}' in process '{}' has no reset - initial state undefined",
                        assigned, proc.label
                    ),
                });
            }
        }
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::policy::input::{CaseStatement, Input, Process, Signal, TypeDeclaration};

    #[test]
    fn incomplete_case_latch_flags() {
        let mut input = Input::default();
        input.processes.push(Process {
            label: "p1".to_string(),
            is_combinational: true,
            file: "a.vhd".to_string(),
            ..Default::default()
        });
        input.case_statements.push(CaseStatement {
            expression: "sel".to_string(),
            has_others: false,
            file: "a.vhd".to_string(),
            line: 10,
            in_process: "p1".to_string(),
            ..Default::default()
        });
        let v = incomplete_case_latch(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "incomplete_case_latch");
    }

    #[test]
    fn enum_case_incomplete_flags() {
        let mut input = Input::default();
        input.processes.push(Process {
            label: "p1".to_string(),
            is_combinational: true,
            file: "a.vhd".to_string(),
            ..Default::default()
        });
        input.signals.push(Signal {
            name: "state".to_string(),
            r#type: "state_t".to_string(),
            ..Default::default()
        });
        input.types.push(TypeDeclaration {
            name: "state_t".to_string(),
            kind: "enum".to_string(),
            enum_literals: vec!["IDLE".to_string(), "RUN".to_string()],
            ..Default::default()
        });
        input.case_statements.push(CaseStatement {
            expression: "state".to_string(),
            has_others: false,
            choices: vec!["IDLE".to_string()],
            file: "a.vhd".to_string(),
            line: 11,
            in_process: "p1".to_string(),
            ..Default::default()
        });
        let v = enum_case_incomplete(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "enum_case_incomplete");
    }
}
