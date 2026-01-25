#!/bin/bash
# =============================================================================
# Grammar Testing Script
# =============================================================================
#
# This script measures grammar quality by counting ERROR nodes across test files.
#
# USAGE:
#   ./test_grammar.sh                         # Test all files in external_tests/
#   ./test_grammar.sh external_tests/neorv32  # Test specific project
#   ./test_grammar.sh external_tests 50       # Test first 50 files only
#
# OPTIONAL ANALYSIS:
#   ANALYZE=1 ./test_grammar.sh                     # Summarize ERROR nodes by construct
#   ANALYZE_MAX_FILES=0 ANALYZE=1 ./test_grammar.sh  # Analyze all failing files
#   ANALYZE_SAMPLES=3 ANALYZE=1 ./test_grammar.sh    # Show sample lines per construct
#
# ANTI-TEST FILTERS:
#   Some suites include non-compliant/negative tests that should fail to parse.
#   These are excluded from the primary score by default, but still tracked.
#   EXCLUDE_GLOBS="*/non_compliant/* */analyzer_failure/* */negative/*"
#
# THE IMPROVEMENT CYCLE:
#   1. Run this script to get baseline score
#   2. Find failing files (check for ERROR nodes with npx tree-sitter parse)
#   3. Identify which VHDL construct causes the ERROR
#   4. Fix grammar.js to handle that construct
#   5. npx tree-sitter generate && go build ./cmd/vhdl-lint
#   6. Run this script again - score should improve!
#
# GOAL: Increase pass rate. Each ERROR node fixed improves downstream analysis.
#
# See: AGENTS.md "The Grammar Improvement Cycle" for detailed workflow.
# =============================================================================
# Test grammar against external test files

# Parallelism control (default: number of CPUs). Set THREADS=1 to run serially.
if command -v nproc >/dev/null 2>&1; then
    THREADS="${THREADS:-$(nproc)}"
else
    THREADS="${THREADS:-1}"
fi
if [ "$THREADS" -lt 1 ]; then
    THREADS=1
fi

ANALYZE="${ANALYZE:-0}"
ANALYZE_MAX_FILES="${ANALYZE_MAX_FILES:-200}"
ANALYZE_SAMPLES="${ANALYZE_SAMPLES:-2}"
ANALYZE_TOP="${ANALYZE_TOP:-15}"
ANALYZE_EXPECTED="${ANALYZE_EXPECTED:-0}"
EXCLUDE_GLOBS="${EXCLUDE_GLOBS:-"*/non_compliant/* */analyzer_failure/* */negative/*"}"

PASS=0
FAIL=0
XFAIL=0
XPASS=0
TOTAL=0

# Test against a subset or all files
TEST_DIR="${1:-external_tests}"
MAX_FILES="${2:-0}"  # 0 = all files

ROOT_DIR="$(pwd)"
echo "Testing against files in: $TEST_DIR"

if command -v rg >/dev/null 2>&1; then
    mapfile -t FILES < <(rg --files -g "*.vhd" -g "*.vhdl" "$TEST_DIR" | sort)
else
    mapfile -t FILES < <(find "$TEST_DIR" \( -name "*.vhd" -o -name "*.vhdl" \) 2>/dev/null | sort)
fi

if [ "$MAX_FILES" -gt 0 ]; then
    FILES=("${FILES[@]:0:$MAX_FILES}")
fi

TOTAL=${#FILES[@]}

if [ "$THREADS" -le 1 ]; then
    FAIL_LIST="$(mktemp)"
    XFAIL_LIST="$(mktemp)"
    for f in "${FILES[@]}"; do
        expected=0
        for pat in $EXCLUDE_GLOBS; do
            case "$f" in
                $pat) expected=1; break ;;
            esac
        done
        if ./target/release/vhdl-compiler "$f" > /dev/null 2>&1; then
            if [ "$expected" -eq 1 ]; then
                XPASS=$((XPASS + 1))
            else
                PASS=$((PASS + 1))
            fi
        else
            if [ "$expected" -eq 1 ]; then
                XFAIL=$((XFAIL + 1))
                echo "$f" >> "$XFAIL_LIST"
            else
                FAIL=$((FAIL + 1))
                echo "$f" >> "$FAIL_LIST"
            fi
        fi

        # Progress indicator every 500 files
        if [ $(( (PASS + FAIL + XFAIL + XPASS) % 500 )) -eq 0 ] && [ $((PASS + FAIL + XFAIL + XPASS)) -gt 0 ]; then
            echo "Progress: $((PASS + FAIL + XFAIL + XPASS)) files tested (pass=$PASS fail=$FAIL xfail=$XFAIL xpass=$XPASS)"
        fi
    done
else
    TMP_RESULTS="$(mktemp)"
    printf "%s\n" "${FILES[@]}" | xargs -P "$THREADS" -I{} sh -c '
        path="$1"
        expected=0
        for pat in '"$EXCLUDE_GLOBS"'; do
            case "$path" in
                $pat) expected=1; break ;;
            esac
        done
        if ./target/release/vhdl-compiler "$path" > /dev/null 2>&1; then
            printf "PASS\t%s\t%s\n" "$expected" "$path"
        else
            printf "FAIL\t%s\t%s\n" "$expected" "$path"
        fi
    ' _ {} > "$TMP_RESULTS"

    if command -v rg >/dev/null 2>&1; then
        PASS=$(rg -c "^PASS\t0\t" "$TMP_RESULTS" || true)
        FAIL=$(rg -c "^FAIL\t0\t" "$TMP_RESULTS" || true)
        XFAIL=$(rg -c "^FAIL\t1\t" "$TMP_RESULTS" || true)
        XPASS=$(rg -c "^PASS\t1\t" "$TMP_RESULTS" || true)
    else
        PASS=$(grep -c "^PASS	0	" "$TMP_RESULTS" || true)
        FAIL=$(grep -c "^FAIL	0	" "$TMP_RESULTS" || true)
        XFAIL=$(grep -c "^FAIL	1	" "$TMP_RESULTS" || true)
        XPASS=$(grep -c "^PASS	1	" "$TMP_RESULTS" || true)
    fi

    FAIL_LIST="$(mktemp)"
    XFAIL_LIST="$(mktemp)"
    awk -F'\t' '$1 == "FAIL" && $2 == "0" {print $3}' "$TMP_RESULTS" > "$FAIL_LIST"
    awk -F'\t' '$1 == "FAIL" && $2 == "1" {print $3}' "$TMP_RESULTS" > "$XFAIL_LIST"
    rm -f "$TMP_RESULTS"
fi

PASS="${PASS:-0}"
FAIL="${FAIL:-0}"
XFAIL="${XFAIL:-0}"
XPASS="${XPASS:-0}"

SCORE_TOTAL=$((PASS + FAIL))
if [ "$SCORE_TOTAL" -eq 0 ]; then
    SCORE_TOTAL=1
fi
PERCENT=$(echo "scale=2; $PASS * 100 / $SCORE_TOTAL" | bc)
echo ""
echo "=== Results ==="
echo "Total:  $TOTAL"
echo "Pass:   $PASS"
echo "Fail:   $FAIL"
echo "XFail:  $XFAIL (excluded non-compliant)"
echo "XPass:  $XPASS (unexpectedly parsed)"
echo "Score:  ${PERCENT}% (excludes non-compliant)"

if [ "$ANALYZE" -eq 1 ] && [ "$FAIL" -gt 0 -o "$ANALYZE_EXPECTED" -eq 1 ]; then
    echo ""
    echo "=== Error Analysis (tree-sitter) ==="
    STATS_TMP="$(mktemp)"
    COUNT_TMP="$(mktemp)"

    mapfile -t FAIL_FILES < "$FAIL_LIST"
    if [ "$ANALYZE_EXPECTED" -eq 1 ] && [ -s "$XFAIL_LIST" ]; then
        mapfile -t XFAIL_FILES < "$XFAIL_LIST"
        FAIL_FILES+=("${XFAIL_FILES[@]}")
    fi
    if [ "$ANALYZE_MAX_FILES" -gt 0 ] && [ "${#FAIL_FILES[@]}" -gt "$ANALYZE_MAX_FILES" ]; then
        FAIL_FILES=("${FAIL_FILES[@]:0:$ANALYZE_MAX_FILES}")
        echo "Analyzing ${#FAIL_FILES[@]} of $FAIL failing files (limit ${ANALYZE_MAX_FILES})."
    else
        echo "Analyzing ${#FAIL_FILES[@]} failing files."
    fi

    for f in "${FAIL_FILES[@]}"; do
        if [[ "$f" = /* ]]; then
            abs_path="$f"
        else
            abs_path="$ROOT_DIR/$f"
        fi
        (cd tree-sitter-vhdl && npx tree-sitter parse "$abs_path" 2>/dev/null) | awk \
            -v file="$f" \
            -v statfile="$STATS_TMP" \
            -v countfile="$COUNT_TMP" '
            BEGIN {
                inerr=0; errcount=0; kind=""; steps=0; errline=0;
                skip["identifier"]=1; skip["number"]=1; skip["character_literal"]=1;
                skip["bit_string_literal"]=1; skip["string_literal"]=1; skip["comment"]=1;
            }
            /\(ERROR/ {
                errcount++; kind="ERROR"; inerr=1; steps=0; errline=0;
                if (match($0, /\[[0-9]+, *[0-9]+\]/)) {
                    token = substr($0, RSTART+1, RLENGTH-2);
                    split(token, parts, ",");
                    gsub(/ /, "", parts[1]);
                    errline = parts[1] + 1;
                }
                next
            }
            /\(MISSING/ {
                errcount++; kind="MISSING"; inerr=1; steps=0; errline=0;
                if (match($0, /\[[0-9]+, *[0-9]+\]/)) {
                    token = substr($0, RSTART+1, RLENGTH-2);
                    split(token, parts, ",");
                    gsub(/ /, "", parts[1]);
                    errline = parts[1] + 1;
                }
                next
            }
            inerr {
                steps++;
                if (match($0, /\([a-zA-Z_]+/)) {
                    child = substr($0, RSTART + 1, RLENGTH - 1);
                    if (!skip[child]) {
                        printf "%s\t%s\t%s\t%d\n", kind, child, file, errline >> statfile;
                        inerr=0;
                    }
                }
                if (steps > 20 && inerr) {
                    printf "%s\tunknown\t%s\t%d\n", kind, file, errline >> statfile;
                    inerr=0;
                }
            }
            END { printf "%d\t%s\n", errcount, file >> countfile; }
        '
    done

    if [ -s "$STATS_TMP" ]; then
        echo ""
        echo "Top error constructs (ERROR/MISSING -> first child node):"
        awk -F'\t' '{print $1 "\t" $2}' "$STATS_TMP" | sort | uniq -c | sort -nr | head -n "$ANALYZE_TOP"

        if [ "$ANALYZE_SAMPLES" -gt 0 ]; then
            echo ""
            echo "Examples (file:line: source):"
            while read -r count kind node; do
                echo " $kind $node ($count)"
                awk -F'\t' -v k="$kind" -v n="$node" -v max="$ANALYZE_SAMPLES" '
                    $1 == k && $2 == n && $4 > 0 {
                        print $3 "\t" $4;
                        c++;
                        if (c >= max) exit;
                    }
                ' "$STATS_TMP" | while IFS=$'\t' read -r file line; do
                    if [ -n "$line" ] && [ -f "$file" ]; then
                        text=$(sed -n "${line}p" "$file" 2>/dev/null | sed 's/^[[:space:]]*//')
                        echo "  $file:$line: $text"
                    else
                        echo "  $file:$line"
                    fi
                done
            done < <(awk -F'\t' '{print $1 "\t" $2}' "$STATS_TMP" | sort | uniq -c | sort -nr | head -n "$ANALYZE_TOP")
        fi
    else
        echo ""
        echo "No ERROR/MISSING nodes captured in analysis."
    fi

    if [ -s "$COUNT_TMP" ]; then
        echo ""
        echo "Top failing files (by error count):"
        sort -nr "$COUNT_TMP" | head -n 20 | awk '{printf "  %s (%s errors)\n", $2, $1}'
    fi

    rm -f "$STATS_TMP" "$COUNT_TMP"
fi

rm -f "$FAIL_LIST" "$XFAIL_LIST"
