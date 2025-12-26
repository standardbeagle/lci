package workflow_scenarios

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/standardbeagle/lci/internal/mcp"
)

// TestSideEffects_GoProject tests that side effect tracking works on a real Go project
func TestSideEffects_GoProject(t *testing.T) {
	ctx := GetProject(t, "go", "chi")

	t.Run("SideEffects_Summary", func(t *testing.T) {
		result := ctx.SideEffects("chi_side_effects", mcp.SideEffectsOptions{
			Mode: "summary",
		})

		// CRITICAL: Side effect tracking must find functions
		require.Greater(t, result.TotalFunctions, 0,
			"Side effect tracking found 0 functions - feature is broken!")

		// Chi has many functions, should find a significant number
		assert.Greater(t, result.TotalFunctions, 10,
			"Chi has >50 functions, but only found %d - incomplete indexing?", result.TotalFunctions)

		// Should have some pure functions
		assert.Greater(t, result.PureFunctions, 0,
			"No pure functions found - purity detection broken?")

		// Should have some impure functions
		assert.Greater(t, result.ImpureFunctions, 0,
			"No impure functions found - side effect detection broken?")

		// Ratio should be valid
		assert.InDelta(t, float64(result.PureFunctions)/float64(result.TotalFunctions),
			result.PurityRatio, 0.01,
			"Purity ratio calculation incorrect")

		t.Logf("Side effects: total=%d pure=%d impure=%d ratio=%.2f",
			result.TotalFunctions, result.PureFunctions, result.ImpureFunctions, result.PurityRatio)
	})

	t.Run("CodeInsight_PuritySummary", func(t *testing.T) {
		result := ctx.CodeInsight("chi_purity", mcp.CodeInsightOptions{
			Mode: "overview",
		})

		// Log output first for debugging
		t.Logf("Code insight output:\n%s", truncateForLog(result.Raw, 1200))

		// CRITICAL: Purity summary must be present in health dashboard
		// The PuritySummary struct uses JSON keys like "purity_ratio", "pure_funcs"
		require.True(t, result.Contains("purity") || result.Contains("pure_funcs"),
			"Purity summary missing from code_insight output - integration broken!")
	})
}
