package main

import (
    "fmt"
    "os"

    "github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
)

func main() {
    if len(os.Args) < 2 {
        fmt.Fprintln(os.Stderr, "usage: extract_types <file>")
        os.Exit(1)
    }
    ext := extractor.New()
    facts, err := ext.Extract(os.Args[1])
    if err != nil {
        fmt.Fprintf(os.Stderr, "extract failed: %v\n", err)
        os.Exit(1)
    }
    for _, ty := range facts.Types {
        fmt.Printf("%s\tkind=%s\tpackage=%s\n", ty.Name, ty.Kind, ty.InPackage)
    }
}
