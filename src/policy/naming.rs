use crate::policy::helpers::{is_clock_name, is_reset_name};
use crate::policy::input::{Input, Port};
use crate::policy::result::Violation;

pub fn violations(_input: &Input) -> Vec<Violation> {
    Vec::new()
}

pub fn optional_violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(entity_naming(input));
    out.extend(signal_input_naming(input));
    out.extend(signal_output_naming(input));
    out.extend(active_low_naming(input));
    out
}

fn entity_naming(input: &Input) -> Vec<Violation> {
    input
        .entities
        .iter()
        .filter(|entity| entity.name != entity.name.to_ascii_lowercase())
        .map(|entity| Violation {
            rule: "naming_convention".to_string(),
            severity: "info".to_string(),
            file: entity.file.clone(),
            line: entity.line,
            message: format!("Entity '{}' should use lowercase naming", entity.name),
        })
        .collect()
}

fn signal_input_naming(input: &Input) -> Vec<Violation> {
    input
        .ports
        .iter()
        .filter(|port| port.direction == "in")
        .filter(|port| !port.name.to_ascii_lowercase().ends_with("_i"))
        .filter(|port| !is_clock_name(&port.name))
        .filter(|port| !is_reset_name(&port.name))
        .map(|port| Violation {
            rule: "signal_input_naming".to_string(),
            severity: "info".to_string(),
            file: entity_file(input, port).unwrap_or_default(),
            line: port.line,
            message: format!("Input port '{}' should end with '_i' suffix", port.name),
        })
        .collect()
}

fn signal_output_naming(input: &Input) -> Vec<Violation> {
    input
        .ports
        .iter()
        .filter(|port| port.direction == "out")
        .filter(|port| !port.name.to_ascii_lowercase().ends_with("_o"))
        .map(|port| Violation {
            rule: "signal_output_naming".to_string(),
            severity: "info".to_string(),
            file: entity_file(input, port).unwrap_or_default(),
            line: port.line,
            message: format!("Output port '{}' should end with '_o' suffix", port.name),
        })
        .collect()
}

fn active_low_naming(input: &Input) -> Vec<Violation> {
    input
        .signals
        .iter()
        .filter(|sig| is_active_low_name(&sig.name))
        .filter(|sig| {
            let lower = sig.name.to_ascii_lowercase();
            !lower.ends_with("_n") && !lower.ends_with('n')
        })
        .map(|sig| Violation {
            rule: "active_low_naming".to_string(),
            severity: "info".to_string(),
            file: sig.file.clone(),
            line: sig.line,
            message: format!(
                "Active-low signal '{}' should end with '_n' suffix",
                sig.name
            ),
        })
        .collect()
}

fn is_active_low_name(name: &str) -> bool {
    let lower = name.to_ascii_lowercase();
    lower.contains("not_") || lower.starts_with("n_")
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
    use crate::policy::input::{Entity, Input, Port, Signal};

    #[test]
    fn entity_naming_flags_uppercase() {
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "CORE".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        let violations = entity_naming(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "naming_convention");
    }

    #[test]
    fn signal_input_naming_flags_missing_suffix() {
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "core".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        input.ports.push(Port {
            name: "data".to_string(),
            direction: "in".to_string(),
            in_entity: "core".to_string(),
            line: 2,
            ..Default::default()
        });
        let violations = signal_input_naming(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "signal_input_naming");
    }

    #[test]
    fn signal_output_naming_flags_missing_suffix() {
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "core".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        input.ports.push(Port {
            name: "data".to_string(),
            direction: "out".to_string(),
            in_entity: "core".to_string(),
            line: 2,
            ..Default::default()
        });
        let violations = signal_output_naming(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "signal_output_naming");
    }

    #[test]
    fn active_low_naming_flags_missing_suffix() {
        let mut input = Input::default();
        input.signals.push(Signal {
            name: "not_reset".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        let violations = active_low_naming(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "active_low_naming");
    }
}
