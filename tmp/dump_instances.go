package main

import (
  "fmt"
  "os"

  "github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
)

func main() {
  if len(os.Args) < 2 {
    fmt.Println("usage: dump_instances <file>")
    os.Exit(1)
  }
  file := os.Args[1]
  ext := extractor.New()
  facts, err := ext.Extract(file)
  if err != nil {
    fmt.Fprintf(os.Stderr, "extract: %v\n", err)
    os.Exit(1)
  }
  for _, inst := range facts.Instances {
    fmt.Printf("inst %s target=%s line=%d in_arch=%s\n", inst.Name, inst.Target, inst.Line, inst.InArch)
    if len(inst.PortMap) > 0 {
      fmt.Printf("  ports: %v\n", inst.PortMap)
    }
  }
}
