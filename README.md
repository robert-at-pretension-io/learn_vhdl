# VHDL Compliance Compiler

A production-ready VHDL linting and compliance tool built with Go, Tree-sitter, OPA, and CUE.

## What Is This?

This stack transforms VHDL from "text files" into a "queryable database," enabling safety checks that were previously impossible without expensive proprietary tools.

**Current Capabilities:**
- Parse VHDL with error recovery (70%+ IEEE 1076-2008 compliance)
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
| VHDL 2008 Compliance-Tests | 100% (29/29) |
| IEEE 1076-2008 | 70.21% (132/188) |

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

The `external_tests/` directory contains real-world VHDL projects:

| Project | Description | Files |
|---------|-------------|-------|
| neorv32 | RISC-V processor | ~70 |
| hdl4fpga | FPGA IP library | ~270 |
| PoC | VLSI-EDA IP cores | varies |
| ghdl | GHDL test suite | varies |
| Compliance-Tests | IEEE 1076-2008 | 188 |

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

## Future Work

### Extractors
- [ ] FSM state/transition extraction
- [ ] Generate statement handling
- [ ] Attribute extraction (`'range`, `'length`, etc.)
- [ ] Aggregate expression analysis (`others => '0'`)

### Policies
- [ ] Latch inference detection
- [ ] Multi-driver detection
- [ ] Clock domain crossing analysis
- [ ] Combinational loop detection
- [ ] FSM completeness checking

### System-Level Analysis
- [x] Component instantiation extraction
- [x] Port map extraction
- [ ] Hierarchical signal tracing
- [ ] Cross-module clock mismatch detection
- [ ] Floating input detection

---

## License

MIT

## Contributing

Contributions welcome:
- Grammar improvements for missing VHDL constructs
- New OPA policy rules
- Semantic extractors
- Documentation
