# VHDL Compiler Project — Working Guide

## Purpose & Philosophy
- **Learning project**: learn VHDL and compiler construction by building a real linter.
- **Panic is failure**: never crash on user input; always return explicit diagnostics.
- **No silent failures**: contract violations crash immediately (CUE validation).
- **Grammar is source of truth**: fix parse errors at the grammar before extractor/rules.

## Architecture (Pipeline)
1. **Tree‑sitter grammar** (`tree-sitter-vhdl/grammar.js`) parses VHDL (error‑tolerant).
2. **Extractor (Go)** walks tree → `FileFacts` (entities, signals, processes, instances, etc.).
3. **Indexer (Go)** builds cross‑file symbol table + normalized policy input.
4. **Contract guard (CUE)** validates `schema/ir.cue` (crash on mismatch).
5. **Policy engine (Rust)** evaluates rules → violations with file/line.

## Tight Iteration Loops

### 1) Grammar loop (fix ERROR nodes first)
```bash
# Find ERROR nodes quickly
cd tree-sitter-vhdl
npx tree-sitter parse ../path/to/file.vhd 2>&1 | grep ERROR

# Full suite baseline + analysis
ANALYZE=1 ./test_grammar.sh external_tests

# Focused retest on previous failures
FOCUS_FAILS=1 ./test_grammar.sh

# Or top‑N worst offenders
FOCUS_TOP=50 ./test_grammar.sh
```
**After grammar edits**:
```bash
cd tree-sitter-vhdl && npx tree-sitter generate
cd ..
go clean -cache && go build ./cmd/vhdl-lint
```

### 2) Extractor / Rules loop
```bash
# Update extractor → update schema → update rules

# Tests for rules
 go test ./internal/policy -run TestPolicyRuleFixtures

# Rust policy unit tests
 cargo test -q

# Rebuild binary when needed
 go build -o vhdl-lint ./cmd/vhdl-lint
```

### 3) False‑positive hunt (fast)
```bash
./vhdl-lint external_tests/Compliance-Tests/vhdl_2008/ 2>&1 | \
  grep -oP '\\[[a-z_]+\\]' | sort | uniq -c | sort -rn

# Inspect one rule hit
./vhdl-lint external_tests/Compliance-Tests/vhdl_2008/ 2>&1 | grep '\\[some_rule\\]'
```

## Gotchas (Do Not Skip)
- **Go CGO cache**: after `npx tree-sitter generate`, always run:
  ```bash
  go clean -cache && go build ./cmd/vhdl-lint
  ```
- **ERROR nodes poison downstream**: fix grammar before extractor/rules.
- **Policy cache**: cached results live under `<root>/.vhdl_lint_cache/policy_cache.json`.
  Use `--clear-policy-cache` after rule changes or rule‑hash issues.
- **Contract strictness**: if CUE validation fails, fix the source (grammar/extractor/indexer), never suppress.

## CLI Flags (vhdl-lint)
```bash
./vhdl-lint init                     # create config
./vhdl-lint <path>                   # lint path
./vhdl-lint -v <path>                # verbose
./vhdl-lint -p <path>                # progress
./vhdl-lint -t <path>                # trace (progress + per‑file summaries)
./vhdl-lint -j <path>                # JSON output
./vhdl-lint --timing <path>          # timing.jsonl
./vhdl-lint --policy-trace <path>    # Rust per‑rule timing
./vhdl-lint --policy-stream <path>   # stream Rust stderr
./vhdl-lint --clear-policy-cache <path>
./vhdl-lint -c config.json <path>    # explicit config
```

## Environment Variables
- `VHDL_POLICY_DAEMON=1` — use incremental Rust policy daemon (delta eval).
- `VHDL_POLICY_BIN=/path/to/vhdl_policy` — override policy binary.
- `VHDL_POLICYD_BIN=/path/to/vhdl_policyd` — override daemon binary.
- `VHDL_POLICY_PROFILE=debug|release` — build profile for policy binaries.
- `VHDL_POLICY_TRACE_TIMING=1` — enable Rust per‑rule timing.
- `VHDL_POLICY_STREAM=1` — stream Rust stderr without timing.

## Scripts & Tools
- `./test_grammar.sh` — grammar health + XPASS workflows.
- `./dev.sh` — watch mode for grammar edits (auto‑rebuilds parser).
- `tools/timing_report.py timing.jsonl` — human‑readable timing report.
- `tools/timing_trace.py timing.jsonl --out timing_trace.json` — Chrome trace.

## Caching & Incremental Behavior
- Cache root: `<root>/.vhdl_lint_cache/`.
- `facts/`, `index.json`, `fact_tables.json`, `policy_cache.json`.
- Facts cache keys on **file content + parser/extractor versions**.
- Policy cache keys on **config + third‑party list + Rust rule hash**.
- If cache validation fails, fall back to full evaluation (never silent).

## Rule/Fixture Discipline
- For every new rule: add one **positive** and one **negative** fixture.
- Update `testdata/policy_rules/manifest.json` and `manifest_negative.json`.
- Run `go test ./internal/policy -run TestPolicyRuleFixtures`.

## Debugging Checklist
1. Check for parse `ERROR` nodes (Tree‑sitter).
2. Validate extractor facts (Go tests / trace output).
3. Validate policy input with CUE (errors should crash loudly).
4. Confirm Rust policy behavior (unit tests + fixtures).
