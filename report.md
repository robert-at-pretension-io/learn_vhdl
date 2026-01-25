# VHDL Grammar Test Report

This report documents the grammar test results, excluded files, and analysis of parsing failures.

## Current Test Results

| Metric | Value |
|--------|-------|
| **Total Files** | **12,762** |
| **Valid Acceptance** | **98.75%** |
| **Grammar Quality** | **85.40%** |

### Detailed Breakdown

| Category | Count | Description |
|----------|-------|-------------|
| **Pass** | 10,074 | Valid VHDL accepted (GOOD) |
| **XFail** | 825 | Invalid VHDL rejected (GOOD) |
| **Fail** | 127 | Valid VHDL rejected (BAD - grammar too strict) |
| **XPass** | 1,736 | Invalid VHDL accepted (BAD - grammar too loose) |

---

## External Test Repositories

| Repository | Files | Pass | Fail | Pass Rate | Description |
|------------|-------|------|------|-----------|-------------|
| ghdl | 9,613 | ~9,500 | ~100 | 99%+ | GHDL test suite |
| grlib | 806 | ~800 | ~6 | 99%+ | LEON3/5 SPARC processors |
| osvvm | 639 | ~630 | ~9 | 98%+ | Verification methodology |
| vunit | 332 | ~325 | ~7 | 98%+ | Unit testing framework |
| **uvvm** | 291 | 289 | 2 | **99.3%** | Universal VHDL Verification (NEW) |
| PoC | 291 | ~285 | ~6 | 98%+ | IP cores library |
| vhdl-tests | 282 | ~275 | ~7 | 97%+ | IEEE compliance tests |
| hdl4fpga | 274 | ~270 | ~4 | 98%+ | FPGA IP library |
| **open-logic** | 183 | 182 | 1 | **99.5%** | FPGA Standard Library (NEW) |
| **jcore** | 178 | 163 | 15 | **91.6%** | SuperH-compatible processor (NEW) |
| Compliance-Tests | 78 | ~70 | ~8 | 90%+ | IEEE 1076-2008 tests |
| neorv32 | 69 | ~68 | ~1 | 98%+ | RISC-V processor |

### New Repositories Analysis

#### UVVM (289/291 pass - 99.3%)

[UVVM](https://github.com/UVVM/UVVM) is the Universal VHDL Verification Methodology, backed by the European Space Agency.

**Failures (2 files):**
- `uvvm_util/src/license_pkg.vhd` - Uses `report (expression);` with parentheses
- `bitvis_vip_sbi/tb/maintenance_tb/sbi_vvc_multi_cycle_read_tb.vhd` - Same issue

**Issue:** The `report` statement with parenthesized argument: `report (to_string(...));`

**Fix needed:** Grammar should accept `report (expression)` in addition to `report expression`.

#### Open Logic (182/183 pass - 99.5%)

[Open Logic](https://github.com/open-logic/open-logic) is a pure VHDL-2008 FPGA standard library.

**Failures (1 file):**
- `doc/axi/slave/RbExample.vhd` - Documentation code snippet (not standalone VHDL)

**Issue:** File is a process snippet without entity/architecture wrapper.

**Fix needed:** None - this is a documentation fragment, should be excluded.

#### J-Core (163/178 pass - 91.6%)

[J-Core](https://github.com/j-core/jcore-soc) is a BSD-licensed SuperH-compatible processor.

**Failures (15 files):**
- Various files in `components/` and `targets/`

**Issue:** Uses VHDL `group` declarations:
```vhdl
-- synopsys translate_off
group bus_sigs : bus_ports(db_i, db_o);
-- synopsys translate_on
```

**Fix needed:** Add `group` declaration support to grammar. Groups are rarely used but valid VHDL for applying attributes to signal collections.

---

## Excluded Files Summary

### TRUE_NEGATIVE_TESTS (~830 files)

Intentionally invalid files that SHOULD fail to parse:

| Pattern | Files | Reason |
|---------|-------|--------|
| `*/non_compliant/*` | ~600 | Standard negative tests |
| `*/negative/*` | ~100 | Standard negative tests |
| `*/analyzer_failure/*` | ~50 | Semantic errors |
| `*/ghdl/testsuite/gna/bug0100/*` | 45 | Intentional syntax errors |
| `*/ghdl/testsuite/gna/issue2070/*` | 62 | Fuzz/garbage |
| `*/ghdl/testsuite/gna/issue2116/*` | 71 | Fuzz/garbage |
| `*/grlib/*/*.in.vhd` | 67 | Config fragments |
| Others | ~35 | Various corruption tests |

### TEMP_IGNORED_TESTS (~630 files)

Valid VHDL with unsupported constructs:

| Category | Files | Priority | Constructs |
|----------|-------|----------|------------|
| VHDL-AMS | 534 | None | `nature`, `terminal`, `quantity` |
| Generic packages | ~15 | High | `generic (type T)`, package instantiation |
| PSL | ~12 | None | `vunit`, `assert always` |
| Edge cases | ~65 | Medium | Various VHDL-2008 features |
| UTF-16 | 2 | None | Encoding issue |
| VHDL-2019 | 1 | Low | Conditional compilation |

---

## Grammar Improvement Targets

### High Priority

1. **Group declarations** (J-Core) - 15 files
   ```vhdl
   group signal_group : group_template(sig1, sig2);
   ```

2. **Report with parentheses** (UVVM) - 2 files
   ```vhdl
   report (to_string(value));  -- Currently fails
   report to_string(value);    -- Currently works
   ```

3. **Generic packages/types** - ~15 files
   ```vhdl
   generic (type T);
   package p is new pkg generic map (T => integer);
   ```

### Medium Priority

4. **Case generate** - `case expr generate`
5. **Matching case** - `case? expr is`
6. **If generate else** - `if cond generate ... else generate`
7. **Context clauses** - `context lib.ctx`

### Low Priority / Keep Excluded

8. **VHDL-AMS** - Different language (IEEE 1076.1)
9. **PSL** - Separate standard (IEEE 1850)
10. **VHDL-2019** - Limited adoption

---

## XPASS Analysis (Grammar Too Permissive)

The grammar accepts 1,736 invalid files it should reject:

| Source | Count | Severity |
|--------|-------|----------|
| `non_compliant/*` | ~1,167 | High - syntax errors accepted |
| `synth/err*` | ~55 | High - synthesis errors |
| Various GHDL issues | ~200 | Medium - mixed |
| Other | ~300 | Low - mostly semantic |

**Note:** High XPASS count in `non_compliant` indicates the grammar is too permissive and accepts invalid syntax. This is a lower priority than fixing FAIL cases but should be addressed for grammar correctness.

---

## File Count Summary

| Category | Files | % |
|----------|-------|---|
| Pass (valid accepted) | 10,074 | 78.9% |
| XFail (invalid rejected) | 825 | 6.5% |
| Fail (valid rejected) | 127 | 1.0% |
| XPass (invalid accepted) | 1,736 | 13.6% |
| **Total** | **12,762** | 100% |

**Grammar Quality Score:** 85.40% ((Pass + XFail) / Total)

The grammar correctly handles 85.4% of all test cases. The main issues are:
- 127 valid VHDL files rejected (grammar too strict)
- 1,736 invalid VHDL files accepted (grammar too permissive)
