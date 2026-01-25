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

**Current Status:** 70.21% IEEE 1076-2008 compliance (132/188 tests passing)

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
- ✅ Rebuild Go: `go build ./cmd/vhdl-lint`

**To debug parse failures:**
```bash
# See ERROR nodes in a file
npx tree-sitter parse file.vhd 2>&1 | grep ERROR

# Get full parse tree
npx tree-sitter parse file.vhd
```

#### Recent Lessons & Advice

- Use `ANALYZE=1 ./test_grammar.sh` to identify the top failing constructs before editing grammar.
- Exclude anti-tests (`*/non_compliant/*`, `*/negative/*`, `*/analyzer_failure/*`) from the score, but still inspect them for grammar clues.
- Favor minimal `conflicts` additions over broad regex fallbacks; re-run `test_grammar.sh` after each change.
- If `npx tree-sitter generate` panics, run `npm run build` in `tree-sitter-vhdl/` (pinned CLI).
- Prefer deferred decisions for VHDL "syntactic homonyms" like `name(0)`; unify names instead of guessing array vs call.
- Avoid local-maximum hacks that only improve a narrow test set; aim for abstractions that generalize.

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

## External Tests: A Tool for Improvement, Not Error Finding

The `external_tests/` directory contains real-world VHDL projects:

- **neorv32** - RISC-V processor (production quality)
- **hdl4fpga** - FPGA IP library
- **PoC** - VLSI-EDA IP cores
- **ghdl** - GHDL test suite
- **Compliance-Tests** - IEEE 1076-2008 compliance tests

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
| `unused_signal` | Signal used in generate/port map not tracked | Extractor |
| `undriven_signal` | Assignment in generate statement missed | Grammar + Extractor |
| `sensitivity_list_incomplete` | Complex expression reads not extracted | Extractor |
| `unresolved_dependency` | Package/library not in search path | Indexer |

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
