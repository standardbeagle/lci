package main

import (
	"fmt"
	"github.com/standardbeagle/lci/internal/types"
	"unsafe"
)

func main() {
	s := types.Symbol{}
	e := types.EnhancedSymbol{}
	fmt.Printf("Symbol size: %d bytes\n", unsafe.Sizeof(s))
	fmt.Printf("EnhancedSymbol size: %d bytes\n", unsafe.Sizeof(e))
	fmt.Printf("\nIf you have 10,000 symbols:\n")
	fmt.Printf("  Separate []Symbol: %.2f KB\n", float64(10000*int(unsafe.Sizeof(s)))/1024)
	fmt.Printf("  Already in []*EnhancedSymbol: (embedded, no extra cost)\n")
	fmt.Printf("  WASTED by duplicating: %.2f KB\n", float64(10000*int(unsafe.Sizeof(s)))/1024)
}
