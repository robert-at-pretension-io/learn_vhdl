package main

import (
  "fmt"
  "os"
  "strings"

  "github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
)

func main() {
  if len(os.Args) < 2 {
    fmt.Println("usage: print_procs <file> [filter]")
    os.Exit(1)
  }
  file := os.Args[1]
  filter := ""
  if len(os.Args) > 2 {
    filter = os.Args[2]
  }
  e := extractor.New()
  facts, err := e.Extract(file)
  if err != nil {
    fmt.Println("extract error:", err)
    os.Exit(1)
  }
  for _, proc := range facts.Procedures {
    if filter != "" && !strings.Contains(proc.Name, filter) {
      continue
    }
    fmt.Printf("%s:%d proc=%s pkg=%s params=%v\n", file, proc.Line, proc.Name, proc.InPackage, proc.Parameters)
  }
}
