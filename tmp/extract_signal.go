package main

import (
    "fmt"
    "os"

    "github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
)

func main() {
    if len(os.Args) < 2 {
        fmt.Fprintln(os.Stderr, "usage: extract_signal <file>")
        os.Exit(1)
    }
    path := os.Args[1]
    ext := extractor.New()
    facts, err := ext.Extract(path)
    if err != nil {
        fmt.Fprintf(os.Stderr, "extract failed: %v\n", err)
        os.Exit(1)
    }
    for _, sig := range facts.Signals {
        fmt.Printf("%s\t%s\tline=%d\tctx=%s\n", sig.Name, sig.Type, sig.Line, sig.InEntity)
    }
}
