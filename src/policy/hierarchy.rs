use regex::Regex;

use crate::policy::helpers;
use crate::policy::input::{Association, Entity, Input, Instance};
use crate::policy::result::Violation;

pub fn violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(floating_instance_input(input));
    out.extend(port_width_mismatch(input));
    out
}

pub fn optional_violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(sparse_port_map(input));
    out.extend(empty_port_map(input));
    out.extend(instance_name_matches_component(input));
    out.extend(repeated_component_instantiation(input));
    out.extend(many_instances(input));
    out.extend(hardcoded_port_value(input));
    out.extend(open_port_connection(input));
    out
}

fn sparse_port_map(input: &Input) -> Vec<Violation> {
    input
        .instances
        .iter()
        .filter(|inst| !inst.port_map.is_empty() && inst.port_map.len() < 3)
        .map(|inst| Violation {
            rule: "sparse_port_map".to_string(),
            severity: "info".to_string(),
            file: inst.file.clone(),
            line: inst.line,
            message: format!(
                "Instance '{}' has only {} port connections - verify all required ports are connected",
                inst.name,
                inst.port_map.len()
            ),
        })
        .collect()
}

fn empty_port_map(input: &Input) -> Vec<Violation> {
    input
        .instances
        .iter()
        .filter(|inst| inst.port_map.is_empty())
        .map(|inst| Violation {
            rule: "empty_port_map".to_string(),
            severity: "warning".to_string(),
            file: inst.file.clone(),
            line: inst.line,
            message: format!(
                "Instance '{}' has no named port map - using positional mapping or no connections",
                inst.name
            ),
        })
        .collect()
}

fn instance_name_matches_component(input: &Input) -> Vec<Violation> {
    input
        .instances
        .iter()
        .filter_map(|inst| {
            let comp_name = inst
                .target
                .split('.')
                .last()
                .unwrap_or(inst.target.as_str());
            if inst.name.eq_ignore_ascii_case(comp_name) {
                Some(Violation {
                    rule: "instance_name_matches_component".to_string(),
                    severity: "info".to_string(),
                    file: inst.file.clone(),
                    line: inst.line,
                    message: format!(
                        "Instance name '{}' matches component name - consider a unique instance name",
                        inst.name
                    ),
                })
            } else {
                None
            }
        })
        .collect()
}

fn repeated_component_instantiation(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    let mut counts = std::collections::HashMap::new();
    let mut first_instance = std::collections::HashMap::new();
    for (idx, inst) in input.instances.iter().enumerate() {
        if inst.target.is_empty() {
            continue;
        }
        let key = format!("{}|{}", inst.file, inst.target.to_ascii_lowercase());
        let count = counts.entry(key.clone()).or_insert(0usize);
        *count += 1;
        first_instance.entry(key).or_insert(idx);
    }
    for (key, count) in counts {
        if count <= 5 {
            continue;
        }
        if let Some(first_idx) = first_instance.get(&key) {
            if let Some(inst) = input.instances.get(*first_idx) {
                out.push(Violation {
                    rule: "repeated_component_instantiation".to_string(),
                    severity: "info".to_string(),
                    file: inst.file.clone(),
                    line: inst.line,
                    message: format!(
                        "Component '{}' instantiated {} times - consider generate statement or hierarchical design",
                        inst.target, count
                    ),
                });
            }
        }
    }
    out
}

fn many_instances(input: &Input) -> Vec<Violation> {
    input
        .architectures
        .iter()
        .filter_map(|arch| {
            let count = input
                .instances
                .iter()
                .filter(|inst| inst.in_arch == arch.name && inst.file == arch.file)
                .count();
            if count > 20 {
                Some(Violation {
                    rule: "many_instances".to_string(),
                    severity: "info".to_string(),
                    file: arch.file.clone(),
                    line: arch.line,
                    message: format!(
                        "Architecture '{}' has {} instances - consider hierarchical decomposition",
                        arch.name, count
                    ),
                })
            } else {
                None
            }
        })
        .collect()
}

fn hardcoded_port_value(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for inst in &input.instances {
        for (port_name, formal) in &inst.port_map {
            if is_literal_value(formal) {
                out.push(Violation {
                    rule: "hardcoded_port_value".to_string(),
                    severity: "info".to_string(),
                    file: inst.file.clone(),
                    line: inst.line,
                    message: format!(
                        "Instance '{}' has hardcoded value '{}' on port '{}' - consider using a constant/signal",
                        inst.name, formal, port_name
                    ),
                });
            }
        }
    }
    out
}

fn is_literal_value(val: &str) -> bool {
    val.starts_with('\'')
        || val.starts_with('"')
        || Regex::new(r"^[0-9]+$").unwrap().is_match(val)
        || val.eq_ignore_ascii_case("open")
}

fn open_port_connection(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for inst in &input.instances {
        for (port_name, formal) in &inst.port_map {
            if formal.eq_ignore_ascii_case("open") {
                out.push(Violation {
                    rule: "open_port_connection".to_string(),
                    severity: "info".to_string(),
                    file: inst.file.clone(),
                    line: inst.line,
                    message: format!(
                        "Instance '{}' has 'open' connection on port '{}'",
                        inst.name, port_name
                    ),
                });
            }
        }
    }
    out
}

fn floating_instance_input(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for inst in &input.instances {
        if helpers::file_in_testbench(input, &inst.file) {
            continue;
        }
        let target_lower = inst.target.to_ascii_lowercase();
        for entity in &input.entities {
            if !target_matches_entity(&target_lower, &entity.name.to_ascii_lowercase()) {
                continue;
            }
            for port in &entity.ports {
                if port.direction != "in" {
                    continue;
                }
                if !port.default.trim().is_empty() {
                    continue;
                }
                if port_connected_in_instance(inst, &port.name) {
                    continue;
                }
                if helpers::is_clock_name(&port.name) || helpers::is_reset_name(&port.name) {
                    continue;
                }
                out.push(Violation {
                    rule: "floating_instance_input".to_string(),
                    severity: "error".to_string(),
                    file: inst.file.clone(),
                    line: inst.line,
                    message: format!(
                        "Instance '{}' has unconnected input port '{}' from entity '{}'",
                        inst.name, port.name, entity.name
                    ),
                });
            }
        }
    }
    out
}

fn target_matches_entity(target: &str, entity_name: &str) -> bool {
    target == entity_name || target.ends_with(&format!(".{}", entity_name))
}

fn port_connected_in_instance(inst: &Instance, port_name: &str) -> bool {
    inst.port_map
        .keys()
        .any(|key| key.eq_ignore_ascii_case(port_name))
}

fn port_width_mismatch(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for inst in &input.instances {
        let target_lower = inst.target.to_ascii_lowercase();
        for entity in &input.entities {
            if !target_matches_entity(&target_lower, &entity.name.to_ascii_lowercase()) {
                continue;
            }
            for port in &entity.ports {
                if port.width == 0 {
                    continue;
                }
                let actual_signal = get_port_connection(inst, entity, &port.name);
                if actual_signal.is_empty() || actual_signal.eq_ignore_ascii_case("open") {
                    continue;
                }
                let signal_width = get_actual_width(input, &actual_signal, &inst.in_arch);
                if signal_width == 0 {
                    continue;
                }
                if signal_width != port.width {
                    out.push(Violation {
                        rule: "port_width_mismatch".to_string(),
                        severity: "error".to_string(),
                        file: inst.file.clone(),
                        line: inst.line,
                        message: format!(
                            "Width mismatch: signal '{}' ({} bits) connected to port '{}' ({} bits) in instance '{}'",
                            actual_signal, signal_width, port.name, port.width, inst.name
                        ),
                    });
                }
            }
        }
    }
    out
}

fn get_port_connection(inst: &Instance, entity: &Entity, port_name: &str) -> String {
    // Prefer association elements (captures slices/indexing)
    for assoc in &inst.associations {
        if assoc.kind != "port" || assoc.is_positional {
            continue;
        }
        if assoc.formal.eq_ignore_ascii_case(port_name) {
            return association_actual(assoc);
        }
    }

    // Positional associations: map by entity port order
    if let Some(pos) = entity
        .ports
        .iter()
        .position(|p| p.name.eq_ignore_ascii_case(port_name))
    {
        for assoc in &inst.associations {
            if assoc.kind == "port" && assoc.is_positional && assoc.position_index == pos {
                return association_actual(assoc);
            }
        }
    }

    // Fallback to port map strings
    if let Some(actual) = inst.port_map.get(port_name) {
        return actual.clone();
    }
    for (key, value) in &inst.port_map {
        if key.eq_ignore_ascii_case(port_name) {
            return value.clone();
        }
    }
    String::new()
}

fn is_literal_or_expr(s: &str) -> bool {
    Regex::new(r"^[0-9]").unwrap().is_match(s)
        || s.contains('+')
        || s.contains('-')
        || s.contains('*')
        || s.contains('&')
        || Regex::new(r#"^[xXbBoO]""#).unwrap().is_match(s)
        || Regex::new(r"^'.'$").unwrap().is_match(s)
        || s.to_ascii_lowercase().contains("others")
}

fn get_actual_width(input: &Input, actual: &str, scope_arch: &str) -> usize {
    if actual.is_empty() || actual.eq_ignore_ascii_case("open") {
        return 0;
    }
    if is_literal_or_expr(actual) {
        return 0;
    }
    let base = base_name(actual);
    let mut base_width = get_signal_width(input, base, scope_arch);
    if base_width == 0 && base != actual {
        base_width = get_signal_width(input, actual, scope_arch);
    }

    if let Some(width) = indexed_width(actual, base_width) {
        return width;
    }

    base_width
}

fn get_signal_width(input: &Input, signal_name: &str, scope_arch: &str) -> usize {
    let mut widths = Vec::new();
    if !scope_arch.is_empty() {
        for sig in &input.signals {
            if sig.in_entity.eq_ignore_ascii_case(scope_arch)
                && sig.name.eq_ignore_ascii_case(signal_name)
            {
                widths.push(sig.width);
            }
        }
        if let Some(entity_name) = arch_entity_name(input, scope_arch) {
            for port in &input.ports {
                if port.in_entity.eq_ignore_ascii_case(&entity_name)
                    && port.name.eq_ignore_ascii_case(signal_name)
                {
                    widths.push(port.width);
                }
            }
        }
        if !widths.is_empty() {
            return widths.into_iter().max().unwrap_or(0);
        }
        return 0;
    }

    for sig in &input.signals {
        if sig.name.eq_ignore_ascii_case(signal_name) {
            widths.push(sig.width);
        }
    }
    for port in &input.ports {
        if port.name.eq_ignore_ascii_case(signal_name) {
            widths.push(port.width);
        }
    }
    widths.into_iter().max().unwrap_or(0)
}

fn association_actual(assoc: &Association) -> String {
    if !assoc.actual.is_empty() {
        if !assoc.actual_full.is_empty()
            && assoc.actual_full.contains('(')
            && !assoc.actual.contains('(')
        {
            return assoc.actual_full.clone();
        }
        return assoc.actual.clone();
    }
    if !assoc.actual_full.is_empty() {
        return assoc.actual_full.clone();
    }
    assoc.actual_base.clone()
}

fn base_name(actual: &str) -> &str {
    let mut end = actual.len();
    for (idx, ch) in actual.char_indices() {
        if ch == '(' || ch == '.' || ch == '\'' || ch.is_whitespace() {
            end = idx;
            break;
        }
    }
    actual[..end].trim()
}

fn arch_entity_name(input: &Input, arch_name: &str) -> Option<String> {
    input
        .architectures
        .iter()
        .find(|arch| arch.name.eq_ignore_ascii_case(arch_name))
        .map(|arch| arch.entity_name.clone())
}

fn indexed_width(actual: &str, base_width: usize) -> Option<usize> {
    let start = actual.find('(')?;
    let end = actual.rfind(')')?;
    if end <= start {
        return None;
    }
    let inside = actual[start + 1..end].trim();
    if inside.contains(',') {
        return Some(0);
    }
    let lower = inside.to_ascii_lowercase();
    if let Some(idx) = lower.find(" downto ") {
        let (left, right) = inside.split_at(idx);
        let right = &right[" downto ".len()..];
        let a = left.trim().parse::<isize>().ok()?;
        let b = right.trim().parse::<isize>().ok()?;
        return Some((a - b).abs() as usize + 1);
    }
    if let Some(idx) = lower.find(" to ") {
        let (left, right) = inside.split_at(idx);
        let right = &right[" to ".len()..];
        let a = left.trim().parse::<isize>().ok()?;
        let b = right.trim().parse::<isize>().ok()?;
        return Some((a - b).abs() as usize + 1);
    }
    // Single index: if base width is unknown, avoid guessing.
    if base_width == 0 {
        return Some(0);
    }
    Some(1)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::policy::input::{Association, Entity, Input, Instance, Port, Signal};

    #[test]
    fn sparse_port_map_flags() {
        let mut input = Input::default();
        let mut inst = Instance::default();
        inst.name = "u1".to_string();
        inst.file = "a.vhd".to_string();
        inst.port_map.insert("a".to_string(), "sig".to_string());
        input.instances.push(inst);
        let v = sparse_port_map(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "sparse_port_map");
    }

    #[test]
    fn floating_instance_input_flags() {
        let mut input = Input::default();
        let mut inst = Instance::default();
        inst.name = "u1".to_string();
        inst.target = "work.ent".to_string();
        inst.file = "a.vhd".to_string();
        input.instances.push(inst);
        let mut entity = Entity::default();
        entity.name = "ent".to_string();
        entity.ports.push(Port {
            name: "data_in".to_string(),
            direction: "in".to_string(),
            ..Default::default()
        });
        input.entities.push(entity);
        let v = floating_instance_input(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "floating_instance_input");
    }

    #[test]
    fn floating_instance_input_ignores_defaulted_port() {
        let mut input = Input::default();
        let mut inst = Instance::default();
        inst.name = "u1".to_string();
        inst.target = "work.ent".to_string();
        inst.file = "a.vhd".to_string();
        input.instances.push(inst);
        let mut entity = Entity::default();
        entity.name = "ent".to_string();
        entity.ports.push(Port {
            name: "cfg_i".to_string(),
            direction: "in".to_string(),
            default: "'0'".to_string(),
            ..Default::default()
        });
        input.entities.push(entity);
        let v = floating_instance_input(&input);
        assert!(v.is_empty());
    }

    #[test]
    fn port_width_mismatch_ignores_sliced_actual() {
        let mut input = Input::default();
        let mut entity = Entity::default();
        entity.name = "child".to_string();
        entity.ports.push(Port {
            name: "shamt_i".to_string(),
            direction: "in".to_string(),
            width: 5,
            ..Default::default()
        });
        input.entities.push(entity);

        let mut inst = Instance::default();
        inst.name = "u1".to_string();
        inst.target = "work.child".to_string();
        inst.file = "a.vhd".to_string();
        inst.line = 1;
        inst.port_map
            .insert("shamt_i".to_string(), "opb".to_string());
        inst.associations.push(Association {
            kind: "port".to_string(),
            formal: "shamt_i".to_string(),
            actual: "opb(4 downto 0)".to_string(),
            ..Default::default()
        });
        input.instances.push(inst);

        input.signals.push(Signal {
            name: "opb".to_string(),
            width: 32,
            ..Default::default()
        });

        let v = port_width_mismatch(&input);
        assert!(v.is_empty());
    }

    #[test]
    fn port_width_mismatch_skips_unknown_index_width() {
        let mut input = Input::default();
        let mut entity = Entity::default();
        entity.name = "child".to_string();
        entity.ports.push(Port {
            name: "res_o".to_string(),
            direction: "out".to_string(),
            width: 32,
            ..Default::default()
        });
        input.entities.push(entity);

        let mut inst = Instance::default();
        inst.name = "u1".to_string();
        inst.target = "work.child".to_string();
        inst.file = "a.vhd".to_string();
        inst.line = 1;
        inst.associations.push(Association {
            kind: "port".to_string(),
            formal: "res_o".to_string(),
            actual: "cp_result(0)".to_string(),
            ..Default::default()
        });
        input.instances.push(inst);

        input.signals.push(Signal {
            name: "cp_result".to_string(),
            width: 0,
            ..Default::default()
        });

        let v = port_width_mismatch(&input);
        assert!(v.is_empty());
    }
}
