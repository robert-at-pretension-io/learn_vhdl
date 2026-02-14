package main

import (
    "fmt"
    "strings"

    "github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
)

func main() {
    ext := extractor.New()
    facts, err := ext.Extract("external_tests/osvvm/AXI4/Axi4/src/Axi4ManagerVti.vhd")
    if err != nil {
        panic(err)
    }
    for _, proc := range facts.Processes {
        for _, call := range proc.ProcedureCalls {
            if strings.EqualFold(call.Name, "DoAxiValidHandshake") {
                fmt.Printf("line %d\n", call.Line)
                for i, arg := range call.Args {
                    fmt.Printf("  [%d] %q\n", i, arg)
                }
                return
            }
        }
    }
}
