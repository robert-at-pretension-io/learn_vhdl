use crate::policy::helpers::{is_clock_name, is_reset_name};
use crate::policy::input::{Input, Instance};
use crate::policy::result::Violation;

pub fn violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(undriven_output_port(input));
    out.extend(output_port_read(input));
    out.extend(inout_as_output(input));
    out.extend(inout_as_input(input));
    out
}

pub fn optional_violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(unused_input_port(input));
    out
}

fn unused_input_port(input: &Input) -> Vec<Violation> {
    input
        .ports
        .iter()
        .filter(|port| port.direction == "in")
        .filter(|port| entity_has_architecture(input, &port.in_entity))
        .filter(|port| !is_clock_name(&port.name))
        .filter(|port| !is_reset_name(&port.name))
        .filter(|port| !port_is_read(input, &port.name))
        .map(|port| Violation {
            rule: "unused_input_port".to_string(),
            severity: "warning".to_string(),
            file: entity_file(input, &port.in_entity).unwrap_or_default(),
            line: port.line,
            message: format!("Input port '{}' is never read", port.name),
        })
        .collect()
}

fn undriven_output_port(input: &Input) -> Vec<Violation> {
    input
        .ports
        .iter()
        .filter(|port| port.direction == "out")
        .filter(|port| entity_has_architecture(input, &port.in_entity))
        .filter(|port| !port_is_assigned(input, &port.name))
        .map(|port| Violation {
            rule: "undriven_output_port".to_string(),
            severity: "error".to_string(),
            file: entity_file(input, &port.in_entity).unwrap_or_default(),
            line: port.line,
            message: format!(
                "Output port '{}' is never assigned (floating output)",
                port.name
            ),
        })
        .collect()
}

fn output_port_read(input: &Input) -> Vec<Violation> {
    if !is_legacy_standard(input) {
        return Vec::new();
    }
    input
        .ports
        .iter()
        .filter(|port| port.direction == "out")
        .filter(|port| entity_has_architecture(input, &port.in_entity))
        .filter(|port| port_is_read(input, &port.name))
        .map(|port| Violation {
            rule: "output_port_read".to_string(),
            severity: "info".to_string(),
            file: entity_file(input, &port.in_entity).unwrap_or_default(),
            line: port.line,
            message: format!(
                "Output port '{}' is read internally (use buffer or internal signal for VHDL-93 compatibility)",
                port.name
            ),
        })
        .collect()
}

fn inout_as_output(input: &Input) -> Vec<Violation> {
    input
        .ports
        .iter()
        .filter(|port| port.direction.eq_ignore_ascii_case("inout"))
        .filter(|port| entity_has_architecture(input, &port.in_entity))
        .filter(|port| port_is_assigned(input, &port.name))
        .filter(|port| !port_is_read(input, &port.name))
        .map(|port| Violation {
            rule: "inout_as_output".to_string(),
            severity: "info".to_string(),
            file: entity_file(input, &port.in_entity).unwrap_or_default(),
            line: port.line,
            message: format!(
                "Inout port '{}' is only written, never read - consider 'out' direction",
                port.name
            ),
        })
        .collect()
}

fn inout_as_input(input: &Input) -> Vec<Violation> {
    input
        .ports
        .iter()
        .filter(|port| port.direction.eq_ignore_ascii_case("inout"))
        .filter(|port| entity_has_architecture(input, &port.in_entity))
        .filter(|port| port_is_read(input, &port.name))
        .filter(|port| !port_is_assigned(input, &port.name))
        .map(|port| Violation {
            rule: "inout_as_input".to_string(),
            severity: "info".to_string(),
            file: entity_file(input, &port.in_entity).unwrap_or_default(),
            line: port.line,
            message: format!(
                "Inout port '{}' is only read, never written - consider 'in' direction",
                port.name
            ),
        })
        .collect()
}

fn port_is_read(input: &Input, port_name: &str) -> bool {
    let port_lower = port_name.to_ascii_lowercase();
    input.processes.iter().any(|proc| {
        proc.read_signals
            .iter()
            .any(|sig| sig.eq_ignore_ascii_case(&port_lower))
            || proc
                .sensitivity_list
                .iter()
                .any(|sig| sig.eq_ignore_ascii_case(&port_lower))
    }) || instance_reads_port(&input.instances, &port_lower)
        || input.concurrent_assignments.iter().any(|ca| {
            ca.read_signals
                .iter()
                .any(|sig| sig.eq_ignore_ascii_case(&port_lower))
        })
}

fn instance_reads_port(instances: &[Instance], port_lower: &str) -> bool {
    instances.iter().any(|inst| {
        inst.port_map
            .values()
            .any(|actual| actual.to_ascii_lowercase().contains(port_lower))
    })
}

fn port_is_assigned(input: &Input, port_name: &str) -> bool {
    let port_lower = port_name.to_ascii_lowercase();
    input.processes.iter().any(|proc| {
        proc.assigned_signals
            .iter()
            .any(|sig| sig.eq_ignore_ascii_case(&port_lower))
    }) || input.instances.iter().any(|inst| {
        inst.port_map
            .values()
            .any(|formal| formal.eq_ignore_ascii_case(&port_lower))
    }) || input
        .concurrent_assignments
        .iter()
        .any(|ca| ca.target.eq_ignore_ascii_case(&port_lower))
}

fn entity_has_architecture(input: &Input, entity_name: &str) -> bool {
    input
        .architectures
        .iter()
        .any(|arch| arch.entity_name.eq_ignore_ascii_case(entity_name))
}

fn entity_file(input: &Input, entity_name: &str) -> Option<String> {
    input
        .entities
        .iter()
        .find(|entity| entity.name.eq_ignore_ascii_case(entity_name))
        .map(|entity| entity.file.clone())
}

fn is_legacy_standard(input: &Input) -> bool {
    matches!(input.standard.as_str(), "1993" | "2002")
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::policy::input::{Architecture, ConcurrentAssignment, Entity, Port, Process};

    fn base_input() -> Input {
        Input {
            standard: "1993".to_string(),
            ..Default::default()
        }
    }

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
    fn unused_input_port_warns() {
        let mut input = base_input();
        add_entity_arch(&mut input, "core");
        input.ports.push(Port {
            name: "data_in".to_string(),
            direction: "in".to_string(),
            in_entity: "core".to_string(),
            line: 3,
            ..Default::default()
        });
        let violations = unused_input_port(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "unused_input_port");
    }

    #[test]
    fn undriven_output_port_errors() {
        let mut input = base_input();
        add_entity_arch(&mut input, "core");
        input.ports.push(Port {
            name: "data_out".to_string(),
            direction: "out".to_string(),
            in_entity: "core".to_string(),
            line: 4,
            ..Default::default()
        });
        let violations = undriven_output_port(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "undriven_output_port");
    }

    #[test]
    fn output_port_read_flags_legacy() {
        let mut input = base_input();
        add_entity_arch(&mut input, "core");
        input.ports.push(Port {
            name: "data_out".to_string(),
            direction: "out".to_string(),
            in_entity: "core".to_string(),
            line: 5,
            ..Default::default()
        });
        input.processes.push(Process {
            read_signals: vec!["data_out".to_string()],
            ..Default::default()
        });
        let violations = output_port_read(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "output_port_read");
    }

    #[test]
    fn inout_only_written_flags_output() {
        let mut input = base_input();
        add_entity_arch(&mut input, "core");
        input.ports.push(Port {
            name: "io".to_string(),
            direction: "inout".to_string(),
            in_entity: "core".to_string(),
            line: 6,
            ..Default::default()
        });
        input.processes.push(Process {
            assigned_signals: vec!["io".to_string()],
            ..Default::default()
        });
        let violations = inout_as_output(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "inout_as_output");
    }

    #[test]
    fn inout_only_read_flags_input() {
        let mut input = base_input();
        add_entity_arch(&mut input, "core");
        input.ports.push(Port {
            name: "io".to_string(),
            direction: "inout".to_string(),
            in_entity: "core".to_string(),
            line: 7,
            ..Default::default()
        });
        input.processes.push(Process {
            read_signals: vec!["io".to_string()],
            ..Default::default()
        });
        let violations = inout_as_input(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "inout_as_input");
    }

    #[test]
    fn port_read_via_concurrent_assignment() {
        let mut input = base_input();
        add_entity_arch(&mut input, "core");
        input.ports.push(Port {
            name: "data_in".to_string(),
            direction: "in".to_string(),
            in_entity: "core".to_string(),
            line: 8,
            ..Default::default()
        });
        input.concurrent_assignments.push(ConcurrentAssignment {
            read_signals: vec!["data_in".to_string()],
            ..Default::default()
        });
        let violations = unused_input_port(&input);
        assert!(violations.is_empty());
    }
}
