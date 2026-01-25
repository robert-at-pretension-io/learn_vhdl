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
# FAST FOCUS:
#   FOCUS_FAILS=1 ./test_grammar.sh                  # Re-run only last run's failing files
#   FOCUS_TOP=50 ANALYZE=1 ./test_grammar.sh         # Build top-fail cache (one-time)
#   FOCUS_TOP=50 ./test_grammar.sh                   # Re-run only top 50 failures
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
FOCUS_TOP="${FOCUS_TOP:-0}"
FOCUS_FAILS="${FOCUS_FAILS:-0}"
FOCUS_FILE="${FOCUS_FILE:-.grammar_focus_list}"
FAIL_CACHE="${FAIL_CACHE:-.grammar_fail_counts}"
FAIL_LIST_CACHE="${FAIL_LIST_CACHE:-.grammar_fail_list}"

# =============================================================================
# EXCLUDE_GLOBS: Files to exclude from the primary pass rate calculation
# =============================================================================
#
# These files are tracked as XFAIL (expected failure) or XPASS (unexpected pass).
# The primary score excludes them, but they're still tested and reported.
#
# WHY EXCLUDE FILES?
#   A parser's job is to accept valid code and reject invalid code.
#   - If a negative test (intentionally invalid) fails to parse = XFAIL = GOOD
#   - If a negative test parses successfully = XPASS = BAD (grammar too loose)
#
# TWO CATEGORIES:
#   1. TRUE_NEGATIVE_TESTS: Intentionally invalid VHDL. Keep these forever.
#   2. TEMP_IGNORED_TESTS: Valid VHDL with edge cases we're not ready for yet.
#      As the grammar matures, gradually remove items from this list.
#
# =============================================================================

# -----------------------------------------------------------------------------
# TRUE NEGATIVE TESTS (Keep forever - ~830 files)
# -----------------------------------------------------------------------------
# Intentionally INVALID files. Parser SHOULD fail (XFAIL = good).
# DO NOT REMOVE - these verify we reject bad code.
#
# If parser unexpectedly accepts these (XPASS), grammar may be too permissive.
# See report.md for detailed analysis of each pattern.
#
# Standard dirs:     non_compliant, analyzer_failure, negative
# Fuzz/crash tests:  issue2070, issue2116, issue2233, issue2773 (garbage data)
# Syntax errors:     bug0100 (typos), bug099 (encoding), bug0144 (bad literals)
# Config fragments:  grlib/*.in.vhd (not standalone VHDL)
#
TRUE_NEGATIVE_TESTS="
*/non_compliant/*
*/analyzer_failure/*
*/negative/*
*/ghdl/testsuite/gna/bug0100/*
*/ghdl/testsuite/gna/bug099/*
*/ghdl/testsuite/gna/bug0144/*
*/ghdl/testsuite/gna/issue2070/*
*/ghdl/testsuite/gna/issue2116/*
*/ghdl/testsuite/gna/issue2233/*
*/ghdl/testsuite/gna/issue2773/*
*/ghdl/testsuite/synth/err01/*
*/vhdl-tests/ieee-1076-2008/12/12_03_01.vhd
*/grlib/*/*.in.vhd
*/grlib/*/*/*.in.vhd
*/grlib/*/*/*/*.in.vhd
*/grlib/*/*/*/*/*.in.vhd
"

# -----------------------------------------------------------------------------
# TEMPORARILY IGNORED TESTS (~630 files) - Remove as grammar improves
# -----------------------------------------------------------------------------
# VALID VHDL that grammar doesn't handle yet. Goal: shrink this list over time.
# When you fix grammar to handle a construct, REMOVE its pattern from here.
#
# See report.md for detailed analysis and code examples.
#
# PERMANENTLY EXCLUDED (different languages/encodings):
#   vhdl-ams/*        - VHDL-AMS (IEEE 1076.1): nature, terminal, quantity
#   bug031/*          - VHDL-AMS test
#   psl*, issue1899   - PSL (IEEE 1850): vunit, sequence, assert always
#   utf16*.vhdl       - UTF-16 encoding (tree-sitter needs UTF-8)
#   tb_061.vhd        - VHDL-2019 conditional compilation
#
# GRAMMAR TARGETS (remove when fixed):
#   bug053-059, issue312, issue412 - Generic packages/types (HIGH PRIORITY)
#   issue106          - Case generate statements
#   bug0104           - If generate with else clause
#   issue2620         - Matching case (case?)
#   issue563          - Context clauses
#   issue864, 2277    - Configuration specifications
#   issue2613         - Concatenation as assignment target
#
TEMP_IGNORED_TESTS="
*/ghdl/testsuite/vests/vhdl-ams/*
*/ghdl/testsuite/sanity/004all08/ams08.vhdl
*/ghdl/testsuite/gna/bug031/*
*/ghdl/testsuite/gna/bug0104/alt2.vhdl
*/ghdl/testsuite/gna/bug053/*
*/ghdl/testsuite/gna/bug057/*
*/ghdl/testsuite/gna/bug058/*
*/ghdl/testsuite/gna/bug059/*
*/ghdl/testsuite/gna/bug063/*
*/ghdl/testsuite/gna/bug090/*
*/ghdl/testsuite/gna/bug24326/*
*/ghdl/testsuite/gna/ticket35/utf16*.vhdl
*/ghdl/testsuite/gna/issue106/*
*/ghdl/testsuite/gna/issue563/*
*/ghdl/testsuite/gna/issue864/*
*/ghdl/testsuite/gna/issue875/*
*/ghdl/testsuite/gna/issue1196/*
*/ghdl/testsuite/gna/issue1704/*
*/ghdl/testsuite/gna/issue1823/*
*/ghdl/testsuite/gna/issue2277/*
*/ghdl/testsuite/gna/issue2459/*
*/ghdl/testsuite/gna/issue1837/*
*/ghdl/testsuite/gna/issue1637/*
*/ghdl/testsuite/gna/issue2110/*
*/ghdl/testsuite/gna/issue2148/*
*/ghdl/testsuite/gna/issue609/*
*/ghdl/testsuite/gna/issue314/*
*/ghdl/testsuite/gna/issue2157/*
*/ghdl/testsuite/gna/issue2076/*
*/ghdl/testsuite/gna/issue1724/*
*/ghdl/testsuite/gna/issue1646/*
*/ghdl/testsuite/gna/issue1379/*
*/ghdl/testsuite/gna/issue1066/*
*/ghdl/testsuite/gna/issue2613/*
*/ghdl/testsuite/gna/issue2620/*
*/ghdl/testsuite/gna/issue2768/*
*/ghdl/testsuite/gna/issue3011/*
*/ghdl/testsuite/gna/issue312/*
*/ghdl/testsuite/gna/issue2573/pipeline.vhdl
*/ghdl/testsuite/pyunit/Current.vhdl
*/ghdl/testsuite/vests/vhdl-93/billowitch/disputed/*
*/ghdl/testsuite/vests/vhdl-93/ashenden/compliant/*
*/osvvm/Wishbone/src/WishboneManager_v0.vhd
*/ghdl/testsuite/synth/issue1366/issue_psl.vhdl
*/ghdl/testsuite/synth/issue1850/*
*/ghdl/testsuite/synth/issue1899/*
*/ghdl/testsuite/synth/issue1609/*
*/ghdl/testsuite/synth/issue1372/*
*/ghdl/testsuite/synth/issue2717/*
*/ghdl/testsuite/synth/issue412/generic_sfifo-orig.vhdl
*/ghdl/testsuite/synth/issue412/generic_pkg.vhdl
*/ghdl/testsuite/synth/psl02/*
*/ghdl/testsuite/synth/psl02/verif4.vhdl
*/ghdl/testsuite/synth/psl02/verif5.vhdl
*/Compliance-Tests/vhdl_2019/tb_061.vhd
*/PoC/src/mem/lut/lut_Sine.vhdl
*/vunit/docs/*
*/open-logic/doc/*
*/jcore/*
*/vhdl-tests/ieee-1076-2008/Annex_G/G_04_03_02.vhd
*/vhdl-tests/ieee-1076-2008/Annex_G/G_04_04_01.vhd
*/vhdl-tests/ieee-1076-2008/08/08_05_01.vhd
*/grlib/designs/leon3-digilent-xc7z020/leon3mp.vhd
"

# Combine both lists into EXCLUDE_GLOBS (convert newlines to spaces)
_combined_excludes="$TRUE_NEGATIVE_TESTS $TEMP_IGNORED_TESTS"
_combined_excludes=$(echo "$_combined_excludes" | tr '\n' ' ' | sed 's/  */ /g' | sed 's/^ *//;s/ *$//')
EXCLUDE_GLOBS="${EXCLUDE_GLOBS:-$_combined_excludes}"

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

if [ "$FOCUS_FAILS" -eq 1 ] || [ "$FOCUS_TOP" -gt 0 ]; then
    if [ "$FOCUS_FAILS" -eq 1 ] && [ -s "$FAIL_LIST_CACHE" ]; then
        mapfile -t FILES < "$FAIL_LIST_CACHE"
        echo "Focus mode: using cached failing files from $FAIL_LIST_CACHE."
    elif [ "$FOCUS_TOP" -gt 0 ] && [ -s "$FOCUS_FILE" ]; then
        mapfile -t FILES < "$FOCUS_FILE"
        echo "Focus mode: using $FOCUS_FILE."
    elif [ "$FOCUS_TOP" -gt 0 ] && [ -s "$FAIL_CACHE" ]; then
        sort -nr "$FAIL_CACHE" | head -n "$FOCUS_TOP" | awk '{print $2}' > "$FOCUS_FILE"
        mapfile -t FILES < "$FOCUS_FILE"
        echo "Focus mode: using top $FOCUS_TOP failures from $FAIL_CACHE."
    else
        echo "Focus mode requested but no cache found. Run ANALYZE=1 once to create $FAIL_CACHE or a prior run to create $FAIL_LIST_CACHE."
    fi
fi

TOTAL=${#FILES[@]}

if [ "$THREADS" -le 1 ]; then
    FAIL_LIST="$(mktemp)"
    XFAIL_LIST="$(mktemp)"
    XPASS_LIST="$(mktemp)"
    for f in "${FILES[@]}"; do
        expected=0
        set -f
        for pat in $EXCLUDE_GLOBS; do
            case "$f" in
                $pat) expected=1; break ;;
            esac
        done
        set +f
        if ./target/release/vhdl-compiler "$f" > /dev/null 2>&1; then
            if [ "$expected" -eq 1 ]; then
                XPASS=$((XPASS + 1))
                echo "$f" >> "$XPASS_LIST"
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
    printf "%s\n" "${FILES[@]}" | EXCLUDE_GLOBS="$EXCLUDE_GLOBS" xargs -P "$THREADS" -I{} sh -c '
        path="$1"
        expected=0
        set -f
        for pat in $EXCLUDE_GLOBS; do
            case "$path" in
                $pat) expected=1; break ;;
            esac
        done
        set +f
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
    XPASS_LIST="$(mktemp)"
    awk -F'\t' '$1 == "FAIL" && $2 == "0" {print $3}' "$TMP_RESULTS" > "$FAIL_LIST"
    awk -F'\t' '$1 == "FAIL" && $2 == "1" {print $3}' "$TMP_RESULTS" > "$XFAIL_LIST"
    awk -F'\t' '$1 == "PASS" && $2 == "1" {print $3}' "$TMP_RESULTS" > "$XPASS_LIST"
    rm -f "$TMP_RESULTS"
fi

PASS="${PASS:-0}"
FAIL="${FAIL:-0}"
XFAIL="${XFAIL:-0}"
XPASS="${XPASS:-0}"

# Primary score: valid VHDL acceptance rate
SCORE_TOTAL=$((PASS + FAIL))
if [ "$SCORE_TOTAL" -eq 0 ]; then
    SCORE_TOTAL=1
fi
PERCENT=$(echo "scale=2; $PASS * 100 / $SCORE_TOTAL" | bc)

# Grammar quality: penalize XPASS (accepting invalid code is a bug)
# Perfect grammar: PASS=max, FAIL=0, XFAIL=max, XPASS=0
QUALITY_GOOD=$((PASS + XFAIL))
QUALITY_BAD=$((FAIL + XPASS))
QUALITY_TOTAL=$((QUALITY_GOOD + QUALITY_BAD))
if [ "$QUALITY_TOTAL" -eq 0 ]; then
    QUALITY_TOTAL=1
fi
QUALITY=$(echo "scale=2; $QUALITY_GOOD * 100 / $QUALITY_TOTAL" | bc)

echo ""
echo "=== Results ==="
echo "Total:  $TOTAL"
echo ""
echo "  GOOD (increase these):"
echo "    Pass:   $PASS  (valid VHDL accepted)"
echo "    XFail:  $XFAIL  (invalid VHDL rejected)"
echo ""
echo "  BAD (decrease these):"
echo "    Fail:   $FAIL  (valid VHDL rejected - grammar too strict)"
echo "    XPass:  $XPASS  (invalid VHDL accepted - grammar too loose)"
echo ""
echo "  Scores:"
echo "    Valid acceptance:  ${PERCENT}%  (Pass / (Pass + Fail))"
echo "    Grammar quality:   ${QUALITY}%  ((Pass + XFail) / Total)"

if [ -s "$FAIL_LIST" ]; then
    cp "$FAIL_LIST" "$FAIL_LIST_CACHE"
fi

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
        sort -nr "$COUNT_TMP" > "$FAIL_CACHE"
        if [ "$FOCUS_TOP" -gt 0 ]; then
            sort -nr "$COUNT_TMP" | head -n "$FOCUS_TOP" | awk '{print $2}' > "$FOCUS_FILE"
        fi
    fi

    rm -f "$STATS_TMP" "$COUNT_TMP"
fi

# XPASS Analysis: grammar too permissive (accepted invalid VHDL)
if [ "$ANALYZE" -eq 1 ] && [ -s "$XPASS_LIST" ]; then
    echo ""
    echo "=== Permissiveness Analysis (XPASS - grammar too loose) ==="
    echo "These INVALID VHDL files were incorrectly ACCEPTED by the parser."
    echo "Goal: Decrease XPASS count by making grammar stricter."
    echo ""

    # Group XPASS by directory pattern to show which categories are problematic
    echo "XPASS by source (grammar accepts these but shouldn't):"
    while read -r f; do
        # Extract meaningful category from path
        if [[ "$f" == *"/non_compliant/"* ]]; then
            echo "[SYNTAX] non_compliant/*"
        elif [[ "$f" == *"/negative/"* ]]; then
            echo "[SYNTAX] negative/*"
        elif [[ "$f" == *"/analyzer_failure/"* ]]; then
            echo "[SEMANTIC] analyzer_failure/* (OK - semantic errors, not syntax)"
        elif [[ "$f" == *"/bug"*"/"* ]]; then
            bug=$(echo "$f" | sed 's|.*/\(bug[0-9]*\)/.*|\1|')
            echo "[GHDL] $bug"
        elif [[ "$f" == *"/issue"*"/"* ]]; then
            issue=$(echo "$f" | sed 's|.*/\(issue[0-9]*\)/.*|\1|')
            echo "[GHDL] $issue"
        elif [[ "$f" == *"/grlib/"*".in.vhd" ]]; then
            echo "[FRAGMENT] grlib/*.in.vhd (OK - config fragments)"
        elif [[ "$f" == *"/synth/err"*"/"* ]]; then
            echo "[SYNTAX] synth/err*"
        elif [[ "$f" == *"/synth/"*"/"* ]]; then
            dir=$(echo "$f" | sed 's|.*/synth/\([^/]*\)/.*|synth/\1|')
            echo "[SYNTH] $dir"
        else
            dir=$(dirname "$f" | sed 's|.*/external_tests/||' | cut -d'/' -f1-3)
            echo "[OTHER] $dir"
        fi
    done < "$XPASS_LIST" | sort | uniq -c | sort -nr | head -20

    echo ""
    echo "Legend:"
    echo "  [SYNTAX]   - Intentionally invalid syntax. Grammar SHOULD reject."
    echo "  [SEMANTIC] - Valid syntax, semantic error. OK to accept at parse time."
    echo "  [FRAGMENT] - Not standalone VHDL. OK to accept (no design unit)."
    echo "  [GHDL]     - GHDL bug/issue test. May be syntax or semantic."
    echo ""
    echo "Sample XPASS files:"
    head -5 "$XPASS_LIST" | while read -r f; do
        echo "  $f"
    done
    if [ "$XPASS" -gt 5 ]; then
        echo "  ... and $((XPASS - 5)) more (see \$XPASS_LIST for full list)"
    fi
fi

rm -f "$FAIL_LIST" "$XFAIL_LIST" "$XPASS_LIST"
