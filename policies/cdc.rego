# Clock Domain Crossing (CDC) Detection Rules
package vhdl.cdc

# =============================================================================
# CDC VIOLATION RULES
# =============================================================================
#
# CDC crossings are a common source of metastability bugs in digital designs.
# These rules detect potential CDC issues:
# 1. Unsynchronized single-bit crossings (warning - needs synchronizer)
# 2. Unsynchronized multi-bit crossings (error - needs special handling)
# 3. Insufficient synchronizer stages (warning - recommend 2+ stages)
#
# Note: The extractor already detects synchronizers, so these rules focus on
# flagging crossings that lack proper synchronization.
# =============================================================================

# Rule: Unsynchronized single-bit CDC crossing
# Single-bit signals crossing clock domains without a synchronizer
cdc_unsync_single_bit[violation] {
    cdc := input.cdc_crossings[_]
    not cdc.is_synchronized
    not cdc.is_multi_bit

    violation := {
        "rule": "cdc_unsync_single_bit",
        "severity": "warning",
        "file": cdc.file,
        "line": cdc.line,
        "message": sprintf("Signal '%s' crosses from %s to %s clock domain without synchronizer",
            [cdc.signal, cdc.source_clock, cdc.dest_clock])
    }
}

# Rule: Unsynchronized multi-bit CDC crossing
# Multi-bit signals crossing clock domains without proper handling
# This is more serious than single-bit - needs handshaking or Gray coding
cdc_unsync_multi_bit[violation] {
    cdc := input.cdc_crossings[_]
    not cdc.is_synchronized
    cdc.is_multi_bit

    violation := {
        "rule": "cdc_unsync_multi_bit",
        "severity": "error",
        "file": cdc.file,
        "line": cdc.line,
        "message": sprintf("Multi-bit signal '%s' crosses from %s to %s clock domain - requires handshaking or Gray code",
            [cdc.signal, cdc.source_clock, cdc.dest_clock])
    }
}

# Rule: Insufficient synchronizer stages
# When a synchronizer is detected but has fewer than 2 stages
cdc_insufficient_sync[violation] {
    cdc := input.cdc_crossings[_]
    cdc.is_synchronized
    cdc.sync_stages < 2

    violation := {
        "rule": "cdc_insufficient_sync",
        "severity": "warning",
        "file": cdc.file,
        "line": cdc.line,
        "message": sprintf("Signal '%s' has only %d synchronizer stage(s), recommend 2+",
            [cdc.signal, cdc.sync_stages])
    }
}

# =============================================================================
# AGGREGATE ALL CDC VIOLATIONS
# =============================================================================

cdc_violations[v] {
    cdc_unsync_single_bit[v]
}

cdc_violations[v] {
    cdc_unsync_multi_bit[v]
}

cdc_violations[v] {
    cdc_insufficient_sync[v]
}
