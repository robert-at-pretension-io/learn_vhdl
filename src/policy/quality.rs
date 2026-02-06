use regex::Regex;
use std::collections::{HashMap, HashSet};

use crate::policy::helpers;
use crate::policy::input::{Input, Port};
use crate::policy::result::Violation;

pub fn violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(buffer_port(input));
    out.extend(trivial_architecture(input));
    out.extend(unlabeled_generate(input));
    out
}

pub fn optional_violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(duplicate_signal_in_entity(input));
    out.extend(very_long_file(input));
    out.extend(large_package(input));
    out.extend(short_signal_name(input));
    out.extend(long_signal_name(input));
    out.extend(short_port_name(input));
    out.extend(entity_name_with_numbers(input));
    out.extend(mixed_port_directions(input));
    out.extend(bidirectional_port(input));
    out.extend(many_signals(input));
    out.extend(deep_generate_nesting(input));
    out.extend(magic_width_number(input));
    out.extend(hardcoded_generic(input));
    out.extend(file_entity_mismatch(input));
    out.extend(duplicate_port_in_entity(input));
    out.extend(duplicate_entity_in_file(input));
    out.extend(duplicate_use_clause(input));
    out.extend(use_all_abuse(input));
    out.extend(signal_fanout(input));
    out.extend(unused_library_clause(input));
    out.extend(unused_use_clause(input));
    out
}

fn very_long_file(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    let mut files: Vec<&str> = input.entities.iter().map(|e| e.file.as_str()).collect();
    files.sort();
    files.dedup();
    for file in files {
        let entities_in_file = input.entities.iter().filter(|e| e.file == file).count();
        let archs_in_file = input
            .architectures
            .iter()
            .filter(|a| a.file == file)
            .count();
        let total = entities_in_file + archs_in_file;
        if total > 5 {
            out.push(Violation {
                rule: "very_long_file".to_string(),
                severity: "info".to_string(),
                file: file.to_string(),
                line: 1,
                message: format!(
                    "File contains {} design units - consider splitting into separate files",
                    total
                ),
            });
        }
    }
    out
}

fn large_package(input: &Input) -> Vec<Violation> {
    input
        .packages
        .iter()
        .filter_map(|pkg| {
            let count = input
                .signals
                .iter()
                .filter(|s| s.in_entity == pkg.name)
                .count();
            if count > 50 {
                Some(Violation {
                    rule: "large_package".to_string(),
                    severity: "info".to_string(),
                    file: pkg.file.clone(),
                    line: pkg.line,
                    message: format!(
                        "Package '{}' is very large ({} items) - consider splitting",
                        pkg.name, count
                    ),
                })
            } else {
                None
            }
        })
        .collect()
}

fn short_signal_name(input: &Input) -> Vec<Violation> {
    input
        .signals
        .iter()
        .filter(|sig| sig.name.chars().count() == 1)
        .filter(|sig| !is_loop_variable(&sig.name))
        .map(|sig| Violation {
            rule: "short_signal_name".to_string(),
            severity: "info".to_string(),
            file: sig.file.clone(),
            line: sig.line,
            message: format!(
                "Signal '{}' has very short name - consider a more descriptive name",
                sig.name
            ),
        })
        .collect()
}

fn long_signal_name(input: &Input) -> Vec<Violation> {
    input
        .signals
        .iter()
        .filter(|sig| sig.name.chars().count() > 40)
        .map(|sig| Violation {
            rule: "long_signal_name".to_string(),
            severity: "info".to_string(),
            file: sig.file.clone(),
            line: sig.line,
            message: format!(
                "Signal '{}' has very long name ({} chars) - consider abbreviating",
                sig.name,
                sig.name.chars().count()
            ),
        })
        .collect()
}

fn short_port_name(input: &Input) -> Vec<Violation> {
    input
        .ports
        .iter()
        .filter(|port| port.name.chars().count() == 1)
        .filter(|port| !is_loop_variable(&port.name))
        .map(|port| Violation {
            rule: "short_port_name".to_string(),
            severity: "info".to_string(),
            file: entity_file(input, port).unwrap_or_default(),
            line: port.line,
            message: format!(
                "Port '{}' has very short name - consider a more descriptive name",
                port.name
            ),
        })
        .collect()
}

fn entity_name_with_numbers(input: &Input) -> Vec<Violation> {
    let re = Regex::new(".*[0-9].*").unwrap();
    input
        .entities
        .iter()
        .filter(|entity| re.is_match(&entity.name))
        .filter(|entity| !is_versioned_name(&entity.name))
        .map(|entity| Violation {
            rule: "entity_name_with_numbers".to_string(),
            severity: "info".to_string(),
            file: entity.file.clone(),
            line: entity.line,
            message: format!(
                "Entity '{}' contains numbers - consider a more descriptive name",
                entity.name
            ),
        })
        .collect()
}

fn mixed_port_directions(input: &Input) -> Vec<Violation> {
    input
        .entities
        .iter()
        .filter_map(|entity| {
            if entity.ports.len() <= 4 {
                return None;
            }
            if has_direction_alternation(&entity.ports) {
                Some(Violation {
                    rule: "mixed_port_directions".to_string(),
                    severity: "info".to_string(),
                    file: entity.file.clone(),
                    line: entity.line,
                    message: format!(
                        "Entity '{}' has mixed port directions - consider grouping inputs and outputs together",
                        entity.name
                    ),
                })
            } else {
                None
            }
        })
        .collect()
}

fn bidirectional_port(input: &Input) -> Vec<Violation> {
    input
        .ports
        .iter()
        .filter(|port| port.direction.eq_ignore_ascii_case("inout"))
        .map(|port| Violation {
            rule: "bidirectional_port".to_string(),
            severity: "info".to_string(),
            file: entity_file(input, port).unwrap_or_default(),
            line: port.line,
            message: format!(
                "Port '{}' is bidirectional (inout) - consider separate in/out ports unless truly needed",
                port.name
            ),
        })
        .collect()
}

fn buffer_port(input: &Input) -> Vec<Violation> {
    input
        .ports
        .iter()
        .filter(|port| port.direction.eq_ignore_ascii_case("buffer"))
        .map(|port| Violation {
            rule: "buffer_port".to_string(),
            severity: "warning".to_string(),
            file: entity_file(input, port).unwrap_or_default(),
            line: port.line,
            message: format!(
                "Port '{}' uses deprecated 'buffer' direction - use 'out' with internal signal instead",
                port.name
            ),
        })
        .collect()
}

fn trivial_architecture(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for arch in &input.architectures {
        let procs = input
            .processes
            .iter()
            .filter(|p| p.in_arch == arch.name && p.file == arch.file)
            .count();
        let concurrents = input
            .concurrent_assignments
            .iter()
            .filter(|c| c.in_arch == arch.name && c.file == arch.file)
            .count();
        let instances = input
            .instances
            .iter()
            .filter(|i| i.in_arch == arch.name && i.file == arch.file)
            .count();
        let gen_procs = input
            .processes
            .iter()
            .filter(|p| p.in_arch.starts_with(&format!("{}.", arch.name)) && p.file == arch.file)
            .count();
        let gen_concurrents = input
            .concurrent_assignments
            .iter()
            .filter(|c| c.in_arch.starts_with(&format!("{}.", arch.name)) && c.file == arch.file)
            .count();
        let gen_instances = input
            .instances
            .iter()
            .filter(|i| i.in_arch.starts_with(&format!("{}.", arch.name)) && i.file == arch.file)
            .count();
        let generates = input
            .generates
            .iter()
            .filter(|g| g.in_arch == arch.name && g.file == arch.file)
            .count();
        if procs + gen_procs == 0
            && concurrents + gen_concurrents == 0
            && instances + gen_instances == 0
            && generates == 0
        {
            out.push(Violation {
                rule: "trivial_architecture".to_string(),
                severity: "warning".to_string(),
                file: arch.file.clone(),
                line: arch.line,
                message: format!(
                    "Architecture '{}' has no processes, concurrent statements, or instances",
                    arch.name
                ),
            });
        }
    }
    out
}

fn file_entity_mismatch(input: &Input) -> Vec<Violation> {
    input
        .entities
        .iter()
        .filter_map(|entity| {
            let filename = extract_filename(&entity.file);
            let entities_in_file = input
                .entities
                .iter()
                .filter(|e| e.file == entity.file)
                .count();
            if entities_in_file != 1 {
                return None;
            }
            if filename.to_ascii_lowercase() != entity.name.to_ascii_lowercase() {
                Some(Violation {
                    rule: "file_entity_mismatch".to_string(),
                    severity: "info".to_string(),
                    file: entity.file.clone(),
                    line: entity.line,
                    message: format!(
                        "Entity '{}' is in file '{}' - consider renaming file to '{}.vhd'",
                        entity.name, filename, entity.name
                    ),
                })
            } else {
                None
            }
        })
        .collect()
}

fn unlabeled_generate(input: &Input) -> Vec<Violation> {
    input
        .generates
        .iter()
        .filter(|gen| gen.label.is_empty())
        .map(|gen| Violation {
            rule: "unlabeled_generate".to_string(),
            severity: "warning".to_string(),
            file: gen.file.clone(),
            line: gen.line,
            message: "Generate block without label - labels are required for generate blocks"
                .to_string(),
        })
        .collect()
}

fn many_signals(input: &Input) -> Vec<Violation> {
    input
        .entities
        .iter()
        .filter_map(|entity| {
            let signals = input
                .signals
                .iter()
                .filter(|s| s.in_entity == entity.name)
                .count();
            if signals > 50 {
                Some(Violation {
                    rule: "many_signals".to_string(),
                    severity: "info".to_string(),
                    file: entity.file.clone(),
                    line: entity.line,
                    message: format!(
                        "Entity '{}' has {} signals - consider refactoring into sub-modules",
                        entity.name, signals
                    ),
                })
            } else {
                None
            }
        })
        .collect()
}

fn deep_generate_nesting(input: &Input) -> Vec<Violation> {
    input
        .generates
        .iter()
        .filter_map(|gen| {
            let dots = gen.in_arch.split('.').count().saturating_sub(1);
            if dots > 3 {
                Some(Violation {
                    rule: "deep_generate_nesting".to_string(),
                    severity: "info".to_string(),
                    file: gen.file.clone(),
                    line: gen.line,
                    message: format!(
                        "Generate block '{}' is deeply nested ({} levels) - consider flattening",
                        gen.label, dots
                    ),
                })
            } else {
                None
            }
        })
        .collect()
}

fn magic_width_number(input: &Input) -> Vec<Violation> {
    let re = Regex::new(r"\(\s*([0-9]+)\s+downto\s+([0-9]+)\s*\)").unwrap();
    input
        .signals
        .iter()
        .filter_map(|sig| {
            let width = if sig.width > 0 {
                sig.width as i32
            } else {
                let lower = sig.r#type.to_ascii_lowercase();
                if let Some(caps) = re.captures(&lower) {
                    let high: i32 = caps.get(1)?.as_str().parse().ok()?;
                    let low: i32 = caps.get(2)?.as_str().parse().ok()?;
                    high - low + 1
                } else {
                    0
                }
            };
            if width > 8 && !matches!(width, 16 | 32 | 64 | 128) {
                return Some(Violation {
                    rule: "magic_width_number".to_string(),
                    severity: "info".to_string(),
                    file: sig.file.clone(),
                    line: sig.line,
                    message: format!(
                        "Signal '{}' has magic width {} - consider using a constant",
                        sig.name, width
                    ),
                });
            }
            None
        })
        .collect()
}

fn duplicate_signal_in_entity(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    let mut seen = std::collections::HashMap::new();
    for sig in &input.signals {
        let key = format!(
            "{}|{}|{}",
            sig.file,
            sig.in_entity,
            sig.name.to_ascii_lowercase()
        );
        if let Some(first_line) = seen.get(&key) {
            out.push(Violation {
                rule: "duplicate_signal_in_entity".to_string(),
                severity: "error".to_string(),
                file: sig.file.clone(),
                line: sig.line,
                message: format!(
                    "Signal '{}' declared multiple times in same scope (first at line {})",
                    sig.name, first_line
                ),
            });
        } else {
            seen.insert(key, sig.line);
        }
    }
    out
}

fn duplicate_port_in_entity(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    let mut seen = std::collections::HashMap::new();
    for port in &input.ports {
        let file = entity_file(input, port).unwrap_or_default();
        let key = format!(
            "{}|{}|{}",
            port.in_entity,
            file,
            port.name.to_ascii_lowercase()
        );
        if let Some(first_line) = seen.get(&key) {
            out.push(Violation {
                rule: "duplicate_port_in_entity".to_string(),
                severity: "error".to_string(),
                file,
                line: port.line,
                message: format!(
                    "Port '{}' declared multiple times in same entity (first at line {})",
                    port.name, first_line
                ),
            });
        } else {
            seen.insert(key, port.line);
        }
    }
    out
}

fn duplicate_entity_in_file(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    let mut seen = std::collections::HashMap::new();
    for entity in &input.entities {
        let key = format!("{}|{}", entity.file, entity.name.to_ascii_lowercase());
        if let Some(first_line) = seen.get(&key) {
            out.push(Violation {
                rule: "duplicate_entity_in_file".to_string(),
                severity: "error".to_string(),
                file: entity.file.clone(),
                line: entity.line,
                message: format!(
                    "Entity '{}' declared multiple times in same file (first at line {})",
                    entity.name, first_line
                ),
            });
        } else {
            seen.insert(key, entity.line);
        }
    }
    out
}

fn hardcoded_generic(input: &Input) -> Vec<Violation> {
    let re = Regex::new(r"^[0-9]+$").unwrap();
    let mut out = Vec::new();
    for inst in &input.instances {
        for value in inst.generic_map.values() {
            if re.is_match(value) {
                if let Ok(num) = value.parse::<i32>() {
                    if num > 8 {
                        out.push(Violation {
                            rule: "hardcoded_generic".to_string(),
                            severity: "info".to_string(),
                            file: inst.file.clone(),
                            line: inst.line,
                            message: format!(
                                "Instance '{}' has hardcoded generic value '{}' - consider using a constant or generic",
                                inst.name, value
                            ),
                        });
                    }
                }
            }
        }
    }
    out
}

fn duplicate_use_clause(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    let mut seen: HashMap<String, usize> = HashMap::new();
    for uc in &input.use_clauses {
        for item in &uc.items {
            let key = format!("{}|{}", uc.file, item.to_ascii_lowercase());
            if let Some(first_line) = seen.get(&key) {
                out.push(Violation {
                    rule: "duplicate_use_clause".to_string(),
                    severity: "info".to_string(),
                    file: uc.file.clone(),
                    line: uc.line,
                    message: format!(
                        "Use clause '{}' duplicated (first at line {})",
                        item, first_line
                    ),
                });
            } else {
                seen.insert(key, uc.line);
            }
        }
    }
    out
}

fn use_all_abuse(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    let standard_pkgs: HashSet<&str> = [
        "ieee.std_logic_1164",
        "ieee.numeric_std",
        "ieee.std_logic_arith",
        "ieee.std_logic_unsigned",
        "ieee.std_logic_signed",
        "ieee.math_real",
        "ieee.math_complex",
        "std.textio",
        "std.standard",
        "std.env",
    ]
    .iter()
    .copied()
    .collect();
    for uc in &input.use_clauses {
        for item in &uc.items {
            if !item.to_ascii_lowercase().ends_with(".all") {
                continue;
            }
            let pkg = item
                .to_ascii_lowercase()
                .trim_end_matches(".all")
                .to_string();
            if standard_pkgs.contains(pkg.as_str()) {
                continue;
            }
            if helpers::file_in_testbench(input, &uc.file) {
                continue;
            }
            out.push(Violation {
                rule: "use_all_abuse".to_string(),
                severity: "info".to_string(),
                file: uc.file.clone(),
                line: uc.line,
                message: format!(
                    "Use clause '{}' imports everything — consider importing specific items",
                    item
                ),
            });
        }
    }
    out
}

fn signal_fanout(input: &Input) -> Vec<Violation> {
    let mut reader_count: HashMap<String, usize> = HashMap::new();
    for proc in &input.processes {
        let mut seen_in_proc: HashSet<String> = HashSet::new();
        for sig in &proc.read_signals {
            let key = sig.to_ascii_lowercase();
            if seen_in_proc.insert(key.clone()) {
                *reader_count.entry(key).or_insert(0) += 1;
            }
        }
    }
    let mut out = Vec::new();
    for sig in &input.signals {
        let key = sig.name.to_ascii_lowercase();
        if let Some(&count) = reader_count.get(&key) {
            if count > 10 {
                if helpers::file_in_testbench(input, &sig.file) {
                    continue;
                }
                out.push(Violation {
                    rule: "signal_fanout".to_string(),
                    severity: "info".to_string(),
                    file: sig.file.clone(),
                    line: sig.line,
                    message: format!(
                        "Signal '{}' is read by {} processes — high fanout may cause timing issues",
                        sig.name, count
                    ),
                });
            }
        }
    }
    out
}

fn unused_library_clause(input: &Input) -> Vec<Violation> {
    let skip_libs: HashSet<&str> = ["ieee", "std", "work"].iter().copied().collect();
    let mut out = Vec::new();
    for lc in &input.library_clauses {
        if helpers::file_in_testbench(input, &lc.file) {
            continue;
        }
        for lib in &lc.libraries {
            let lib_lower = lib.to_ascii_lowercase();
            if skip_libs.contains(lib_lower.as_str()) {
                continue;
            }
            let prefix = format!("{}.", lib_lower);
            let used_in_use = input
                .use_clauses
                .iter()
                .any(|uc| {
                    uc.file == lc.file
                        && uc.items.iter().any(|item| {
                            item.to_ascii_lowercase().starts_with(&prefix)
                        })
                });
            if used_in_use {
                continue;
            }
            let used_in_instance = input
                .instances
                .iter()
                .any(|inst| {
                    inst.file == lc.file
                        && inst.target.to_ascii_lowercase().starts_with(&prefix)
                });
            if used_in_instance {
                continue;
            }
            let used_in_context = input
                .context_clauses
                .iter()
                .any(|cc| {
                    cc.file == lc.file
                        && cc.name.to_ascii_lowercase().starts_with(&prefix)
                });
            if used_in_context {
                continue;
            }
            out.push(Violation {
                rule: "unused_library_clause".to_string(),
                severity: "info".to_string(),
                file: lc.file.clone(),
                line: lc.line,
                message: format!(
                    "Library '{}' is declared but never referenced by a use clause or entity instantiation",
                    lib
                ),
            });
        }
    }
    out
}

fn std_package_exports(pkg: &str) -> Option<Vec<&'static str>> {
    match pkg {
        "ieee.std_logic_1164" => Some(vec![
            "std_logic", "std_logic_vector", "std_ulogic", "std_ulogic_vector",
            "to_bit", "to_bitvector", "to_stdulogic", "to_stdulogicvector",
            "to_stdlogicvector", "rising_edge", "falling_edge", "is_x", "to_01",
            "to_x01", "to_x01z", "to_ux01", "resolved",
        ]),
        "ieee.numeric_std" => Some(vec![
            "signed", "unsigned", "to_integer", "to_unsigned", "to_signed",
            "resize", "shift_left", "shift_right", "rotate_left", "rotate_right",
        ]),
        "ieee.std_logic_arith" => Some(vec![
            "signed", "unsigned", "conv_integer", "conv_unsigned", "conv_signed",
            "conv_std_logic_vector",
        ]),
        "ieee.math_real" => Some(vec![
            "math_pi", "math_e", "ceil", "floor", "round", "log2", "sqrt",
            "realmax", "realmin", "math_2_pi", "math_1_over_pi",
        ]),
        "ieee.math_complex" => Some(vec!["complex", "complex_polar"]),
        "std.textio" => Some(vec![
            "line", "text", "readline", "writeline", "read", "write",
            "file_open", "file_close", "hread", "hwrite", "oread", "owrite",
        ]),
        "std.env" => Some(vec!["stop", "finish", "resolution_limit"]),
        // std_logic_unsigned/signed provide operator overloads — skip
        _ => None,
    }
}

fn collect_used_names(input: &Input, file: &str) -> HashSet<String> {
    let mut names = HashSet::new();
    // Signal types
    for sig in &input.signals {
        if sig.file == file {
            let base = helpers::base_type_name(&sig.r#type);
            if !base.is_empty() {
                names.insert(base);
            }
        }
    }
    // Port types
    for port in &input.ports {
        let port_file = input
            .entities
            .iter()
            .find(|e| e.name.eq_ignore_ascii_case(&port.in_entity))
            .map(|e| e.file.as_str())
            .unwrap_or("");
        if port_file == file {
            let base = helpers::base_type_name(&port.r#type);
            if !base.is_empty() {
                names.insert(base);
            }
        }
    }
    // Process variable types, function/procedure calls, read/assigned signals
    for proc in &input.processes {
        if proc.file == file {
            for var in &proc.variables {
                let base = helpers::base_type_name(&var.r#type);
                if !base.is_empty() {
                    names.insert(base);
                }
            }
            for fc in &proc.function_calls {
                names.insert(fc.name.to_ascii_lowercase());
            }
            for pc in &proc.procedure_calls {
                names.insert(pc.name.to_ascii_lowercase());
            }
            for sig in &proc.read_signals {
                names.insert(sig.to_ascii_lowercase());
            }
            for sig in &proc.assigned_signals {
                names.insert(sig.to_ascii_lowercase());
            }
        }
    }
    // Constant types
    for cd in &input.constant_decls {
        if cd.file == file {
            let base = helpers::base_type_name(&cd.r#type);
            if !base.is_empty() {
                names.insert(base);
            }
        }
    }
    // Subtype base types
    for st in &input.subtypes {
        if st.file == file {
            let base = helpers::base_type_name(&st.base_type);
            if !base.is_empty() {
                names.insert(base);
            }
        }
    }
    // Type declarations (for record/enum usage)
    for td in &input.types {
        if td.file == file {
            names.insert(td.name.to_ascii_lowercase());
        }
    }
    // Concurrent assignment read signals
    for ca in &input.concurrent_assignments {
        if ca.file == file {
            for sig in &ca.read_signals {
                names.insert(sig.to_ascii_lowercase());
            }
        }
    }
    // Generic types
    for entity in &input.entities {
        if entity.file == file {
            for gen in &entity.generics {
                let base = helpers::base_type_name(&gen.r#type);
                if !base.is_empty() {
                    names.insert(base);
                }
            }
        }
    }
    // Function/procedure declarations (return types, parameter types)
    for f in &input.functions {
        if f.file == file {
            let base = helpers::base_type_name(&f.return_type);
            if !base.is_empty() {
                names.insert(base);
            }
            for p in &f.parameters {
                let base = helpers::base_type_name(&p.r#type);
                if !base.is_empty() {
                    names.insert(base);
                }
            }
        }
    }
    for p in &input.procedures {
        if p.file == file {
            for param in &p.parameters {
                let base = helpers::base_type_name(&param.r#type);
                if !base.is_empty() {
                    names.insert(base);
                }
            }
        }
    }
    names
}

fn unused_use_clause(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    // Cache used names per file
    let mut file_names_cache: HashMap<String, HashSet<String>> = HashMap::new();
    // Build user package exports: package_key -> set of exported names
    let mut user_pkg_exports: HashMap<String, HashSet<String>> = HashMap::new();
    for td in &input.types {
        if !td.in_package.is_empty() {
            // Find which library this package lives in
            let lib = package_library(input, &td.in_package, &td.file);
            let key = format!("{}.{}", lib, td.in_package.to_ascii_lowercase());
            user_pkg_exports
                .entry(key)
                .or_default()
                .insert(td.name.to_ascii_lowercase());
        }
    }
    for st in &input.subtypes {
        if !st.in_package.is_empty() {
            let lib = package_library(input, &st.in_package, &st.file);
            let key = format!("{}.{}", lib, st.in_package.to_ascii_lowercase());
            user_pkg_exports
                .entry(key)
                .or_default()
                .insert(st.name.to_ascii_lowercase());
        }
    }
    for f in &input.functions {
        if !f.in_package.is_empty() {
            let lib = package_library(input, &f.in_package, &f.file);
            let key = format!("{}.{}", lib, f.in_package.to_ascii_lowercase());
            user_pkg_exports
                .entry(key)
                .or_default()
                .insert(f.name.to_ascii_lowercase());
        }
    }
    for p in &input.procedures {
        if !p.in_package.is_empty() {
            let lib = package_library(input, &p.in_package, &p.file);
            let key = format!("{}.{}", lib, p.in_package.to_ascii_lowercase());
            user_pkg_exports
                .entry(key)
                .or_default()
                .insert(p.name.to_ascii_lowercase());
        }
    }
    for cd in &input.constant_decls {
        if !cd.in_package.is_empty() {
            let lib = package_library(input, &cd.in_package, &cd.file);
            let key = format!("{}.{}", lib, cd.in_package.to_ascii_lowercase());
            user_pkg_exports
                .entry(key)
                .or_default()
                .insert(cd.name.to_ascii_lowercase());
        }
    }

    for uc in &input.use_clauses {
        if helpers::file_in_testbench(input, &uc.file) {
            continue;
        }
        for item in &uc.items {
            let lower = item.to_ascii_lowercase();
            if lower == "std.standard.all" {
                continue;
            }
            // Parse: <library>.<package>[.<selector>]
            let parts: Vec<&str> = lower.split('.').collect();
            if parts.len() < 2 {
                continue;
            }
            let pkg_key = format!("{}.{}", parts[0], parts[1]);
            let selector = if parts.len() >= 3 { parts[2] } else { "" };

            // Get exports for this package
            let exports: Vec<String> = if let Some(std_exports) = std_package_exports(&pkg_key) {
                std_exports.iter().map(|s| s.to_string()).collect()
            } else if let Some(user_exports) = user_pkg_exports.get(&pkg_key) {
                user_exports.iter().cloned().collect()
            } else {
                // Unknown package — skip to avoid false positives
                continue;
            };

            if exports.is_empty() {
                continue;
            }

            let used_names = file_names_cache
                .entry(uc.file.clone())
                .or_insert_with(|| collect_used_names(input, &uc.file));

            let is_used = if selector == "all" {
                exports.iter().any(|exp| used_names.contains(exp))
            } else if !selector.is_empty() {
                used_names.contains(selector)
            } else {
                // No selector — can't determine usage
                continue;
            };

            if !is_used {
                out.push(Violation {
                    rule: "unused_use_clause".to_string(),
                    severity: "info".to_string(),
                    file: uc.file.clone(),
                    line: uc.line,
                    message: format!(
                        "Use clause '{}' imports package whose exports are never referenced",
                        item
                    ),
                });
            }
        }
    }
    out
}

fn package_library(input: &Input, pkg_name: &str, pkg_file: &str) -> String {
    // Check file_info for the library
    for fi in &input.files {
        if fi.path == pkg_file && !fi.library.is_empty() {
            return fi.library.to_ascii_lowercase();
        }
    }
    // Default: if it's in the same project, it's "work"
    let _ = input;
    let _ = pkg_name;
    "work".to_string()
}

fn extract_filename(path: &str) -> String {
    let file = path.split('/').last().unwrap_or(path);
    file.trim_end_matches(".vhdl")
        .trim_end_matches(".vhd")
        .to_string()
}

fn is_loop_variable(name: &str) -> bool {
    matches!(
        name.to_ascii_lowercase().as_str(),
        "i" | "j" | "k" | "n" | "x" | "y"
    )
}

fn is_versioned_name(name: &str) -> bool {
    let lower = name.to_ascii_lowercase();
    Regex::new(r".*_v[0-9]+$").unwrap().is_match(&lower)
        || Regex::new(r".*_rev[0-9]+$").unwrap().is_match(&lower)
}

fn has_direction_alternation(ports: &[Port]) -> bool {
    if ports.len() <= 2 {
        return false;
    }
    for window in ports.windows(3) {
        let p1 = &window[0].direction;
        let p2 = &window[1].direction;
        let p3 = &window[2].direction;
        if p1 == "in" && p2 == "out" && p3 == "in" {
            return true;
        }
        if p1 == "out" && p2 == "in" && p3 == "out" {
            return true;
        }
    }
    false
}

fn entity_file(input: &Input, port: &Port) -> Option<String> {
    input
        .entities
        .iter()
        .find(|entity| entity.name.eq_ignore_ascii_case(&port.in_entity))
        .map(|entity| entity.file.clone())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::policy::input::{Entity, GenerateStatement, Input, LibraryClause, Port, Process, Signal, UseClause};

    #[test]
    fn very_long_file_flags() {
        let mut input = Input::default();
        for i in 0..6 {
            input.entities.push(Entity {
                name: format!("e{}", i),
                file: "a.vhd".to_string(),
                line: i + 1,
                ..Default::default()
            });
        }
        let violations = very_long_file(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "very_long_file");
    }

    #[test]
    fn duplicate_signal_in_entity_flags() {
        let mut input = Input::default();
        input.signals.push(Signal {
            name: "sig".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            in_entity: "core".to_string(),
            ..Default::default()
        });
        input.signals.push(Signal {
            name: "sig".to_string(),
            file: "a.vhd".to_string(),
            line: 2,
            in_entity: "core".to_string(),
            ..Default::default()
        });
        let violations = duplicate_signal_in_entity(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "duplicate_signal_in_entity");
    }

    #[test]
    fn duplicate_port_in_entity_flags() {
        let mut input = Input::default();
        input.ports.push(Port {
            name: "p".to_string(),
            line: 1,
            in_entity: "core".to_string(),
            ..Default::default()
        });
        input.ports.push(Port {
            name: "p".to_string(),
            line: 2,
            in_entity: "core".to_string(),
            ..Default::default()
        });
        let violations = duplicate_port_in_entity(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "duplicate_port_in_entity");
    }

    #[test]
    fn duplicate_entity_in_file_flags() {
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "core".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        input.entities.push(Entity {
            name: "core".to_string(),
            file: "a.vhd".to_string(),
            line: 2,
            ..Default::default()
        });
        let violations = duplicate_entity_in_file(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "duplicate_entity_in_file");
    }

    #[test]
    fn buffer_port_flags() {
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "core".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        input.ports.push(Port {
            name: "p".to_string(),
            direction: "buffer".to_string(),
            in_entity: "core".to_string(),
            line: 2,
            ..Default::default()
        });
        let violations = buffer_port(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "buffer_port");
    }

    #[test]
    fn unlabeled_generate_flags() {
        let mut input = Input::default();
        input.generates.push(GenerateStatement {
            label: "".to_string(),
            file: "a.vhd".to_string(),
            line: 3,
            ..Default::default()
        });
        let violations = unlabeled_generate(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "unlabeled_generate");
    }

    #[test]
    fn duplicate_use_clause_flags() {
        let mut input = Input::default();
        input.use_clauses.push(UseClause {
            items: vec!["ieee.std_logic_1164.all".to_string()],
            file: "a.vhd".to_string(),
            line: 1,
        });
        input.use_clauses.push(UseClause {
            items: vec!["ieee.std_logic_1164.all".to_string()],
            file: "a.vhd".to_string(),
            line: 5,
        });
        let v = duplicate_use_clause(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "duplicate_use_clause");
    }

    #[test]
    fn duplicate_use_clause_different_files() {
        let mut input = Input::default();
        input.use_clauses.push(UseClause {
            items: vec!["ieee.std_logic_1164.all".to_string()],
            file: "a.vhd".to_string(),
            line: 1,
        });
        input.use_clauses.push(UseClause {
            items: vec!["ieee.std_logic_1164.all".to_string()],
            file: "b.vhd".to_string(),
            line: 1,
        });
        let v = duplicate_use_clause(&input);
        assert!(v.is_empty());
    }

    #[test]
    fn use_all_abuse_flags_non_standard() {
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "core".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        input.use_clauses.push(UseClause {
            items: vec!["work.my_pkg.all".to_string()],
            file: "a.vhd".to_string(),
            line: 2,
        });
        let v = use_all_abuse(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "use_all_abuse");
    }

    #[test]
    fn use_all_abuse_skip_standard() {
        let mut input = Input::default();
        input.use_clauses.push(UseClause {
            items: vec!["ieee.std_logic_1164.all".to_string()],
            file: "a.vhd".to_string(),
            line: 1,
        });
        let v = use_all_abuse(&input);
        assert!(v.is_empty());
    }

    #[test]
    fn signal_fanout_flags() {
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "ent".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        input.signals.push(Signal {
            name: "hot_sig".to_string(),
            file: "a.vhd".to_string(),
            line: 2,
            ..Default::default()
        });
        for i in 0..12 {
            input.processes.push(Process {
                label: format!("p{}", i),
                read_signals: vec!["hot_sig".to_string()],
                file: "a.vhd".to_string(),
                line: 10 + i,
                ..Default::default()
            });
        }
        let v = signal_fanout(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "signal_fanout");
    }

    #[test]
    fn signal_fanout_skip_low() {
        let mut input = Input::default();
        input.signals.push(Signal {
            name: "normal".to_string(),
            file: "a.vhd".to_string(),
            line: 2,
            ..Default::default()
        });
        for i in 0..3 {
            input.processes.push(Process {
                label: format!("p{}", i),
                read_signals: vec!["normal".to_string()],
                file: "a.vhd".to_string(),
                line: 10 + i,
                ..Default::default()
            });
        }
        let v = signal_fanout(&input);
        assert!(v.is_empty());
    }

    #[test]
    fn unused_library_clause_flags() {
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "core".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        input.library_clauses.push(LibraryClause {
            libraries: vec!["unisim".to_string()],
            file: "a.vhd".to_string(),
            line: 1,
        });
        let v = unused_library_clause(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "unused_library_clause");
    }

    #[test]
    fn unused_library_clause_skip_std() {
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "core".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        input.library_clauses.push(LibraryClause {
            libraries: vec!["ieee".to_string()],
            file: "a.vhd".to_string(),
            line: 1,
        });
        let v = unused_library_clause(&input);
        assert!(v.is_empty());
    }

    #[test]
    fn unused_library_clause_skip_when_used() {
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "core".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        input.library_clauses.push(LibraryClause {
            libraries: vec!["unisim".to_string()],
            file: "a.vhd".to_string(),
            line: 1,
        });
        input.use_clauses.push(UseClause {
            items: vec!["unisim.vcomponents.all".to_string()],
            file: "a.vhd".to_string(),
            line: 2,
        });
        let v = unused_library_clause(&input);
        assert!(v.is_empty());
    }

    #[test]
    fn unused_use_clause_flags() {
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "core".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        input.use_clauses.push(UseClause {
            items: vec!["ieee.math_real.all".to_string()],
            file: "a.vhd".to_string(),
            line: 2,
        });
        // No math_real symbols used
        input.signals.push(Signal {
            name: "data".to_string(),
            r#type: "std_logic".to_string(),
            file: "a.vhd".to_string(),
            line: 3,
            ..Default::default()
        });
        let v = unused_use_clause(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "unused_use_clause");
    }

    #[test]
    fn unused_use_clause_skip_used() {
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "core".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        input.use_clauses.push(UseClause {
            items: vec!["ieee.std_logic_1164.all".to_string()],
            file: "a.vhd".to_string(),
            line: 2,
        });
        input.signals.push(Signal {
            name: "data".to_string(),
            r#type: "std_logic".to_string(),
            file: "a.vhd".to_string(),
            line: 3,
            ..Default::default()
        });
        let v = unused_use_clause(&input);
        assert!(v.is_empty());
    }

    #[test]
    fn unused_use_clause_skip_unknown_pkg() {
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "core".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        input.use_clauses.push(UseClause {
            items: vec!["work.my_pkg.all".to_string()],
            file: "a.vhd".to_string(),
            line: 2,
        });
        let v = unused_use_clause(&input);
        assert!(v.is_empty());
    }
}
