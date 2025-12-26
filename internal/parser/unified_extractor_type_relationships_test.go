package parser

import (
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

// TestUnifiedExtractor_TypeRelationships_Go tests Go interface/struct embedding
func TestUnifiedExtractor_TypeRelationships_Go(t *testing.T) {
	parser := NewTreeSitterParser()

	tests := []struct {
		name     string
		code     string
		expected []struct {
			refType types.ReferenceType
			target  string // ReferencedName
		}
	}{
		{
			name: "interface embedding",
			code: `package main

type Reader interface {
	Read(p []byte) (n int, err error)
}

type Writer interface {
	Write(p []byte) (n int, err error)
}

type ReadWriter interface {
	Reader
	Writer
}
`,
			expected: []struct {
				refType types.ReferenceType
				target  string
			}{
				{types.RefTypeExtends, "Reader"},
				{types.RefTypeExtends, "Writer"},
			},
		},
		{
			name: "struct embedding",
			code: `package main

type Base struct {
	name string
}

type Derived struct {
	Base
	age int
}
`,
			expected: []struct {
				refType types.ReferenceType
				target  string
			}{
				{types.RefTypeExtends, "Base"},
			},
		},
		{
			name: "multiple struct embeddings",
			code: `package main

type Logger struct{}
type Metrics struct{}

type Service struct {
	Logger
	Metrics
	name string
}
`,
			expected: []struct {
				refType types.ReferenceType
				target  string
			}{
				{types.RefTypeExtends, "Logger"},
				{types.RefTypeExtends, "Metrics"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use ParseFileEnhanced which initializes parsers and calls unified extractor
			_, _, _, _, refs, _ := parser.ParseFileEnhanced("test.go", []byte(tt.code))

			// Filter for extends/implements refs only
			var typeRefs []types.Reference
			for _, ref := range refs {
				if ref.Type == types.RefTypeExtends || ref.Type == types.RefTypeImplements {
					typeRefs = append(typeRefs, ref)
				}
			}

			if len(typeRefs) != len(tt.expected) {
				t.Errorf("expected %d type relationship refs, got %d", len(tt.expected), len(typeRefs))
				for _, ref := range typeRefs {
					t.Logf("  found: type=%v target=%s", ref.Type, ref.ReferencedName)
				}
				return
			}

			for i, exp := range tt.expected {
				if typeRefs[i].Type != exp.refType {
					t.Errorf("ref[%d]: expected type %v, got %v", i, exp.refType, typeRefs[i].Type)
				}
				if typeRefs[i].ReferencedName != exp.target {
					t.Errorf("ref[%d]: expected target %q, got %q", i, exp.target, typeRefs[i].ReferencedName)
				}
			}
		})
	}
}

// TestUnifiedExtractor_TypeRelationships_TypeScript tests TypeScript extends/implements
func TestUnifiedExtractor_TypeRelationships_TypeScript(t *testing.T) {
	parser := NewTreeSitterParser()

	tests := []struct {
		name     string
		code     string
		expected []struct {
			refType types.ReferenceType
			target  string
		}
	}{
		{
			name: "class extends",
			code: `class Child extends Parent {
	constructor() {
		super();
	}
}
`,
			expected: []struct {
				refType types.ReferenceType
				target  string
			}{
				{types.RefTypeExtends, "Parent"},
			},
		},
		{
			name: "class implements single interface",
			code: `class Service implements IService {
	doWork(): void {}
}
`,
			expected: []struct {
				refType types.ReferenceType
				target  string
			}{
				{types.RefTypeImplements, "IService"},
			},
		},
		{
			name: "class extends and implements multiple",
			code: `class MyClass extends BaseClass implements Interface1, Interface2 {
	method() {}
}
`,
			expected: []struct {
				refType types.ReferenceType
				target  string
			}{
				{types.RefTypeExtends, "BaseClass"},
				{types.RefTypeImplements, "Interface1"},
				{types.RefTypeImplements, "Interface2"},
			},
		},
		{
			name: "interface extends",
			code: `interface ExtendedInterface extends BaseInterface {
	additionalMethod(): void;
}
`,
			expected: []struct {
				refType types.ReferenceType
				target  string
			}{
				{types.RefTypeExtends, "BaseInterface"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, _, refs, _ := parser.ParseFileEnhanced("test.ts", []byte(tt.code))

			// Filter for extends/implements refs only
			var typeRefs []types.Reference
			for _, ref := range refs {
				if ref.Type == types.RefTypeExtends || ref.Type == types.RefTypeImplements {
					typeRefs = append(typeRefs, ref)
				}
			}

			if len(typeRefs) != len(tt.expected) {
				t.Errorf("expected %d type relationship refs, got %d", len(tt.expected), len(typeRefs))
				for _, ref := range typeRefs {
					t.Logf("  found: type=%v target=%s", ref.Type, ref.ReferencedName)
				}
				return
			}

			for i, exp := range tt.expected {
				if typeRefs[i].Type != exp.refType {
					t.Errorf("ref[%d]: expected type %v, got %v", i, exp.refType, typeRefs[i].Type)
				}
				if typeRefs[i].ReferencedName != exp.target {
					t.Errorf("ref[%d]: expected target %q, got %q", i, exp.target, typeRefs[i].ReferencedName)
				}
			}
		})
	}
}

// TestUnifiedExtractor_TypeRelationships_Python tests Python class inheritance
func TestUnifiedExtractor_TypeRelationships_Python(t *testing.T) {
	parser := NewTreeSitterParser()

	tests := []struct {
		name     string
		code     string
		expected []struct {
			refType types.ReferenceType
			target  string
		}
	}{
		{
			name: "single inheritance",
			code: `class Child(Parent):
    def __init__(self):
        pass
`,
			expected: []struct {
				refType types.ReferenceType
				target  string
			}{
				{types.RefTypeExtends, "Parent"},
			},
		},
		{
			name: "multiple inheritance",
			code: `class Child(Parent1, Parent2, Mixin):
    pass
`,
			expected: []struct {
				refType types.ReferenceType
				target  string
			}{
				{types.RefTypeExtends, "Parent1"},
				{types.RefTypeExtends, "Parent2"},
				{types.RefTypeExtends, "Mixin"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, _, refs, _ := parser.ParseFileEnhanced("test.py", []byte(tt.code))

			// Filter for extends/implements refs only
			var typeRefs []types.Reference
			for _, ref := range refs {
				if ref.Type == types.RefTypeExtends || ref.Type == types.RefTypeImplements {
					typeRefs = append(typeRefs, ref)
				}
			}

			if len(typeRefs) != len(tt.expected) {
				t.Errorf("expected %d type relationship refs, got %d", len(tt.expected), len(typeRefs))
				for _, ref := range typeRefs {
					t.Logf("  found: type=%v target=%s", ref.Type, ref.ReferencedName)
				}
				return
			}

			for i, exp := range tt.expected {
				if typeRefs[i].Type != exp.refType {
					t.Errorf("ref[%d]: expected type %v, got %v", i, exp.refType, typeRefs[i].Type)
				}
				if typeRefs[i].ReferencedName != exp.target {
					t.Errorf("ref[%d]: expected target %q, got %q", i, exp.target, typeRefs[i].ReferencedName)
				}
			}
		})
	}
}

// TestUnifiedExtractor_TypeRelationships_Rust tests Rust trait implementations
func TestUnifiedExtractor_TypeRelationships_Rust(t *testing.T) {
	parser := NewTreeSitterParser()

	tests := []struct {
		name     string
		code     string
		expected []struct {
			refType types.ReferenceType
			target  string
		}
	}{
		{
			name: "impl trait for type",
			code: `impl Display for MyStruct {
    fn display(&self) -> String {
        String::new()
    }
}
`,
			expected: []struct {
				refType types.ReferenceType
				target  string
			}{
				{types.RefTypeImplements, "Display"},
			},
		},
		{
			name: "inherent impl (no trait)",
			code: `impl MyStruct {
    fn new() -> Self {
        MyStruct {}
    }
}
`,
			expected: []struct {
				refType types.ReferenceType
				target  string
			}{
				// No type relationships for inherent impls
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, _, refs, _ := parser.ParseFileEnhanced("test.rs", []byte(tt.code))

			// Filter for extends/implements refs only
			var typeRefs []types.Reference
			for _, ref := range refs {
				if ref.Type == types.RefTypeExtends || ref.Type == types.RefTypeImplements {
					typeRefs = append(typeRefs, ref)
				}
			}

			if len(typeRefs) != len(tt.expected) {
				t.Errorf("expected %d type relationship refs, got %d", len(tt.expected), len(typeRefs))
				for _, ref := range typeRefs {
					t.Logf("  found: type=%v target=%s", ref.Type, ref.ReferencedName)
				}
				return
			}

			for i, exp := range tt.expected {
				if typeRefs[i].Type != exp.refType {
					t.Errorf("ref[%d]: expected type %v, got %v", i, exp.refType, typeRefs[i].Type)
				}
				if typeRefs[i].ReferencedName != exp.target {
					t.Errorf("ref[%d]: expected target %q, got %q", i, exp.target, typeRefs[i].ReferencedName)
				}
			}
		})
	}
}

// TestUnifiedExtractor_TypeRelationships_Java tests Java extends/implements
func TestUnifiedExtractor_TypeRelationships_Java(t *testing.T) {
	parser := NewTreeSitterParser()

	tests := []struct {
		name     string
		code     string
		expected []struct {
			refType types.ReferenceType
			target  string
		}
	}{
		{
			name: "class extends",
			code: `class Child extends Parent {
    public void method() {}
}
`,
			expected: []struct {
				refType types.ReferenceType
				target  string
			}{
				{types.RefTypeExtends, "Parent"},
			},
		},
		{
			name: "class implements single interface",
			code: `class Service implements Runnable {
    public void run() {}
}
`,
			expected: []struct {
				refType types.ReferenceType
				target  string
			}{
				{types.RefTypeImplements, "Runnable"},
			},
		},
		{
			name: "class extends and implements multiple",
			code: `class MyClass extends BaseClass implements Interface1, Interface2 {
    public void method() {}
}
`,
			expected: []struct {
				refType types.ReferenceType
				target  string
			}{
				{types.RefTypeExtends, "BaseClass"},
				{types.RefTypeImplements, "Interface1"},
				{types.RefTypeImplements, "Interface2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, _, refs, _ := parser.ParseFileEnhanced("Test.java", []byte(tt.code))

			// Filter for extends/implements refs only
			var typeRefs []types.Reference
			for _, ref := range refs {
				if ref.Type == types.RefTypeExtends || ref.Type == types.RefTypeImplements {
					typeRefs = append(typeRefs, ref)
				}
			}

			if len(typeRefs) != len(tt.expected) {
				t.Errorf("expected %d type relationship refs, got %d", len(tt.expected), len(typeRefs))
				for _, ref := range typeRefs {
					t.Logf("  found: type=%v target=%s", ref.Type, ref.ReferencedName)
				}
				return
			}

			for i, exp := range tt.expected {
				if typeRefs[i].Type != exp.refType {
					t.Errorf("ref[%d]: expected type %v, got %v", i, exp.refType, typeRefs[i].Type)
				}
				if typeRefs[i].ReferencedName != exp.target {
					t.Errorf("ref[%d]: expected target %q, got %q", i, exp.target, typeRefs[i].ReferencedName)
				}
			}
		})
	}
}

// TestUnifiedExtractor_TypeRelationships_CSharp tests C# base list
func TestUnifiedExtractor_TypeRelationships_CSharp(t *testing.T) {
	parser := NewTreeSitterParser()

	tests := []struct {
		name     string
		code     string
		expected []struct {
			refType types.ReferenceType
			target  string
		}
	}{
		{
			name: "class with single base",
			code: `class Child : Parent {
    public void Method() {}
}
`,
			expected: []struct {
				refType types.ReferenceType
				target  string
			}{
				{types.RefTypeExtends, "Parent"},
			},
		},
		{
			name: "class with base and interfaces",
			code: `class MyClass : BaseClass, Interface1, Interface2 {
    public void Method() {}
}
`,
			expected: []struct {
				refType types.ReferenceType
				target  string
			}{
				{types.RefTypeExtends, "BaseClass"},
				{types.RefTypeImplements, "Interface1"},
				{types.RefTypeImplements, "Interface2"},
			},
		},
		{
			name: "interface extending interface",
			code: `interface IExtended : IBase {
    void Method();
}
`,
			expected: []struct {
				refType types.ReferenceType
				target  string
			}{
				{types.RefTypeExtends, "IBase"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, _, refs, _ := parser.ParseFileEnhanced("Test.cs", []byte(tt.code))

			// Filter for extends/implements refs only
			var typeRefs []types.Reference
			for _, ref := range refs {
				if ref.Type == types.RefTypeExtends || ref.Type == types.RefTypeImplements {
					typeRefs = append(typeRefs, ref)
				}
			}

			if len(typeRefs) != len(tt.expected) {
				t.Errorf("expected %d type relationship refs, got %d", len(tt.expected), len(typeRefs))
				for _, ref := range typeRefs {
					t.Logf("  found: type=%v target=%s", ref.Type, ref.ReferencedName)
				}
				return
			}

			for i, exp := range tt.expected {
				if typeRefs[i].Type != exp.refType {
					t.Errorf("ref[%d]: expected type %v, got %v", i, exp.refType, typeRefs[i].Type)
				}
				if typeRefs[i].ReferencedName != exp.target {
					t.Errorf("ref[%d]: expected target %q, got %q", i, exp.target, typeRefs[i].ReferencedName)
				}
			}
		})
	}
}

// TestUnifiedExtractor_TypeRelationships_NoRelationships tests files without type relationships
func TestUnifiedExtractor_TypeRelationships_NoRelationships(t *testing.T) {
	parser := NewTreeSitterParser()

	tests := []struct {
		name string
		ext  string
		code string
	}{
		{
			name: "go simple function",
			ext:  ".go",
			code: `package main

func main() {
	fmt.Println("Hello")
}
`,
		},
		{
			name: "ts simple function",
			ext:  ".ts",
			code: `function greet(name: string): void {
	console.log("Hello " + name);
}
`,
		},
		{
			name: "python simple function",
			ext:  ".py",
			code: `def greet(name):
    print(f"Hello {name}")
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, _, refs, _ := parser.ParseFileEnhanced("test"+tt.ext, []byte(tt.code))

			// Filter for extends/implements refs only
			var typeRefs []types.Reference
			for _, ref := range refs {
				if ref.Type == types.RefTypeExtends || ref.Type == types.RefTypeImplements {
					typeRefs = append(typeRefs, ref)
				}
			}

			if len(typeRefs) != 0 {
				t.Errorf("expected 0 type relationship refs, got %d", len(typeRefs))
				for _, ref := range typeRefs {
					t.Logf("  found: type=%v target=%s", ref.Type, ref.ReferencedName)
				}
			}
		})
	}
}

// TestUnifiedExtractor_GoInterfaceUsage tests Go interface usage detection
// (assignment to interface-typed variables, return as interface, type assertion)
func TestUnifiedExtractor_GoInterfaceUsage(t *testing.T) {
	parser := NewTreeSitterParser()

	tests := []struct {
		name     string
		code     string
		expected []struct {
			refType types.ReferenceType
			target  string // Interface name (ReferencedName)
			quality string // Expected quality level
		}
	}{
		{
			name: "var assignment to interface",
			code: `package main

type Writer interface {
	Write([]byte) (int, error)
}

type File struct{}

func test() {
	var w Writer = &File{}
	_ = w
}
`,
			expected: []struct {
				refType types.ReferenceType
				target  string
				quality string
			}{
				{types.RefTypeImplements, "Writer", types.RefQualityAssigned},
			},
		},
		{
			name: "return as interface",
			code: `package main

type Writer interface {
	Write([]byte) (int, error)
}

type File struct{}

func NewWriter() Writer {
	return &File{}
}
`,
			expected: []struct {
				refType types.ReferenceType
				target  string
				quality string
			}{
				{types.RefTypeImplements, "Writer", types.RefQualityReturned},
			},
		},
		{
			name: "type assertion",
			code: `package main

type Writer interface {
	Write([]byte) (int, error)
}

func test(x interface{}) {
	_ = x.(Writer)
}
`,
			expected: []struct {
				refType types.ReferenceType
				target  string
				quality string
			}{
				{types.RefTypeImplements, "Writer", types.RefQualityCast},
			},
		},
		{
			name: "multiple patterns combined",
			code: `package main

type Reader interface {
	Read([]byte) (int, error)
}

type Writer interface {
	Write([]byte) (int, error)
}

type File struct{}

func NewReader() Reader {
	return &File{}
}

func test(x interface{}) {
	var w Writer = &File{}
	_ = w
	_ = x.(Reader)
}
`,
			expected: []struct {
				refType types.ReferenceType
				target  string
				quality string
			}{
				{types.RefTypeImplements, "Reader", types.RefQualityReturned}, // return in NewReader
				{types.RefTypeImplements, "Writer", types.RefQualityAssigned}, // var assignment
				{types.RefTypeImplements, "Reader", types.RefQualityCast},     // type assertion
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, _, refs, _ := parser.ParseFileEnhanced("test.go", []byte(tt.code))

			// Filter for implements refs with quality markers
			var implRefs []types.Reference
			for _, ref := range refs {
				if ref.Type == types.RefTypeImplements && ref.Quality != "" {
					implRefs = append(implRefs, ref)
				}
			}

			if len(implRefs) != len(tt.expected) {
				t.Errorf("expected %d interface usage refs, got %d", len(tt.expected), len(implRefs))
				for _, ref := range implRefs {
					t.Logf("  found: type=%v target=%s quality=%s", ref.Type, ref.ReferencedName, ref.Quality)
				}
				return
			}

			for i, exp := range tt.expected {
				if implRefs[i].Type != exp.refType {
					t.Errorf("ref[%d]: expected type %v, got %v", i, exp.refType, implRefs[i].Type)
				}
				if implRefs[i].ReferencedName != exp.target {
					t.Errorf("ref[%d]: expected target %q, got %q", i, exp.target, implRefs[i].ReferencedName)
				}
				if implRefs[i].Quality != exp.quality {
					t.Errorf("ref[%d]: expected quality %q, got %q", i, exp.quality, implRefs[i].Quality)
				}
			}
		})
	}
}

// TestRefQualityRank tests the quality ranking function
func TestRefQualityRank(t *testing.T) {
	tests := []struct {
		quality  string
		expected int
	}{
		{types.RefQualityPrecise, 100},
		{types.RefQualityAssigned, 95},
		{types.RefQualityReturned, 90},
		{types.RefQualityCast, 85},
		{types.RefQualityHeuristic, 50},
		{"unknown", 0},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.quality, func(t *testing.T) {
			got := types.RefQualityRank(tt.quality)
			if got != tt.expected {
				t.Errorf("RefQualityRank(%q) = %d, want %d", tt.quality, got, tt.expected)
			}
		})
	}

	// Test ordering: precise > assigned > returned > cast > heuristic
	if types.RefQualityRank(types.RefQualityPrecise) <= types.RefQualityRank(types.RefQualityAssigned) {
		t.Error("precise should rank higher than assigned")
	}
	if types.RefQualityRank(types.RefQualityAssigned) <= types.RefQualityRank(types.RefQualityReturned) {
		t.Error("assigned should rank higher than returned")
	}
	if types.RefQualityRank(types.RefQualityReturned) <= types.RefQualityRank(types.RefQualityCast) {
		t.Error("returned should rank higher than cast")
	}
	if types.RefQualityRank(types.RefQualityCast) <= types.RefQualityRank(types.RefQualityHeuristic) {
		t.Error("cast should rank higher than heuristic")
	}
}

// TestUnifiedExtractor_TypeRelationships_PHP tests PHP extends/implements
func TestUnifiedExtractor_TypeRelationships_PHP(t *testing.T) {
	parser := NewTreeSitterParser()

	tests := []struct {
		name     string
		code     string
		expected []struct {
			refType types.ReferenceType
			target  string // ReferencedName
		}
	}{
		{
			name: "class extends",
			code: `<?php
class Child extends Parent {
    public function foo() {}
}
`,
			expected: []struct {
				refType types.ReferenceType
				target  string
			}{
				{types.RefTypeExtends, "Parent"},
			},
		},
		{
			name: "class implements single interface",
			code: `<?php
class Service implements ServiceInterface {
    public function handle() {}
}
`,
			expected: []struct {
				refType types.ReferenceType
				target  string
			}{
				{types.RefTypeImplements, "ServiceInterface"},
			},
		},
		{
			name: "class extends and implements multiple",
			code: `<?php
class MyClass extends BaseClass implements Interface1, Interface2 {
    public function foo() {}
}
`,
			expected: []struct {
				refType types.ReferenceType
				target  string
			}{
				{types.RefTypeExtends, "BaseClass"},
				{types.RefTypeImplements, "Interface1"},
				{types.RefTypeImplements, "Interface2"},
			},
		},
		{
			name: "interface extends",
			code: `<?php
interface ExtendedInterface extends BaseInterface {
    public function additionalMethod();
}
`,
			expected: []struct {
				refType types.ReferenceType
				target  string
			}{
				{types.RefTypeExtends, "BaseInterface"},
			},
		},
		{
			name: "interface extends multiple",
			code: `<?php
interface CombinedInterface extends Interface1, Interface2 {
    public function combined();
}
`,
			expected: []struct {
				refType types.ReferenceType
				target  string
			}{
				{types.RefTypeExtends, "Interface1"},
				{types.RefTypeExtends, "Interface2"},
			},
		},
		{
			name: "class with namespace",
			code: `<?php
namespace App\Models;

class User extends \App\Models\BaseModel implements \App\Contracts\UserInterface {
    private string $name;
}
`,
			expected: []struct {
				refType types.ReferenceType
				target  string
			}{
				{types.RefTypeExtends, "\\App\\Models\\BaseModel"},
				{types.RefTypeImplements, "\\App\\Contracts\\UserInterface"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use ParseFileEnhanced which initializes parsers and calls unified extractor
			_, _, _, _, refs, _ := parser.ParseFileEnhanced("test.php", []byte(tt.code))

			// Filter for extends/implements refs only
			var typeRefs []types.Reference
			for _, ref := range refs {
				if ref.Type == types.RefTypeExtends || ref.Type == types.RefTypeImplements {
					typeRefs = append(typeRefs, ref)
				}
			}

			if len(typeRefs) != len(tt.expected) {
				t.Errorf("got %d type refs, want %d", len(typeRefs), len(tt.expected))
				for i, ref := range typeRefs {
					t.Logf("  ref[%d]: type=%v, target=%q", i, ref.Type, ref.ReferencedName)
				}
				return
			}

			for i, exp := range tt.expected {
				if typeRefs[i].Type != exp.refType {
					t.Errorf("ref[%d].Type = %v, want %v", i, typeRefs[i].Type, exp.refType)
				}
				if typeRefs[i].ReferencedName != exp.target {
					t.Errorf("ref[%d].ReferencedName = %q, want %q", i, typeRefs[i].ReferencedName, exp.target)
				}
			}
		})
	}
}
