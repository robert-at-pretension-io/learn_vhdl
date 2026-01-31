use crate::policy::input::Input;
use crate::policy::result::Violation;

pub fn violations(_input: &Input) -> Vec<Violation> {
    Vec::new()
}

pub fn optional_violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(unguarded_division(input));
    out.extend(unguarded_multiplication(input));
    out.extend(unguarded_exponent(input));
    out.extend(power_hotspot(input));
    out.extend(combinational_multiplier(input));
    out.extend(weak_guard(input));
    out.extend(dsp_candidate_no_control(input));
    out.extend(clock_gating_opportunity(input));
    out
}

fn unguarded_multiplication(input: &Input) -> Vec<Violation> {
    input
        .arithmetic_ops
        .iter()
        .filter(|op| op.operator == "*" && !op.is_guarded && op.operands.len() >= 2)
        .map(|op| Violation {
            rule: "unguarded_multiplication".to_string(),
            severity: "warning".to_string(),
            file: op.file.clone(),
            line: op.line,
            message: "Multiplier without operand isolation - runs every cycle even when unused. Guard with enable signal.".to_string(),
        })
        .collect()
}

fn unguarded_division(input: &Input) -> Vec<Violation> {
    input
        .arithmetic_ops
        .iter()
        .filter(|op| is_division_op(&op.operator) && !op.is_guarded)
        .map(|op| Violation {
            rule: "unguarded_division".to_string(),
            severity: "error".to_string(),
            file: op.file.clone(),
            line: op.line,
            message: format!(
                "Division/modulo operator '{}' without operand isolation - VERY expensive, runs every cycle!",
                op.operator
            ),
        })
        .collect()
}

fn is_division_op(op: &str) -> bool {
    matches!(op.to_ascii_lowercase().as_str(), "/" | "mod" | "rem")
}

fn unguarded_exponent(input: &Input) -> Vec<Violation> {
    input
        .arithmetic_ops
        .iter()
        .filter(|op| op.operator == "**" && !op.is_guarded)
        .map(|op| Violation {
            rule: "unguarded_exponent".to_string(),
            severity: "warning".to_string(),
            file: op.file.clone(),
            line: op.line,
            message:
                "Exponentiation '**' without operand isolation - implement with proper enable gating"
                    .to_string(),
        })
        .collect()
}

fn power_hotspot(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for proc in &input.processes {
        let count = input
            .arithmetic_ops
            .iter()
            .filter(|op| op.in_process == proc.label && is_expensive_op(&op.operator))
            .count();
        if count > 3 {
            out.push(Violation {
                rule: "power_hotspot".to_string(),
                severity: "warning".to_string(),
                file: proc.file.clone(),
                line: proc.line,
                message: format!(
                    "Process '{}' contains {} expensive operations - power hotspot, consider operand isolation",
                    proc.label, count
                ),
            });
        }
    }
    out
}

fn is_expensive_op(op: &str) -> bool {
    matches!(
        op.to_ascii_lowercase().as_str(),
        "*" | "/" | "mod" | "rem" | "**"
    )
}

fn combinational_multiplier(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for op in &input.arithmetic_ops {
        if op.operator != "*" {
            continue;
        }
        if let Some(proc) = input.processes.iter().find(|p| p.label == op.in_process) {
            if proc.is_combinational {
                out.push(Violation {
                    rule: "combinational_multiplier".to_string(),
                    severity: "warning".to_string(),
                    file: op.file.clone(),
                    line: op.line,
                    message: "Multiplier in combinational process - active continuously, consider clocked implementation with enable"
                        .to_string(),
                });
            }
        }
    }
    out
}

fn weak_guard(input: &Input) -> Vec<Violation> {
    input
        .arithmetic_ops
        .iter()
        .filter(|op| op.is_guarded && !op.guard_signal.is_empty() && is_expensive_op(&op.operator))
        .filter(|op| is_weak_guard(&op.guard_signal))
        .map(|op| Violation {
            rule: "weak_guard".to_string(),
            severity: "info".to_string(),
            file: op.file.clone(),
            line: op.line,
            message: format!(
                "Expensive operation guarded by '{}' - verify this actually gates operand toggling",
                op.guard_signal
            ),
        })
        .collect()
}

fn is_weak_guard(sig: &str) -> bool {
    matches!(
        sig.to_ascii_lowercase().as_str(),
        "enable" | "en" | "valid" | "vld" | "ready" | "rdy"
    )
}

fn dsp_candidate_no_control(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for op in &input.arithmetic_ops {
        if op.operator != "*" || op.operands.len() != 2 || op.is_guarded {
            continue;
        }
        let operand = &op.operands[0];
        if let Some(sig) = input
            .signals
            .iter()
            .find(|sig| sig.name.eq_ignore_ascii_case(operand))
        {
            if is_wide_type(&sig.r#type) {
                out.push(Violation {
                    rule: "dsp_candidate_no_control".to_string(),
                    severity: "info".to_string(),
                    file: op.file.clone(),
                    line: op.line,
                    message: format!(
                        "Wide signal '{}' multiplication - likely DSP block, add clock enable for power savings",
                        operand
                    ),
                });
            }
        }
    }
    out
}

fn is_wide_type(t: &str) -> bool {
    let lower = t.to_ascii_lowercase();
    lower.contains("unsigned") || lower.contains("signed") || lower.contains("std_logic_vector")
}

fn clock_gating_opportunity(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for proc in &input.processes {
        if !proc.is_sequential || proc.assigned_signals.len() <= 5 {
            continue;
        }
        for read in &proc.read_signals {
            if is_enable_signal(read) {
                out.push(Violation {
                    rule: "clock_gating_opportunity".to_string(),
                    severity: "info".to_string(),
                    file: proc.file.clone(),
                    line: proc.line,
                    message: format!(
                        "Process '{}' has {} registers with enable '{}' - candidate for clock gating",
                        proc.label,
                        proc.assigned_signals.len(),
                        read
                    ),
                });
            }
        }
    }
    out
}

fn is_enable_signal(sig: &str) -> bool {
    let lower = sig.to_ascii_lowercase();
    ["enable", "en", "ce", "clken", "clk_en", "clock_enable"]
        .iter()
        .any(|pattern| lower.contains(pattern))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::policy::input::{ArithmeticOp, Input};

    #[test]
    fn unguarded_division_flags() {
        let mut input = Input::default();
        input.arithmetic_ops.push(ArithmeticOp {
            operator: "/".to_string(),
            is_guarded: false,
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        let v = unguarded_division(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "unguarded_division");
    }
}
