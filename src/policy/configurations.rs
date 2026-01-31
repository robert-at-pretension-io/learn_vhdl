use crate::policy::input::Input;
use crate::policy::result::Violation;

pub fn violations(input: &Input) -> Vec<Violation> {
    configuration_missing_entity(input)
}

fn configuration_missing_entity(input: &Input) -> Vec<Violation> {
    input
        .configurations
        .iter()
        .filter(|cfg| !entity_exists(input, &cfg.entity_name))
        .map(|cfg| Violation {
            rule: "configuration_missing_entity".to_string(),
            severity: "error".to_string(),
            file: cfg.file.clone(),
            line: cfg.line,
            message: format!(
                "Configuration '{}' references missing entity '{}'",
                cfg.name, cfg.entity_name
            ),
        })
        .collect()
}

fn entity_exists(input: &Input, name: &str) -> bool {
    input
        .entities
        .iter()
        .any(|entity| entity.name.eq_ignore_ascii_case(name))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::policy::input::{Configuration, Entity, Input};

    #[test]
    fn configuration_missing_entity_flags() {
        let mut input = Input::default();
        input.configurations.push(Configuration {
            name: "cfg".to_string(),
            entity_name: "missing".to_string(),
            file: "a.vhd".to_string(),
            line: 10,
        });
        let violations = configuration_missing_entity(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "configuration_missing_entity");
    }

    #[test]
    fn configuration_missing_entity_passes_when_entity_exists() {
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "core".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        input.configurations.push(Configuration {
            name: "cfg".to_string(),
            entity_name: "core".to_string(),
            file: "a.vhd".to_string(),
            line: 10,
        });
        let violations = configuration_missing_entity(&input);
        assert!(violations.is_empty());
    }
}
