package parser

import (
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

// TestZigParser tests the zig parser.
func TestZigParser(t *testing.T) {
	parser := NewTreeSitterParser()

	testCode := `const std = @import("std");
const print = std.debug.print;

// Constants and variables
const PI: f32 = 3.14159;
var global_counter: u32 = 0;

// Struct definition
const Point = struct {
    x: f32,
    y: f32,
    
    // Method
    pub fn distance(self: Point, other: Point) f32 {
        const dx = self.x - other.x;
        const dy = self.y - other.y;
        return @sqrt(dx * dx + dy * dy);
    }
    
    // Associated function
    pub fn zero() Point {
        return Point{ .x = 0, .y = 0 };
    }
};

// Union
const Value = union(enum) {
    int: i32,
    float: f32,
    boolean: bool,
    
    pub fn asFloat(self: Value) f32 {
        return switch (self) {
            .int => |i| @intToFloat(f32, i),
            .float => |f| f,
            .boolean => |b| if (b) 1.0 else 0.0,
        };
    }
};

// Function with error handling
fn divide(a: f32, b: f32) !f32 {
    if (b == 0) {
        return error.DivisionByZero;
    }
    return a / b;
}

// Generic function
fn swap(comptime T: type, a: *T, b: *T) void {
    const temp = a.*;
    a.* = b.*;
    b.* = temp;
}

// Main function
pub fn main() !void {
    print("Hello, Zig!\n");
    
    // Using structs
    var p1 = Point{ .x = 3.0, .y = 4.0 };
    const p2 = Point.zero();
    const dist = p1.distance(p2);
    print("Distance: {d}\n", .{dist});
}

// Test
test "point distance calculation" {
    const p1 = Point{ .x = 0, .y = 0 };
    const p2 = Point{ .x = 3, .y = 4 };
    const expected: f32 = 5.0;
    const actual = p1.distance(p2);
    try std.testing.expectEqual(expected, actual);
}`

	_, symbols, _ := parser.ParseFile("test.zig", []byte(testCode))

	// Verify we extracted symbols
	if len(symbols) == 0 {
		t.Fatal("No symbols extracted from Zig code")
	}

	t.Logf("Extracted %d symbols", len(symbols))

	// Track found symbols
	foundSymbols := make(map[string]types.SymbolType)
	for _, symbol := range symbols {
		foundSymbols[symbol.Name] = symbol.Type
		t.Logf("Found symbol: %s (type: %s) at line %d", symbol.Name, symbol.Type, symbol.Line)
	}

	// Check that we found key symbols (based on basic query patterns)
	keySymbols := []string{"Point", "Value", "divide", "swap", "main"}

	for _, symbolName := range keySymbols {
		if _, found := foundSymbols[symbolName]; !found {
			t.Errorf("Expected to find symbol: %s", symbolName)
		}
	}

	// Verify we found different symbol types
	foundTypes := make(map[types.SymbolType]bool)
	for _, symbolType := range foundSymbols {
		foundTypes[symbolType] = true
	}

	// Should have at least struct and function types
	expectedTypes := []types.SymbolType{
		types.SymbolTypeStruct,
		types.SymbolTypeFunction,
	}

	for _, expectedType := range expectedTypes {
		if !foundTypes[expectedType] {
			t.Errorf("Expected to find symbol type: %s", expectedType)
		}
	}
}

// TestZigLanguageDetection tests the zig language detection.
func TestZigLanguageDetection(t *testing.T) {
	parser := NewTreeSitterParser()

	// Test .zig extension
	language := parser.GetLanguageFromExtension(".zig")
	if language != "zig" {
		t.Errorf("Expected language 'zig', got '%s'", language)
	}
}

// TestZigLazyLoading tests the zig lazy loading.
func TestZigLazyLoading(t *testing.T) {
	parser := NewTreeSitterParser()

	// Initially, Zig parser should not be initialized
	if parser.initialized[".zig"] {
		t.Error("Zig parser should not be initialized initially")
	}

	// Parsing a Zig file should trigger lazy initialization
	testCode := `const MyStruct = struct {
    value: i32,
    
    pub fn getValue(self: MyStruct) i32 {
        return self.value;
    }
};

pub fn main() void {
    const instance = MyStruct{ .value = 42 };
}`

	_, symbols, _ := parser.ParseFile("test.zig", []byte(testCode))

	// Should have initialized Zig parser
	if !parser.initialized[".zig"] {
		t.Error("Zig parser should be initialized after parsing")
	}

	// Should have found the struct
	if len(symbols) == 0 {
		t.Error("Should have found at least one symbol")
	}

	found := false
	for _, symbol := range symbols {
		if symbol.Name == "MyStruct" && symbol.Type == types.SymbolTypeStruct {
			found = true
			break
		}
	}

	if !found {
		t.Error("Should have found MyStruct symbol")
	}
}

// TestZigCommunityParserFramework tests the zig community parser framework.
func TestZigCommunityParserFramework(t *testing.T) {
	parser := NewTreeSitterParser()

	// Test that community parser framework is working
	testCode := `fn hello() void {
    std.debug.print("Hello from Zig!\n", .{});
}`

	_, symbols, _ := parser.ParseFile("test.zig", []byte(testCode))

	// Should have found the function
	foundFunction := false
	for _, symbol := range symbols {
		if symbol.Name == "hello" && symbol.Type == types.SymbolTypeFunction {
			foundFunction = true
			break
		}
	}

	if !foundFunction {
		t.Error("Community parser should have found hello function")
	}
}
