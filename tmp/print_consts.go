package main

import (
  "fmt"
  "os"
  "strings"

  "github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
)

func main() {
  if len(os.Args) < 2 {
    fmt.Println("usage: print_consts <file> [name]")
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
  for _, c := range facts.ConstantDecls {
    if filter != "" && !strings.EqualFold(c.Name, filter) {
      continue
    }
    fmt.Printf("const %s type=%s inArch=%s inPkg=%s\n", c.Name, c.Type, c.InArch, c.InPackage)
  }
}
