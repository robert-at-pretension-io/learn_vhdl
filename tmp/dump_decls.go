package main

import (
  "fmt"
  "os"
  "strings"

  "github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
)

func main() {
  if len(os.Args) < 2 {
    fmt.Println("usage: dump_decls <file> [name]")
    os.Exit(1)
  }
  file := os.Args[1]
  name := ""
  if len(os.Args) > 2 {
    name = os.Args[2]
  }
  ext := extractor.New()
  facts, err := ext.Extract(file)
  if err != nil {
    fmt.Fprintf(os.Stderr, "extract: %v\n", err)
    os.Exit(1)
  }
  for _, fn := range facts.Functions {
    if name != "" && !strings.EqualFold(fn.Name, name) {
      continue
    }
    fmt.Printf("%s.%s %s(%d params)\n", fn.InPackage, fn.InArch, fn.Name, len(fn.Parameters))
    for _, p := range fn.Parameters {
      fmt.Printf("  %s : %s %s default=%q\n", p.Name, p.Direction, p.Type, p.Default)
    }
  }
}
