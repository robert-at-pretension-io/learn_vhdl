# VHDL Compliance Compiler

A compiler-grade VHDL linting and static analysis tool built with Go, Tree-sitter, OPA, and CUE.

## What Is This?

This stack transforms VHDL from "text files" into a "queryable database," enabling safety checks that were previously impossible without expensive proprietary tools.

**Current Status:**
| Metric | Value |
|--------|-------|
| Test Files | 12,762 |
| Valid Acceptance | 98.75% |
| Grammar Quality | 85.40% |

**Current Capabilities:**
- Parse VHDL with error recovery (98.75% acceptance across 12,700+ test files)
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
│                    - Naming conventions, unresolved deps                 │
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
- (Future) Clock domain crossing detection
- (Future) Latch inference detection

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

| Rank | Task | Impact | Blocking |
|------|------|--------|----------|
| **#1** | **Type System & Function Extraction** | Enables type-aware rules, width checking | Latch detection, overload resolution |
| **#2** | **Latch Inference Detection** | Critical synthesis rule | Needs type info for proper detection |
| **#3** | **Package Contents Indexing** | Complete cross-file resolution | Type lookups, constant evaluation |
| **#4** | **Generate Elaboration** | Evaluate for-generate ranges | Instance counting, signal tracing |
| **#5** | **CDC Enhancement** | Clock domain crossing analysis | Multi-clock designs |

### Phase 1: Grammar Completion (Current Focus)
- [x] 98.75% valid acceptance rate
- [x] Generate statements (if/for/case) - grammar and extraction
- [ ] Group declarations
- [ ] Generic packages with type parameters
- [ ] Matching case statements

### Phase 2: Library & Scope Model
- [x] Per-library symbol registration
- [x] Nested scope tracking (generate statements)
- [ ] Package contents indexing
- [ ] Library-qualified name resolution

### Phase 3: Type System (HIGH PRIORITY)
- [ ] Function/procedure extraction with signatures
- [ ] Type declaration extraction (records, enums, arrays)
- [ ] Constant extraction with values
- [ ] Type signature storage in indexer
- [ ] Overload resolution
- [ ] Type compatibility checking
- [ ] Width validation

### Phase 4: Advanced Analysis
- [ ] Latch inference detection
- [ ] Clock domain crossing analysis
- [ ] Multi-driver detection
- [ ] FSM extraction and completeness checking
- [ ] Combinational loop detection

---

## Future Work (Detailed)

### Grammar
- [ ] Generate statements (if/for/case generate)
- [ ] Group declarations and group templates
- [ ] Generic packages with `generic (type T)`
- [ ] Package instantiation `package p is new pkg generic map (...)`
- [ ] Matching case statement `case? expr is`
- [ ] Context clauses `context lib.ctx`
- [ ] Configuration specifications

### Extractors
- [ ] Function/procedure signatures with parameter types
- [ ] Package contents (types, constants, functions)
- [ ] Generate instance scopes with local signals
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
- [ ] Latch inference detection
- [ ] Multi-driver detection
- [ ] Clock domain crossing analysis
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

## License

MIT

## Contributing

Contributions welcome:
- Grammar improvements for missing VHDL constructs
- New OPA policy rules
- Semantic extractors
- Documentation
