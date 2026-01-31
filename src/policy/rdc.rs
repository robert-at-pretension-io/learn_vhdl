use crate::policy::helpers;
use crate::policy::input::Input;
use crate::policy::result::Violation;

pub fn violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(reset_crosses_domains(input));
    out
}

pub fn optional_violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(combinational_reset_gen(input));
    out.extend(async_reset_unsynchronized(input));
    out.extend(partial_reset_domain(input));
    out.extend(short_reset_sync(input));
    out
}

fn async_reset_unsynchronized(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for proc in &input.processes {
        if !proc.is_sequential || !proc.has_reset || proc.reset_signal.is_empty() {
            continue;
        }
        let is_async = helpers::signal_in_list(&proc.reset_signal, &proc.sensitivity_list);
        if !is_async {
            continue;
        }
        if has_reset_synchronizer(input, &proc.reset_signal, &proc.clock_signal) {
            continue;
        }
        out.push(Violation {
            rule: "async_reset_unsynchronized".to_string(),
            severity: "warning".to_string(),
            file: proc.file.clone(),
            line: proc.line,
            message: format!(
                "Async reset '{}' used directly in process '{}' - needs synchronization to '{}' clock domain",
                proc.reset_signal, proc.label, proc.clock_signal
            ),
        });
    }
    out
}

fn has_reset_synchronizer(input: &Input, reset_sig: &str, clock_sig: &str) -> bool {
    input.processes.iter().any(|proc| {
        proc.is_sequential
            && proc.clock_signal == clock_sig
            && proc
                .read_signals
                .iter()
                .any(|sig| sig.eq_ignore_ascii_case(reset_sig))
            && proc
                .assigned_signals
                .iter()
                .any(|assigned| is_sync_name(assigned, reset_sig))
    })
}

fn is_sync_name(assigned: &str, reset_sig: &str) -> bool {
    let lower_assigned = assigned.to_ascii_lowercase();
    let lower_reset = reset_sig.to_ascii_lowercase();
    let patterns = ["_sync", "_synced", "_meta", "_d1", "_d2", "_ff1", "_ff2"];
    if patterns
        .iter()
        .any(|pattern| lower_assigned.contains(&lower_reset) && lower_assigned.contains(pattern))
    {
        return true;
    }
    lower_assigned.contains("sync") && helpers::is_reset_name(assigned)
}

fn reset_crosses_domains(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for proc1 in &input.processes {
        if !proc1.has_reset || proc1.reset_signal.is_empty() {
            continue;
        }
        for proc2 in &input.processes {
            if !proc2.has_reset || proc2.reset_signal != proc1.reset_signal {
                continue;
            }
            if proc1.clock_signal.is_empty()
                || proc2.clock_signal.is_empty()
                || proc1.clock_signal == proc2.clock_signal
            {
                continue;
            }
            if proc1.line >= proc2.line {
                continue;
            }
            out.push(Violation {
                rule: "reset_crosses_domains".to_string(),
                severity: "error".to_string(),
                file: proc1.file.clone(),
                line: proc1.line,
                message: format!(
                    "Reset '{}' used in multiple clock domains ('{}' and '{}') - each domain needs synchronized reset",
                    proc1.reset_signal, proc1.clock_signal, proc2.clock_signal
                ),
            });
        }
    }
    out
}

fn partial_reset_domain(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for proc1 in &input.processes {
        if !proc1.is_sequential || !proc1.has_reset || proc1.clock_signal.is_empty() {
            continue;
        }
        for proc2 in &input.processes {
            if !proc2.is_sequential || proc2.has_reset {
                continue;
            }
            if proc2.clock_signal != proc1.clock_signal {
                continue;
            }
            if proc2.file != proc1.file {
                continue;
            }
            if proc2.assigned_signals.is_empty() {
                continue;
            }
            out.push(Violation {
                rule: "partial_reset_domain".to_string(),
                severity: "warning".to_string(),
                file: proc2.file.clone(),
                line: proc2.line,
                message: format!(
                    "Process '{}' in clock domain '{}' has no reset, but other processes in same domain do - potential state inconsistency",
                    proc2.label, proc2.clock_signal
                ),
            });
        }
    }
    out
}

fn combinational_reset_gen(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for proc in &input.processes {
        if !proc.has_reset {
            continue;
        }
        let reset_sig = &proc.reset_signal;
        for ca in &input.concurrent_assignments {
            if !ca.target.eq_ignore_ascii_case(reset_sig) {
                continue;
            }
            if ca.read_signals.len() <= 1 {
                continue;
            }
            out.push(Violation {
                rule: "combinational_reset_gen".to_string(),
                severity: "error".to_string(),
                file: ca.file.clone(),
                line: ca.line,
                message: format!(
                    "Reset signal '{}' generated by combinational logic - prone to glitches",
                    reset_sig
                ),
            });
        }
    }
    out
}

fn short_reset_sync(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for proc in &input.processes {
        if !proc.is_sequential {
            continue;
        }
        for assigned in &proc.assigned_signals {
            if !assigned.to_ascii_lowercase().contains("sync") {
                continue;
            }
            if !helpers::is_reset_name(assigned) {
                continue;
            }
            if has_second_stage(&proc.assigned_signals, assigned) {
                continue;
            }
            out.push(Violation {
                rule: "short_reset_sync".to_string(),
                severity: "warning".to_string(),
                file: proc.file.clone(),
                line: proc.line,
                message: format!(
                    "Reset synchronizer '{}' appears to be single-stage - use 2+ stages for metastability",
                    assigned
                ),
            });
        }
    }
    out
}

fn has_second_stage(signals: &[String], first_stage: &str) -> bool {
    for sig in signals {
        if sig == first_stage {
            continue;
        }
        let lower = sig.to_ascii_lowercase();
        if lower.contains("sync") && lower.contains('2') {
            return true;
        }
        if lower.contains("meta") {
            return true;
        }
    }
    false
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::policy::input::Process;

    #[test]
    fn reset_crosses_domains_flags() {
        let mut input = Input::default();
        input.processes.push(Process {
            label: "p1".to_string(),
            has_reset: true,
            reset_signal: "rst".to_string(),
            clock_signal: "clk_a".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        input.processes.push(Process {
            label: "p2".to_string(),
            has_reset: true,
            reset_signal: "rst".to_string(),
            clock_signal: "clk_b".to_string(),
            file: "a.vhd".to_string(),
            line: 2,
            ..Default::default()
        });
        let v = reset_crosses_domains(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "reset_crosses_domains");
    }
}
