package parser

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_csharp "github.com/tree-sitter/tree-sitter-c-sharp/bindings/go"
	"testing"
)

// TestCSharpTreeSitterDebug tests the c sharp tree sitter debug.
func TestCSharpTreeSitterDebug(t *testing.T) {
	// Test basic tree-sitter C# parsing
	code := `namespace Test {
    public class MyClass {
        public void MyMethod() {
            Console.WriteLine("Hello");
        }
    }
}`

	parser := tree_sitter.NewParser()
	languagePtr := tree_sitter_csharp.Language()
	language := tree_sitter.NewLanguage(languagePtr)
	err := parser.SetLanguage(language)
	if err != nil {
		t.Fatalf("Failed to set language: %v", err)
	}

	tree := parser.Parse([]byte(code), nil)
	if tree == nil {
		t.Fatal("Failed to parse code")
	}
	defer tree.Close()

	root := tree.RootNode()
	t.Logf("Root node type: %s", root.Kind())
	t.Logf("Root node has %d children", root.ChildCount())

	// Walk the tree
	var walk func(node *tree_sitter.Node, depth int)
	walk = func(node *tree_sitter.Node, depth int) {
		indent := ""
		for i := 0; i < depth; i++ {
			indent += "  "
		}
		t.Logf("%s%s [%d:%d - %d:%d]", indent, node.Kind(),
			node.StartPosition().Row, node.StartPosition().Column,
			node.EndPosition().Row, node.EndPosition().Column)

		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			walk(child, depth+1)
		}
	}

	walk(root, 0)

	// Test a simple query
	queryStr := `(class_declaration name: (identifier) @class.name) @class`
	query, _ := tree_sitter.NewQuery(language, queryStr)
	if query != nil {
		t.Log("Query created successfully")
		// Note: query is managed by Go runtime now, no need to close

		cursor := tree_sitter.NewQueryCursor()
		defer cursor.Close()

		matches := cursor.Matches(query, root, []byte(code))

		matchCount := 0
		for {
			match := matches.Next()
			if match == nil {
				break
			}
			matchCount++
			t.Logf("Match %d has %d captures", match.Id(), len(match.Captures))
			for _, capture := range match.Captures {
				node := capture.Node
				t.Logf("  Capture %d: %s = %s", capture.Index, node.Kind(),
					string(code[node.StartByte():node.EndByte()]))
			}
		}
		t.Logf("Total matches: %d", matchCount)
	}
}
