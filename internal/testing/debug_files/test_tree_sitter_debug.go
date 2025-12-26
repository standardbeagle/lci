//go:build debug
// +build debug

package main

import (
	"fmt"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/tree-sitter/tree-sitter-go/bindings/go"
)

func printTree(node *tree_sitter.Node, content []byte, depth int) {
	if node == nil {
		return
	}

	indent := ""
	for i := 0; i < depth; i++ {
		indent += "  "
	}

	nodeText := ""
	if node.EndByte() <= uint(len(content)) {
		nodeText = string(content[node.StartByte():node.EndByte()])
		if len(nodeText) > 50 {
			nodeText = nodeText[:50] + "..."
		}
	}

	fmt.Printf("%s%s [%d:%d - %d:%d] %q\n",
		indent,
		node.Kind(),
		node.StartPosition().Row,
		node.StartPosition().Column,
		node.EndPosition().Row,
		node.EndPosition().Column,
		nodeText)

	for i := uint(0); i < node.ChildCount(); i++ {
		printTree(node.Child(i), content, depth+1)
	}
}

func main() {
	code := []byte(`func main() {
    f := func() {
        println("hello")
    }
}`)

	parser := tree_sitter.NewParser()
	parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_go.Language()))

	tree := parser.Parse(code, nil)
	if tree == nil {
		fmt.Println("Failed to parse")
		return
	}

	printTree(tree.RootNode(), code, 0)
}
