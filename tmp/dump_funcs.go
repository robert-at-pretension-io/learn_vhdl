package main

import (
  "fmt"
  "os"
  "strings"

  "github.com/robert-at-pretension-io/vhdl-lint/internal/config"
  "github.com/robert-at-pretension-io/vhdl-lint/internal/indexer"
)

func main() {
  if len(os.Args) < 2 {
    fmt.Println("usage: dump_funcs <path> [name]")
    os.Exit(1)
  }
  path := os.Args[1]
  name := ""
  if len(os.Args) > 2 {
    name = os.Args[2]
  }
  cfg, err := config.Load(path)
  if err != nil {
    fmt.Fprintf(os.Stderr, "config error: %v\n", err)
    os.Exit(1)
  }
  idx := indexer.NewWithConfig(cfg)
  idx.JSONOutput = false
  if err := idx.Run(path); err != nil {
    fmt.Fprintf(os.Stderr, "indexer error: %v\n", err)
    os.Exit(1)
  }
  for _, facts := range idx.Facts {
    for _, fn := range facts.Functions {
      if name != "" && !strings.EqualFold(fn.Name, name) {
        continue
      }
      fmt.Printf("%s.%s %s(%d params) file=%s\n", fn.InPackage, fn.InArch, fn.Name, len(fn.Parameters), facts.File)
      for _, p := range fn.Parameters {
        fmt.Printf("  %s : %s %s default=%q\n", p.Name, p.Direction, p.Type, p.Default)
      }
    }
  }
}
