package main

import (
  "fmt"
  "os"

  "github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
)

func main() {
  if len(os.Args) < 2 {
    fmt.Println("usage: dump_signals <file> [name]")
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
  for _, sig := range facts.Signals {
    if filter != "" && sig.Name != filter {
      continue
    }
    fmt.Printf("%s : %s (in_entity=%s)\n", sig.Name, sig.Type, sig.InEntity)
  }
}
