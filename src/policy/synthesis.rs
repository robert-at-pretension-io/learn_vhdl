use regex::Regex;

use crate::policy::helpers;
use crate::policy::input::Input;
use crate::policy::result::Violation;
use std::collections::HashSet;

pub fn violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(signal_crosses_clock_domain(input));
    out
}

pub fn optional_violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(gated_clock_detection(input));
    out.extend(multiple_clock_domains(input));
    out.extend(very_wide_bus(input));
    out.extend(critical_signal_no_reset(input));
    out.extend(combinational_reset(input));
    out.extend(potential_memory_inference(input));
    out.extend(unregistered_output(input));
    out
}

fn multiple_clock_domains(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for arch in &input.architectures {
        let clocks: std::collections::HashSet<String> = input
            .processes
            .iter()
            .filter(|p| p.file == arch.file && !p.clock_signal.is_empty())
            .map(|p| p.clock_signal.to_ascii_lowercase())
            .collect();
        if clocks.len() > 1 {
            let clock_list: Vec<String> = clocks.into_iter().collect();
            out.push(Violation {
                rule: "multiple_clock_domains".to_string(),
                severity: "warning".to_string(),
                file: arch.file.clone(),
                line: arch.line,
                message: format!(
                    "Architecture '{}' uses multiple clocks {:?} - ensure proper CDC synchronization",
                    arch.name, clock_list
                ),
            });
        }
    }
    out
}

fn signal_crosses_clock_domain(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for proc1 in input.processes.iter().filter(|p| p.is_sequential) {
        for proc2 in input.processes.iter().filter(|p| p.is_sequential) {
            if proc1.clock_signal.is_empty()
                || proc2.clock_signal.is_empty()
                || proc1.clock_signal.eq_ignore_ascii_case(&proc2.clock_signal)
            {
                continue;
            }
            if proc1.file != proc2.file {
                continue;
            }
            for assigned in &proc1.assigned_signals {
                for read in &proc2.read_signals {
                    if !assigned.eq_ignore_ascii_case(read) {
                        continue;
                    }
                    out.push(Violation {
                        rule: "signal_crosses_clock_domain".to_string(),
                        severity: "error".to_string(),
                        file: proc1.file.clone(),
                        line: proc1.line,
                        message: format!(
                            "Signal '{}' written in '{}' domain, read in '{}' domain - needs synchronizer",
                            assigned, proc1.clock_signal, proc2.clock_signal
                        ),
                    });
                }
            }
        }
    }
    out
}

fn very_wide_bus(input: &Input) -> Vec<Violation> {
    input
        .signals
        .iter()
        .filter_map(|sig| {
            let width = extract_width(&sig.r#type);
            if width > 64 {
                Some(Violation {
                    rule: "very_wide_bus".to_string(),
                    severity: "info".to_string(),
                    file: sig.file.clone(),
                    line: sig.line,
                    message: format!(
                        "Signal '{}' is {} bits wide - consider pipelining for timing closure",
                        sig.name, width
                    ),
                })
            } else {
                None
            }
        })
        .collect()
}

fn extract_width(type_str: &str) -> usize {
    let lower = type_str.to_ascii_lowercase();
    let re_downto = Regex::new(r"\(([0-9]+) downto 0\)").unwrap();
    let re_to = Regex::new(r"\(0 to ([0-9]+)\)").unwrap();
    if let Some(caps) = re_downto.captures(&lower) {
        if let Ok(val) = caps.get(1).unwrap().as_str().parse::<usize>() {
            return val + 1;
        }
    }
    if let Some(caps) = re_to.captures(&lower) {
        if let Ok(val) = caps.get(1).unwrap().as_str().parse::<usize>() {
            return val + 1;
        }
    }
    0
}

fn critical_signal_no_reset(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for proc in input
        .processes
        .iter()
        .filter(|p| p.is_sequential && !p.has_reset)
    {
        for assigned in &proc.assigned_signals {
            if is_critical_signal_name(assigned) {
                out.push(Violation {
                    rule: "critical_signal_no_reset".to_string(),
                    severity: "warning".to_string(),
                    file: proc.file.clone(),
                    line: proc.line,
                    message: format!(
                        "Critical signal '{}' in process '{}' has no reset initialization",
                        assigned, proc.label
                    ),
                });
            }
        }
    }
    out
}

fn is_critical_signal_name(name: &str) -> bool {
    let lower = name.to_ascii_lowercase();
    lower.contains("valid")
        || lower.contains("enable")
        || lower.contains("ready")
        || lower.contains("error")
        || lower.contains("state")
        || lower.contains("count")
}

fn gated_clock_detection(input: &Input) -> Vec<Violation> {
    let clock_signals = collect_clock_signals(input);
    let mut out = Vec::new();
    for ca in &input.concurrent_assignments {
        if helpers::is_clock_name(&ca.target)
            && clock_signals.contains(&ca.target.to_ascii_lowercase())
            && !helpers::concurrent_in_testbench(input, ca)
        {
            out.push(Violation {
                rule: "gated_clock_detection".to_string(),
                severity: "warning".to_string(),
                file: ca.file.clone(),
                line: ca.line,
                message: format!(
                    "Clock signal '{}' assigned in concurrent statement - potential gated clock (use clock enable instead)",
                    ca.target
                ),
            });
        }
    }
    for proc in &input.processes {
        if proc.is_combinational {
            for assigned in &proc.assigned_signals {
                if helpers::is_clock_name(assigned)
                    && clock_signals.contains(&assigned.to_ascii_lowercase())
                    && !helpers::process_in_testbench(input, proc)
                {
                    out.push(Violation {
                        rule: "gated_clock_detection".to_string(),
                        severity: "warning".to_string(),
                        file: proc.file.clone(),
                        line: proc.line,
                        message: format!(
                            "Clock signal '{}' assigned in combinational process - potential gated clock",
                            assigned
                        ),
                    });
                }
            }
        }
    }
    out
}

fn collect_clock_signals(input: &Input) -> HashSet<String> {
    let mut clocks = HashSet::new();
    for proc in &input.processes {
        if proc.is_sequential && !proc.clock_signal.is_empty() {
            clocks.insert(proc.clock_signal.to_ascii_lowercase());
        }
    }
    clocks
}

fn combinational_reset(input: &Input) -> Vec<Violation> {
    input
        .concurrent_assignments
        .iter()
        .filter(|ca| helpers::is_reset_name(&ca.target))
        .filter(|ca| !ca.read_signals.is_empty())
        .map(|ca| Violation {
            rule: "combinational_reset".to_string(),
            severity: "info".to_string(),
            file: ca.file.clone(),
            line: ca.line,
            message: format!(
                "Reset signal '{}' generated combinationally - consider dedicated reset controller",
                ca.target
            ),
        })
        .collect()
}

fn potential_memory_inference(input: &Input) -> Vec<Violation> {
    input
        .signals
        .iter()
        .filter(|sig| is_array_type(&sig.r#type))
        .map(|sig| Violation {
            rule: "potential_memory_inference".to_string(),
            severity: "info".to_string(),
            file: sig.file.clone(),
            line: sig.line,
            message: format!(
                "Signal '{}' with type '{}' may infer memory block - verify synthesis results",
                sig.name, sig.r#type
            ),
        })
        .collect()
}

fn is_array_type(t: &str) -> bool {
    if t.to_ascii_lowercase().contains("array") {
        return true;
    }
    let mut seen_close = false;
    for ch in t.chars() {
        if ch == ')' {
            seen_close = true;
            continue;
        }
        if seen_close {
            if ch == '(' {
                return true;
            }
            if !ch.is_whitespace() {
                seen_close = false;
            }
        }
    }
    false
}

fn unregistered_output(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for port in input.ports.iter().filter(|p| p.direction == "out") {
        if helpers::is_testbench_name(&port.in_entity) {
            continue;
        }
        if output_driven_by_sequential(input, &port.name) {
            continue;
        }
        if !output_is_driven(input, &port.name) {
            continue;
        }
        let file = get_entity_file(input, &port.in_entity);
        out.push(Violation {
            rule: "unregistered_output".to_string(),
            severity: "warning".to_string(),
            file,
            line: port.line,
            message: format!(
                "Output port '{}' is driven by combinational logic - consider registering for timing closure",
                port.name
            ),
        });
    }
    out
}

fn output_driven_by_sequential(input: &Input, port_name: &str) -> bool {
    input.processes.iter().any(|proc| {
        proc.is_sequential
            && proc
                .assigned_signals
                .iter()
                .any(|sig| sig.eq_ignore_ascii_case(port_name))
    })
}

fn output_is_driven(input: &Input, port_name: &str) -> bool {
    input.processes.iter().any(|proc| {
        proc.assigned_signals
            .iter()
            .any(|sig| sig.eq_ignore_ascii_case(port_name))
    }) || input
        .concurrent_assignments
        .iter()
        .any(|ca| ca.target.eq_ignore_ascii_case(port_name))
}

fn get_entity_file(input: &Input, entity_name: &str) -> String {
    input
        .entities
        .iter()
        .find(|entity| entity.name.eq_ignore_ascii_case(entity_name))
        .map(|entity| entity.file.clone())
        .unwrap_or_else(|| "unknown".to_string())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::policy::input::{Input, Signal};

    #[test]
    fn very_wide_bus_flags() {
        let mut input = Input::default();
        input.signals.push(Signal {
            name: "bus".to_string(),
            r#type: "std_logic_vector(128 downto 0)".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        let v = very_wide_bus(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "very_wide_bus");
    }
}
