use crate::policy::helpers::{self, is_testbench_name};
use crate::policy::input::{Component, Input};
use crate::policy::result::Violation;
use std::collections::{HashMap, HashSet};
use std::path::Path;

pub fn violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(missing_ports(input));
    out.extend(orphan_architecture(input));
    out.extend(unresolved_component(input));
    out.extend(unresolved_dependency(input));
    out.extend(unresolved_import(input));
    out.extend(potential_latch(input));
    out.extend(entity_without_arch(input));
    out.extend(duplicate_entity_in_library(input));
    out.extend(duplicate_package_in_library(input));
    out
}

pub fn optional_violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(duplicate_architecture_name(input));
    out
}

fn duplicate_architecture_name(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    let mut seen: HashMap<(String, String), (String, usize)> = HashMap::new();

    for arch in &input.architectures {
        let key = (
            arch.entity_name.to_ascii_lowercase(),
            arch.name.to_ascii_lowercase(),
        );
        if let Some((first_file, first_line)) = seen.get(&key) {
            out.push(Violation {
                rule: "duplicate_architecture_name".to_string(),
                severity: "warning".to_string(),
                file: arch.file.clone(),
                line: arch.line,
                message: format!(
                    "Duplicate architecture '{}' for entity '{}' (also in '{}:{}')",
                    arch.name, arch.entity_name, first_file, first_line
                ),
            });
        } else {
            seen.insert(key, (arch.file.clone(), arch.line));
        }
    }
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
    let lib_map = file_library_map(input);
    let known_libs = known_libraries(input);
    let entity_index = entity_index(input, &lib_map);
    let mut out = Vec::new();

    for dep in input
        .dependencies
        .iter()
        .filter(|dep| !dep.resolved && dep.kind == "instantiation")
    {
        let file_lib = library_for_file(&lib_map, &dep.source);
        let severity = severity_for_file(input, &dep.source, &file_lib, "error");
        let (lib, name) = parse_target(&dep.target, &file_lib, &known_libs);
        let mut msg = format!(
            "Unresolved dependency: '{}' via instantiation (file library: {}; work => {})",
            dep.target, file_lib, file_lib
        );
        let mut candidates = Vec::new();
        if let Some(list) = entity_index.get(&name.to_ascii_lowercase()) {
            for (cand_lib, cand_file, cand_line) in list {
                candidates.push(format!(
                    "{}.{} ({})",
                    cand_lib,
                    name,
                    format_location(cand_file, *cand_line)
                ));
            }
        }
        if !candidates.is_empty() {
            msg.push_str(". Candidates: ");
            msg.push_str(&format_list(&candidates, 4));
        } else if !lib.is_empty() && !is_known_library(&lib, &known_libs) {
            msg.push_str(&format!(
                ". Library '{}' is not mapped. Action: add it to vhdl_lint.json libraries/files",
                lib
            ));
        } else {
            msg.push_str(". Action: check entity name or add missing source file to config");
        }

        out.push(Violation {
            rule: "unresolved_dependency".to_string(),
            severity,
            file: dep.source.clone(),
            line: dep.line,
            message: msg,
        });
    }

    out
}

fn unresolved_import(input: &Input) -> Vec<Violation> {
    let lib_map = file_library_map(input);
    let known_libs = known_libraries(input);
    let pkg_index = package_index(input, &lib_map);
    let ctx_index = context_index(input, &lib_map);
    let mut out = Vec::new();

    for dep in input
        .dependencies
        .iter()
        .filter(|dep| !dep.resolved)
        .filter(|dep| matches!(dep.kind.as_str(), "use" | "library" | "context"))
    {
        let file_lib = library_for_file(&lib_map, &dep.source);
        let severity = severity_for_file(input, &dep.source, &file_lib, "error");
        match dep.kind.as_str() {
            "library" => {
                let lib = dep.target.trim().to_ascii_lowercase();
                let mut msg = format!(
                    "Unresolved library '{}' (file library: {}; work => {})",
                    dep.target.trim(),
                    file_lib,
                    file_lib
                );
                if !is_known_library(&lib, &known_libs) && !lib.is_empty() {
                    msg.push_str(&format!(
                        ". Action: add library '{}' to vhdl_lint.json libraries/files",
                        lib
                    ));
                }
                let known = known_library_list(&known_libs);
                if !known.is_empty() {
                    msg.push_str(&format!(". Known libraries: {}", format_list(&known, 6)));
                }
                out.push(Violation {
                    rule: "unresolved_library".to_string(),
                    severity,
                    file: dep.source.clone(),
                    line: dep.line,
                    message: msg,
                });
            }
            "use" => {
                let (lib, pkg) = parse_use_target(&dep.target, &file_lib, &known_libs);
                if !is_known_library(&lib, &known_libs) {
                    let mut msg = format!(
                        "Unresolved library '{}' referenced in use clause '{}' (file library: {}; work => {})",
                        lib, dep.target, file_lib, file_lib
                    );
                    msg.push_str(&format!(
                        ". Action: add library '{}' to vhdl_lint.json libraries/files",
                        lib
                    ));
                    out.push(Violation {
                        rule: "unresolved_library".to_string(),
                        severity,
                        file: dep.source.clone(),
                        line: dep.line,
                        message: msg,
                    });
                } else {
                    let mut msg = format!(
                        "Unresolved package '{}.{}' in use clause '{}' (file library: {}; work => {})",
                        lib, pkg, dep.target, file_lib, file_lib
                    );
                    if let Some(cands) = pkg_index.get(&pkg.to_ascii_lowercase()) {
                        let formatted = format_candidates(cands, &pkg, 4);
                        if !formatted.is_empty() {
                            msg.push_str(&format!(". Found package '{}' in {}", pkg, formatted));
                        }
                    }
                    msg.push_str(
                        ". Action: add the missing package file or adjust library mapping",
                    );
                    out.push(Violation {
                        rule: "unresolved_package".to_string(),
                        severity,
                        file: dep.source.clone(),
                        line: dep.line,
                        message: msg,
                    });
                }
            }
            "context" => {
                let (lib, ctx) = parse_context_target(&dep.target, &file_lib, &known_libs);
                if !is_known_library(&lib, &known_libs) {
                    let mut msg = format!(
                        "Unresolved library '{}' referenced in context clause '{}' (file library: {}; work => {})",
                        lib, dep.target, file_lib, file_lib
                    );
                    msg.push_str(&format!(
                        ". Action: add library '{}' to vhdl_lint.json libraries/files",
                        lib
                    ));
                    out.push(Violation {
                        rule: "unresolved_library".to_string(),
                        severity,
                        file: dep.source.clone(),
                        line: dep.line,
                        message: msg,
                    });
                } else {
                    let mut msg = format!(
                        "Unresolved context '{}.{}' in context clause '{}' (file library: {}; work => {})",
                        lib, ctx, dep.target, file_lib, file_lib
                    );
                    if let Some(cands) = ctx_index.get(&ctx.to_ascii_lowercase()) {
                        let formatted = format_candidates(cands, &ctx, 4);
                        if !formatted.is_empty() {
                            msg.push_str(&format!(". Found context '{}' in {}", ctx, formatted));
                        }
                    }
                    msg.push_str(
                        ". Action: add the missing context file or adjust library mapping",
                    );
                    out.push(Violation {
                        rule: "unresolved_package".to_string(),
                        severity,
                        file: dep.source.clone(),
                        line: dep.line,
                        message: msg,
                    });
                }
            }
            _ => {}
        }
    }

    out
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
            let mut msg = format!(
                "Entity '{}' is defined multiple times in library '{}' (first seen at {}:{})",
                entity.name, lib, first_file, first_line
            );
            msg.push_str(
                ". Consider splitting variants into separate libraries or exclude files via vhdl_lint.json (libraries/files/ignorePatterns)",
            );
            out.push(Violation {
                rule: "duplicate_entity_in_library".to_string(),
                severity: "error".to_string(),
                file: entity.file.clone(),
                line: entity.line,
                message: msg,
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
        if !pkg.in_arch.is_empty() {
            continue;
        }
        let lib = library_for_file(&lib_map, &pkg.file);
        let key = (lib.clone(), pkg.name.to_ascii_lowercase());
        if let Some((first_file, first_line)) = seen.get(&key) {
            if &pkg.file == first_file {
                continue;
            }
            let mut msg = format!(
                "Package '{}' is defined multiple times in library '{}' (first seen at {}:{})",
                pkg.name, lib, first_file, first_line
            );
            msg.push_str(
                ". Consider splitting variants into separate libraries or exclude files via vhdl_lint.json (libraries/files/ignorePatterns)",
            );
            out.push(Violation {
                rule: "duplicate_package_in_library".to_string(),
                severity: "error".to_string(),
                file: pkg.file.clone(),
                line: pkg.line,
                message: msg,
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

fn known_libraries(input: &Input) -> HashSet<String> {
    let mut libs = HashSet::new();
    for file in &input.files {
        let lib = if file.library.is_empty() {
            "work".to_string()
        } else {
            file.library.to_ascii_lowercase()
        };
        libs.insert(lib);
    }
    libs.insert("work".to_string());
    libs.insert("ieee".to_string());
    libs.insert("std".to_string());
    libs
}

fn known_library_list(libs: &HashSet<String>) -> Vec<String> {
    let mut list: Vec<String> = libs.iter().cloned().collect();
    list.sort();
    list
}

fn is_known_library(name: &str, libs: &HashSet<String>) -> bool {
    if name.is_empty() {
        return false;
    }
    libs.iter().any(|l| l.eq_ignore_ascii_case(name))
}

fn format_list(items: &[String], limit: usize) -> String {
    if items.is_empty() {
        return String::new();
    }
    if items.len() <= limit {
        return items.join(", ");
    }
    format!(
        "{}, ... (+{})",
        items[..limit].join(", "),
        items.len() - limit
    )
}

fn format_candidates(
    candidates: &Vec<(String, String, usize)>,
    name: &str,
    limit: usize,
) -> String {
    let mut items = Vec::new();
    for (lib, file, line) in candidates {
        items.push(format!(
            "{}.{} ({})",
            lib,
            name,
            format_location(file, *line)
        ));
    }
    format_list(&items, limit)
}

fn format_location(file: &str, line: usize) -> String {
    let short = Path::new(file)
        .file_name()
        .and_then(|s| s.to_str())
        .unwrap_or(file);
    if line == 0 {
        short.to_string()
    } else {
        format!("{}:{}", short, line)
    }
}

fn package_index(
    input: &Input,
    lib_map: &HashMap<String, String>,
) -> HashMap<String, Vec<(String, String, usize)>> {
    let mut index: HashMap<String, Vec<(String, String, usize)>> = HashMap::new();
    for pkg in &input.packages {
        let lib = library_for_file(lib_map, &pkg.file);
        index
            .entry(pkg.name.to_ascii_lowercase())
            .or_default()
            .push((lib, pkg.file.clone(), pkg.line));
    }
    index
}

fn context_index(
    input: &Input,
    lib_map: &HashMap<String, String>,
) -> HashMap<String, Vec<(String, String, usize)>> {
    let mut index: HashMap<String, Vec<(String, String, usize)>> = HashMap::new();
    for ctx in &input.context_declarations {
        let lib = library_for_file(lib_map, &ctx.file);
        index
            .entry(ctx.name.to_ascii_lowercase())
            .or_default()
            .push((lib, ctx.file.clone(), ctx.line));
    }
    index
}

fn entity_index(
    input: &Input,
    lib_map: &HashMap<String, String>,
) -> HashMap<String, Vec<(String, String, usize)>> {
    let mut index: HashMap<String, Vec<(String, String, usize)>> = HashMap::new();
    for ent in &input.entities {
        let lib = library_for_file(lib_map, &ent.file);
        index
            .entry(ent.name.to_ascii_lowercase())
            .or_default()
            .push((lib, ent.file.clone(), ent.line));
    }
    index
}

fn parse_use_target(
    target: &str,
    file_lib: &str,
    known_libs: &HashSet<String>,
) -> (String, String) {
    let trimmed = target.trim();
    if trimmed.is_empty() {
        return (file_lib.to_string(), String::new());
    }
    let mut parts: Vec<&str> = trimmed.split('.').collect();
    if let Some(last) = parts.last() {
        if last.eq_ignore_ascii_case("all") {
            parts.pop();
        }
    }
    if parts.len() >= 2 && trimmed.split('.').count() >= 3 {
        let mut lib = parts[0].to_ascii_lowercase();
        if lib == "work" {
            lib = file_lib.to_ascii_lowercase();
        }
        return (lib, parts[1].to_string());
    }
    parse_target_parts(&parts, file_lib, known_libs)
}

fn parse_context_target(
    target: &str,
    file_lib: &str,
    known_libs: &HashSet<String>,
) -> (String, String) {
    let trimmed = target.trim();
    if trimmed.is_empty() {
        return (file_lib.to_string(), String::new());
    }
    let parts: Vec<&str> = trimmed.split('.').collect();
    if parts.len() >= 2 {
        let mut lib = parts[0].to_ascii_lowercase();
        if lib == "work" {
            lib = file_lib.to_ascii_lowercase();
        }
        return (lib, parts[1].to_string());
    }
    parse_target_parts(&parts, file_lib, known_libs)
}

fn parse_target(target: &str, file_lib: &str, known_libs: &HashSet<String>) -> (String, String) {
    let trimmed = target.trim();
    if trimmed.is_empty() {
        return (file_lib.to_string(), String::new());
    }
    let parts: Vec<&str> = trimmed.split('.').collect();
    if parts.len() >= 2 {
        let mut lib = parts[0].to_ascii_lowercase();
        if lib == "work" {
            lib = file_lib.to_ascii_lowercase();
        }
        return (lib, parts[parts.len() - 1].to_string());
    }
    parse_target_parts(&parts, file_lib, known_libs)
}

fn parse_target_parts(
    parts: &[&str],
    file_lib: &str,
    known_libs: &HashSet<String>,
) -> (String, String) {
    if parts.is_empty() {
        return (file_lib.to_string(), String::new());
    }
    if parts.len() >= 2 {
        let first = parts[0].to_ascii_lowercase();
        let second = parts[1].to_string();
        if first == "work" {
            return (file_lib.to_ascii_lowercase(), second);
        }
        if is_known_library(&first, known_libs) {
            return (first, second);
        }
        return (file_lib.to_ascii_lowercase(), parts[0].to_string());
    }
    (file_lib.to_ascii_lowercase(), parts[0].to_string())
}

fn severity_for_file(input: &Input, file: &str, file_lib: &str, default: &str) -> String {
    if helpers::file_in_testbench(input, file) || file_lib.eq_ignore_ascii_case("test") {
        return "warning".to_string();
    }
    default.to_string()
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
    fn unresolved_import_flags_use_clause() {
        let mut input = base_input();
        input.dependencies.push(Dependency {
            source: "a.vhd".to_string(),
            target: "work.missing_pkg.all".to_string(),
            kind: "use".to_string(),
            line: 4,
            resolved: false,
        });
        let violations = unresolved_import(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "unresolved_package");
        assert!(violations[0].message.contains("work.missing_pkg"));
    }

    #[test]
    fn unresolved_import_flags_library_clause() {
        let mut input = base_input();
        input.dependencies.push(Dependency {
            source: "a.vhd".to_string(),
            target: "missinglib".to_string(),
            kind: "library".to_string(),
            line: 2,
            resolved: false,
        });
        let violations = unresolved_import(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "unresolved_library");
        assert!(violations[0].message.contains("missinglib"));
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
            in_arch: String::new(),
        });
        input.packages.push(Package {
            name: "dup_pkg".to_string(),
            file: "b.vhd".to_string(),
            line: 2,
            in_arch: String::new(),
        });
        let violations = duplicate_package_in_library(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "duplicate_package_in_library");
    }

    #[test]
    fn duplicate_architecture_name_flags() {
        let mut input = base_input();
        input.architectures.push(Architecture {
            name: "rtl".to_string(),
            entity_name: "core".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
        });
        input.architectures.push(Architecture {
            name: "rtl".to_string(),
            entity_name: "core".to_string(),
            file: "b.vhd".to_string(),
            line: 2,
        });
        let violations = duplicate_architecture_name(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "duplicate_architecture_name");
        assert_eq!(violations[0].severity, "warning");
    }

    #[test]
    fn duplicate_architecture_name_flags_same_file() {
        let mut input = base_input();
        input.architectures.push(Architecture {
            name: "rtl".to_string(),
            entity_name: "core".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
        });
        input.architectures.push(Architecture {
            name: "rtl".to_string(),
            entity_name: "core".to_string(),
            file: "a.vhd".to_string(),
            line: 10,
        });
        let violations = duplicate_architecture_name(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "duplicate_architecture_name");
    }

    #[test]
    fn duplicate_architecture_name_skips_different_entity() {
        let mut input = base_input();
        input.architectures.push(Architecture {
            name: "rtl".to_string(),
            entity_name: "core_a".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
        });
        input.architectures.push(Architecture {
            name: "rtl".to_string(),
            entity_name: "core_b".to_string(),
            file: "b.vhd".to_string(),
            line: 2,
        });
        let violations = duplicate_architecture_name(&input);
        assert!(violations.is_empty());
    }
}
