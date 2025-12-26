package testing

import (
	"testing"
	"time"
)

// RetryTimingAssertion implements stampede prevention by retrying timing-sensitive assertions
// This handles transient system load that can cause timing tests to fail spuriously.
//
// Parameters:
//   - t: testing.T instance
//   - maxRetries: number of retry attempts (minimum 1 for stampede prevention)
//   - testFn: function that performs the timed operation and returns (duration, error)
//   - threshold: maximum acceptable duration
//   - description: description of what's being timed
//
// Returns true if the assertion passed, false otherwise
func RetryTimingAssertion(t *testing.T, maxRetries int, testFn func() (time.Duration, error), threshold time.Duration, description string) bool {
	t.Helper()

	if maxRetries < 1 {
		maxRetries = 1
	}

	var lastDuration time.Duration

	for attempt := 0; attempt < maxRetries; attempt++ {
		duration, err := testFn()
		lastDuration = duration

		if err != nil {
			if attempt < maxRetries-1 {
				t.Logf("Attempt %d/%d: %s failed with error: %v, retrying after brief pause...",
					attempt+1, maxRetries, description, err)
				time.Sleep(100 * time.Millisecond)
				continue
			}
			t.Errorf("%s failed after %d attempts: %v", description, maxRetries, err)
			return false
		}

		if duration <= threshold {
			if attempt > 0 {
				t.Logf("✓ %s: %v (passed on attempt %d/%d)", description, duration, attempt+1, maxRetries)
			}
			return true
		}

		if attempt < maxRetries-1 {
			t.Logf("Attempt %d/%d: %s took %v (limit: %v), retrying after brief pause...",
				attempt+1, maxRetries, description, duration, threshold)
			time.Sleep(200 * time.Millisecond) // Give system time to settle
		}
	}

	t.Errorf("%s too slow after %d attempts: %v (expected <%v)",
		description, maxRetries, lastDuration, threshold)
	return false
}

// RetryTimingAssertionWithSetup is like RetryTimingAssertion but allows setup before each retry
//
// Parameters:
//   - t: testing.T instance
//   - maxRetries: number of retry attempts (minimum 1 for stampede prevention)
//   - setupFn: function called before each attempt (for resetting state)
//   - testFn: function that performs the timed operation and returns (duration, error)
//   - threshold: maximum acceptable duration
//   - description: description of what's being timed
func RetryTimingAssertionWithSetup(t *testing.T, maxRetries int, setupFn func() error, testFn func() (time.Duration, error), threshold time.Duration, description string) bool {
	t.Helper()

	if maxRetries < 1 {
		maxRetries = 1
	}

	var lastDuration time.Duration

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Run setup before each attempt
		if setupFn != nil {
			if err := setupFn(); err != nil {
				t.Fatalf("Setup failed for %s: %v", description, err)
				return false
			}
		}

		duration, err := testFn()
		lastDuration = duration

		if err != nil {
			if attempt < maxRetries-1 {
				t.Logf("Attempt %d/%d: %s failed with error: %v, retrying...",
					attempt+1, maxRetries, description, err)
				time.Sleep(100 * time.Millisecond)
				continue
			}
			t.Errorf("%s failed after %d attempts: %v", description, maxRetries, err)
			return false
		}

		if duration <= threshold {
			if attempt > 0 {
				t.Logf("✓ %s: %v (passed on attempt %d/%d)", description, duration, attempt+1, maxRetries)
			}
			return true
		}

		if attempt < maxRetries-1 {
			t.Logf("Attempt %d/%d: %s took %v (limit: %v), retrying...",
				attempt+1, maxRetries, description, duration, threshold)
			time.Sleep(200 * time.Millisecond)
		}
	}

	t.Errorf("%s too slow after %d attempts: %v (expected <%v)",
		description, maxRetries, lastDuration, threshold)
	return false
}
