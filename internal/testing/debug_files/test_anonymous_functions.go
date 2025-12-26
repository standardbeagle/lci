//go:build debug
// +build debug

package main

import "fmt"

func main() {
	// Test 1: Simple anonymous function
	f1 := func() {
		fmt.Println("inside anonymous function 1")
	}
	f1()

	// Test 2: Anonymous function with parameters
	f2 := func(x int) int {
		// This is a comment inside anonymous function 2
		return x * 2
	}
	fmt.Println(f2(5))

	// Test 3: Anonymous function as goroutine
	go func() {
		fmt.Println("inside anonymous goroutine")
	}()

	// Test 4: Anonymous function in slice
	funcs := []func(){
		func() { fmt.Println("first anonymous in slice") },
		func() { fmt.Println("second anonymous in slice") },
	}
	for _, fn := range funcs {
		fn()
	}

	// Test 5: Nested anonymous functions
	outer := func() {
		inner := func() {
			fmt.Println("inside nested anonymous function")
		}
		inner()
	}
	outer()
}

// Test 6: Anonymous function as method receiver
type Handler struct{}

func (h Handler) Process(fn func(string)) {
	fn("processing")
}

func TestMethodWithAnonymous() {
	h := Handler{}
	h.Process(func(msg string) {
		fmt.Println("anonymous handler:", msg)
	})
}
