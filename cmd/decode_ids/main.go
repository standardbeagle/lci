package main

import (
	"fmt"
	"github.com/standardbeagle/lci/internal/searchtypes"
	"github.com/standardbeagle/lci/internal/types"
)

func main() {
	testIDs := []string{"FS", "BE", "Dq", "DXDnqKY"}

	for _, encoded := range testIDs {
		symbolID, err := searchtypes.DecodeSymbolID(encoded)
		if err != nil {
			fmt.Printf("Error decoding %s: %v\n", encoded, err)
			continue
		}

		// Decode SymbolID structure
		id := uint64(symbolID)
		fileID := types.FileID(id >> 32)
		lineNum := uint32((id >> 16) & 0xFFFF)
		symType := uint16(id & 0xFFFF)

		fmt.Printf("%s -> SymbolID=%d (0x%x) [FileID=%d, Line=%d, Type=%d]\n",
			encoded, symbolID, id, fileID, lineNum, symType)
	}
}
