package main

import (
  "fmt"
  "os"

  "github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
)

func main() {
  if len(os.Args) < 2 {
    fmt.Println("usage: dump_vars <file> [process]")
    os.Exit(1)
  }
  file := os.Args[1]
  filter := ""
  if len(os.Args) > 2 {
    filter = os.Args[2]
  }
  ext := extractor.New()
  facts, err := ext.Extract(file)
  if err != nil {
    fmt.Fprintf(os.Stderr, "extract: %v\n", err)
    os.Exit(1)
  }
  for _, proc := range facts.Processes {
    if filter != "" && proc.Label != filter {
      continue
    }
    fmt.Printf("proc %s vars=%d\n", proc.Label, len(proc.Variables))
    for _, v := range proc.Variables {
      fmt.Printf("  %s : %s\n", v.Name, v.Type)
    }
  }
}
