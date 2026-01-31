use std::collections::HashMap;

use serde::Deserialize;

#[derive(Debug, Clone, Deserialize, Default)]
pub struct Input {
    #[serde(default)]
    pub standard: String,
    #[serde(default)]
    pub file_count: usize,
    #[serde(default)]
    pub entities: Vec<Entity>,
    #[serde(default)]
    pub architectures: Vec<Architecture>,
    #[serde(default)]
    pub packages: Vec<Package>,
    #[serde(default)]
    pub components: Vec<Component>,
    #[serde(default)]
    pub use_clauses: Vec<UseClause>,
    #[serde(default)]
    pub library_clauses: Vec<LibraryClause>,
    #[serde(default)]
    pub context_clauses: Vec<ContextClause>,
    #[serde(default)]
    pub signals: Vec<Signal>,
    #[serde(default)]
    pub ports: Vec<Port>,
    #[serde(default)]
    pub dependencies: Vec<Dependency>,
    #[serde(default)]
    pub symbols: Vec<Symbol>,
    #[serde(default)]
    pub scopes: Vec<Scope>,
    #[serde(default)]
    pub symbol_defs: Vec<SymbolDef>,
    #[serde(default)]
    pub name_uses: Vec<NameUse>,
    #[serde(default)]
    pub files: Vec<FileInfo>,
    #[serde(default)]
    pub verification_blocks: Vec<VerificationBlock>,
    #[serde(default)]
    pub verification_tags: Vec<VerificationTag>,
    #[serde(default)]
    pub verification_tag_errors: Vec<VerificationTagError>,
    #[serde(default)]
    pub instances: Vec<Instance>,
    #[serde(default)]
    pub case_statements: Vec<CaseStatement>,
    #[serde(default)]
    pub processes: Vec<Process>,
    #[serde(default)]
    pub concurrent_assignments: Vec<ConcurrentAssignment>,
    #[serde(default)]
    pub generates: Vec<GenerateStatement>,
    #[serde(default)]
    pub configurations: Vec<Configuration>,
    #[serde(default)]
    pub types: Vec<TypeDeclaration>,
    #[serde(default)]
    pub subtypes: Vec<SubtypeDeclaration>,
    #[serde(default)]
    pub functions: Vec<FunctionDeclaration>,
    #[serde(default)]
    pub procedures: Vec<ProcedureDeclaration>,
    #[serde(default)]
    pub constant_decls: Vec<ConstantDeclaration>,
    #[serde(default)]
    pub enum_literals: Vec<String>,
    #[serde(default)]
    pub constants: Vec<String>,
    #[serde(default)]
    pub shared_variables: Vec<String>,
    #[serde(default)]
    pub comparisons: Vec<Comparison>,
    #[serde(default)]
    pub arithmetic_ops: Vec<ArithmeticOp>,
    #[serde(default)]
    pub signal_deps: Vec<SignalDep>,
    #[serde(default)]
    pub cdc_crossings: Vec<CDCCrossing>,
    #[serde(default)]
    pub signal_usages: Vec<SignalUsage>,
    #[serde(default)]
    pub lint_config: LintConfig,
    #[serde(default)]
    pub third_party_files: Vec<String>,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct LintConfig {
    #[serde(default)]
    pub rules: HashMap<String, String>,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct Entity {
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub file: String,
    #[serde(default)]
    pub line: usize,
    #[serde(default)]
    pub ports: Vec<Port>,
    #[serde(default)]
    pub generics: Vec<GenericDecl>,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct Architecture {
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub entity_name: String,
    #[serde(default)]
    pub file: String,
    #[serde(default)]
    pub line: usize,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct Package {
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub file: String,
    #[serde(default)]
    pub line: usize,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct Component {
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub entity_ref: String,
    #[serde(default)]
    pub file: String,
    #[serde(default)]
    pub line: usize,
    #[serde(default)]
    pub is_instance: bool,
    #[serde(default)]
    pub ports: Vec<Port>,
    #[serde(default)]
    pub generics: Vec<GenericDecl>,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct Signal {
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub r#type: String,
    #[serde(default)]
    pub file: String,
    #[serde(default)]
    pub line: usize,
    #[serde(default)]
    pub in_entity: String,
    #[serde(default)]
    pub width: usize,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct Port {
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub direction: String,
    #[serde(default)]
    pub r#type: String,
    #[serde(default)]
    pub default: String,
    #[serde(default)]
    pub line: usize,
    #[serde(default)]
    pub in_entity: String,
    #[serde(default)]
    pub width: usize,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct GenericDecl {
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub kind: String,
    #[serde(default)]
    pub r#type: String,
    #[serde(default)]
    pub class: String,
    #[serde(default)]
    pub default: String,
    #[serde(default)]
    pub line: usize,
    #[serde(default)]
    pub in_entity: String,
    #[serde(default)]
    pub in_component: String,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct UseClause {
    #[serde(default)]
    pub items: Vec<String>,
    #[serde(default)]
    pub file: String,
    #[serde(default)]
    pub line: usize,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct LibraryClause {
    #[serde(default)]
    pub libraries: Vec<String>,
    #[serde(default)]
    pub file: String,
    #[serde(default)]
    pub line: usize,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct ContextClause {
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub file: String,
    #[serde(default)]
    pub line: usize,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct Dependency {
    #[serde(default)]
    pub source: String,
    #[serde(default)]
    pub target: String,
    #[serde(default)]
    pub kind: String,
    #[serde(default)]
    pub line: usize,
    #[serde(default)]
    pub resolved: bool,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct Symbol {
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub kind: String,
    #[serde(default)]
    pub file: String,
    #[serde(default)]
    pub line: usize,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct FileInfo {
    #[serde(default)]
    pub path: String,
    #[serde(default)]
    pub library: String,
    #[serde(default)]
    pub is_third_party: bool,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct Scope {
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub kind: String,
    #[serde(default)]
    pub file: String,
    #[serde(default)]
    pub line: usize,
    #[serde(default)]
    pub parent: String,
    #[serde(default)]
    pub path: Vec<String>,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct SymbolDef {
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub kind: String,
    #[serde(default)]
    pub file: String,
    #[serde(default)]
    pub line: usize,
    #[serde(default)]
    pub scope: String,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct NameUse {
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub kind: String,
    #[serde(default)]
    pub file: String,
    #[serde(default)]
    pub line: usize,
    #[serde(default)]
    pub scope: String,
    #[serde(default)]
    pub context: String,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct VerificationBlock {
    #[serde(default)]
    pub label: String,
    #[serde(default)]
    pub line_start: usize,
    #[serde(default)]
    pub line_end: usize,
    #[serde(default)]
    pub file: String,
    #[serde(default)]
    pub in_arch: String,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct VerificationTag {
    #[serde(default)]
    pub id: String,
    #[serde(default)]
    pub scope: String,
    #[serde(default)]
    pub bindings: HashMap<String, String>,
    #[serde(default)]
    pub file: String,
    #[serde(default)]
    pub line: usize,
    #[serde(default)]
    pub raw: String,
    #[serde(default)]
    pub in_arch: String,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct VerificationTagError {
    #[serde(default)]
    pub file: String,
    #[serde(default)]
    pub line: usize,
    #[serde(default)]
    pub raw: String,
    #[serde(default)]
    pub message: String,
    #[serde(default)]
    pub in_arch: String,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct Instance {
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub target: String,
    #[serde(default)]
    pub port_map: HashMap<String, String>,
    #[serde(default)]
    pub generic_map: HashMap<String, String>,
    #[serde(default)]
    pub associations: Vec<Association>,
    #[serde(default)]
    pub file: String,
    #[serde(default)]
    pub line: usize,
    #[serde(default)]
    pub in_arch: String,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct Association {
    #[serde(default)]
    pub kind: String,
    #[serde(default)]
    pub formal: String,
    #[serde(default)]
    pub actual: String,
    #[serde(default)]
    pub is_positional: bool,
    #[serde(default)]
    pub actual_kind: String,
    #[serde(default)]
    pub actual_base: String,
    #[serde(default)]
    pub actual_full: String,
    #[serde(default)]
    pub line: usize,
    #[serde(default)]
    pub position_index: usize,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct CaseStatement {
    #[serde(default)]
    pub expression: String,
    #[serde(default)]
    pub choices: Vec<String>,
    #[serde(default)]
    pub has_others: bool,
    #[serde(default)]
    pub file: String,
    #[serde(default)]
    pub line: usize,
    #[serde(default)]
    pub in_process: String,
    #[serde(default)]
    pub in_arch: String,
    #[serde(default)]
    pub is_complete: bool,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct ConcurrentAssignment {
    #[serde(default)]
    pub target: String,
    #[serde(default)]
    pub read_signals: Vec<String>,
    #[serde(default)]
    pub file: String,
    #[serde(default)]
    pub line: usize,
    #[serde(default)]
    pub in_arch: String,
    #[serde(default)]
    pub kind: String,
    #[serde(default)]
    pub in_generate: bool,
    #[serde(default)]
    pub generate_label: String,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct Comparison {
    #[serde(default)]
    pub left_operand: String,
    #[serde(default)]
    pub operator: String,
    #[serde(default)]
    pub right_operand: String,
    #[serde(default)]
    pub file: String,
    #[serde(default)]
    pub line: usize,
    #[serde(default)]
    pub in_arch: String,
    #[serde(default)]
    pub is_literal: bool,
    #[serde(default)]
    pub literal_value: String,
    #[serde(default)]
    pub literal_bits: usize,
    #[serde(default)]
    pub result_drives: String,
    #[serde(default)]
    pub in_process: String,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct ArithmeticOp {
    #[serde(default)]
    pub operator: String,
    #[serde(default)]
    pub operands: Vec<String>,
    #[serde(default)]
    pub result: String,
    #[serde(default)]
    pub file: String,
    #[serde(default)]
    pub line: usize,
    #[serde(default)]
    pub is_guarded: bool,
    #[serde(default)]
    pub guard_signal: String,
    #[serde(default)]
    pub in_process: String,
    #[serde(default)]
    pub in_arch: String,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct SignalDep {
    #[serde(default)]
    pub source: String,
    #[serde(default)]
    pub target: String,
    #[serde(default)]
    pub file: String,
    #[serde(default)]
    pub line: usize,
    #[serde(default)]
    pub is_sequential: bool,
    #[serde(default)]
    pub in_process: String,
    #[serde(default)]
    pub in_arch: String,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct CDCCrossing {
    #[serde(default)]
    pub signal: String,
    #[serde(default)]
    pub source_clock: String,
    #[serde(default)]
    pub dest_clock: String,
    #[serde(default)]
    pub is_synchronized: bool,
    #[serde(default)]
    pub sync_stages: usize,
    #[serde(default)]
    pub is_multi_bit: bool,
    #[serde(default)]
    pub source_proc: String,
    #[serde(default)]
    pub dest_proc: String,
    #[serde(default)]
    pub file: String,
    #[serde(default)]
    pub line: usize,
    #[serde(default)]
    pub in_arch: String,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct SignalUsage {
    #[serde(default)]
    pub signal: String,
    #[serde(default)]
    pub is_read: bool,
    #[serde(default)]
    pub is_written: bool,
    #[serde(default)]
    pub in_process: String,
    #[serde(default)]
    pub in_port_map: bool,
    #[serde(default)]
    pub instance_name: String,
    #[serde(default)]
    pub in_psl: bool,
    #[serde(default)]
    pub line: usize,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct Process {
    #[serde(default)]
    pub label: String,
    #[serde(default)]
    pub sensitivity_list: Vec<String>,
    #[serde(default)]
    pub is_sequential: bool,
    #[serde(default)]
    pub is_combinational: bool,
    #[serde(default)]
    pub clock_signal: String,
    #[serde(default)]
    pub clock_edge: String,
    #[serde(default)]
    pub has_reset: bool,
    #[serde(default)]
    pub reset_signal: String,
    #[serde(default)]
    pub reset_async: bool,
    #[serde(default)]
    pub assigned_signals: Vec<String>,
    #[serde(default)]
    pub read_signals: Vec<String>,
    #[serde(default)]
    pub variables: Vec<VariableDecl>,
    #[serde(default)]
    pub procedure_calls: Vec<ProcedureCall>,
    #[serde(default)]
    pub function_calls: Vec<FunctionCall>,
    #[serde(default)]
    pub wait_statements: Vec<WaitStatement>,
    #[serde(default)]
    pub file: String,
    #[serde(default)]
    pub line: usize,
    #[serde(default)]
    pub in_arch: String,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct VariableDecl {
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub r#type: String,
    #[serde(default)]
    pub line: usize,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct ProcedureCall {
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub line: usize,
    #[serde(default)]
    pub in_process: String,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct FunctionCall {
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub line: usize,
    #[serde(default)]
    pub in_process: String,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct WaitStatement {
    #[serde(default)]
    pub line: usize,
    #[serde(default)]
    pub in_process: String,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct GenerateStatement {
    #[serde(default)]
    pub label: String,
    #[serde(default)]
    pub kind: String,
    #[serde(default)]
    pub file: String,
    #[serde(default)]
    pub line: usize,
    #[serde(default)]
    pub in_arch: String,
    #[serde(default)]
    pub condition: String,
    #[serde(default)]
    pub loop_var: String,
    #[serde(default)]
    pub range_low: String,
    #[serde(default)]
    pub range_high: String,
    #[serde(default)]
    pub range_dir: String,
    #[serde(default)]
    pub can_elaborate: bool,
    #[serde(default)]
    pub iteration_count: i64,
    #[serde(default)]
    pub signals: Vec<String>,
    #[serde(default)]
    pub instances: Vec<String>,
    #[serde(default)]
    pub processes: Vec<String>,
    #[serde(default)]
    pub file_scope: String,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct Configuration {
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub entity_name: String,
    #[serde(default)]
    pub file: String,
    #[serde(default)]
    pub line: usize,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct TypeDeclaration {
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub kind: String,
    #[serde(default)]
    pub line: usize,
    #[serde(default)]
    pub file: String,
    #[serde(default)]
    pub in_arch: String,
    #[serde(default)]
    pub in_package: String,
    #[serde(default)]
    pub enum_literals: Vec<String>,
    #[serde(default)]
    pub fields: Vec<RecordField>,
    #[serde(default)]
    pub element_type: String,
    #[serde(default)]
    pub unconstrained: bool,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct RecordField {
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub r#type: String,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct SubtypeDeclaration {
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub base_type: String,
    #[serde(default)]
    pub constraint: String,
    #[serde(default)]
    pub line: usize,
    #[serde(default)]
    pub file: String,
    #[serde(default)]
    pub in_arch: String,
    #[serde(default)]
    pub in_package: String,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct FunctionDeclaration {
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub return_type: String,
    #[serde(default)]
    pub parameters: Vec<SubprogramParameter>,
    #[serde(default)]
    pub is_pure: bool,
    #[serde(default)]
    pub has_body: bool,
    #[serde(default)]
    pub line: usize,
    #[serde(default)]
    pub file: String,
    #[serde(default)]
    pub in_arch: String,
    #[serde(default)]
    pub in_package: String,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct ProcedureDeclaration {
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub parameters: Vec<SubprogramParameter>,
    #[serde(default)]
    pub has_body: bool,
    #[serde(default)]
    pub line: usize,
    #[serde(default)]
    pub file: String,
    #[serde(default)]
    pub in_arch: String,
    #[serde(default)]
    pub in_package: String,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct SubprogramParameter {
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub direction: String,
    #[serde(default)]
    pub r#type: String,
    #[serde(default)]
    pub line: usize,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct ConstantDeclaration {
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub r#type: String,
    #[serde(default)]
    pub value: String,
    #[serde(default)]
    pub file: String,
    #[serde(default)]
    pub line: usize,
    #[serde(default)]
    pub in_package: String,
    #[serde(default)]
    pub in_arch: String,
}
