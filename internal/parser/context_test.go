package parser

import (
	"context"
	"testing"
	"time"
)

// TestParseFileWithContext_Cancellation tests the parse file with context cancellation.
func TestParseFileWithContext_Cancellation(t *testing.T) {
	parser := NewTreeSitterParser()

	// Create a context that will be cancelled
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Wait for context to be cancelled
	time.Sleep(10 * time.Millisecond)

	testCode := `
	class TestClass {
		public void testMethod() {
			// Some code here
		}
	}
	`

	// This should handle the cancelled context gracefully
	blocks, symbols, imports := parser.ParseFileWithContext(ctx, "test.java", []byte(testCode))

	// The parsing should complete quickly and return empty results due to cancelled context
	// or complete normally if parsing is fast enough
	t.Logf("Parsed with cancelled context: blocks=%d, symbols=%d, imports=%d",
		len(blocks), len(symbols), len(imports))
}

// TestParseFileWithContext_Success tests the parse file with context success.
func TestParseFileWithContext_Success(t *testing.T) {
	parser := NewTreeSitterParser()

	// Create a context with sufficient timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testCode := `
	public class TestClass {
		private int value;
		
		public void testMethod() {
			System.out.println("Hello");
		}
	}
	`

	// This should complete successfully
	blocks, symbols, imports := parser.ParseFileWithContext(ctx, "test.java", []byte(testCode))

	// Should find at least the class and method
	if len(symbols) == 0 {
		t.Error("Expected to find symbols with valid context")
	}

	t.Logf("Parsed with valid context: blocks=%d, symbols=%d, imports=%d",
		len(blocks), len(symbols), len(imports))
}
