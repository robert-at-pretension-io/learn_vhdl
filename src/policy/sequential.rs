use crate::policy::helpers;
use crate::policy::input::Input;
use crate::policy::result::Violation;

pub fn violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(missing_clock_sensitivity(input));
    out.extend(signal_in_seq_and_comb(input));
    out
}

pub fn optional_violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(missing_reset_sensitivity(input));
    out.extend(very_wide_register(input));
    out.extend(mixed_edge_clocking(input));
    out.extend(async_reset_naming(input));
    out
}

fn missing_clock_sensitivity(input: &Input) -> Vec<Violation> {
    input
        .processes
        .iter()
        .filter(|proc| proc.is_sequential)
        .filter(|proc| !proc.clock_signal.is_empty())
        .filter(|proc| proc.wait_statements.is_empty())
        .filter(|proc| !helpers::has_all_sensitivity(&proc.sensitivity_list))
        .filter(|proc| {
            !proc
                .sensitivity_list
                .iter()
                .any(|s| s.eq_ignore_ascii_case(&proc.clock_signal))
        })
        .map(|proc| Violation {
            rule: "missing_clock_sensitivity".to_string(),
            severity: "error".to_string(),
            file: proc.file.clone(),
            line: proc.line,
            message: format!(
                "Sequential process '{}' uses clock '{}' but it's not in sensitivity list",
                proc.label, proc.clock_signal
            ),
        })
        .collect()
}

fn missing_reset_sensitivity(input: &Input) -> Vec<Violation> {
    input
        .processes
        .iter()
        .filter(|proc| proc.is_sequential)
        .filter(|proc| proc.has_reset)
        .filter(|proc| !proc.reset_signal.is_empty())
        .filter(|proc| proc.reset_async)
        .filter(|proc| !helpers::has_all_sensitivity(&proc.sensitivity_list))
        .filter(|proc| {
            !proc
                .sensitivity_list
                .iter()
                .any(|s| s.eq_ignore_ascii_case(&proc.reset_signal))
        })
        .map(|proc| Violation {
            rule: "missing_reset_sensitivity".to_string(),
            severity: "warning".to_string(),
            file: proc.file.clone(),
            line: proc.line,
            message: format!(
                "Process '{}' uses reset '{}' but it's not in sensitivity list (sync reset?)",
                proc.label, proc.reset_signal
            ),
        })
        .collect()
}

fn very_wide_register(input: &Input) -> Vec<Violation> {
    input
        .processes
        .iter()
        .filter(|proc| proc.is_sequential)
        .filter(|proc| proc.assigned_signals.len() > 15)
        .map(|proc| Violation {
            rule: "very_wide_register".to_string(),
            severity: "info".to_string(),
            file: proc.file.clone(),
            line: proc.line,
            message: format!(
                "Sequential process '{}' assigns {} signals - consider splitting for clarity",
                proc.label,
                proc.assigned_signals.len()
            ),
        })
        .collect()
}

fn mixed_edge_clocking(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for (i, proc1) in input.processes.iter().enumerate() {
        if !proc1.is_sequential || proc1.clock_signal.is_empty() {
            continue;
        }
        for proc2 in input.processes.iter().skip(i + 1) {
            if !proc2.is_sequential || proc2.clock_signal.is_empty() {
                continue;
            }
            if !proc1.clock_signal.eq_ignore_ascii_case(&proc2.clock_signal) {
                continue;
            }
            if proc1.clock_edge == proc2.clock_edge {
                continue;
            }
            if proc1.file != proc2.file {
                continue;
            }
            out.push(Violation {
                rule: "mixed_edge_clocking".to_string(),
                severity: "warning".to_string(),
                file: proc1.file.clone(),
                line: proc1.line,
                message: format!(
                    "Processes '{}' ({} edge) and '{}' ({} edge) use same clock '{}' with different edges",
                    proc1.label, proc1.clock_edge, proc2.label, proc2.clock_edge, proc1.clock_signal
                ),
            });
        }
    }
    out
}

fn signal_in_seq_and_comb(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for proc_seq in input.processes.iter().filter(|p| p.is_sequential) {
        for proc_comb in input.processes.iter().filter(|p| p.is_combinational) {
            if proc_seq.file != proc_comb.file {
                continue;
            }
            for assigned_seq in &proc_seq.assigned_signals {
                for assigned_comb in &proc_comb.assigned_signals {
                    if !assigned_seq.eq_ignore_ascii_case(assigned_comb) {
                        continue;
                    }
                    if !helpers::is_actual_signal(input, assigned_seq) {
                        continue;
                    }
                    if helpers::is_composite_identifier(input, assigned_seq) {
                        continue;
                    }
                    out.push(Violation {
                        rule: "signal_in_seq_and_comb".to_string(),
                        severity: "error".to_string(),
                        file: proc_seq.file.clone(),
                        line: proc_seq.line,
                        message: format!(
                            "Signal '{}' assigned in both sequential process '{}' and combinational process '{}'",
                            assigned_seq, proc_seq.label, proc_comb.label
                        ),
                    });
                }
            }
        }
    }
    out
}

fn async_reset_naming(input: &Input) -> Vec<Violation> {
    input
        .processes
        .iter()
        .filter(|proc| proc.is_sequential)
        .filter(|proc| proc.has_reset)
        .filter(|proc| !proc.reset_signal.is_empty())
        .filter(|proc| !is_active_low_reset_name(&proc.reset_signal))
        .map(|proc| Violation {
            rule: "async_reset_naming".to_string(),
            severity: "info".to_string(),
            file: proc.file.clone(),
            line: proc.line,
            message: format!(
                "Reset signal '{}' doesn't follow active-low naming convention (*_n, *n)",
                proc.reset_signal
            ),
        })
        .collect()
}

fn is_active_low_reset_name(name: &str) -> bool {
    let lower = name.to_ascii_lowercase();
    lower.ends_with("_n") || (lower.ends_with('n') && helpers::is_reset_name(name))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::policy::input::Process;

    #[test]
    fn missing_clock_sensitivity_flags() {
        let mut input = Input::default();
        input.processes.push(Process {
            label: "seq".to_string(),
            is_sequential: true,
            clock_signal: "clk".to_string(),
            sensitivity_list: vec!["rst".to_string()],
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        let v = missing_clock_sensitivity(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "missing_clock_sensitivity");
    }

    #[test]
    fn signal_in_seq_and_comb_flags() {
        let mut input = Input::default();
        input.processes.push(Process {
            label: "seq".to_string(),
            is_sequential: true,
            assigned_signals: vec!["sig".to_string()],
            file: "a.vhd".to_string(),
            line: 2,
            ..Default::default()
        });
        input.processes.push(Process {
            label: "comb".to_string(),
            is_combinational: true,
            assigned_signals: vec!["sig".to_string()],
            file: "a.vhd".to_string(),
            line: 3,
            ..Default::default()
        });
        let v = signal_in_seq_and_comb(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "signal_in_seq_and_comb");
    }

    #[test]
    fn async_reset_naming_flags() {
        let mut input = Input::default();
        input.processes.push(Process {
            label: "seq".to_string(),
            is_sequential: true,
            has_reset: true,
            reset_signal: "reset".to_string(),
            file: "a.vhd".to_string(),
            line: 4,
            ..Default::default()
        });
        let v = async_reset_naming(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "async_reset_naming");
    }
}
