use crate::policy::helpers;
use crate::policy::input::Input;
use crate::policy::result::Violation;

pub fn violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(magic_number_comparison(input));
    out.extend(trigger_drives_output(input));
    out.extend(multi_trigger_process(input));
    out
}

pub fn optional_violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(large_literal_comparison(input));
    out.extend(counter_trigger(input));
    out.extend(inverted_trigger(input));
    out
}

fn large_literal_comparison(input: &Input) -> Vec<Violation> {
    input
        .comparisons
        .iter()
        .filter(|comp| comp.is_literal && comp.literal_bits > 16)
        .map(|comp| Violation {
            rule: "large_literal_comparison".to_string(),
            severity: "warning".to_string(),
            file: comp.file.clone(),
            line: comp.line,
            message: format!(
                "Suspicious comparison: '{}' {} literal '{}' ({} bits) - potential trojan trigger",
                comp.left_operand, comp.operator, comp.literal_value, comp.literal_bits
            ),
        })
        .collect()
}

fn magic_number_comparison(input: &Input) -> Vec<Violation> {
    input
        .comparisons
        .iter()
        .filter(|comp| comp.is_literal && is_magic_number(&comp.literal_value))
        .map(|comp| Violation {
            rule: "magic_number_comparison".to_string(),
            severity: "error".to_string(),
            file: comp.file.clone(),
            line: comp.line,
            message: format!(
                "CRITICAL: Comparison against known magic number '{}' - HIGH PROBABILITY TROJAN TRIGGER",
                comp.literal_value
            ),
        })
        .collect()
}

fn is_magic_number(lit: &str) -> bool {
    let upper = lit.to_ascii_uppercase();
    let patterns = [
        "DEADBEEF", "CAFEBABE", "FEEDFACE", "BAADF00D", "DEADC0DE", "C0FFEE00", "BADC0DE0",
        "0BADF00D", "DEAD", "BEEF", "CAFE", "BABE", "FEED", "FACE",
    ];
    patterns.iter().any(|pattern| upper.contains(pattern))
}

fn trigger_drives_output(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for comp in input.comparisons.iter().filter(|c| {
        c.is_literal && c.literal_bits > 8 && c.operator == "=" && !c.result_drives.is_empty()
    }) {
        if input.ports.iter().any(|port| {
            port.name.eq_ignore_ascii_case(&comp.result_drives) && port.direction == "out"
        }) {
            out.push(Violation {
                rule: "trigger_drives_output".to_string(),
                severity: "error".to_string(),
                file: comp.file.clone(),
                line: comp.line,
                message: format!(
                    "ALERT: Literal comparison '{}' = '{}' drives output port '{}' - classic trojan pattern",
                    comp.left_operand, comp.literal_value, comp.result_drives
                ),
            });
        }
    }
    out
}

fn counter_trigger(input: &Input) -> Vec<Violation> {
    input
        .comparisons
        .iter()
        .filter(|comp| comp.is_literal && comp.literal_bits >= 16)
        .filter(|comp| helpers::is_counter_name(&comp.left_operand))
        .map(|comp| Violation {
            rule: "counter_trigger".to_string(),
            severity: "warning".to_string(),
            file: comp.file.clone(),
            line: comp.line,
            message: format!(
                "Counter '{}' compared against large literal '{}' - potential time bomb trigger",
                comp.left_operand, comp.literal_value
            ),
        })
        .collect()
}

fn inverted_trigger(input: &Input) -> Vec<Violation> {
    input
        .comparisons
        .iter()
        .filter(|comp| comp.is_literal && comp.literal_bits > 16 && comp.operator == "/=")
        .map(|comp| Violation {
            rule: "inverted_trigger".to_string(),
            severity: "warning".to_string(),
            file: comp.file.clone(),
            line: comp.line,
            message: format!(
                "Inverted comparison '/=' against large literal '{}' - could hide trojan by inverting trigger logic",
                comp.literal_value
            ),
        })
        .collect()
}

fn multi_trigger_process(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for proc in &input.processes {
        if helpers::process_in_testbench(input, proc)
            || helpers::file_in_testbench(input, &proc.file)
        {
            continue;
        }
        let count = input
            .comparisons
            .iter()
            .filter(|comp| {
                comp.in_process == proc.label && comp.is_literal && comp.literal_bits > 12
            })
            .count();
        if count > 2 {
            out.push(Violation {
                rule: "multi_trigger_process".to_string(),
                severity: "error".to_string(),
                file: proc.file.clone(),
                line: proc.line,
                message: format!(
                    "Process '{}' contains {} large literal comparisons - suspicious concentration of potential triggers",
                    proc.label, count
                ),
            });
        }
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::policy::input::{Architecture, Comparison, Entity, Input, Port, Process};

    #[test]
    fn magic_number_comparison_flags() {
        let mut input = Input::default();
        input.comparisons.push(Comparison {
            is_literal: true,
            literal_value: "DEADBEEF".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        let v = magic_number_comparison(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "magic_number_comparison");
    }

    #[test]
    fn trigger_drives_output_flags() {
        let mut input = Input::default();
        input.ports.push(Port {
            name: "out_sig".to_string(),
            direction: "out".to_string(),
            ..Default::default()
        });
        input.comparisons.push(Comparison {
            is_literal: true,
            literal_bits: 16,
            operator: "=".to_string(),
            result_drives: "out_sig".to_string(),
            literal_value: "FF".to_string(),
            file: "a.vhd".to_string(),
            line: 2,
            ..Default::default()
        });
        let v = trigger_drives_output(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "trigger_drives_output");
    }

    #[test]
    fn multi_trigger_process_flags() {
        let mut input = Input::default();
        input.processes.push(Process {
            label: "p1".to_string(),
            file: "a.vhd".to_string(),
            line: 3,
            ..Default::default()
        });
        for _ in 0..3 {
            input.comparisons.push(Comparison {
                in_process: "p1".to_string(),
                is_literal: true,
                literal_bits: 13,
                file: "a.vhd".to_string(),
                line: 4,
                ..Default::default()
            });
        }
        let v = multi_trigger_process(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "multi_trigger_process");
    }

    #[test]
    fn multi_trigger_process_ignores_testbench() {
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "tb_top".to_string(),
            file: "tb.vhd".to_string(),
            ..Default::default()
        });
        input.architectures.push(Architecture {
            name: "rtl".to_string(),
            entity_name: "tb_top".to_string(),
            file: "tb.vhd".to_string(),
            line: 1,
        });
        input.processes.push(Process {
            label: "p1".to_string(),
            in_arch: "rtl".to_string(),
            file: "tb.vhd".to_string(),
            line: 2,
            ..Default::default()
        });
        input.comparisons.push(Comparison {
            in_process: "p1".to_string(),
            is_literal: true,
            literal_bits: 32,
            ..Default::default()
        });
        input.comparisons.push(Comparison {
            in_process: "p1".to_string(),
            is_literal: true,
            literal_bits: 32,
            ..Default::default()
        });
        input.comparisons.push(Comparison {
            in_process: "p1".to_string(),
            is_literal: true,
            literal_bits: 32,
            ..Default::default()
        });
        let v = multi_trigger_process(&input);
        assert!(v.is_empty());
    }
}
