#!/bin/bash
# Test grammar against external test files

PASS=0
FAIL=0
TOTAL=0

# Test against a subset or all files
TEST_DIR="${1:-external_tests}"
MAX_FILES="${2:-0}"  # 0 = all files

echo "Testing against files in: $TEST_DIR"

for f in $(find "$TEST_DIR" -name "*.vhd" -o -name "*.vhdl" 2>/dev/null | sort); do
    TOTAL=$((TOTAL + 1))
    
    if [ "$MAX_FILES" -gt 0 ] && [ "$TOTAL" -gt "$MAX_FILES" ]; then
        TOTAL=$((TOTAL - 1))
        break
    fi
    
    if ./target/release/vhdl-compiler "$f" > /dev/null 2>&1; then
        PASS=$((PASS + 1))
    else
        FAIL=$((FAIL + 1))
    fi
    
    # Progress indicator every 100 files
    if [ $((TOTAL % 500)) -eq 0 ]; then
        echo "Progress: $TOTAL files tested ($PASS pass, $FAIL fail)"
    fi
done

PERCENT=$(echo "scale=2; $PASS * 100 / $TOTAL" | bc)
echo ""
echo "=== Results ==="
echo "Total:  $TOTAL"
echo "Pass:   $PASS"
echo "Fail:   $FAIL"
echo "Score:  ${PERCENT}%"
