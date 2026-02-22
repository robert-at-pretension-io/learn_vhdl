#!/bin/bash
set -e
if [ "$#" -lt 1 ]; then
  echo "Usage: ./compile.sh <top.vhd> [sub1.vhd sub2.vhd ...]"
  echo "Example: ./compile.sh cpu_top.vhd alu.vhd decoder.vhd"
  exit 1
fi

CIRCT_BIN=../cirt/build/bin
ABC=../yosys/yosys-abc
BOUND=${BOUND:-15}

# Use the first file passed as the base for output naming
INPUT="$1"
DIR="$(dirname "$INPUT")"
BASE="$(basename "${INPUT%.*}")"
MLIR="${DIR}/${BASE}.mlir"
IC3_MLIR="${DIR}/${BASE}_ic3.mlir"
IC3_AIG="${DIR}/${BASE}_ic3.aig"
SV="${DIR}/${BASE}.sv"

echo "[1/3] Parsing and Linking Micro-VHDL Project..."
# Notice we are now using the flags we added to main.go, and passing all files
GO_OUT=$(go run . -o "$MLIR" -ic3 "$IC3_MLIR" "$@")
echo "$GO_OUT"

# Extract the automatically deduced Top Module from the Go output
MODULE=$(echo "$GO_OUT" | grep -oP '(?<=TOP_MODULE=)\w+' | head -1)

if [ -z "$MODULE" ]; then
    echo "Error: Could not determine top module. Check semantic output."
    exit 1
fi

echo "--> Detected Top-Level Entity: ${MODULE}"

# Feature flags — computed once, used in multiple steps below.
HAS_BAD=$(grep -c '__verif_bad' "$IC3_MLIR" 2>/dev/null || true)
HAS_LIVENESS=$(grep -c 'assert_fair' "$IC3_MLIR" 2>/dev/null || true)
HAS_ASSUME=$(grep -c 'verif.assume' "$MLIR" 2>/dev/null || true)
HAS_LTL=$(grep -c 'ltl' "$MLIR" 2>/dev/null || true)
HAS_CONTRACT=$(grep -c 'verif.contract' "$MLIR" 2>/dev/null || true)
HAS_ASSERT=$(grep -c 'verif.assert' "$MLIR" 2>/dev/null || true)
HAS_COVER=$(grep -c 'verif.cover' "$MLIR" 2>/dev/null || true)

# Since there are multiple files, we cat them all to check for 'after_reset'
HAS_AFTER_RESET=$(cat "$@" | grep -c 'after_reset' 2>/dev/null || true)

echo "[2a/3] Bounded Model Checking via Z3 SMT Solver (module=${MODULE}, bound=${BOUND} cycles)..."
# circt-bmc unrolls the circuit for BOUND clock cycles and hands the resulting
# SMT formula to Z3.  Any reachable state that violates a verif.assert will be
# reported as "Assertion can be violated!" with a counterexample trace.
# Override the bound: BOUND=30 ./compile.sh file.vhd
#
# circt-bmc cannot process verif.contract directly (ConvertHWToSMT fails on
# verif.symbolic_value emitted by the apply-mode lowering).  Strip contracts
# before BMC; they are verified separately in step [2c/3] below.
if [ "$HAS_CONTRACT" -gt 0 ] && [ "$HAS_ASSERT" -eq 0 ]; then
  echo "  (no direct PSL assertions — contract-only design, skipping BMC; contracts checked in step [2c/3])"
else
  BMC_MLIR="$MLIR"
  if [ "$HAS_CONTRACT" -gt 0 ]; then
    BMC_MLIR="${DIR}/${BASE}_bmc.mlir"
    "${CIRCT_BIN}/circt-opt" --strip-contracts "$MLIR" -o "$BMC_MLIR"
  fi
  "${CIRCT_BIN}/circt-bmc" \
    "$BMC_MLIR" \
    --module="${MODULE}" \
    -b "${BOUND}" \
    --run \
    --shared-libs=/usr/lib/x86_64-linux-gnu/libz3.so
fi

# Only run IC3 when the IC3 MLIR contains a __verif_bad or assert_fair output
# (i.e. the design has PSL assertions worth proving unboundedly).
# Skip IC3 when psl assume is present: circt-translate --export-aiger does not
# support the verif dialect, so assumptions cannot be encoded as AIGER constraints.
# The BMC step (Z3) handles constrained verification correctly via verif.assume.
if [ "$HAS_LIVENESS" -gt 0 ]; then
  echo "[2b/3] Unbounded IC3/PDR Liveness proof via ABC (module=${MODULE})..."
  "${CIRCT_BIN}/circt-opt" --hw-flatten-modules --symbol-dce "$IC3_MLIR" | "${CIRCT_BIN}/circt-synth" --format=mlir --until-before=all | "${CIRCT_BIN}/circt-translate" --export-aiger -o "$IC3_AIG"
  "$ABC" -c "read_aiger ${IC3_AIG}; l2s; pdr; quit" 2>&1
elif [ "$HAS_BAD" -gt 0 ] && [ "$HAS_LTL" -gt 0 ]; then
  echo "[2b/3] LTL temporal safety properties present — IC3/PDR skipped (BMC covers this)."
elif [ "$HAS_BAD" -gt 0 ] && [ "$HAS_ASSUME" -gt 0 ]; then
  echo "[2b/3] PSL assumes present — IC3/PDR skipped (assumptions cannot be encoded as AIGER constraints; BMC covers this)."
elif [ "$HAS_BAD" -gt 0 ] && [ "$HAS_AFTER_RESET" -gt 0 ]; then
  echo "[2b/3] after_reset assertions present — IC3/PDR skipped (unconstrained initial states cause spurious CEX; BMC covers this)."
elif [ "$HAS_BAD" -gt 0 ] && [ "$HAS_CONTRACT" -gt 0 ]; then
  echo "[2b/3] Contracts present — IC3/PDR skipped (assumptions cannot be encoded as AIGER constraints; BMC covers this)."
elif [ "$HAS_BAD" -gt 0 ]; then
  echo "[2b/3] Unbounded IC3/PDR Safety proof via ABC (module=${MODULE})..."
  "${CIRCT_BIN}/circt-opt" --hw-flatten-modules --symbol-dce "$IC3_MLIR" | "${CIRCT_BIN}/circt-synth" --format=mlir --until-before=all | "${CIRCT_BIN}/circt-translate" --export-aiger -o "$IC3_AIG"
  "$ABC" -c "read_aiger ${IC3_AIG}; pdr; quit" 2>&1
else
  echo "[2b/3] No PSL assertions found — skipping IC3/PDR."
fi

# Contract verification: lower verif.contract → verif.formal → circt-bmc.
#
# Why not pass the MLIR directly to circt-bmc?
#   --lower-contracts emits an apply-mode hw.module that contains verif.symbolic_value.
#   circt-bmc's ConvertHWToSMT pass converts all hw.modules to func.funcs, after which
#   verif.symbolic_value is no longer inside hw.module and the dialect verifier rejects it.
#   The fix: extract only the verif.formal blocks (the actual check modules) and pass
#   those to circt-bmc, which handles them via its own internal LowerTestsPass.
if [ "$HAS_CONTRACT" -gt 0 ]; then
  echo "[2c/3] Contract verification (module=${MODULE})..."
  CONTRACT_LOWERED="${DIR}/${BASE}_contract.mlir"
  FORMAL_ONLY="${DIR}/${BASE}_formal.mlir"

  # Step 1: lower verif.contract → verif.formal check blocks + apply-mode hw.module
  "${CIRCT_BIN}/circt-opt" --lower-contracts "$MLIR" -o "$CONTRACT_LOWERED"

  # Step 2: extract only verif.formal blocks — drop the apply-mode hw.module
  python3 - "$CONTRACT_LOWERED" "$FORMAL_ONLY" << 'PYEOF'
import sys
in_f, depth, out = False, 0, ["module {"]
for line in open(sys.argv[1]):
    s = line.rstrip()
    if not in_f and "verif.formal " in s:
        in_f = True
        depth = 0
    if in_f:
        out.append(s)
        depth += s.count("{") - s.count("}")
        if depth <= 0:
            in_f = False
out.append("}")
open(sys.argv[2], "w").write("\n".join(out) + "\n")
PYEOF

  # Step 3: verify each contract check module via circt-bmc's LowerTestsPass
  idx=0
  while true; do
    cmod="${MODULE}_CheckContract_${idx}"
    grep -q "@${cmod}" "$FORMAL_ONLY" 2>/dev/null || break
    echo "  Checking ${cmod}..."
    "${CIRCT_BIN}/circt-bmc" \
      "$FORMAL_ONLY" \
      --module="${cmod}" \
      -b "${BOUND}" \
      --run \
      --shared-libs=/usr/lib/x86_64-linux-gnu/libz3.so
    idx=$((idx + 1))
  done
  [ "$idx" -eq 0 ] && echo "  (no contract check modules generated — check contract syntax)"
fi

if [ "$HAS_COVER" -gt 0 ]; then
  echo "[2d/3] Cover verification via Z3 SMT Solver (module=${MODULE})..."
  COVER_PY="${DIR}/check_cover.py"
  cat << 'EOF' > "$COVER_PY"
import sys, re, subprocess
mlir_file = sys.argv[1]
module = sys.argv[2]
bound = sys.argv[3]
circt_bmc = sys.argv[4]

with open(mlir_file) as f:
    content = f.read()

covers = list(re.finditer(r'(\s*)verif\.cover\s+(%[a-zA-Z0-9_]+)\s*:\s*i1', content))
if not covers:
    sys.exit(0)

print(f"  Found {len(covers)} cover properties to check.")

for i, match in enumerate(covers):
    indent = match.group(1)
    val = match.group(2)
    
    true_val = f"%true_cover_{i}"
    not_val = f"%not_cover_{i}"
    replacement = f"{indent}{true_val} = hw.constant -1 : i1\n{indent}{not_val} = comb.xor {val}, {true_val} : i1\n{indent}verif.assert {not_val} : i1"
    
    new_content = ""
    lines = content.split('\n')
    for line in lines:
        if 'verif.cover' in line and match.group(0).strip() in line:
            new_content += replacement + "\n"
        elif 'verif.assert' in line or 'verif.assume' in line or 'verif.cover' in line:
            pass # strip all other asserts, assumes, covers
        else:
            new_content += line + "\n"
            
    tmp_file = f"{mlir_file}.cover{i}.mlir"
    with open(tmp_file, "w") as f:
        f.write(new_content)
        
    res = subprocess.run([circt_bmc, tmp_file, f"--module={module}", "-b", bound, "--run", "--shared-libs=/usr/lib/x86_64-linux-gnu/libz3.so"], capture_output=True, text=True)
    if "Assertion can be violated!" in res.stdout:
        print(f"  Cover {i+1} REACHABLE! (Trace found)")
    else:
        print(f"  Cover {i+1} UNREACHABLE! (No trace found within bound {bound})")
EOF
  python3 "$COVER_PY" "$MLIR" "$MODULE" "$BOUND" "${CIRCT_BIN}/circt-bmc"
fi

if [ "$HAS_ASSERT" -gt 0 ] || [ "$HAS_ASSUME" -gt 0 ] || [ "$HAS_LTL" -gt 0 ] || [ "$HAS_LIVENESS" -gt 0 ]; then
  echo "[2e/3] Word-Level BTOR2 Export (module=${MODULE})..."
  BTOR2="${DIR}/${BASE}.btor2"
  "${CIRCT_BIN}/circt-opt" "$MLIR" --hw-flatten-modules --symbol-dce --lower-ltl-to-bmc -canonicalize --convert-hw-to-btor2 2>/dev/null | awk '/^module \{/{exit} {print}' > "$BTOR2" || true
  if [ -s "$BTOR2" ]; then
    echo "  Successfully exported word-level transition system to ${BTOR2}"
    if command -v btormc &> /dev/null; then
      BTOR_OUT=$(btormc "$BTOR2" 2>&1)
      if echo "$BTOR_OUT" | grep -q "^sat"; then
        BOUND_FOUND=$(echo "$BTOR_OUT" | grep -oP "^b\d+" | head -1 | tr -d 'b')
        echo "  Assertion can be violated! (Counterexample found at bound $BOUND_FOUND)"
        
        # Generate VCD Trace
        TRACE_TXT="${DIR}/${BASE}_trace.txt"
        TRACE_VCD="${DIR}/${BASE}_trace.vcd"
        btormc --trace-gen-full "$BTOR2" > "$TRACE_TXT" 2>/dev/null || true
        if [ -f "btor2vcd.py" ]; then
          python3 btor2vcd.py "$TRACE_TXT" "$TRACE_VCD" >/dev/null 2>&1
          echo "  VCD trace generated: $TRACE_VCD"
        fi
      elif echo "$BTOR_OUT" | grep -q "^unsat"; then
        echo "  Property proved! (No counterexample found)"
      else
        echo "  btormc output: $BTOR_OUT"
      fi
    elif command -v pono &> /dev/null; then
      pono "$BTOR2"
    else
      echo "  (Install 'btormc' or 'pono' to run word-level IC3/PDR on this file)"
    fi
  else
    echo "  (BTOR2 export failed or resulted in empty file)"
  fi
fi

echo "[3/3] Lowering to SystemVerilog..."
"${CIRCT_BIN}/circt-opt" "$MLIR" --lower-seq-to-sv \
  | "${CIRCT_BIN}/firtool" --format=mlir --verilog \
  > "$SV"

echo "Done: ${SV}"
