use crate::policy::helpers::{self, is_testbench_name};
use crate::policy::input::{Component, Input};
use crate::policy::result::Violation;
use std::collections::HashMap;

pub fn violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(missing_ports(input));
    out.extend(orphan_architecture(input));
    out.extend(unresolved_component(input));
    out.extend(unresolved_dependency(input));
    out.extend(potential_latch(input));
    out.extend(entity_without_arch(input));
    out.extend(duplicate_entity_in_library(input));
    out.extend(duplicate_package_in_library(input));
    out
}

fn missing_ports(input: &Input) -> Vec<Violation> {
    input
        .entities
        .iter()
        .filter(|entity| entity.ports.is_empty() && !is_testbench_name(&entity.name))
        .map(|entity| Violation {
            rule: "entity_has_ports".to_string(),
            severity: "warning".to_string(),
            file: entity.file.clone(),
            line: entity.line,
            message: format!("Entity '{}' has no ports defined", entity.name),
        })
        .collect()
}

fn orphan_architecture(input: &Input) -> Vec<Violation> {
    input
        .architectures
        .iter()
        .filter(|arch| !entity_exists(input, &arch.entity_name))
        .map(|arch| Violation {
            rule: "architecture_has_entity".to_string(),
            severity: "error".to_string(),
            file: arch.file.clone(),
            line: arch.line,
            message: format!(
                "Architecture '{}' references undefined entity '{}'",
                arch.name, arch.entity_name
            ),
        })
        .collect()
}

fn unresolved_component(input: &Input) -> Vec<Violation> {
    input
        .components
        .iter()
        .filter(|comp| comp.is_instance && !comp.entity_ref.is_empty())
        .filter(|comp| !component_or_entity_exists(input, comp))
        .map(|comp| Violation {
            rule: "component_resolved".to_string(),
            severity: "warning".to_string(),
            file: comp.file.clone(),
            line: comp.line,
            message: format!(
                "Component instance '{}' references undefined '{}'",
                comp.name, comp.entity_ref
            ),
        })
        .collect()
}

fn unresolved_dependency(input: &Input) -> Vec<Violation> {
    input
        .dependencies
        .iter()
        .filter(|dep| !dep.resolved && dep.kind == "instantiation")
        .map(|dep| Violation {
            rule: "unresolved_dependency".to_string(),
            severity: "error".to_string(),
            file: dep.source.clone(),
            line: dep.line,
            message: format!("Unresolved dependency: '{}'", dep.target),
        })
        .collect()
}

fn potential_latch(input: &Input) -> Vec<Violation> {
    input
        .case_statements
        .iter()
        .filter(|cs| !cs.has_others)
        .filter(|cs| case_in_combinational_process(input, cs))
        .filter(|cs| !helpers::file_in_testbench(input, &cs.file))
        .map(|cs| Violation {
            rule: "potential_latch".to_string(),
            severity: "warning".to_string(),
            file: cs.file.clone(),
            line: cs.line,
            message: format!(
                "Case statement on '{}' missing 'when others =>' (potential latch in process '{}')",
                cs.expression, cs.in_process
            ),
        })
        .collect()
}

fn case_in_combinational_process(input: &Input, cs: &crate::policy::input::CaseStatement) -> bool {
    if cs.in_process.is_empty() {
        return false;
    }
    input.processes.iter().any(|proc| {
        proc.label == cs.in_process && proc.in_arch == cs.in_arch && proc.is_combinational
    })
}

fn entity_without_arch(input: &Input) -> Vec<Violation> {
    input
        .entities
        .iter()
        .filter(|entity| !has_architecture(input, &entity.name))
        .map(|entity| Violation {
            rule: "entity_without_arch".to_string(),
            severity: "warning".to_string(),
            file: entity.file.clone(),
            line: entity.line,
            message: format!("Entity '{}' has no architecture defined", entity.name),
        })
        .collect()
}

fn duplicate_entity_in_library(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    let lib_map = file_library_map(input);
    let mut seen: HashMap<(String, String), (String, usize)> = HashMap::new();

    for entity in &input.entities {
        if helpers::is_third_party_file(input, &entity.file) {
            continue;
        }
        let lib = library_for_file(&lib_map, &entity.file);
        let key = (lib.clone(), entity.name.to_ascii_lowercase());
        if let Some((first_file, first_line)) = seen.get(&key) {
            if &entity.file == first_file {
                continue;
            }
            out.push(Violation {
                rule: "duplicate_entity_in_library".to_string(),
                severity: "error".to_string(),
                file: entity.file.clone(),
                line: entity.line,
                message: format!(
                    "Entity '{}' is defined multiple times in library '{}' (first seen at {}:{})",
                    entity.name, lib, first_file, first_line
                ),
            });
        } else {
            seen.insert(key, (entity.file.clone(), entity.line));
        }
    }
    out
}

fn duplicate_package_in_library(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    let lib_map = file_library_map(input);
    let mut seen: HashMap<(String, String), (String, usize)> = HashMap::new();

    for pkg in &input.packages {
        if helpers::is_third_party_file(input, &pkg.file) {
            continue;
        }
        let lib = library_for_file(&lib_map, &pkg.file);
        let key = (lib.clone(), pkg.name.to_ascii_lowercase());
        if let Some((first_file, first_line)) = seen.get(&key) {
            if &pkg.file == first_file {
                continue;
            }
            out.push(Violation {
                rule: "duplicate_package_in_library".to_string(),
                severity: "error".to_string(),
                file: pkg.file.clone(),
                line: pkg.line,
                message: format!(
                    "Package '{}' is defined multiple times in library '{}' (first seen at {}:{})",
                    pkg.name, lib, first_file, first_line
                ),
            });
        } else {
            seen.insert(key, (pkg.file.clone(), pkg.line));
        }
    }
    out
}

fn file_library_map(input: &Input) -> HashMap<String, String> {
    let mut map = HashMap::new();
    for file in &input.files {
        let lib = if file.library.is_empty() {
            "work".to_string()
        } else {
            file.library.to_ascii_lowercase()
        };
        map.insert(file.path.clone(), lib);
    }
    map
}

fn library_for_file(map: &HashMap<String, String>, file: &str) -> String {
    map.get(file).cloned().unwrap_or_else(|| "work".to_string())
}

fn entity_exists(input: &Input, name: &str) -> bool {
    input
        .entities
        .iter()
        .any(|entity| entity.name.eq_ignore_ascii_case(name))
}

fn component_or_entity_exists(input: &Input, comp: &Component) -> bool {
    let target = base_entity_name(&comp.entity_ref);
    input
        .entities
        .iter()
        .any(|entity| entity.name.eq_ignore_ascii_case(&target))
        || input
            .components
            .iter()
            .filter(|c| !c.is_instance)
            .any(|c| c.name.eq_ignore_ascii_case(&target))
}

fn base_entity_name(name: &str) -> String {
    if let Some(last) = name.split('.').last() {
        last.to_ascii_lowercase()
    } else {
        name.to_ascii_lowercase()
    }
}

fn has_architecture(input: &Input, entity_name: &str) -> bool {
    input
        .architectures
        .iter()
        .any(|arch| arch.entity_name.eq_ignore_ascii_case(entity_name))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::policy::input::{
        Architecture, CaseStatement, Component, Dependency, Entity, FileInfo, Input, Package, Port,
        Process,
    };

    fn base_input() -> Input {
        Input::default()
    }

    #[test]
    fn missing_ports_flags_non_testbench() {
        let mut input = base_input();
        input.entities.push(Entity {
            name: "core".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ports: vec![],
            generics: vec![],
        });
        let violations = missing_ports(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "entity_has_ports");
    }

    #[test]
    fn missing_ports_skips_testbench() {
        let mut input = base_input();
        input.entities.push(Entity {
            name: "core_tb".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ports: vec![],
            generics: vec![],
        });
        let violations = missing_ports(&input);
        assert!(violations.is_empty());
    }

    #[test]
    fn orphan_architecture_flags_missing_entity() {
        let mut input = base_input();
        input.architectures.push(Architecture {
            name: "rtl".to_string(),
            entity_name: "missing".to_string(),
            file: "a.vhd".to_string(),
            line: 2,
        });
        let violations = orphan_architecture(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "architecture_has_entity");
    }

    #[test]
    fn unresolved_component_flags_missing_target() {
        let mut input = base_input();
        input.components.push(Component {
            name: "u1".to_string(),
            entity_ref: "work.missing".to_string(),
            file: "a.vhd".to_string(),
            line: 3,
            is_instance: true,
            ports: vec![],
            generics: vec![],
        });
        let violations = unresolved_component(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "component_resolved");
    }

    #[test]
    fn unresolved_dependency_flags_instantiation() {
        let mut input = base_input();
        input.dependencies.push(Dependency {
            source: "a.vhd".to_string(),
            target: "work.missing".to_string(),
            kind: "instantiation".to_string(),
            line: 4,
            resolved: false,
        });
        let violations = unresolved_dependency(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "unresolved_dependency");
    }

    #[test]
    fn potential_latch_flags_missing_others() {
        let mut input = base_input();
        input.case_statements.push(CaseStatement {
            expression: "state".to_string(),
            has_others: false,
            file: "a.vhd".to_string(),
            line: 5,
            in_process: "p1".to_string(),
            ..Default::default()
        });
        input.processes.push(Process {
            label: "p1".to_string(),
            in_arch: "".to_string(),
            is_combinational: true,
            file: "a.vhd".to_string(),
            line: 5,
            ..Default::default()
        });
        let violations = potential_latch(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "potential_latch");
    }

    #[test]
    fn entity_without_arch_flags_missing_arch() {
        let mut input = base_input();
        input.entities.push(Entity {
            name: "core".to_string(),
            file: "a.vhd".to_string(),
            line: 6,
            ports: vec![Port::default()],
            generics: vec![],
        });
        let violations = entity_without_arch(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "entity_without_arch");
    }

    #[test]
    fn duplicate_entity_in_library_flags() {
        let mut input = base_input();
        input.files = vec![
            FileInfo {
                path: "a.vhd".to_string(),
                library: "work".to_string(),
                ..Default::default()
            },
            FileInfo {
                path: "b.vhd".to_string(),
                library: "work".to_string(),
                ..Default::default()
            },
        ];
        input.entities.push(Entity {
            name: "dup_ent".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ports: vec![],
            generics: vec![],
        });
        input.entities.push(Entity {
            name: "dup_ent".to_string(),
            file: "b.vhd".to_string(),
            line: 2,
            ports: vec![],
            generics: vec![],
        });
        let violations = duplicate_entity_in_library(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "duplicate_entity_in_library");
    }

    #[test]
    fn duplicate_entity_in_library_skips_other_library() {
        let mut input = base_input();
        input.files = vec![
            FileInfo {
                path: "a.vhd".to_string(),
                library: "lib_a".to_string(),
                ..Default::default()
            },
            FileInfo {
                path: "b.vhd".to_string(),
                library: "lib_b".to_string(),
                ..Default::default()
            },
        ];
        input.entities.push(Entity {
            name: "dup_ent".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ports: vec![],
            generics: vec![],
        });
        input.entities.push(Entity {
            name: "dup_ent".to_string(),
            file: "b.vhd".to_string(),
            line: 2,
            ports: vec![],
            generics: vec![],
        });
        let violations = duplicate_entity_in_library(&input);
        assert!(violations.is_empty());
    }

    #[test]
    fn duplicate_package_in_library_flags() {
        let mut input = base_input();
        input.files = vec![
            FileInfo {
                path: "a.vhd".to_string(),
                library: "work".to_string(),
                ..Default::default()
            },
            FileInfo {
                path: "b.vhd".to_string(),
                library: "work".to_string(),
                ..Default::default()
            },
        ];
        input.packages.push(Package {
            name: "dup_pkg".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
        });
        input.packages.push(Package {
            name: "dup_pkg".to_string(),
            file: "b.vhd".to_string(),
            line: 2,
        });
        let violations = duplicate_package_in_library(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "duplicate_package_in_library");
    }
}
