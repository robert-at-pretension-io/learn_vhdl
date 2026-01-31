use crate::policy::helpers;
use crate::policy::input::{Input, SignalDep};
use crate::policy::result::Violation;
use std::collections::{HashMap, HashSet};

pub fn violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(combinational_feedback(input));
    out.extend(empty_sensitivity_combinational(input));
    out.extend(direct_combinational_loop(input));
    out.extend(two_stage_loop(input));
    out.extend(three_stage_loop(input));
    out.extend(cross_process_loop(input));
    out
}

pub fn optional_violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(large_combinational_process(input));
    out.extend(vhdl2008_sensitivity_all(input));
    out.extend(long_sensitivity_list(input));
    out.extend(potential_comb_loop(input));
    out
}

fn combinational_feedback(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for proc in &input.processes {
        if !proc.is_combinational {
            continue;
        }
        for assigned in &proc.assigned_signals {
            for read in &proc.read_signals {
                if !assigned.eq_ignore_ascii_case(read) {
                    continue;
                }
                if helpers::is_clock_name(assigned) || helpers::is_reset_name(assigned) {
                    continue;
                }
                if !helpers::is_actual_signal(input, assigned) {
                    continue;
                }
                if helpers::is_composite_identifier(input, assigned) {
                    continue;
                }
                out.push(Violation {
                    rule: "combinational_feedback".to_string(),
                    severity: "warning".to_string(),
                    file: proc.file.clone(),
                    line: proc.line,
                    message: format!(
                        "Combinational process '{}' reads signal '{}' that it assigns - potential combinational loop",
                        proc.label, assigned
                    ),
                });
            }
        }
    }
    out
}

fn large_combinational_process(input: &Input) -> Vec<Violation> {
    input
        .processes
        .iter()
        .filter(|proc| proc.is_combinational)
        .filter_map(|proc| {
            let total = proc.read_signals.len() + proc.assigned_signals.len();
            if total > 30 {
                Some(Violation {
                    rule: "large_combinational_process".to_string(),
                    severity: "info".to_string(),
                    file: proc.file.clone(),
                    line: proc.line,
                    message: format!(
                        "Large combinational process '{}' ({} signals) - may cause timing issues",
                        proc.label, total
                    ),
                })
            } else {
                None
            }
        })
        .collect()
}

fn empty_sensitivity_combinational(input: &Input) -> Vec<Violation> {
    input
        .processes
        .iter()
        .filter(|proc| proc.is_combinational)
        .filter(|proc| proc.sensitivity_list.is_empty())
        .filter(|proc| !proc.assigned_signals.is_empty())
        .filter(|proc| !helpers::process_in_testbench(input, proc))
        .map(|proc| Violation {
            rule: "empty_sensitivity_combinational".to_string(),
            severity: "error".to_string(),
            file: proc.file.clone(),
            line: proc.line,
            message: format!(
                "Combinational process '{}' has empty sensitivity list - will only execute once!",
                proc.label
            ),
        })
        .collect()
}

fn vhdl2008_sensitivity_all(input: &Input) -> Vec<Violation> {
    input
        .processes
        .iter()
        .filter(|proc| helpers::has_all_sensitivity(&proc.sensitivity_list))
        .map(|proc| Violation {
            rule: "vhdl2008_sensitivity_all".to_string(),
            severity: "info".to_string(),
            file: proc.file.clone(),
            line: proc.line,
            message: format!(
                "Process '{}' uses VHDL-2008 'all' sensitivity - good practice but requires VHDL-2008 support",
                proc.label
            ),
        })
        .collect()
}

fn long_sensitivity_list(input: &Input) -> Vec<Violation> {
    input
        .processes
        .iter()
        .filter(|proc| proc.is_combinational)
        .filter(|proc| !helpers::has_all_sensitivity(&proc.sensitivity_list))
        .filter(|proc| proc.sensitivity_list.len() > 8)
        .map(|proc| Violation {
            rule: "long_sensitivity_list".to_string(),
            severity: "info".to_string(),
            file: proc.file.clone(),
            line: proc.line,
            message: format!(
                "Process '{}' has {} signals in sensitivity list - consider using 'all' (VHDL-2008)",
                proc.label,
                proc.sensitivity_list.len()
            ),
        })
        .collect()
}

fn direct_combinational_loop(input: &Input) -> Vec<Violation> {
    let mut sequential_targets = HashSet::new();
    for dep in &input.signal_deps {
        if dep.is_sequential {
            sequential_targets.insert(dep.target.clone());
        }
    }
    input
        .signal_deps
        .iter()
        .filter(|dep| !dep.is_sequential && dep.source == dep.target)
        .filter(|dep| !sequential_targets.contains(&dep.target))
        .filter(|dep| !helpers::file_in_testbench(input, &dep.file))
        .filter(|dep| !helpers::is_resolved_signal(input, &dep.target))
        .map(|dep| Violation {
            rule: "direct_combinational_loop".to_string(),
            severity: "error".to_string(),
            file: dep.file.clone(),
            line: dep.line,
            message: format!(
                "Direct combinational loop: signal '{}' depends on itself",
                dep.source
            ),
        })
        .collect()
}

fn two_stage_loop(input: &Input) -> Vec<Violation> {
    let deps = filtered_combinational_deps(input);
    let mut edges: HashMap<(String, String), Vec<&SignalDep>> = HashMap::new();
    for dep in &deps {
        let key = (dep.source.to_ascii_lowercase(), dep.target.to_ascii_lowercase());
        edges.entry(key).or_default().push(*dep);
    }

    let mut out = Vec::new();
    let mut seen: HashSet<(String, String)> = HashSet::new();
    for dep in deps {
        let src = dep.source.to_ascii_lowercase();
        let dst = dep.target.to_ascii_lowercase();
        if src == dst {
            continue;
        }
        if edges.contains_key(&(dst.clone(), src.clone())) {
            let key = if src < dst {
                (src.clone(), dst.clone())
            } else {
                (dst.clone(), src.clone())
            };
            if seen.insert(key) {
                out.push(Violation {
                    rule: "two_stage_combinational_loop".to_string(),
                    severity: "error".to_string(),
                    file: dep.file.clone(),
                    line: dep.line,
                    message: format!(
                        "Combinational loop detected: '{}' -> '{}' -> '{}'",
                        dep.source, dep.target, dep.source
                    ),
                });
            }
        }
    }
    out
}

fn three_stage_loop(input: &Input) -> Vec<Violation> {
    let deps = filtered_combinational_deps(input);
    let mut adj: HashMap<String, Vec<String>> = HashMap::new();
    let mut names: HashMap<String, String> = HashMap::new();
    for dep in &deps {
        let src = dep.source.to_ascii_lowercase();
        let dst = dep.target.to_ascii_lowercase();
        adj.entry(src.clone()).or_default().push(dst.clone());
        names.entry(src).or_insert_with(|| dep.source.clone());
        names.entry(dst).or_insert_with(|| dep.target.clone());
    }

    let mut out = Vec::new();
    let mut seen: HashSet<String> = HashSet::new();
    for dep in deps {
        let a = dep.source.to_ascii_lowercase();
        let b = dep.target.to_ascii_lowercase();
        if a == b {
            continue;
        }
        let Some(nexts) = adj.get(&b) else { continue };
        for c in nexts {
            if c == &a || c == &b {
                continue;
            }
            let Some(targets) = adj.get(c) else { continue };
            if !targets.iter().any(|t| t == &a) {
                continue;
            }
            let mut key_parts = vec![a.clone(), b.clone(), c.clone()];
            key_parts.sort();
            let key = key_parts.join("|");
            if !seen.insert(key) {
                continue;
            }
            let b_name = names.get(&b).cloned().unwrap_or_else(|| b.clone());
            let c_name = names.get(c).cloned().unwrap_or_else(|| c.clone());
            out.push(Violation {
                rule: "three_stage_combinational_loop".to_string(),
                severity: "error".to_string(),
                file: dep.file.clone(),
                line: dep.line,
                message: format!(
                    "Combinational loop detected: '{}' -> '{}' -> '{}' -> '{}'",
                    dep.source, b_name, c_name, dep.source
                ),
            });
        }
    }
    out
}

fn potential_comb_loop(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for proc in &input.processes {
        if !proc.is_combinational {
            continue;
        }
        for assigned in &proc.assigned_signals {
            for read in &proc.read_signals {
                if !assigned.eq_ignore_ascii_case(read) {
                    continue;
                }
                if is_loop_false_positive(assigned) {
                    continue;
                }
                if !helpers::is_actual_signal(input, assigned) {
                    continue;
                }
                out.push(Violation {
                    rule: "potential_combinational_loop".to_string(),
                    severity: "warning".to_string(),
                    file: proc.file.clone(),
                    line: proc.line,
                    message: format!(
                        "Potential combinational loop in process '{}': signal '{}' is both read and written",
                        proc.label, assigned
                    ),
                });
            }
        }
    }
    out
}

fn is_loop_false_positive(sig: &str) -> bool {
    let lower = sig.to_ascii_lowercase();
    lower.contains("next") || lower.contains("state")
}

fn cross_process_loop(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    let mut pair_map: HashMap<(String, String), Vec<&ProcessInfo>> = HashMap::new();
    let mut procs: Vec<ProcessInfo> = Vec::new();

    for proc in &input.processes {
        if !proc.is_combinational {
            continue;
        }
        if helpers::process_in_testbench(input, proc) {
            continue;
        }
        let mut assigned = Vec::new();
        let mut read = Vec::new();
        for sig in &proc.assigned_signals {
            if helpers::is_actual_signal(input, sig) {
                assigned.push(sig.to_ascii_lowercase());
            }
        }
        for sig in &proc.read_signals {
            if helpers::is_actual_signal(input, sig) {
                read.push(sig.to_ascii_lowercase());
            }
        }
        if assigned.is_empty() || read.is_empty() {
            continue;
        }
        procs.push(ProcessInfo {
            label: proc.label.clone(),
            file: proc.file.clone(),
            line: proc.line,
            assigned,
            read,
        });
    }

    for proc in &procs {
        for a in &proc.assigned {
            for b in &proc.read {
                if a == b {
                    continue;
                }
                pair_map
                    .entry((a.clone(), b.clone()))
                    .or_default()
                    .push(proc);
            }
        }
    }

    let mut seen: HashSet<(String, String, String, String)> = HashSet::new();
    for ((a, b), procs_ab) in &pair_map {
        if a >= b {
            continue;
        }
        let Some(procs_ba) = pair_map.get(&(b.clone(), a.clone())) else { continue };
        for proc1 in procs_ab {
            for proc2 in procs_ba {
                if proc1.label == proc2.label {
                    continue;
                }
                let key = (
                    proc1.label.clone(),
                    proc2.label.clone(),
                    a.clone(),
                    b.clone(),
                );
                if !seen.insert(key) {
                    continue;
                }
                if proc1.line >= proc2.line {
                    continue;
                }
                out.push(Violation {
                    rule: "cross_process_combinational_loop".to_string(),
                    severity: "error".to_string(),
                    file: proc1.file.clone(),
                    line: proc1.line,
                    message: format!(
                        "Cross-process combinational loop between '{}' and '{}' via signals '{}' and '{}'",
                        proc1.label, proc2.label, a, b
                    ),
                });
            }
        }
    }
    out
}

#[derive(Clone)]
struct ProcessInfo {
    label: String,
    file: String,
    line: usize,
    assigned: Vec<String>,
    read: Vec<String>,
}

fn filtered_combinational_deps(input: &Input) -> Vec<&SignalDep> {
    input
        .signal_deps
        .iter()
        .filter(|dep| !dep.is_sequential)
        .filter(|dep| {
            !helpers::is_resolved_signal(input, &dep.source)
                && !helpers::is_resolved_signal(input, &dep.target)
        })
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::policy::input::{Entity, Input, Process, Signal, SignalDep};

    #[test]
    fn combinational_feedback_flags() {
        let mut input = Input::default();
        input.processes.push(Process {
            label: "p1".to_string(),
            is_combinational: true,
            read_signals: vec!["a".to_string()],
            assigned_signals: vec!["a".to_string()],
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        let v = combinational_feedback(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "combinational_feedback");
    }

    #[test]
    fn empty_sensitivity_combinational_flags() {
        let mut input = Input::default();
        input.processes.push(Process {
            label: "p1".to_string(),
            is_combinational: true,
            assigned_signals: vec!["a".to_string()],
            file: "a.vhd".to_string(),
            line: 2,
            ..Default::default()
        });
        let v = empty_sensitivity_combinational(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "empty_sensitivity_combinational");
    }

    #[test]
    fn direct_combinational_loop_flags() {
        let mut input = Input::default();
        input.signal_deps.push(SignalDep {
            source: "a".to_string(),
            target: "a".to_string(),
            file: "a.vhd".to_string(),
            line: 3,
            ..Default::default()
        });
        let v = direct_combinational_loop(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "direct_combinational_loop");
    }

    #[test]
    fn direct_combinational_loop_ignores_testbench_file() {
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "tb_top".to_string(),
            file: "tb.vhd".to_string(),
            ..Default::default()
        });
        input.signal_deps.push(SignalDep {
            source: "clk".to_string(),
            target: "clk".to_string(),
            file: "tb.vhd".to_string(),
            line: 10,
            ..Default::default()
        });
        let v = direct_combinational_loop(&input);
        assert!(v.is_empty());
    }

    #[test]
    fn direct_combinational_loop_skips_resolved_signal() {
        let mut input = Input::default();
        input.signals.push(Signal {
            name: "bus".to_string(),
            r#type: "std_logic".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            in_entity: "rtl".to_string(),
            ..Default::default()
        });
        input.signal_deps.push(SignalDep {
            source: "bus".to_string(),
            target: "bus".to_string(),
            file: "a.vhd".to_string(),
            line: 2,
            ..Default::default()
        });
        let v = direct_combinational_loop(&input);
        assert!(v.is_empty());
    }
}
