package main

import (
    "fmt"
    "strings"

    "github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
)

func main() {
    ext := extractor.New()
    facts, err := ext.Extract("external_tests/osvvm/AXI4/common/src/Axi4CommonPkg.vhd")
    if err != nil {
        panic(err)
    }
    for _, proc := range facts.Procedures {
        if strings.EqualFold(proc.Name, "DoAxiValidHandshake") {
            fmt.Printf("proc %s params:\n", proc.Name)
            for _, p := range proc.Parameters {
                fmt.Printf("- %s : %s\n", p.Name, p.Type)
            }
        }
        if strings.EqualFold(proc.Name, "DoAxiReadyHandshake") {
            fmt.Printf("proc %s params:\n", proc.Name)
            for _, p := range proc.Parameters {
                fmt.Printf("- %s : %s\n", p.Name, p.Type)
            }
        }
    }
}
