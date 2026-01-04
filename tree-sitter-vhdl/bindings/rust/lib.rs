extern "C" {
    fn tree_sitter_vhdl() -> tree_sitter::Language;
}

pub fn language() -> tree_sitter::Language {
    unsafe { tree_sitter_vhdl() }
}
