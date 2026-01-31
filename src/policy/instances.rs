use crate::policy::helpers::valid_instance_prefix;
use crate::policy::input::Input;
use crate::policy::result::Violation;

pub fn violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(positional_mapping(input));
    out.extend(instance_naming_convention(input));
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
}
