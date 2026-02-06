use regex::Regex;

use crate::policy::helpers::valid_instance_prefix;
use crate::policy::input::Input;
use crate::policy::result::Violation;

pub fn violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(positional_mapping(input));
    out.extend(instance_naming_convention(input));
    out
}

pub fn optional_violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(generic_without_default(input));
    out.extend(constant_port_connection(input));
    out.extend(entity_port_not_connected(input));
    out
}

pub fn positional_mapping(input: &Input) -> Vec<Violation> {
    input
        .instances
        .iter()
        .filter(|inst| inst.port_map.is_empty())
        .map(|inst| Violation {
            rule: "positional_mapping".to_string(),
            severity: "warning".to_string(),
            file: inst.file.clone(),
            line: inst.line,
            message: format!(
                "Instance '{}' uses positional port mapping - use named mapping for safety",
                inst.name
            ),
        })
        .collect()
}

pub fn instance_naming_convention(input: &Input) -> Vec<Violation> {
    input
        .instances
        .iter()
        .filter(|inst| !valid_instance_prefix(&inst.name))
        .map(|inst| Violation {
            rule: "instance_naming_convention".to_string(),
            severity: "info".to_string(),
            file: inst.file.clone(),
            line: inst.line,
            message: format!(
                "Instance '{}' should use a standard prefix (u_, i_, or inst_)",
                inst.name
            ),
        })
        .collect()
}

fn generic_without_default(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for entity in &input.entities {
        for generic in &entity.generics {
            if generic.default.trim().is_empty() {
                out.push(Violation {
                    rule: "generic_without_default".to_string(),
                    severity: "info".to_string(),
                    file: entity.file.clone(),
                    line: generic.line,
                    message: format!(
                        "Generic '{}' in entity '{}' has no default value - must be provided at every instantiation",
                        generic.name, entity.name
                    ),
                });
            }
        }
    }
    out
}

fn constant_port_connection(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for inst in &input.instances {
        let target_lower = inst.target.to_ascii_lowercase();
        for (port_name, actual) in &inst.port_map {
            if !is_output_literal(actual) {
                continue;
            }
            for entity in &input.entities {
                if !target_matches(&target_lower, &entity.name.to_ascii_lowercase()) {
                    continue;
                }
                for port in &entity.ports {
                    if !port.name.eq_ignore_ascii_case(port_name) {
                        continue;
                    }
                    if port.direction.eq_ignore_ascii_case("out")
                        || port.direction.eq_ignore_ascii_case("inout")
                    {
                        out.push(Violation {
                            rule: "constant_port_connection".to_string(),
                            severity: "info".to_string(),
                            file: inst.file.clone(),
                            line: inst.line,
                            message: format!(
                                "Instance '{}' connects literal '{}' to output port '{}' - output will be ignored",
                                inst.name, actual, port_name
                            ),
                        });
                    }
                }
            }
        }
    }
    out
}

fn is_output_literal(val: &str) -> bool {
    val.starts_with('\'')
        || val.starts_with('"')
        || val.starts_with("x\"")
        || val.starts_with("X\"")
        || Regex::new(r"^[0-9]+$").unwrap().is_match(val)
}

fn target_matches(target: &str, entity_name: &str) -> bool {
    target == entity_name || target.ends_with(&format!(".{}", entity_name))
}

fn entity_port_not_connected(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for entity in &input.entities {
        let instances_of: Vec<&crate::policy::input::Instance> = input
            .instances
            .iter()
            .filter(|inst| {
                let t = inst.target.to_ascii_lowercase();
                t == entity.name.to_ascii_lowercase()
                    || t.ends_with(&format!(".{}", entity.name.to_ascii_lowercase()))
            })
            .collect();
        if instances_of.is_empty() {
            continue;
        }
        for port in &entity.ports {
            if port.direction.eq_ignore_ascii_case("in") {
                continue;
            }
            let connected = instances_of
                .iter()
                .any(|inst| inst.port_map.keys().any(|k| k.eq_ignore_ascii_case(&port.name)));
            if !connected {
                out.push(Violation {
                    rule: "entity_port_not_connected".to_string(),
                    severity: "warning".to_string(),
                    file: entity.file.clone(),
                    line: port.line,
                    message: format!(
                        "Output port '{}' of entity '{}' is not connected in any instance",
                        port.name, entity.name
                    ),
                });
            }
        }
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;

    fn input_with_instance(name: &str, port_map_len: usize) -> Input {
        let mut input = Input::default();
        let mut port_map = std::collections::HashMap::new();
        for idx in 0..port_map_len {
            port_map.insert(format!("p{}", idx), format!("a{}", idx));
        }
        input.instances.push(crate::policy::input::Instance {
            name: name.to_string(),
            file: "test.vhd".to_string(),
            line: 10,
            port_map,
            ..Default::default()
        });
        input
    }

    #[test]
    fn positional_mapping_flags_empty_map() {
        let input = input_with_instance("u_ok", 0);
        let violations = positional_mapping(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "positional_mapping");
    }

    #[test]
    fn positional_mapping_ignores_named_map() {
        let input = input_with_instance("u_ok", 2);
        let violations = positional_mapping(&input);
        assert!(violations.is_empty());
    }

    #[test]
    fn instance_naming_convention_flags_bad_prefix() {
        let input = input_with_instance("core0", 1);
        let violations = instance_naming_convention(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "instance_naming_convention");
    }

    #[test]
    fn instance_naming_convention_accepts_valid_prefixes() {
        let mut input = Input::default();
        input.instances.push(crate::policy::input::Instance {
            name: "u_core".to_string(),
            file: "test.vhd".to_string(),
            line: 10,
            port_map: std::iter::once(("p0".to_string(), "a0".to_string())).collect(),
            ..Default::default()
        });
        input.instances.push(crate::policy::input::Instance {
            name: "i_core".to_string(),
            file: "test.vhd".to_string(),
            line: 11,
            port_map: std::iter::once(("p0".to_string(), "a0".to_string())).collect(),
            ..Default::default()
        });
        input.instances.push(crate::policy::input::Instance {
            name: "inst_core".to_string(),
            file: "test.vhd".to_string(),
            line: 12,
            port_map: std::iter::once(("p0".to_string(), "a0".to_string())).collect(),
            ..Default::default()
        });
        let violations = instance_naming_convention(&input);
        assert!(violations.is_empty());
    }

    #[test]
    fn generic_without_default_flags_missing() {
        use crate::policy::input::{Entity, GenericDecl};
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "core".to_string(),
            file: "test.vhd".to_string(),
            line: 1,
            generics: vec![
                GenericDecl {
                    name: "WIDTH".to_string(),
                    default: "".to_string(),
                    line: 5,
                    ..Default::default()
                },
                GenericDecl {
                    name: "DEPTH".to_string(),
                    default: "8".to_string(),
                    line: 6,
                    ..Default::default()
                },
            ],
            ..Default::default()
        });
        let v = generic_without_default(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "generic_without_default");
        assert!(v[0].message.contains("WIDTH"));
    }

    #[test]
    fn generic_without_default_ignores_defaulted() {
        use crate::policy::input::{Entity, GenericDecl};
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "core".to_string(),
            file: "test.vhd".to_string(),
            line: 1,
            generics: vec![GenericDecl {
                name: "WIDTH".to_string(),
                default: "16".to_string(),
                line: 5,
                ..Default::default()
            }],
            ..Default::default()
        });
        let v = generic_without_default(&input);
        assert!(v.is_empty());
    }

    #[test]
    fn constant_port_connection_flags_literal_on_output() {
        use crate::policy::input::{Entity, Instance, Port};
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "child".to_string(),
            file: "child.vhd".to_string(),
            line: 1,
            ports: vec![Port {
                name: "data_o".to_string(),
                direction: "out".to_string(),
                ..Default::default()
            }],
            ..Default::default()
        });
        let mut inst = Instance::default();
        inst.name = "u_child".to_string();
        inst.target = "work.child".to_string();
        inst.file = "top.vhd".to_string();
        inst.line = 10;
        inst.port_map
            .insert("data_o".to_string(), "'0'".to_string());
        input.instances.push(inst);
        let v = constant_port_connection(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "constant_port_connection");
        assert!(v[0].message.contains("data_o"));
    }

    #[test]
    fn constant_port_connection_ignores_input_port() {
        use crate::policy::input::{Entity, Instance, Port};
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "child".to_string(),
            file: "child.vhd".to_string(),
            line: 1,
            ports: vec![Port {
                name: "data_i".to_string(),
                direction: "in".to_string(),
                ..Default::default()
            }],
            ..Default::default()
        });
        let mut inst = Instance::default();
        inst.name = "u_child".to_string();
        inst.target = "work.child".to_string();
        inst.file = "top.vhd".to_string();
        inst.line = 10;
        inst.port_map
            .insert("data_i".to_string(), "'1'".to_string());
        input.instances.push(inst);
        let v = constant_port_connection(&input);
        assert!(v.is_empty());
    }

    #[test]
    fn entity_port_not_connected_flags_unused_output() {
        use crate::policy::input::{Entity, Instance, Port};
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "child".to_string(),
            file: "child.vhd".to_string(),
            line: 1,
            ports: vec![
                Port {
                    name: "data_o".to_string(),
                    direction: "out".to_string(),
                    line: 3,
                    ..Default::default()
                },
                Port {
                    name: "valid_o".to_string(),
                    direction: "out".to_string(),
                    line: 4,
                    ..Default::default()
                },
            ],
            ..Default::default()
        });
        let mut inst = Instance::default();
        inst.name = "u_child".to_string();
        inst.target = "work.child".to_string();
        inst.file = "top.vhd".to_string();
        inst.line = 10;
        inst.port_map
            .insert("data_o".to_string(), "sig_data".to_string());
        input.instances.push(inst);
        let v = entity_port_not_connected(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "entity_port_not_connected");
        assert!(v[0].message.contains("valid_o"));
    }

    #[test]
    fn entity_port_not_connected_skips_entity_with_no_instances() {
        use crate::policy::input::{Entity, Port};
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "orphan".to_string(),
            file: "orphan.vhd".to_string(),
            line: 1,
            ports: vec![Port {
                name: "data_o".to_string(),
                direction: "out".to_string(),
                line: 3,
                ..Default::default()
            }],
            ..Default::default()
        });
        let v = entity_port_not_connected(&input);
        assert!(v.is_empty());
    }

    #[test]
    fn entity_port_not_connected_skips_input_ports() {
        use crate::policy::input::{Entity, Instance, Port};
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "child".to_string(),
            file: "child.vhd".to_string(),
            line: 1,
            ports: vec![Port {
                name: "data_i".to_string(),
                direction: "in".to_string(),
                line: 3,
                ..Default::default()
            }],
            ..Default::default()
        });
        let mut inst = Instance::default();
        inst.name = "u_child".to_string();
        inst.target = "work.child".to_string();
        inst.file = "top.vhd".to_string();
        inst.line = 10;
        input.instances.push(inst);
        let v = entity_port_not_connected(&input);
        assert!(v.is_empty());
    }
}
