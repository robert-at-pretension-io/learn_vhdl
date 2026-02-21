package micro_vhdl

//#cgo CFLAGS: -std=c11 -fPIC
//#include "src/parser.c"
import "C"
import "unsafe"

// Language returns the tree-sitter Language for this grammar.
func Language() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_micro_vhdl())
}