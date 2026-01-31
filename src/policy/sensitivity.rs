use crate::policy::helpers;
use crate::policy::input::Input;
use crate::policy::result::Violation;
use crate::policy::signals;

pub fn violations(input: &Input) -> Vec<Violation> {
    sensitivity_list_incomplete(input)
}

pub fn optional_violations(input: &Input) -> Vec<Violation> {
    sensitivity_list_superfluous(input)
}

fn skip_sensitivity(input: &Input, proc_index: usize) -> bool {
    let proc = &input.processes[proc_index];
    helpers::single_file_mode(input) && helpers::sensitivity_list_has_clock(&proc.sensitivity_list)
}

fn sensitivity_list_incomplete(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for (idx, proc) in input.processes.iter().enumerate() {
        if !proc.is_combinational {
            continue;
        }
        if helpers::has_all_sensitivity(&proc.sensitivity_list) {
            continue;
        }
        if proc.assigned_signals.is_empty() {
            continue;
        }
        if skip_sensitivity(input, idx) {
            continue;
        }
        if helpers::process_in_testbench(input, proc) {
            continue;
        }
        for read_sig in &proc.read_signals {
            if !signals::is_declared_identifier(input, read_sig) {
                continue;
            }
            if !signals::is_actual_signal(input, read_sig) {
                continue;
            }
            if helpers::is_skip_name(input, read_sig) {
                continue;
            }
            if helpers::sig_in_sensitivity(read_sig, &proc.sensitivity_list) {
                continue;
            }
            out.push(Violation {
                rule: "sensitivity_list_incomplete".to_string(),
                severity: "error".to_string(),
                file: proc.file.clone(),
                line: proc.line,
                message: format!(
                    "Signal '{}' read in combinational process '{}' but missing from sensitivity list",
                    read_sig, proc.label
                ),
            });
        }
    }
    out
}

fn sensitivity_list_superfluous(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for (idx, proc) in input.processes.iter().enumerate() {
        if !proc.is_combinational {
            continue;
        }
        if skip_sensitivity(input, idx) {
            continue;
        }
        for sens_sig in &proc.sensitivity_list {
            if sens_sig.eq_ignore_ascii_case("all") {
                continue;
            }
            if helpers::sig_in_reads(sens_sig, &proc.read_signals) {
                continue;
            }
            out.push(Violation {
                rule: "sensitivity_list_superfluous".to_string(),
                severity: "info".to_string(),
                file: proc.file.clone(),
                line: proc.line,
                message: format!(
                    "Signal '{}' in sensitivity list but never read in process '{}'",
                    sens_sig, proc.label
                ),
            });
        }
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::policy::input::{Input, Process};

    #[test]
    fn sensitivity_list_incomplete_flags() {
        let mut input = Input::default();
        input.processes.push(Process {
            label: "p1".to_string(),
            is_combinational: true,
            read_signals: vec!["a".to_string()],
            assigned_signals: vec!["b".to_string()],
            sensitivity_list: vec!["b".to_string()],
            file: "a.vhd".to_string(),
            line: 5,
            ..Default::default()
        });
        input.signals.push(crate::policy::input::Signal {
            name: "a".to_string(),
            ..Default::default()
        });
        let v = sensitivity_list_incomplete(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "sensitivity_list_incomplete");
    }

    #[test]
    fn sensitivity_list_superfluous_flags() {
        let mut input = Input::default();
        input.processes.push(Process {
            label: "p1".to_string(),
            is_combinational: true,
            read_signals: vec!["a".to_string()],
            sensitivity_list: vec!["a".to_string(), "b".to_string()],
            file: "a.vhd".to_string(),
            line: 8,
            ..Default::default()
        });
        let v = sensitivity_list_superfluous(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "sensitivity_list_superfluous");
    }
}
