package semantic

import (
	"runtime"
	"testing"

	"github.com/hbollon/go-edlib"
	"github.com/surgebase/porter2"
)

// BenchmarkJaroWinklerMemory measures memory allocation of JaroWinkler alone
func BenchmarkJaroWinklerMemory(b *testing.B) {
	tests := []struct {
		name string
		a    string
		b    string
	}{
		{"short", "getUserName", "getUserNme"},
		{"medium", "XMLHttpRequest", "XmlHttpReqest"},
		{"long", "AbstractFactoryPatternBuilder", "AbstactFactryPaternBuilder"},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = edlib.StringsSimilarity(tt.a, tt.b, edlib.JaroWinkler)
			}
		})
	}
}

// BenchmarkLevenshteinMemory measures memory allocation of Levenshtein alone
func BenchmarkLevenshteinMemory(b *testing.B) {
	tests := []struct {
		name string
		a    string
		b    string
	}{
		{"short", "getUserName", "getUserNme"},
		{"medium", "XMLHttpRequest", "XmlHttpReqest"},
		{"long", "AbstractFactoryPatternBuilder", "AbstactFactryPaternBuilder"},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = edlib.StringsSimilarity(tt.a, tt.b, edlib.Levenshtein)
			}
		})
	}
}

// BenchmarkPorter2StemmerMemory measures memory allocation of Porter2 stemmer
func BenchmarkPorter2StemmerMemory(b *testing.B) {
	words := []string{
		"running",
		"runner",
		"runs",
		"authentication",
		"controller",
		"manager",
		"processor",
		"handler",
		"configuration",
		"initialization",
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, word := range words {
			_ = porter2.Stem(word)
		}
	}
}

// TestJaroWinklerGCPressure measures GC pressure from JaroWinkler
func TestJaroWinklerGCPressure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping GC pressure test in short mode")
	}

	pairs := []struct{ a, b string }{
		{"getUserName", "getUserNme"},
		{"createTable", "creatTable"},
		{"HTTPServer", "HttpServer"},
		{"parseJSON", "parseJson"},
	}

	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	// Run 1000 comparisons
	for i := 0; i < 1000; i++ {
		for _, pair := range pairs {
			_, _ = edlib.StringsSimilarity(pair.a, pair.b, edlib.JaroWinkler)
		}
	}

	runtime.GC()
	runtime.ReadMemStats(&m2)

	allocated := m2.TotalAlloc - m1.TotalAlloc
	perCall := allocated / (1000 * uint64(len(pairs)))

	t.Logf("JaroWinkler: Total allocated: %d bytes", allocated)
	t.Logf("JaroWinkler: Per call: %d bytes", perCall)
	t.Logf("JaroWinkler: GC runs: %d", m2.NumGC-m1.NumGC)

	// Verify it's not insane
	if perCall > 10000 {
		t.Errorf("JaroWinkler allocates %d bytes per call, expected <10KB", perCall)
	}
}

// TestPorter2GCPressure measures GC pressure from Porter2 stemmer
func TestPorter2GCPressure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping GC pressure test in short mode")
	}

	words := []string{
		"running", "runner", "runs",
		"authentication", "authenticating", "authenticate",
		"controller", "controlling", "controls",
	}

	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	// Run 1000 stemming operations
	for i := 0; i < 1000; i++ {
		for _, word := range words {
			porter2.Stem(word)
		}
	}

	runtime.GC()
	runtime.ReadMemStats(&m2)

	allocated := m2.TotalAlloc - m1.TotalAlloc
	perCall := allocated / (1000 * uint64(len(words)))

	t.Logf("Porter2: Total allocated: %d bytes", allocated)
	t.Logf("Porter2: Per call: %d bytes", perCall)
	t.Logf("Porter2: GC runs: %d", m2.NumGC-m1.NumGC)

	// Verify it's not insane
	if perCall > 1000 {
		t.Errorf("Porter2 allocates %d bytes per call, expected <1KB", perCall)
	}
}

// BenchmarkSimpleStringComparison shows baseline for comparison
func BenchmarkSimpleStringComparison(b *testing.B) {
	str1 := "getUserName"
	str2 := "getUserNme"

	b.Run("Equals", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = str1 == str2
		}
	})

	b.Run("Contains", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = str1[0:5] == str2[0:5]
		}
	})

	b.Run("PrefixMatch", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			prefix := min(len(str1), len(str2))
			match := true
			for j := 0; j < prefix; j++ {
				if str1[j] != str2[j] {
					match = false
					break
				}
			}
			_ = match
		}
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
