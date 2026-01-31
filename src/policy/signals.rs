use regex::Regex;
use std::collections::HashSet;

use crate::policy::helpers;
use crate::policy::input::{Architecture, Input, Process, Signal};
use crate::policy::result::Violation;

pub fn violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    let usage = SignalUsageIndex::from_input(input);
    out.extend(unused_signal(input, &usage));
    out.extend(undriven_signal(input, &usage));
    out.extend(multi_driven_signal(input));
    out.extend(undeclared_signal_usage(input, &usage));
    out.extend(input_port_driven(input));
    out
}

pub fn optional_violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(wide_signal(input));
    out.extend(duplicate_signal_name(input));
    out
}

pub fn is_declared_identifier(input: &Input, name: &str) -> bool {
    input
        .signals
        .iter()
        .any(|sig| sig.name.eq_ignore_ascii_case(name))
        || input
            .ports
            .iter()
            .any(|port| port.name.eq_ignore_ascii_case(name))
        || input.constants.iter().any(|c| c.eq_ignore_ascii_case(name))
        || input
            .enum_literals
            .iter()
            .any(|lit| lit.eq_ignore_ascii_case(name))
        || input
            .types
            .iter()
            .any(|t| t.name.eq_ignore_ascii_case(name))
        || input
            .subtypes
            .iter()
            .any(|st| st.name.eq_ignore_ascii_case(name))
        || input
            .functions
            .iter()
            .any(|func| func.name.eq_ignore_ascii_case(name))
        || input
            .procedures
            .iter()
            .any(|proc| proc.name.eq_ignore_ascii_case(name))
        || input
            .shared_variables
            .iter()
            .any(|v| v.eq_ignore_ascii_case(name))
        || input.processes.iter().any(|proc| {
            proc.variables
                .iter()
                .any(|var| var.name.eq_ignore_ascii_case(name))
        })
        || input.entities.iter().any(|entity| {
            entity
                .generics
                .iter()
                .any(|gen| gen.name.eq_ignore_ascii_case(name))
        })
        || input.components.iter().any(|comp| {
            comp.generics
                .iter()
                .any(|gen| gen.name.eq_ignore_ascii_case(name))
        })
}

pub fn is_actual_signal(input: &Input, name: &str) -> bool {
    !is_enum_literal(input, name)
        && !is_constant(input, name)
        && !helpers::is_shared_variable(input, name)
}

fn is_enum_literal(input: &Input, name: &str) -> bool {
    input
        .enum_literals
        .iter()
        .any(|lit| lit.eq_ignore_ascii_case(name))
}

fn is_constant(input: &Input, name: &str) -> bool {
    input.constants.iter().any(|c| c.eq_ignore_ascii_case(name))
}

fn skip_undeclared_read(
    input: &Input,
    usage: &SignalUsageIndex,
    file: &str,
    in_arch: &str,
    name: &str,
) -> bool {
    if helpers::single_file_mode(input) && helpers::arch_missing_entity_for_context(input, in_arch)
    {
        return true;
    }
    helpers::single_file_mode(input)
        && helpers::file_has_use_clause(input, file)
        && !usage.has_assigned(name)
}

fn skip_undeclared_write(input: &Input, in_arch: &str) -> bool {
    helpers::single_file_mode(input) && helpers::arch_missing_entity_for_context(input, in_arch)
}

fn unused_signal(input: &Input, usage: &SignalUsageIndex) -> Vec<Violation> {
    input
        .signals
        .iter()
        .filter(|sig| !helpers::file_in_testbench(input, &sig.file))
        .filter(|sig| !usage.has_used(&sig.name))
        .map(|sig| Violation {
            rule: "unused_signal".to_string(),
            severity: "warning".to_string(),
            file: sig.file.clone(),
            line: sig.line,
            message: format!("Signal '{}' is declared but never used", sig.name),
        })
        .collect()
}

fn undriven_signal(input: &Input, usage: &SignalUsageIndex) -> Vec<Violation> {
    input
        .signals
        .iter()
        .filter(|sig| usage.has_read(&sig.name))
        .filter(|sig| !usage.has_assigned(&sig.name))
        .map(|sig| Violation {
            rule: "undriven_signal".to_string(),
            severity: "error".to_string(),
            file: sig.file.clone(),
            line: sig.line,
            message: format!(
                "Signal '{}' is read but never assigned (undriven)",
                sig.name
            ),
        })
        .collect()
}

#[derive(Debug, Default)]
struct SignalUsageIndex {
    used: HashSet<String>,
    read: HashSet<String>,
    assigned: HashSet<String>,
}

impl SignalUsageIndex {
    fn from_input(input: &Input) -> Self {
        let mut index = SignalUsageIndex::default();

        for proc in &input.processes {
            for sig in &proc.read_signals {
                if is_actual_signal(input, sig) {
                    index.insert_read(sig);
                }
            }
            for sig in &proc.assigned_signals {
                if is_actual_signal(input, sig) {
                    index.insert_assigned(sig);
                }
            }
        }

        for ca in &input.concurrent_assignments {
            for sig in &ca.read_signals {
                if is_actual_signal(input, sig) {
                    index.insert_read(sig);
                }
            }
            if is_actual_signal(input, &ca.target) {
                index.insert_assigned(&ca.target);
            }
        }

        for usage in &input.signal_usages {
            if !is_actual_signal(input, &usage.signal) {
                continue;
            }
            if usage.is_read {
                index.insert_read(&usage.signal);
            }
            if usage.is_written || usage.in_port_map {
                index.insert_assigned(&usage.signal);
            }
            if usage.in_port_map {
                index.insert_used(&usage.signal);
            }
        }

        index
    }

    fn insert_read(&mut self, name: &str) {
        let key = name.to_ascii_lowercase();
        self.read.insert(key.clone());
        self.used.insert(key);
    }

    fn insert_assigned(&mut self, name: &str) {
        let key = name.to_ascii_lowercase();
        self.assigned.insert(key.clone());
        self.used.insert(key);
    }

    fn insert_used(&mut self, name: &str) {
        self.used.insert(name.to_ascii_lowercase());
    }

    fn has_used(&self, name: &str) -> bool {
        self.used.contains(&name.to_ascii_lowercase())
    }

    fn has_read(&self, name: &str) -> bool {
        self.read.contains(&name.to_ascii_lowercase())
    }

    fn has_assigned(&self, name: &str) -> bool {
        self.assigned.contains(&name.to_ascii_lowercase())
    }
}

fn multi_driven_signal(input: &Input) -> Vec<Violation> {
    input
        .signals
        .iter()
        .filter(|sig| !signal_in_testbench(input, sig))
        .filter(|sig| !helpers::is_composite_type(input, &sig.r#type))
        .filter(|sig| !helpers::is_resolved_type(&sig.r#type))
        .filter(|sig| helpers::is_unresolved_scalar_type(&sig.r#type))
        .filter_map(|sig| {
            let drivers = count_drivers_in_entity(input, &sig.name, &sig.in_entity, &sig.file);
            if drivers > 1 {
                Some(Violation {
                    rule: "multi_driven_signal".to_string(),
                    severity: "warning".to_string(),
                    file: sig.file.clone(),
                    line: sig.line,
                    message: format!(
                        "Signal '{}' is assigned in {} places (review for multi-driver)",
                        sig.name, drivers
                    ),
                })
            } else {
                None
            }
        })
        .collect()
}

fn undeclared_signal_usage(input: &Input, usage: &SignalUsageIndex) -> Vec<Violation> {
    let mut out = Vec::new();
    for proc in &input.processes {
        for name in &proc.read_signals {
            if helpers::is_skip_name(input, name) {
                continue;
            }
            if skip_undeclared_read(input, usage, &proc.file, &proc.in_arch, name) {
                continue;
            }
            if !is_declared_identifier(input, name) {
                out.push(Violation {
                    rule: "undeclared_signal_usage".to_string(),
                    severity: "warning".to_string(),
                    file: proc.file.clone(),
                    line: proc.line,
                    message: format!(
                        "Signal '{}' is read but not declared in this design unit",
                        name
                    ),
                });
            }
        }
        for name in &proc.assigned_signals {
            if helpers::is_skip_name(input, name) {
                continue;
            }
            if skip_undeclared_write(input, &proc.in_arch) {
                continue;
            }
            if !is_declared_identifier(input, name) {
                out.push(Violation {
                    rule: "undeclared_signal_usage".to_string(),
                    severity: "warning".to_string(),
                    file: proc.file.clone(),
                    line: proc.line,
                    message: format!(
                        "Signal '{}' is assigned but not declared in this design unit",
                        name
                    ),
                });
            }
        }
    }
    for ca in &input.concurrent_assignments {
        for name in &ca.read_signals {
            if helpers::is_skip_name(input, name) {
                continue;
            }
            if skip_undeclared_read(input, usage, &ca.file, &ca.in_arch, name) {
                continue;
            }
            if !is_declared_identifier(input, name) {
                out.push(Violation {
                    rule: "undeclared_signal_usage".to_string(),
                    severity: "warning".to_string(),
                    file: ca.file.clone(),
                    line: ca.line,
                    message: format!(
                        "Signal '{}' is read but not declared in this design unit",
                        name
                    ),
                });
            }
        }
        if helpers::is_skip_name(input, &ca.target) {
            continue;
        }
        if skip_undeclared_write(input, &ca.in_arch) {
            continue;
        }
        if !is_declared_identifier(input, &ca.target) {
            out.push(Violation {
                rule: "undeclared_signal_usage".to_string(),
                severity: "warning".to_string(),
                file: ca.file.clone(),
                line: ca.line,
                message: format!(
                    "Signal '{}' is assigned but not declared in this design unit",
                    ca.target
                ),
            });
        }
    }
    out
}

fn input_port_driven(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for port in input
        .ports
        .iter()
        .filter(|p| p.direction.eq_ignore_ascii_case("in"))
    {
        for proc in &input.processes {
            if proc
                .assigned_signals
                .iter()
                .any(|sig| sig.eq_ignore_ascii_case(&port.name))
            {
                if helpers::file_in_testbench(input, &proc.file) {
                    continue;
                }
                if let Some(arch) = input
                    .architectures
                    .iter()
                    .find(|arch| arch.name == proc.in_arch)
                {
                    if arch.entity_name.eq_ignore_ascii_case(&port.in_entity) {
                        out.push(Violation {
                            rule: "input_port_driven".to_string(),
                            severity: "error".to_string(),
                            file: proc.file.clone(),
                            line: proc.line,
                            message: format!(
                                "Input port '{}' is assigned in process '{}' (illegal driver)",
                                port.name, proc.label
                            ),
                        });
                    }
                }
            }
        }
        for ca in &input.concurrent_assignments {
            if ca.target.eq_ignore_ascii_case(&port.name) {
                if helpers::file_in_testbench(input, &ca.file) {
                    continue;
                }
                if let Some(arch) = input
                    .architectures
                    .iter()
                    .find(|arch| arch.name == ca.in_arch)
                {
                    if arch.entity_name.eq_ignore_ascii_case(&port.in_entity) {
                        out.push(Violation {
                            rule: "input_port_driven".to_string(),
                            severity: "error".to_string(),
                            file: ca.file.clone(),
                            line: ca.line,
                            message: format!(
                                "Input port '{}' is driven by concurrent assignment (illegal driver)",
                                port.name
                            ),
                        });
                    }
                }
            }
        }
    }
    out
}

fn count_drivers_in_entity(
    input: &Input,
    sig_name: &str,
    entity_name: &str,
    sig_file: &str,
) -> usize {
    let mut proc_count = 0;
    for proc in &input.processes {
        if !sig_assigned_in_process(input, sig_name, proc) {
            continue;
        }
        if let Some(arch) = input
            .architectures
            .iter()
            .find(|arch| arch.name == proc.in_arch && arch.file == proc.file)
        {
            if arch_matches_entity(arch, entity_name) && arch.file == sig_file {
                proc_count += 1;
            }
        }
    }
    let non_gen_drivers = input
        .concurrent_assignments
        .iter()
        .filter(|ca| ca.target.eq_ignore_ascii_case(sig_name) && !ca.in_generate)
        .filter(|ca| {
            input.architectures.iter().any(|arch| {
                arch.name == ca.in_arch
                    && arch_matches_entity(arch, entity_name)
                    && arch.file == ca.file
                    && arch.file == sig_file
            })
        })
        .count();
    let mut gen_labels: Vec<String> = Vec::new();
    for ca in input
        .concurrent_assignments
        .iter()
        .filter(|ca| ca.target.eq_ignore_ascii_case(sig_name) && ca.in_generate)
    {
        if !input.architectures.iter().any(|arch| {
            arch.name == ca.in_arch
                && arch_matches_entity(arch, entity_name)
                && arch.file == ca.file
                && arch.file == sig_file
        }) {
            continue;
        }
        if !gen_labels
            .iter()
            .any(|label| label.eq_ignore_ascii_case(&ca.generate_label))
        {
            gen_labels.push(ca.generate_label.clone());
        }
    }
    proc_count + non_gen_drivers + gen_labels.len()
}

fn arch_matches_entity(arch: &Architecture, entity_or_arch: &str) -> bool {
    arch.entity_name.eq_ignore_ascii_case(entity_or_arch)
        || arch
            .name
            .eq_ignore_ascii_case(&helpers::base_arch_name(entity_or_arch))
}

fn signal_in_testbench(input: &Input, sig: &Signal) -> bool {
    input.architectures.iter().any(|arch| {
        arch.name
            .eq_ignore_ascii_case(&helpers::base_arch_name(&sig.in_entity))
            && helpers::is_testbench_name(&arch.entity_name)
    })
}

fn sig_assigned_in_process(input: &Input, sig_name: &str, proc: &Process) -> bool {
    proc.assigned_signals.iter().any(|assigned| {
        assigned.eq_ignore_ascii_case(sig_name) && is_actual_signal(input, assigned)
    })
}

fn wide_signal(input: &Input) -> Vec<Violation> {
    input
        .signals
        .iter()
        .filter_map(|sig| {
            let width = extract_vector_width(&sig.r#type);
            if width > 128 {
                Some(Violation {
                    rule: "wide_signal".to_string(),
                    severity: "info".to_string(),
                    file: sig.file.clone(),
                    line: sig.line,
                    message: format!(
                        "Signal '{}' is {} bits wide - consider if this width is necessary",
                        sig.name, width
                    ),
                })
            } else {
                None
            }
        })
        .collect()
}

fn extract_vector_width(type_str: &str) -> usize {
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

fn duplicate_signal_name(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    for (i, sig1) in input.signals.iter().enumerate() {
        for sig2 in input.signals.iter().skip(i + 1) {
            if !sig1.name.eq_ignore_ascii_case(&sig2.name) {
                continue;
            }
            if sig1.in_entity == sig2.in_entity {
                continue;
            }
            if helpers::is_common_signal_name(&sig1.name) {
                continue;
            }
            out.push(Violation {
                rule: "duplicate_signal_name".to_string(),
                severity: "info".to_string(),
                file: sig1.file.clone(),
                line: sig1.line,
                message: format!(
                    "Signal '{}' also exists in entity '{}' - verify intentional",
                    sig1.name, sig2.in_entity
                ),
            });
        }
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::policy::input::{Architecture, Entity, Input, Port, Process};

    #[test]
    fn unused_signal_flags() {
        let mut input = Input::default();
        input.signals.push(Signal {
            name: "unused".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            in_entity: "ent".to_string(),
            ..Default::default()
        });
        let usage = SignalUsageIndex::from_input(&input);
        let v = unused_signal(&input, &usage);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "unused_signal");
    }

    #[test]
    fn undriven_signal_flags() {
        let mut input = Input::default();
        input.signals.push(Signal {
            name: "sig".to_string(),
            file: "a.vhd".to_string(),
            line: 2,
            in_entity: "ent".to_string(),
            ..Default::default()
        });
        input.processes.push(Process {
            read_signals: vec!["sig".to_string()],
            file: "a.vhd".to_string(),
            ..Default::default()
        });
        let usage = SignalUsageIndex::from_input(&input);
        let v = undriven_signal(&input, &usage);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "undriven_signal");
    }

    #[test]
    fn multi_driven_signal_flags() {
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "ent".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        input.architectures.push(Architecture {
            name: "rtl".to_string(),
            entity_name: "ent".to_string(),
            file: "a.vhd".to_string(),
            line: 2,
        });
        input.signals.push(Signal {
            name: "sig".to_string(),
            r#type: "bit".to_string(),
            file: "a.vhd".to_string(),
            line: 3,
            in_entity: "ent".to_string(),
            ..Default::default()
        });
        input.processes.push(Process {
            assigned_signals: vec!["sig".to_string()],
            in_arch: "rtl".to_string(),
            file: "a.vhd".to_string(),
            ..Default::default()
        });
        input.processes.push(Process {
            assigned_signals: vec!["sig".to_string()],
            in_arch: "rtl".to_string(),
            file: "a.vhd".to_string(),
            ..Default::default()
        });
        let v = multi_driven_signal(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "multi_driven_signal");
    }

    #[test]
    fn multi_driven_signal_ignores_resolved_type() {
        let mut input = Input::default();
        input.entities.push(Entity {
            name: "ent".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        input.architectures.push(Architecture {
            name: "rtl".to_string(),
            entity_name: "ent".to_string(),
            file: "a.vhd".to_string(),
            line: 2,
        });
        input.signals.push(Signal {
            name: "bus".to_string(),
            r#type: "std_logic".to_string(),
            file: "a.vhd".to_string(),
            line: 3,
            in_entity: "ent".to_string(),
            ..Default::default()
        });
        input.processes.push(Process {
            assigned_signals: vec!["bus".to_string()],
            in_arch: "rtl".to_string(),
            file: "a.vhd".to_string(),
            ..Default::default()
        });
        input.processes.push(Process {
            assigned_signals: vec!["bus".to_string()],
            in_arch: "rtl".to_string(),
            file: "a.vhd".to_string(),
            ..Default::default()
        });
        let v = multi_driven_signal(&input);
        assert!(v.is_empty());
    }

    #[test]
    fn undeclared_signal_usage_flags() {
        let mut input = Input::default();
        input.processes.push(Process {
            read_signals: vec!["mystery".to_string()],
            file: "a.vhd".to_string(),
            line: 10,
            in_arch: "rtl".to_string(),
            ..Default::default()
        });
        let usage = SignalUsageIndex::from_input(&input);
        let v = undeclared_signal_usage(&input, &usage);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "undeclared_signal_usage");
    }

    #[test]
    fn input_port_driven_flags() {
        let mut input = Input::default();
        input.architectures.push(Architecture {
            name: "rtl".to_string(),
            entity_name: "ent".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
        });
        input.ports.push(Port {
            name: "in_sig".to_string(),
            direction: "in".to_string(),
            in_entity: "ent".to_string(),
            line: 2,
            ..Default::default()
        });
        input.processes.push(Process {
            label: "p1".to_string(),
            assigned_signals: vec!["in_sig".to_string()],
            file: "a.vhd".to_string(),
            line: 3,
            in_arch: "rtl".to_string(),
            ..Default::default()
        });
        let v = input_port_driven(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "input_port_driven");
    }

    #[test]
    fn wide_signal_flags() {
        let mut input = Input::default();
        input.signals.push(Signal {
            name: "bus".to_string(),
            r#type: "std_logic_vector(255 downto 0)".to_string(),
            file: "a.vhd".to_string(),
            line: 4,
            ..Default::default()
        });
        let v = wide_signal(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "wide_signal");
    }

    #[test]
    fn duplicate_signal_name_flags() {
        let mut input = Input::default();
        input.signals.push(Signal {
            name: "data_bus".to_string(),
            in_entity: "ent1".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        input.signals.push(Signal {
            name: "data_bus".to_string(),
            in_entity: "ent2".to_string(),
            file: "b.vhd".to_string(),
            line: 2,
            ..Default::default()
        });
        let v = duplicate_signal_name(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "duplicate_signal_name");
    }
}
