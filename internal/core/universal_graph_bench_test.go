package core

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/types"
)

// BenchmarkUniversalGraphMemoryUsage benchmarks memory usage of the Universal Symbol Graph
func BenchmarkUniversalGraphMemoryUsage(b *testing.B) {
	// Test different node limits
	nodeLimits := []int{1000, 10000, 50000, 100000}

	for _, limit := range nodeLimits {
		b.Run(fmt.Sprintf("nodes_%d", limit), func(b *testing.B) {
			benchmarkMemoryUsageWithLimit(b, limit)
		})
	}
}

func benchmarkMemoryUsageWithLimit(b *testing.B, maxNodes int) {
	// Force GC and get baseline memory
	runtime.GC()
	runtime.GC()
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		usg := NewUniversalSymbolGraphWithConfig(maxNodes)

		// Add nodes up to limit
		nodesToAdd := maxNodes
		if nodesToAdd > 1000 { // Limit test data to reasonable size
			nodesToAdd = 1000
		}

		for j := 0; j < nodesToAdd; j++ {
			node := &types.UniversalSymbolNode{
				Identity: types.SymbolIdentity{
					ID: types.CompositeSymbolID{
						FileID:        types.FileID(j % 100),
						LocalSymbolID: uint32(j),
					},
					Name:     fmt.Sprintf("symbol_%d", j),
					Kind:     types.SymbolKindFunction,
					Language: "go",
					Location: types.SymbolLocation{
						FileID: types.FileID(j % 100),
						Line:   j % 1000,
						Column: 1,
					},
				},
				Usage: types.SymbolUsage{
					FirstSeen: time.Now(),
				},
			}

			err := usg.AddSymbol(node)
			if err != nil {
				b.Fatalf("Failed to add symbol: %v", err)
			}
		}

		// Force some LRU operations by exceeding limit
		if nodesToAdd >= maxNodes {
			for k := 0; k < 100; k++ {
				extraNode := &types.UniversalSymbolNode{
					Identity: types.SymbolIdentity{
						ID: types.CompositeSymbolID{
							FileID:        types.FileID(1000),
							LocalSymbolID: uint32(nodesToAdd + k),
						},
						Name:     fmt.Sprintf("extra_symbol_%d", k),
						Kind:     types.SymbolKindFunction,
						Language: "go",
						Location: types.SymbolLocation{
							FileID: types.FileID(1000),
							Line:   k,
							Column: 1,
						},
					},
					Usage: types.SymbolUsage{
						FirstSeen: time.Now(),
					},
				}
				_ = usg.AddSymbol(extraNode)
			}
		}

		// Test some queries to verify LRU access tracking
		for q := 0; q < 10; q++ {
			id := types.CompositeSymbolID{
				FileID:        types.FileID(q % 100),
				LocalSymbolID: uint32(q),
			}
			_, _ = usg.GetSymbol(id)
		}

		// Test memory limits are working
		if len(usg.nodes) > maxNodes {
			b.Errorf("Universal Graph exceeded memory limit: %d > %d", len(usg.nodes), maxNodes)
		}
	}

	b.StopTimer()

	// Measure final memory usage
	runtime.GC()
	runtime.GC()
	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)

	memUsed := memAfter.Alloc - memBefore.Alloc
	b.ReportMetric(float64(memUsed)/1024/1024, "MB")
	b.ReportMetric(float64(memAfter.Sys)/1024/1024, "sys_MB")
}

// BenchmarkUniversalGraphQueryPerformance benchmarks query performance to ensure <5ms constraint
func BenchmarkUniversalGraphQueryPerformance(b *testing.B) {
	usg := NewUniversalSymbolGraph()

	// Pre-populate with test data
	for i := 0; i < 10000; i++ {
		node := &types.UniversalSymbolNode{
			Identity: types.SymbolIdentity{
				ID: types.CompositeSymbolID{
					FileID:        types.FileID(i % 100),
					LocalSymbolID: uint32(i),
				},
				Name:     fmt.Sprintf("test_symbol_%d", i),
				Kind:     types.SymbolKindFunction,
				Language: "go",
				Location: types.SymbolLocation{
					FileID: types.FileID(i % 100),
					Line:   i % 1000,
					Column: 1,
				},
			},
			Usage: types.SymbolUsage{
				FirstSeen: time.Now(),
			},
		}
		_ = usg.AddSymbol(node)
	}

	// Benchmark queries
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Query by ID
		id := types.CompositeSymbolID{
			FileID:        types.FileID(i % 100),
			LocalSymbolID: uint32(i % 10000),
		}
		start := time.Now()
		_, _ = usg.GetSymbol(id)
		queryTime := time.Since(start)

		// Verify <5ms constraint
		if queryTime > 5*time.Millisecond {
			b.Errorf("Query took too long: %v > 5ms", queryTime)
		}
	}
}

// TestMemoryLimitEnforcement tests that memory limits are properly enforced
func TestMemoryLimitEnforcement(t *testing.T) {
	maxNodes := 100
	usg := NewUniversalSymbolGraphWithConfig(maxNodes)

	// Add nodes up to the limit
	for i := 0; i < maxNodes; i++ {
		node := &types.UniversalSymbolNode{
			Identity: types.SymbolIdentity{
				ID: types.CompositeSymbolID{
					FileID:        types.FileID(1),
					LocalSymbolID: uint32(i),
				},
				Name:     fmt.Sprintf("symbol_%d", i),
				Kind:     types.SymbolKindFunction,
				Language: "go",
				Location: types.SymbolLocation{
					FileID: types.FileID(1),
					Line:   i,
					Column: 1,
				},
			},
			Usage: types.SymbolUsage{
				FirstSeen: time.Now(),
			},
		}

		err := usg.AddSymbol(node)
		if err != nil {
			t.Fatalf("Failed to add symbol %d: %v", i, err)
		}
	}

	// Verify we're at the limit
	if len(usg.nodes) != maxNodes {
		t.Errorf("Expected %d nodes, got %d", maxNodes, len(usg.nodes))
	}

	// Add more nodes to trigger LRU eviction
	for i := maxNodes; i < maxNodes+50; i++ {
		node := &types.UniversalSymbolNode{
			Identity: types.SymbolIdentity{
				ID: types.CompositeSymbolID{
					FileID:        types.FileID(2),
					LocalSymbolID: uint32(i),
				},
				Name:     fmt.Sprintf("overflow_symbol_%d", i),
				Kind:     types.SymbolKindFunction,
				Language: "go",
				Location: types.SymbolLocation{
					FileID: types.FileID(2),
					Line:   i,
					Column: 1,
				},
			},
			Usage: types.SymbolUsage{
				FirstSeen: time.Now(),
			},
		}

		err := usg.AddSymbol(node)
		if err != nil {
			t.Fatalf("Failed to add overflow symbol %d: %v", i, err)
		}

		// Verify limit is still enforced
		if len(usg.nodes) > maxNodes {
			t.Errorf("Memory limit exceeded: %d > %d", len(usg.nodes), maxNodes)
		}
	}

	// Verify final state
	if len(usg.nodes) != maxNodes {
		t.Errorf("Expected %d nodes after overflow, got %d", maxNodes, len(usg.nodes))
	}

	// Verify LRU access order is maintained
	if len(usg.accessOrder) != len(usg.nodes) {
		t.Errorf("LRU access order length mismatch: %d != %d", len(usg.accessOrder), len(usg.nodes))
	}

	t.Logf("Successfully enforced memory limit of %d nodes", maxNodes)
}
