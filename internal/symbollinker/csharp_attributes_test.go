package symbollinker

import (
	"testing"

	"github.com/tree-sitter/go-tree-sitter"
	"github.com/standardbeagle/lci/internal/types"
)

// TestCSharpAttributes_ClassAttributes tests extraction of class-level attributes
func TestCSharpAttributes_ClassAttributes(t *testing.T) {
	t.Skip("Skipping until tree-sitter parsing is available in tests")
	code := `using System;

[Serializable]
[Obsolete("Use NewClass instead")]
public class OldClass {
    public string Name { get; set; }
}

[ApiController]
[Route("api/[controller]")]
public class UserController {
    [HttpGet]
    public IActionResult GetUsers() {
        return Ok();
    }
}
`
	fileID := types.FileID(1)
	extractor := NewCSharpExtractor()
	tree := parseCSharpCode(t, code)
	if tree == nil {
		t.Skip("Tree-sitter parsing not available")
	}
	table, err := extractor.ExtractSymbols(fileID, []byte(code), tree)

	if err != nil {
		t.Fatalf("ExtractSymbols failed: %v", err)
	}

	// Find the OldClass
	var oldClass *types.EnhancedSymbolInfo
	for _, sym := range table.Symbols {
		if sym.Name == "OldClass" {
			oldClass = sym
			break
		}
	}
	if oldClass == nil {
		t.Fatal("OldClass not found")
	}

	// Verify it has attributes
	if oldClass.Attributes == nil || len(oldClass.Attributes) == 0 {
		t.Fatal("Expected attributes to be extracted")
	}

	// Should have two attributes: Serializable and Obsolete
	if len(oldClass.Attributes) != 2 {
		t.Errorf("Expected 2 attributes, got %d", len(oldClass.Attributes))
	}

	// Check for Serializable attribute
	hasSerializable := false
	hasObsolete := false
	for _, attr := range oldClass.Attributes {
		if attr.Value == "Serializable" {
			hasSerializable = true
			if attr.Type != types.AttrTypeDecorator {
				t.Errorf("Expected AttrTypeDecorator, got %v", attr.Type)
			}
		}
		if attr.Value == "Obsolete(\"Use NewClass instead\")" || attr.Value == "Obsolete" {
			hasObsolete = true
		}
	}

	if !hasSerializable {
		t.Error("Expected Serializable attribute")
	}
	if !hasObsolete {
		t.Error("Expected Obsolete attribute")
	}
}

// TestCSharpAttributes_MethodAttributes tests extraction of method-level attributes
func TestCSharpAttributes_MethodAttributes(t *testing.T) {
	t.Skip("Skipping until tree-sitter parsing is available in tests")
	code := `using System;

public class TestClass {
    [Test]
    [Category("Unit")]
    [Timeout(5000)]
    public void TestMethod() {
        // test code
    }

    [HttpPost]
    [Authorize(Roles = "Admin")]
    public IActionResult CreateUser(User user) {
        return Ok();
    }
}
`
	fileID := types.FileID(1)
	extractor := NewCSharpExtractor()
	tree := parseCSharpCode(t, code)
	if tree == nil {
		t.Skip("Tree-sitter parsing not available")
	}
	table, err := extractor.ExtractSymbols(fileID, []byte(code), tree)

	if err != nil {
		t.Fatalf("ExtractSymbols failed: %v", err)
	}

	// Find the TestMethod
	var testMethod *types.EnhancedSymbolInfo
	for _, sym := range table.Symbols {
		if sym.Name == "TestClass.TestMethod" {
			testMethod = sym
			break
		}
	}
	if testMethod == nil {
		t.Fatal("TestMethod not found")
	}

	// Verify it has attributes
	if testMethod.Attributes == nil || len(testMethod.Attributes) == 0 {
		t.Fatal("Expected attributes to be extracted")
	}

	// Should have three attributes
	if len(testMethod.Attributes) < 3 {
		t.Errorf("Expected at least 3 attributes, got %d", len(testMethod.Attributes))
	}

	// Check for Test attribute
	hasTest := false
	for _, attr := range testMethod.Attributes {
		if attr.Value == "Test" {
			hasTest = true
		}
	}
	if !hasTest {
		t.Error("Expected Test attribute")
	}
}

// TestCSharpAttributes_PropertyAttributes tests extraction of property-level attributes
func TestCSharpAttributes_PropertyAttributes(t *testing.T) {
	t.Skip("Skipping until tree-sitter parsing is available in tests")
	code := `using System.ComponentModel.DataAnnotations;

public class User {
    [Required]
    [StringLength(100)]
    public string Name { get; set; }

    [EmailAddress]
    [Required(ErrorMessage = "Email is required")]
    public string Email { get; set; }

    [Range(18, 120)]
    public int Age { get; set; }
}
`
	fileID := types.FileID(1)
	extractor := NewCSharpExtractor()
	tree := parseCSharpCode(t, code)
	if tree == nil {
		t.Skip("Tree-sitter parsing not available")
	}
	table, err := extractor.ExtractSymbols(fileID, []byte(code), tree)

	if err != nil {
		t.Fatalf("ExtractSymbols failed: %v", err)
	}

	// Find the Name property
	var nameProperty *types.EnhancedSymbolInfo
	for _, sym := range table.Symbols {
		if sym.Name == "User.Name" {
			nameProperty = sym
			break
		}
	}
	if nameProperty == nil {
		t.Fatal("Name property not found")
	}

	// Verify it has attributes
	if nameProperty.Attributes == nil || len(nameProperty.Attributes) == 0 {
		t.Fatal("Expected attributes to be extracted")
	}

	// Should have two attributes: Required and StringLength
	if len(nameProperty.Attributes) < 2 {
		t.Errorf("Expected at least 2 attributes, got %d", len(nameProperty.Attributes))
	}

	// Check for Required attribute
	hasRequired := false
	for _, attr := range nameProperty.Attributes {
		if attr.Value == "Required" {
			hasRequired = true
		}
	}
	if !hasRequired {
		t.Error("Expected Required attribute")
	}
}

// TestCSharpAttributes_MultipleAttributeLists tests multiple attribute lists on same declaration
func TestCSharpAttributes_MultipleAttributeLists(t *testing.T) {
	t.Skip("Skipping until tree-sitter parsing is available in tests")
	code := `using System;

[Serializable]
[Obsolete]
[CustomAttribute("value")]
public class MultiAttributeClass {
}

public class TestClass {
    [Test]
    [Category("Integration")]
    [ExpectedException(typeof(ArgumentException))]
    public void MultiAttributeMethod() {
    }
}
`
	fileID := types.FileID(1)
	extractor := NewCSharpExtractor()
	tree := parseCSharpCode(t, code)
	if tree == nil {
		t.Skip("Tree-sitter parsing not available")
	}
	table, err := extractor.ExtractSymbols(fileID, []byte(code), tree)

	if err != nil {
		t.Fatalf("ExtractSymbols failed: %v", err)
	}

	// Find the MultiAttributeClass
	var multiClass *types.EnhancedSymbolInfo
	for _, sym := range table.Symbols {
		if sym.Name == "MultiAttributeClass" {
			multiClass = sym
			break
		}
	}
	if multiClass == nil {
		t.Fatal("MultiAttributeClass not found")
	}

	// Verify it has all three attributes
	if len(multiClass.Attributes) < 3 {
		t.Errorf("Expected at least 3 attributes, got %d", len(multiClass.Attributes))
	}
}

// Helper function
func parseCSharpCode(t *testing.T, code string) *tree_sitter.Tree {
	t.Helper()
	// Tree-sitter parsing not implemented in test environment yet
	return nil
}
