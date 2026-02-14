package main

import (
    "fmt"
    "strings"

    "github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
)

func main() {
    ext := extractor.New()
    facts, err := ext.Extract("external_tests/PoC/tb/dstruct/dstruct_deque_tb.vhdl")
    if err != nil {
        panic(err)
    }
    for _, proc := range facts.Processes {
        for _, call := range proc.ProcedureCalls {
            if strings.EqualFold(call.Name, "simAssertion") {
                fmt.Printf("line %d args=%v\n", call.Line, call.Args)
            }
        }
    }
}
