use std::collections::HashSet;

use crate::policy::helpers;
use crate::policy::input::Input;
use crate::policy::result::Violation;

pub fn optional_violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(unused_constant(input));
    out.extend(unused_component(input));
    out.extend(unused_type(input));
    out.extend(unused_subprogram(input));
    out.extend(unused_subtype(input));
    out.extend(unused_generic(input));
    out.extend(record_field_unused(input));
    out.extend(dead_generate(input));
    out.extend(generate_range_mismatch(input));
    out
}

fn unused_constant(input: &Input) -> Vec<Violation> {
    let referenced = build_referenced_names(input);
    input
        .constant_decls
        .iter()
        .filter(|c| !c.in_arch.is_empty() && c.in_package.is_empty())
        .filter(|c| !helpers::file_in_testbench(input, &c.file))
        .filter(|c| !referenced.contains(&c.name.to_ascii_lowercase()))
        .map(|c| Violation {
            rule: "unused_constant".to_string(),
            severity: "info".to_string(),
            file: c.file.clone(),
            line: c.line,
            message: format!(
                "Constant '{}' is declared but never referenced",
                c.name
            ),
        })
        .collect()
}

fn unused_component(input: &Input) -> Vec<Violation> {
    let instantiated: HashSet<String> = input
        .instances
        .iter()
        .map(|inst| inst.target.to_ascii_lowercase())
        .collect();
    input
        .components
        .iter()
        .filter(|comp| !comp.is_instance)
        .filter(|comp| !helpers::file_in_testbench(input, &comp.file))
        .filter(|comp| !instantiated.contains(&comp.name.to_ascii_lowercase()))
        .map(|comp| Violation {
            rule: "unused_component".to_string(),
            severity: "warning".to_string(),
            file: comp.file.clone(),
            line: comp.line,
            message: format!(
                "Component '{}' is declared but never instantiated",
                comp.name
            ),
        })
        .collect()
}

fn unused_type(input: &Input) -> Vec<Violation> {
    let referenced_types = build_referenced_type_names(input);
    input
        .types
        .iter()
        .filter(|t| !t.in_arch.is_empty() && t.in_package.is_empty())
        .filter(|t| !helpers::file_in_testbench(input, &t.file))
        .filter(|t| !referenced_types.contains(&t.name.to_ascii_lowercase()))
        .map(|t| Violation {
            rule: "unused_type".to_string(),
            severity: "info".to_string(),
            file: t.file.clone(),
            line: t.line,
            message: format!("Type '{}' is declared but never used", t.name),
        })
        .collect()
}

fn unused_subprogram(input: &Input) -> Vec<Violation> {
    let mut called: HashSet<String> = HashSet::new();
    for proc in &input.processes {
        for fc in &proc.function_calls {
            called.insert(fc.name.to_ascii_lowercase());
        }
        for pc in &proc.procedure_calls {
            called.insert(pc.name.to_ascii_lowercase());
        }
    }
    let mut out = Vec::new();
    for func in &input.functions {
        if func.in_arch.is_empty() || !func.in_package.is_empty() {
            continue;
        }
        if helpers::file_in_testbench(input, &func.file) {
            continue;
        }
        if !called.contains(&func.name.to_ascii_lowercase()) {
            out.push(Violation {
                rule: "unused_subprogram".to_string(),
                severity: "info".to_string(),
                file: func.file.clone(),
                line: func.line,
                message: format!(
                    "Function '{}' is declared but never called",
                    func.name
                ),
            });
        }
    }
    for proc in &input.procedures {
        if proc.in_arch.is_empty() || !proc.in_package.is_empty() {
            continue;
        }
        if helpers::file_in_testbench(input, &proc.file) {
            continue;
        }
        if !called.contains(&proc.name.to_ascii_lowercase()) {
            out.push(Violation {
                rule: "unused_subprogram".to_string(),
                severity: "info".to_string(),
                file: proc.file.clone(),
                line: proc.line,
                message: format!(
                    "Procedure '{}' is declared but never called",
                    proc.name
                ),
            });
        }
    }
    out
}

fn unused_subtype(input: &Input) -> Vec<Violation> {
    let referenced_types = build_referenced_type_names(input);
    input
        .subtypes
        .iter()
        .filter(|st| !st.in_arch.is_empty() && st.in_package.is_empty())
        .filter(|st| !helpers::file_in_testbench(input, &st.file))
        .filter(|st| !referenced_types.contains(&st.name.to_ascii_lowercase()))
        .map(|st| Violation {
            rule: "unused_subtype".to_string(),
            severity: "info".to_string(),
            file: st.file.clone(),
            line: st.line,
            message: format!("Subtype '{}' is declared but never used", st.name),
        })
        .collect()
}

fn unused_generic(input: &Input) -> Vec<Violation> {
    let referenced = build_referenced_names(input);
    let mut out = Vec::new();
    for entity in &input.entities {
        if helpers::file_in_testbench(input, &entity.file) {
            continue;
        }
        for gen in &entity.generics {
            if !referenced.contains(&gen.name.to_ascii_lowercase()) {
                out.push(Violation {
                    rule: "unused_generic".to_string(),
                    severity: "info".to_string(),
                    file: entity.file.clone(),
                    line: gen.line,
                    message: format!(
                        "Generic '{}' on entity '{}' is never referenced in any architecture",
                        gen.name, entity.name
                    ),
                });
            }
        }
    }
    out
}

fn record_field_unused(input: &Input) -> Vec<Violation> {
    let mut referenced_fields: HashSet<String> = HashSet::new();
    let mut whole_signal_names: HashSet<String> = HashSet::new();

    // Collect field names from dotted references and whole-signal names
    let mut process_name = |sig: &str| {
        if let Some(field) = sig.split('.').nth(1) {
            referenced_fields.insert(field.to_ascii_lowercase());
        } else {
            whole_signal_names.insert(sig.to_ascii_lowercase());
        }
    };

    for proc in &input.processes {
        for sig in &proc.read_signals {
            process_name(sig);
        }
        for sig in &proc.assigned_signals {
            process_name(sig);
        }
    }
    for ca in &input.concurrent_assignments {
        for sig in &ca.read_signals {
            process_name(sig);
        }
        process_name(&ca.target);
    }
    for inst in &input.instances {
        for val in inst.port_map.values() {
            process_name(val);
        }
        for assoc in &inst.associations {
            if !assoc.actual_base.is_empty() {
                process_name(&assoc.actual_base);
            }
        }
    }

    // Build map from record type name -> set of signals of that type
    let mut record_type_signals: std::collections::HashMap<String, Vec<String>> =
        std::collections::HashMap::new();
    for sig in &input.signals {
        let base = helpers::base_type_name(&sig.r#type);
        record_type_signals
            .entry(base)
            .or_default()
            .push(sig.name.to_ascii_lowercase());
    }

    let mut out = Vec::new();
    for td in &input.types {
        if td.kind != "record" {
            continue;
        }
        if td.in_arch.is_empty() || !td.in_package.is_empty() {
            continue;
        }
        if helpers::file_in_testbench(input, &td.file) {
            continue;
        }
        // If any signal of this record type is used as a whole (no field qualification),
        // consider all fields as referenced
        let type_lower = td.name.to_ascii_lowercase();
        let whole_record_used = record_type_signals
            .get(&type_lower)
            .map(|sigs| sigs.iter().any(|s| whole_signal_names.contains(s)))
            .unwrap_or(false);
        if whole_record_used {
            continue;
        }
        for field in &td.fields {
            if !referenced_fields.contains(&field.name.to_ascii_lowercase()) {
                out.push(Violation {
                    rule: "record_field_unused".to_string(),
                    severity: "info".to_string(),
                    file: td.file.clone(),
                    line: td.line,
                    message: format!(
                        "Record field '{}' in type '{}' is never referenced",
                        field.name, td.name
                    ),
                });
            }
        }
    }
    out
}

fn dead_generate(input: &Input) -> Vec<Violation> {
    input
        .generates
        .iter()
        .filter(|gen| gen.kind.eq_ignore_ascii_case("for"))
        .filter(|gen| gen.can_elaborate)
        .filter(|gen| gen.iteration_count == 0)
        .filter(|gen| !helpers::file_in_testbench(input, &gen.file))
        .map(|gen| Violation {
            rule: "dead_generate".to_string(),
            severity: "warning".to_string(),
            file: gen.file.clone(),
            line: gen.line,
            message: format!(
                "For-generate '{}' has 0 iterations — generates no hardware",
                gen.label
            ),
        })
        .collect()
}

fn generate_range_mismatch(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for gen in &input.generates {
        if !gen.kind.eq_ignore_ascii_case("for") {
            continue;
        }
        if !gen.can_elaborate || gen.iteration_count <= 0 {
            continue;
        }
        if helpers::file_in_testbench(input, &gen.file) {
            continue;
        }
        let iter_count = gen.iteration_count as usize;
        for sig_name in &gen.signals {
            if let Some(sig) = input
                .signals
                .iter()
                .find(|s| s.name.eq_ignore_ascii_case(sig_name))
            {
                if sig.width > 0 && sig.width != iter_count {
                    out.push(Violation {
                        rule: "generate_range_mismatch".to_string(),
                        severity: "info".to_string(),
                        file: gen.file.clone(),
                        line: gen.line,
                        message: format!(
                            "For-generate '{}' iterates {} times but signal '{}' has width {} — verify range",
                            gen.label, iter_count, sig.name, sig.width
                        ),
                    });
                }
            }
        }
    }
    out
}

fn build_referenced_names(input: &Input) -> HashSet<String> {
    let mut names: HashSet<String> = HashSet::new();
    for proc in &input.processes {
        for sig in &proc.read_signals {
            names.insert(sig.to_ascii_lowercase());
        }
        for sig in &proc.assigned_signals {
            names.insert(sig.to_ascii_lowercase());
        }
    }
    for ca in &input.concurrent_assignments {
        for sig in &ca.read_signals {
            names.insert(sig.to_ascii_lowercase());
        }
        names.insert(ca.target.to_ascii_lowercase());
    }
    for comp in &input.comparisons {
        names.insert(comp.left_operand.to_ascii_lowercase());
        names.insert(comp.right_operand.to_ascii_lowercase());
    }
    for inst in &input.instances {
        for val in inst.generic_map.values() {
            names.insert(val.to_ascii_lowercase());
        }
        for val in inst.port_map.values() {
            names.insert(val.to_ascii_lowercase());
        }
        for assoc in &inst.associations {
            if !assoc.actual_base.is_empty() {
                names.insert(assoc.actual_base.to_ascii_lowercase());
            }
        }
    }
    // Include constant initializer values (e.g. constant X := GENERIC_NAME)
    for c in &input.constant_decls {
        if !c.value.is_empty() {
            // Split on common operators/spaces to extract individual identifiers
            for token in c.value.split(|ch: char| !ch.is_alphanumeric() && ch != '_') {
                if !token.is_empty() {
                    names.insert(token.to_ascii_lowercase());
                }
            }
        }
    }
    names
}

fn build_referenced_type_names(input: &Input) -> HashSet<String> {
    let mut names: HashSet<String> = HashSet::new();
    let mut add_type = |type_str: &str| {
        let base = helpers::base_type_name(type_str);
        if !base.is_empty() {
            names.insert(base);
        }
    };
    for sig in &input.signals {
        add_type(&sig.r#type);
    }
    for port in &input.ports {
        add_type(&port.r#type);
    }
    for proc in &input.processes {
        for var in &proc.variables {
            add_type(&var.r#type);
        }
    }
    for c in &input.constant_decls {
        add_type(&c.r#type);
    }
    for st in &input.subtypes {
        add_type(&st.base_type);
    }
    for func in &input.functions {
        add_type(&func.return_type);
        for param in &func.parameters {
            add_type(&param.r#type);
        }
    }
    for proc in &input.procedures {
        for param in &proc.parameters {
            add_type(&param.r#type);
        }
    }
    names
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::policy::input::{
        Architecture, Component, ConstantDeclaration, Entity, FunctionDeclaration,
        GenerateStatement, GenericDecl, Input, Instance, ProcedureDeclaration, Process,
        RecordField, Signal, SubtypeDeclaration, TypeDeclaration,
    };

    #[test]
    fn unused_constant_flags() {
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "ent".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        input.architectures.push(Architecture {
            name: "rtl".to_string(),
            entity_name: "ent".to_string(),
            file: "a.vhd".to_string(),
            line: 2,
        });
        input.constant_decls.push(ConstantDeclaration {
            name: "DEAD_CONST".to_string(),
            r#type: "integer".to_string(),
            file: "a.vhd".to_string(),
            line: 3,
            in_arch: "rtl".to_string(),
            ..Default::default()
        });
        let v = unused_constant(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "unused_constant");
    }

    #[test]
    fn unused_constant_skip_referenced() {
        let mut input = Input::default();
        input.constant_decls.push(ConstantDeclaration {
            name: "USED_CONST".to_string(),
            r#type: "integer".to_string(),
            file: "a.vhd".to_string(),
            line: 3,
            in_arch: "rtl".to_string(),
            ..Default::default()
        });
        input.processes.push(Process {
            read_signals: vec!["used_const".to_string()],
            file: "a.vhd".to_string(),
            ..Default::default()
        });
        let v = unused_constant(&input);
        assert!(v.is_empty());
    }

    #[test]
    fn unused_constant_skip_package() {
        let mut input = Input::default();
        input.constant_decls.push(ConstantDeclaration {
            name: "PKG_CONST".to_string(),
            r#type: "integer".to_string(),
            file: "a.vhd".to_string(),
            line: 3,
            in_arch: "rtl".to_string(),
            in_package: "my_pkg".to_string(),
            ..Default::default()
        });
        let v = unused_constant(&input);
        assert!(v.is_empty());
    }

    #[test]
    fn unused_component_flags() {
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "ent".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        input.components.push(Component {
            name: "dead_comp".to_string(),
            is_instance: false,
            file: "a.vhd".to_string(),
            line: 5,
            ..Default::default()
        });
        let v = unused_component(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "unused_component");
    }

    #[test]
    fn unused_component_skip_instantiated() {
        let mut input = Input::default();
        input.components.push(Component {
            name: "used_comp".to_string(),
            is_instance: false,
            file: "a.vhd".to_string(),
            line: 5,
            ..Default::default()
        });
        input.instances.push(Instance {
            target: "used_comp".to_string(),
            ..Default::default()
        });
        let v = unused_component(&input);
        assert!(v.is_empty());
    }

    #[test]
    fn unused_type_flags() {
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "ent".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        input.types.push(TypeDeclaration {
            name: "dead_type".to_string(),
            kind: "enum".to_string(),
            file: "a.vhd".to_string(),
            line: 4,
            in_arch: "rtl".to_string(),
            ..Default::default()
        });
        let v = unused_type(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "unused_type");
    }

    #[test]
    fn unused_type_skip_used_by_signal() {
        let mut input = Input::default();
        input.types.push(TypeDeclaration {
            name: "my_type".to_string(),
            kind: "enum".to_string(),
            file: "a.vhd".to_string(),
            line: 4,
            in_arch: "rtl".to_string(),
            ..Default::default()
        });
        input.signals.push(Signal {
            name: "sig".to_string(),
            r#type: "my_type".to_string(),
            file: "a.vhd".to_string(),
            line: 5,
            ..Default::default()
        });
        let v = unused_type(&input);
        assert!(v.is_empty());
    }

    #[test]
    fn unused_subprogram_flags_function() {
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "ent".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        input.functions.push(FunctionDeclaration {
            name: "dead_func".to_string(),
            file: "a.vhd".to_string(),
            line: 6,
            in_arch: "rtl".to_string(),
            ..Default::default()
        });
        let v = unused_subprogram(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "unused_subprogram");
        assert!(v[0].message.contains("dead_func"));
    }

    #[test]
    fn unused_subprogram_flags_procedure() {
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "ent".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        input.procedures.push(ProcedureDeclaration {
            name: "dead_proc".to_string(),
            file: "a.vhd".to_string(),
            line: 7,
            in_arch: "rtl".to_string(),
            ..Default::default()
        });
        let v = unused_subprogram(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "unused_subprogram");
        assert!(v[0].message.contains("dead_proc"));
    }

    #[test]
    fn unused_subprogram_skip_called() {
        let mut input = Input::default();
        input.functions.push(FunctionDeclaration {
            name: "used_func".to_string(),
            file: "a.vhd".to_string(),
            line: 6,
            in_arch: "rtl".to_string(),
            ..Default::default()
        });
        input.processes.push(Process {
            function_calls: vec![crate::policy::input::FunctionCall {
                name: "used_func".to_string(),
                ..Default::default()
            }],
            file: "a.vhd".to_string(),
            ..Default::default()
        });
        let v = unused_subprogram(&input);
        assert!(v.is_empty());
    }

    #[test]
    fn unused_subprogram_skip_package() {
        let mut input = Input::default();
        input.functions.push(FunctionDeclaration {
            name: "pkg_func".to_string(),
            file: "a.vhd".to_string(),
            line: 6,
            in_arch: "rtl".to_string(),
            in_package: "my_pkg".to_string(),
            ..Default::default()
        });
        let v = unused_subprogram(&input);
        assert!(v.is_empty());
    }

    #[test]
    fn unused_subtype_flags() {
        let mut input = Input::default();
        input.subtypes.push(SubtypeDeclaration {
            name: "dead_sub".to_string(),
            base_type: "integer".to_string(),
            file: "a.vhd".to_string(),
            line: 4,
            in_arch: "rtl".to_string(),
            ..Default::default()
        });
        let v = unused_subtype(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "unused_subtype");
    }

    #[test]
    fn unused_subtype_skip_used() {
        let mut input = Input::default();
        input.subtypes.push(SubtypeDeclaration {
            name: "my_int".to_string(),
            base_type: "integer".to_string(),
            file: "a.vhd".to_string(),
            line: 4,
            in_arch: "rtl".to_string(),
            ..Default::default()
        });
        input.signals.push(Signal {
            name: "cnt".to_string(),
            r#type: "my_int".to_string(),
            file: "a.vhd".to_string(),
            line: 5,
            ..Default::default()
        });
        let v = unused_subtype(&input);
        assert!(v.is_empty());
    }

    #[test]
    fn unused_generic_flags() {
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "ent".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            generics: vec![GenericDecl {
                name: "DEAD_GEN".to_string(),
                line: 2,
                ..Default::default()
            }],
            ..Default::default()
        });
        let v = unused_generic(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "unused_generic");
    }

    #[test]
    fn unused_generic_skip_referenced() {
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "ent".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            generics: vec![GenericDecl {
                name: "WIDTH".to_string(),
                line: 2,
                ..Default::default()
            }],
            ..Default::default()
        });
        input.processes.push(Process {
            read_signals: vec!["width".to_string()],
            file: "a.vhd".to_string(),
            ..Default::default()
        });
        let v = unused_generic(&input);
        assert!(v.is_empty());
    }

    #[test]
    fn record_field_unused_flags() {
        let mut input = Input::default();
        input.types.push(TypeDeclaration {
            name: "my_rec".to_string(),
            kind: "record".to_string(),
            file: "a.vhd".to_string(),
            line: 3,
            in_arch: "rtl".to_string(),
            fields: vec![
                RecordField {
                    name: "used_field".to_string(),
                    r#type: "std_logic".to_string(),
                },
                RecordField {
                    name: "dead_field".to_string(),
                    r#type: "std_logic".to_string(),
                },
            ],
            ..Default::default()
        });
        input.processes.push(Process {
            read_signals: vec!["r.used_field".to_string()],
            file: "a.vhd".to_string(),
            ..Default::default()
        });
        let v = record_field_unused(&input);
        assert_eq!(v.len(), 1);
        assert!(v[0].message.contains("dead_field"));
    }

    #[test]
    fn dead_generate_flags() {
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "ent".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        input.generates.push(GenerateStatement {
            label: "gen_dead".to_string(),
            kind: "for".to_string(),
            can_elaborate: true,
            iteration_count: 0,
            file: "a.vhd".to_string(),
            line: 5,
            ..Default::default()
        });
        let v = dead_generate(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "dead_generate");
    }

    #[test]
    fn dead_generate_skip_nonzero() {
        let mut input = Input::default();
        input.generates.push(GenerateStatement {
            label: "gen_ok".to_string(),
            kind: "for".to_string(),
            can_elaborate: true,
            iteration_count: 8,
            file: "a.vhd".to_string(),
            line: 5,
            ..Default::default()
        });
        let v = dead_generate(&input);
        assert!(v.is_empty());
    }

    #[test]
    fn generate_range_mismatch_flags() {
        let mut input = Input::default();
        input.generates.push(GenerateStatement {
            label: "gen_mis".to_string(),
            kind: "for".to_string(),
            can_elaborate: true,
            iteration_count: 10,
            signals: vec!["bus".to_string()],
            file: "a.vhd".to_string(),
            line: 5,
            ..Default::default()
        });
        input.signals.push(Signal {
            name: "bus".to_string(),
            width: 8,
            file: "a.vhd".to_string(),
            line: 3,
            ..Default::default()
        });
        let v = generate_range_mismatch(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "generate_range_mismatch");
    }

    #[test]
    fn generate_range_mismatch_skip_matching() {
        let mut input = Input::default();
        input.generates.push(GenerateStatement {
            label: "gen_ok".to_string(),
            kind: "for".to_string(),
            can_elaborate: true,
            iteration_count: 8,
            signals: vec!["bus".to_string()],
            file: "a.vhd".to_string(),
            line: 5,
            ..Default::default()
        });
        input.signals.push(Signal {
            name: "bus".to_string(),
            width: 8,
            file: "a.vhd".to_string(),
            line: 3,
            ..Default::default()
        });
        let v = generate_range_mismatch(&input);
        assert!(v.is_empty());
    }
}
