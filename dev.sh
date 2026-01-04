#!/bin/bash
set -e

echo "=== Generating grammar ==="
cd tree-sitter-vhdl
npm run build
cd ..

echo ""
echo "=== Building parser ==="
cargo build

echo ""
echo "=== Parsing test.vhdl ==="
cargo run -- "${1:-test.vhdl}"
