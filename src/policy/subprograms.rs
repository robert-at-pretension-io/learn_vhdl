use crate::policy::helpers;
use crate::policy::input::Input;
use crate::policy::result::Violation;
use std::collections::{HashMap, HashSet};

pub fn violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(function_param_invalid_mode(input));
    out.extend(procedure_param_invalid_mode(input));
    out.extend(unresolved_qualified_function_call(input));
    out.extend(unresolved_qualified_procedure_call(input));
    out
}

pub fn optional_violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(function_body_missing(input));
    out.extend(procedure_body_missing(input));
    out
}

fn function_body_missing(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for func in &input.functions {
        if func.has_body {
            continue;
        }
        if !func.in_package.is_empty() {
            continue; // Package specs normally have declarations without bodies
        }
        // Check if another declaration of same function has a body in the same file
        let has_body_elsewhere = input.functions.iter().any(|other| {
            other.name.eq_ignore_ascii_case(&func.name) && other.has_body && other.file == func.file
        });
        if has_body_elsewhere {
            continue;
        }
        out.push(Violation {
            rule: "function_body_missing".to_string(),
            severity: "warning".to_string(),
            file: func.file.clone(),
            line: func.line,
            message: format!("Function '{}' declared without body", func.name),
        });
    }
    out
}

fn procedure_body_missing(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for proc_decl in &input.procedures {
        if proc_decl.has_body {
            continue;
        }
        if !proc_decl.in_package.is_empty() {
            continue;
        }
        let has_body_elsewhere = input.procedures.iter().any(|other| {
            other.name.eq_ignore_ascii_case(&proc_decl.name)
                && other.has_body
                && other.file == proc_decl.file
        });
        if has_body_elsewhere {
            continue;
        }
        out.push(Violation {
            rule: "procedure_body_missing".to_string(),
            severity: "warning".to_string(),
            file: proc_decl.file.clone(),
            line: proc_decl.line,
            message: format!("Procedure '{}' declared without body", proc_decl.name),
        });
    }
    out
}

fn function_param_invalid_mode(input: &Input) -> Vec<Violation> {
    let mut violations = Vec::new();
    for func in &input.functions {
        if !func.is_pure {
            continue;
        }
        for param in &func.parameters {
            if param.direction.is_empty() {
                continue;
            }
            if param.direction.eq_ignore_ascii_case("in") {
                continue;
            }
            violations.push(Violation {
                rule: "function_param_invalid_mode".to_string(),
                severity: "error".to_string(),
                file: func.file.clone(),
                line: param.line,
                message: format!(
                    "Function '{}' parameter '{}' has invalid mode '{}' (only 'in' allowed)",
                    func.name, param.name, param.direction
                ),
            });
        }
    }
    violations
}

fn procedure_param_invalid_mode(input: &Input) -> Vec<Violation> {
    let invalid_modes = ["buffer", "linkage"];
    let mut violations = Vec::new();
    for proc_decl in &input.procedures {
        for param in &proc_decl.parameters {
            if invalid_modes
                .iter()
                .any(|m| param.direction.eq_ignore_ascii_case(m))
            {
                violations.push(Violation {
                    rule: "procedure_param_invalid_mode".to_string(),
                    severity: "error".to_string(),
                    file: proc_decl.file.clone(),
                    line: param.line,
                    message: format!(
                        "Procedure '{}' parameter '{}' has invalid mode '{}'",
                        proc_decl.name, param.name, param.direction
                    ),
                });
            }
        }
    }
    violations
}

fn unresolved_qualified_function_call(input: &Input) -> Vec<Violation> {
    unresolved_qualified_call_entries(input, "function_call", "function")
        .into_iter()
        .map(|entry| Violation {
            rule: "unresolved_qualified_function_call".to_string(),
            severity: "error".to_string(),
            file: entry.file,
            line: entry.line,
            message: format!(
                "Function call '{}' has no matching function in package '{}'",
                entry.name, entry.package
            ),
        })
        .collect()
}

fn unresolved_qualified_procedure_call(input: &Input) -> Vec<Violation> {
    unresolved_qualified_call_entries(input, "procedure_call", "procedure")
        .into_iter()
        .map(|entry| Violation {
            rule: "unresolved_qualified_procedure_call".to_string(),
            severity: "error".to_string(),
            file: entry.file,
            line: entry.line,
            message: format!(
                "Procedure call '{}' has no matching procedure in package '{}'",
                entry.name, entry.package
            ),
        })
        .collect()
}

fn unresolved_qualified_call_entries(
    input: &Input,
    use_kind: &str,
    def_kind: &str,
) -> Vec<UnresolvedCall> {
    let mut package_scopes: HashMap<String, Vec<String>> = HashMap::new();
    let mut defs: HashSet<(String, String)> = HashSet::new();
    let mut pkg_by_file_line: HashMap<(String, usize), String> = HashMap::new();

    for def in &input.symbol_defs {
        if def.kind.eq_ignore_ascii_case("package") {
            package_scopes
                .entry(def.name.to_ascii_lowercase())
                .or_default()
                .push(def.scope.to_ascii_lowercase());
        } else if def.kind.eq_ignore_ascii_case(def_kind) {
            defs.insert((
                def.scope.to_ascii_lowercase(),
                def.name.to_ascii_lowercase(),
            ));
        }
    }

    for pkg in &input.packages {
        if pkg.name.trim().is_empty() {
            continue;
        }
        pkg_by_file_line.insert(
            (pkg.file.clone(), pkg.line),
            pkg.name.to_ascii_lowercase(),
        );
    }

    if !input.dependencies.is_empty() {
        for dep in &input.dependencies {
            if !dep.kind.eq_ignore_ascii_case("package_instantiation") {
                continue;
            }
            let inst_pkg = match pkg_by_file_line.get(&(dep.source.clone(), dep.line)) {
                Some(name) => name.clone(),
                None => continue,
            };
            let target_pkg = match parse_target_pkg_name(&dep.target) {
                Some(name) => name,
                None => continue,
            };
            let target_scopes = match package_scopes.get(&target_pkg) {
                Some(scopes) => scopes.clone(),
                None => continue,
            };
            if target_scopes.is_empty() {
                continue;
            }
            package_scopes
                .entry(inst_pkg)
                .or_default()
                .extend(target_scopes);
        }
    }

    if package_scopes.is_empty() {
        return Vec::new();
    }

    let mut missing = Vec::new();
    for use_ in &input.name_uses {
        if use_.kind != use_kind {
            continue;
        }
        if helpers::is_third_party_file(input, &use_.file) {
            continue;
        }
        let (pkg, name) = match parse_qualified_name(&use_.name) {
            Some(parts) => parts,
            None => continue,
        };
        if input
            .shared_variables
            .iter()
            .any(|shared| shared.eq_ignore_ascii_case(&pkg))
        {
            continue;
        }
        let pkg_scopes = match package_scopes.get(&pkg) {
            Some(scopes) => scopes,
            None => continue, // Avoid false positives for external/standard packages
        };
        let name_key = name.to_ascii_lowercase();
        let mut found = false;
        for scope in pkg_scopes {
            if defs.contains(&(scope.to_ascii_lowercase(), name_key.clone())) {
                found = true;
                break;
            }
        }
        if !found {
            missing.push(UnresolvedCall {
                name: use_.name.clone(),
                package: pkg,
                file: use_.file.clone(),
                line: use_.line,
            });
        }
    }

    missing
}

fn parse_qualified_name(name: &str) -> Option<(String, String)> {
    let parts: Vec<&str> = name
        .split('.')
        .map(str::trim)
        .filter(|p| !p.is_empty())
        .collect();
    if parts.len() < 2 {
        return None;
    }
    let func = parts[parts.len() - 1];
    let pkg = parts[parts.len() - 2];
    if func.is_empty() || pkg.is_empty() {
        return None;
    }
    Some((pkg.to_ascii_lowercase(), func.to_ascii_lowercase()))
}

fn parse_target_pkg_name(target: &str) -> Option<String> {
    let parts: Vec<&str> = target
        .split('.')
        .map(str::trim)
        .filter(|p| !p.is_empty())
        .collect();
    if parts.is_empty() {
        return None;
    }
    Some(parts[parts.len() - 1].to_ascii_lowercase())
}

struct UnresolvedCall {
    name: String,
    package: String,
    file: String,
    line: usize,
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::policy::input::{
        FunctionDeclaration, Input, NameUse, ProcedureDeclaration, SubprogramParameter, SymbolDef,
    };

    fn param(name: &str, direction: &str) -> SubprogramParameter {
        SubprogramParameter {
            name: name.to_string(),
            direction: direction.to_string(),
            line: 10,
            ..Default::default()
        }
    }

    #[test]
    fn function_param_invalid_mode_flags_non_in() {
        let mut input = Input::default();
        input.functions.push(FunctionDeclaration {
            name: "f".to_string(),
            file: "a.vhd".to_string(),
            is_pure: true,
            parameters: vec![param("p", "out")],
            ..Default::default()
        });
        let violations = function_param_invalid_mode(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "function_param_invalid_mode");
    }

    #[test]
    fn function_param_invalid_mode_allows_empty_or_in() {
        let mut input = Input::default();
        input.functions.push(FunctionDeclaration {
            name: "f".to_string(),
            file: "a.vhd".to_string(),
            is_pure: true,
            parameters: vec![param("p1", ""), param("p2", "in")],
            ..Default::default()
        });
        let violations = function_param_invalid_mode(&input);
        assert!(violations.is_empty());
    }

    #[test]
    fn procedure_param_invalid_mode_flags_buffer_linkage() {
        let mut input = Input::default();
        input.procedures.push(ProcedureDeclaration {
            name: "p".to_string(),
            file: "a.vhd".to_string(),
            parameters: vec![param("p1", "buffer"), param("p2", "linkage")],
            ..Default::default()
        });
        let violations = procedure_param_invalid_mode(&input);
        assert_eq!(violations.len(), 2);
        assert_eq!(violations[0].rule, "procedure_param_invalid_mode");
    }

    #[test]
    fn procedure_param_invalid_mode_allows_inout() {
        let mut input = Input::default();
        input.procedures.push(ProcedureDeclaration {
            name: "p".to_string(),
            file: "a.vhd".to_string(),
            parameters: vec![param("p1", "in"), param("p2", "out"), param("p3", "inout")],
            ..Default::default()
        });
        let violations = procedure_param_invalid_mode(&input);
        assert!(violations.is_empty());
    }

    #[test]
    fn unresolved_qualified_function_call_flags_missing() {
        let mut input = Input::default();
        input.symbol_defs.push(SymbolDef {
            name: "pkg".to_string(),
            kind: "package".to_string(),
            file: "a.vhd".to_string(),
            scope: "file:a.vhd::package:pkg".to_string(),
            line: 1,
        });
        input.name_uses.push(NameUse {
            name: "pkg.missing_fn".to_string(),
            kind: "function_call".to_string(),
            file: "a.vhd".to_string(),
            line: 10,
            scope: "file:a.vhd::arch:rtl".to_string(),
            context: "p1".to_string(),
        });
        let violations = unresolved_qualified_function_call(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "unresolved_qualified_function_call");
    }

    #[test]
    fn unresolved_qualified_function_call_allows_defined() {
        let mut input = Input::default();
        input.symbol_defs.push(SymbolDef {
            name: "pkg".to_string(),
            kind: "package".to_string(),
            file: "a.vhd".to_string(),
            scope: "file:a.vhd::package:pkg".to_string(),
            line: 1,
        });
        input.symbol_defs.push(SymbolDef {
            name: "good_fn".to_string(),
            kind: "function".to_string(),
            file: "a.vhd".to_string(),
            scope: "file:a.vhd::package:pkg".to_string(),
            line: 2,
        });
        input.name_uses.push(NameUse {
            name: "pkg.good_fn".to_string(),
            kind: "function_call".to_string(),
            file: "a.vhd".to_string(),
            line: 10,
            scope: "file:a.vhd::arch:rtl".to_string(),
            context: "p1".to_string(),
        });
        let violations = unresolved_qualified_function_call(&input);
        assert!(violations.is_empty());
    }

    #[test]
    fn unresolved_qualified_procedure_call_flags_missing() {
        let mut input = Input::default();
        input.symbol_defs.push(SymbolDef {
            name: "pkg".to_string(),
            kind: "package".to_string(),
            file: "a.vhd".to_string(),
            scope: "file:a.vhd::package:pkg".to_string(),
            line: 1,
        });
        input.name_uses.push(NameUse {
            name: "pkg.missing_proc".to_string(),
            kind: "procedure_call".to_string(),
            file: "a.vhd".to_string(),
            line: 10,
            scope: "file:a.vhd::arch:rtl".to_string(),
            context: "p1".to_string(),
        });
        let violations = unresolved_qualified_procedure_call(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "unresolved_qualified_procedure_call");
    }

    #[test]
    fn function_body_missing_flags() {
        let mut input = Input::default();
        input.functions.push(FunctionDeclaration {
            name: "my_func".to_string(),
            file: "a.vhd".to_string(),
            has_body: false,
            in_package: "".to_string(),
            ..Default::default()
        });
        let violations = function_body_missing(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "function_body_missing");
    }

    #[test]
    fn function_body_missing_skips_package_spec() {
        let mut input = Input::default();
        input.functions.push(FunctionDeclaration {
            name: "my_func".to_string(),
            file: "a.vhd".to_string(),
            has_body: false,
            in_package: "my_pkg".to_string(),
            ..Default::default()
        });
        let violations = function_body_missing(&input);
        assert!(violations.is_empty());
    }

    #[test]
    fn function_body_missing_skips_when_body_exists() {
        let mut input = Input::default();
        input.functions.push(FunctionDeclaration {
            name: "my_func".to_string(),
            file: "a.vhd".to_string(),
            has_body: false,
            in_package: "".to_string(),
            line: 1,
            ..Default::default()
        });
        input.functions.push(FunctionDeclaration {
            name: "my_func".to_string(),
            file: "a.vhd".to_string(),
            has_body: true,
            in_package: "".to_string(),
            line: 5,
            ..Default::default()
        });
        let violations = function_body_missing(&input);
        assert!(violations.is_empty());
    }

    #[test]
    fn procedure_body_missing_flags() {
        let mut input = Input::default();
        input.procedures.push(ProcedureDeclaration {
            name: "my_proc".to_string(),
            file: "a.vhd".to_string(),
            has_body: false,
            in_package: "".to_string(),
            ..Default::default()
        });
        let violations = procedure_body_missing(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "procedure_body_missing");
    }

    #[test]
    fn procedure_body_missing_skips_package_spec() {
        let mut input = Input::default();
        input.procedures.push(ProcedureDeclaration {
            name: "my_proc".to_string(),
            file: "a.vhd".to_string(),
            has_body: false,
            in_package: "my_pkg".to_string(),
            ..Default::default()
        });
        let violations = procedure_body_missing(&input);
        assert!(violations.is_empty());
    }

    #[test]
    fn procedure_body_missing_skips_when_body_exists() {
        let mut input = Input::default();
        input.procedures.push(ProcedureDeclaration {
            name: "my_proc".to_string(),
            file: "a.vhd".to_string(),
            has_body: false,
            in_package: "".to_string(),
            line: 1,
            ..Default::default()
        });
        input.procedures.push(ProcedureDeclaration {
            name: "my_proc".to_string(),
            file: "a.vhd".to_string(),
            has_body: true,
            in_package: "".to_string(),
            line: 5,
            ..Default::default()
        });
        let violations = procedure_body_missing(&input);
        assert!(violations.is_empty());
    }

    #[test]
    fn unresolved_qualified_call_skips_external_packages() {
        let mut input = Input::default();
        input.name_uses.push(NameUse {
            name: "ieee.std_logic_1164.to_stdlogicvector".to_string(),
            kind: "function_call".to_string(),
            file: "a.vhd".to_string(),
            line: 10,
            scope: "file:a.vhd::arch:rtl".to_string(),
            context: "p1".to_string(),
        });
        let violations = unresolved_qualified_function_call(&input);
        assert!(violations.is_empty());
    }
}
