package parser

import (
	"runtime"
	"testing"
)

// BenchmarkPhase4AllLanguages benchmarks parsing performance across all 7 languages
func BenchmarkPhase4AllLanguages(b *testing.B) {
	parser := NewTreeSitterParser()

	testCases := []struct {
		name     string
		filename string
		content  string
	}{
		{
			"JavaScript",
			"test.js",
			`function hello() { return "world"; }
class Test { method() { return 42; } }
export default Test;
import { utils } from './utils';`,
		},
		{
			"TypeScript",
			"test.ts",
			`function hello(): string { return "world"; }
class Test { method(): number { return 42; } }
interface ITest { prop: string; }
type MyType = string | number;`,
		},
		{
			"Go",
			"test.go",
			`package main
func main() { fmt.Println("Hello") }
type Person struct { Name string }
func (p Person) String() string { return p.Name }`,
		},
		{
			"Python",
			"test.py",
			`def hello():
    print("world")

class Test:
    def method(self):
        return 42
        
import os
from typing import List`,
		},
		{
			"Rust",
			"test.rs",
			`fn main() { println!("Hello"); }
struct Point { x: i32, y: i32 }
impl Point { fn new() -> Point { Point { x: 0, y: 0 } } }
trait Display { fn fmt(&self); }
mod utils { pub fn helper() {} }`,
		},
		{
			"C++",
			"test.cpp",
			`int add(int a, int b) { return a + b; }
class Calculator { 
public: 
    int multiply(int x, int y); 
    int divide(int x, int y);
};
namespace math { int subtract(int a, int b); }
#include <iostream>`,
		},
		{
			"Java",
			"Test.java",
			`public class Test {
    public void method1() { System.out.println("Hello"); }
    private int method2() { return 42; }
    public Test() { /* constructor */ }
}
package com.example;
import java.util.List;`,
		},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			// Measure memory allocations
			var m1, m2 runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&m1)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				blocks, symbols, imports := parser.ParseFile(tc.filename, []byte(tc.content))
				// Prevent compiler optimization
				_ = blocks
				_ = symbols
				_ = imports
			}
			b.StopTimer()

			runtime.ReadMemStats(&m2)

			// Log performance metrics
			avgAllocBytes := (m2.TotalAlloc - m1.TotalAlloc) / uint64(b.N)
			b.ReportMetric(float64(avgAllocBytes), "alloc-bytes/op")

			// Calculate operations per second
			opsPerSec := float64(b.N) / b.Elapsed().Seconds()
			b.ReportMetric(opsPerSec, "ops/sec")
		})
	}
}

// BenchmarkPhase4LargeFile benchmarks parsing performance on larger files
func BenchmarkPhase4LargeFile(b *testing.B) {
	parser := NewTreeSitterParser()

	// Create a larger Rust file with multiple constructs
	largeRustCode := `
// Large Rust file for performance testing
use std::collections::HashMap;
use std::fmt::Display;

fn main() {
    let mut map = HashMap::new();
    map.insert("hello", "world");
    println!("Hello, world!");
}

pub struct User {
    id: u64,
    name: String,
    email: String,
    active: bool,
}

impl User {
    pub fn new(id: u64, name: String, email: String) -> Self {
        User {
            id,
            name,
            email,
            active: true,
        }
    }

    pub fn deactivate(&mut self) {
        self.active = false;
    }

    pub fn get_display_name(&self) -> &str {
        &self.name
    }
}

impl Display for User {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "User({}): {}", self.id, self.name)
    }
}

pub trait UserRepository {
    fn find_by_id(&self, id: u64) -> Option<User>;
    fn save(&mut self, user: User) -> Result<(), String>;
    fn delete(&mut self, id: u64) -> Result<(), String>;
}

pub mod database {
    use super::*;
    
    pub struct InMemoryUserRepository {
        users: HashMap<u64, User>,
    }
    
    impl InMemoryUserRepository {
        pub fn new() -> Self {
            Self {
                users: HashMap::new(),
            }
        }
    }
    
    impl UserRepository for InMemoryUserRepository {
        fn find_by_id(&self, id: u64) -> Option<User> {
            self.users.get(&id).cloned()
        }
        
        fn save(&mut self, user: User) -> Result<(), String> {
            self.users.insert(user.id, user);
            Ok(())
        }
        
        fn delete(&mut self, id: u64) -> Result<(), String> {
            self.users.remove(&id);
            Ok(())
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_user_creation() {
        let user = User::new(1, "John".to_string(), "john@example.com".to_string());
        assert_eq!(user.id, 1);
        assert_eq!(user.name, "John");
    }
    
    #[test]
    fn test_user_deactivation() {
        let mut user = User::new(1, "John".to_string(), "john@example.com".to_string());
        user.deactivate();
        assert!(!user.active);
    }
}
`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		blocks, symbols, imports := parser.ParseFile("large_test.rs", []byte(largeRustCode))
		// Prevent compiler optimization
		_ = blocks
		_ = symbols
		_ = imports
	}
}

// BenchmarkPhase4Memory benchmarks memory usage across languages
func BenchmarkPhase4Memory(b *testing.B) {
	parser := NewTreeSitterParser()

	codeSnippets := map[string]string{
		"rust":   `fn hello() { println!("world"); } struct Point { x: i32 }`,
		"cpp":    `int add(int a, int b) { return a + b; } class Test {};`,
		"java":   `public class Test { public void method() {} }`,
		"js":     `function hello() { return "world"; } class Test {}`,
		"go":     `func main() {} type Person struct { Name string }`,
		"python": `def hello(): pass\nclass Test: pass`,
	}

	for lang, code := range codeSnippets {
		b.Run(lang+"_memory", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				filename := "test." + lang
				if lang == "cpp" {
					filename = "test.cpp"
				} else if lang == "js" {
					filename = "test.js"
				} else if lang == "python" {
					filename = "test.py"
				}
				blocks, symbols, imports := parser.ParseFile(filename, []byte(code))
				// Prevent optimization
				_ = blocks
				_ = symbols
				_ = imports
			}
		})
	}
}
