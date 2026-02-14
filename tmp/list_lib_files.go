package main

import (
  "fmt"
  "os"
  "strings"

  "github.com/robert-at-pretension-io/vhdl-lint/internal/config"
)

func main() {
  if len(os.Args) < 3 {
    fmt.Println("usage: list_lib_files <config> <root>")
    os.Exit(1)
  }
  cfgPath := os.Args[1]
  root := os.Args[2]
  cfg, err := config.LoadFile(cfgPath)
  if err != nil {
    fmt.Println("load:", err)
    os.Exit(1)
  }
  libs, err := cfg.ResolveLibraries(root)
  if err != nil {
    fmt.Println("resolve:", err)
    os.Exit(1)
  }
  for _, lib := range libs {
    for _, f := range lib.Files {
      if strings.Contains(f, "altera") || strings.Contains(f, "Altera") {
        fmt.Printf("%s: %s\n", lib.Name, f)
      }
    }
  }
}
