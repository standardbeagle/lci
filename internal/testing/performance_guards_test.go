package testing

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPerformanceGuard_Measure(t *testing.T) {
	guard := NewPerformanceGuard(t)

	measurement := guard.Measure("test", func() {
		time.Sleep(10 * time.Millisecond)
	})

	assert.Equal(t, "test", measurement.Name)
	assert.GreaterOrEqual(t, measurement.Duration, 10*time.Millisecond)
	assert.Less(t, measurement.Duration, 100*time.Millisecond)
}

func TestPerformanceGuard_MeasureN(t *testing.T) {
	guard := NewPerformanceGuard(t)

	measurements := guard.MeasureN("test", 10, func() {
		time.Sleep(1 * time.Millisecond)
	})

	assert.Len(t, measurements, 10)
	for _, m := range measurements {
		assert.GreaterOrEqual(t, m.Duration, 1*time.Millisecond)
	}
}

func TestPerformanceGuard_Check_Pass(t *testing.T) {
	guard := NewPerformanceGuard(t).
		WithThreshold("fast", PerformanceThreshold{
			MaxDuration: 100 * time.Millisecond,
		})

	guard.Measure("fast", func() {
		time.Sleep(1 * time.Millisecond)
	})

	result := guard.Check("fast")
	assert.True(t, result.Passed)
	assert.Empty(t, result.Violations)
}

func TestPerformanceGuard_Check_Fail(t *testing.T) {
	guard := NewPerformanceGuard(t).
		WithThreshold("slow", PerformanceThreshold{
			MaxDuration: 1 * time.Millisecond,
		})

	guard.Measure("slow", func() {
		time.Sleep(10 * time.Millisecond)
	})

	result := guard.Check("slow")
	assert.False(t, result.Passed)
	assert.NotEmpty(t, result.Violations)
}

func TestPerformanceGuard_Percentiles(t *testing.T) {
	guard := NewPerformanceGuard(t).
		WithThreshold("test", PerformanceThreshold{
			P50Threshold: 50 * time.Millisecond,
			P95Threshold: 100 * time.Millisecond,
		})

	// Record measurements with varying durations
	for i := 0; i < 100; i++ {
		guard.Measure("test", func() {
			time.Sleep(time.Duration(i%10) * time.Millisecond)
		})
	}

	result := guard.Check("test")
	assert.Greater(t, result.P50, time.Duration(0))
	assert.Greater(t, result.P95, time.Duration(0))
	assert.GreaterOrEqual(t, result.P95, result.P50)
}

func TestRegressionGuard_NoRegression(t *testing.T) {
	guard := NewRegressionGuard(10.0) // 10% threshold

	guard.SetBaseline("op1", 100*time.Millisecond)
	guard.RecordMeasurement("op1", 105*time.Millisecond) // 5% slower

	hasRegression, delta := guard.CheckRegression("op1")
	assert.False(t, hasRegression)
	assert.Less(t, delta, 0.1) // Less than 10%
}

func TestRegressionGuard_WithRegression(t *testing.T) {
	guard := NewRegressionGuard(10.0) // 10% threshold

	guard.SetBaseline("op1", 100*time.Millisecond)
	guard.RecordMeasurement("op1", 120*time.Millisecond) // 20% slower

	hasRegression, delta := guard.CheckRegression("op1")
	assert.True(t, hasRegression)
	assert.GreaterOrEqual(t, delta, 0.1) // At least 10%
}

func TestRegressionGuard_CheckAll(t *testing.T) {
	guard := NewRegressionGuard(10.0)

	guard.SetBaseline("fast", 100*time.Millisecond)
	guard.SetBaseline("slow", 100*time.Millisecond)

	guard.RecordMeasurement("fast", 105*time.Millisecond) // OK
	guard.RecordMeasurement("slow", 150*time.Millisecond) // 50% slower

	regressions := guard.CheckAllRegressions()
	assert.Len(t, regressions, 1)
	assert.Contains(t, regressions, "slow")
}

func TestMemoryGuard_WithinBounds(t *testing.T) {
	guard := NewMemoryGuard(t).WithMaxHeapMB(1000)

	// Allocate small amount
	_ = make([]byte, 1024)

	err := guard.Check()
	assert.NoError(t, err)
}

func TestMemoryGuard_Checkpoint(t *testing.T) {
	guard := NewMemoryGuard(t)

	guard.Checkpoint()

	// Allocate some memory
	data := make([]byte, 1024*1024) // 1MB
	_ = data

	guard.Checkpoint()

	// Should have recorded growth
	assert.Len(t, guard.checkpoints, 2)
}

func TestThroughputGuard_RecordOps(t *testing.T) {
	guard := NewThroughputGuard(t, 100) // 100 ops/sec minimum

	guard.RecordOps(1000, time.Second) // 1000 ops/sec

	avg := guard.GetAverageThroughput()
	assert.Equal(t, float64(1000), avg)
}

func TestThroughputGuard_MeasureOps(t *testing.T) {
	guard := NewThroughputGuard(t, 1) // Very low minimum

	ops := guard.MeasureOps(100, func() {
		time.Sleep(10 * time.Millisecond)
	})

	assert.Greater(t, ops, float64(0))
}

func TestStatisticalHelpers(t *testing.T) {
	t.Run("average", func(t *testing.T) {
		durations := []time.Duration{
			10 * time.Millisecond,
			20 * time.Millisecond,
			30 * time.Millisecond,
		}

		avg := average(durations)
		assert.Equal(t, 20*time.Millisecond, avg)
	})

	t.Run("average_empty", func(t *testing.T) {
		avg := average([]time.Duration{})
		assert.Equal(t, time.Duration(0), avg)
	})

	t.Run("percentile_50", func(t *testing.T) {
		durations := []time.Duration{
			10 * time.Millisecond,
			20 * time.Millisecond,
			30 * time.Millisecond,
			40 * time.Millisecond,
			50 * time.Millisecond,
		}

		p50 := percentile(durations, 50)
		assert.Equal(t, 30*time.Millisecond, p50)
	})

	t.Run("percentile_95", func(t *testing.T) {
		durations := make([]time.Duration, 100)
		for i := range durations {
			durations[i] = time.Duration(i) * time.Millisecond
		}

		p95 := percentile(durations, 95)
		assert.GreaterOrEqual(t, p95, 90*time.Millisecond)
	})

	t.Run("percentile_empty", func(t *testing.T) {
		p := percentile([]time.Duration{}, 50)
		assert.Equal(t, time.Duration(0), p)
	})
}

func TestSortDurations(t *testing.T) {
	durations := []time.Duration{
		50 * time.Millisecond,
		10 * time.Millisecond,
		30 * time.Millisecond,
		20 * time.Millisecond,
		40 * time.Millisecond,
	}

	sortDurations(durations)

	for i := 1; i < len(durations); i++ {
		assert.LessOrEqual(t, durations[i-1], durations[i])
	}
}

func TestFormatViolations(t *testing.T) {
	violations := []string{
		"First violation",
		"Second violation",
	}

	formatted := formatViolations(violations)
	assert.Contains(t, formatted, "First violation")
	assert.Contains(t, formatted, "Second violation")
}
