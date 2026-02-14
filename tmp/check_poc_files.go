package main

import (
  "fmt"
  "os"

  "github.com/robert-at-pretension-io/vhdl-lint/internal/config"
)

func main() {
  cfg, err := config.LoadFile("vhdl_lint.json")
  if err != nil {
    fmt.Println("load:", err)
    os.Exit(1)
  }
  if _, ok := cfg.ApplyProjectOverrides("external_tests/PoC"); ok {
    // project overrides applied
  }
  libs, err := cfg.ResolveLibraries("external_tests/PoC")
  if err != nil {
    fmt.Println("resolve:", err)
    os.Exit(1)
  }
  target := "/home/elliot/Projects/vhdl_compiler/external_tests/PoC/src/arith/xilinx/arith_cca_xilinx.vhdl"
  found := false
  for _, lib := range libs {
    for _, f := range lib.Files {
      if f == target {
        fmt.Printf("found in lib %s\n", lib.Name)
        found = true
      }
    }
  }
  if !found {
    fmt.Println("not found")
  }
}
