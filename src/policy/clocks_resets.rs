use crate::policy::helpers::{is_clock_name, is_reset_name, is_single_bit_type};
use crate::policy::input::{Input, Port};
use crate::policy::result::Violation;

pub fn violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(clock_not_std_logic(input));
    out.extend(reset_not_std_logic(input));
    out.extend(multiple_clocks_in_process(input));
    out
}

pub fn optional_violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(async_reset_active_high(input));
    out.extend(missing_reset(input));
    out
}

fn clock_not_std_logic(input: &Input) -> Vec<Violation> {
    input
        .ports
        .iter()
        .filter(|port| is_clock_name(&port.name))
        .filter(|port| port.direction == "in")
        .filter(|port| !is_single_bit_type(&port.r#type))
        .map(|port| Violation {
            rule: "clock_not_std_logic".to_string(),
            severity: "error".to_string(),
            file: entity_file(input, port).unwrap_or_default(),
            line: port.line,
            message: format!(
                "Clock signal '{}' should be std_logic, not '{}'",
                port.name, port.r#type
            ),
        })
        .collect()
}

fn reset_not_std_logic(input: &Input) -> Vec<Violation> {
    input
        .ports
        .iter()
        .filter(|port| is_reset_name(&port.name))
        .filter(|port| port.direction == "in")
        .filter(|port| !is_single_bit_type(&port.r#type))
        .map(|port| Violation {
            rule: "reset_not_std_logic".to_string(),
            severity: "error".to_string(),
            file: entity_file(input, port).unwrap_or_default(),
            line: port.line,
            message: format!(
                "Reset signal '{}' should be std_logic, not '{}'",
                port.name, port.r#type
            ),
        })
        .collect()
}

fn multiple_clocks_in_process(input: &Input) -> Vec<Violation> {
    input
        .processes
        .iter()
        .filter(|proc| proc.is_sequential)
        .filter_map(|proc| {
            let clocks: Vec<String> = proc
                .sensitivity_list
                .iter()
                .filter(|sig| is_clock_name(sig))
                .cloned()
                .collect();
            if clocks.len() > 1 {
                Some(Violation {
                    rule: "multiple_clocks_in_process".to_string(),
                    severity: "error".to_string(),
                    file: proc.file.clone(),
                    line: proc.line,
                    message: format!(
                        "Process '{}' appears to use multiple clocks {:?} - potential CDC issue",
                        proc.label, clocks
                    ),
                })
            } else {
                None
            }
        })
        .collect()
}

fn missing_reset(input: &Input) -> Vec<Violation> {
    input
        .processes
        .iter()
        .filter(|proc| proc.is_sequential)
        .filter(|proc| !proc.has_reset)
        .filter(|proc| !proc.assigned_signals.is_empty())
        .map(|proc| Violation {
            rule: "missing_reset".to_string(),
            severity: "warning".to_string(),
            file: proc.file.clone(),
            line: proc.line,
            message: format!(
                "Sequential process '{}' has no reset - power-on state will be unknown",
                proc.label
            ),
        })
        .collect()
}

fn async_reset_active_high(input: &Input) -> Vec<Violation> {
    input
        .processes
        .iter()
        .filter(|proc| proc.is_sequential)
        .filter(|proc| proc.has_reset)
        .filter(|proc| !proc.reset_signal.is_empty())
        .filter(|proc| {
            let lower = proc.reset_signal.to_ascii_lowercase();
            !lower.ends_with('n') && !lower.ends_with("_n") && !lower.contains("_n_")
        })
        .map(|proc| Violation {
            rule: "async_reset_active_high".to_string(),
            severity: "info".to_string(),
            file: proc.file.clone(),
            line: proc.line,
            message: format!(
                "Reset '{}' in process '{}' may be active-high - consider using active-low reset (rstn, rst_n)",
                proc.reset_signal, proc.label
            ),
        })
        .collect()
}

fn entity_file(input: &Input, port: &Port) -> Option<String> {
    input
        .entities
        .iter()
        .find(|entity| entity.name.eq_ignore_ascii_case(&port.in_entity))
        .map(|entity| entity.file.clone())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::policy::input::{Architecture, Entity, Input, Process};

    fn add_entity_arch(input: &mut Input, name: &str) {
        input.entities.push(Entity {
            name: name.to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        input.architectures.push(Architecture {
            name: "rtl".to_string(),
            entity_name: name.to_string(),
            file: "a.vhd".to_string(),
            line: 2,
        });
    }

    #[test]
    fn clock_not_std_logic_flags() {
        let mut input = Input::default();
        add_entity_arch(&mut input, "core");
        input.ports.push(Port {
            name: "clk".to_string(),
            direction: "in".to_string(),
            r#type: "std_logic_vector".to_string(),
            in_entity: "core".to_string(),
            line: 3,
            ..Default::default()
        });
        let violations = clock_not_std_logic(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "clock_not_std_logic");
    }

    #[test]
    fn multiple_clocks_in_process_flags() {
        let mut input = Input::default();
        input.processes.push(Process {
            label: "p1".to_string(),
            is_sequential: true,
            sensitivity_list: vec!["clk_a".to_string(), "clk_b".to_string()],
            file: "a.vhd".to_string(),
            line: 4,
            ..Default::default()
        });
        let violations = multiple_clocks_in_process(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "multiple_clocks_in_process");
    }

    #[test]
    fn missing_reset_flags() {
        let mut input = Input::default();
        input.processes.push(Process {
            label: "p1".to_string(),
            is_sequential: true,
            has_reset: false,
            assigned_signals: vec!["sig".to_string()],
            file: "a.vhd".to_string(),
            line: 5,
            ..Default::default()
        });
        let violations = missing_reset(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "missing_reset");
    }
}
