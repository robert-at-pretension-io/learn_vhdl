use crate::policy::input::Input;
use crate::policy::result::Violation;

pub fn violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(complex_process(input));
    out.extend(comb_process_no_default(input));
    out
}

fn complex_process(input: &Input) -> Vec<Violation> {
    input
        .processes
        .iter()
        .filter(|proc| proc.assigned_signals.len() > 20)
        .map(|proc| Violation {
            rule: "complex_process".to_string(),
            severity: "info".to_string(),
            file: proc.file.clone(),
            line: proc.line,
            message: format!(
                "Process '{}' assigns {} signals - consider splitting into smaller processes",
                proc.label,
                proc.assigned_signals.len()
            ),
        })
        .collect()
}

fn comb_process_no_default(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for proc in &input.processes {
        if !proc.is_combinational {
            continue;
        }
        if proc.assigned_signals.is_empty() {
            continue;
        }
        let has_incomplete_case = input
            .case_statements
            .iter()
            .any(|cs| cs.in_process == proc.label && !cs.has_others);
        if has_incomplete_case {
            out.push(Violation {
                rule: "comb_process_no_default".to_string(),
                severity: "warning".to_string(),
                file: proc.file.clone(),
                line: proc.line,
                message: format!(
                    "Combinational process '{}' has incomplete case statement - may infer latch",
                    proc.label
                ),
            });
        }
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::policy::input::{CaseStatement, Input, Process};

    #[test]
    fn complex_process_flags_many_assigns() {
        let mut input = Input::default();
        input.processes.push(Process {
            label: "p1".to_string(),
            assigned_signals: (0..21).map(|i| format!("s{}", i)).collect(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        let violations = complex_process(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "complex_process");
    }

    #[test]
    fn comb_process_no_default_flags_incomplete_case() {
        let mut input = Input::default();
        input.processes.push(Process {
            label: "p1".to_string(),
            is_combinational: true,
            assigned_signals: vec!["a".to_string()],
            file: "a.vhd".to_string(),
            line: 2,
            ..Default::default()
        });
        input.case_statements.push(CaseStatement {
            in_process: "p1".to_string(),
            has_others: false,
            ..Default::default()
        });
        let violations = comb_process_no_default(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "comb_process_no_default");
    }
}
