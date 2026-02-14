package main

import (
    "fmt"
    "os"

    "github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
)

func main() {
    if len(os.Args) < 2 {
        fmt.Fprintln(os.Stderr, "usage: extract_subtypes <file>")
        os.Exit(1)
    }
    ext := extractor.New()
    facts, err := ext.Extract(os.Args[1])
    if err != nil {
        fmt.Fprintf(os.Stderr, "extract failed: %v\n", err)
        os.Exit(1)
    }
    for _, st := range facts.Subtypes {
        fmt.Printf("%s\tbase=%s\tpackage=%s\n", st.Name, st.BaseType, st.InPackage)
    }
}
