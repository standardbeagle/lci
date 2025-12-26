package core

import (
	"math"
	"sync/atomic"
)

// Configuration methods for ContextLookupEngine

// SetMaxContextDepth sets the maximum depth for relationship analysis
func (cle *ContextLookupEngine) SetMaxContextDepth(depth int) {
	if depth > 0 && depth <= 20 {
		atomic.StoreInt32(&cle.maxContextDepth, int32(depth))
	}
}

// SetIncludeAIText sets whether to include AI-generated text
func (cle *ContextLookupEngine) SetIncludeAIText(include bool) {
	var val int32 = 0
	if include {
		val = 1
	}
	atomic.StoreInt32(&cle.includeAIText, val)
}

// SetConfidenceThreshold sets the minimum confidence threshold
func (cle *ContextLookupEngine) SetConfidenceThreshold(threshold float64) {
	if threshold >= 0.0 && threshold <= 1.0 {
		atomic.StoreInt64(&cle.confidenceThreshold, int64(math.Float64bits(threshold)))
	}
}

// GetMaxContextDepth returns the current max context depth
func (cle *ContextLookupEngine) GetMaxContextDepth() int {
	return int(atomic.LoadInt32(&cle.maxContextDepth))
}

// GetIncludeAIText returns whether AI text is included
func (cle *ContextLookupEngine) GetIncludeAIText() bool {
	return atomic.LoadInt32(&cle.includeAIText) != 0
}

// GetConfidenceThreshold returns the current confidence threshold
func (cle *ContextLookupEngine) GetConfidenceThreshold() float64 {
	bits := atomic.LoadInt64(&cle.confidenceThreshold)
	return math.Float64frombits(uint64(bits))
}