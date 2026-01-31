use crate::policy::helpers;
use crate::policy::input::{Input, Process, VerificationTag, VerificationTagError};
use crate::policy::result::{AmbiguousConstruct, MissingCheckTask, VerificationAnchor, Violation};
use serde::Deserialize;
use std::collections::{HashMap, HashSet};
use std::env;
use std::fs;

#[derive(Debug, Clone, Deserialize)]
struct CheckEntry {
    id: String,
    #[serde(default)]
    scope_type: String,
    #[serde(default)]
    required_bindings: Vec<String>,
    #[serde(default)]
    needs_cover: bool,
    #[serde(default)]
    severity: String,
    #[serde(default)]
    requires_bound: bool,
}

#[derive(Debug, Clone, PartialEq, Eq, Hash)]
enum ConstructKind {
    Fsm,
    Counter,
    ReadyValid,
    Fifo,
}

impl ConstructKind {
    fn label(&self) -> &'static str {
        match self {
            ConstructKind::Fsm => "fsm",
            ConstructKind::Counter => "counter",
            ConstructKind::ReadyValid => "ready_valid",
            ConstructKind::Fifo => "fifo",
        }
    }
}

#[derive(Debug, Clone)]
struct Construct {
    kind: ConstructKind,
    in_arch: String,
    file: String,
    line: usize,
    bindings: HashMap<String, String>,
}

struct DetectionReport {
    constructs: Vec<Construct>,
    ambiguous: Vec<AmbiguousConstruct>,
}

pub struct VerificationAnalysis {
    pub violations: Vec<Violation>,
    pub missing_checks: Vec<MissingCheckTask>,
    pub ambiguous_constructs: Vec<AmbiguousConstruct>,
}

pub fn analyze(input: &Input) -> VerificationAnalysis {
    let registry = registry_by_id();
    let tags_by_scope = tags_by_scope(input, &registry);
    let detection = detect_constructs(input);
    let mut violations = Vec::new();
    violations.extend(invalid_tag_violations(input, &registry));
    violations.extend(missing_liveness_bound(input, &registry));
    violations.extend(missing_cover_companion(input, &registry, &tags_by_scope));
    violations.extend(missing_verification_block(input, &detection.constructs));
    violations.extend(missing_check_violations(
        input,
        &detection.constructs,
        &tags_by_scope,
        &registry,
    ));
    violations.extend(ambiguous_construct_warnings(&detection.ambiguous));

    let missing_checks = missing_check_tasks(
        input,
        &detection.constructs,
        &tags_by_scope,
        &registry,
    );

    VerificationAnalysis {
        violations,
        missing_checks,
        ambiguous_constructs: detection.ambiguous,
    }
}

fn load_registry() -> Vec<CheckEntry> {
    let payload = if let Ok(path) = env::var("VHDL_CHECK_REGISTRY") {
        fs::read_to_string(&path).unwrap_or_else(|err| {
            panic!("failed to read VHDL_CHECK_REGISTRY {}: {}", path, err)
        })
    } else {
        include_str!("check_registry.json").to_string()
    };

    serde_json::from_str(&payload).unwrap_or_else(|err| {
        panic!("failed to parse verification check registry: {}", err)
    })
}

fn registry_by_id() -> HashMap<String, CheckEntry> {
    let mut map = HashMap::new();
    for mut entry in load_registry() {
        entry.id = entry.id.to_ascii_lowercase();
        entry.scope_type = entry.scope_type.to_ascii_lowercase();
        map.insert(entry.id.clone(), entry);
    }
    map
}

fn invalid_tag_violations(
    input: &Input,
    registry: &HashMap<String, CheckEntry>,
) -> Vec<Violation> {
    let mut out = Vec::new();
    for err in &input.verification_tag_errors {
        out.push(tag_error_violation(err));
    }
    for tag in &input.verification_tags {
        let entry = match registry.get(&tag.id.to_ascii_lowercase()) {
            Some(entry) => entry,
            None => {
                out.push(tag_violation(
                    tag,
                    format!("Unknown verification check id '{}'", tag.id),
                ));
                continue;
            }
        };

        if !scope_matches(&entry.scope_type, &tag.scope) {
            out.push(tag_violation(
                tag,
                format!(
                    "Verification tag '{}' has scope '{}' but registry expects '{}:*'",
                    tag.id, tag.scope, entry.scope_type
                ),
            ));
        }

        let missing = missing_required_bindings(entry, tag);
        if !missing.is_empty() {
            out.push(tag_violation(
                tag,
                format!(
                    "Verification tag '{}' missing required bindings: {}",
                    tag.id,
                    missing.join(", ")
                ),
            ));
        }
    }
    out
}

fn missing_liveness_bound(
    input: &Input,
    registry: &HashMap<String, CheckEntry>,
) -> Vec<Violation> {
    let mut out = Vec::new();
    for tag in &input.verification_tags {
        let entry = match registry.get(&tag.id.to_ascii_lowercase()) {
            Some(entry) => entry,
            None => continue,
        };
        if !entry.requires_bound {
            continue;
        }
        if tag.bindings.get("bound").map(|v| v.trim()).unwrap_or("").is_empty() {
            out.push(Violation {
                rule: "missing_liveness_bound".to_string(),
                severity: "error".to_string(),
                file: tag.file.clone(),
                line: tag.line,
                message: format!(
                    "Verification tag '{}' requires an explicit bound (add bound=)",
                    tag.id
                ),
            });
        }
    }
    out
}

fn missing_cover_companion(
    input: &Input,
    registry: &HashMap<String, CheckEntry>,
    tags_by_scope: &HashMap<String, Vec<&VerificationTag>>,
) -> Vec<Violation> {
    let mut out = Vec::new();
    for tag in &input.verification_tags {
        let entry = match registry.get(&tag.id.to_ascii_lowercase()) {
            Some(entry) => entry,
            None => continue,
        };
        if !entry.needs_cover {
            continue;
        }
        if !tag_is_valid(tag, registry) {
            continue;
        }
        let scope_key = match tag_scope_key(input, tag) {
            Some(scope) => scope,
            None => continue,
        };
        let prefix = match cover_prefix_for(&tag.id) {
            Some(prefix) => prefix,
            None => continue,
        };
        let has_cover = tags_by_scope
            .get(&scope_key)
            .map(|tags| {
                tags.iter().any(|other| {
                    other.id.to_ascii_lowercase().starts_with(&prefix)
                        && tag_is_valid(other, registry)
                })
            })
            .unwrap_or(false);
        if !has_cover {
            out.push(Violation {
                rule: "missing_cover_companion".to_string(),
                severity: "warning".to_string(),
                file: tag.file.clone(),
                line: tag.line,
                message: format!(
                    "Verification tag '{}' requires a cover companion in {}",
                    tag.id, scope_key
                ),
            });
        }
    }
    out
}

fn missing_verification_block(input: &Input, constructs: &[Construct]) -> Vec<Violation> {
    let mut out = Vec::new();
    let mut arches_with_block = HashSet::new();
    for block in &input.verification_blocks {
        arches_with_block.insert(block.in_arch.to_ascii_lowercase());
    }

    let mut arches_with_constructs = HashSet::new();
    for construct in constructs {
        arches_with_constructs.insert(construct.in_arch.to_ascii_lowercase());
    }

    for arch in &input.architectures {
        if !arches_with_constructs.contains(&arch.name.to_ascii_lowercase()) {
            continue;
        }
        if arches_with_block.contains(&arch.name.to_ascii_lowercase()) {
            continue;
        }
        out.push(Violation {
            rule: "missing_verification_block".to_string(),
            severity: "warning".to_string(),
            file: arch.file.clone(),
            line: arch.line,
            message: format!(
                "Architecture '{}' has detectable constructs but no verification block",
                arch.name
            ),
        });
    }
    out
}

fn missing_check_violations(
    input: &Input,
    constructs: &[Construct],
    tags_by_scope: &HashMap<String, Vec<&VerificationTag>>,
    registry: &HashMap<String, CheckEntry>,
) -> Vec<Violation> {
    let mut out = Vec::new();
    let mut emitted = HashSet::new();

    for construct in constructs {
        let scope_key = format!("arch:{}", construct.in_arch.to_ascii_lowercase());
        let tags = tags_by_scope.get(&scope_key);
        let tag_ids: HashSet<String> = tags
            .map(|list| {
                list.iter()
                    .filter(|tag| tag_is_valid(tag, registry))
                    .map(|tag| tag.id.to_ascii_lowercase())
                    .collect()
            })
            .unwrap_or_default();
        for check_id in required_checks_for_construct(&construct.kind) {
            let check_id_lower = check_id.to_ascii_lowercase();
            if tag_ids.contains(&check_id_lower) {
                continue;
            }
            let key = format!("{}::{}", scope_key, check_id_lower);
            if emitted.contains(&key) {
                continue;
            }
            emitted.insert(key);
            let (severity, msg) =
                missing_check_details(input, construct, &scope_key, check_id, registry);
            out.push(Violation {
                rule: "missing_verification_check".to_string(),
                severity,
                file: construct.file.clone(),
                line: construct.line,
                message: msg,
            });
        }
    }
    out
}

fn missing_check_tasks(
    input: &Input,
    constructs: &[Construct],
    tags_by_scope: &HashMap<String, Vec<&VerificationTag>>,
    registry: &HashMap<String, CheckEntry>,
) -> Vec<MissingCheckTask> {
    let mut tasks = Vec::new();
    let mut seen = HashSet::new();
    for construct in constructs {
        let scope_key = format!("arch:{}", construct.in_arch.to_ascii_lowercase());
        let tags = tags_by_scope.get(&scope_key);
        let tag_ids: HashSet<String> = tags
            .map(|list| {
                list.iter()
                    .filter(|tag| tag_is_valid(tag, registry))
                    .map(|tag| tag.id.to_ascii_lowercase())
                    .collect()
            })
            .unwrap_or_default();
        let mut missing_ids = Vec::new();
        for check_id in required_checks_for_construct(&construct.kind) {
            if tag_ids.contains(&check_id.to_ascii_lowercase()) {
                continue;
            }
            missing_ids.push(check_id.to_string());
        }
        if missing_ids.is_empty() {
            continue;
        }
        let key = format!(
            "{}:{}:{}",
            scope_key,
            construct.kind.label(),
            format_bindings(&construct.bindings)
        );
        if seen.contains(&key) {
            continue;
        }
        seen.insert(key);
        let notes = notes_for_missing_checks(registry, &missing_ids);
        tasks.push(MissingCheckTask {
            file: construct.file.clone(),
            scope: scope_key.clone(),
            anchor: anchor_for_arch(input, &construct.in_arch),
            missing_ids,
            bindings: construct.bindings.clone(),
            notes,
        });
    }
    tasks
}

fn missing_check_details(
    input: &Input,
    construct: &Construct,
    scope_key: &str,
    check_id: &str,
    registry: &HashMap<String, CheckEntry>,
) -> (String, String) {
    let severity = registry
        .get(&check_id.to_ascii_lowercase())
        .map(|entry| normalize_severity(&entry.severity))
        .unwrap_or_else(|| "warning".to_string());
    let anchor = anchor_line_for_arch(input, &construct.in_arch);
    let bindings = format_bindings(&construct.bindings);
    let msg = if bindings.is_empty() {
        format!(
            "Missing verification check '{}' for {} in {} (anchor line {})",
            check_id,
            construct.kind.label(),
            scope_key,
            anchor
        )
    } else {
        format!(
            "Missing verification check '{}' for {} in {} (anchor line {}, bindings: {})",
            check_id,
            construct.kind.label(),
            scope_key,
            anchor,
            bindings
        )
    };
    (severity, msg)
}

fn notes_for_missing_checks(
    registry: &HashMap<String, CheckEntry>,
    checks: &[String],
) -> Vec<String> {
    let mut notes = Vec::new();
    for check in checks {
        if let Some(entry) = registry.get(&check.to_ascii_lowercase()) {
            if entry.needs_cover {
                notes.push(format!("{} needs cover companion", check));
            }
            if entry.requires_bound {
                notes.push(format!("{} requires explicit bound", check));
            }
        }
    }
    notes
}

fn ambiguous_construct_warnings(ambiguous: &[AmbiguousConstruct]) -> Vec<Violation> {
    let mut out = Vec::new();
    for amb in ambiguous {
        let mut parts = Vec::new();
        let mut keys: Vec<&String> = amb.candidates.keys().collect();
        keys.sort();
        for key in keys {
            if let Some(values) = amb.candidates.get(key) {
                parts.push(format!("{}=[{}]", key, values.join(", ")));
            }
        }
        out.push(Violation {
            rule: "ambiguous_construct".to_string(),
            severity: "warning".to_string(),
            file: amb.file.clone(),
            line: amb.line,
            message: format!(
                "Ambiguous {} construct in {} (candidates: {})",
                amb.kind,
                amb.scope,
                parts.join("; ")
            ),
        });
    }
    out
}

fn normalize_severity(sev: &str) -> String {
    match sev {
        "error" | "warning" | "info" => sev.to_string(),
        _ => "warning".to_string(),
    }
}

fn tag_error_violation(err: &VerificationTagError) -> Violation {
    Violation {
        rule: "invalid_verification_tag".to_string(),
        severity: "error".to_string(),
        file: err.file.clone(),
        line: err.line,
        message: format!("Malformed verification tag: {}", err.message),
    }
}

fn tag_violation(tag: &VerificationTag, message: String) -> Violation {
    Violation {
        rule: "invalid_verification_tag".to_string(),
        severity: "error".to_string(),
        file: tag.file.clone(),
        line: tag.line,
        message,
    }
}

fn scope_matches(scope_type: &str, scope: &str) -> bool {
    if scope_type.is_empty() {
        return true;
    }
    let prefix = scope.split(':').next().unwrap_or("").trim();
    prefix.eq_ignore_ascii_case(scope_type)
}

fn tag_is_valid(tag: &VerificationTag, registry: &HashMap<String, CheckEntry>) -> bool {
    let entry = match registry.get(&tag.id.to_ascii_lowercase()) {
        Some(entry) => entry,
        None => return false,
    };
    if !scope_matches(&entry.scope_type, &tag.scope) {
        return false;
    }
    if !missing_required_bindings(entry, tag).is_empty() {
        return false;
    }
    if entry.requires_bound && tag.bindings.get("bound").map(|v| v.trim()).unwrap_or("").is_empty()
    {
        return false;
    }
    true
}

fn missing_required_bindings(entry: &CheckEntry, tag: &VerificationTag) -> Vec<String> {
    let mut missing = Vec::new();
    for binding in &entry.required_bindings {
        match tag.bindings.get(binding) {
            Some(value) if !value.trim().is_empty() => {}
            _ => missing.push(binding.clone()),
        }
    }
    missing
}

fn tags_by_scope<'a>(
    input: &'a Input,
    registry: &HashMap<String, CheckEntry>,
) -> HashMap<String, Vec<&'a VerificationTag>> {
    let mut map: HashMap<String, Vec<&VerificationTag>> = HashMap::new();
    for tag in &input.verification_tags {
        if !tag_is_valid(tag, registry) {
            continue;
        }
        if let Some(scope_key) = tag_scope_key(input, tag) {
            map.entry(scope_key).or_default().push(tag);
        }
    }
    map
}

fn tag_scope_key(input: &Input, tag: &VerificationTag) -> Option<String> {
    let (scope_type, scope_name) = parse_scope(&tag.scope)?;
    match scope_type.as_str() {
        "arch" => {
            if tag.in_arch.eq_ignore_ascii_case(&scope_name) {
                Some(format!("arch:{}", scope_name))
            } else {
                None
            }
        }
        "entity" => {
            let arch_to_entity = arch_entity_map(input);
            let arch = tag.in_arch.to_ascii_lowercase();
            let expected = arch_to_entity.get(&arch)?;
            if expected.eq_ignore_ascii_case(&scope_name) {
                Some(format!("entity:{}", scope_name))
            } else {
                None
            }
        }
        _ => None,
    }
}

fn arch_entity_map(input: &Input) -> HashMap<String, String> {
    let mut map = HashMap::new();
    for arch in &input.architectures {
        map.insert(
            arch.name.to_ascii_lowercase(),
            arch.entity_name.to_ascii_lowercase(),
        );
    }
    map
}

fn parse_scope(scope: &str) -> Option<(String, String)> {
    let mut parts = scope.splitn(2, ':');
    let scope_type = parts.next()?.trim().to_ascii_lowercase();
    let scope_name = parts.next()?.trim().to_ascii_lowercase();
    if scope_type.is_empty() || scope_name.is_empty() {
        None
    } else {
        Some((scope_type, scope_name))
    }
}

fn cover_prefix_for(id: &str) -> Option<String> {
    let id = id.to_ascii_lowercase();
    if id.starts_with("cover.") {
        return None;
    }
    let family = id.split('.').next()?;
    Some(format!("cover.{}.", family))
}

fn anchor_for_arch(input: &Input, arch: &str) -> VerificationAnchor {
    if let Some(block) = input
        .verification_blocks
        .iter()
        .find(|block| block.in_arch.eq_ignore_ascii_case(arch))
    {
        return VerificationAnchor {
            label: block.label.clone(),
            line_start: block.line_start,
            line_end: block.line_end,
            exists: true,
        };
    }
    if let Some(arch) = input
        .architectures
        .iter()
        .find(|a| a.name.eq_ignore_ascii_case(arch))
    {
        return VerificationAnchor {
            label: "architecture".to_string(),
            line_start: arch.line,
            line_end: arch.line,
            exists: false,
        };
    }
    VerificationAnchor {
        label: "unknown".to_string(),
        line_start: 1,
        line_end: 1,
        exists: false,
    }
}

fn anchor_line_for_arch(input: &Input, arch: &str) -> usize {
    anchor_for_arch(input, arch).line_start
}

fn format_bindings(bindings: &HashMap<String, String>) -> String {
    if bindings.is_empty() {
        return String::new();
    }
    let mut pairs: Vec<(String, String)> = bindings
        .iter()
        .map(|(k, v)| (k.clone(), v.clone()))
        .collect();
    pairs.sort_by(|a, b| a.0.cmp(&b.0));
    pairs
        .into_iter()
        .map(|(k, v)| format!("{}={}", k, v))
        .collect::<Vec<String>>()
        .join(", ")
}

fn detect_constructs(input: &Input) -> DetectionReport {
    let mut constructs = Vec::new();
    let mut ambiguous = Vec::new();
    constructs.extend(detect_fsm_constructs(input));
    constructs.extend(detect_counter_constructs(input));
    let (rv_constructs, rv_ambiguous) = detect_ready_valid_constructs(input);
    constructs.extend(rv_constructs);
    ambiguous.extend(rv_ambiguous);
    constructs.extend(detect_fifo_constructs(input));

    let mut seen = HashSet::new();
    constructs.retain(|c| {
        let key = format!(
            "{}:{}:{}",
            c.kind.label(),
            c.in_arch.to_ascii_lowercase(),
            format_bindings(&c.bindings)
        );
        if seen.contains(&key) {
            false
        } else {
            seen.insert(key);
            true
        }
    });

    DetectionReport {
        constructs,
        ambiguous,
    }
}

fn detect_fsm_constructs(input: &Input) -> Vec<Construct> {
    let enum_types = enum_type_names(input);
    let mut constructs = Vec::new();

    for cs in &input.case_statements {
        if !is_fsm_case(input, cs, &enum_types) {
            continue;
        }
        let state = cs.expression.trim().to_string();
        let mut bindings = HashMap::new();
        bindings.insert("state".to_string(), state);
        constructs.push(Construct {
            kind: ConstructKind::Fsm,
            in_arch: cs.in_arch.clone(),
            file: cs.file.clone(),
            line: cs.line,
            bindings,
        });
    }
    constructs
}

fn is_fsm_case(
    input: &Input,
    case_stmt: &crate::policy::input::CaseStatement,
    enum_types: &HashSet<String>,
) -> bool {
    let expr = case_stmt.expression.trim();
    if !is_simple_identifier(expr) {
        return false;
    }
    if !signal_is_enum(input, expr, enum_types) {
        return false;
    }
    for process in process_candidates(input, case_stmt) {
        if !process.is_sequential {
            continue;
        }
        if signal_in_list(expr, &process.assigned_signals)
            && signal_in_list(expr, &process.read_signals)
        {
            return true;
        }
    }
    false
}

fn enum_type_names(input: &Input) -> HashSet<String> {
    let mut names = HashSet::new();
    for td in &input.types {
        if td.kind.eq_ignore_ascii_case("enum") {
            names.insert(td.name.to_ascii_lowercase());
        }
    }
    for st in &input.subtypes {
        let base = helpers::base_type_name(&st.base_type);
        if names.contains(&base) {
            names.insert(st.name.to_ascii_lowercase());
        }
    }
    names
}

fn signal_is_enum(input: &Input, signal: &str, enum_types: &HashSet<String>) -> bool {
    input
        .signals
        .iter()
        .find(|sig| sig.name.eq_ignore_ascii_case(signal))
        .map(|sig| enum_types.contains(&helpers::base_type_name(&sig.r#type)))
        .unwrap_or(false)
}

fn detect_counter_constructs(input: &Input) -> Vec<Construct> {
    let mut constructs = Vec::new();
    for process in &input.processes {
        if !process.is_sequential {
            continue;
        }
        for signal in process
            .assigned_signals
            .iter()
            .filter(|sig| signal_in_list(sig, &process.read_signals))
        {
            if !signal_is_numeric(input, signal) {
                continue;
            }
            let mut bindings = HashMap::new();
            bindings.insert("counter".to_string(), signal.clone());
            constructs.push(Construct {
                kind: ConstructKind::Counter,
                in_arch: process.in_arch.clone(),
                file: process.file.clone(),
                line: process.line,
                bindings,
            });
        }
    }
    constructs
}

fn signal_is_numeric(input: &Input, signal: &str) -> bool {
    let sig = match input
        .signals
        .iter()
        .find(|sig| sig.name.eq_ignore_ascii_case(signal))
    {
        Some(sig) => sig,
        None => return false,
    };
    let base = helpers::base_type_name(&sig.r#type);
    matches!(base.as_str(), "integer" | "natural" | "positive")
        || helpers::is_unsigned_type(&sig.r#type)
        || helpers::is_signed_type(&sig.r#type)
}

fn detect_ready_valid_constructs(
    input: &Input,
) -> (Vec<Construct>, Vec<AmbiguousConstruct>) {
    let port_map = port_info_map(input);
    let mut constructs = Vec::new();
    let mut ambiguous = Vec::new();
    for ca in &input.concurrent_assignments {
        if ca.read_signals.len() != 2 {
            continue;
        }
        let a = ca.read_signals[0].clone();
        let b = ca.read_signals[1].clone();
        let pa = port_map.get(&a.to_ascii_lowercase());
        let pb = port_map.get(&b.to_ascii_lowercase());
        let (pa, pb) = match (pa, pb) {
            (Some(pa), Some(pb)) => (pa, pb),
            _ => continue,
        };
        if !pa.single_bit || !pb.single_bit {
            continue;
        }
        let (valid, ready) = match (pa.direction.as_str(), pb.direction.as_str()) {
            ("out", "in") | ("buffer", "in") => (a.clone(), b.clone()),
            ("in", "out") | ("in", "buffer") => (b.clone(), a.clone()),
            _ => {
                let mut candidates = HashMap::new();
                candidates.insert("valid".to_string(), vec![a.clone(), b.clone()]);
                candidates.insert("ready".to_string(), vec![a.clone(), b.clone()]);
                ambiguous.push(AmbiguousConstruct {
                    kind: "ready_valid".to_string(),
                    scope: format!("arch:{}", ca.in_arch.to_ascii_lowercase()),
                    file: ca.file.clone(),
                    line: ca.line,
                    candidates,
                });
                continue;
            }
        };
        let mut bindings = HashMap::new();
        bindings.insert("valid".to_string(), valid);
        bindings.insert("ready".to_string(), ready);
        constructs.push(Construct {
            kind: ConstructKind::ReadyValid,
            in_arch: ca.in_arch.clone(),
            file: ca.file.clone(),
            line: ca.line,
            bindings,
        });
    }
    (constructs, ambiguous)
}

fn detect_fifo_constructs(input: &Input) -> Vec<Construct> {
    let port_map = port_info_map(input);
    let array_signals = array_signals_by_arch(input);
    let mut constructs = Vec::new();

    for (arch, mems) in array_signals {
        for mem in mems {
            let mem_name = mem.0.clone();
            let write_procs = processes_writing_signal(input, &mem_name, &arch);
            let read_procs = processes_reading_signal(input, &mem_name, &arch);
            if write_procs.is_empty() || read_procs.is_empty() {
                continue;
            }
            let wr_en = select_control_input(input, &port_map, &write_procs);
            let rd_en = select_control_input(input, &port_map, &read_procs);
            let full = select_status_output(input, &port_map, &write_procs);
            let empty = select_status_output(input, &port_map, &read_procs);
            let (wr_en, rd_en, full, empty) = match (wr_en, rd_en, full, empty) {
                (Some(wr), Some(rd), Some(f), Some(e)) => (wr, rd, f, e),
                _ => continue,
            };
            let mut bindings = HashMap::new();
            bindings.insert("wr_en".to_string(), wr_en);
            bindings.insert("rd_en".to_string(), rd_en);
            bindings.insert("full".to_string(), full);
            bindings.insert("empty".to_string(), empty);
            constructs.push(Construct {
                kind: ConstructKind::Fifo,
                in_arch: arch.clone(),
                file: mem.1.clone(),
                line: mem.2,
                bindings,
            });
        }
    }
    constructs
}

fn processes_writing_signal(input: &Input, signal: &str, arch: &str) -> HashSet<String> {
    input
        .signal_deps
        .iter()
        .filter(|dep| dep.in_arch.eq_ignore_ascii_case(arch))
        .filter(|dep| dep.target.eq_ignore_ascii_case(signal))
        .filter(|dep| !dep.in_process.is_empty())
        .map(|dep| dep.in_process.clone())
        .collect()
}

fn processes_reading_signal(input: &Input, signal: &str, arch: &str) -> HashSet<String> {
    input
        .signal_deps
        .iter()
        .filter(|dep| dep.in_arch.eq_ignore_ascii_case(arch))
        .filter(|dep| dep.source.eq_ignore_ascii_case(signal))
        .filter(|dep| !dep.in_process.is_empty())
        .map(|dep| dep.in_process.clone())
        .collect()
}

fn select_control_input(
    input: &Input,
    port_map: &HashMap<String, PortInfo>,
    processes: &HashSet<String>,
) -> Option<String> {
    let mut candidates = HashSet::new();
    for proc in input.processes.iter().filter(|p| processes.contains(&p.label)) {
        for sig in &proc.read_signals {
            if sig.eq_ignore_ascii_case(&proc.reset_signal)
                || helpers::is_reset_name(sig)
                || helpers::is_clock_name(sig)
            {
                continue;
            }
            if let Some(info) = port_map.get(&sig.to_ascii_lowercase()) {
                if info.direction == "in" && info.single_bit {
                    candidates.insert(sig.clone());
                }
            }
        }
    }
    if candidates.len() == 1 {
        candidates.into_iter().next()
    } else {
        None
    }
}

fn select_status_output(
    input: &Input,
    port_map: &HashMap<String, PortInfo>,
    processes: &HashSet<String>,
) -> Option<String> {
    let mut candidates = HashSet::new();
    for proc in input.processes.iter().filter(|p| processes.contains(&p.label)) {
        for sig in &proc.assigned_signals {
            if let Some(info) = port_map.get(&sig.to_ascii_lowercase()) {
                if info.direction == "out" && info.single_bit {
                    candidates.insert(sig.clone());
                }
            }
        }
    }
    if candidates.len() == 1 {
        candidates.into_iter().next()
    } else {
        None
    }
}

fn array_signals_by_arch(input: &Input) -> HashMap<String, Vec<(String, String, usize)>> {
    let array_types = array_type_names(input);
    let mut map = HashMap::new();
    for sig in &input.signals {
        let base = helpers::base_type_name(&sig.r#type);
        if array_types.contains(&base) || sig.r#type.to_ascii_lowercase().contains("array") {
            map.entry(sig.in_entity.clone())
                .or_insert_with(Vec::new)
                .push((sig.name.clone(), sig.file.clone(), sig.line));
        }
    }
    map
}

fn array_type_names(input: &Input) -> HashSet<String> {
    input
        .types
        .iter()
        .filter(|td| td.kind.eq_ignore_ascii_case("array"))
        .map(|td| td.name.to_ascii_lowercase())
        .collect()
}

struct PortInfo {
    direction: String,
    single_bit: bool,
}

fn port_info_map(input: &Input) -> HashMap<String, PortInfo> {
    let mut map = HashMap::new();
    for port in &input.ports {
        map.insert(
            port.name.to_ascii_lowercase(),
            PortInfo {
                direction: port.direction.to_ascii_lowercase(),
                single_bit: helpers::is_single_bit_type(&port.r#type),
            },
        );
    }
    map
}

fn process_candidates<'a>(
    input: &'a Input,
    case_stmt: &crate::policy::input::CaseStatement,
) -> Vec<&'a Process> {
    if case_stmt.in_process.is_empty() {
        return input
            .processes
            .iter()
            .filter(|proc| proc.in_arch.eq_ignore_ascii_case(&case_stmt.in_arch))
            .collect();
    }
    input
        .processes
        .iter()
        .filter(|proc| {
            proc.label.eq_ignore_ascii_case(&case_stmt.in_process)
                && proc.in_arch.eq_ignore_ascii_case(&case_stmt.in_arch)
        })
        .collect()
}

fn signal_in_list(signal: &str, list: &[String]) -> bool {
    list.iter()
        .any(|item| item.eq_ignore_ascii_case(signal))
}

fn is_simple_identifier(expr: &str) -> bool {
    !expr.is_empty()
        && expr
            .chars()
            .all(|c| c.is_ascii_alphanumeric() || c == '_')
}

fn required_checks_for_construct(kind: &ConstructKind) -> &'static [&'static str] {
    match kind {
        ConstructKind::Fsm => &[
            "fsm.legal_state",
            "fsm.reset_known",
            "cover.fsm.transition_taken",
        ],
        ConstructKind::ReadyValid => &["rv.stable_while_stalled", "cover.rv.handshake"],
        ConstructKind::Fifo => &[
            "fifo.no_read_empty",
            "fifo.no_write_full",
            "cover.fifo.activity",
        ],
        ConstructKind::Counter => &["ctr.range", "ctr.step_rule", "cover.ctr.moved"],
    }
}
