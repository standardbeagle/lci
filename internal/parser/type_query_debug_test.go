package parser

import (
	"fmt"
	"testing"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
)

// TestGoTypeDeclarationQuery tests if tree-sitter query can match type declarations
func TestGoTypeDeclarationQuery(t *testing.T) {
	code := `package main

type Client struct {
	ID int
}

type Service interface {
	Do() error
}

func NewClient() *Client {
	return &Client{}
}
`

	parser := tree_sitter.NewParser()
	defer parser.Close()

	language := tree_sitter.NewLanguage(tree_sitter_go.Language())
	parser.SetLanguage(language)

	tree := parser.Parse([]byte(code), nil)
	defer tree.Close()

	// Test the exact query from parser.go
	queryStr := `
        (type_declaration
            (type_spec name: (type_identifier) @type.name)) @type
    `

	query, err := tree_sitter.NewQuery(language, queryStr)
	if err != nil {
		t.Fatalf("Failed to create query: %v", err)
	}
	defer query.Close()

	qc := tree_sitter.NewQueryCursor()
	defer qc.Close()

	matches := qc.Matches(query, tree.RootNode(), []byte(code))
	captureNames := query.CaptureNames()

	matchCount := 0
	for {
		match := matches.Next()
		if match == nil {
			break
		}
		matchCount++

		for _, capture := range match.Captures {
			captureName := captureNames[capture.Index]
			node := capture.Node
			text := code[node.StartByte():node.EndByte()]

			fmt.Printf("Match %d: capture=%s, node.Kind()=%s, text=%q\n",
				matchCount, captureName, node.Kind(), text)
		}
	}

	if matchCount == 0 {
		t.Error("Query matched 0 type declarations - the query is broken!")
	} else {
		t.Logf("Successfully matched %d type declarations", matchCount)
	}
}
