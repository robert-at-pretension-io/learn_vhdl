use crate::policy::helpers;
use crate::policy::input::Input;
use crate::policy::result::Violation;

pub fn violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(complex_process(input));
    out.extend(comb_process_no_default(input));
    out
}

pub fn optional_violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(wait_in_clocked_process(input));
    out.extend(process_with_no_statements(input));
    out.extend(constant_if_condition(input));
    out
}

fn wait_in_clocked_process(input: &Input) -> Vec<Violation> {
    input
        .processes
        .iter()
        .filter(|proc| proc.is_sequential)
        .filter(|proc| !proc.clock_edge.is_empty())
        .filter(|proc| !proc.wait_statements.is_empty())
        .map(|proc| Violation {
            rule: "wait_in_clocked_process".to_string(),
            severity: "error".to_string(),
            file: proc.file.clone(),
            line: proc.line,
            message: format!(
                "Sequential process '{}' uses clock edge detection and wait statements - synthesis-incompatible",
                proc.label
            ),
        })
        .collect()
}

fn process_with_no_statements(input: &Input) -> Vec<Violation> {
    input
        .processes
        .iter()
        .filter(|proc| proc.assigned_signals.is_empty())
        .filter(|proc| proc.procedure_calls.is_empty())
        .filter(|proc| proc.function_calls.is_empty())
        .filter(|proc| proc.wait_statements.is_empty())
        .filter(|proc| !proc.label.is_empty())
        .filter(|proc| !helpers::process_in_testbench(input, proc))
        .map(|proc| Violation {
            rule: "process_with_no_statements".to_string(),
            severity: "warning".to_string(),
            file: proc.file.clone(),
            line: proc.line,
            message: format!(
                "Process '{}' has no signal assignments, procedure calls, or wait statements - dead code",
                proc.label
            ),
        })
        .collect()
}

fn is_literal_operand(s: &str) -> bool {
    s.starts_with('\'')
        || s.starts_with('"')
        || s.starts_with("x\"")
        || s.starts_with("X\"")
        || s.chars().next().map_or(false, |c| c.is_ascii_digit())
}

fn constant_if_condition(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for cmp in &input.comparisons {
        if cmp.in_process.is_empty() {
            continue;
        }
        if cmp.left_operand.eq_ignore_ascii_case(&cmp.right_operand) {
            out.push(Violation {
                rule: "constant_if_condition".to_string(),
                severity: "info".to_string(),
                file: cmp.file.clone(),
                line: cmp.line,
                message: format!(
                    "Comparison of '{}' to itself in process '{}' - condition is always true/false",
                    cmp.left_operand, cmp.in_process
                ),
            });
        } else if is_literal_operand(&cmp.left_operand) && is_literal_operand(&cmp.right_operand) {
            out.push(Violation {
                rule: "constant_if_condition".to_string(),
                severity: "info".to_string(),
                file: cmp.file.clone(),
                line: cmp.line,
                message: format!(
                    "Comparison of literals '{}' {} '{}' in process '{}' - condition is constant",
                    cmp.left_operand, cmp.operator, cmp.right_operand, cmp.in_process
                ),
            });
        }
    }
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
    use crate::policy::input::{CaseStatement, Comparison, Input, Process, WaitStatement};

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

    #[test]
    fn wait_in_clocked_process_flags() {
        let mut input = Input::default();
        input.processes.push(Process {
            label: "clk_proc".to_string(),
            is_sequential: true,
            clock_edge: "rising_edge".to_string(),
            wait_statements: vec![WaitStatement {
                line: 10,
                in_process: "clk_proc".to_string(),
            }],
            file: "a.vhd".to_string(),
            line: 5,
            ..Default::default()
        });
        let v = wait_in_clocked_process(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "wait_in_clocked_process");
        assert_eq!(v[0].severity, "error");
    }

    #[test]
    fn wait_in_clocked_process_skips_no_clock_edge() {
        let mut input = Input::default();
        input.processes.push(Process {
            label: "wait_proc".to_string(),
            is_sequential: true,
            clock_edge: String::new(),
            wait_statements: vec![WaitStatement {
                line: 10,
                in_process: "wait_proc".to_string(),
            }],
            file: "a.vhd".to_string(),
            line: 5,
            ..Default::default()
        });
        let v = wait_in_clocked_process(&input);
        assert!(v.is_empty());
    }

    #[test]
    fn process_with_no_statements_flags() {
        let mut input = Input::default();
        input.processes.push(Process {
            label: "empty_proc".to_string(),
            file: "a.vhd".to_string(),
            line: 3,
            ..Default::default()
        });
        let v = process_with_no_statements(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "process_with_no_statements");
        assert_eq!(v[0].severity, "warning");
    }

    #[test]
    fn process_with_no_statements_skips_unlabeled() {
        let mut input = Input::default();
        input.processes.push(Process {
            label: String::new(),
            file: "a.vhd".to_string(),
            line: 3,
            ..Default::default()
        });
        let v = process_with_no_statements(&input);
        assert!(v.is_empty());
    }

    #[test]
    fn constant_if_condition_same_signal() {
        let mut input = Input::default();
        input.comparisons.push(Comparison {
            left_operand: "sig_a".to_string(),
            operator: "=".to_string(),
            right_operand: "SIG_A".to_string(),
            in_process: "p1".to_string(),
            file: "a.vhd".to_string(),
            line: 7,
            ..Default::default()
        });
        let v = constant_if_condition(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "constant_if_condition");
        assert!(v[0].message.contains("to itself"));
    }

    #[test]
    fn constant_if_condition_two_literals() {
        let mut input = Input::default();
        input.comparisons.push(Comparison {
            left_operand: "'1'".to_string(),
            operator: "=".to_string(),
            right_operand: "'0'".to_string(),
            in_process: "p1".to_string(),
            file: "a.vhd".to_string(),
            line: 8,
            ..Default::default()
        });
        let v = constant_if_condition(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "constant_if_condition");
        assert!(v[0].message.contains("literals"));
    }

    #[test]
    fn constant_if_condition_skips_different_signals() {
        let mut input = Input::default();
        input.comparisons.push(Comparison {
            left_operand: "sig_a".to_string(),
            operator: "=".to_string(),
            right_operand: "sig_b".to_string(),
            in_process: "p1".to_string(),
            file: "a.vhd".to_string(),
            line: 9,
            ..Default::default()
        });
        let v = constant_if_condition(&input);
        assert!(v.is_empty());
    }
}
