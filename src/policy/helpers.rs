use regex::Regex;

use crate::policy::input::{ConcurrentAssignment, Input, Process};

pub fn is_testbench_name(name: &str) -> bool {
    let lower = name.to_ascii_lowercase();
    lower.contains("_tb")
        || lower.contains("tb_")
        || lower.ends_with("tb")
        || lower.contains("test")
        || lower.contains("bench")
        || lower.contains("bfm")
        || lower.contains("verification")
}

pub fn is_clock_name(name: &str) -> bool {
    let lower = name.to_ascii_lowercase();
    lower == "clk"
        || lower == "clock"
        || lower.ends_with("_clk")
        || lower.starts_with("clk_")
        || lower.ends_with("_clock")
}

pub fn is_reset_name(name: &str) -> bool {
    let lower = name.to_ascii_lowercase();
    if lower.contains("reset") {
        return true;
    }
    if matches!(lower.as_str(), "rst" | "rstn") {
        return true;
    }
    if lower.starts_with("rst_")
        || lower.starts_with("rstn_")
        || lower.ends_with("_rst")
        || lower.ends_with("_rstn")
        || lower.contains("_rst_")
        || lower.contains("_rstn_")
    {
        return true;
    }
    false
}

pub fn is_single_bit_type(t: &str) -> bool {
    matches!(
        t.to_ascii_lowercase().as_str(),
        "std_logic" | "std_ulogic" | "bit"
    )
}

pub fn valid_instance_prefix(name: &str) -> bool {
    let lower = name.to_ascii_lowercase();
    lower.starts_with("u_") || lower.starts_with("i_") || lower.starts_with("inst_")
}

pub fn is_signed_type(t: &str) -> bool {
    let lower = t.to_ascii_lowercase();
    lower.contains("signed") && !lower.contains("unsigned")
}

pub fn is_unsigned_type(t: &str) -> bool {
    let lower = t.to_ascii_lowercase();
    lower.contains("unsigned")
}

pub fn is_standard_arch_name(name: &str) -> bool {
    matches!(
        name.to_ascii_lowercase().as_str(),
        "rtl"
            | "behavioral"
            | "behavioural"
            | "structural"
            | "dataflow"
            | "testbench"
            | "tb"
            | "sim"
    )
}

pub fn is_shared_variable(input: &Input, name: &str) -> bool {
    input
        .shared_variables
        .iter()
        .any(|v| v.eq_ignore_ascii_case(name))
}

pub fn is_resolved_type(type_str: &str) -> bool {
    let lower = type_str.to_ascii_lowercase();
    lower.contains("std_logic")
        || lower.contains("std_logic_vector")
        || lower.contains("signed")
        || lower.contains("unsigned")
}

pub fn is_resolved_signal(input: &Input, name: &str) -> bool {
    input
        .signals
        .iter()
        .any(|sig| sig.name.eq_ignore_ascii_case(name) && is_resolved_type(&sig.r#type))
        || input
            .ports
            .iter()
            .any(|port| port.name.eq_ignore_ascii_case(name) && is_resolved_type(&port.r#type))
}

pub fn is_unresolved_scalar_type(t: &str) -> bool {
    matches!(
        base_type_name(t).to_ascii_lowercase().as_str(),
        "bit"
            | "std_ulogic"
            | "boolean"
            | "integer"
            | "natural"
            | "positive"
            | "time"
            | "character"
            | "real"
    )
}

pub fn single_file_mode(input: &Input) -> bool {
    input.file_count <= 1
}

pub fn entity_exists(input: &Input, name: &str) -> bool {
    input
        .entities
        .iter()
        .any(|entity| entity.name.eq_ignore_ascii_case(name))
}

pub fn base_arch_name(in_arch: &str) -> String {
    in_arch.split('.').next().unwrap_or(in_arch).to_string()
}

pub fn arch_missing_entity_for_context(input: &Input, in_arch: &str) -> bool {
    let base = base_arch_name(in_arch);
    input
        .architectures
        .iter()
        .filter(|arch| arch.name.eq_ignore_ascii_case(&base))
        .any(|arch| !entity_exists(input, &arch.entity_name))
}

pub fn file_has_use_clause(input: &Input, file: &str) -> bool {
    input
        .dependencies
        .iter()
        .any(|dep| dep.kind == "use" && dep.source == file)
}

pub fn base_type_name(t: &str) -> String {
    let trimmed = t.trim().to_ascii_lowercase();
    let head = trimmed.split_whitespace().next().unwrap_or("");
    let no_params = head.split('(').next().unwrap_or("");
    no_params.split('.').last().unwrap_or(no_params).to_string()
}

pub fn is_named_composite_type(input: &Input, t: &str) -> bool {
    let base = base_type_name(t);
    input.types.iter().any(|td| {
        td.name.eq_ignore_ascii_case(&base) && (td.kind == "record" || td.kind == "array")
    })
}

pub fn is_composite_type(input: &Input, t: &str) -> bool {
    let base = base_type_name(t);
    base.contains("vector")
        || base == "signed"
        || base == "unsigned"
        || t.to_ascii_lowercase().contains("array")
        || is_named_composite_type(input, t)
}

pub fn is_composite_identifier(input: &Input, name: &str) -> bool {
    input
        .signals
        .iter()
        .any(|sig| sig.name.eq_ignore_ascii_case(name) && is_composite_type(input, &sig.r#type))
        || input.ports.iter().any(|port| {
            port.name.eq_ignore_ascii_case(name) && is_composite_type(input, &port.r#type)
        })
}

pub fn is_common_signal_name(name: &str) -> bool {
    matches!(
        name.to_ascii_lowercase().as_str(),
        "clk"
            | "rst"
            | "reset"
            | "data"
            | "addr"
            | "we"
            | "re"
            | "en"
            | "valid"
            | "ready"
            | "ack"
            | "done"
            | "start"
            | "busy"
            | "error"
    )
}

pub fn is_skip_name(input: &Input, name: &str) -> bool {
    let lower = name.to_ascii_lowercase();
    if lower.starts_with("c_")
        || lower.ends_with("_c")
        || lower.ends_with("_v")
        || lower.ends_with("_f")
        || lower.ends_with("_g")
        || lower.starts_with("g_")
    {
        return true;
    }
    if name == name.to_ascii_uppercase() && name.chars().count() > 1 {
        return true;
    }
    if Regex::new(r"^[A-Z]+_").unwrap().is_match(name) {
        return true;
    }
    if matches!(
        lower.as_str(),
        "i" | "j" | "k" | "n" | "r" | "idx" | "index" | "x" | "y"
    ) {
        return true;
    }
    if matches!(
        lower.as_str(),
        "open_ok"
            | "status_error"
            | "name_error"
            | "mode_error"
            | "read_mode"
            | "write_mode"
            | "append_mode"
            | "trace"
            | "debug"
            | "info"
            | "warning"
            | "error"
            | "failure"
            | "fatal"
    ) {
        return true;
    }
    if input
        .generates
        .iter()
        .any(|gen| !gen.loop_var.is_empty() && gen.loop_var.eq_ignore_ascii_case(name))
    {
        return true;
    }
    if matches!(
        lower.as_str(),
        "std_logic"
            | "std_ulogic"
            | "std_logic_vector"
            | "std_ulogic_vector"
            | "integer"
            | "natural"
            | "positive"
            | "boolean"
            | "bit"
            | "bit_vector"
            | "signed"
            | "unsigned"
            | "real"
            | "time"
            | "character"
            | "string"
    ) {
        return true;
    }
    if matches!(
        lower.as_str(),
        "left"
            | "right"
            | "high"
            | "low"
            | "range"
            | "reverse_range"
            | "length"
            | "ascending"
            | "image"
            | "value"
            | "pos"
            | "val"
            | "succ"
            | "pred"
            | "leftof"
            | "rightof"
            | "event"
            | "active"
            | "last_event"
            | "last_active"
            | "last_value"
            | "driving"
            | "driving_value"
            | "simple_name"
            | "instance_name"
            | "path_name"
    ) {
        return true;
    }
    if matches!(
        lower.as_str(),
        "to_integer"
            | "to_unsigned"
            | "to_signed"
            | "to_stdulogicvector"
            | "to_stdlogicvector"
            | "to_bitvector"
            | "to_bit"
            | "to_stdulogic"
            | "to_stdlogic"
            | "std_logic_vector"
            | "conv_integer"
            | "conv_unsigned"
            | "conv_signed"
            | "conv_std_logic_vector"
            | "resize"
            | "shift_left"
            | "shift_right"
            | "rotate_left"
            | "rotate_right"
            | "rising_edge"
            | "falling_edge"
            | "now"
            | "or_reduce"
            | "and_reduce"
            | "xor_reduce"
            | "nand_reduce"
            | "nor_reduce"
            | "xnor_reduce"
            | "minimum"
            | "maximum"
    ) {
        return true;
    }
    if matches!(
        lower.as_str(),
        "downto"
            | "to"
            | "range"
            | "others"
            | "all"
            | "open"
            | "null"
            | "true"
            | "false"
            | "and"
            | "or"
            | "not"
            | "xor"
            | "nand"
            | "nor"
            | "xnor"
            | "mod"
            | "rem"
            | "abs"
            | "sll"
            | "srl"
            | "sla"
            | "sra"
            | "rol"
            | "ror"
            | "is"
            | "of"
            | "if"
            | "then"
            | "else"
            | "elsif"
            | "end"
            | "when"
            | "loop"
            | "for"
            | "while"
            | "next"
            | "exit"
            | "return"
            | "case"
            | "select"
            | "with"
            | "after"
            | "transport"
            | "inertial"
            | "reject"
            | "generate"
            | "assert"
            | "report"
            | "severity"
            | "wait"
            | "until"
    ) {
        return true;
    }
    false
}

pub fn process_in_testbench(input: &Input, proc: &Process) -> bool {
    input.architectures.iter().any(|arch| {
        arch.name.eq_ignore_ascii_case(&proc.in_arch) && is_testbench_name(&arch.entity_name)
    })
}

pub fn concurrent_in_testbench(input: &Input, ca: &ConcurrentAssignment) -> bool {
    input
        .entities
        .iter()
        .any(|entity| entity.file == ca.file && is_testbench_name(&entity.name))
}

pub fn file_in_testbench(input: &Input, file: &str) -> bool {
    input
        .entities
        .iter()
        .any(|entity| entity.file == file && is_testbench_name(&entity.name))
}

pub fn has_all_sensitivity(sens_list: &[String]) -> bool {
    sens_list.iter().any(|s| s.eq_ignore_ascii_case("all"))
}

pub fn sensitivity_list_has_clock(sens_list: &[String]) -> bool {
    sens_list.iter().any(|s| is_clock_name(s))
}

pub fn sig_in_sensitivity(sig: &str, sens_list: &[String]) -> bool {
    let sig_lower = sig.to_ascii_lowercase();
    sens_list.iter().any(|s| s.eq_ignore_ascii_case(sig))
        || sens_list
            .iter()
            .any(|s| sig_lower.starts_with(&format!("{}.", s.to_ascii_lowercase())))
        || sens_list.iter().any(|s| {
            s.to_ascii_lowercase()
                .starts_with(&format!("{}.", sig_lower))
        })
}

pub fn sig_in_reads(sig: &str, read_list: &[String]) -> bool {
    let sig_lower = sig.to_ascii_lowercase();
    read_list.iter().any(|r| r.eq_ignore_ascii_case(sig))
        || read_list.iter().any(|r| {
            r.to_ascii_lowercase()
                .starts_with(&format!("{}.", sig_lower))
        })
        || read_list
            .iter()
            .any(|r| sig_lower.starts_with(&format!("{}.", r.to_ascii_lowercase())))
}

pub fn signal_in_list(sig: &str, list: &[String]) -> bool {
    list.iter().any(|item| item.eq_ignore_ascii_case(sig))
}

pub fn is_counter_name(name: &str) -> bool {
    let lower = name.to_ascii_lowercase();
    ["count", "cnt", "counter", "timer", "tick"]
        .iter()
        .any(|pattern| lower.contains(pattern))
}

pub fn is_state_name(name: &str) -> bool {
    let lower = name.to_ascii_lowercase();
    ["state", "fsm", "mode"]
        .iter()
        .any(|pattern| lower.contains(pattern))
}

pub fn is_enum_literal(input: &Input, name: &str) -> bool {
    input
        .enum_literals
        .iter()
        .any(|lit| lit.eq_ignore_ascii_case(name))
}

pub fn is_constant(input: &Input, name: &str) -> bool {
    input.constants.iter().any(|c| c.eq_ignore_ascii_case(name))
}

pub fn is_actual_signal(input: &Input, name: &str) -> bool {
    !is_enum_literal(input, name) && !is_constant(input, name) && !is_skip_name(input, name)
}

pub fn rule_is_disabled(input: &Input, rule: &str) -> bool {
    if matches!(input.lint_config.rules.get(rule), Some(val) if val == "off") {
        return true;
    }
    is_optional_rule(rule) && !input.lint_config.rules.contains_key(rule)
}

pub fn get_rule_severity(input: &Input, rule: &str) -> Option<String> {
    input.lint_config.rules.get(rule).cloned()
}

pub fn is_third_party_file(input: &Input, file: &str) -> bool {
    input
        .third_party_files
        .iter()
        .any(|f| file == f || file.ends_with(f))
}

pub fn is_optional_rule(rule: &str) -> bool {
    matches!(
        rule,
        "entity_naming"
            | "naming_convention"
            | "entity_has_ports"
            | "entity_no_ports_not_tb"
            | "entity_without_arch"
            | "architecture_has_entity"
            | "configuration_missing_entity"
            | "component_resolved"
            | "signal_input_naming"
            | "signal_output_naming"
            | "active_low_naming"
            | "async_reset_active_high"
            | "missing_reset"
            | "instance_naming_convention"
            | "positional_mapping"
            | "process_label_missing"
            | "architecture_naming_convention"
            | "empty_architecture"
            | "trivial_architecture"
            | "multiple_entities_per_file"
            | "large_entity"
            | "wide_signal"
            | "duplicate_signal_name"
            | "single_state_signal"
            | "fsm_unreachable_state"
            | "state_signal_not_enum"
            | "fsm_missing_default_state"
            | "fsm_unhandled_state"
            | "large_combinational_process"
            | "vhdl2008_sensitivity_all"
            | "long_sensitivity_list"
            | "combinational_feedback"
            | "empty_sensitivity_combinational"
            | "direct_combinational_loop"
            | "two_stage_combinational_loop"
            | "three_stage_combinational_loop"
            | "potential_combinational_loop"
            | "cross_process_combinational_loop"
            | "sensitivity_list_superfluous"
            | "sensitivity_list_incomplete"
            | "missing_reset_sensitivity"
            | "missing_clock_sensitivity"
            | "very_wide_register"
            | "mixed_edge_clocking"
            | "async_reset_naming"
            | "sparse_port_map"
            | "empty_port_map"
            | "instance_name_matches_component"
            | "repeated_component_instantiation"
            | "many_instances"
            | "hardcoded_port_value"
            | "open_port_connection"
            | "floating_instance_input"
            | "very_long_file"
            | "large_package"
            | "short_signal_name"
            | "long_signal_name"
            | "short_port_name"
            | "entity_name_with_numbers"
            | "mixed_port_directions"
            | "bidirectional_port"
            | "unused_signal"
            | "undriven_signal"
            | "undriven_output_port"
            | "inout_as_input"
            | "inout_as_output"
            | "unresolved_dependency"
            | "undeclared_signal_usage"
            | "multi_driven_signal"
            | "unused_input_port"
            | "duplicate_signal_in_entity"
            | "duplicate_port_in_entity"
            | "duplicate_entity_in_file"
            | "file_entity_mismatch"
            | "many_signals"
            | "buffer_port"
            | "deep_generate_nesting"
            | "unlabeled_generate"
            | "magic_width_number"
            | "hardcoded_generic"
            | "multiple_clock_domains"
            | "multiple_clocks_in_process"
            | "very_wide_bus"
            | "critical_signal_no_reset"
            | "combinational_reset"
            | "unregistered_output"
            | "potential_memory_inference"
            | "complex_process"
            | "legacy_packages"
            | "testbench_with_ports"
            | "mismatched_tb_architecture"
            | "tb_with_synth_arch"
            | "combinational_incomplete_assignment"
            | "comb_process_no_default"
            | "conditional_assignment_review"
            | "selected_assignment_review"
            | "combinational_default_values"
            | "enum_case_incomplete"
            | "fsm_no_reset_state"
            | "mixed_signedness"
            | "large_literal_comparison"
            | "magic_number_comparison"
            | "counter_trigger"
            | "inverted_trigger"
            | "multi_trigger_process"
            | "cdc_unsync_single_bit"
            | "cdc_unsync_multi_bit"
            | "cdc_insufficient_sync"
            | "async_reset_unsynchronized"
            | "partial_reset_domain"
            | "short_reset_sync"
            | "reset_crosses_domains"
            | "combinational_reset_gen"
            | "potential_latch"
            | "incomplete_case_latch"
            | "reset_not_std_logic"
            | "clock_not_std_logic"
            | "signal_in_seq_and_comb"
            | "unguarded_multiplication"
            | "unguarded_division"
            | "unguarded_exponent"
            | "power_hotspot"
            | "combinational_multiplier"
            | "weak_guard"
            | "dsp_candidate_no_control"
            | "clock_gating_opportunity"
            | "gated_clock_detection"
            | "signal_crosses_clock_domain"
            | "port_width_mismatch"
            | "input_port_driven"
            | "procedure_param_invalid_mode"
            | "function_param_invalid_mode"
            | "trigger_drives_output"
    )
}
