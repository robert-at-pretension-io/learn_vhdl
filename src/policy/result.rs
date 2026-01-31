use serde::Serialize;
use std::collections::HashMap;

#[derive(Debug, Clone, Serialize, PartialEq, Eq)]
pub struct Violation {
    pub rule: String,
    pub severity: String,
    pub file: String,
    pub line: usize,
    pub message: String,
}

#[derive(Debug, Clone, Serialize, Default)]
pub struct Summary {
    pub total_violations: usize,
    pub errors: usize,
    pub warnings: usize,
    pub info: usize,
}

#[derive(Debug, Clone, Serialize, PartialEq, Eq)]
pub struct VerificationAnchor {
    pub label: String,
    pub line_start: usize,
    pub line_end: usize,
    pub exists: bool,
}

#[derive(Debug, Clone, Serialize, PartialEq, Eq)]
pub struct MissingCheckTask {
    pub file: String,
    pub scope: String,
    pub anchor: VerificationAnchor,
    pub missing_ids: Vec<String>,
    pub bindings: HashMap<String, String>,
    pub notes: Vec<String>,
}

#[derive(Debug, Clone, Serialize, PartialEq, Eq)]
pub struct AmbiguousConstruct {
    pub kind: String,
    pub scope: String,
    pub file: String,
    pub line: usize,
    pub candidates: HashMap<String, Vec<String>>,
}

#[derive(Debug, Clone, Serialize, Default)]
pub struct Result {
    pub violations: Vec<Violation>,
    pub summary: Summary,
    #[serde(default)]
    pub missing_checks: Vec<MissingCheckTask>,
    #[serde(default)]
    pub ambiguous_constructs: Vec<AmbiguousConstruct>,
}
