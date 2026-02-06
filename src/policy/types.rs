use crate::policy::helpers::{is_signed_type, is_unsigned_type};
use crate::policy::input::Input;
use crate::policy::result::Violation;

pub fn violations(_input: &Input) -> Vec<Violation> {
    Vec::new()
}

pub fn optional_violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(mixed_signedness(input));
    out.extend(numeric_std_unsigned_mixing(input));
    out
}

fn numeric_std_unsigned_mixing(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    let mut file_imports: std::collections::HashMap<String, (bool, bool, usize)> =
        std::collections::HashMap::new();

    for uc in &input.use_clauses {
        for item in &uc.items {
            let lower = item.to_ascii_lowercase();
            let entry = file_imports
                .entry(uc.file.clone())
                .or_insert((false, false, uc.line));
            if lower.contains("numeric_std") {
                entry.0 = true;
            }
            if lower.contains("std_logic_unsigned") || lower.contains("std_logic_signed") {
                entry.1 = true;
                entry.2 = uc.line;
            }
        }
    }

    for (file, (has_numeric, has_legacy, line)) in &file_imports {
        if *has_numeric && *has_legacy {
            out.push(Violation {
                rule: "numeric_std_unsigned_mixing".to_string(),
                severity: "warning".to_string(),
                file: file.clone(),
                line: *line,
                message: "File imports both numeric_std and std_logic_unsigned/signed - conflicting operator overloads".to_string(),
            });
        }
    }
    out
}

fn mixed_signedness(input: &Input) -> Vec<Violation> {
    let mut violations = Vec::new();
    let signals = &input.signals;
    for i in 0..signals.len() {
        for j in (i + 1)..signals.len() {
            let s1 = &signals[i];
            let s2 = &signals[j];
            if s1.in_entity != s2.in_entity {
                continue;
            }
            if is_signed_type(&s1.r#type) && is_unsigned_type(&s2.r#type) {
                violations.push(Violation {
                    rule: "mixed_signedness".to_string(),
                    severity: "info".to_string(),
                    file: s1.file.clone(),
                    line: s1.line,
                    message: format!(
                        "Architecture uses both signed ('{}') and unsigned ('{}') types - ensure proper conversions",
                        s1.name, s2.name
                    ),
                });
            }
        }
    }
    violations
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::policy::input::{Input, Signal, UseClause};

    #[test]
    fn mixed_signedness_flags_pair() {
        let mut input = Input::default();
        input.signals.push(Signal {
            name: "a".to_string(),
            r#type: "signed".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            in_entity: "core".to_string(),
            ..Default::default()
        });
        input.signals.push(Signal {
            name: "b".to_string(),
            r#type: "unsigned".to_string(),
            file: "a.vhd".to_string(),
            line: 2,
            in_entity: "core".to_string(),
            ..Default::default()
        });
        let violations = optional_violations(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "mixed_signedness");
    }

    #[test]
    fn numeric_std_unsigned_mixing_flags() {
        let mut input = Input::default();
        input.use_clauses.push(UseClause {
            items: vec!["ieee.numeric_std.all".to_string()],
            file: "a.vhd".to_string(),
            line: 1,
        });
        input.use_clauses.push(UseClause {
            items: vec!["ieee.std_logic_unsigned.all".to_string()],
            file: "a.vhd".to_string(),
            line: 2,
        });
        let violations = numeric_std_unsigned_mixing(&input);
        assert_eq!(violations.len(), 1);
        assert_eq!(violations[0].rule, "numeric_std_unsigned_mixing");
    }

    #[test]
    fn numeric_std_unsigned_mixing_ignores_separate_files() {
        let mut input = Input::default();
        input.use_clauses.push(UseClause {
            items: vec!["ieee.numeric_std.all".to_string()],
            file: "a.vhd".to_string(),
            line: 1,
        });
        input.use_clauses.push(UseClause {
            items: vec!["ieee.std_logic_unsigned.all".to_string()],
            file: "b.vhd".to_string(),
            line: 2,
        });
        let violations = numeric_std_unsigned_mixing(&input);
        assert!(violations.is_empty());
    }

    #[test]
    fn mixed_signedness_ignores_different_entities() {
        let mut input = Input::default();
        input.signals.push(Signal {
            name: "a".to_string(),
            r#type: "signed".to_string(),
            file: "a.vhd".to_string(),
            line: 1,
            in_entity: "core".to_string(),
            ..Default::default()
        });
        input.signals.push(Signal {
            name: "b".to_string(),
            r#type: "unsigned".to_string(),
            file: "a.vhd".to_string(),
            line: 2,
            in_entity: "other".to_string(),
            ..Default::default()
        });
        let violations = optional_violations(&input);
        assert!(violations.is_empty());
    }
}
