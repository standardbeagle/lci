package testpkg

import (
	_ "blank/import"
	. "dot/import"
	"fmt"
	alias "path/to/package"
	"strings"
)

// PublicConstant is an exported constant
const PublicConstant = "value"

// privateConstant is not exported
const privateConstant = 42

// Multiple constants in one declaration
const (
	First  = 1
	Second = 2
	third  = 3
)

// PublicVar is an exported variable
var PublicVar string

// privateVar is not exported
var privateVar int

// Multiple variables
var (
	GlobalOne   = "one"
	globalTwo   = "two"
	GlobalThree string
)

// PublicStruct is an exported struct
type PublicStruct struct {
	PublicField  string
	privateField int
	EmbeddedType
}

// privateStruct is not exported
type privateStruct struct {
	field1 string
	field2 int
}

// PublicInterface is an exported interface
type PublicInterface interface {
	PublicMethod() string
	privateMethod() error
	DoSomething(param string) (result int, err error)
}

// TypeAlias is a type alias
type TypeAlias = string

// CustomType is a custom type
type CustomType int

// PublicFunction is an exported function
func PublicFunction(param1 string, param2 int) (string, error) {
	// Local variable
	localVar := "local"

	// Short variable declaration
	x, y := 10, 20

	// Inside block scope
	if param2 > 0 {
		blockVar := "in block"
		fmt.Println(blockVar)
	}

	// For loop with scope
	for i := 0; i < param2; i++ {
		loopVar := i * 2
		fmt.Println(loopVar)
	}

	// Goroutine
	go fmt.Println("async")

	// Anonymous goroutine
	go func() {
		fmt.Println("anonymous")
	}()

	return localVar + param1, nil
}

// privateFunction is not exported
func privateFunction() {
	defer fmt.Println("deferred")
}

// Method on PublicStruct (exported because receiver is exported)
func (ps PublicStruct) PublicMethod() string {
	return ps.PublicField
}

// Method on PublicStruct (not exported due to lowercase name)
func (ps PublicStruct) privateMethod() error {
	return nil
}

// Pointer receiver method
func (ps *PublicStruct) SetField(value string) {
	ps.PublicField = value
}

// Method on private struct (not exported)
func (p privateStruct) method() {
	// method body
}

// Variadic function
func VariadicFunc(prefix string, values ...int) {
	for _, v := range values {
		fmt.Printf("%s: %d\n", prefix, v)
	}
}

// Generic function (Go 1.18+)
func GenericFunc[T any](value T) T {
	return value
}

// Generic type
type GenericType[T any] struct {
	Value T
}

// init function (special case)
func init() {
	fmt.Println("package initialized")
}
