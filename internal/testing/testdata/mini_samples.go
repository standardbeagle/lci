package testdata

// MiniSamples provides small real-world code snippets for fast testing
// Each sample is designed to have specific trigram patterns for validation

// GoHelloWorld - 5 lines, contains common Go patterns
const GoHelloWorld = `package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}`

// GoSimpleFunction - 8 lines, function with parameters
const GoSimpleFunction = `package util

func add(a, b int) int {
    return a + b
}

func multiply(x, y int) int {
    return x * y
}`

// JavaScriptBasic - 6 lines, basic JS patterns
const JavaScriptBasic = `function greet(name) {
    return "Hello, " + name;
}

const user = { name: "test" };
console.log(greet(user.name));`

// PythonSimple - 7 lines, Python function and class
const PythonSimple = `def calculate(x, y):
    return x * y + 1

class Point:
    def __init__(self, x, y):
        self.x = x
        self.y = y`

// TypeScriptInterface - 9 lines, TS types and interface
const TypeScriptInterface = `interface User {
    name: string;
    age: number;
}

function createUser(name: string, age: number): User {
    return { name, age };
}

export { User, createUser };`

// CppHeaderSnippet - 8 lines, C++ with templates
const CppHeaderSnippet = `#include <iostream>
#include <vector>

template<typename T>
class Container {
public:
    void add(const T& item) { items.push_back(item); }
private:
    std::vector<T> items;
};`

// RustBasic - 10 lines, Rust struct and impl
const RustBasic = `struct Rectangle {
    width: u32,
    height: u32,
}

impl Rectangle {
    fn area(&self) -> u32 {
        self.width * self.height
    }
}`

// TestFile represents a minimal test file with known content
type TestFile struct {
	Name             string
	Content          string
	Language         string
	ExpectedSymbols  int
	ExpectedTrigrams int
}

// GetMiniSamples returns all test files for iteration
func GetMiniSamples() []TestFile {
	return []TestFile{
		{
			Name:             "hello.go",
			Content:          GoHelloWorld,
			Language:         "go",
			ExpectedSymbols:  1,  // main function
			ExpectedTrigrams: 15, // approximate count
		},
		{
			Name:             "util.go",
			Content:          GoSimpleFunction,
			Language:         "go",
			ExpectedSymbols:  2, // add and multiply functions
			ExpectedTrigrams: 20,
		},
		{
			Name:             "greet.js",
			Content:          JavaScriptBasic,
			Language:         "javascript",
			ExpectedSymbols:  1, // greet function
			ExpectedTrigrams: 25,
		},
		{
			Name:             "calc.py",
			Content:          PythonSimple,
			Language:         "python",
			ExpectedSymbols:  3, // calculate function, Point class, __init__
			ExpectedTrigrams: 30,
		},
		{
			Name:             "user.ts",
			Content:          TypeScriptInterface,
			Language:         "typescript",
			ExpectedSymbols:  2, // User interface, createUser function
			ExpectedTrigrams: 35,
		},
		{
			Name:             "container.hpp",
			Content:          CppHeaderSnippet,
			Language:         "cpp",
			ExpectedSymbols:  2, // Container class, add method
			ExpectedTrigrams: 40,
		},
		{
			Name:             "rect.rs",
			Content:          RustBasic,
			Language:         "rust",
			ExpectedSymbols:  2, // Rectangle struct, area method
			ExpectedTrigrams: 25,
		},
	}
}

// GetTrigramTestSamples returns files specifically designed for trigram testing
func GetTrigramTestSamples() []TestFile {
	return []TestFile{
		{
			Name:             "abc.txt",
			Content:          "abc def ghi",
			Language:         "text",
			ExpectedTrigrams: 3, // "abc", "def", "ghi"
		},
		{
			Name:             "repeated.txt",
			Content:          "abcabc abcabc",
			Language:         "text",
			ExpectedTrigrams: 4, // "abc", "bca", "cab", "aca"
		},
		{
			Name:             "unicode.txt",
			Content:          "café naïve résumé",
			Language:         "text",
			ExpectedTrigrams: 6, // Unicode trigrams
		},
		{
			Name:             "symbols.txt",
			Content:          "func() { return x + y; }",
			Language:         "text",
			ExpectedTrigrams: 8, // Mixed alphanumeric and symbols
		},
	}
}
