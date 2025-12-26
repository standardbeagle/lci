package fixtures

// CodeSample represents a real code sample for testing
type CodeSample struct {
	Language          string
	Description       string
	Filename          string
	Content           string
	ExpectedSymbols   []string
	ExpectedFunctions []string
	ExpectedClasses   []string
	ExpectedImports   []string
	Valid             bool
}

// GenerateCodeSamples returns a collection of real code samples for testing
func GenerateCodeSamples() []CodeSample {
	return []CodeSample{
		{
			Language:    "JavaScript",
			Description: "ReactComponent",
			Filename:    "Calculator.js",
			Content: `import React, { useState } from 'react';

export default function Calculator() {
    const [result, setResult] = useState(0);
    
    const add = (a, b) => a + b;
    const subtract = (a, b) => a - b;
    
    return (
        <div>
            <h1>Calculator</h1>
            <p>Result: {result}</p>
        </div>
    );
}`,
			ExpectedSymbols:   []string{"Calculator", "add", "subtract", "useState", "setResult"},
			ExpectedFunctions: []string{"Calculator", "add", "subtract"},
			ExpectedClasses:   []string{},
			ExpectedImports:   []string{"React", "useState"},
			Valid:             true,
		},
		{
			Language:    "Rust",
			Description: "SimpleStruct",
			Filename:    "calculator.rs",
			Content: `#[derive(Debug, Clone)]
pub struct Calculator {
    result: f64,
}

impl Calculator {
    pub fn new() -> Self {
        Calculator { result: 0.0 }
    }
    
    pub fn add(&mut self, a: f64, b: f64) -> f64 {
        self.result = a + b;
        self.result
    }
    
    pub fn subtract(&mut self, a: f64, b: f64) -> f64 {
        self.result = a - b;
        self.result
    }
}`,
			ExpectedSymbols:   []string{"Calculator", "new", "add", "subtract"},
			ExpectedFunctions: []string{"new", "add", "subtract"},
			ExpectedClasses:   []string{"Calculator"},
			ExpectedImports:   []string{},
			Valid:             true,
		},
		{
			Language:    "C++",
			Description: "ClassWithMethods",
			Filename:    "calculator.cpp",
			Content: `#include <iostream>

class Calculator {
private:
    double result;
    
public:
    Calculator() : result(0.0) {}
    
    double add(double a, double b) {
        result = a + b;
        return result;
    }
    
    double subtract(double a, double b) {
        result = a - b;
        return result;
    }
    
    double getResult() const {
        return result;
    }
};

int main() {
    Calculator calc;
    std::cout << calc.add(5, 3) << std::endl;
    return 0;
}`,
			ExpectedSymbols:   []string{"Calculator", "add", "subtract", "getResult", "main"},
			ExpectedFunctions: []string{"add", "subtract", "getResult", "main"},
			ExpectedClasses:   []string{"Calculator"},
			ExpectedImports:   []string{"iostream"},
			Valid:             true,
		},
	}
}
