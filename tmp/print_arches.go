package main

import (
  "fmt"
  "os"

  "github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
)

func main() {
  if len(os.Args) < 2 {
    fmt.Println("usage: print_arches <file>")
    os.Exit(1)
  }
  file := os.Args[1]
  e := extractor.New()
  facts, err := e.Extract(file)
  if err != nil {
    fmt.Println("extract error:", err)
    os.Exit(1)
  }
  for _, arch := range facts.Architectures {
    fmt.Printf("%s:%d arch=%s entity=%s\n", file, arch.Line, arch.Name, arch.EntityName)
  }
}
