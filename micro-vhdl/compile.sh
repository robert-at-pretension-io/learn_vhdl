#!/bin/bash
set -e
if [ "$#" -ne 1 ]; then
  echo "Usage: ./compile.sh <file.vhd>"
  exit 1
fi

CIRCT_BIN=../cirt/build/bin
BOUND=${BOUND:-15}
INPUT="$1"
DIR="$(dirname "$INPUT")"
BASE="$(basename "${INPUT%.*}")"
MLIR="${DIR}/${BASE}.mlir"
SV="${DIR}/${BASE}.sv"

echo "[1/3] Parsing and Extracting Micro-VHDL..."
go run . "$INPUT" "$MLIR"

# Extract top-level module name from the generated MLIR (first hw.module @name)
MODULE=$(grep -oP '(?<=hw\.module @)\w+' "$MLIR" | head -1)

echo "[2/3] Mathematical Formal Proof via Z3 SMT Solver (module=${MODULE}, bound=${BOUND} cycles)..."
# circt-bmc unrolls the circuit for BOUND clock cycles and hands the resulting
# SMT formula to Z3.  Any reachable state that violates a verif.assert will be
# reported as "Assertion can be violated!" with a counterexample trace.
# Override the bound: BOUND=30 ./compile.sh file.vhd
"${CIRCT_BIN}/circt-bmc" \
  "$MLIR" \
  --module="${MODULE}" \
  -b "${BOUND}" \
  --run \
  --shared-libs=/usr/lib/x86_64-linux-gnu/libz3.so

echo "[3/3] Lowering to SystemVerilog..."
"${CIRCT_BIN}/circt-opt" "$MLIR" --lower-seq-to-sv \
  | "${CIRCT_BIN}/firtool" --format=mlir --verilog \
  > "$SV"

echo "Done: ${SV}"
