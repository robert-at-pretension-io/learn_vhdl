use crate::policy::cdc;
use crate::policy::clocks_resets;
use crate::policy::combinational;
use crate::policy::configurations;
use crate::policy::core;
use crate::policy::fsm;
use crate::policy::helpers;
use crate::policy::hierarchy;
use crate::policy::input::Input;
use crate::policy::instances;
use crate::policy::latch;
use crate::policy::naming;
use crate::policy::ports;
use crate::policy::power;
use crate::policy::processes;
use crate::policy::quality;
use crate::policy::rdc;
use crate::policy::result::{AmbiguousConstruct, MissingCheckTask, Result, Summary, Violation};
use crate::policy::security;
use crate::policy::sensitivity;
use crate::policy::sequential;
use crate::policy::signals;
use crate::policy::style;
use crate::policy::subprograms;
use crate::policy::synthesis;
use crate::policy::testbench;
use crate::policy::types;
use crate::policy::verification;
use std::time::{Duration, Instant};

pub fn evaluate(input: &Input) -> Result {
    let timing_enabled = is_timing_enabled();
    let total_start = Instant::now();
    let mut timings: Vec<TimingEntry> = Vec::new();
    let mut raw = Vec::new();
    if timing_enabled {
        eprintln!("=== Policy Timing (live) ===");
    }
    raw.extend(collect_timed(
        "core",
        input,
        timing_enabled,
        &mut timings,
        core::violations,
    ));
    let verification_analysis = if timing_enabled {
        let start = Instant::now();
        let analysis = verification::analyze(input);
        let elapsed = start.elapsed();
        timings.push(TimingEntry {
            name: "verification",
            duration: elapsed,
            count: analysis.violations.len(),
        });
        analysis
    } else {
        verification::analyze(input)
    };
    raw.extend(verification_analysis.violations);
    let missing_checks = verification_analysis.missing_checks;
    let ambiguous_constructs = verification_analysis.ambiguous_constructs;
    raw.extend(collect_timed(
        "cdc",
        input,
        timing_enabled,
        &mut timings,
        cdc::violations,
    ));
    raw.extend(collect_timed(
        "combinational",
        input,
        timing_enabled,
        &mut timings,
        combinational::violations,
    ));
    raw.extend(collect_timed(
        "clocks_resets",
        input,
        timing_enabled,
        &mut timings,
        clocks_resets::violations,
    ));
    raw.extend(collect_timed(
        "clocks_resets_optional",
        input,
        timing_enabled,
        &mut timings,
        clocks_resets::optional_violations,
    ));
    raw.extend(collect_timed(
        "fsm",
        input,
        timing_enabled,
        &mut timings,
        fsm::violations,
    ));
    raw.extend(collect_timed(
        "fsm_optional",
        input,
        timing_enabled,
        &mut timings,
        fsm::optional_violations,
    ));
    raw.extend(collect_timed(
        "configurations",
        input,
        timing_enabled,
        &mut timings,
        configurations::violations,
    ));
    raw.extend(collect_timed(
        "hierarchy",
        input,
        timing_enabled,
        &mut timings,
        hierarchy::violations,
    ));
    raw.extend(collect_timed(
        "instances",
        input,
        timing_enabled,
        &mut timings,
        instances::violations,
    ));
    raw.extend(collect_timed(
        "latch",
        input,
        timing_enabled,
        &mut timings,
        latch::violations,
    ));
    raw.extend(collect_timed(
        "naming",
        input,
        timing_enabled,
        &mut timings,
        naming::violations,
    ));
    raw.extend(collect_timed(
        "naming_optional",
        input,
        timing_enabled,
        &mut timings,
        naming::optional_violations,
    ));
    raw.extend(collect_timed(
        "ports",
        input,
        timing_enabled,
        &mut timings,
        ports::violations,
    ));
    raw.extend(collect_timed(
        "ports_optional",
        input,
        timing_enabled,
        &mut timings,
        ports::optional_violations,
    ));
    raw.extend(collect_timed(
        "processes",
        input,
        timing_enabled,
        &mut timings,
        processes::violations,
    ));
    raw.extend(collect_timed(
        "power",
        input,
        timing_enabled,
        &mut timings,
        power::violations,
    ));
    raw.extend(collect_timed(
        "quality",
        input,
        timing_enabled,
        &mut timings,
        quality::violations,
    ));
    raw.extend(collect_timed(
        "quality_optional",
        input,
        timing_enabled,
        &mut timings,
        quality::optional_violations,
    ));
    raw.extend(collect_timed(
        "rdc",
        input,
        timing_enabled,
        &mut timings,
        rdc::violations,
    ));
    raw.extend(collect_timed(
        "security",
        input,
        timing_enabled,
        &mut timings,
        security::violations,
    ));
    raw.extend(collect_timed(
        "sensitivity",
        input,
        timing_enabled,
        &mut timings,
        sensitivity::violations,
    ));
    raw.extend(collect_timed(
        "sequential",
        input,
        timing_enabled,
        &mut timings,
        sequential::violations,
    ));
    raw.extend(collect_timed(
        "signals",
        input,
        timing_enabled,
        &mut timings,
        signals::violations,
    ));
    raw.extend(collect_timed(
        "style",
        input,
        timing_enabled,
        &mut timings,
        style::violations,
    ));
    raw.extend(collect_timed(
        "style_optional",
        input,
        timing_enabled,
        &mut timings,
        style::optional_violations,
    ));
    raw.extend(collect_timed(
        "subprograms",
        input,
        timing_enabled,
        &mut timings,
        subprograms::violations,
    ));
    raw.extend(collect_timed(
        "synthesis",
        input,
        timing_enabled,
        &mut timings,
        synthesis::violations,
    ));
    raw.extend(collect_timed(
        "testbench",
        input,
        timing_enabled,
        &mut timings,
        testbench::violations,
    ));
    raw.extend(collect_timed(
        "testbench_optional",
        input,
        timing_enabled,
        &mut timings,
        testbench::optional_violations,
    ));
    raw.extend(collect_timed(
        "types",
        input,
        timing_enabled,
        &mut timings,
        types::violations,
    ));
    raw.extend(collect_timed(
        "types_optional",
        input,
        timing_enabled,
        &mut timings,
        types::optional_violations,
    ));

    raw.extend(collect_timed(
        "combinational_optional",
        input,
        timing_enabled,
        &mut timings,
        combinational::optional_violations,
    ));
    raw.extend(collect_timed(
        "hierarchy_optional",
        input,
        timing_enabled,
        &mut timings,
        hierarchy::optional_violations,
    ));
    raw.extend(collect_timed(
        "latch_optional",
        input,
        timing_enabled,
        &mut timings,
        latch::optional_violations,
    ));
    raw.extend(collect_timed(
        "power_optional",
        input,
        timing_enabled,
        &mut timings,
        power::optional_violations,
    ));
    raw.extend(collect_timed(
        "rdc_optional",
        input,
        timing_enabled,
        &mut timings,
        rdc::optional_violations,
    ));
    raw.extend(collect_timed(
        "security_optional",
        input,
        timing_enabled,
        &mut timings,
        security::optional_violations,
    ));
    raw.extend(collect_timed(
        "sensitivity_optional",
        input,
        timing_enabled,
        &mut timings,
        sensitivity::optional_violations,
    ));
    raw.extend(collect_timed(
        "sequential_optional",
        input,
        timing_enabled,
        &mut timings,
        sequential::optional_violations,
    ));
    raw.extend(collect_timed(
        "signals_optional",
        input,
        timing_enabled,
        &mut timings,
        signals::optional_violations,
    ));
    raw.extend(collect_timed(
        "synthesis_optional",
        input,
        timing_enabled,
        &mut timings,
        synthesis::optional_violations,
    ));

    let filtered = filter_violations(input, raw);
    let filtered_missing_checks = filter_missing_checks(input, missing_checks);
    let filtered_ambiguous = filter_ambiguous_constructs(input, ambiguous_constructs);
    if timing_enabled {
        emit_timings(&timings, total_start.elapsed(), filtered.len());
    }
    Result {
        summary: summarize(&filtered),
        violations: filtered,
        missing_checks: filtered_missing_checks,
        ambiguous_constructs: filtered_ambiguous,
    }
}

fn filter_violations(input: &Input, violations: Vec<Violation>) -> Vec<Violation> {
    let mut out = Vec::new();
    for v in violations {
        if helpers::rule_is_disabled(input, &v.rule) {
            continue;
        }
        if helpers::is_third_party_file(input, &v.file) {
            continue;
        }
        let mut final_violation = v;
        if let Some(sev) = helpers::get_rule_severity(input, &final_violation.rule) {
            if is_valid_severity(&sev) {
                final_violation.severity = sev;
            }
        }
        out.push(final_violation);
    }
    out
}

fn summarize(violations: &[Violation]) -> Summary {
    let mut summary = Summary::default();
    summary.total_violations = violations.len();
    for v in violations {
        match v.severity.as_str() {
            "error" => summary.errors += 1,
            "warning" => summary.warnings += 1,
            "info" => summary.info += 1,
            _ => {}
        }
    }
    summary
}

fn is_valid_severity(sev: &str) -> bool {
    matches!(sev, "error" | "warning" | "info")
}

fn filter_missing_checks(
    input: &Input,
    tasks: Vec<MissingCheckTask>,
) -> Vec<MissingCheckTask> {
    if helpers::rule_is_disabled(input, "missing_verification_check") {
        return Vec::new();
    }
    tasks
        .into_iter()
        .filter(|task| !helpers::is_third_party_file(input, &task.file))
        .collect()
}

fn filter_ambiguous_constructs(
    input: &Input,
    items: Vec<AmbiguousConstruct>,
) -> Vec<AmbiguousConstruct> {
    if helpers::rule_is_disabled(input, "ambiguous_construct") {
        return Vec::new();
    }
    items
        .into_iter()
        .filter(|item| !helpers::is_third_party_file(input, &item.file))
        .collect()
}

struct TimingEntry {
    name: &'static str,
    duration: Duration,
    count: usize,
}

fn collect_timed<F>(
    name: &'static str,
    input: &Input,
    enabled: bool,
    timings: &mut Vec<TimingEntry>,
    f: F,
) -> Vec<Violation>
where
    F: FnOnce(&Input) -> Vec<Violation>,
{
    if !enabled {
        return f(input);
    }
    eprintln!("  [start] {}", name);
    let start = Instant::now();
    let out = f(input);
    let entry = TimingEntry {
        name,
        duration: start.elapsed(),
        count: out.len(),
    };
    eprintln!(
        "  [done ] {:<24} {:>6} {}",
        entry.name,
        entry.count,
        format_duration(entry.duration)
    );
    timings.push(entry);
    out
}

fn emit_timings(timings: &[TimingEntry], total: Duration, total_count: usize) {
    eprintln!("=== Policy Timing ===");
    for entry in timings {
        eprintln!(
            "  {:<28} {:>6} {}",
            entry.name,
            entry.count,
            format_duration(entry.duration)
        );
    }
    eprintln!(
        "  {:<28} {:>6} {}",
        "total",
        total_count,
        format_duration(total)
    );
}

fn format_duration(duration: Duration) -> String {
    let micros = duration.as_micros();
    if micros >= 1000 {
        format!("{:.2}ms", micros as f64 / 1000.0)
    } else {
        format!("{}us", micros)
    }
}

fn is_timing_enabled() -> bool {
    match std::env::var("VHDL_POLICY_TRACE_TIMING") {
        Ok(val) => {
            let v = val.to_ascii_lowercase();
            v == "1" || v == "true" || v == "yes" || v == "on"
        }
        Err(_) => false,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::policy::input::{Entity, Input, Signal};

    #[test]
    fn filter_respects_disabled_rules() {
        let mut input = Input::default();
        input
            .lint_config
            .rules
            .insert("entity_has_ports".to_string(), "off".to_string());
        input
            .lint_config
            .rules
            .insert("entity_no_ports_not_tb".to_string(), "off".to_string());
        input
            .lint_config
            .rules
            .insert("file_entity_mismatch".to_string(), "off".to_string());
        input
            .lint_config
            .rules
            .insert("trivial_architecture".to_string(), "off".to_string());
        input
            .lint_config
            .rules
            .insert("unused_signal".to_string(), "off".to_string());
        input.entities.push(Entity {
            name: "core".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        input
            .architectures
            .push(crate::policy::input::Architecture {
                name: "rtl".to_string(),
                entity_name: "core".to_string(),
                file: "a.vhd".to_string(),
                line: 2,
            });
        input.signals.push(Signal {
            name: "sig".to_string(),
            in_entity: "rtl".to_string(),
            ..Default::default()
        });
        let result = evaluate(&input);
        assert!(result.violations.is_empty());
    }

    #[test]
    fn filter_applies_severity_override() {
        let mut input = Input::default();
        input
            .lint_config
            .rules
            .insert("entity_has_ports".to_string(), "error".to_string());
        input
            .lint_config
            .rules
            .insert("entity_no_ports_not_tb".to_string(), "off".to_string());
        input
            .lint_config
            .rules
            .insert("file_entity_mismatch".to_string(), "off".to_string());
        input
            .lint_config
            .rules
            .insert("trivial_architecture".to_string(), "off".to_string());
        input
            .lint_config
            .rules
            .insert("unused_signal".to_string(), "off".to_string());
        input.entities.push(Entity {
            name: "core".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        input
            .architectures
            .push(crate::policy::input::Architecture {
                name: "rtl".to_string(),
                entity_name: "core".to_string(),
                file: "a.vhd".to_string(),
                line: 2,
            });
        input.signals.push(Signal {
            name: "sig".to_string(),
            in_entity: "rtl".to_string(),
            ..Default::default()
        });
        let result = evaluate(&input);
        assert_eq!(result.violations.len(), 1);
        assert_eq!(result.violations[0].severity, "error");
    }
}
