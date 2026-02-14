package main

import (
  "fmt"
  "os"
  "github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
)

func main() {
  if len(os.Args) < 2 {
    fmt.Println("usage: dump_context <file>")
    os.Exit(1)
  }
  file := os.Args[1]
  ext := extractor.New()
  facts, err := ext.Extract(file)
  if err != nil {
    fmt.Fprintf(os.Stderr, "extract: %v\n", err)
    os.Exit(1)
  }
  for _, ctx := range facts.ContextDecls {
    fmt.Printf("context %s\n", ctx.Name)
    fmt.Printf("  libraries: %v\n", ctx.Libraries)
    fmt.Printf("  use_items: %v\n", ctx.UseItems)
    fmt.Printf("  context_refs: %v\n", ctx.ContextRefs)
  }
  for _, clause := range facts.ContextClauses {
    fmt.Printf("context_clause %s (line %d)\n", clause.Name, clause.Line)
  }
}
