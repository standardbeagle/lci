package testing

import (
	"os"
	"runtime"
	"testing"
)

// PerformanceScaler provides scaling factors for performance tests based on runtime conditions
type PerformanceScaler struct {
	CPUCount         int
	GOMAXPROCS       int
	IsCI             bool
	IsRaceDetector   bool
	IsShortMode      bool
	ConcurrencyRatio float64
}

// NewPerformanceScaler creates a performance scaler for the current test environment
func NewPerformanceScaler(t *testing.T) *PerformanceScaler {
	ps := &PerformanceScaler{
		CPUCount:       runtime.NumCPU(),
		GOMAXPROCS:     runtime.GOMAXPROCS(0),
		IsShortMode:    testing.Short(),
		IsRaceDetector: isRaceDetectorEnabled(),
		IsCI:           isRunningInCI(),
	}

	// Calculate concurrency ratio (how constrained are we?)
	ps.ConcurrencyRatio = float64(ps.GOMAXPROCS) / float64(ps.CPUCount)
	if ps.ConcurrencyRatio > 1.0 {
		ps.ConcurrencyRatio = 1.0
	}

	return ps
}

// ScaleDuration adjusts a duration based on runtime conditions
// IMPORTANT: Conservative scaling - only for known constraints, not general loosening
func (ps *PerformanceScaler) ScaleDuration(base float64) float64 {
	scaled := base

	// Only apply scaling for known constrained environments
	// Don't make limits loose for normal development

	// Race detector adds 2-10x overhead due to synchronization tracking
	if ps.IsRaceDetector {
		scaled *= 2.5 // Conservative 2.5x for race detector
	} else if ps.IsCI {
		// CI can be slower but shouldn't be an excuse for slow code
		scaled *= 1.5 // Only 50% more time in CI
	}

	// GOMAXPROCS=1 means serialized execution (often in CI containers)
	// This is a real constraint that affects tree-sitter CGO calls
	if ps.GOMAXPROCS == 1 {
		scaled *= 1.5 // Additional 50% for serialized execution
	}

	return scaled
}

// ScaleIterations adjusts iteration counts for performance tests
func (ps *PerformanceScaler) ScaleIterations(base int) int {
	if ps.IsShortMode {
		// Short mode gets 10% of iterations
		return base / 10
	}

	// Race detector gets fewer iterations to avoid timeouts
	if ps.IsRaceDetector {
		return base / 3
	}

	// CI gets fewer iterations
	if ps.IsCI {
		return base / 2
	}

	return base
}

// ScaleThroughput adjusts throughput expectations (ops/sec)
// IMPORTANT: Conservative scaling - maintain high standards for normal development
func (ps *PerformanceScaler) ScaleThroughput(base float64) float64 {
	scaled := base

	// Only reduce expectations for real constraints

	// Race detector has significant overhead
	if ps.IsRaceDetector {
		scaled *= 0.4 // Race detector can reduce throughput by 60%
	} else if ps.IsCI {
		// CI should still maintain reasonable throughput
		scaled *= 0.7 // Only 30% reduction for CI
	}

	// GOMAXPROCS=1 legitimately reduces throughput
	if ps.GOMAXPROCS == 1 && ps.CPUCount > 1 {
		// Only apply if we actually have multiple CPUs but are constrained
		scaled *= 0.6
	}

	return scaled
}

// LogScalingFactors logs the current scaling factors for debugging
func (ps *PerformanceScaler) LogScalingFactors(t *testing.T) {
	t.Logf("Performance scaling factors:")
	t.Logf("  CPUs: %d, GOMAXPROCS: %d (ratio: %.2f)", ps.CPUCount, ps.GOMAXPROCS, ps.ConcurrencyRatio)
	t.Logf("  CI: %v, Race: %v, Short: %v", ps.IsCI, ps.IsRaceDetector, ps.IsShortMode)
	t.Logf("  Duration scaling: %.2fx", ps.ScaleDuration(1.0))
	t.Logf("  Throughput scaling: %.2fx", ps.ScaleThroughput(1.0))
}

// isRaceDetectorEnabled checks if the race detector is enabled
func isRaceDetectorEnabled() bool {
	// The race detector sets this build tag
	// We can check it at runtime using a simple technique
	return raceEnabled
}

// isRunningInCI checks common CI environment variables
func isRunningInCI() bool {
	ciVars := []string{
		"CI",
		"CONTINUOUS_INTEGRATION",
		"GITHUB_ACTIONS",
		"GITLAB_CI",
		"JENKINS",
		"CIRCLECI",
		"TRAVIS",
		"BUILDKITE",
		"DRONE",
	}

	for _, v := range ciVars {
		if os.Getenv(v) != "" {
			return true
		}
	}

	return false
}
