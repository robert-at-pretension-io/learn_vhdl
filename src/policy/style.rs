use crate::policy::helpers::is_standard_arch_name;
use crate::policy::input::Input;
use crate::policy::result::Violation;

pub fn violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(legacy_packages(input));
    out
}

pub fn optional_violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(large_entity(input));
    out.extend(process_label_missing(input));
    out.extend(architecture_naming_convention(input));
    out.extend(empty_architecture(input));
    out.extend(multiple_entities_per_file(input));
    out
}

fn large_entity(input: &Input) -> Vec<Violation> {
    input
        .entities
        .iter()
        .filter(|entity| entity.ports.len() > 50)
        .map(|entity| Violation {
            rule: "large_entity".to_string(),
            severity: "info".to_string(),
            file: entity.file.clone(),
            line: entity.line,
            message: format!(
                "Entity '{}' has {} ports - consider breaking into sub-modules",
                entity.name,
                entity.ports.len()
            ),
        })
        .collect()
}

fn process_label_missing(input: &Input) -> Vec<Violation> {
    input
        .processes
        .iter()
        .filter(|proc| proc.label.is_empty())
        .map(|proc| Violation {
            rule: "process_label_missing".to_string(),
            severity: "info".to_string(),
            file: proc.file.clone(),
            line: proc.line,
            message: format!(
                "Process at line {} has no label - add 'label: process' for debugging",
                proc.line
            ),
        })
        .collect()
}

fn multiple_entities_per_file(input: &Input) -> Vec<Violation> {
    let mut violations = Vec::new();
    let mut files: Vec<&str> = input.entities.iter().map(|e| e.file.as_str()).collect();
    files.sort();
    files.dedup();

    for file in files {
        let entities: Vec<_> = input.entities.iter().filter(|e| e.file == file).collect();
        if entities.len() > 1 {
            if let Some(first) = entities.first() {
                violations.push(Violation {
                    rule: "multiple_entities_per_file".to_string(),
                    severity: "info".to_string(),
                    file: file.to_string(),
                    line: first.line,
                    message: format!(
                        "File contains {} entities - consider one entity per file",
                        entities.len()
                    ),
                });
            }
        }
    }
    violations
}

fn legacy_packages(input: &Input) -> Vec<Violation> {
    let mut violations = Vec::new();
    for dep in &input.dependencies {
        let lower = dep.target.to_ascii_lowercase();
        let message = if lower.contains("std_logic_arith") {
            Some("Using std_logic_arith (non-standard) - use ieee.numeric_std instead")
        } else if lower.contains("std_logic_unsigned") {
            Some("Using std_logic_unsigned (non-standard) - use ieee.numeric_std instead")
        } else if lower.contains("std_logic_signed") {
            Some("Using std_logic_signed (non-standard) - use ieee.numeric_std instead")
        } else {
            None
        };
        if let Some(msg) = message {
            violations.push(Violation {
                rule: "legacy_packages".to_string(),
                severity: "warning".to_string(),
                file: dep.source.clone(),
                line: dep.line,
                message: msg.to_string(),
            });
        }
    }
    violations
}

fn architecture_naming_convention(input: &Input) -> Vec<Violation> {
    input
        .architectures
        .iter()
        .filter(|arch| !is_standard_arch_name(&arch.name))
        .map(|arch| Violation {
            rule: "architecture_naming_convention".to_string(),
            severity: "info".to_string(),
            file: arch.file.clone(),
            line: arch.line,
            message: format!(
                "Architecture '{}' uses non-standard name - consider rtl, behavioral, or structural",
                arch.name
            ),
        })
        .collect()
}

fn empty_architecture(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for arch in &input.architectures {
        let signals_in_arch = input
            .signals
            .iter()
            .filter(|s| s.in_entity == arch.name)
            .count();
        let instances_in_arch = input
            .instances
            .iter()
            .filter(|i| i.in_arch == arch.name)
            .count();
        let processes_in_arch = input
            .processes
            .iter()
            .filter(|p| p.in_arch == arch.name)
            .count();
        let assigns_in_arch = input
            .concurrent_assignments
            .iter()
            .filter(|a| a.in_arch == arch.name)
            .count();
        if signals_in_arch == 0
            && instances_in_arch == 0
            && processes_in_arch == 0
            && assigns_in_arch == 0
        {
            out.push(Violation {
                rule: "empty_architecture".to_string(),
                severity: "warning".to_string(),
                file: arch.file.clone(),
                line: arch.line,
                message: format!(
                    "Architecture '{}' is empty (no signals, instances, or processes)",
                    arch.name
                ),
            });
        }
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::policy::input::{Architecture, Dependency, Entity, Input, Port, Process, Signal};

    #[test]
    fn large_entity_flags_over_50_ports() {
        let mut input = Input::default();
        let ports: Vec<_> = (0..51)
            .map(|_| Port {
                name: "p".to_string(),
                ..Default::default()
            })
            .collect();
        input.entities.push(Entity {
            name: "core".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ports,
            ..Default::default()
        });
        let violations = large_entity(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "large_entity");
    }

    #[test]
    fn legacy_packages_flags_std_logic_arith() {
        let mut input = Input::default();
        input.dependencies.push(Dependency {
            source: "a.vhd".to_string(),
            target: "ieee.std_logic_arith".to_string(),
            line: 3,
            ..Default::default()
        });
        let violations = legacy_packages(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "legacy_packages");
    }

    #[test]
    fn process_label_missing_flags_empty() {
        let mut input = Input::default();
        input.processes.push(Process {
            label: "".to_string(),
            file: "a.vhd".to_string(),
            line: 10,
            ..Default::default()
        });
        let violations = process_label_missing(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "process_label_missing");
    }

    #[test]
    fn architecture_naming_convention_flags_nonstandard() {
        let mut input = Input::default();
        input.architectures.push(Architecture {
            name: "custom".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        let violations = architecture_naming_convention(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "architecture_naming_convention");
    }

    #[test]
    fn empty_architecture_flags_when_no_contents() {
        let mut input = Input::default();
        input.architectures.push(Architecture {
            name: "rtl".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        let violations = empty_architecture(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "empty_architecture");
    }

    #[test]
    fn multiple_entities_per_file_flags() {
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "a".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        input.entities.push(Entity {
            name: "b".to_string(),
            file: "a.vhd".to_string(),
            line: 2,
            ..Default::default()
        });
        let violations = multiple_entities_per_file(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "multiple_entities_per_file");
    }

    #[test]
    fn empty_architecture_ignored_when_signal_exists() {
        let mut input = Input::default();
        input.architectures.push(Architecture {
            name: "rtl".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        input.signals.push(Signal {
            name: "sig".to_string(),
            in_entity: "rtl".to_string(),
            ..Default::default()
        });
        let violations = empty_architecture(&input);
        assert!(violations.is_empty());
    }
}
