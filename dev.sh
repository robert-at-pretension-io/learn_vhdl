#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

build_and_run() {
    clear
    echo -e "${YELLOW}=== Generating grammar ===${NC}"
    cd tree-sitter-vhdl
    npm run build 2>&1
    cd ..

    echo ""
    echo -e "${YELLOW}=== Building parser ===${NC}"
    cargo build 2>&1

    echo ""
    echo -e "${YELLOW}=== Parsing ${1:-test.vhdl} ===${NC}"
    cargo run -- "${1:-test.vhdl}" 2>&1 || true
    
    echo ""
    echo -e "${GREEN}=== Waiting for changes to grammar.js ===${NC}"
}

# Check if we should run in watch mode
if [[ "$1" == "--once" ]]; then
    # Single run mode (old behavior)
    shift
    echo "=== Generating grammar ==="
    cd tree-sitter-vhdl
    npm run build
    cd ..

    echo ""
    echo "=== Building parser ==="
    cargo build

    echo ""
    echo "=== Parsing ${1:-test.vhdl} ==="
    cargo run -- "${1:-test.vhdl}"
else
    # Watch mode (default)
    TEST_FILE="${1:-test.vhdl}"
    
    # Check if inotifywait is available
    if command -v inotifywait &> /dev/null; then
        # Initial build
        build_and_run "$TEST_FILE"
        
        # Watch for changes
        while true; do
            inotifywait -q -e modify,close_write tree-sitter-vhdl/grammar.js "$TEST_FILE" 2>/dev/null
            sleep 0.1  # Small delay to let file writes complete
            build_and_run "$TEST_FILE"
        done
    elif command -v fswatch &> /dev/null; then
        # macOS alternative using fswatch
        # Initial build
        build_and_run "$TEST_FILE"
        
        # Watch for changes
        fswatch -o tree-sitter-vhdl/grammar.js "$TEST_FILE" | while read; do
            sleep 0.1
            build_and_run "$TEST_FILE"
        done
    else
        echo -e "${RED}Watch mode requires 'inotifywait' (Linux) or 'fswatch' (macOS)${NC}"
        echo ""
        echo "Install with:"
        echo "  Ubuntu/Debian: sudo apt install inotify-tools"
        echo "  Fedora:        sudo dnf install inotify-tools"
        echo "  macOS:         brew install fswatch"
        echo ""
        echo "Or run with --once flag for single execution:"
        echo "  ./dev.sh --once [test_file.vhdl]"
        exit 1
    fi
fi
