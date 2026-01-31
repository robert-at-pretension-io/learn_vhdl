use crate::policy::helpers::{is_signed_type, is_unsigned_type};
use crate::policy::input::Input;
use crate::policy::result::Violation;

pub fn violations(_input: &Input) -> Vec<Violation> {
    Vec::new()
}

pub fn optional_violations(input: &Input) -> Vec<Violation> {
    mixed_signedness(input)
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
    use crate::policy::input::{Input, Signal};

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
