package main

import (
  "fmt"
  "os"
  "strings"

  "github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
)

func main() {
  if len(os.Args) < 3 {
    fmt.Println("usage: print_type_fields <file> <type>")
    os.Exit(1)
  }
  file := os.Args[1]
  name := os.Args[2]
  ext := extractor.New()
  facts, err := ext.Extract(file)
  if err != nil {
    fmt.Fprintf(os.Stderr, "extract: %v\n", err)
    os.Exit(1)
  }
  for _, ty := range facts.Types {
    if !strings.EqualFold(ty.Name, name) {
      continue
    }
    fmt.Printf("type %s kind=%s inArch=%s\n", ty.Name, ty.Kind, ty.InArch)
    for _, f := range ty.Fields {
      fmt.Printf("  field %s : %s\n", f.Name, f.Type)
    }
  }
}
