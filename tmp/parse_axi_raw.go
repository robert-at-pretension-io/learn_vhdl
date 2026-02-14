package main

import (
    "fmt"
    "os"
    "strings"
)

func stripLineComments(raw string) string {
    if !strings.Contains(raw, "--") {
        return raw
    }
    lines := strings.Split(raw, "\n")
    for i, line := range lines {
        if idx := strings.Index(line, "--"); idx != -1 {
            lines[i] = line[:idx]
        }
    }
    return strings.Join(lines, "\n")
}

func splitArgsRespectParens(raw string) []string {
    var args []string
    start := 0
    depth := 0
    for i := 0; i < len(raw); i++ {
        switch raw[i] {
        case '(':
            depth++
        case ')':
            if depth > 0 {
                depth--
            }
        case ',':
            if depth == 0 {
                part := strings.TrimSpace(raw[start:i])
                if part != "" {
                    args = append(args, part)
                }
                start = i + 1
            }
        }
    }
    if start < len(raw) {
        part := strings.TrimSpace(raw[start:])
        if part != "" {
            args = append(args, part)
        }
    }
    return args
}

func main() {
    data, err := os.ReadFile("external_tests/osvvm/AXI4/Axi4/src/Axi4ManagerVti.vhd")
    if err != nil {
        panic(err)
    }
    src := string(data)
    idx := strings.Index(src, "DoAxiValidHandshake (")
    if idx == -1 {
        panic("not found")
    }
    // parse using depth after stripping comments
    content := stripLineComments(src[idx:])
    openIdx := strings.Index(content, "(")
    if openIdx == -1 {
        panic("open paren not found")
    }
    depth := 0
    closeIdx := -1
    for i := openIdx; i < len(content); i++ {
        switch content[i] {
        case '(':
            depth++
        case ')':
            depth--
            if depth == 0 {
                closeIdx = i
                i = len(content)
            }
        }
    }
    if closeIdx == -1 {
        panic("close paren not found")
    }
    argList := content[openIdx+1 : closeIdx]
    args := splitArgsRespectParens(argList)
    fmt.Printf("PARSED ARGS (%d):\n", len(args))
    for _, a := range args {
        fmt.Printf("- %s\n", strings.TrimSpace(a))
    }
}
