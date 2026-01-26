# VHDL Compliance Compiler

A compiler-grade VHDL linting and static analysis tool built with Go, Tree-sitter, OPA, and CUE.

## What Is This?

This stack transforms VHDL from "text files" into a "queryable database," enabling safety checks that were previously impossible without expensive proprietary tools.

**Current Status:**
| Metric | Value |
|--------|-------|
| Test Files | 12,762 |
| Valid Acceptance | 100.00% |
| Grammar Quality | 80.97% |

**Current Capabilities:**
- Parse VHDL with error recovery (100% valid acceptance across 12,700+ test files)
- Extract semantic information: entities, architectures, signals, ports, processes
- Detect clock domains, reset patterns, and signal read/write analysis
- Extract component instantiations with port/generic mappings
- Cross-file symbol resolution and dependency tracking
- Policy-based linting with OPA (Rego rules)
- CUE contract validation to prevent silent failures

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           VHDL Source Files                              │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                    Tree-sitter Parser (grammar.js)                       │
│                    - Error-tolerant parsing                              │
│                    - Produces concrete syntax tree                       │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                    Go Extractor (internal/extractor)                     │
│                    - Entities, Architectures, Packages                   │
│                    - Signals, Ports, Processes                           │
│                    - Component Instances + Port Maps                     │
│                    - Clock Domains, Reset Detection                      │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                    Go Indexer (internal/indexer)                         │
│                    - Cross-file symbol table                             │
│                    - Dependency resolution                               │
│                    - Builds normalized OPA input                         │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                    CUE Validator (internal/validator)                    │
│                    - Contract enforcement (schema/ir.cue)                │
│                    - Prevents silent failures in OPA                     │
│                    - Crashes immediately on schema mismatch              │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                    OPA Policy Engine (policies/*.rego)                   │
│                    - Declarative compliance rules                        │
│                    - Latch detection, sensitivity analysis               │
│                    - Extensible rule packs                               │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                    Violations & Reports                                  │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Key Components

### 1. Tree-sitter Grammar (`tree-sitter-vhdl/grammar.js`)

**Role:** Error-tolerant VHDL parser.

- Parses VHDL with GLR algorithm that tolerates syntax errors
- Produces structured syntax tree even for incomplete code
- Supports case-insensitive keywords (VHDL requirement)
- Visible semantic nodes for extraction: `port_direction`, `sensitivity_list`, `association_element`, etc.

**Current Status:**
| Test Suite | Pass Rate |
|------------|-----------|
| Full external_tests | 94%+ (12,000+ files) |
| GRLIB, OSVVM, VUnit | Production VHDL-2008 |
| ghdl test suite | Extensive edge cases |

**Grammar Lessons:**
- Prefer deferred decisions for VHDL "syntactic homonyms" like `name(0)`; unify names instead of guessing array vs call.
- Avoid local-maximum hacks that only improve a narrow test set; aim for abstractions that generalize.
- Discrete ranges show up in loops, slices, and type constraints; handle subtype indications like `natural range 0 to ...`.
- Report strings can be qualified expressions (`string'("...")`); treat them as first-class expressions.
- Non-UTF8 test files are a data issue, not a grammar issue; exclude or re-encode rather than loosening the grammar.
- XPASS reduction is usually best served by tightening broad regex fallbacks before adding lots of tiny exceptions.

### 2. Go Extractor (`internal/extractor/`)

**Role:** Extract semantic facts from syntax trees.

**Extracted Facts:**
- **Entities** - name, file, line, ports
- **Architectures** - name, entity reference, file, line
- **Packages** - name, file, line
- **Signals** - name, type, containing entity
- **Ports** - name, direction (in/out/inout/buffer), type
- **Processes** - label, sensitivity list, clock/reset detection
- **Instances** - label, target entity, port map, generic map

**Semantic Analysis:**
- **Clock Domain Detection** - `rising_edge(clk)` / `falling_edge(clk)`
- **Reset Pattern Detection** - async vs sync, active high/low
- **Signal Read/Write Analysis** - which signals are read/written in each process

### 3. Go Indexer (`internal/indexer/`)

**Role:** Cross-file linking and symbol resolution.

- Builds global symbol table from all files
- Resolves dependencies (`use`, `library`, `component`)
- Detects missing imports and unresolved references
- Normalizes data for OPA consumption

### 4. CUE Validator (`internal/validator/`)

**Role:** Contract enforcement between Go and OPA.

The CUE schema (`schema/ir.cue`) defines the exact structure OPA expects. If the extractor produces data that doesn't match:
- **Without CUE:** OPA silently receives `undefined`, rules don't fire, you think code is clean
- **With CUE:** Immediate crash with clear error message

Example validation rules:
```cue
#Port: {
    name:      string & =~"^[a-zA-Z_][a-zA-Z0-9_]*$"
    direction: "in" | "out" | "inout" | "buffer" | "linkage" | ""
    type:      string & !=""
    line:      int & >=1
}
```

### 5. OPA Policy Engine (`policies/`)

**Role:** Declarative compliance rules in Rego.

Example rules:
- Entity naming conventions
- Unresolved dependencies
- Missing component definitions
- Clock domain crossing detection
- (Future) Latch inference detection

---

## Pain Points Addressed

This tool targets the four major time sinks in FPGA development:

### 1. The "20-Minute Typo" (Compilation Penalty) ✅ SOLVED

**The Pain:** In software, a typo is caught in 0.5 seconds. In FPGA land, you click "Synthesize," wait 20 minutes, and see `Error: Symbol 'rst_n' is not declared.`

**Our Solution:** Instant feedback. Catch undeclared symbols, missing dependencies, and orphan architectures before you ever touch the synthesis tool.

| Check | Rule | Status |
|-------|------|--------|
| Undeclared signals | `undeclared_signal_usage` | ✅ |
| Missing components | `unresolved_component` | ✅ |
| Bad use clauses | `unresolved_dependency` | ✅ |
| Orphan architectures | `orphan_architecture` | ✅ |
| Syntax errors | Tree-sitter GLR parser | ✅ |

**Value:** "Save 1 hour/day of synthesis wait time by catching 3 typos before you click Run."

---

### 2. "Who Drives This Signal?" (Spaghetti Code) ⚠️ PARTIAL

**The Pain:** You inherit a 10-year-old codebase. Signal `enable_rx` is toggling unexpectedly. Finding the driver means grepping 50 files and tracing port maps manually.

**Our Solution:** We extract the full dependency graph. The data exists - visualization is the gap.

| Feature | Status | Notes |
|---------|--------|-------|
| Signal dependency tracking | ✅ | `signal_deps` data structure |
| Instance hierarchy | ✅ | `instances` with port/generic maps |
| Signal read/write tracking | ✅ | `signal_usages` per process |
| `--show-driver` CLI | ❌ | **TODO** |
| Hierarchy graph export | ❌ | **TODO** |

**Planned:** `--show-driver enable_rx` → "Signal driven by process 'rx_ctrl' at line 142"

---

### 3. FSM Deadlocks and "Lockup" ⚠️ PARTIAL

**The Pain:** FSMs are the brain of the FPGA. If a state has no exit condition, the chip freezes. You can't see this in code - you have to simulate every path or stare at it for hours.

**Our Solution:** We detect unreachable and unhandled states. Dead states (no exit) are the gap.

| Check | Rule | Status |
|-------|------|--------|
| Unreachable states | `fsm_unreachable_state` | ✅ |
| Unhandled states in case | `fsm_unhandled_state` | ✅ |
| Missing default state | `fsm_missing_default_state` | ✅ |
| FSM without reset | `fsm_no_reset` | ✅ |
| Dead states (no exit) | ❌ | **TODO** |
| FSM graph export | ❌ | **TODO** |

**Example Dead State:**
```vhdl
case state is
  when S_INIT => state <= S_RUN;
  when S_RUN  => state <= S_DONE;
  when S_DONE => null; -- DEAD: no transition out!
end case;
```

---

### 4. The "Bit-Width Silent Killer" ⚠️ IN PROGRESS

**The Pain:** You connect a 12-bit signal to a 10-bit port. The tool silently truncates. The chip works "mostly" - until the counter overflows. This takes days to debug because the logic looks correct.

**Our Solution:** Width checking on port connections.

| Check | Rule | Status |
|-------|------|--------|
| Port width mismatch | `port_width_mismatch` | ✅ |
| Vector truncation | ❌ | **TODO** |
| Signal width estimation | ✅ | `estimateSignalWidth()` |

---

### Summary: Coverage vs. Pain Points

| Pain Point | Coverage | Marketing Hook |
|------------|----------|----------------|
| Synthesis Wait Times | ✅ 100% | "Catch typos before you click Synthesize" |
| Legacy Code Exploration | ⚠️ 70% | "Visualize the architecture instantly" |
| FSM Lockups | ⚠️ 80% | "Never ship a dead state" |
| Width Mismatches | ✅ 90% | "Never let a truncation kill your math" |

---

## Quick Start

```bash
# Build the tool
go build -o vhdl-lint ./cmd/vhdl-lint

# Lint a single file
./vhdl-lint ./test.vhd

# Lint a directory
./vhdl-lint ./src/

# Verbose output (shows extracted data)
./vhdl-lint -v ./src/
```

**Example Output:**
```
Found 1 VHDL files

=== Verbose: Extracted Ports ===
  test.clk: direction="in" type="std_logic"
  test.rst: direction="in" type="std_logic"
  test.q: direction="out" type="std_logic"

=== Verbose: Extracted Processes ===
  rtl.: sequential, sensitivity=[clk rst]
    clock: clk (rising_edge)
    reset: rst (async)
    writes: [q]
    reads: [d internal]

=== Verbose: Clock Domains ===
  clk (rising): drives [q]

=== Verbose: Instances ===
  u_cpu: work.cpu
    generics: map[WIDTH:8]
    ports: map[clk:clk dout:internal_data rst:rst]

=== Policy Summary ===
  Errors:   0
  Warnings: 0
  Info:     0

=== Extraction Summary ===
  Files:    1
  Symbols:  1
  Entities: 1
  Packages: 0
  Signals:  2
  Ports:    3
```

---

## Grammar Development

### The Golden Rule: Fix Issues at the Grammar Level

**CRITICAL:** When encountering parse issues or false positives in linting, ALWAYS fix the problem at the grammar level (`grammar.js`), not in downstream components (extractor, policy rules).

**Why This Matters:**

1. **ERROR nodes corrupt extraction.** When the grammar can't parse a construct, Tree-sitter creates `ERROR` nodes. These nodes can contain raw text including VHDL keywords that get misidentified as signal names, causing false positives.

2. **Workarounds compound.** Filtering keywords in the extractor or adding skip lists in OPA rules creates technical debt. Each workaround masks the root cause and makes the codebase harder to maintain.

3. **The grammar IS the source of truth.** If a VHDL construct isn't properly defined in `grammar.js`, no amount of downstream filtering will fix it correctly.

**Example of Wrong vs Right:**

```
WRONG: Grammar can't parse `2**func(x)`, so extractor extracts "downto" as a signal.
       -> Add "downto" to skip list in OPA rules  ❌

RIGHT: Fix grammar to properly parse exponentiation with function calls.
       -> ERROR nodes disappear, keywords are never misidentified ✅
```

**When You See False Positives:**

1. First, check if the file parses cleanly: `npx tree-sitter parse file.vhd`
2. If there are ERROR nodes, find what construct the grammar can't handle
3. Fix the grammar rule in `grammar.js`
4. Regenerate and rebuild

### The Grammar Improvement Cycle

Grammar development follows a tight iterative loop:

```
┌──────────────────┐     ┌──────────────────┐     ┌──────────────────┐
│ ./test_grammar.sh│────▶│ Find ERROR nodes │────▶│ Fix grammar.js   │
│ (measure)        │     │ (identify)       │     │ (improve)        │
└──────────────────┘     └──────────────────┘     └────────┬─────────┘
        ▲                                                   │
        │                                                   ▼
        │                                         ┌──────────────────┐
        │                                         │ npx tree-sitter  │
        │                                         │ generate         │
        │                                         └────────┬─────────┘
        │                                                   │
        │                                                   ▼
        │                                         ┌──────────────────┐
        └─────────────────────────────────────────│ go build         │
                                                  │ (rebuild linter) │
                                                  └──────────────────┘
```

**Step 1: Measure baseline**
```bash
# Test against all external VHDL projects
./test_grammar.sh external_tests

# Output shows pass rate:
# Total: 1247, Pass: 1089, Fail: 158, Score: 87.33%
```

**Step 2: Find what's failing**
```bash
# See ERROR nodes in a specific file
cd tree-sitter-vhdl
npx tree-sitter parse ../external_tests/neorv32/rtl/core/neorv32_cpu.vhd 2>&1 | grep ERROR
```

**Step 3: Fix the grammar**
```bash
# Edit grammar rule
vim grammar.js

# Regenerate parser
npx tree-sitter generate

# Quick test - ERROR should be gone
npx tree-sitter parse ../external_tests/neorv32/rtl/core/neorv32_cpu.vhd 2>&1 | grep ERROR
```

**Step 4: Rebuild and verify**
```bash
cd ..
go build -o vhdl-lint ./cmd/vhdl-lint
./test_grammar.sh external_tests  # Score should improve!
```

**Step 5: Repeat** until pass rate is acceptable.

### Development Commands

```bash
# Edit grammar
vim tree-sitter-vhdl/grammar.js

# Regenerate parser
cd tree-sitter-vhdl && npx tree-sitter generate

# Rebuild Go linter (picks up new parser)
go build -o vhdl-lint ./cmd/vhdl-lint

# Test grammar against external projects
./test_grammar.sh external_tests

# Test against specific project
./test_grammar.sh external_tests/neorv32

# Limit to first N files (faster iteration)
./test_grammar.sh external_tests 50

# Check a specific file for ERROR nodes
cd tree-sitter-vhdl
npx tree-sitter parse /path/to/file.vhd 2>&1 | grep ERROR

# Full parse tree for debugging
npx tree-sitter parse /path/to/file.vhd > /tmp/tree.txt
```

### Common ERROR Patterns

| ERROR near | Likely cause | Where to fix |
|------------|--------------|--------------|
| `<=` | Signal assignment target | `_simple_signal_assignment` |
| `when ... else` | Conditional expression | `_conditional_signal_assignment` |
| `generate` | Generate statement | `generate_statement` |
| `'attr` | Attribute expression | `attribute_name` |
| `**` | Exponentiation | Operator precedence |
| `(others => '0')` | Aggregate | `_aggregate` rules |

### Lessons Learned (Keep These Handy)

- Treat `external_tests` results as signal, not verdict. Most failures are grammar gaps, not bugs in real-world code.
- Use `ANALYZE=1 ./test_grammar.sh` to see the most common failing constructs before changing anything.
- Exclude known anti-tests (`non_compliant`, `negative`, `analyzer_failure`) from the pass rate, but still review them when debugging.
- Prefer fixing parse ambiguity with minimal conflicts; avoid broad fallback rules unless you also re-run `test_grammar.sh`.
- If `npx tree-sitter generate` panics, run `npm run build` in `tree-sitter-vhdl/` (uses the pinned CLI).

---

## Project Structure

```
.
├── tree-sitter-vhdl/
│   ├── grammar.js              # VHDL grammar definition
│   ├── src/                    # Generated C parser + external scanner
│   │   ├── parser.c            # Generated by tree-sitter
│   │   └── scanner.c           # Hand-written external scanner (bit strings)
│   └── bindings/go/            # Go bindings
├── cmd/
│   └── vhdl-lint/main.go       # CLI entry point
├── internal/
│   ├── extractor/              # Fact extraction from syntax tree
│   ├── indexer/                # Cross-file linking, symbol table
│   ├── policy/                 # OPA integration
│   └── validator/              # CUE schema validation
├── policies/
│   └── *.rego                  # OPA policy rules (modular)
├── schema/
│   └── ir.cue                  # CUE schema contract
├── external_tests/             # Real-world VHDL for testing (see below)
└── test_grammar.sh             # Grammar testing script
```

---

## External Tests: Improving the Tool, Not Finding Bugs

The `external_tests/` directory contains **12,000+ VHDL files** from real-world projects:

| Project | Files | Description |
|---------|-------|-------------|
| ghdl | ~9,600 | GHDL test suite |
| grlib | ~800 | LEON3/5 SPARC + SoC IP |
| osvvm | ~640 | VHDL-2008 verification |
| vunit | ~330 | Unit testing framework |
| PoC | ~290 | VLSI-EDA IP cores |
| hdl4fpga | ~270 | FPGA IP library |
| neorv32 | ~70 | RISC-V processor |
| Compliance-Tests | ~80 | IEEE 1076-2008 |

### Key Philosophy: Assume False Positives

**When running against external_tests, violations are likely FALSE POSITIVES.**

These are production-quality projects that:
- Have been synthesized and deployed on real FPGAs
- Are reviewed by experienced hardware engineers
- Work correctly with commercial VHDL tools

If the linter reports `unused_signal 'cpu_trace'` but the signal is used in a generate statement, **our tool has a bug, not neorv32.**

### Using External Tests

```bash
# Run against real-world code
./vhdl-lint external_tests/neorv32/rtl/core/

# Investigate violations (they're probably false positives)
./vhdl-lint -v external_tests/neorv32/rtl/core/neorv32_top.vhd

# Check for grammar issues
cd tree-sitter-vhdl
npx tree-sitter parse ../external_tests/neorv32/rtl/core/neorv32_top.vhd 2>&1 | grep ERROR
```

### Efficient Grammar Testing with Caching

Running 12,000+ files takes time. The test script caches results for fast iteration:

```bash
# Step 1: Run full suite ONCE to build cache (a few minutes)
ANALYZE=1 ./test_grammar.sh external_tests

# This creates cache files:
#   .grammar_fail_counts  - Error count per file (worst first)
#   .grammar_fail_list    - All failing files
#   .grammar_focus_list   - Top N failures

# Step 2: Analyze cache to find worst offenders
cat .grammar_fail_counts | head -20

# Step 3: Fix grammar.js based on analysis
# Step 4: Fast re-test with focus mode (instant feedback)
FOCUS_FAILS=1 ./test_grammar.sh        # Re-test only failures
FOCUS_TOP=50 ./test_grammar.sh         # Re-test top 50 worst

# Step 5: Verify improvement with full suite
ANALYZE=1 ./test_grammar.sh external_tests
```

**Key insight:** Don't re-run 12,000 files after every edit. Use FOCUS mode for iteration, full suite for verification.

### The Improvement Loop

**Phase 1: Grammar (fix parsing first)**
```bash
# 1. Run grammar tests to see baseline
./test_grammar.sh external_tests/neorv32

# 2. Find files with ERROR nodes
cd tree-sitter-vhdl
npx tree-sitter parse ../external_tests/neorv32/rtl/core/neorv32_cpu.vhd 2>&1 | grep ERROR

# 3. Identify the VHDL construct causing the ERROR
# 4. Fix grammar.js to handle that construct
# 5. Regenerate: npx tree-sitter generate
# 6. Verify ERROR is gone
# 7. Rebuild Go: go build -o vhdl-lint ./cmd/vhdl-lint
# 8. Repeat until pass rate improves
```

**Phase 2: Extraction (once parsing is clean)**
```bash
# Run linter and look at violations
./vhdl-lint external_tests/neorv32/rtl/core/

# For each violation, ask:
# - Is this a real bug? (unlikely for production code)
# - Or is extraction missing something?

# Common extraction gaps:
# - Signal used in generate statement not tracked
# - Port map associations not extracted
# - Record field access not handled
```

**Phase 2b: Semantic XPASS Loop (shrink semantic misses)**
```bash
# 1. Build the XPASS list with grammar tests
ANALYZE=1 ./test_grammar.sh external_tests

# 2. Run the linter only on semantic negatives (analyzer_failure)
LINT_XPASS=1 LINT_FILTER=semantic LINT_MAX_FILES=0 ./test_grammar.sh

# Optional: focus syntax negatives instead
LINT_XPASS=1 LINT_FILTER=syntax LINT_MAX_FILES=0 ./test_grammar.sh

# 3. Use the output to drive new rules:
#    - "Top rule hits" -> where rules already catch semantics
#    - "Clean" list -> missing semantic rules or extractor gaps
# 4. Add/adjust Rego rules, then repeat
```

**Semantic XPASS checklist**
- Clean list shrinking? If not, add rules or extractor coverage.
- Rule hits dominated by style/naming? Consider disabling for semantic runs.
- Unexpected clean files in analyzer_failure? Inspect for missing read/write extraction.
- Extractors are imperfect: always run the linter on known-good (semantically valid) code to catch false positives early.

**Phase 3: Policy (tune false positive rate)**
```bash
# If extraction is correct but policy is too strict:
# - Adjust Rego rules in policies/*.rego
# - Add exceptions for known-good patterns
# - Filter out enum literals, constants, keywords
```

**The Golden Rule:** Fix issues at the lowest level possible:
1. Grammar issues -> fix grammar.js
2. Extraction issues -> fix extractor
3. Only then -> adjust policies

Never add workarounds in policies for grammar/extraction bugs!

---

## Known Limitations & Roadmap

This section documents the critical gaps that must be addressed for production use. These are the "hidden demons" that kill VHDL tools in complex environments.

### 1. Library Mapping Problem (Namespace Resolution) ✅ IMPLEMENTED

**The Problem:** VHDL files exist inside *libraries*, not in a vacuum. The `work` library is relative - it changes based on how files are compiled. If `file_A.vhd` is compiled into `lib_common` and `file_B.vhd` into `lib_main`, they cannot see each other via `work`.

**Implementation Status: COMPLETE**
- ✅ Config supports `LibraryConfig` with file glob patterns
- ✅ `FileLibraries` map tracks which files belong to which libraries
- ✅ Symbols registered with actual library prefix (e.g., `mylib.entity_name`)
- ✅ Dependency resolution translates `work.x` to file's actual library

**How it works:**
```go
// When registering symbols, use actual library name
libName := idx.FileLibraries[f].LibraryName  // e.g., "mylib"
idx.Symbols.Add(Symbol{
    Name: fmt.Sprintf("%s.%s", libName, entity.Name),
    ...
})

// When resolving dependencies, translate "work" references
if strings.HasPrefix(qualName, "work.") {
    qualName = fileLib + qualName[4:]  // "work.utils" -> "mylib.utils"
}
```

**Remaining Work:**
- [ ] Index package contents (types, constants, functions)
- [ ] Support library-qualified names in expressions

### 2. Generate Statement Handling (Scopes & Elaboration) ✅ IMPLEMENTED

**The Problem:** Generate statements (`if`/`for`/`case` generate) create conditional, parameterized hardware. They're not edge cases in modern VHDL - they're the backbone of reusable IP.

**Implementation Status: COMPLETE**
- ✅ Grammar: `for_generate`, `if_generate`, `case_generate` as visible nodes
- ✅ Grammar: Field annotations for `loop_var`, `range`, `condition`, `expression`
- ✅ Extractor: `GenerateStatement` type with nested declarations
- ✅ Extractor: Recursive extraction of signals, instances, processes inside generates
- ✅ Extractor: Scoped signal tracking (e.g., `arch.gen_label.signal`)
- ✅ Policy/CUE: Generate statement schema validation

**How it works:**
```go
// Grammar exposes generate type as visible node
case "for_generate":
    gen.Kind = "for"
    gen.LoopVar = child.ChildByFieldName("loop_var").Content(source)

// Extractor recursively extracts nested content
func extractGenerateBody(node *sitter.Node, ...) {
    switch n.Type() {
    case "signal_declaration":
        gen.Signals = append(gen.Signals, e.extractSignals(n, ...))
    case "process_statement":
        gen.Processes = append(gen.Processes, e.extractProcess(n, ...))
    case "generate_statement":
        gen.Generates = append(gen.Generates, e.extractGenerateStatement(n, ...))
    }
}
```

**Remaining Work:**
- [ ] Conditional elaboration (evaluate generate conditions)
- [ ] Loop unrolling for analysis

### 3. Type Resolution & Overloading

**The Problem:** VHDL allows function overloading - multiple functions with the same name, distinguished only by argument types. Without type resolution, you cannot know which function is being called.

**Current State:**
- Signal `Type` field stored as string
- No function/procedure extraction
- No overload resolution
- No type compatibility checking
- No width validation

**Impact:**
- Cannot implement "unused function" detection (wrong overload flagged)
- Cannot check signal assignment width compatibility
- Cannot validate function argument types
- Cannot detect type mismatches

**Required Fix:**
- [ ] Extractor: Extract function/procedure signatures
- [ ] Indexer: Multi-pass type resolution
  - Pass 1: Collect all types and subprograms
  - Pass 2: Resolve signal types
  - Pass 3: Resolve expression types to match function signatures
- [ ] Policy: Type-aware rules (width checks, type compatibility)

### 4. Additional Grammar Gaps

These constructs cause parse failures in production code:

| Construct | Files Affected | Priority |
|-----------|----------------|----------|
| `group` declarations | 15 (J-Core) | Medium |
| `report (expr)` with parens | 2 (UVVM) | Low |
| Generic packages/types | ~15 | High |
| Case generate | ~5 | High |
| Matching case (`case?`) | ~5 | Medium |

---

## Prioritized Roadmap

Based on impact analysis, these are the highest-value improvements ranked by ROI:

| Rank | Task | Impact | Status |
|------|------|--------|--------|
| **#1** | **Type System & Function Extraction** | Enables type-aware rules, width checking | ✅ IMPLEMENTED |
| **#2** | **Latch Inference Detection** | Critical synthesis rule | ✅ IMPLEMENTED |
| **#3** | **Package Contents Indexing** | Complete cross-file resolution | ✅ IMPLEMENTED |
| **#4** | **Generate Elaboration** | Evaluate for-generate ranges | ✅ IMPLEMENTED |
| **#5** | **CDC Enhancement** | Clock domain crossing analysis | ✅ IMPLEMENTED |

### Phase 1: Grammar Completion (Current Focus)
- [x] 98.75% valid acceptance rate
- [x] Generate statements (if/for/case) - grammar and extraction
- [ ] Group declarations
- [ ] Generic packages with type parameters
- [ ] Matching case statements

### Phase 2: Library & Scope Model ✅ IMPLEMENTED
- [x] Per-library symbol registration
- [x] Nested scope tracking (generate statements)
- [x] Package contents indexing (types, constants, functions, procedures)
- [x] Library-qualified name resolution (work.pkg.type format)

### Phase 3: Type System ✅ IMPLEMENTED
- [x] Function/procedure extraction with signatures
- [x] Type declaration extraction (records, enums, arrays)
- [x] Subtype declaration extraction
- [x] Parameter extraction (direction, type, class, default)
- [ ] Overload resolution (future)
- [ ] Type compatibility checking (future)
- [ ] Width validation (future)

### Phase 4: Advanced Analysis
- [x] Latch inference detection ✅ IMPLEMENTED
  - Incomplete case statements (missing `when others`)
  - Enum case without full coverage (uses type system!)
  - Combinational process feedback detection
  - FSM state signals without reset
- [x] Generate elaboration ✅ IMPLEMENTED
  - Evaluate for-generate ranges (0 to 7, WIDTH-1 downto 0)
  - Simple constant expression evaluation (+, -, *, /)
  - Tracks iteration counts for instance counting
- [x] Clock domain crossing analysis
  - Detects unsynchronized single-bit and multi-bit crossings
  - Identifies synchronizer patterns (2+ stage flip-flop chains)
  - Rules: `cdc_unsync_single_bit`, `cdc_unsync_multi_bit`, `cdc_insufficient_sync`
- [ ] Multi-driver detection
- [ ] FSM extraction and completeness checking
- [ ] Combinational loop detection

---

## Future Work (Detailed)

### Grammar
- [x] Generate statements (if/for/case generate)
- [ ] Group declarations and group templates
- [ ] Generic packages with `generic (type T)`
- [ ] Package instantiation `package p is new pkg generic map (...)`
- [ ] Matching case statement `case? expr is`
- [ ] Context clauses `context lib.ctx`
- [ ] Configuration specifications

### Extractors
- [x] Function/procedure signatures with parameter types
- [x] Type declarations (enum, record, array, physical, range)
- [x] Subtype declarations with constraints
- [x] Generate instance scopes with local signals
- [ ] Package contents indexing in symbol table
- [ ] FSM state/transition extraction
- [ ] Attribute expressions (`'range`, `'length`, `'high`, `'low`)

### Indexer
- [ ] Per-library symbol tables
- [ ] Package dependency resolution
- [ ] Type inheritance and subtype tracking
- [ ] Overloaded function resolution
- [ ] Scope-aware symbol lookup

### Policies
- [ ] Type-aware width checking
- [x] Latch inference detection
- [ ] Multi-driver detection
- [x] Clock domain crossing analysis
- [ ] Combinational loop detection
- [ ] FSM completeness checking
- [ ] Floating input detection

### System-Level
- [x] Component instantiation extraction
- [x] Port map extraction
- [ ] Hierarchical signal tracing
- [ ] Cross-module clock mismatch detection
- [ ] Full design elaboration

---

## Complete Rule Reference

The linter includes **80+ rules** organized by category. All rules can be configured via `vhdl_lint.json`.

### Signal Rules (`policies/signals.rego`)

| Rule | Severity | Description |
|------|----------|-------------|
| `unused_signal` | Warning | Signal declared but never read or written |
| `undriven_signal` | Error | Signal read but never assigned |
| `multi_driven_signal` | Error | Signal assigned in multiple places (multi-driver) |
| `undeclared_signal_usage` | Error | Signal used but not declared in scope |
| `input_port_driven` | Error | Input port illegally driven inside architecture |
| `wide_signal` | Info | Signal wider than 128 bits |
| `duplicate_signal_name` | Info | Same signal name in different entities |

### Port Rules (`policies/ports.rego`)

| Rule | Severity | Description |
|------|----------|-------------|
| `unused_input_port` | Warning | Input port never read |
| `undriven_output_port` | Error | Output port never assigned (floating) |
| `output_port_read` | Error | Output port read internally (illegal in VHDL-93) |
| `inout_as_output` | Info | Bidirectional port only written, never read |
| `inout_as_input` | Info | Bidirectional port only read, never written |

### Process Rules (`policies/processes.rego`, `policies/sequential.rego`, `policies/combinational.rego`)

| Rule | Severity | Description |
|------|----------|-------------|
| `sensitivity_list_incomplete` | Warning | Signal read in process but missing from sensitivity |
| `sensitivity_list_superfluous` | Info | Signal in sensitivity list but never read |
| `missing_clock_sensitivity` | Error | Sequential process missing clock in sensitivity |
| `missing_reset_sensitivity` | Warning | Process has reset but not in sensitivity |
| `mixed_edge_clocking` | Warning | Both rising and falling edge on same clock |
| `signal_in_seq_and_comb` | Warning | Signal assigned in both sequential and combinational |
| `complex_process` | Info | Process with many assignments (complexity) |
| `process_label_missing` | Info | Process without label |
| `empty_sensitivity_combinational` | Warning | Combinational process with empty sensitivity list |
| `large_combinational_process` | Info | Large combinational logic block |

### Clock & Reset Rules (`policies/clocks_resets.rego`, `policies/rdc.rego`)

| Rule | Severity | Description |
|------|----------|-------------|
| `missing_reset` | Warning | Sequential process without reset |
| `multiple_clocks_in_process` | Error | More than one clock edge in same process |
| `clock_not_std_logic` | Warning | Clock signal not std_logic type |
| `reset_not_std_logic` | Warning | Reset signal not std_logic type |
| `async_reset_active_high` | Info | Async reset uses active-high (vs active-low convention) |
| `async_reset_unsynchronized` | Warning | Async reset crosses clock domain without sync |
| `reset_crosses_domains` | Warning | Reset signal used in different clock domains |
| `combinational_reset_gen` | Warning | Reset generated by combinational logic |
| `short_reset_sync` | Warning | Reset synchronizer too short (<2 stages) |

### CDC Rules (`policies/cdc.rego`)

| Rule | Severity | Description |
|------|----------|-------------|
| `cdc_unsync_single_bit` | Warning | Single-bit signal crosses domain unsynchronized |
| `cdc_unsync_multi_bit` | Error | Multi-bit signal crosses domain (needs handshake) |
| `cdc_insufficient_sync` | Warning | CDC synchronizer has <2 flip-flop stages |

### Latch Rules (`policies/latch.rego`)

| Rule | Severity | Description |
|------|----------|-------------|
| `incomplete_case_latch` | Error | Case statement missing `when others` in combinational |
| `enum_case_incomplete` | Warning | Enum case doesn't cover all literals |
| `combinational_incomplete_assignment` | Warning | Not all signals assigned in all branches |
| `fsm_no_reset` | Warning | FSM state signal has no reset value |
| `many_signals_no_default` | Warning | Many signals without default in combinational |

### Combinational Loop Rules (`policies/combinational.rego`)

| Rule | Severity | Description |
|------|----------|-------------|
| `direct_combinational_loop` | Error | Signal depends on itself (A -> A) |
| `two_stage_loop` | Error | Two-stage feedback loop (A -> B -> A) |
| `three_stage_loop` | Error | Three-stage feedback loop |
| `cross_process_loop` | Error | Loop across multiple processes |
| `potential_comb_loop` | Warning | Potential loop detected (heuristic) |
| `combinational_feedback` | Warning | Process output feeds back to input |

### FSM Rules (`policies/fsm.rego`)

| Rule | Severity | Description |
|------|----------|-------------|
| `fsm_missing_default_state` | Error | FSM case without `when others` |
| `fsm_unhandled_state` | Warning | Enum state not handled in case statement |
| `fsm_unreachable_state` | Warning | State never assigned (unreachable) |
| `state_signal_not_enum` | Info | State signal should use enum type |
| `single_state_signal` | Info | Only one state in FSM (trivial) |

### Synthesis Rules (`policies/synthesis.rego`)

| Rule | Severity | Description |
|------|----------|-------------|
| `unregistered_output` | Warning | Output port driven by combinational logic |
| `gated_clock_detection` | Warning | Clock signal assigned (potential gated clock) |
| `multiple_clock_domains` | Info | Design has multiple clock domains |
| `signal_crosses_clock_domain` | Warning | Signal used in different clock domains |
| `very_wide_bus` | Info | Bus wider than 256 bits |
| `combinational_reset` | Warning | Reset generated combinationally |
| `potential_memory_inference` | Info | Array assignment pattern may infer RAM |

### Hierarchy Rules (`policies/hierarchy.rego`)

| Rule | Severity | Description |
|------|----------|-------------|
| `floating_instance_input` | Error | Instance input port not connected |
| `sparse_port_map` | Warning | Port map missing many connections |
| `empty_port_map` | Warning | Instance with no port connections |
| `hardcoded_port_value` | Info | Literal value in port map |
| `open_port_connection` | Info | Port explicitly left open |
| `many_instances` | Info | Architecture has many instances |
| `instance_name_matches_component` | Info | Instance name same as component (confusing) |

### Instance Rules (`policies/instances.rego`)

| Rule | Severity | Description |
|------|----------|-------------|
| `positional_mapping` | Warning | Port map uses positional instead of named |
| `instance_naming_convention` | Info | Instance doesn't follow u_ or i_ prefix |

### Naming Rules (`policies/naming.rego`)

| Rule | Severity | Description |
|------|----------|-------------|
| `entity_naming` | Info | Entity name doesn't follow convention |
| `signal_input_naming` | Info | Input signal missing _i suffix |
| `signal_output_naming` | Info | Output signal missing _o suffix |
| `active_low_naming` | Info | Active-low signal missing _n suffix |
| `async_reset_naming` | Info | Async reset not named rst/rstn/arst |

### Style Rules (`policies/style.rego`)

| Rule | Severity | Description |
|------|----------|-------------|
| `large_entity` | Warning | Entity has >50 ports |
| `multiple_entities_per_file` | Info | File contains multiple entities |
| `legacy_packages` | Info | Using std_logic_arith instead of numeric_std |
| `architecture_naming_convention` | Info | Architecture not named rtl/behavioral/structural |
| `empty_architecture` | Warning | Architecture has no content |

### Quality Rules (`policies/quality.rego`)

| Rule | Severity | Description |
|------|----------|-------------|
| `trivial_architecture` | Warning | Architecture with no statements |
| `file_entity_mismatch` | Info | Filename doesn't match entity name |
| `buffer_port` | Warning | Deprecated buffer port mode |
| `bidirectional_port` | Info | Inout port (often problematic) |
| `very_long_file` | Info | File with >5 design units |
| `large_package` | Info | Package with >50 items |
| `short_signal_name` | Info | Single-character signal name |
| `long_signal_name` | Info | Signal name >40 characters |
| `duplicate_signal_in_entity` | Error | Same signal declared twice |
| `unlabeled_generate` | Warning | Generate block without required label |
| `many_signals` | Info | Entity has >50 signals |
| `deep_generate_nesting` | Info | Generate nested >3 levels deep |
| `magic_width_number` | Info | Signal width is magic number |
| `hardcoded_generic` | Info | Instance generic is literal number |

### Security Rules (`policies/security.rego`)

| Rule | Severity | Description |
|------|----------|-------------|
| `large_literal_comparison` | Warning | Comparison against >32-bit literal (trojan trigger) |
| `magic_number_comparison` | Info | Comparison against hardcoded value |
| `trigger_drives_output` | Warning | Comparison directly drives output |
| `counter_trigger` | Warning | Counter value used as comparison trigger |
| `multi_trigger_process` | Warning | Process with multiple trigger comparisons |

### Power Rules (`policies/power.rego`)

| Rule | Severity | Description |
|------|----------|-------------|
| `unguarded_multiplication` | Warning | Multiplier without clock enable |
| `unguarded_division` | Warning | Divider without clock enable |
| `unguarded_exponent` | Warning | Exponent operation without guard |
| `power_hotspot` | Info | Multiple expensive operations in one process |
| `combinational_multiplier` | Warning | Multiplier in combinational logic |
| `clock_gating_opportunity` | Info | Logic could benefit from clock gating |

### Testbench Rules (`policies/testbench.rego`)

| Rule | Severity | Description |
|------|----------|-------------|
| `testbench_with_ports` | Warning | Testbench entity has ports |
| `entity_no_ports_not_tb` | Info | Entity without ports not named *_tb |
| `mismatched_tb_architecture` | Info | Testbench arch not named tb/testbench |

### Core Rules (`policies/core.rego`)

| Rule | Severity | Description |
|------|----------|-------------|
| `orphan_architecture` | Warning | Architecture without matching entity |
| `entity_without_arch` | Warning | Entity without any architecture |
| `unresolved_component` | Warning | Component declaration not found |
| `unresolved_dependency` | Warning | Use clause target not found |
| `potential_latch` | Warning | Potential latch inference detected |

---

## Configuring Rules

Create `vhdl_lint.json` to customize rule severities:

```json
{
  "rules": {
    "unused_signal": "warning",
    "multi_driven_signal": "error",
    "short_signal_name": "off",
    "large_entity": "info"
  }
}
```

Valid severity values: `"off"`, `"info"`, `"warning"`, `"error"`

Run `vhdl-lint init` to generate a default configuration.

---

## JSON Output

For CI/CD integration, use JSON output mode:

```bash
# Output as JSON
./vhdl-lint --json ./src/

# Example: count violations by rule
./vhdl-lint --json ./src/ | jq '.violations | group_by(.rule) | map({rule: .[0].rule, count: length})'

# Example: filter only errors
./vhdl-lint --json ./src/ | jq '.violations[] | select(.severity == "error")'
```

JSON schema is validated with CUE (`internal/validator/output_schema.cue`).

---

## License

MIT

## Contributing

Contributions welcome:
- Grammar improvements for missing VHDL constructs
- New OPA policy rules
- Semantic extractors
- Documentation
