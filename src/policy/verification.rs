use crate::policy::helpers;
use crate::policy::input::{
    Input, Process, VerificationTag, VerificationTagError, VerificationWaiver,
    VerificationWaiverError,
};
use crate::policy::result::{
    AmbiguousConstruct, MissingCheckTask, VerificationAnchor, Violation, Waiver,
};
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
    binding_kinds: HashMap<String, Vec<String>>,
    #[serde(default)]
    binding_types: HashMap<String, String>,
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
    ResetHygiene,
    Pulse,
    Arb,
}

impl ConstructKind {
    fn label(&self) -> &'static str {
        match self {
            ConstructKind::Fsm => "fsm",
            ConstructKind::Counter => "counter",
            ConstructKind::ReadyValid => "ready_valid",
            ConstructKind::Fifo => "fifo",
            ConstructKind::ResetHygiene => "reset_hygiene",
            ConstructKind::Pulse => "pulse",
            ConstructKind::Arb => "arb",
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

struct BindingContext {
    signals_by_arch: HashMap<String, HashMap<String, String>>,
    ports_by_arch: HashMap<String, HashMap<String, String>>,
    constants_by_arch: HashMap<String, HashMap<String, String>>,
    global_constants: HashMap<String, String>,
    enum_types: HashSet<String>,
}

struct WaiverState {
    waivers: Vec<Waiver>,
    by_scope: HashMap<String, HashSet<String>>,
    violations: Vec<Violation>,
}

pub struct VerificationAnalysis {
    pub violations: Vec<Violation>,
    pub missing_checks: Vec<MissingCheckTask>,
    pub ambiguous_constructs: Vec<AmbiguousConstruct>,
    pub waivers: Vec<Waiver>,
}

pub fn analyze(input: &Input) -> VerificationAnalysis {
    let registry = registry_by_id();
    let binding_context = build_binding_context(input);
    let waiver_state = build_waiver_state(input);
    let tags_by_scope = tags_by_scope(input, &registry, &binding_context);
    let detection = detect_constructs(input);
    let mut violations = Vec::new();
    violations.extend(waiver_state.violations.iter().cloned());
    violations.extend(invalid_tag_violations(input, &registry));
    violations.extend(scope_mismatch_violations(input, &registry));
    violations.extend(stale_binding_violations(
        input,
        &registry,
        &binding_context,
        &waiver_state,
    ));
    violations.extend(missing_liveness_bound(input, &registry, &waiver_state));
    violations.extend(missing_cover_companion(
        input,
        &registry,
        &tags_by_scope,
        &waiver_state,
    ));
    violations.extend(missing_verification_block(
        input,
        &detection.constructs,
        &waiver_state,
    ));
    violations.extend(missing_check_violations(
        input,
        &detection.constructs,
        &tags_by_scope,
        &registry,
        &waiver_state,
    ));
    violations.extend(ambiguous_construct_warnings(&detection.ambiguous));

    let missing_checks = missing_check_tasks(
        input,
        &detection.constructs,
        &tags_by_scope,
        &registry,
        &waiver_state,
    );

    VerificationAnalysis {
        violations,
        missing_checks,
        ambiguous_constructs: detection.ambiguous,
        waivers: waiver_state.waivers,
    }
}

fn load_registry() -> Vec<CheckEntry> {
    let payload = if let Ok(path) = env::var("VHDL_CHECK_REGISTRY") {
        fs::read_to_string(&path)
            .unwrap_or_else(|err| panic!("failed to read VHDL_CHECK_REGISTRY {}: {}", path, err))
    } else {
        include_str!("check_registry.json").to_string()
    };

    serde_json::from_str(&payload)
        .unwrap_or_else(|err| panic!("failed to parse verification check registry: {}", err))
}

fn registry_by_id() -> HashMap<String, CheckEntry> {
    let mut map = HashMap::new();
    for mut entry in load_registry() {
        entry.id = entry.id.to_ascii_lowercase();
        entry.scope_type = entry.scope_type.to_ascii_lowercase();
        entry.required_bindings = entry
            .required_bindings
            .into_iter()
            .map(|b| b.to_ascii_lowercase())
            .collect();
        entry.binding_kinds = entry
            .binding_kinds
            .into_iter()
            .map(|(k, v)| {
                let kinds = v
                    .into_iter()
                    .map(|kind| kind.to_ascii_lowercase())
                    .collect();
                (k.to_ascii_lowercase(), kinds)
            })
            .collect();
        entry.binding_types = entry
            .binding_types
            .into_iter()
            .map(|(k, v)| (k.to_ascii_lowercase(), v.to_ascii_lowercase()))
            .collect();
        map.insert(entry.id.clone(), entry);
    }
    map
}

fn invalid_tag_violations(input: &Input, registry: &HashMap<String, CheckEntry>) -> Vec<Violation> {
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

fn scope_mismatch_violations(
    input: &Input,
    registry: &HashMap<String, CheckEntry>,
) -> Vec<Violation> {
    let mut out = Vec::new();
    let arch_to_entity = arch_entity_map(input);

    for tag in &input.verification_tags {
        if !tag_is_valid(tag, registry) {
            continue;
        }
        if tag.in_arch.trim().is_empty() {
            continue;
        }
        let (scope_type, scope_name) = match parse_scope(&tag.scope) {
            Some(v) => v,
            None => continue,
        };
        match scope_type.as_str() {
            "arch" => {
                if !tag.in_arch.eq_ignore_ascii_case(&scope_name) {
                    out.push(Violation {
                        rule: "scope_mismatch".to_string(),
                        severity: "warning".to_string(),
                        file: tag.file.clone(),
                        line: tag.line,
                        message: format!(
                            "Verification tag '{}' scope '{}' does not match enclosing architecture '{}'",
                            tag.id, tag.scope, tag.in_arch
                        ),
                    });
                }
            }
            "entity" => {
                let arch = tag.in_arch.to_ascii_lowercase();
                if let Some(expected) = arch_to_entity.get(&arch) {
                    if !expected.eq_ignore_ascii_case(&scope_name) {
                        out.push(Violation {
                            rule: "scope_mismatch".to_string(),
                            severity: "warning".to_string(),
                            file: tag.file.clone(),
                            line: tag.line,
                            message: format!(
                                "Verification tag '{}' scope '{}' does not match enclosing architecture '{}'",
                                tag.id, tag.scope, tag.in_arch
                            ),
                        });
                    }
                }
            }
            _ => {}
        }
    }

    out
}

fn build_waiver_state(input: &Input) -> WaiverState {
    let mut state = WaiverState {
        waivers: Vec::new(),
        by_scope: HashMap::new(),
        violations: Vec::new(),
    };

    for err in &input.verification_waiver_errors {
        state.violations.push(waiver_error_violation(err));
    }

    for waiver in &input.verification_waivers {
        let scope_key = match waiver_scope_key(input, waiver) {
            Some(scope) => scope,
            None => {
                state.violations.push(waiver_violation(
                    waiver,
                    format!(
                        "Verification waiver '{}' has scope '{}' that does not match its declaration",
                        waiver.id, waiver.scope
                    ),
                ));
                continue;
            }
        };
        let id = waiver.id.to_ascii_lowercase();
        state.by_scope.entry(scope_key).or_default().insert(id);
        state.waivers.push(Waiver {
            id: waiver.id.clone(),
            scope: waiver.scope.clone(),
            reason: waiver.reason.clone(),
            owner: waiver.owner.clone(),
            expires: waiver.expires.clone(),
            file: waiver.file.clone(),
            line: waiver.line,
            raw: waiver.raw.clone(),
        });
    }

    state
}

fn waiver_scope_key(input: &Input, waiver: &VerificationWaiver) -> Option<String> {
    let (scope_type, scope_name) = parse_scope(&waiver.scope)?;
    match scope_type.as_str() {
        "arch" => {
            let arch = base_arch_name(&waiver.in_arch);
            if arch.eq_ignore_ascii_case(&scope_name) {
                Some(format!("arch:{}", scope_name))
            } else {
                None
            }
        }
        "entity" => {
            let arch_to_entity = arch_entity_map(input);
            let arch = base_arch_name(&waiver.in_arch);
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

fn waiver_applies(state: &WaiverState, scope_key: &str, id: &str) -> bool {
    let id = id.to_ascii_lowercase();
    state
        .by_scope
        .get(scope_key)
        .map(|set| set.contains(&id))
        .unwrap_or(false)
}

fn missing_liveness_bound(
    input: &Input,
    registry: &HashMap<String, CheckEntry>,
    waivers: &WaiverState,
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
        if tag
            .bindings
            .get("bound")
            .map(|v| v.trim())
            .unwrap_or("")
            .is_empty()
        {
            let scope_key = match tag_scope_key(input, tag) {
                Some(scope) => scope,
                None => continue,
            };
            if waiver_applies(waivers, &scope_key, "missing_liveness_bound") {
                continue;
            }
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
    waivers: &WaiverState,
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
        if waiver_applies(waivers, &scope_key, "missing_cover_companion") {
            continue;
        }
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

fn stale_binding_violations(
    input: &Input,
    registry: &HashMap<String, CheckEntry>,
    binding_context: &BindingContext,
    waivers: &WaiverState,
) -> Vec<Violation> {
    let mut out = Vec::new();
    for tag in &input.verification_tags {
        let entry = match registry.get(&tag.id.to_ascii_lowercase()) {
            Some(entry) => entry,
            None => continue,
        };
        if !tag_is_valid(tag, registry) {
            continue;
        }
        let stale = stale_bindings_for_tag(input, tag, entry, binding_context);
        if stale.is_empty() {
            continue;
        }
        if let Some(scope_key) = tag_scope_key(input, tag) {
            if waiver_applies(waivers, &scope_key, "stale_verification_tag_binding") {
                continue;
            }
        }
        out.push(Violation {
            rule: "stale_verification_tag_binding".to_string(),
            severity: "error".to_string(),
            file: tag.file.clone(),
            line: tag.line,
            message: format!(
                "Stale verification tag binding in scope {} for id {}: {}",
                tag.scope,
                tag.id,
                stale.join(", ")
            ),
        });
    }
    out
}

fn missing_verification_block(
    input: &Input,
    constructs: &[Construct],
    waivers: &WaiverState,
) -> Vec<Violation> {
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
        let scope_key = format!("arch:{}", arch.name.to_ascii_lowercase());
        if waiver_applies(waivers, &scope_key, "missing_verification_block") {
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
    waivers: &WaiverState,
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
            if waiver_applies(waivers, &scope_key, "missing_verification_check")
                || waiver_applies(waivers, &scope_key, &check_id_lower)
            {
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
    waivers: &WaiverState,
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
            let check_id_lower = check_id.to_ascii_lowercase();
            if tag_ids.contains(&check_id_lower) {
                continue;
            }
            if waiver_applies(waivers, &scope_key, "missing_verification_check")
                || waiver_applies(waivers, &scope_key, &check_id_lower)
            {
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
    if is_stray_tag_error(err) {
        return Violation {
            rule: "stray_verification_tag".to_string(),
            severity: "warning".to_string(),
            file: err.file.clone(),
            line: err.line,
            message: err.message.clone(),
        };
    }
    Violation {
        rule: "invalid_verification_tag".to_string(),
        severity: "error".to_string(),
        file: err.file.clone(),
        line: err.line,
        message: format!("Malformed verification tag: {}", err.message),
    }
}

fn is_stray_tag_error(err: &VerificationTagError) -> bool {
    err.message
        .to_ascii_lowercase()
        .contains("outside verification block")
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

fn waiver_error_violation(err: &VerificationWaiverError) -> Violation {
    Violation {
        rule: "invalid_verification_waiver".to_string(),
        severity: "error".to_string(),
        file: err.file.clone(),
        line: err.line,
        message: format!("Malformed verification waiver: {}", err.message),
    }
}

fn waiver_violation(waiver: &VerificationWaiver, message: String) -> Violation {
    Violation {
        rule: "invalid_verification_waiver".to_string(),
        severity: "error".to_string(),
        file: waiver.file.clone(),
        line: waiver.line,
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
    if entry.requires_bound
        && tag
            .bindings
            .get("bound")
            .map(|v| v.trim())
            .unwrap_or("")
            .is_empty()
    {
        return false;
    }
    true
}

fn binding_value<'a>(tag: &'a VerificationTag, key: &str) -> Option<&'a String> {
    for (k, v) in &tag.bindings {
        if k.eq_ignore_ascii_case(key) {
            return Some(v);
        }
    }
    None
}

fn binding_values_fresh(
    input: &Input,
    tag: &VerificationTag,
    entry: &CheckEntry,
    binding_context: &BindingContext,
) -> bool {
    stale_bindings_for_tag(input, tag, entry, binding_context).is_empty()
}

fn stale_bindings_for_tag(
    input: &Input,
    tag: &VerificationTag,
    entry: &CheckEntry,
    binding_context: &BindingContext,
) -> Vec<String> {
    let arch = base_arch_name(&tag.in_arch);
    let mut stale = Vec::new();
    for (key, raw) in &tag.bindings {
        let key_lower = key.to_ascii_lowercase();
        let expected_kinds = entry
            .binding_kinds
            .get(&key_lower)
            .cloned()
            .unwrap_or_else(|| {
                vec![
                    "signal".to_string(),
                    "port".to_string(),
                    "constant".to_string(),
                ]
            });
        let expected_type = entry.binding_types.get(&key_lower).cloned();
        let identifiers = extract_identifiers(raw);
        if identifiers.is_empty() {
            stale.push(format!("{}={}", key, raw));
            continue;
        }
        let mut ok = false;
        for ident in identifiers {
            if binding_identifier_ok(
                input,
                binding_context,
                &arch,
                &ident,
                &expected_kinds,
                expected_type.as_deref(),
            ) {
                ok = true;
                break;
            }
        }
        if !ok {
            stale.push(format!("{}={}", key, raw));
        }
    }
    stale
}

fn missing_required_bindings(entry: &CheckEntry, tag: &VerificationTag) -> Vec<String> {
    let mut missing = Vec::new();
    for binding in &entry.required_bindings {
        match binding_value(tag, binding) {
            Some(value) if !value.trim().is_empty() => {}
            _ => missing.push(binding.clone()),
        }
    }
    missing
}

fn tags_by_scope<'a>(
    input: &'a Input,
    registry: &HashMap<String, CheckEntry>,
    binding_context: &BindingContext,
) -> HashMap<String, Vec<&'a VerificationTag>> {
    let mut map: HashMap<String, Vec<&VerificationTag>> = HashMap::new();
    for tag in &input.verification_tags {
        if !tag_is_valid(tag, registry) {
            continue;
        }
        if let Some(entry) = registry.get(&tag.id.to_ascii_lowercase()) {
            if !binding_values_fresh(input, tag, entry, binding_context) {
                continue;
            }
        } else {
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
    constructs.extend(detect_reset_hygiene_constructs(input));
    constructs.extend(detect_pulse_constructs(input));
    constructs.extend(detect_arb_constructs(input));

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

fn detect_ready_valid_constructs(input: &Input) -> (Vec<Construct>, Vec<AmbiguousConstruct>) {
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

fn detect_reset_hygiene_constructs(input: &Input) -> Vec<Construct> {
    struct ResetGroup {
        signals: HashSet<String>,
        file: String,
        line: usize,
    }

    let enum_types = enum_type_names(input);
    let port_defs = port_defs_by_arch(input);
    let mut states: HashMap<String, ResetGroup> = HashMap::new();
    let mut counters: HashMap<String, ResetGroup> = HashMap::new();
    let mut outputs: HashMap<String, ResetGroup> = HashMap::new();

    for proc in &input.processes {
        if !proc.is_sequential || !proc.has_reset {
            continue;
        }
        let arch = base_arch_name(&proc.in_arch);
        for sig in &proc.assigned_signals {
            if sig.eq_ignore_ascii_case(&proc.reset_signal)
                || helpers::is_reset_name(sig)
                || helpers::is_clock_name(sig)
            {
                continue;
            }

            if signal_is_enum(input, sig, &enum_types) {
                let group = states.entry(arch.clone()).or_insert_with(|| ResetGroup {
                    signals: HashSet::new(),
                    file: proc.file.clone(),
                    line: proc.line,
                });
                group.signals.insert(sig.clone());
            }

            if signal_is_numeric(input, sig) {
                let group = counters.entry(arch.clone()).or_insert_with(|| ResetGroup {
                    signals: HashSet::new(),
                    file: proc.file.clone(),
                    line: proc.line,
                });
                group.signals.insert(sig.clone());
            }

            if let Some(port_map) = port_defs.get(&arch) {
                if let Some(port) = port_map.get(&sig.to_ascii_lowercase()) {
                    if port.direction == "out" || port.direction == "buffer" {
                        let group = outputs.entry(arch.clone()).or_insert_with(|| ResetGroup {
                            signals: HashSet::new(),
                            file: proc.file.clone(),
                            line: proc.line,
                        });
                        group.signals.insert(sig.clone());
                    }
                }
            }
        }
    }

    let mut constructs = Vec::new();
    for (arch, group) in states {
        if group.signals.is_empty() {
            continue;
        }
        let bindings = reset_group_bindings(&group.signals);
        constructs.push(Construct {
            kind: ConstructKind::ResetHygiene,
            in_arch: arch,
            file: group.file,
            line: group.line,
            bindings,
        });
    }
    for (arch, group) in counters {
        if group.signals.is_empty() {
            continue;
        }
        let bindings = reset_group_bindings(&group.signals);
        constructs.push(Construct {
            kind: ConstructKind::ResetHygiene,
            in_arch: arch,
            file: group.file,
            line: group.line,
            bindings,
        });
    }
    for (arch, group) in outputs {
        if group.signals.is_empty() {
            continue;
        }
        let bindings = reset_group_bindings(&group.signals);
        constructs.push(Construct {
            kind: ConstructKind::ResetHygiene,
            in_arch: arch,
            file: group.file,
            line: group.line,
            bindings,
        });
    }

    constructs
}

fn reset_group_bindings(signals: &HashSet<String>) -> HashMap<String, String> {
    let mut list: Vec<String> = signals.iter().cloned().collect();
    list.sort();
    let mut bindings = HashMap::new();
    bindings.insert("signals".to_string(), list.join(", "));
    bindings
}

fn detect_pulse_constructs(input: &Input) -> Vec<Construct> {
    let mut registered: HashMap<String, HashSet<String>> = HashMap::new();
    for proc in &input.processes {
        if !proc.is_sequential {
            continue;
        }
        let arch = base_arch_name(&proc.in_arch);
        let entry = registered.entry(arch).or_default();
        for sig in &proc.assigned_signals {
            entry.insert(sig.to_ascii_lowercase());
        }
    }

    let signal_types = signal_types_by_arch(input);
    let port_defs = port_defs_by_arch(input);
    let mut constructs = Vec::new();
    for ca in &input.concurrent_assignments {
        if ca.read_signals.len() != 2 {
            continue;
        }
        let arch = base_arch_name(&ca.in_arch);
        if !single_bit_in_arch(&signal_types, &port_defs, &arch, &ca.target) {
            continue;
        }
        let reg_set = match registered.get(&arch) {
            Some(set) => set,
            None => continue,
        };
        let mut has_registered = false;
        for sig in &ca.read_signals {
            if reg_set.contains(&sig.to_ascii_lowercase()) {
                has_registered = true;
                break;
            }
        }
        if !has_registered {
            continue;
        }
        let mut bindings = HashMap::new();
        bindings.insert("pulse".to_string(), ca.target.clone());
        constructs.push(Construct {
            kind: ConstructKind::Pulse,
            in_arch: arch,
            file: ca.file.clone(),
            line: ca.line,
            bindings,
        });
    }
    constructs
}

fn detect_arb_constructs(input: &Input) -> Vec<Construct> {
    let port_defs = port_defs_by_arch(input);
    let mut constructs = Vec::new();

    for proc in &input.processes {
        if !proc.is_sequential {
            continue;
        }
        let arch = base_arch_name(&proc.in_arch);
        let port_map = match port_defs.get(&arch) {
            Some(map) => map,
            None => continue,
        };
        let mut grants = HashSet::new();
        let mut reqs = HashSet::new();

        for sig in &proc.assigned_signals {
            if let Some(port) = port_map.get(&sig.to_ascii_lowercase()) {
                if (port.direction == "out" || port.direction == "buffer") && port.single_bit {
                    grants.insert(sig.clone());
                }
            }
        }

        for sig in &proc.read_signals {
            if sig.eq_ignore_ascii_case(&proc.reset_signal)
                || helpers::is_reset_name(sig)
                || helpers::is_clock_name(sig)
            {
                continue;
            }
            if let Some(port) = port_map.get(&sig.to_ascii_lowercase()) {
                if port.direction == "in" && port.single_bit {
                    reqs.insert(sig.clone());
                }
            }
        }

        if grants.len() < 2 || reqs.is_empty() {
            continue;
        }

        let mut grant_list: Vec<String> = grants.into_iter().collect();
        grant_list.sort();
        let mut req_list: Vec<String> = reqs.into_iter().collect();
        req_list.sort();
        let mut bindings = HashMap::new();
        bindings.insert("grants".to_string(), grant_list.join(", "));
        bindings.insert("reqs".to_string(), req_list.join(", "));
        constructs.push(Construct {
            kind: ConstructKind::Arb,
            in_arch: arch,
            file: proc.file.clone(),
            line: proc.line,
            bindings,
        });
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
    for proc in input
        .processes
        .iter()
        .filter(|p| processes.contains(&p.label))
    {
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
    for proc in input
        .processes
        .iter()
        .filter(|p| processes.contains(&p.label))
    {
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

#[derive(Clone)]
struct PortDef {
    direction: String,
    single_bit: bool,
}

fn port_defs_by_arch(input: &Input) -> HashMap<String, HashMap<String, PortDef>> {
    let mut by_arch: HashMap<String, HashMap<String, PortDef>> = HashMap::new();
    let arch_entities = arch_entity_map(input);
    for (arch, entity) in &arch_entities {
        let mut map = HashMap::new();
        for port in &input.ports {
            if port.in_entity.eq_ignore_ascii_case(entity) {
                map.insert(
                    port.name.to_ascii_lowercase(),
                    PortDef {
                        direction: port.direction.to_ascii_lowercase(),
                        single_bit: helpers::is_single_bit_type(&port.r#type),
                    },
                );
            }
        }
        by_arch.insert(arch.clone(), map);
    }
    by_arch
}

fn signal_types_by_arch(input: &Input) -> HashMap<String, HashMap<String, String>> {
    let mut by_arch: HashMap<String, HashMap<String, String>> = HashMap::new();
    for sig in &input.signals {
        let arch = base_arch_name(&sig.in_entity);
        by_arch
            .entry(arch)
            .or_default()
            .insert(sig.name.to_ascii_lowercase(), sig.r#type.clone());
    }
    by_arch
}

fn single_bit_in_arch(
    signal_types: &HashMap<String, HashMap<String, String>>,
    port_defs: &HashMap<String, HashMap<String, PortDef>>,
    arch: &str,
    name: &str,
) -> bool {
    let id = name.to_ascii_lowercase();
    if let Some(map) = signal_types.get(arch) {
        if let Some(ty) = map.get(&id) {
            return helpers::is_single_bit_type(ty);
        }
    }
    if let Some(map) = port_defs.get(arch) {
        if let Some(port) = map.get(&id) {
            return port.single_bit;
        }
    }
    false
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
    list.iter().any(|item| item.eq_ignore_ascii_case(signal))
}

fn is_simple_identifier(expr: &str) -> bool {
    !expr.is_empty() && expr.chars().all(|c| c.is_ascii_alphanumeric() || c == '_')
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
        ConstructKind::ResetHygiene => &[
            "reset.no_unknown_after_grace",
            "reset.asserted_implies_known_defaults",
        ],
        ConstructKind::Pulse => &["pulse.width_one_cycle", "cover.pulse.fired"],
        ConstructKind::Arb => &[
            "arb.onehot0",
            "arb.no_grant_without_req",
            "cover.arb.grant_seen",
        ],
    }
}

fn build_binding_context(input: &Input) -> BindingContext {
    let enum_types = enum_type_names(input);
    let mut signals_by_arch: HashMap<String, HashMap<String, String>> = HashMap::new();
    for sig in &input.signals {
        let arch = base_arch_name(&sig.in_entity);
        if arch.is_empty() {
            continue;
        }
        signals_by_arch
            .entry(arch)
            .or_default()
            .insert(sig.name.to_ascii_lowercase(), sig.r#type.clone());
    }

    let arch_entities = arch_entity_map(input);
    let mut ports_by_arch: HashMap<String, HashMap<String, String>> = HashMap::new();
    for (arch, entity) in &arch_entities {
        let mut map = HashMap::new();
        for port in &input.ports {
            if port.in_entity.eq_ignore_ascii_case(entity) {
                map.insert(port.name.to_ascii_lowercase(), port.r#type.clone());
            }
        }
        ports_by_arch.insert(arch.clone(), map);
    }

    let mut constants_by_arch: HashMap<String, HashMap<String, String>> = HashMap::new();
    let mut global_constants: HashMap<String, String> = HashMap::new();
    for constant in &input.constant_decls {
        if !constant.in_arch.is_empty() {
            let arch = base_arch_name(&constant.in_arch);
            constants_by_arch
                .entry(arch)
                .or_default()
                .insert(constant.name.to_ascii_lowercase(), constant.r#type.clone());
        } else {
            global_constants.insert(constant.name.to_ascii_lowercase(), constant.r#type.clone());
        }
    }

    BindingContext {
        signals_by_arch,
        ports_by_arch,
        constants_by_arch,
        global_constants,
        enum_types,
    }
}

fn base_arch_name(in_arch: &str) -> String {
    in_arch
        .split('.')
        .next()
        .unwrap_or(in_arch)
        .to_ascii_lowercase()
}

fn binding_identifier_ok(
    input: &Input,
    binding_context: &BindingContext,
    arch: &str,
    ident: &str,
    expected_kinds: &[String],
    expected_type: Option<&str>,
) -> bool {
    let id = ident.to_ascii_lowercase();
    for kind in expected_kinds {
        match kind.as_str() {
            "signal" => {
                if let Some(type_str) = binding_context
                    .signals_by_arch
                    .get(arch)
                    .and_then(|map| map.get(&id))
                {
                    if type_matches(input, binding_context, expected_type, type_str) {
                        return true;
                    }
                }
            }
            "port" => {
                if let Some(type_str) = binding_context
                    .ports_by_arch
                    .get(arch)
                    .and_then(|map| map.get(&id))
                {
                    if type_matches(input, binding_context, expected_type, type_str) {
                        return true;
                    }
                }
            }
            "constant" => {
                if let Some(type_str) = binding_context
                    .constants_by_arch
                    .get(arch)
                    .and_then(|map| map.get(&id))
                    .or_else(|| binding_context.global_constants.get(&id))
                {
                    if type_matches(input, binding_context, expected_type, type_str) {
                        return true;
                    }
                }
            }
            "any" => return true,
            _ => {}
        }
    }
    false
}

fn type_matches(
    input: &Input,
    binding_context: &BindingContext,
    expected: Option<&str>,
    type_str: &str,
) -> bool {
    let expected = match expected {
        Some(value) if !value.is_empty() => value,
        _ => return true,
    };
    match expected {
        "enum" => binding_context
            .enum_types
            .contains(&helpers::base_type_name(type_str)),
        "bit" => helpers::is_single_bit_type(type_str),
        "vector" => helpers::is_composite_type(input, type_str),
        "numeric" => {
            let base = helpers::base_type_name(type_str);
            matches!(base.as_str(), "integer" | "natural" | "positive")
                || helpers::is_unsigned_type(type_str)
                || helpers::is_signed_type(type_str)
        }
        _ => true,
    }
}

fn extract_identifiers(raw: &str) -> Vec<String> {
    let mut out = Vec::new();
    let mut current = String::new();
    for ch in raw.chars() {
        if ch.is_ascii_alphanumeric() || ch == '_' {
            current.push(ch);
        } else {
            if !current.is_empty() {
                out.push(current.clone());
                current.clear();
            }
        }
    }
    if !current.is_empty() {
        out.push(current);
    }
    out
}
