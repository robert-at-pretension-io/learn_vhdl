package main

import (
  "fmt"
  "os"

  "github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
)

func main() {
  if len(os.Args) < 2 {
    fmt.Println("usage: print_contexts <file>")
    os.Exit(1)
  }
  file := os.Args[1]
  e := extractor.New()
  facts, err := e.Extract(file)
  if err != nil {
    fmt.Println("extract error:", err)
    os.Exit(1)
  }
  for _, decl := range facts.ContextDecls {
    fmt.Printf("%s:%d context=%s libs=%v use=%v refs=%v\n", file, decl.Line, decl.Name, decl.Libraries, decl.UseItems, decl.ContextRefs)
  }
  for _, clause := range facts.ContextClauses {
    fmt.Printf("%s:%d context_clause=%s\n", file, clause.Line, clause.Name)
  }
}
