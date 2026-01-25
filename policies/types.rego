# Type Safety Rules
# Rules for type usage: signed/unsigned mixing, type conversions
package vhdl.types

import data.vhdl.helpers

# Rule: Mixing signed/unsigned in same architecture
# If both signed and unsigned signals exist, there might be type conversion issues
# Disabled by default - common in practice
mixed_signedness[violation] {
    sig1 := input.signals[i]
    sig2 := input.signals[j]
    i < j
    sig1.in_entity == sig2.in_entity
    helpers.is_signed_type(sig1.type)
    helpers.is_unsigned_type(sig2.type)
    violation := {
        "rule": "mixed_signedness",
        "severity": "info",
        "file": sig1.file,
        "line": sig1.line,
        "message": sprintf("Architecture uses both signed ('%s') and unsigned ('%s') types - ensure proper conversions", [sig1.name, sig2.name])
    }
}

# Disabled by default - too noisy in practice
violations := set()

# Optional violations
optional_violations := mixed_signedness
