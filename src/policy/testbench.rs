use crate::policy::helpers::is_testbench_name;
use crate::policy::input::Input;
use crate::policy::result::Violation;

pub fn violations(input: &Input) -> Vec<Violation> {
    entity_no_ports_not_tb(input)
}

pub fn optional_violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(testbench_with_ports(input));
    out.extend(mismatched_tb_architecture(input));
    out.extend(tb_with_synth_arch(input));
    out
}

fn testbench_with_ports(input: &Input) -> Vec<Violation> {
    input
        .entities
        .iter()
        .filter(|entity| is_testbench_name(&entity.name))
        .filter(|entity| !entity.ports.is_empty())
        .map(|entity| Violation {
            rule: "testbench_with_ports".to_string(),
            severity: "info".to_string(),
            file: entity.file.clone(),
            line: entity.line,
            message: format!(
                "Entity '{}' looks like a testbench but has {} ports - testbenches typically have no ports",
                entity.name,
                entity.ports.len()
            ),
        })
        .collect()
}

fn entity_no_ports_not_tb(input: &Input) -> Vec<Violation> {
    input
        .entities
        .iter()
        .filter(|entity| entity.ports.is_empty())
        .filter(|entity| !is_testbench_name(&entity.name))
        .filter(|entity| !is_package_name(&entity.name))
        .map(|entity| Violation {
            rule: "entity_no_ports_not_tb".to_string(),
            severity: "warning".to_string(),
            file: entity.file.clone(),
            line: entity.line,
            message: format!(
                "Entity '{}' has no ports but doesn't look like a testbench",
                entity.name
            ),
        })
        .collect()
}

fn mismatched_tb_architecture(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for arch in &input.architectures {
        if !is_testbench_arch_name(&arch.name) {
            continue;
        }
        if let Some(entity) = input
            .entities
            .iter()
            .find(|e| e.name.eq_ignore_ascii_case(&arch.entity_name))
        {
            if !is_testbench_name(&entity.name) {
                out.push(Violation {
                    rule: "mismatched_tb_architecture".to_string(),
                    severity: "info".to_string(),
                    file: arch.file.clone(),
                    line: arch.line,
                    message: format!(
                        "Architecture '{}' has testbench name but entity '{}' doesn't",
                        arch.name, arch.entity_name
                    ),
                });
            }
        }
    }
    out
}

fn tb_with_synth_arch(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for arch in &input.architectures {
        if !is_synthesis_arch_name(&arch.name) {
            continue;
        }
        if let Some(entity) = input
            .entities
            .iter()
            .find(|e| e.name.eq_ignore_ascii_case(&arch.entity_name))
        {
            if is_testbench_name(&entity.name) {
                out.push(Violation {
                    rule: "tb_with_synth_arch".to_string(),
                    severity: "info".to_string(),
                    file: arch.file.clone(),
                    line: arch.line,
                    message: format!(
                        "Testbench entity '{}' has synthesis-style architecture name '{}'",
                        entity.name, arch.name
                    ),
                });
            }
        }
    }
    out
}

fn is_testbench_arch_name(name: &str) -> bool {
    matches!(
        name.to_ascii_lowercase().as_str(),
        "tb" | "testbench" | "test" | "sim"
    )
}

fn is_synthesis_arch_name(name: &str) -> bool {
    matches!(
        name.to_ascii_lowercase().as_str(),
        "rtl" | "synth" | "synthesis"
    )
}

fn is_package_name(name: &str) -> bool {
    let lower = name.to_ascii_lowercase();
    lower.contains("_pkg") || lower.ends_with("pkg") || lower.contains("package")
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::policy::input::{Architecture, Entity, Input, Port};

    #[test]
    fn testbench_with_ports_flags() {
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "core_tb".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ports: vec![Port::default()],
            ..Default::default()
        });
        let violations = testbench_with_ports(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "testbench_with_ports");
    }

    #[test]
    fn entity_no_ports_not_tb_flags() {
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "core".to_string(),
            file: "a.vhd".to_string(),
            line: 2,
            ..Default::default()
        });
        let violations = entity_no_ports_not_tb(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "entity_no_ports_not_tb");
    }

    #[test]
    fn mismatched_tb_architecture_flags() {
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "core".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        input.architectures.push(Architecture {
            name: "testbench".to_string(),
            entity_name: "core".to_string(),
            file: "a.vhd".to_string(),
            line: 3,
        });
        let violations = mismatched_tb_architecture(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "mismatched_tb_architecture");
    }

    #[test]
    fn tb_with_synth_arch_flags() {
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "core_tb".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        input.architectures.push(Architecture {
            name: "rtl".to_string(),
            entity_name: "core_tb".to_string(),
            file: "a.vhd".to_string(),
            line: 3,
        });
        let violations = tb_with_synth_arch(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "tb_with_synth_arch");
    }
}
