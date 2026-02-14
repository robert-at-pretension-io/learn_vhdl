package main

import (
  "fmt"
  "os"
  "regexp"

  sitter "github.com/smacker/go-tree-sitter"
  tree_sitter_vhdl "github.com/tree-sitter/tree-sitter-vhdl"
)

func main() {
  if len(os.Args) < 2 {
    fmt.Println("usage: dump_loops <file>")
    os.Exit(1)
  }
  path := os.Args[1]
  source, err := os.ReadFile(path)
  if err != nil {
    fmt.Println("read:", err)
    os.Exit(1)
  }
  parser := sitter.NewParser()
  parser.SetLanguage(sitter.NewLanguage(tree_sitter_vhdl.Language()))
  tree := parser.Parse(nil, source)
  root := tree.RootNode()
  re := regexp.MustCompile(`(?i)\bfor\s+([a-zA-Z_][a-zA-Z0-9_]*)\s+in\b`)
  var walk func(n *sitter.Node)
  walk = func(n *sitter.Node) {
    if n == nil {
      return
    }
    if n.Type() == "loop_statement" {
      text := n.Content(source)
      match := re.FindStringSubmatch(text)
      fmt.Printf("loop @%d:%d match=%v\n%s\n---\n", n.StartPoint().Row+1, n.StartPoint().Column+1, match, text)
    }
    for i := 0; i < int(n.ChildCount()); i++ {
      walk(n.Child(i))
    }
  }
  walk(root)
}
