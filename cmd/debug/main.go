package main

import (
	"fmt"
	"os"

	sitter "github.com/smacker/go-tree-sitter"
	tree_sitter_vhdl "github.com/tree-sitter/tree-sitter-vhdl"
)

func main() {
	parser := sitter.NewParser()
	lang := sitter.NewLanguage(tree_sitter_vhdl.Language())
	parser.SetLanguage(lang)
	
	source := []byte(`entity test is end;
architecture a of test is
begin
  process(x)
  begin
    if cmd_starts_msg(rinputs.word) then
      null;
    end if;
  end process;
end;`)
	
	tree, _ := parser.ParseCtx(nil, nil, source)
	root := tree.RootNode()
	
	// Find the if_statement
	var findIfStmt func(n *sitter.Node) *sitter.Node
	findIfStmt = func(n *sitter.Node) *sitter.Node {
		if n.Type() == "if_statement" {
			return n
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			if found := findIfStmt(n.Child(i)); found != nil {
				return found
			}
		}
		return nil
	}
	
	ifNode := findIfStmt(root)
	if ifNode == nil {
		fmt.Println("No if_statement found")
		os.Exit(1)
	}
	
	fmt.Printf("if_statement has %d children:\n", ifNode.ChildCount())
	for i := 0; i < int(ifNode.ChildCount()); i++ {
		child := ifNode.Child(i)
		fieldName := ifNode.FieldNameForChild(i)
		fmt.Printf("  [%d] type=%s field=%q content=%q\n", i, child.Type(), fieldName, child.Content(source))
	}
}
