#!/bin/bash

# VHDL Parse Tree Viewer
# Usage:
#   ./view.sh [file.vhdl]              - View once
#   ./view.sh [file.vhdl] --watch      - Watch mode (auto-refresh on changes)
#   ./view.sh [file.vhdl] --lines N    - Limit tree output to N lines

FILE="test.vhdl"
WATCH_MODE=false
LINES=""

# Parse arguments
while [[ $# -gt 0 ]]; do
    case "$1" in
        --watch)
            WATCH_MODE=true
            shift
            ;;
        --lines)
            LINES="$2"
            shift 2
            ;;
        *)
            FILE="$1"
            shift
            ;;
    esac
done

# Colors
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
NC='\033[0m'

show_tree() {
    clear
    echo -e "${YELLOW}=== Source: $FILE ===${NC}"
    echo ""
    cat -n "$FILE"
    echo ""

    echo -e "${YELLOW}=== Parse Tree ===${NC}"
    echo ""

    # Generate and parse
    cd tree-sitter-vhdl
    npm run build > /dev/null 2>&1

    if [[ -n "$LINES" ]]; then
        npx tree-sitter parse "../$FILE" 2>/dev/null | head -n "$LINES"
    else
        npx tree-sitter parse "../$FILE" 2>/dev/null
    fi
    cd ..

    if [[ "$WATCH_MODE" == true ]]; then
        echo ""
        echo -e "${GREEN}Watching for changes to $FILE and grammar.js... (Ctrl+C to exit)${NC}"
    fi
}

if [[ "$WATCH_MODE" == true ]]; then
    # Check if watch tools are available
    if command -v inotifywait &> /dev/null; then
        # Initial display
        show_tree

        # Watch for changes to the VHDL file or grammar.js
        while true; do
            inotifywait -q -e modify,close_write "$FILE" tree-sitter-vhdl/grammar.js 2>/dev/null
            sleep 0.2  # Small delay to let writes complete
            show_tree
        done
    elif command -v fswatch &> /dev/null; then
        # macOS alternative
        show_tree
        fswatch -o "$FILE" tree-sitter-vhdl/grammar.js | while read; do
            sleep 0.2
            show_tree
        done
    else
        echo -e "${YELLOW}Watch mode requires 'inotifywait' (Linux) or 'fswatch' (macOS)${NC}"
        echo ""
        echo "Install with:"
        echo "  Ubuntu/Debian: sudo apt install inotify-tools"
        echo "  Fedora:        sudo dnf install inotify-tools"
        echo "  macOS:         brew install fswatch"
        echo ""
        echo "Showing tree once instead:"
        echo ""
        show_tree
    fi
else
    # Single run
    show_tree
fi
