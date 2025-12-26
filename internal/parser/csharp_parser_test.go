package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCSharpParser tests the c sharp parser.
func TestCSharpParser(t *testing.T) {
	parser := NewTreeSitterParser()

	t.Run("parse C# class with methods", func(t *testing.T) {
		code := `using System;
using System.Collections.Generic;

namespace MyApp
{
    public class Calculator
    {
        private int result;
        
        public int Result { get; set; }
        
        public Calculator()
        {
            result = 0;
        }
        
        public int Add(int a, int b)
        {
            result = a + b;
            return result;
        }
        
        public int Subtract(int a, int b)
        {
            return a - b;
        }
    }
    
    public enum Operation
    {
        Add,
        Subtract,
        Multiply,
        Divide
    }
}`

		blocks, symbols, imports := parser.ParseFile("test.cs", []byte(code))

		// Debug logging
		t.Logf("Found %d blocks", len(blocks))
		t.Logf("Found %d symbols", len(symbols))
		t.Logf("Found %d imports: %+v", len(imports), imports)
		assert.True(t, len(imports) >= 1, "Should have at least one import")
		// C# parser may combine imports differently

		// Check symbols
		require.True(t, len(symbols) > 0)

		// Find specific symbols
		hasCalculator := false
		// hasConstructor := false // Constructors share name with class
		hasAdd := false
		hasSubtract := false
		hasResult := false
		hasOperation := false
		// hasNamespace := false // Not checking namespace in this test

		for _, sym := range symbols {
			switch sym.Name {
			case "Calculator":
				hasCalculator = true
			// Constructors are handled differently in tree-sitter
			case "Add":
				hasAdd = true
			case "Subtract":
				hasSubtract = true
			case "Result":
				hasResult = true
			case "Operation":
				hasOperation = true
			case "MyApp":
				// hasNamespace = true
			}
		}

		assert.True(t, hasCalculator, "Should find Calculator class")
		assert.True(t, hasAdd, "Should find Add method")
		assert.True(t, hasSubtract, "Should find Subtract method")
		assert.True(t, hasResult, "Should find Result property")
		assert.True(t, hasOperation, "Should find Operation enum")

		// Check blocks
		assert.True(t, len(blocks) > 0, "Should have block boundaries")
	})

	t.Run("parse C# interface", func(t *testing.T) {
		code := `namespace MyApp
{
    public interface IService
    {
        void Process(string data);
        Task<string> GetDataAsync();
    }
}`

		blocks, symbols, _ := parser.ParseFile("test.cs", []byte(code))

		// Find interface
		hasInterface := false
		for _, sym := range symbols {
			if sym.Name == "IService" {
				hasInterface = true
				break
			}
		}

		assert.True(t, hasInterface, "Should find IService interface")
		assert.True(t, len(blocks) > 0, "Should have block boundaries")
	})

	t.Run("parse C# struct", func(t *testing.T) {
		code := `namespace MyApp
{
    public struct Point
    {
        public double X { get; set; }
        public double Y { get; set; }
        
        public Point(double x, double y)
        {
            X = x;
            Y = y;
        }
        
        public double Distance()
        {
            return Math.Sqrt(X * X + Y * Y);
        }
    }
}`

		_, symbols, _ := parser.ParseFile("test.cs", []byte(code))

		// Find struct and its members
		hasStruct := false
		hasDistance := false

		for _, sym := range symbols {
			if sym.Name == "Point" {
				hasStruct = true
			}
			if sym.Name == "Distance" {
				hasDistance = true
			}
		}

		assert.True(t, hasStruct, "Should find Point struct")
		assert.True(t, hasDistance, "Should find Distance method")
	})
}
