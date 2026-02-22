#!/bin/bash

# Function to print a file with a header
print_file() {
    echo "================================================================================"
    echo "FILE: $1"
    echo "================================================================================"
    cat "$1"
    echo ""
    echo ""
}

# Collect all the important source code
(
    print_file "tree-sitter-micro-vhdl/grammar.js"
    print_file "ast.go"
    print_file "extractor.go"
    print_file "semantics.go"
    print_file "mlir.go"
    print_file "main.go"
    print_file "compile.sh"
    print_file "verif-ltl-guide.md"
    print_file "verif-extensions.md"
    print_file "k-induction-ic3.md"
    print_file "examples/vhd/test_assume.vhd"
    print_file "examples/vhd/test_after_reset.vhd"
    print_file "examples/vhd/test_seq.vhd"
    print_file "examples/vhd/test_contract.vhd"
    print_file "examples/vhd/test_formal.vhd"
    print_file "examples/vhd/test_liveness.vhd"
    print_file "liveness-plan.md"
    print_file "next-level-verification-ideas.md"
    print_file "btor2vcd.py"
    print_file "formal_mutation.py"
    print_file "README.md"
) > .paste_buffer.txt

# Try to copy to clipboard if a tool is available
if command -v xclip &> /dev/null; then
    xclip -selection clipboard < .paste_buffer.txt
    echo "Successfully copied the source code to your clipboard using xclip!"
elif command -v xsel &> /dev/null; then
    xsel --clipboard < .paste_buffer.txt
    echo "Successfully copied the source code to your clipboard using xsel!"
elif command -v pbcopy &> /dev/null; then
    pbcopy < .paste_buffer.txt
    echo "Successfully copied the source code to your clipboard using pbcopy!"
else
    echo "Could not find a clipboard utility (xclip, xsel, pbcopy)."
    echo "All the source code has been combined into 'micro-vhdl/.paste_buffer.txt'."
    echo "You can view it or copy it manually from there."
fi
