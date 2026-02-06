package extractor

// =============================================================================
// CONSOLIDATED VHDL BUILT-IN SETS
// =============================================================================
// Centralizes hardcoded VHDL keyword/function/attribute lists that were
// previously scattered across switch statements in extractor.go.
// Each set is a map[string]bool for O(1) lookup.

// VHDLKeywords contains VHDL reserved words used to filter out keywords
// in range extraction and similar contexts.
var VHDLKeywords = map[string]bool{
	// Control flow
	"if": true, "then": true, "else": true, "elsif": true,
	"for": true, "in": true, "loop": true, "while": true,
	"case": true, "when": true, "of": true, "with": true, "select": true,
	"return": true, "exit": true, "next": true,
	// Block structure
	"begin": true, "end": true, "is": true,
	"generate": true, "block": true,
	"process": true, "function": true, "procedure": true,
	"package": true, "entity": true, "architecture": true,
	"component": true, "configuration": true,
	"protected": true,
	// Declarations
	"signal": true, "variable": true, "constant": true,
	"type": true, "subtype": true, "alias": true,
	"array": true, "record": true, "range": true,
	"port": true, "generic": true, "map": true,
	"use": true, "library": true, "all": true,
	"file": true, "access": true,
	// Values / misc
	"others": true, "null": true, "open": true,
	"downto": true, "to": true,
	// Concurrent / assertion
	"assert": true, "report": true, "severity": true, "wait": true,
	"force": true, "release": true,
	// Logical / arithmetic operators
	"and": true, "or": true, "not": true, "xor": true,
	"nand": true, "nor": true, "xnor": true,
	"abs": true, "mod": true, "rem": true,
	"sll": true, "srl": true, "sla": true, "sra": true,
	"rol": true, "ror": true,
}

// CommonVHDLFunctions contains standard VHDL functions and type conversions.
// Used to distinguish function calls from signal reads in indexed_name prefixes.
var CommonVHDLFunctions = map[string]bool{
	// Type conversions
	"unsigned": true, "signed": true,
	"std_logic_vector": true, "std_ulogic_vector": true,
	"integer": true, "natural": true, "positive": true, "real": true,
	// numeric_std
	"to_unsigned": true, "to_signed": true, "to_integer": true, "to_real": true,
	"resize": true, "shift_left": true, "shift_right": true,
	"rotate_left": true, "rotate_right": true,
	// synopsys / legacy conversions
	"conv_integer": true, "conv_unsigned": true,
	"conv_signed": true, "conv_std_logic_vector": true,
	// Timing / edge
	"rising_edge": true, "falling_edge": true, "now": true, "time": true,
	// Fixed / float point
	"to_ufixed": true, "to_sfixed": true, "to_float": true,
	"to_slv": true, "to_stdlogicvector": true, "to_stdulogicvector": true,
	"to_bitvector": true, "to_bit": true, "to_01": true,
	// Comparison / query
	"std_match": true, "is_x": true,
	"minimum": true, "maximum": true,
}

// StaticAttributes contains VHDL predefined attributes that are compile-time
// constants or type properties (i.e. not signal-valued).
var StaticAttributes = map[string]bool{
	// Range / bound attributes
	"left": true, "right": true, "high": true, "low": true,
	"range": true, "reverse_range": true, "length": true, "ascending": true,
	// Scalar type attributes
	"image": true, "value": true, "pos": true, "val": true,
	"succ": true, "pred": true, "leftof": true, "rightof": true,
	// Object attributes
	"driving": true, "driving_value": true,
	// Name attributes
	"simple_name": true, "instance_name": true, "path_name": true,
	// Type attributes
	"base": true, "subtype": true, "element": true,
	// Signal attributes (return signals, but treated as static for extraction)
	"delayed": true, "stable": true, "quiet": true, "transaction": true,
	// Signal query attributes
	"active": true, "last_event": true, "last_active": true, "last_value": true,
	"event": true,
}

// CallOperatorNames contains VHDL operator names that can appear as
// function call names (operator overloading). These should not be treated
// as regular identifiers.
var CallOperatorNames = map[string]bool{
	"and": true, "or": true, "xor": true, "xnor": true,
	"nand": true, "nor": true, "not": true,
	"mod": true, "rem": true,
	"sll": true, "srl": true, "sla": true, "sra": true,
	"rol": true, "ror": true, "abs": true,
}

// SingleBitTypes contains VHDL types that are known to be exactly 1 bit wide.
var SingleBitTypes = map[string]bool{
	"std_logic":  true,
	"std_ulogic": true,
	"bit":        true,
	"boolean":    true,
}
