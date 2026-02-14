package main

import (
  "fmt"
  "os"

  "github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
)

func main() {
  if len(os.Args) < 2 {
    fmt.Println("usage: dump_components <file>")
    os.Exit(1)
  }
  file := os.Args[1]
  ext := extractor.New()
  facts, err := ext.Extract(file)
  if err != nil {
    fmt.Fprintf(os.Stderr, "extract: %v\n", err)
    os.Exit(1)
  }
  for _, comp := range facts.Components {
    fmt.Printf("comp name=%s is_instance=%v line=%d\n", comp.Name, comp.IsInstance, comp.Line)
  }
}
