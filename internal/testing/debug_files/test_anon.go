//go:build debug
// +build debug

package main

func main() {
	f := func() {
		println("anonymous")
	}
	f()
}
