# Improvement Suggestions: Grammar & Extractor

A critique of `tree-sitter-vhdl/grammar.js` (~2770 lines) and `internal/extractor/extractor.go` (~7300 lines), organized by severity and area.

---

## Grammar (`grammar.js`)

### High Priority

#### 1. ~~Regex catch-all in `_simple_expression` masks parse errors~~ DONE

**Resolution**: The `_simple_expression` rule was dead code — defined but never referenced anywhere in the grammar or extractor. Removed entirely. Parser regenerated, binary rebuilt, all 40 policy fixture tests pass.

#### 2. ~~Configuration maps use raw regex instead of grammar rules~~ DONE

**Resolution**: Replaced all 6 instances of inline `/[^)]+/` and `/[^)]*/` regex patterns in `component_configuration` and `configuration_specification` rules with references to the proper `$.generic_map_aspect` and `$.port_map_aspect` rules (which use `$.association_list`). Configuration port/generic maps now produce structured parse trees. Verified against VESTS configuration test files (tc3116, tc856, tc1205, tc409, tc3115) and test.vhdl — zero parse errors. All 40 policy fixture tests pass.

#### 3. ~~60+ conflict declarations signal deep ambiguity~~ PARTIALLY DONE

**Resolution**: Reduced from 64 conflicts to 54 (9 from `_expression_term` removal, 1 from `subprogram_body` removal) (see item #4). Remaining 55 conflicts are categorized as:
- **`_simple_name` ambiguity** (13): genuine VHDL ambiguity between identifiers as names vs. type marks vs. expressions. Requires `_name` unification to reduce further.
- **Aggregate/index** (5): `association_list` vs. `_index_expression` overlap — genuine.
- **PSL** (5): PSL property/sequence parsing conflicts.
- **Generate/block** (3): concurrent vs. declarative item overlap.
- **Configuration** (2): block vs. component configuration.
- **Structural** (27): entity/architecture/type/subprogram boundary ambiguities — mostly genuine.

Also fixed a regression: removing `_expression_term` broke `force`/`release` signal assignment parsing (the flat fallback was masking an `_expression` vs. `when` ambiguity). Fixed by using `$._logical_expression` instead of `$._expression` in `_force_release_assignment` to prevent the expression parser from consuming `when`, and bumped dynamic precedence to 10. This **improved** parsing — previously these assignments parsed as wrong node types; now they parse correctly.

#### 4. ~~`_expression_term` duplicates the expression hierarchy~~ DONE

**Resolution**: Removed `_expression_term` rule and its two fallback references in `_expression` and `_expression_no_conditional`. All expression parsing now routes through the structured hierarchy (`_logical_expression` → `_relational_expression` → `_shift_expression` → ... → `_primary`). Removed 9 associated conflict declarations (64 → 55). Parser regenerated, all tests pass. The `_expression_term` was the root cause of several forced GLR conflicts and unpredictable parse trees.

### Medium Priority

#### 5. Case-insensitive keywords via individual regex patterns — DEFERRED

**Investigation results**: Tree-sitter added native `/library/i` regex flag support in v0.21.0 (Feb 2024), but our grammar uses tree-sitter 0.20.8. Upgrading would allow replacing ~90 regex rules like `/[lL][iI][bB][rR][aA][rR][yY]/` with `/library/i`. A `kw()` helper function (Fortran-style) could provide a smaller improvement without upgrading. The `word` rule is for keyword extraction optimization and doesn't help with case-insensitivity. External scanner is overkill.

**Recommended path**: Upgrade tree-sitter to 0.21+ and use native `/i` flags. This is a parser infrastructure change best done as a standalone effort, not mixed with other grammar changes.

#### 6. Incomplete PSL / VHDL-2019 support

The grammar includes PSL directives and some VHDL-2019 features (generic types, interface packages), but coverage appears partial:
- PSL `assert` / `assume` / `cover` are parsed but property expressions use simplified rules
- VHDL-2019 conditional expressions (`when ... else` in concurrent context) may not cover all forms
- Protected type bodies have basic structure but generics on protected types aren't handled

**Suggestion**: Document which VHDL standard versions are targeted and what's deliberately excluded. Create tracking issues for incomplete features. This helps users understand the tool's scope.

#### 7. ~~`subprogram_body` is an alias, not a distinct construct~~ DONE

**Resolution**: `subprogram_body` was an exact duplicate of `subprogram_declaration` (both were `choice($.function_declaration, $.procedure_declaration)`). The unified `function_declaration`/`procedure_declaration` rules already handle both declaration-only (ending with `;`) and body (with `is ... begin ... end`) via `choice(';', seq($._kw_is, ...))`. Removed the redundant `subprogram_body` rule, replaced all 5 references with `$.subprogram_declaration`, removed duplicate entries, and eliminated 1 associated conflict. Conflict count: 55 → 54. All tests pass.

### Low Priority

#### 8–10. Low priority grammar cleanup — ASSESSED

**Assessment**: Reviewed all three items:
- **`optional()` consistency**: No functional impact; the grammar consistently uses `optional()` throughout. Not worth changing.
- **`field()` annotations**: The grammar already has 100+ `field()` annotations covering all major constructs used by the extractor (`name`, `label`, `target`, `entity`, `sensitivity`, `ports`, `type`, etc.). The remaining unannotated children are in lesser-used rules. Adding more would provide diminishing returns.
- **External scanner**: Only handles bit string literals. Extended identifiers and tool directives are rare enough that inline handling is fine. Not worth the C maintenance burden.

**Conclusion**: No changes needed. The grammar's annotation coverage already matches the extractor's access patterns.

---

## Extractor (`extractor.go`)

### High Priority

#### 1. ~~Fragile string-based concurrent assignment classification~~ DONE

**Resolution**: Replaced `strings.Contains`/`strings.HasPrefix` classification with parse tree structure analysis. New approach:
- **Selected**: Target field byte offset > node start + 2 (the `with expr select` prefix pushes target right)
- **Conditional**: Named child count > 2 (has condition/else branches beyond target + value)
- **Simple**: Default (target at start, 1-2 named children)

This eliminates false positives from signal names like `data_with_parity` or `sel_when_ready`. All policy fixture and extractor e2e tests pass.

#### 2. ~~Multiple fallback paths for signal declaration parsing~~ DONE

**Resolution**: The grammar's `signal_declaration` rule already consistently provides `field('names', ...)` and `field('type', ...)`. Replaced the 4-level fallback cascade (field lookup → colon-based iteration → byte range extraction → bare identifier) with a clean primary path using grammar fields plus a single minimal fallback for partial-parse edge cases. Reduced from ~50 lines to ~25 lines. All tests pass.

#### 3. ~~No ERROR node detection or metrics~~ DONE

**Resolution**: Added `ErrorNodeCount int` field to `FileFacts`. During tree walk, ERROR nodes are now detected, counted, and the walker skips into ERROR subtrees (children are unreliable). The count is exposed in two places:
- **Progress output** (`-p`/`-t`): `(extracted, 98ms, 1 parse errors)`
- **Trace summary** (`-t`): `facts: ... ERRORS=1`

Verified against external test projects — files like `grlib/lib/tech/atc18/components/atmel_simprims.vhd` (17 errors), `PoC/src/common/physical.vhdl` (11 errors) now clearly show their parse quality. All tests pass.

#### 4. ~~Reset polarity is hardcoded~~ DONE

**Resolution**: Added `ResetPolarity` field to `Process` struct and polarity detection in the reset condition analyzer. When a reset comparison has a character literal RHS:
- `rst = '0'` or `rst_n = '0'` → `active_low`
- `rst = '1'` → `active_high` (default)

The polarity is now propagated to `ResetInfo` instead of the hardcoded `"active_high"`. Falls back to `"active_high"` only if polarity detection fails (e.g., non-comparison reset patterns). All tests pass.

### Medium Priority

#### 5. ~~Duplicate port extraction logic~~ DONE

**Resolution**: Factored out shared `extractPortsFromNode(node, source, ownerName)` function. Both `extractPortsFromEntity` and `extractPortsFromComponent` now delegate to it. The entity version adds `declaredSignals` registration and appends to `facts.Ports`; the component version just returns the slice. Eliminated ~40 lines of duplicate code. All tests pass.

#### 6. Signal read extraction — ASSESSED, NOT DUPLICATE

**Assessment**: Reviewed all signal read functions:
- `extractReadsFromNode` / `extractReadsFromNodeSkipping`: Main function, extracts base signal names with `declaredSignals`/`variableSet` filtering, attribute handling
- `extractReadsWithFullPaths` / `extractReadsWithFullPathsSkipping`: Extracts full `a.b.c` paths for signal dependency analysis
- `extractCaseExpressionReads` / `extractIfConditionReads`: Specialized wrappers for case/if contexts

These serve genuinely different purposes — base names vs. full paths, filtered vs. unfiltered. Merging into one parameterized function would add complexity (options struct, conditional logic) without clear benefit. The code is not as duplicated as initially assessed.

#### 7. Multiple process analysis passes — DEFERRED

**Assessment**: The 5 passes (semantics, case statements, comparisons, arithmetic ops, signal deps) each have distinct recursive walkers with different node-type handling and return patterns. Merging into a single-pass visitor would require a large "god function" with 5 sets of accumulators and complex dispatch logic. The performance cost of multiple walks over typically-small process trees is modest — tree-sitter node access is cheap. This optimization should only be done if profiling shows process analysis is a bottleneck. Current priority: correctness over premature optimization.

#### 8. ~~CDC synchronizer detection only handles simple chains~~ DONE (documented)

**Resolution**: Added comprehensive doc comment to `DetectCDCCrossings()` documenting the detection scope (simple 2-stage FF chains, multi-bit flagging, single-file analysis) and explicit list of what's NOT detected (gray code, handshake, FIFO, vendor primitives, MCP signals). This helps users understand the tool's CDC analysis limitations without surprises.

#### 9. ~~Type width calculation uses regex~~ DONE

**Resolution**: `CalculateWidth()` now returns -1 for "unknown but likely multi-bit" types instead of 0. This covers:
- Parameterized ranges: `std_logic_vector(WIDTH-1 downto 0)` → -1
- Attribute-based ranges: `unsigned(data'length-1 downto 0)` → -1
- Unconstrained vectors: bare `signed`, `unsigned` → -1

The CDC crossing detector now treats -1 as multi-bit (`IsMultiBit = width > 1 || width == -1`), ensuring parameterized vector crossings are correctly flagged as multi-bit CDC violations. The policy input builder clamps -1 to 0 at the Go→Rust boundary via `policyWidth()` since the Rust engine already treats 0 as "unknown". Also hoisted two `regexp.MustCompile` calls to package-level variables (`reVectorRange`, `reIntegerRange`) to avoid per-call compilation. All tests pass.

### Low Priority

#### 10–12. Low priority extractor cleanup — ASSESSED

**Assessment**: Reviewed all three items:
- **Tree query caching (#10)**: There are ~244 `ChildByFieldName`/`NamedChild`/`ChildCount` calls across the extractor. Tree-sitter node access is O(n) in child count with n typically <10 — adding a cache layer would introduce allocation overhead that likely exceeds traversal savings. Should only be done if profiling shows this is a bottleneck.
- **Magic numbers (#11)**: The extractor does NOT apply threshold-based classification — it extracts raw facts only. All threshold-based rules (complex process >20 assignments, wide signal >128 bits, long sensitivity >8 signals) live in the Rust policy engine where they belong. No changes needed.
- **Mixed concerns in walkTree (#12)**: `walkTree` is ~500 lines of structural extraction. Semantic analysis (CDC detection, FSM identification, process classification) already runs as separate post-extraction passes in `ExtractFacts`. The walkTree function IS structurally focused. Splitting it further would fragment the extraction logic without clear benefit.

**Conclusion**: No changes needed. The extractor's architecture already matches the suggested improvements.

---

## Cross-Cutting Concerns

### ~~Grammar ↔ Extractor Contract~~ DONE

**Resolution**: Created `internal/extractor/contract_test.go` with `TestGrammarExtractorContract` that validates:
1. **Field name contract**: Every `ChildByFieldName()` string used by the extractor exists as a `field('name', ...)` in `grammar.js`
2. **Grammar sync**: The static `grammarFieldNames` map matches the live `grammar.js` field definitions (detects additions and removals)
3. **Extractor sync**: The static `extractorFieldNames` list matches live `ChildByFieldName()` calls in `extractor.go`
4. **Hidden node warnings**: Flags any `.Type() == "_hidden_rule"` comparisons (hidden tree-sitter rules don't produce named nodes)

Current contract: 39 field names used by extractor, all 39 exist in grammar's 46 field definitions. The 7 unused grammar fields (`aliased_name`, `alternative`, `consequence`, `end_label`, `event`, `generic_map`, `value`) are available for future use. Test includes instructions for updating both lists when grammar or extractor changes.

### Testing Strategy — ASSESSED

**Assessment**: The test infrastructure is stronger than initially assessed:
- **37 test files** across all packages with good coverage
- **Extractor E2E tests**: 1564 lines covering signals, processes, instances, assignments
- **Unit tests**: `width_test.go`, `verification_constructs_test.go`, `verification_tags_test.go`
- **Policy fixtures**: Comprehensive positive/negative test coverage for ~170 rules
- **New**: Grammar-extractor contract test (see above)

**Remaining gaps** (recommended for future work):
- Tree-sitter corpus tests (`tree-sitter-vhdl/test/corpus/`) for declarative grammar shape testing
- Golden file / snapshot tests for extractor `FileFacts` output
- Performance benchmarks (`Benchmark*` functions)

### Performance — PARTIALLY ADDRESSED

**What was done**:
- Hoisted 2 per-call `regexp.MustCompile` invocations to package-level variables (`reVectorRange`, `reIntegerRange`) in `CalculateWidth` (item #9 above)
- 3 other inline `regexp.MustCompile` calls remain in rarely-called functions (loop variable extraction, type text parsing, alias detection) — not hot paths

**Remaining** (recommended for future work):
- Add `--cpu-profile` / `--mem-profile` flags using `runtime/pprof`
- Profile with `go tool pprof` on ghdl (9613 files) to find actual hotspots
- Grammar conflict count reduced from 64 to 54 (items #3, #4, #7 above), reducing GLR overhead
