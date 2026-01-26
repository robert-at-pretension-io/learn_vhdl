# Shared Helper Functions for VHDL Compliance Rules
package vhdl.helpers

# =============================================================================
# POLICY PHILOSOPHY: FIX AT THE SOURCE, NOT HERE
# =============================================================================
#
# Before adding a workaround here, ask: "Is this a grammar or extractor bug?"
#
# WRONG: Grammar can't parse `rec.field <= '1';`, keywords leak into signals
#        -> Add keyword to is_skip_name() list below
#
# RIGHT: Fix grammar.js to parse selected_name in concurrent assignments
#        -> ERROR nodes disappear, keywords never appear in signal lists
#
# The helpers here should filter LEGITIMATE false positives (enum literals,
# constants, loop variables), NOT work around parsing/extraction bugs.
#
# If you find yourself adding VHDL keywords here, STOP and fix the grammar.
#
# See: AGENTS.md "The Grammar Improvement Cycle"
# =============================================================================

# =============================================================================
# Name Classification Helpers
# =============================================================================

# Check if name looks like a clock (stricter check)
is_clock_name(name) {
    lower(name) == "clk"
}
is_clock_name(name) {
    lower(name) == "clock"
}
is_clock_name(name) {
    endswith(lower(name), "_clk")
}
is_clock_name(name) {
    startswith(lower(name), "clk_")
}
is_clock_name(name) {
    endswith(lower(name), "_clock")
}

# Check if name looks like a reset
is_reset_name(name) {
    contains(lower(name), "rst")
}
is_reset_name(name) {
    contains(lower(name), "reset")
}

# Check if type is single-bit
is_single_bit_type(t) {
    lower(t) == "std_logic"
}
is_single_bit_type(t) {
    lower(t) == "std_ulogic"
}
is_single_bit_type(t) {
    lower(t) == "bit"
}

# Common signal names that are expected to be duplicated
is_common_signal_name(name) {
    common := {"clk", "rst", "reset", "data", "addr", "we", "re", "en", "valid", "ready", "ack", "done", "start", "busy", "error"}
    common[lower(name)]
}

# =============================================================================
# Skip Name Helpers (for sensitivity list analysis)
# =============================================================================

# Skip names that shouldn't trigger sensitivity list warnings
is_skip_name(name) {
    startswith(lower(name), "c_")  # c_constant
}
is_skip_name(name) {
    endswith(lower(name), "_c")  # constant_c
}
is_skip_name(name) {
    endswith(lower(name), "_v")  # variable_v
}
is_skip_name(name) {
    endswith(lower(name), "_f")  # function_f
}
is_skip_name(name) {
    endswith(lower(name), "_g")  # generic_g
}
is_skip_name(name) {
    startswith(lower(name), "g_")  # g_generic
}
is_skip_name(name) {
    name == upper(name)
    count(name) > 1
}
is_skip_name(name) {
    # Names starting with uppercase followed by underscore are likely constants/generics
    re_match("^[A-Z]+_", name)
}
is_skip_name(name) {
    # Loop/generate variables
    loop_vars := {"i", "j", "k", "n", "r", "idx", "index", "x", "y"}
    loop_vars[lower(name)]
}
is_skip_name(name) {
    type_names := {"std_logic", "std_ulogic", "std_logic_vector", "std_ulogic_vector",
                   "integer", "natural", "positive", "boolean", "bit", "bit_vector",
                   "signed", "unsigned", "real", "time", "character", "string"}
    type_names[lower(name)]
}
is_skip_name(name) {
    # Common VHDL attributes (extracted from signal'attr)
    attrs := {"left", "right", "high", "low", "range", "reverse_range", "length",
              "ascending", "image", "value", "pos", "val", "succ", "pred", "leftof",
              "rightof", "event", "active", "last_event", "last_active", "last_value",
              "driving", "driving_value", "simple_name", "instance_name", "path_name"}
    attrs[lower(name)]
}
is_skip_name(name) {
    # Common ieee function names and type conversion functions
    func_names := {"to_integer", "to_unsigned", "to_signed", "to_stdulogicvector",
                   "to_stdlogicvector", "to_bitvector", "to_bit", "to_stdulogic",
                   "to_stdlogic", "std_logic_vector", "conv_integer", "conv_unsigned",
                   "conv_signed", "conv_std_logic_vector", "resize", "shift_left",
                   "shift_right", "rotate_left", "rotate_right", "rising_edge",
                   "falling_edge", "now", "or_reduce", "and_reduce", "xor_reduce",
                   "nand_reduce", "nor_reduce", "xnor_reduce", "minimum", "maximum"}
    func_names[lower(name)]
}
is_skip_name(name) {
    # VHDL keywords that might be incorrectly extracted as signal reads
    keywords := {"downto", "to", "range", "others", "all", "open", "null", "true", "false",
                 "and", "or", "not", "xor", "nand", "nor", "xnor", "mod", "rem", "abs",
                 "sll", "srl", "sla", "sra", "rol", "ror", "is", "of", "if", "then", "else",
                 "elsif", "end", "when", "loop", "for", "while", "next", "exit", "return",
                 "case", "select", "with", "after", "transport", "inertial", "reject",
                 "generate", "assert", "report", "severity", "wait", "until"}
    keywords[lower(name)]
}

# =============================================================================
# Sensitivity List Helpers
# =============================================================================

# Check if sensitivity list has 'all'
has_all_sensitivity(sens_list) {
    s := sens_list[_]
    lower(s) == "all"
}

# Check if signal is in sensitivity list (with hierarchical name support)
sig_in_sensitivity(sig, sens_list) {
    s := sens_list[_]
    lower(s) == lower(sig)
}
sig_in_sensitivity(sig, sens_list) {
    # Parent in list: sensitivity has "ctrl_i", read is "ctrl_i.foo" -> OK
    s := sens_list[_]
    startswith(lower(sig), concat("", [lower(s), "."]))
}
sig_in_sensitivity(sig, sens_list) {
    # Child in list: sensitivity has "ctrl_i.foo", read is "ctrl_i" -> OK
    s := sens_list[_]
    startswith(lower(s), concat("", [lower(sig), "."]))
}

# Check if signal is in read signals (with hierarchical name support)
sig_in_reads(sig, read_list) {
    r := read_list[_]
    lower(r) == lower(sig)
}
sig_in_reads(sig, read_list) {
    # Child read: sensitivity has "ctrl_i", read has "ctrl_i.foo" -> sens is used
    r := read_list[_]
    startswith(lower(r), concat("", [lower(sig), "."]))
}
sig_in_reads(sig, read_list) {
    # Parent read: sensitivity has "ctrl_i.foo", read has "ctrl_i" -> sens might be used
    r := read_list[_]
    startswith(lower(sig), concat("", [lower(r), "."]))
}

# =============================================================================
# Type Helpers
# =============================================================================

is_signed_type(t) {
    contains(lower(t), "signed")
    not contains(lower(t), "unsigned")
}

is_unsigned_type(t) {
    contains(lower(t), "unsigned")
}

# Standard architecture names
is_standard_arch_name(name) {
    standard := {"rtl", "behavioral", "behavioural", "structural", "dataflow", "testbench", "tb", "sim"}
    standard[lower(name)]
}

# Valid instance prefixes
valid_instance_prefix(name) {
    startswith(lower(name), "u_")
}
valid_instance_prefix(name) {
    startswith(lower(name), "i_")
}
valid_instance_prefix(name) {
    startswith(lower(name), "inst_")
}

# =============================================================================
# Counter/FSM Name Helpers
# =============================================================================

# Check if name looks like a counter
is_counter_name(name) {
    counter_patterns := ["count", "cnt", "counter", "timer", "tick"]
    pattern := counter_patterns[_]
    contains(lower(name), pattern)
}

# Check if name looks like a state signal
is_state_name(name) {
    state_patterns := ["state", "fsm", "mode"]
    pattern := state_patterns[_]
    contains(lower(name), pattern)
}

# =============================================================================
# Generic Signal List Helpers
# =============================================================================

# Check if signal is in a list (case-insensitive)
signal_in_list(sig, list) {
    item := list[_]
    lower(item) == lower(sig)
}

# =============================================================================
# Type System Filtering (Enum Literals / Constants)
# =============================================================================
# These helpers filter out false positives by checking if a name is actually
# an enum literal or constant rather than a signal.

# Check if name is an enum literal (extracted from type declarations)
is_enum_literal(name) {
    lit := input.enum_literals[_]
    lower(lit) == lower(name)
}

# Check if name is a constant (extracted from constant declarations)
is_constant(name) {
    c := input.constants[_]
    lower(c) == lower(name)
}

# Check if name is an actual signal (not enum/constant/keyword/function)
is_actual_signal(name) {
    not is_enum_literal(name)
    not is_constant(name)
    not is_skip_name(name)
}

# =============================================================================
# Lint Configuration Helpers
# =============================================================================
# These helpers check lint_config passed from Go to filter/adjust violations

# Check if a rule is disabled (severity == "off")
rule_is_disabled(rule_name) {
    input.lint_config.rules[rule_name] == "off"
}

# Get the configured severity for a rule (returns null if not configured)
# The aggregator will use the rule's original severity if null is returned
get_rule_severity(rule_name) := severity {
    severity := input.lint_config.rules[rule_name]
    severity != null
} else := null

# Check if rule is enabled (not "off")
rule_is_enabled(rule_name) {
    not rule_is_disabled(rule_name)
}

# =============================================================================
# Third-Party File Filtering
# =============================================================================
# Third-party libraries should have warnings suppressed

# Check if a file is from a third-party library
is_third_party_file(file) {
    tp_file := input.third_party_files[_]
    file == tp_file
}

# Check if a file is from a third-party library (also match by filename ending)
is_third_party_file(file) {
    tp_file := input.third_party_files[_]
    endswith(file, tp_file)
}
