# VHDL Compliance Compiler

A production-ready VHDL linting and compliance tool built with Go, Tree-sitter, and OPA.

## What Is This?

This stack transforms VHDL from "text files" into a "queryable database," enabling safety checks that were previously impossible without expensive proprietary tools.

---

## The Compliance Compiler Stack

### 1. The Parser: Tree-sitter (VHDL)

**Role:** The "Crash-Proof" Translator.

**Why Tree-sitter:**
- **Error Tolerance:** Unlike GHDL (which stops at the first syntax error), Tree-sitter produces a valid tree even for broken code. This allows you to lint a file while the user is still typing.
- **Speed:** It is incremental. Re-parsing a 5,000-line file after a 1-line change takes milliseconds, enabling real-time IDE feedback.
- **Standard:** It is the industry standard for modern editors (Neovim, Zed, Helix), meaning your grammar maintenance effort benefits the entire ecosystem.

### 2. The Extractor: Scheme Queries (`.scm`)

**Role:** The Data Compressor.

**Why Scheme Queries:**
- **Decoupling:** Separates "Syntax" from "Logic." If the VHDL grammar changes (e.g., how `port_list` is nested), you update the `.scm` file, not your Go code.
- **Performance:** A raw VHDL syntax tree is massive (~10MB JSON for a large file). Scheme queries filter this down to just the 1% of data you care about (Entity names, Port types), preventing your policy engine from choking on noise.

### 3. The Orchestrator: Go (The Indexer)

**Role:** The "Stateful Glue."

**Why Go:**
- **Cross-File State:** Tree-sitter processes one file at a time. Go holds the "World View"—mapping `component` declarations in File A to `entity` definitions in File B.
- **Parallelism:** Go's goroutines allow you to parse, extract, and index thousands of VHDL files concurrently, making the tool feel instant.
- **Single Binary:** Compile the OPA engine, Tree-sitter runtime, and your logic into a single `./vhdl-lint` binary that is easy to distribute in CI pipelines.

### 4. The Policy Engine: OPA (Rego)

**Role:** The "Semantic Validator."

**Why OPA:**
- **Declarative Power:** Writing "Find all signals that cross clock domains without synchronization" is a 5-line query in Rego. In Go, it would be 200 lines of fragile loop logic.
- **Standardization:** Uses the same policy language as Kubernetes and Terraform tools. Integrate your VHDL checks into standard enterprise "Policy as Code" workflows.
- **Maintainability:** Publish "Rule Packs" (e.g., *DO-254 Safety Pack*) as separate `.rego` files without recompiling the main tool.

### 5. The Configurator: CUE

**Role:** The "Type-Safe Contract."

**Why CUE:**
- **Schema Validation:** CUE acts as the contract between your Go extractor and OPA. It ensures the data sent to the policy engine is valid, preventing "silent failures" where a rule passes simply because a field name changed.
- **Artifact Generation:** Generates complex output files (Testbenches, Tcl scripts) with mathematical certainty, eliminating syntax errors in your build scripts.

---

## The Go Indexer: Deep Dive

The Go Indexer is the most complex component. It acts as a **Linker** for your VHDL project, turning isolated **Facts** (from Tree-sitter) into **Relationships** (for OPA).

### Task 1: VHDL Path Resolution (The Librarian)

VHDL has a complex library system.

**The Problem:** When a file says `library ieee; use ieee.std_logic_1164.all;`, it refers to a vendor installation path (e.g., `/opt/xilinx/...`), not a file in your project.

**The Solution:**
1. **Scan:** Walk the user's project directory recursively.
2. **Map:** Build a `map[string]string` registry of **Logical Library** → **Physical Path**:
   ```
   work   → ./src/
   ip_lib → ./vendor/ip/
   ```
3. **Resolve:** When parsing `use work.my_pkg.all`, look up `work` in the map, find the files, and link them.

### Task 2: Fact Normalization (The Flattener)

OPA is a "Relational" engine—it wants data that looks like SQL tables.

**The Problem:** Tree-sitter outputs a tree: `Entity → PortList → Port → Type`.

**The Solution:** Flatten into lists of objects:

```json
{
  "entities": [
    { "id": "work.cpu", "file": "src/cpu.vhd", "ports": ["clk", "rst"] }
  ],
  "dependencies": [
    { "source": "work.cpu", "target": "work.alu", "type": "instantiation" }
  ]
}
```

This allows OPA to run fast joins: *"Find all entities that depend on `alu`"* becomes an O(1) lookup.

### Task 3: Schema Validation (The Guard Rail)

**The Problem:** If you rename `port_name` to `name` in your `.scm` query, your OPA rules silently stop working.

**The Solution:**
1. Marshal extracted data to JSON.
2. Validate against `schema/ir.cue` using the CUE runtime.
3. If validation fails, **panic immediately**: *"Extractor produced invalid data: field 'port_name' missing."*
4. Guarantees no "Silent False Positives."

### Task 4: The Symbol Table (The Cross-File Brain)

**The Solution:**
1. **Pass 1 (Discovery):** Parse every file. Extract **Exports** (Entity names, Package names). Store in Global Symbol Map.
2. **Pass 2 (Resolution):** Parse every file again. Check if **Imports** (`component my_comp`) exist in the Global Symbol Map.
3. **Report:** Inject `found: true/false` flags into the JSON sent to OPA.

### Indexer Pseudocode

```go
func RunIndexer(rootPath string) {
    // 1. Scan Files
    files := FindVHDLFiles(rootPath)

    // 2. Pass 1: Parallel Extraction (Goroutines)
    var symbols SyncMap
    for _, file := range files {
        go func(f) {
            ast := TreeSitterParse(f)
            facts := SchemeExtract(ast)
            symbols.Add(facts.Exports) // e.g. "work.my_cpu"
        }(file)
    }

    // 3. Pass 2: Linker
    var normalizedData OPAInput
    for _, file := range files {
        extractedImports := GetImports(file)
        for _, imp := range extractedImports {
             if !symbols.Has(imp) {
                 normalizedData.Errors.Add(file, "Missing Import: " + imp)
             }
        }
    }

    // 4. Validate against CUE Schema
    if err := CueValidate(normalizedData); err != nil {
        panic("CRITICAL: Indexer generated bad data!")
    }

    // 5. Hand off to OPA
    OPA.Eval(normalizedData)
}
```

---

## Project Structure

```
.
├── tree-sitter-vhdl/
│   ├── grammar.js          # VHDL grammar definition
│   ├── queries/
│   │   └── extract.scm     # Scheme queries for fact extraction
│   └── bindings/           # Language bindings
├── cmd/
│   └── vhdl-lint/          # Main CLI entry point
├── internal/
│   ├── indexer/            # Go indexer (symbol table, linker)
│   ├── extractor/          # Tree-sitter + Scheme query runner
│   └── validator/          # CUE schema validation
├── policies/
│   └── *.rego              # OPA policy rules
├── schema/
│   └── ir.cue              # Intermediate representation schema
└── test.vhdl               # Test file
```

---

## Quick Start

```bash
# Build the tool
go build -o vhdl-lint ./cmd/vhdl-lint

# Lint a VHDL project
./vhdl-lint ./src/

# Run with specific policy pack
./vhdl-lint --policies=policies/do254.rego ./src/
```

---

## Grammar Development

The Tree-sitter grammar is in `tree-sitter-vhdl/grammar.js`.

```bash
# Regenerate parser after grammar changes
cd tree-sitter-vhdl && npx tree-sitter generate

# Test parsing
./vhdl-lint --parse-only test.vhdl
```

### Current Grammar Status

| Test Suite | Pass Rate |
|------------|-----------|
| VHDL 2008 Compliance-Tests | 100% |
| IEEE 1076-2008 | 65% |

---

## License

MIT

## Contributing

Contributions welcome:
- Grammar improvements for missing VHDL constructs
- New OPA policy rules
- Better error messages
- Documentation
