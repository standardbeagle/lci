package indexing

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/standardbeagle/lci/internal/config"
)

// TestMasterIndex_SideEffectTracking tests that side effects work through the full stack.
// This is the CRITICAL test that MUST FAIL when side effect tracking is broken.
func TestMasterIndex_SideEffectTracking(t *testing.T) {
	// Create temp directory with test files
	tmpDir := t.TempDir()

	// Write test files with known pure and impure functions
	err := os.WriteFile(filepath.Join(tmpDir, "pure.go"), []byte(`package test

func PureAdd(a, b int) int {
	return a + b
}

func PureMul(x int) int {
	return x * 2
}
`), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, "impure.go"), []byte(`package test

import "fmt"

func ImpurePrint(x int) {
	fmt.Println(x)
}

func ImpureModifySlice(s []int) {
	s[0] = 999
}
`), 0644)
	require.NoError(t, err)

	cfg := &config.Config{
		Project: config.Project{Root: tmpDir},
		Index:   config.Index{MaxFileSize: 10 * 1024 * 1024},
	}

	// Create MasterIndex and index the directory
	mi := NewMasterIndex(cfg)
	defer mi.Close()

	ctx := context.Background()
	err = mi.IndexDirectory(ctx, tmpDir)
	require.NoError(t, err, "Indexing should succeed")

	// CRITICAL: Side effect propagator MUST exist
	propagator := mi.GetSideEffectPropagator()
	require.NotNil(t, propagator, "CRITICAL: SideEffectPropagator is nil!")

	// CRITICAL: Must have side effects collected
	allEffects := propagator.GetAllSideEffects()
	require.NotEmpty(t, allEffects,
		"CRITICAL: No side effects collected - feature is completely broken!")

	t.Logf("Collected %d side effects from parser", len(allEffects))

	// Build purity summary
	totalFunctions := len(allEffects)
	pureFunctions := 0
	impureFunctions := 0

	for _, info := range allEffects {
		if info == nil {
			continue
		}
		if info.IsPure {
			pureFunctions++
		} else {
			impureFunctions++
		}
	}

	t.Logf("Purity summary: total=%d pure=%d impure=%d ratio=%.2f",
		totalFunctions, pureFunctions, impureFunctions,
		float64(pureFunctions)/float64(totalFunctions))

	// CRITICAL ASSERTIONS
	require.Greater(t, totalFunctions, 0,
		"CRITICAL: TotalFunctions is 0!")
	require.Greater(t, pureFunctions, 0,
		"CRITICAL: PureFunctions is 0 - cannot detect pure functions!")
	require.Greater(t, impureFunctions, 0,
		"CRITICAL: ImpureFunctions is 0 - cannot detect impure functions!")

	// We should have at least 2 pure functions (PureAdd, PureMul)
	assert.GreaterOrEqual(t, pureFunctions, 2,
		"Should detect at least 2 pure functions, got %d", pureFunctions)

	// We should have at least 2 impure functions (ImpurePrint, ImpureModifySlice)
	assert.GreaterOrEqual(t, impureFunctions, 2,
		"Should detect at least 2 impure functions, got %d", impureFunctions)
}
