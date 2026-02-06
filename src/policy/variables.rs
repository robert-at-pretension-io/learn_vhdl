use std::collections::HashSet;

use crate::policy::helpers;
use crate::policy::input::Input;
use crate::policy::result::Violation;

pub fn optional_violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(unused_variable(input));
    out.extend(variable_shadows_signal(input));
    out.extend(uninitialized_variable_read(input));
    out
}

fn unused_variable(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for proc in &input.processes {
        if helpers::process_in_testbench(input, proc) {
            continue;
        }
        let mut referenced: HashSet<String> = HashSet::new();
        for sig in &proc.read_variables {
            referenced.insert(sig.to_ascii_lowercase());
        }
        for sig in &proc.assigned_variables {
            referenced.insert(sig.to_ascii_lowercase());
        }
        for fc in &proc.function_calls {
            for arg in &fc.args {
                referenced.insert(arg.to_ascii_lowercase());
            }
        }
        for pc in &proc.procedure_calls {
            for arg in &pc.args {
                referenced.insert(arg.to_ascii_lowercase());
            }
        }
        for var in &proc.variables {
            if !referenced.contains(&var.name.to_ascii_lowercase()) {
                out.push(Violation {
                    rule: "unused_variable".to_string(),
                    severity: "warning".to_string(),
                    file: proc.file.clone(),
                    line: var.line,
                    message: format!(
                        "Variable '{}' in process '{}' is declared but never used",
                        var.name, proc.label
                    ),
                });
            }
        }
    }
    out
}

fn variable_shadows_signal(input: &Input) -> Vec<Violation> {
    let signal_names: HashSet<String> = input
        .signals
        .iter()
        .map(|sig| sig.name.to_ascii_lowercase())
        .collect();
    let port_names: HashSet<String> = input
        .ports
        .iter()
        .map(|port| port.name.to_ascii_lowercase())
        .collect();
    let mut out = Vec::new();
    for proc in &input.processes {
        if helpers::process_in_testbench(input, proc) {
            continue;
        }
        for var in &proc.variables {
            let lower = var.name.to_ascii_lowercase();
            if signal_names.contains(&lower) || port_names.contains(&lower) {
                out.push(Violation {
                    rule: "variable_shadows_signal".to_string(),
                    severity: "warning".to_string(),
                    file: proc.file.clone(),
                    line: var.line,
                    message: format!(
                        "Variable '{}' in process '{}' shadows a signal or port with the same name",
                        var.name, proc.label
                    ),
                });
            }
        }
    }
    out
}

fn uninitialized_variable_read(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for proc in &input.processes {
        if helpers::process_in_testbench(input, proc) {
            continue;
        }
        let assigned: HashSet<String> = proc
            .assigned_variables
            .iter()
            .map(|s| s.to_ascii_lowercase())
            .collect();
        for var in &proc.variables {
            let lower = var.name.to_ascii_lowercase();
            let is_read = proc
                .read_variables
                .iter()
                .any(|r| r.eq_ignore_ascii_case(&var.name));
            if is_read && !assigned.contains(&lower) {
                out.push(Violation {
                    rule: "uninitialized_variable_read".to_string(),
                    severity: "warning".to_string(),
                    file: proc.file.clone(),
                    line: var.line,
                    message: format!(
                        "Variable '{}' in process '{}' is read but never assigned — defaults to 'U'",
                        var.name, proc.label
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
    use crate::policy::input::{Input, Port, Process, Signal, VariableDecl};

    #[test]
    fn unused_variable_flags() {
        let mut input = Input::default();
        input.processes.push(Process {
            label: "p1".to_string(),
            variables: vec![VariableDecl {
                name: "dead_var".to_string(),
                r#type: "integer".to_string(),
                line: 5,
            }],
            file: "a.vhd".to_string(),
            line: 3,
            ..Default::default()
        });
        let v = unused_variable(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "unused_variable");
        assert!(v[0].message.contains("dead_var"));
    }

    #[test]
    fn unused_variable_skip_referenced() {
        let mut input = Input::default();
        input.processes.push(Process {
            label: "p1".to_string(),
            variables: vec![VariableDecl {
                name: "used_var".to_string(),
                r#type: "integer".to_string(),
                line: 5,
            }],
            read_variables: vec!["used_var".to_string()],
            file: "a.vhd".to_string(),
            line: 3,
            ..Default::default()
        });
        let v = unused_variable(&input);
        assert!(v.is_empty());
    }

    #[test]
    fn unused_variable_skip_assigned() {
        let mut input = Input::default();
        input.processes.push(Process {
            label: "p1".to_string(),
            variables: vec![VariableDecl {
                name: "wr_var".to_string(),
                r#type: "integer".to_string(),
                line: 5,
            }],
            assigned_variables: vec!["wr_var".to_string()],
            file: "a.vhd".to_string(),
            line: 3,
            ..Default::default()
        });
        let v = unused_variable(&input);
        assert!(v.is_empty());
    }

    #[test]
    fn variable_shadows_signal_flags() {
        let mut input = Input::default();
        input.signals.push(Signal {
            name: "data_reg".to_string(),
            file: "a.vhd".to_string(),
            line: 2,
            ..Default::default()
        });
        input.processes.push(Process {
            label: "p1".to_string(),
            variables: vec![VariableDecl {
                name: "data_reg".to_string(),
                r#type: "std_logic".to_string(),
                line: 5,
            }],
            file: "a.vhd".to_string(),
            line: 3,
            ..Default::default()
        });
        let v = variable_shadows_signal(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "variable_shadows_signal");
    }

    #[test]
    fn variable_shadows_port_flags() {
        let mut input = Input::default();
        input.ports.push(Port {
            name: "din".to_string(),
            direction: "in".to_string(),
            ..Default::default()
        });
        input.processes.push(Process {
            label: "p1".to_string(),
            variables: vec![VariableDecl {
                name: "din".to_string(),
                r#type: "std_logic".to_string(),
                line: 5,
            }],
            file: "a.vhd".to_string(),
            line: 3,
            ..Default::default()
        });
        let v = variable_shadows_signal(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "variable_shadows_signal");
    }

    #[test]
    fn variable_shadows_signal_no_shadow() {
        let mut input = Input::default();
        input.signals.push(Signal {
            name: "data_reg".to_string(),
            file: "a.vhd".to_string(),
            line: 2,
            ..Default::default()
        });
        input.processes.push(Process {
            label: "p1".to_string(),
            variables: vec![VariableDecl {
                name: "temp".to_string(),
                r#type: "std_logic".to_string(),
                line: 5,
            }],
            file: "a.vhd".to_string(),
            line: 3,
            ..Default::default()
        });
        let v = variable_shadows_signal(&input);
        assert!(v.is_empty());
    }

    #[test]
    fn uninitialized_variable_read_flags() {
        let mut input = Input::default();
        input.processes.push(Process {
            label: "p1".to_string(),
            variables: vec![VariableDecl {
                name: "uninit_var".to_string(),
                r#type: "integer".to_string(),
                line: 5,
            }],
            read_variables: vec!["uninit_var".to_string()],
            file: "a.vhd".to_string(),
            line: 3,
            ..Default::default()
        });
        let v = uninitialized_variable_read(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "uninitialized_variable_read");
    }

    #[test]
    fn uninitialized_variable_read_skip_assigned() {
        let mut input = Input::default();
        input.processes.push(Process {
            label: "p1".to_string(),
            variables: vec![VariableDecl {
                name: "good_var".to_string(),
                r#type: "integer".to_string(),
                line: 5,
            }],
            read_variables: vec!["good_var".to_string()],
            assigned_variables: vec!["good_var".to_string()],
            file: "a.vhd".to_string(),
            line: 3,
            ..Default::default()
        });
        let v = uninitialized_variable_read(&input);
        assert!(v.is_empty());
    }
}
