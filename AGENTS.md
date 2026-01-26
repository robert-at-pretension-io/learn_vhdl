# VHDL Compiler Project

## Project Purpose

This is a **learning project** with two goals:

1. **Learn VHDL** by building a compiler/linter for it
2. **Learn compiler construction** using modern, declarative tools

The philosophy is "learn by doing" - instead of just reading about VHDL, we build a tool that understands it. Every grammar rule we write teaches us something about the language.

## The Core Philosophy: "Panic is Failure"

The compiler never crashes on user input. It ingests code, understands what it can, and explicitly reports what it cannot.

* **User Errors:** "You wrote bad VHDL (Latch inferred)."
* **System Edges:** "I don't know how to handle `generate` statements yet."
* **Contract Violations:** The tool crashes *immediately* if internal data doesn't match the schema - no silent failures.

---

## The Architecture: The "Go-Powered Declarative Stack"

### Phase 1: The Resilient Parser (Tree-sitter)

* **The Goal:** Turn text into a tree without crashing.
* **The Tech:** **Tree-sitter** (C parser with Go bindings).
* **The Configuration:** `tree-sitter-vhdl/grammar.js`.
* **How it works:**
  * GLR algorithm tolerates syntax errors
  * Produces valid tree even for broken code
  * Supports case-insensitive keywords (VHDL requirement)
  * Visible semantic nodes (`port_direction`, `association_element`, etc.) for extraction
* **The Edge:** If it sees unknown syntax, it inserts an `(ERROR)` node and resynchronizes.

**Current Status:** 94%+ grammar compliance across 12,000+ test files

#### CRITICAL: Grammar is the Source of Truth

**When you encounter false positives or extraction bugs, FIX THE GRAMMAR FIRST.**

ERROR nodes are poisonous. When the grammar can't parse a VHDL construct:
1. Tree-sitter creates an `ERROR` node containing raw text
2. Keywords inside ERROR nodes appear as identifiers
3. The extractor sees fake "signals" that don't exist
4. OPA rules fire false positives

**Never work around grammar bugs:**
- ❌ Adding skip lists in OPA rules
- ❌ Filtering keywords in the extractor
- ❌ Ignoring certain file patterns

**Always fix at the source:**
- ✅ Identify which VHDL construct causes the ERROR
- ✅ Update `grammar.js` to handle that construct
- ✅ Regenerate: `npx tree-sitter generate`
- ✅ Rebuild Go: `go clean -cache && go build ./cmd/vhdl-lint`

**CRITICAL: Go Build Cache Gotcha**

The Go bindings include `tree-sitter-vhdl/src/parser.c` via CGO. After regenerating the parser with `npx tree-sitter generate`, you **MUST** clear the Go build cache:

```bash
go clean -cache && go build ./cmd/vhdl-lint
```

Without `go clean -cache`, Go will use the cached compiled parser and your grammar changes won't take effect. This is a common source of confusion when grammar changes appear to work in `tree-sitter parse` but not in the Go linter.

**To debug parse failures:**
```bash
# See ERROR nodes in a file
npx tree-sitter parse file.vhd 2>&1 | grep ERROR

# Get full parse tree
npx tree-sitter parse file.vhd
```

#### Recent Lessons & Advice

- **CRITICAL**: After `npx tree-sitter generate`, always run `go clean -cache` before `go build` - CGO caches compiled C code and won't pick up parser changes otherwise.
- Use `ANALYZE=1 ./test_grammar.sh` to identify the top failing constructs before editing grammar.
- Exclude anti-tests (`*/non_compliant/*`, `*/negative/*`, `*/analyzer_failure/*`) from the score, but still inspect them for grammar clues.
- Favor minimal `conflicts` additions over broad regex fallbacks; re-run `test_grammar.sh` after each change.
- If `npx tree-sitter generate` panics, run `npm run build` in `tree-sitter-vhdl/` (pinned CLI).
- Prefer deferred decisions for VHDL "syntactic homonyms" like `name(0)`; unify names instead of guessing array vs call.
- Avoid local-maximum hacks that only improve a narrow test set; aim for abstractions that generalize.
- Discrete ranges appear in `for` loops, slices, and type constraints; accept subtype indications like `integer range 1 to 3`.
- Qualified expressions often appear inside `report` strings (`string'("...")`); include them in report parsing.
- Encoding failures are not grammar bugs; exclude non-UTF8 test files or re-encode them.
- To reduce XPASS quickly, tighten broad fallbacks first (`_simple_expression`, `_simple_statement`) before adding targeted strictness.
- Syntax-only negatives (e.g., `bug0100`, `synth/err*`) are best fixed in grammar; semantic negatives belong in extractor/OPA.

#### Semantic Issues: What Must Be Tested (Rules Needed)

Grammar alone cannot detect semantic violations. To correctly flag semantic issues, add extractor + OPA rules and ensure tests cover:

- **Type compatibility:** reject assignments between incompatible types (e.g., `display := item_name;`), mismatched array dimensions, illegal conversions.
- **Port/generic conformance:** actuals match formals (types, directions, widths), missing/extra associations, illegal mode usage.
- **Drivers and resolution:** multiple drivers on unresolved types, undriven signals, partial assignment to composite signals in a process.
- **Process semantics:** latch inference (incomplete assignments in combinational processes), sensitivity list completeness, clock/reset patterns.
- **Visibility and scope:** unresolved identifiers, illegal use of declarations outside scope, missing library/package imports.
- **Constraint validity:** range direction mismatches, null range usage where disallowed, invalid index bounds.
- **Elaboration rules:** illegal generates, duplicate labels, configuration mismatches, component/entity binding errors.

Recommended test sources for semantic rules:
- `*/non_compliant/*/analyzer_failure/*` (semantic-invalid by design)
- `external_tests/ghdl/testsuite/synth/err*` (semantic misuses)
- Targeted positive tests (valid VHDL) to guard against false positives.

#### AI-Assisted Grammar Improvement Workflow

The test suite contains **12,000+ VHDL files** from production projects (GRLIB, OSVVM, VUnit, neorv32, ghdl, etc.). Running the full suite and caching results enables efficient AI-assisted improvement.

**Step 1: Run Full Test Suite with Analysis (Human or AI)**
```bash
# This takes a few minutes but only needs to run once per session
ANALYZE=1 ./test_grammar.sh external_tests
```

This creates three cache files:
- `.grammar_fail_counts` - Error count per file (sorted by severity)
- `.grammar_fail_list` - List of all failing files
- `.grammar_focus_list` - Top N worst offenders (when using FOCUS_TOP)

**Step 2: AI Analyzes Cache to Find Patterns**
```bash
# AI should read the cache files to identify worst offenders
cat .grammar_fail_counts | head -20   # Top 20 files by error count
cat .grammar_fail_list | wc -l        # Total failing files

# AI examines a few of the worst files
cd tree-sitter-vhdl
npx tree-sitter parse ../path/to/worst_file.vhd 2>&1 | grep -A2 ERROR
```

The AI looks for patterns:
- Same ERROR construct appearing in many files = high-value fix
- Single file with many errors = unusual construct or encoding issue
- Cluster of files from same project = project-specific VHDL style

**Step 3: Fix Grammar Based on Analysis**
```bash
# AI edits grammar.js to handle the identified construct
vim grammar.js

# Regenerate parser
npx tree-sitter generate
```

**Step 4: Fast Iteration with Focus Mode**
```bash
# Re-test ONLY the previously failing files (instant feedback)
FOCUS_FAILS=1 ./test_grammar.sh

# Or re-test only top N worst offenders
FOCUS_TOP=50 ./test_grammar.sh
```

**Step 5: Verify with Full Suite**
```bash
# After fixing, run full suite again to confirm improvement
ANALYZE=1 ./test_grammar.sh external_tests
# Score should improve, cache files update automatically
```

**The Key Insight:** Don't re-run 12,000 files after every grammar edit. Instead:
1. Run full suite ONCE to build cache
2. AI analyzes cache to prioritize fixes
3. Use FOCUS_FAILS or FOCUS_TOP for instant feedback during iteration
4. Run full suite again only to verify final improvement

#### Semantic XPASS Reduction Loop (Linter Pass)

Use the grammar XPASS list as input to the linter to shrink **semantic** misses:

```bash
# 1) Build XPASS list
ANALYZE=1 ./test_grammar.sh external_tests

# 2) Run linter over semantic negatives only (analyzer_failure)
LINT_XPASS=1 LINT_FILTER=semantic LINT_MAX_FILES=0 ./test_grammar.sh

# Optional: focus syntax negatives (non_compliant/negative/synth/err)
LINT_XPASS=1 LINT_FILTER=syntax LINT_MAX_FILES=0 ./test_grammar.sh
```

Interpret the output:
- **Top rule hits** show which semantic rules are already firing.
- **Clean list** are semantic negatives with *no* violations → add Rego rules or extractor coverage.
- Iterate: add rules → rerun `LINT_XPASS` → confirm clean list shrinks.

Quick checklist:
- Clean list shrinking across iterations?
- Rule hits dominated by style/naming? Temporarily disable them for semantic runs.
- Clean files in `analyzer_failure` likely need extractor/read-write coverage.
- Extractors are imperfect: run the linter on semantically valid projects to detect false positives.

#### Proving Ourselves: Evidence Strategy

We cannot formally prove correctness for all VHDL, but we can build a strong,
auditable case that the parser, extractor, and rules behave as intended. This
is how we replace "trust me" with measurable evidence.

**Evidence Stack (what and why):**
1. **Grammar Soundness (no ERROR nodes)**
   - Why: ERROR nodes poison the AST and create false positives downstream.
   - Evidence: `./test_grammar.sh` shows zero ERROR nodes on valid corpora and
     no regressions in `FAIL` or `XPASS`.
2. **Contract Integrity (CUE validation)**
   - Why: schema drift silently disables rules.
   - Evidence: `internal/validator` validates against `schema/ir.cue` every run.
3. **Rule Correctness (two-sided fixtures)**
   - Why: a rule is only trustworthy if it fires when it should and stays quiet
     when it should not.
   - Evidence: positive and negative fixtures in `testdata/policy_rules/`,
     enforced by `internal/policy/policy_rules_test.go`.
4. **Precision and Recall on Real Corpora**
   - Why: production VHDL is the best ground truth we have.
   - Evidence: running `./vhdl-lint` on `external_tests/` yields zero or
     explicitly justified violations. Any surprise is treated as a tool bug.
5. **Differential Oracles (reference tools)**
   - Why: independent parsers/checkers catch blind spots.
   - Evidence: compare parse/diagnostics with a reference (e.g., GHDL) and
     track divergences as regressions.
6. **CI Gates and Proof Artifacts**
   - Why: correctness claims must be repeatable.
   - Evidence: CI publishes grammar pass rate, ERROR node counts, rule coverage,
     and lint summaries. Regressions fail the build.

**Operational Checklist (what to do when you change things):**
- Grammar changes: run `ANALYZE=1 ./test_grammar.sh external_tests`; ensure no
  new ERROR nodes, `FAIL` does not increase; update `report.md` if metrics move.
- Extractor changes: ensure `internal/validator` passes; add or update fixtures
  that prove the new data is extracted correctly.
- Rule changes: add one positive fixture and one negative fixture; update
  `testdata/policy_rules/manifest.json`; run `go test ./internal/policy -run TestPolicyRuleFixtures`.
- False positives on real code: treat as a tool bug; fix grammar/extractor
  before touching rules or allowlists.

**Example AI Session:**
```
Human: Improve the grammar pass rate

AI: Let me first run the full test suite to build the cache...
    [runs ANALYZE=1 ./test_grammar.sh external_tests]
    Current score: 94.19% (569 failures)

    Analyzing .grammar_fail_counts...
    Top failures are in osvvm/AXI4 files with "protected type" constructs.

    Let me examine one of these files...
    [reads tree-sitter parse output]

    The ERROR is around "protected body" - this VHDL-2008 construct isn't in the grammar.

    [edits grammar.js to add protected_type_body rule]
    [runs npx tree-sitter generate]

    Quick verification with focus mode...
    [runs FOCUS_TOP=50 ./test_grammar.sh]

    50 files now pass that previously failed!

    Running full suite to confirm...
    [runs ANALYZE=1 ./test_grammar.sh external_tests]
    New score: 95.8% (+160 files fixed)
```

### Phase 2: The Extractor (Go)

* **The Goal:** Transform syntax trees into semantic facts.
* **The Tech:** **Go** walking the Tree-sitter tree.
* **What it extracts:**
  * **Structural:** Entities, Architectures, Packages, Signals, Ports
  * **Behavioral:** Processes with sensitivity lists
  * **Semantic:** Clock domains, reset patterns, signal read/write analysis
  * **Hierarchical:** Component instances with port/generic mappings

### Phase 3: The Indexer (Go)

* **The Goal:** Build the cross-file "world view."
* **The Tech:** **Go** with concurrent file processing.
* **The Logic:**
  * **Pass 1:** Parse all files, build global symbol table (entities, packages)
  * **Pass 2:** Resolve imports, check dependencies
  * **Output:** Normalized JSON for policy evaluation
* **The Edge:** Reports missing dependencies, unresolved components.

### Phase 4: The Contract Guard (CUE)

* **The Goal:** Guarantee data integrity between Go and OPA.
* **The Tech:** **CUE** schema validation.
* **The Logic:**
  * Go marshals extracted data to JSON
  * CUE validates against `schema/ir.cue`
  * If validation fails, **crash immediately** - no silent failures
* **Why this matters:** Without CUE, if a field name changes, OPA silently receives `undefined` and produces no violations. With CUE, you get an immediate error.

### Phase 5: The Policy Engine (OPA)

* **The Goal:** Declarative compliance checking.
* **The Tech:** **OPA** with Rego policy language.
* **The Logic:**
  * Receives normalized JSON from indexer
  * Evaluates declarative rules
  * Returns violations with file/line locations
* **Current rules:**
  * Naming conventions
  * Unresolved dependencies
  * Missing component definitions
* **Future rules:**
  * Latch inference
  * Clock domain crossings
  * Multi-driver detection

---

## The "Why" Matrix

| Problem Domain | Chosen Tech | Why this specifically? |
| --- | --- | --- |
| **Syntax** | **Tree-sitter** | Error-tolerant parsing, incremental updates, industry standard |
| **Extraction** | **Go** | Fast, concurrent, single binary deployment |
| **Cross-file** | **Go Indexer** | Symbol table, dependency graph, parallel processing |
| **Contract** | **CUE** | Type-safe schema validation, prevents silent failures |
| **Policy** | **OPA/Rego** | Declarative rules, standard enterprise tooling |

---

## Data Flow

```
VHDL Files
    │
    ▼
Tree-sitter Parser (grammar.js)
    │
    ▼ Concrete Syntax Tree
    │
Go Extractor
    │
    ▼ FileFacts (entities, signals, processes, instances...)
    │
Go Indexer
    │
    ▼ Normalized JSON (policy.Input)
    │
CUE Validator ──── CRASH if schema mismatch
    │
    ▼ Validated JSON
    │
OPA Policy Engine
    │
    ▼ Violations
    │
Human-readable Report
```

---

## Key Data Structures

### FileFacts (from Extractor)
```go
type FileFacts struct {
    File          string
    Entities      []Entity
    Architectures []Architecture
    Packages      []Package
    Signals       []Signal
    Ports         []Port
    Processes     []Process
    Instances     []Instance      // NEW: component instantiations
    ClockDomains  []ClockDomain   // Semantic: clock analysis
    SignalUsages  []SignalUsage   // Semantic: read/write tracking
    ResetInfos    []ResetInfo     // Semantic: reset detection
}
```

### Instance (for hierarchical analysis)
```go
type Instance struct {
    Name       string            // "u_cpu"
    Target     string            // "work.cpu"
    PortMap    map[string]string // formal -> actual signal
    GenericMap map[string]string // formal -> actual value
    Line       int
    InArch     string
}
```

### Process (with semantic analysis)
```go
type Process struct {
    Label           string
    SensitivityList []string
    IsSequential    bool     // Has clock edge
    ClockSignal     string   // "clk"
    ClockEdge       string   // "rising" or "falling"
    HasReset        bool
    ResetSignal     string
    ResetAsync      bool     // Async if checked before clock
    AssignedSignals []string // Signals written
    ReadSignals     []string // Signals read
}
```

---

## The Developer's "Game Loop"

This is the tight feedback loop for learning:

1. **Find a VHDL construct** you don't understand
2. **Run `./test_grammar.sh`** to see current pass rate
3. **Edit `grammar.js`** to recognize the construct
4. **Run `npx tree-sitter generate`** to rebuild parser
5. **Run `./test_grammar.sh`** again - score should improve
6. **Add extraction logic** in Go if semantic data is needed
7. **Add CUE schema** if new data types are introduced
8. **Add OPA rules** to check for violations
9. **Repeat** - each cycle teaches you more VHDL

---

## The Grammar Improvement Cycle

The grammar is the foundation of everything. When it can't parse a construct, ERROR nodes propagate through the entire pipeline causing false positives. Here's the systematic approach to improving it:

### Step 1: Measure Current State

```bash
# Run against all external tests to get baseline
./test_grammar.sh external_tests

# Or test specific project
./test_grammar.sh external_tests/neorv32

# Or limit to first N files for quick iteration
./test_grammar.sh external_tests 50
```

Example output:
```
Testing against files in: external_tests
Progress: 500 files tested (423 pass, 77 fail)
Progress: 1000 files tested (891 pass, 109 fail)

=== Results ===
Total:  1247
Pass:   1089
Fail:   158
Score:  87.33%
```

### Step 2: Find Failing Files

```bash
# Find files that fail to parse cleanly
for f in $(find external_tests/neorv32 -name "*.vhd"); do
  if ! npx tree-sitter parse "$f" 2>&1 | grep -q ERROR; then
    echo "PASS: $f"
  else
    echo "FAIL: $f"
  fi
done
```

### Step 3: Identify the Problem

```bash
# See ERROR nodes in a specific file
cd tree-sitter-vhdl
npx tree-sitter parse ../external_tests/neorv32/rtl/core/neorv32_cpu_control.vhd 2>&1 | grep -A2 ERROR

# Get full parse tree for detailed analysis
npx tree-sitter parse ../external_tests/neorv32/rtl/core/neorv32_cpu_control.vhd > /tmp/tree.txt

# Find the exact line causing issues
npx tree-sitter parse ../external_tests/neorv32/rtl/core/neorv32_cpu_control.vhd 2>&1 | grep "ERROR" | head -5
```

### Step 4: Fix the Grammar

```bash
# Edit the grammar
vim grammar.js

# Regenerate the parser (creates src/parser.c)
npx tree-sitter generate

# Quick test on the problem file
npx tree-sitter parse ../external_tests/neorv32/rtl/core/neorv32_cpu_control.vhd 2>&1 | grep ERROR
```

### Step 5: Rebuild and Verify

```bash
# Go back to project root
cd ..

# Rebuild the Go linter (picks up new parser)
go build -o vhdl-lint ./cmd/vhdl-lint

# Re-run grammar tests
./test_grammar.sh external_tests

# Score should improve!
```

### Step 6: Iterate

Repeat steps 2-5 until the pass rate improves. Common patterns to look for:

| ERROR Pattern | Likely Grammar Issue |
|---------------|---------------------|
| `ERROR` around `<=` | Signal assignment target not matching |
| `ERROR` around `when` | Conditional expression/assignment |
| `ERROR` around `generate` | Generate statement handling |
| `ERROR` around `'` | Attribute expression |
| `ERROR` around `**` | Exponentiation operator |

### The Full Workflow Diagram

```
                    ┌────────────────────┐
                    │  ./test_grammar.sh │
                    │  See pass rate     │
                    └─────────┬──────────┘
                              │
                              ▼
                    ┌────────────────────┐
              ┌─────│  Pass rate OK?     │─────┐
              │     └────────────────────┘     │
              │ NO                             │ YES
              ▼                                ▼
    ┌────────────────────┐           ┌────────────────────┐
    │ Find failing files │           │ Done! Commit       │
    │ grep ERROR         │           │ grammar changes    │
    └─────────┬──────────┘           └────────────────────┘
              │
              ▼
    ┌────────────────────┐
    │ Identify construct │
    │ causing ERROR      │
    └─────────┬──────────┘
              │
              ▼
    ┌────────────────────┐
    │ Edit grammar.js    │
    │ Add/fix rule       │
    └─────────┬──────────┘
              │
              ▼
    ┌────────────────────┐
    │ npx tree-sitter    │
    │ generate           │
    └─────────┬──────────┘
              │
              ▼
    ┌────────────────────┐
    │ Test problem file  │
    │ ERROR gone?        │
    └─────────┬──────────┘
              │
              ▼
    ┌────────────────────┐
    │ go build           │
    │ ./cmd/vhdl-lint    │
    └─────────┬──────────┘
              │
              └────────────────► (back to top)
```

### Tips for Grammar Development

1. **Start small**: Fix one construct at a time, verify it works
2. **Use precedence carefully**: `prec()`, `prec.left()`, `prec.right()`, `prec.dynamic()`
3. **Check conflicts**: `npx tree-sitter generate` shows conflict warnings
4. **Test incrementally**: Don't wait until you've made 10 changes to test
5. **Read the failing code**: Sometimes the VHDL itself reveals what construct is missing

---

## AI Agent Quick Start: False Positive Hunting

**This is the fastest way to improve the linter.** Use `external_tests/Compliance-Tests/vhdl_2008/` for rapid iteration.

### Why Compliance-Tests?

| Dataset | Files | Parse Time | Feedback Loop |
|---------|-------|------------|---------------|
| `Compliance-Tests/vhdl_2008/` | ~50 | ~2 seconds | **Instant** |
| `Full external_tests/` | 12,000+ | ~5 minutes | Slow |

Compliance-Tests are **small, clean, VHDL-2008 focused** - perfect for finding extractor/rule bugs without waiting.

### The 30-Second Feedback Loop

```bash
# 1. Run linter on Compliance-Tests (2 seconds)
./vhdl-lint external_tests/Compliance-Tests/vhdl_2008/ 2>&1 | grep -oP '\[[a-z_]+\]' | sort | uniq -c | sort -rn

# 2. Pick a suspicious rule (high count = likely false positive)
./vhdl-lint external_tests/Compliance-Tests/vhdl_2008/ 2>&1 | grep '\[input_port_driven\]'

# 3. Read one of the flagged files
# 4. Identify the bug (grammar? extractor? rule?)
# 5. Fix it
# 6. Rebuild: go build -o vhdl-lint ./cmd/vhdl-lint
# 7. Re-run step 1 - count should drop!
```

### AI Agent Checklist

When hunting false positives:

1. **Start with Compliance-Tests** - fast iteration, clean VHDL-2008
2. **Count by rule** - `| grep -oP '\[[a-z_]+\]' | sort | uniq -c | sort -rn`
3. **High counts = systematic bugs** - one fix eliminates many violations
4. **Read the actual VHDL** - production code is correct; violations are our bugs
5. **Fix at the lowest level**:
   - Grammar ERROR nodes → fix `grammar.js`
   - Wrong extraction → fix `internal/extractor/extractor.go`
   - Over-strict rule → fix `policies/*.rego`

### Common False Positive Patterns (Quick Reference)

| Symptom | Likely Cause | Fix Location |
|---------|--------------|--------------|
| `undeclared_signal_usage` for `ns`, `ps`, `ms` | Time units in `after X ns` | Extractor: skip `physical_literal` children |
| `undeclared_signal_usage` for external names | VHDL-2008 `<< signal .path >>` | Extractor: skip `external_name` children |
| `input_port_driven` across entities | Rule not scoped to same entity | Policy: check `arch.entity_name == port.in_entity` |
| `empty_sensitivity_combinational` in testbenches | Process uses `wait` statements | Extractor: detect `HasWait`, don't mark as combinational |
| `gated_clock_detection` for clock generators | `clk <= not clk after 5 ns` | Policy: distinguish testbench patterns |

### Example AI Session: Finding & Fixing False Positives

```
Human: Find and fix false positives

AI: Running linter on Compliance-Tests for fast feedback...
    ./vhdl-lint external_tests/Compliance-Tests/vhdl_2008/ 2>&1 | grep -oP '\[[a-z_]+\]' | sort | uniq -c | sort -rn

    Results:
      8 [input_port_driven]
      4 [empty_sensitivity_combinational]
      2 [undeclared_signal_usage]

    Investigating input_port_driven (highest count)...
    Looking at tb_sensitivity_all.vhd - it has TWO entities in one file.
    The rule is matching port names from entity A against signals in entity B.

    Root cause: Rule doesn't scope ports to their entity.
    Fix location: policies/signals.rego - add arch.entity_name check

    [Edits the rule]
    [Rebuilds: go build -o vhdl-lint ./cmd/vhdl-lint]
    [Re-runs: ./vhdl-lint external_tests/Compliance-Tests/vhdl_2008/...]

    input_port_driven count: 8 → 0 ✓

    Moving to empty_sensitivity_combinational...
```

---

## External Tests: A Tool for Improvement, Not Error Finding

The `external_tests/` directory contains **12,000+ VHDL files** from real-world projects:

| Project | Files | Description |
|---------|-------|-------------|
| **ghdl** | ~9,600 | GHDL test suite (largest) |
| **grlib** | ~800 | LEON3/5 SPARC processors + SoC IP |
| **osvvm** | ~640 | VHDL-2008 verification methodology |
| **vunit** | ~330 | Unit testing framework |
| **PoC** | ~290 | VLSI-EDA IP cores |
| **vhdl-tests** | ~280 | Miscellaneous tests |
| **hdl4fpga** | ~270 | FPGA IP library |
| **Compliance-Tests** | ~80 | IEEE 1076-2008 compliance |
| **neorv32** | ~70 | RISC-V processor |

### The Key Insight: Production Code is Correct

**When you run the linter against external_tests and see violations, assume they are FALSE POSITIVES until proven otherwise.**

Real-world projects like neorv32 are:
- Battle-tested in production FPGAs
- Reviewed by experienced engineers
- Synthesized and verified by commercial tools

If our linter says `signal 'cpu_trace' is unused` but the signal is clearly used in a generate statement, **the bug is in our tool, not in neorv32.**

### How to Use External Tests

**Purpose:** Improve grammar and extraction, NOT find bugs in production code.

```bash
# Run against neorv32
./vhdl-lint external_tests/neorv32/rtl/core/

# When you see violations, investigate:
# 1. Is it a real issue? (Unlikely for production code)
# 2. Or is our extraction missing something?

# Debug with verbose output
./vhdl-lint -v external_tests/neorv32/rtl/core/neorv32_top.vhd

# Check for parse errors (ERROR nodes)
cd tree-sitter-vhdl && npx tree-sitter parse ../external_tests/neorv32/rtl/core/neorv32_top.vhd 2>&1 | grep ERROR
```

### Common False Positive Patterns

| Violation | Likely Cause | Fix Location |
|-----------|--------------|--------------|
| `undeclared_signal_usage` | Record field access `rec.field` reported as undeclared `field` | Extractor - need full path tracking |
| `sensitivity_list_incomplete` | Record field reads not tracked with full path | Extractor - tied to undeclared_signal |
| `unused_signal` | Signal used in generate/port map not tracked | Extractor |
| `undriven_signal` | Assignment in generate statement missed | Grammar + Extractor |
| `unresolved_dependency` | Package/library not in search path | Indexer |
| `multi_driven_signal` | Multiple assignments in different branches of case/if | Extractor - false positive in sequential logic |

### Running False Positive Analysis

**Step 1: Run against production code**
```bash
# Single file analysis
./vhdl-lint external_tests/neorv32/rtl/core/neorv32_cpu_control.vhd 2>&1

# Count violations by rule type
./vhdl-lint external_tests/neorv32/rtl/core/neorv32_cpu_control.vhd 2>&1 | \
  grep -oP '\[([a-z_]+)\]' | sort | uniq -c | sort -rn
```

**Step 2: Identify the top offenders**
- High counts of the same rule = systematic false positive
- Look at the actual VHDL code to understand what's being missed

**Step 3: Categorize the issue**
- **Grammar**: Parse tree has ERROR nodes or wrong structure
- **Extractor**: Grammar is correct but facts aren't extracted
- **Rule**: Extraction is correct but rule logic is too strict

**Step 4: Fix at the source** (never work around in rules)

### Known False Positive Issues (2026-01-25)

| Issue | Severity | Status | Description | Root Cause |
|-------|----------|--------|-------------|------------|
| Record field tracking | HIGH | ✅ FIXED | `exec.ir` reported as undeclared `ir` | Extractor was adding suffix fields as separate reads; fixed in `extractReadsFromNode` to skip "suffix" field identifiers |
| Sensitivity list | HIGH | ✅ FIXED | Record fields missing from sensitivity analysis | Tied to record field tracking - fixed by same change |
| External name false positives | HIGH | ✅ FIXED | VHDL-2008 `<< signal .path >>` identifiers reported as undeclared | Extractor now skips `external_name` node children |
| Time unit false positives | MEDIUM | ✅ FIXED | `ns`, `ps`, etc. in `after 5 ns` reported as undeclared signals | Extractor now skips `physical_literal` node children |
| Wait statement combinational | MEDIUM | ✅ FIXED | Processes with `wait` flagged as "empty sensitivity combinational" | Added `HasWait` field; processes with wait aren't combinational |
| Input port driven cross-entity | HIGH | ✅ FIXED | Signals in entity B flagged as driving input ports from entity A | Rule now checks `arch.entity_name == port.in_entity` |
| OPA get_signal_width conflict | LOW | ✅ FIXED | Policy evaluation failed on multiple signals with same name | Changed to use `max()` for deterministic return value |
| Multi-driven in sequential | MEDIUM | TODO | Assignments in different if/case branches flagged | Rule doesn't account for mutual exclusion |

### False Positive Analysis Results (neorv32_cpu_control.vhd)

**Before fix:**
- 256 `undeclared_signal_usage`
- 52 `sensitivity_list_incomplete`

**After fix:**
- 0 `undeclared_signal_usage`
- 0 `sensitivity_list_incomplete`

### The Improvement Loop

1. **Run on external_tests** - collect violations
2. **Assume false positives** - investigate each one
3. **Find the gap** - what construct aren't we handling?
4. **Fix at the source**:
   - Grammar issue? Fix `grammar.js`
   - Extraction issue? Fix extractor
   - Policy too strict? Adjust Rego rules
5. **Verify** - violations should decrease
6. **Repeat** - until false positive rate is acceptable

### Performance Notes

Large files with complex expressions can be slow:
- `neorv32_cpu_alu_fpu.vhd` (117KB): ~7-8 seconds
- `neorv32_top.vhd` (73KB): ~300ms
- Most files: ~100ms

The slowdown is due to GLR parsing with many conflicts. This is a grammar complexity issue, not a bug.

---

## Current Capabilities

### Extraction
- [x] Entity declarations with ports
- [x] Architecture bodies
- [x] Package declarations
- [x] Signal declarations (multi-name)
- [x] Port declarations with direction
- [x] Process statements with sensitivity lists
- [x] Component instantiations with port/generic maps
- [x] Clock domain detection (rising_edge/falling_edge)
- [x] Reset pattern detection (async/sync)
- [x] Signal read/write analysis

### Policies
- [x] Naming conventions
- [x] Unresolved dependencies
- [x] Missing component definitions

### System-Level
- [x] Cross-file symbol resolution
- [x] Dependency tracking
- [x] Port map extraction

---

## Extractor Alignment Project

**Goal:** Make the extractor leverage the grammar's rich structure instead of parsing via `Content()` and string scanning.

**Philosophy:** "Grammar smart, extractor dumb" — the grammar should expose structured nodes with named fields; the extractor should do simple pattern matching on those nodes.

### Phase 1: Use Existing Grammar Features (No Grammar Changes)

| Task | Status | Notes |
|------|--------|-------|
| Use `association_element` formal/actual fields | ✅ DONE | `extractAssociationElement` now uses `ChildByFieldName("formal")` / `ChildByFieldName("actual")` |
| Use `ChildByFieldName("condition")` in if_statement | ✅ DONE | Uses `alias($._expression, $.condition)` wrapper; extractor uses `extractClockEdgeFromIfStatement` and `checkResetPattern` |
| Use `ChildByFieldName("expression")` in case_statement | ✅ DONE | `extractCaseExpressionReads` now uses `ChildByFieldName("expression")` |
| Use structured `relational_expression` node | ✅ DONE | Fixed duplicate extraction by adding early returns and parent-type checks |
| Use structured `multiplicative_expression` node | ✅ DONE | Fixed duplicate extraction for arithmetic operators |
| Handle sensitivity list `all` from AST | ✅ DONE | `extractSensitivityList` now detects "all" from first child content |
| Use `for_generate` fields (`loop_var`, `range`) | ✅ DONE | Added `extractRangeFromNode` to walk range AST instead of string parsing |

### Phase 2: Add Unified Name Helper

| Task | Status | Notes |
|------|--------|-------|
| Create `extractNameInfo(node)` helper | ✅ DONE | Returns `NameInfo{Base, FullPath, IsIndexed, IsAttribute, IsCall, AttrName, IndexExprs}` |
| Replace `extractBaseSignal` calls | ✅ DONE | Now calls `extractNameInfo(node).Base` |
| Replace `extractFullSignalPath` calls | ✅ DONE | Now calls `extractNameInfo(node).FullPath` |
| Replace `isFunctionCall` heuristics | ✅ DONE | Now uses AST parent-type checking + `isCommonVHDLFunction` |
| Fix signal read/write extraction | ✅ DONE | Refactored `extractReadsFromNode` to use `extractNameInfo` for unified name handling |

### Phase 3: Grammar Enhancements (Expose Hidden Structure)

| Task | Status | Notes |
|------|--------|-------|
| Add `field('condition')` to if_statement | ✅ DONE | Used `alias($._expression, $.condition)` to create wrapper node; ChildByFieldName works with wrapper |
| Add `field('target')` to signal assignments | ✅ DONE | Used `alias($._assignment_target, $.assignment_target)` wrapper in all signal assignment rules; simplified `extractConcurrentAssignment` and fixed `extractSignalDepsFromAssignment` |
| Add `field('waveform')` to signal assignments | ⏸️ SKIP | Not needed - extractor uses `extractReadsFromNode` which walks entire RHS |
| Expose `discrete_range` with `low`/`high`/`direction` fields | ✅ DONE | Created visible `discrete_range` wrapper and `range_expression` with `low`/`direction`/`high` fields; simplified `extractRangeFromNode` |
| Add visible `function_call` node | ⚠️ PARTIAL | Created visible `function_call` rule with `name`/`arguments` fields, but VHDL syntax is ambiguous - parser may prefer `_name` path; semantic analysis still needed |
| Make `_name` visible or add wrapper | ⏸️ SKIP | Current `_name` rule has `prefix`/`suffix`/`content` fields that extractor already uses; making `_name` visible would affect many grammar locations |

### Phase 4: Refactor Content() Parsers

| Task | Status | Notes |
|------|--------|-------|
| Refactor `extractConcurrentAssignment` | ✅ DONE | Now uses `ChildByFieldName("target")` instead of scanning for `<=`; removed 70+ lines of complex scanning logic |
| Refactor `checkResetPattern` | ⬜ TODO | Use `relational_expression` instead of flat sibling scan |
| Refactor `parseForGenerateRange` | ✅ DONE | `extractRangeFromNode` now uses `ChildByFieldName` with `low`/`direction`/`high` fields from `range_expression` |
| Refactor `extractSignals` type parsing | ⬜ TODO | Use grammar structure instead of byte positions |
| Refactor `extractTypeName` | ⬜ TODO | Use grammar fields |

### Phase 5: Semantic Analysis Improvements

| Task | Status | Notes |
|------|--------|-------|
| Clock edge detection from AST | ⬜ TODO | Consider adding `clock_edge_call` node |
| Reset polarity detection | ⬜ TODO | Use `relational_expression` to detect `= '0'` vs `= '1'` vs `/= '1'` |
| Improve CDC crossing detection | ⬜ TODO | Leverage better signal tracking |

### Progress Log

| Date | Changes |
|------|---------|
| 2026-01-25 | Created tracking section, began Phase 1 |
| 2026-01-25 | Phase 1: Fixed association_element, relational/multiplicative extraction, sensitivity list "all" |
| 2026-01-25 | Phase 2: Added `NameInfo` struct and `extractNameInfo` helper, replaced extractBaseSignal/extractFullSignalPath/isFunctionCall |
| 2026-01-25 | Phase 1: Refactored case_statement to use ChildByFieldName("expression"), added extractRangeFromNode for for_generate |
| 2026-01-25 | Phase 1 COMPLETE: Only if_statement condition blocked (needs grammar change) |
| 2026-01-25 | Phase 2: Refactored `extractReadsFromNode` to use `extractNameInfo` |
| 2026-01-25 | Phase 2 COMPLETE: All tasks done |
| 2026-01-25 | Phase 3: Added field('condition') to if_statement in grammar.js |
| 2026-01-25 | DISCOVERY: Go tree-sitter bindings FieldNameForChild() doesn't return field names for flat expression tokens - documented limitation |
| 2026-01-25 | SOLUTION: Use `alias($._expression, $.condition)` to create visible wrapper node - ChildByFieldName works with wrapper nodes |
| 2026-01-25 | CRITICAL GOTCHA: Must run `go clean -cache` after `npx tree-sitter generate` - CGO caches compiled C parser |
| 2026-01-25 | Phase 3: Added field('target') to all signal assignment rules using `alias($._assignment_target, $.assignment_target)` wrapper |
| 2026-01-25 | Phase 4: Refactored `extractConcurrentAssignment` - removed 70+ lines of complex `<=` scanning logic, now uses `ChildByFieldName("target")` |
| 2026-01-25 | Phase 4: Fixed `extractSignalDepsFromAssignment` to use the new assignment_target wrapper |
| 2026-01-25 | Phase 3: Added visible `discrete_range` wrapper and `range_expression` with `low`/`direction`/`high` fields |
| 2026-01-25 | Phase 4: Simplified `extractRangeFromNode` to use `ChildByFieldName` instead of manual child iteration |
| 2026-01-25 | Phase 3: Added visible `function_call` rule with `name`/`arguments` fields (limited by VHDL syntax ambiguity) |
| 2026-01-25 | **Phase 3 COMPLETE**: All practical grammar enhancements done; remaining items deferred (waveform not needed, _name too disruptive) |
| 2026-01-25 | FALSE POSITIVE ANALYSIS: Ran linter on external_tests/neorv32, found 256 `undeclared_signal_usage` violations |
| 2026-01-25 | ROOT CAUSE: Record field access `exec.ir` in wrapper nodes (like condition) creates flat `prefix`/`suffix` children, extractor was adding both as reads |
| 2026-01-25 | FIX: Updated `extractReadsFromNode` to check field names and skip identifiers with "suffix" or "content" field names |
| 2026-01-25 | RESULT: 256 `undeclared_signal_usage` → 0, 52 `sensitivity_list_incomplete` → 0 on neorv32_cpu_control.vhd |

---

## Future Work

### Grammar (improve IEEE 1076-2008 compliance)
- [ ] Generate statements
- [ ] Configuration declarations
- [ ] Block statements
- [ ] Protected types

### Extractors
- [ ] FSM state/transition extraction
- [ ] Attribute expressions (`'range`, `'length`)
- [ ] Aggregate analysis (`others => '0'`)
- [ ] Case statement coverage

### Policies
- [ ] Latch inference detection
- [ ] Multi-driver detection
- [ ] Clock domain crossing analysis
- [ ] Combinational loop detection
- [ ] FSM completeness checking
- [ ] Floating input detection

### System-Level
- [ ] Hierarchical signal tracing
- [ ] Cross-module clock mismatch detection
- [ ] Full hierarchy elaboration
