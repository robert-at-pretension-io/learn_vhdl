package main

import (
  "fmt"
  "os"

  "github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
)

func main() {
  if len(os.Args) < 2 {
    fmt.Println("usage: dump_calls <file> [name]")
    os.Exit(1)
  }
  file := os.Args[1]
  name := ""
  if len(os.Args) > 2 {
    name = os.Args[2]
  }
  e := extractor.New()
  facts, err := e.Extract(file)
  if err != nil {
    fmt.Fprintf(os.Stderr, "extract error: %v\n", err)
    os.Exit(1)
  }
  for _, proc := range facts.Processes {
    for _, call := range proc.FunctionCalls {
      if name == "" || call.Name == name {
        fmt.Printf("FUNC %s:%d arch=%s proc=%s %s len=%d\n", file, call.Line, proc.InArch, proc.Label, call.Name, len(call.Args))
        for i, arg := range call.Args {
          fmt.Printf("  arg[%d]=%q\n", i, arg)
        }
      }
    }
    for _, call := range proc.ProcedureCalls {
      if name == "" || call.Name == name {
        fmt.Printf("PROC %s:%d arch=%s proc=%s %s len=%d\n", file, call.Line, proc.InArch, proc.Label, call.Name, len(call.Args))
        for i, arg := range call.Args {
          fmt.Printf("  arg[%d]=%q\n", i, arg)
        }
      }
    }
  }
}
