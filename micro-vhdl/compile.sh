#!/bin/bash
set -e
if [ "$#" -ne 1 ]; then
  echo "Usage: ./compile.sh <file.vhd>"
  exit 1
fi
go run . "$1"
../cirt-bin/bin/circt-opt build.mlir --lower-seq-to-sv | ../cirt-bin/bin/firtool --format=mlir --verilog > "${1%.*}.sv"
echo "Successfully compiled to ${1%.*}.sv:"
cat "${1%.*}.sv"
