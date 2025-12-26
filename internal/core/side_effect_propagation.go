package core

import (
	"github.com/standardbeagle/lci/internal/types"
)

// SideEffectPropagator propagates side effect information through the call graph.
// This allows transitive purity analysis - a function that calls an impure function
// is itself impure, even if its local analysis shows no side effects.
type SideEffectPropagator struct {
	// Graph infrastructure
	refTracker  *ReferenceTracker
	symbolIndex *SymbolIndex

	// Side effect data
	sideEffectInfo map[types.SymbolID]*types.SideEffectInfo

	// Configuration
	config *SideEffectPropagationConfig

	// Propagation state
	propagated map[types.SymbolID]bool
	iterations int
}

// SideEffectPropagationConfig controls propagation behavior
type SideEffectPropagationConfig struct {
	// MaxIterations limits propagation iterations (for cycle detection)
	MaxIterations int

	// PropagateIO - if true, I/O effects propagate to callers
	PropagateIO bool

	// PropagateThrows - if true, throw/panic effects propagate to callers
	PropagateThrows bool

	// PropagateGlobalWrites - if true, global write effects propagate to callers
	PropagateGlobalWrites bool

	// ConfidenceDecay - how much confidence decays per hop (e.g., 0.9 = 10% decay)
	ConfidenceDecay float64

	// MinConfidence - minimum confidence to continue propagation
	MinConfidence float64
}

// DefaultSideEffectPropagationConfig returns sensible defaults
func DefaultSideEffectPropagationConfig() *SideEffectPropagationConfig {
	return &SideEffectPropagationConfig{
		MaxIterations:         100,
		PropagateIO:           true,
		PropagateThrows:       true,
		PropagateGlobalWrites: true,
		ConfidenceDecay:       0.95, // 5% decay per hop
		MinConfidence:         0.3,  // Stop at 30% confidence
	}
}

// NewSideEffectPropagator creates a new propagator
func NewSideEffectPropagator(refTracker *ReferenceTracker, symbolIndex *SymbolIndex, config *SideEffectPropagationConfig) *SideEffectPropagator {
	if config == nil {
		config = DefaultSideEffectPropagationConfig()
	}
	return &SideEffectPropagator{
		refTracker:     refTracker,
		symbolIndex:    symbolIndex,
		sideEffectInfo: make(map[types.SymbolID]*types.SideEffectInfo),
		config:         config,
		propagated:     make(map[types.SymbolID]bool),
	}
}

// AddLocalSideEffect registers locally-detected side effect info for a symbol
func (sep *SideEffectPropagator) AddLocalSideEffect(symbolID types.SymbolID, info *types.SideEffectInfo) {
	sep.sideEffectInfo[symbolID] = info
}

// Propagate performs transitive side effect propagation through the call graph.
// Side effects flow upstream: if A calls B and B has side effects, A inherits them.
func (sep *SideEffectPropagator) Propagate() error {
	if sep.refTracker == nil {
		return nil
	}

	// Reset propagation state
	sep.propagated = make(map[types.SymbolID]bool)
	sep.iterations = 0

	// Iterate until convergence or max iterations
	for sep.iterations < sep.config.MaxIterations {
		changed := sep.propagateIteration()
		sep.iterations++

		if !changed {
			break
		}
	}

	return nil
}

// propagateIteration performs one iteration of side effect propagation.
// Returns true if any values changed (needs another iteration).
func (sep *SideEffectPropagator) propagateIteration() bool {
	changed := false

	for symbolID, info := range sep.sideEffectInfo {
		// Get callers of this symbol
		callers := sep.refTracker.GetCallerSymbols(symbolID)

		for _, callerID := range callers {
			if sep.propagateToCallerOnce(callerID, info) {
				changed = true
			}
		}
	}

	return changed
}

// propagateToCallerOnce propagates side effects from callee to caller.
// Returns true if the caller's side effect info was updated.
func (sep *SideEffectPropagator) propagateToCallerOnce(callerID types.SymbolID, calleeInfo *types.SideEffectInfo) bool {
	if calleeInfo == nil {
		return false
	}

	// Get or create caller's info
	callerInfo := sep.sideEffectInfo[callerID]
	if callerInfo == nil {
		callerInfo = types.NewSideEffectInfo()
		sep.sideEffectInfo[callerID] = callerInfo
	}

	changed := false

	// Determine which effects to propagate (include both local and already-transitive from callee)
	combinedCalleeEffects := calleeInfo.Categories | calleeInfo.TransitiveCategories
	categoriesToPropagate := sep.getCategoriesToPropagate(combinedCalleeEffects)

	// Propagate categories
	oldCategories := callerInfo.TransitiveCategories
	callerInfo.TransitiveCategories |= categoriesToPropagate

	if callerInfo.TransitiveCategories != oldCategories {
		changed = true
	}

	// Update transitive confidence (decay with depth)
	if calleeInfo.Confidence != types.ConfidenceNone {
		// Propagate with decay
		propagatedConfidence := sep.decayConfidence(calleeInfo.TransitiveConfidence)
		if propagatedConfidence == types.ConfidenceNone {
			propagatedConfidence = sep.decayConfidence(calleeInfo.Confidence)
		}

		if propagatedConfidence > callerInfo.TransitiveConfidence {
			callerInfo.TransitiveConfidence = propagatedConfidence
			changed = true
		}
	}

	// Update overall purity assessment
	if changed {
		sep.updatePurityAssessment(callerInfo)
	}

	return changed
}

// getCategoriesToPropagate filters categories based on configuration
func (sep *SideEffectPropagator) getCategoriesToPropagate(categories types.SideEffectCategory) types.SideEffectCategory {
	var result types.SideEffectCategory

	// Always propagate these
	result |= categories & (types.SideEffectParamWrite | types.SideEffectReceiverWrite)

	if sep.config.PropagateIO {
		result |= categories & (types.SideEffectIO | types.SideEffectNetwork |
			types.SideEffectDatabase | types.SideEffectChannel)
	}

	if sep.config.PropagateThrows {
		result |= categories & types.SideEffectThrow
	}

	if sep.config.PropagateGlobalWrites {
		result |= categories & types.SideEffectGlobalWrite
	}

	// Always propagate uncertainty
	result |= categories & (types.SideEffectUncertain | types.SideEffectExternalCall | types.SideEffectDynamicCall)

	return result
}

// decayConfidence applies confidence decay for transitive propagation
func (sep *SideEffectPropagator) decayConfidence(conf types.PurityConfidence) types.PurityConfidence {
	// Convert confidence to numeric for decay calculation
	// Then convert back to enum

	confValue := float64(conf) / float64(types.ConfidenceProven)
	decayed := confValue * sep.config.ConfidenceDecay

	if decayed < sep.config.MinConfidence {
		return types.ConfidenceNone
	}

	// Convert back to enum
	if decayed >= 0.9 {
		return types.ConfidenceProven
	}
	if decayed >= 0.75 {
		return types.ConfidenceHigh
	}
	if decayed >= 0.5 {
		return types.ConfidenceMedium
	}
	if decayed >= 0.25 {
		return types.ConfidenceLow
	}
	return types.ConfidenceNone
}

// updatePurityAssessment updates the purity assessment after propagation
// This is Phase 2: resolve unresolved calls and upgrade purity levels
func (sep *SideEffectPropagator) updatePurityAssessment(info *types.SideEffectInfo) {
	// Combine local and transitive categories
	combined := info.Categories | info.TransitiveCategories

	// Phase 2: Resolve dependent function purity
	// Update DependentFunctions map with actual purity from propagation
	for funcName := range info.PurityClassification.DependentFunctions {
		// Try to find this function in our side effect info
		// For now, conservatively assume if no transitive effects propagated, it's pure
		// This will be refined when we have proper symbol resolution
		info.PurityClassification.DependentFunctions[funcName] = (combined == types.SideEffectNone)
	}

	// Phase 2 logic: If we have unresolved calls but no transitive effects,
	// then all unresolved calls must be pure (otherwise they would have propagated effects)
	// Upgrade Level 2 (InternallyPure) -> Level 1 (Pure)
	hasUnresolvedCalls := len(info.UnresolvedCalls) > 0
	if combined == types.SideEffectNone {
		if hasUnresolvedCalls {
			// All unresolved calls were pure - upgrade to Level 1
			info.PurityLevel = types.PurityLevelPure
			// Update all dependent functions to pure
			for funcName := range info.PurityClassification.DependentFunctions {
				info.PurityClassification.DependentFunctions[funcName] = true
			}
		} else {
			// No effects and no unresolved calls - already Level 1
			info.PurityLevel = types.PurityLevelPure
		}
		info.IsPure = true
		info.PurityScore = 1.0
		info.PurityConfidence = float64(info.Confidence) / float64(types.ConfidenceProven)
	} else {
		// Has side effects - recompute level based on combined effects
		info.PurityLevel = types.ComputePurityLevel(combined, hasUnresolvedCalls)
		info.IsPure = false

		// Compute score based on level
		switch info.PurityLevel {
		case types.PurityLevelObjectState:
			info.PurityScore = 0.6
			info.PurityConfidence = 0.8
		case types.PurityLevelModuleGlobal:
			info.PurityScore = 0.3
			info.PurityConfidence = 0.8
		case types.PurityLevelExternalDependency:
			info.PurityScore = 0.0
			info.PurityConfidence = 0.9
		default:
			info.PurityScore = 0.0
			info.PurityConfidence = 0.5
		}
	}
}

// GetSideEffectInfo returns the side effect info for a symbol (including propagated effects)
func (sep *SideEffectPropagator) GetSideEffectInfo(symbolID types.SymbolID) *types.SideEffectInfo {
	return sep.sideEffectInfo[symbolID]
}

// GetAllSideEffects returns all side effect info
func (sep *SideEffectPropagator) GetAllSideEffects() map[types.SymbolID]*types.SideEffectInfo {
	return sep.sideEffectInfo
}

// GetIterationCount returns the number of propagation iterations performed
func (sep *SideEffectPropagator) GetIterationCount() int {
	return sep.iterations
}

// PropagateSideEffectsFromResults takes results from the unified extractor
// and registers them for propagation using the reference tracker to find symbol IDs.
func (sep *SideEffectPropagator) PropagateSideEffectsFromResults(
	fileID types.FileID,
	results map[string]*types.SideEffectInfo,
) {
	if results == nil || sep.refTracker == nil {
		return
	}

	// Match side effect results to symbols by line number
	for _, info := range results {
		startLine := info.StartLine

		// Find symbol at this line using refTracker
		symbol := sep.refTracker.GetSymbolAtLine(fileID, startLine)
		if symbol != nil && (symbol.Type == types.SymbolTypeFunction || symbol.Type == types.SymbolTypeMethod) {
			sep.AddLocalSideEffect(symbol.ID, info)
		}
	}
}

// GetPurityReport generates a human-readable purity report for a symbol
func (sep *SideEffectPropagator) GetPurityReport(symbolID types.SymbolID) *PurityReport {
	info := sep.GetSideEffectInfo(symbolID)
	if info == nil {
		return nil
	}

	report := &PurityReport{
		SymbolID:             symbolID,
		FunctionName:         info.FunctionName,
		IsPure:               info.IsPure,
		LocalCategories:      info.Categories,
		TransitiveCategories: info.TransitiveCategories,
		Confidence:           info.Confidence,
		PurityScore:          info.PurityScore,
		Reasons:              []string{},
	}

	// Add reasons for impurity
	combined := info.Categories | info.TransitiveCategories

	if combined&types.SideEffectParamWrite != 0 {
		report.Reasons = append(report.Reasons, "Mutates parameter")
	}
	if combined&types.SideEffectReceiverWrite != 0 {
		report.Reasons = append(report.Reasons, "Mutates receiver/this")
	}
	if combined&types.SideEffectGlobalWrite != 0 {
		report.Reasons = append(report.Reasons, "Writes to global state")
	}
	if combined&types.SideEffectClosureWrite != 0 {
		report.Reasons = append(report.Reasons, "Writes to captured closure variable")
	}
	if combined&types.SideEffectIO != 0 {
		report.Reasons = append(report.Reasons, "Performs I/O")
	}
	if combined&types.SideEffectNetwork != 0 {
		report.Reasons = append(report.Reasons, "Performs network operations")
	}
	if combined&types.SideEffectDatabase != 0 {
		report.Reasons = append(report.Reasons, "Performs database operations")
	}
	if combined&types.SideEffectThrow != 0 {
		report.Reasons = append(report.Reasons, "Can throw/panic")
	}
	if combined&types.SideEffectChannel != 0 {
		report.Reasons = append(report.Reasons, "Uses channel operations")
	}
	if combined&types.SideEffectExternalCall != 0 {
		report.Reasons = append(report.Reasons, "Calls external/unknown function")
	}
	if combined&types.SideEffectDynamicCall != 0 {
		report.Reasons = append(report.Reasons, "Uses dynamic dispatch")
	}
	if combined&types.SideEffectUncertain != 0 {
		report.Reasons = append(report.Reasons, "Analysis uncertain")
	}

	return report
}

// PurityReport is a human-readable report of a function's purity
type PurityReport struct {
	SymbolID             types.SymbolID
	FunctionName         string
	IsPure               bool
	LocalCategories      types.SideEffectCategory
	TransitiveCategories types.SideEffectCategory
	Confidence           types.PurityConfidence
	PurityScore          float64
	Reasons              []string
}

// GetImpureFunctions returns all functions with detected side effects
func (sep *SideEffectPropagator) GetImpureFunctions() []types.SymbolID {
	var impure []types.SymbolID

	for symbolID, info := range sep.sideEffectInfo {
		if !info.IsPure {
			impure = append(impure, symbolID)
		}
	}

	return impure
}

// GetPureFunctions returns all functions detected as pure
func (sep *SideEffectPropagator) GetPureFunctions() []types.SymbolID {
	var pure []types.SymbolID

	for symbolID, info := range sep.sideEffectInfo {
		if info.IsPure {
			pure = append(pure, symbolID)
		}
	}

	return pure
}

// GetFunctionsByPurity returns functions grouped by their purity confidence
func (sep *SideEffectPropagator) GetFunctionsByPurity() map[types.PurityConfidence][]types.SymbolID {
	result := make(map[types.PurityConfidence][]types.SymbolID)

	for symbolID, info := range sep.sideEffectInfo {
		conf := info.Confidence
		result[conf] = append(result[conf], symbolID)
	}

	return result
}
