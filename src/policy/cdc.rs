use crate::policy::input::Input;
use crate::policy::result::Violation;

pub fn violations(input: &Input) -> Vec<Violation> {
    let mut out = Vec::new();
    out.extend(cdc_unsync_single_bit(input));
    out.extend(cdc_unsync_multi_bit(input));
    out.extend(cdc_insufficient_sync(input));
    out
}

fn cdc_unsync_single_bit(input: &Input) -> Vec<Violation> {
    input
        .cdc_crossings
        .iter()
        .filter(|cdc| !cdc.is_synchronized && !cdc.is_multi_bit)
        .map(|cdc| Violation {
            rule: "cdc_unsync_single_bit".to_string(),
            severity: "warning".to_string(),
            file: cdc.file.clone(),
            line: cdc.line,
            message: format!(
                "Signal '{}' crosses from {} to {} clock domain without synchronizer",
                cdc.signal, cdc.source_clock, cdc.dest_clock
            ),
        })
        .collect()
}

fn cdc_unsync_multi_bit(input: &Input) -> Vec<Violation> {
    input
        .cdc_crossings
        .iter()
        .filter(|cdc| !cdc.is_synchronized && cdc.is_multi_bit)
        .map(|cdc| Violation {
            rule: "cdc_unsync_multi_bit".to_string(),
            severity: "error".to_string(),
            file: cdc.file.clone(),
            line: cdc.line,
            message: format!(
                "Multi-bit signal '{}' crosses from {} to {} clock domain - requires handshaking or Gray code",
                cdc.signal, cdc.source_clock, cdc.dest_clock
            ),
        })
        .collect()
}

fn cdc_insufficient_sync(input: &Input) -> Vec<Violation> {
    input
        .cdc_crossings
        .iter()
        .filter(|cdc| cdc.is_synchronized && cdc.sync_stages < 2)
        .map(|cdc| Violation {
            rule: "cdc_insufficient_sync".to_string(),
            severity: "warning".to_string(),
            file: cdc.file.clone(),
            line: cdc.line,
            message: format!(
                "Signal '{}' has only {} synchronizer stage(s), recommend 2+",
                cdc.signal, cdc.sync_stages
            ),
        })
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::policy::input::{CDCCrossing, Input};

    #[test]
    fn cdc_unsync_single_bit_flags() {
        let mut input = Input::default();
        input.cdc_crossings.push(CDCCrossing {
            signal: "sig".to_string(),
            source_clock: "clk_a".to_string(),
            dest_clock: "clk_b".to_string(),
            is_synchronized: false,
            is_multi_bit: false,
            file: "a.vhd".to_string(),
            line: 1,
            ..Default::default()
        });
        let v = cdc_unsync_single_bit(&input);
        assert_eq!(v.len(), 1);
        assert_eq!(v[0].rule, "cdc_unsync_single_bit");
    }
}
