# Policy Aggregator
# Combines violations from all policy modules into a single result
package vhdl.compliance

import data.vhdl.core
import data.vhdl.sensitivity
import data.vhdl.clocks_resets
import data.vhdl.signals
import data.vhdl.ports
import data.vhdl.instances
import data.vhdl.style
import data.vhdl.naming
import data.vhdl.processes
import data.vhdl.types
import data.vhdl.fsm
import data.vhdl.combinational
import data.vhdl.sequential
import data.vhdl.hierarchy
import data.vhdl.quality
import data.vhdl.synthesis
import data.vhdl.testbench
import data.vhdl.latch
import data.vhdl.cdc
# Advanced static analysis modules
import data.vhdl.security
import data.vhdl.rdc
import data.vhdl.power
import data.vhdl.helpers

# =============================================================================
# Aggregate all violations from policy modules
# =============================================================================

# Core rules (always enabled) - entity, architecture, component, dependency
core_violations := core.violations

# Sensitivity list rules (always enabled)
sensitivity_violations := sensitivity.violations

# Clock/reset rules (always enabled)
clock_reset_violations := clocks_resets.violations

# Signal analysis rules (enabled - concurrent assignments now tracked)
signal_violations := signals.violations

# Port analysis rules (enabled - concurrent assignments now tracked)
port_violations := ports.violations

# Instance rules
instance_violations := instances.violations

# Style rules
style_violations := style.violations

# Naming rules (disabled by default - project-specific)
naming_violations := naming.violations

# Process rules
process_violations := processes.violations

# Type rules (disabled by default)
type_violations := types.violations

# FSM rules
fsm_violations := fsm.violations

# Combinational logic rules
combinational_violations := combinational.violations

# Sequential logic rules
sequential_violations := sequential.violations

# Hierarchy rules
hierarchy_violations := hierarchy.violations

# Quality rules
quality_violations := quality.violations

# Synthesis rules
synthesis_violations := synthesis.violations

# Testbench rules
testbench_violations := testbench.violations

# Latch inference detection rules
latch_violations := latch.violations

# Clock Domain Crossing rules
cdc_violations := cdc.cdc_violations

# =============================================================================
# Advanced Static Analysis ("God-Tier" Rules)
# =============================================================================

# Security rules (hardware trojan detection)
security_violations := security.violations

# Reset Domain Crossing rules
rdc_violations := rdc.violations

# Power analysis rules (operand isolation)
power_violations := power.violations

# =============================================================================
# All enabled violations (filtered by lint config and third-party files)
# =============================================================================

# Raw violations from all modules before filtering
raw_violations := core_violations | sensitivity_violations | clock_reset_violations | signal_violations | port_violations | instance_violations | style_violations | naming_violations | process_violations | type_violations | fsm_violations | combinational_violations | sequential_violations | hierarchy_violations | quality_violations | synthesis_violations | testbench_violations | latch_violations | cdc_violations | security_violations | rdc_violations | power_violations

# Filter out:
# 1. Violations for rules that are disabled (severity == "off")
# 2. Violations from third-party files
# 3. Apply configured severity overrides
all_violations[filtered] {
    v := raw_violations[_]

    # Skip if rule is disabled
    not helpers.rule_is_disabled(v.rule)

    # Skip if from third-party file
    not helpers.is_third_party_file(v.file)

    # Apply configured severity (or keep original if not configured)
    configured_severity := helpers.get_rule_severity(v.rule)
    final_severity := severity_or_default(configured_severity, v.severity)

    # Build filtered violation with possibly adjusted severity
    filtered := {
        "rule": v.rule,
        "severity": final_severity,
        "file": v.file,
        "line": v.line,
        "message": v.message
    }
}

# Helper to use configured severity if valid, otherwise keep original
severity_or_default(configured, original) := configured {
    configured != null
    configured != ""
    # Only use configured if it's a valid severity
    valid_severities := {"error", "warning", "info"}
    valid_severities[configured]
} else := original

# =============================================================================
# Summary statistics
# =============================================================================

summary := {
    "total_violations": count(all_violations),
    "errors": count([v | v := all_violations[_]; v.severity == "error"]),
    "warnings": count([v | v := all_violations[_]; v.severity == "warning"]),
    "info": count([v | v := all_violations[_]; v.severity == "info"]),
    "by_category": {
        "core": count(core_violations),
        "sensitivity": count(sensitivity_violations),
        "clocks_resets": count(clock_reset_violations),
        "signals": count(signal_violations),
        "ports": count(port_violations),
        "instances": count(instance_violations),
        "style": count(style_violations),
        "naming": count(naming_violations),
        "processes": count(process_violations),
        "types": count(type_violations),
        "fsm": count(fsm_violations),
        "combinational": count(combinational_violations),
        "sequential": count(sequential_violations),
        "hierarchy": count(hierarchy_violations),
        "quality": count(quality_violations),
        "synthesis": count(synthesis_violations),
        "testbench": count(testbench_violations),
        "latch": count(latch_violations),
        "cdc": count(cdc_violations),
        # Advanced static analysis
        "security": count(security_violations),
        "rdc": count(rdc_violations),
        "power": count(power_violations)
    }
}

# =============================================================================
# Optional rules (can be enabled per-project)
# =============================================================================

# To enable optional rules, create a custom aggregator:
#
#   package myproject.compliance
#   import data.vhdl.compliance
#   import data.vhdl.naming
#   import data.vhdl.quality
#
#   all_violations := compliance.all_violations | naming.optional_violations | quality.optional_violations

optional_rules := {
    # Naming conventions
    "naming.optional_violations": [
        "entity_naming - Entity should use lowercase",
        "signal_input_naming - Input ports should end with _i",
        "signal_output_naming - Output ports should end with _o",
        "active_low_naming - Active-low signals should end with _n"
    ],
    # Clock/reset conventions
    "clocks_resets.optional_violations": [
        "async_reset_active_high - Reset should be active-low"
    ],
    # Instance conventions
    "instances.optional_violations": [
        "instance_naming_convention - Instances should use u_/i_/inst_ prefix"
    ],
    # Style preferences
    "style.optional_violations": [
        "process_label_missing - Processes should have labels",
        "architecture_naming_convention - Use rtl/behavioral/structural names",
        "empty_architecture - Architecture has no content"
    ],
    # Signal analysis
    "signals.optional_violations": [
        "wide_signal - Very wide signals (>128 bits)",
        "duplicate_signal_name - Same signal name in different entities"
    ],
    # FSM
    "fsm.optional_violations": [
        "single_state_signal - FSM without next_state pattern"
    ],
    # Combinational
    "combinational.optional_violations": [
        "large_combinational_process - Many signals in combinational process",
        "vhdl2008_sensitivity_all - Uses VHDL-2008 'all' sensitivity",
        "long_sensitivity_list - Consider using 'all'"
    ],
    # Sequential
    "sequential.optional_violations": [
        "missing_reset_sensitivity - Reset not in sensitivity list",
        "very_wide_register - Process assigns many signals",
        "mixed_edge_clocking - Same clock, different edges",
        "async_reset_naming - Reset naming convention"
    ],
    # Hierarchy
    "hierarchy.optional_violations": [
        "sparse_port_map - Few ports connected",
        "instance_name_matches_component - Instance name same as component",
        "repeated_component_instantiation - Same component many times",
        "many_instances - Many instances in architecture",
        "hardcoded_port_value - Literal value on port",
        "open_port_connection - Port connected to 'open'"
    ],
    # Quality
    "quality.optional_violations": [
        "very_long_file - Many design units in file",
        "large_package - Package has many items",
        "short_signal_name - Single character signal name",
        "long_signal_name - Very long signal name",
        "short_port_name - Single character port name",
        "entity_name_with_numbers - Numbers in entity name",
        "mixed_port_directions - Input/output ports interleaved",
        "bidirectional_port - inout port"
    ],
    # Synthesis
    "synthesis.optional_violations": [
        "multiple_clock_domains - Multiple clocks in architecture",
        "very_wide_bus - Bus wider than 64 bits",
        "critical_signal_no_reset - Critical signal without reset",
        "combinational_reset - Reset generated combinationally",
        "potential_memory_inference - Array type may infer RAM"
    ],
    # Testbench
    "testbench.optional_violations": [
        "testbench_with_ports - TB entity has ports",
        "mismatched_tb_architecture - TB arch name mismatch",
        "tb_with_synth_arch - TB with RTL architecture name"
    ],
    # Latch inference detection
    "latch.optional_violations": [
        "combinational_incomplete_assignment - Signal read and written in comb process",
        "conditional_assignment_review - Verify conditional has else clause",
        "selected_assignment_review - Verify selected has when others",
        "combinational_default_values - Suggest default values at process start",
        "fsm_no_reset_state - State signal without reset"
    ],
    # Security (hardware trojan detection)
    "security.optional_violations": [
        "large_literal_comparison - Comparison against >16-bit literal",
        "counter_trigger - Counter compared to large literal",
        "inverted_trigger - /= with large literal"
    ],
    # Clock Domain Crossing
    "cdc.optional_violations": [
        "cdc_unsync_single_bit - Single-bit signal crossing clock domain",
        "cdc_unsync_multi_bit - Multi-bit signal crossing clock domain",
        "cdc_insufficient_sync - Synchronizer has < 2 stages"
    ],
    # Reset Domain Crossing
    "rdc.optional_violations": [
        "async_reset_unsynchronized - Async reset needs synchronizer",
        "partial_reset_domain - Some registers missing reset",
        "short_reset_sync - Single-stage reset synchronizer"
    ],
    # Power analysis
    "power.optional_violations": [
        "unguarded_multiplication - Multiplier without enable",
        "unguarded_exponent - Exponentiation without enable",
        "power_hotspot - Many expensive ops in process",
        "combinational_multiplier - Multiplier in comb process",
        "weak_guard - Guard may not isolate operands",
        "dsp_candidate_no_control - DSP without clock enable",
        "clock_gating_opportunity - Candidate for clock gating"
    ]
}
