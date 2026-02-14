package main

import (
  "fmt"
  "os"
  "strings"

  "github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
)

func main() {
  if len(os.Args) < 2 {
    fmt.Println("usage: print_proc_calls_raw <file> [filter]")
    os.Exit(1)
  }
  file := os.Args[1]
  filter := ""
  if len(os.Args) > 2 {
    filter = os.Args[2]
  }
  e := extractor.New()
  facts, err := e.Extract(file)
  if err != nil {
    fmt.Println("extract error:", err)
    os.Exit(1)
  }
  for _, proc := range facts.Processes {
    for _, call := range proc.ProcedureCalls {
      if filter != "" && !strings.Contains(call.Name, filter) {
        continue
      }
      fmt.Printf("%s:%d process=%s call=%s args=", file, call.Line, proc.Label, call.Name)
      for i, arg := range call.Args {
        if i > 0 {
          fmt.Print(", ")
        }
        fmt.Printf("%q", arg)
      }
      fmt.Println()
    }
  }
}
