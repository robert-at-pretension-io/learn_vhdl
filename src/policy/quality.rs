use regex::Regex;

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
    use crate::policy::input::{Entity, GenerateStatement, Input, Port, Signal};

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
}
