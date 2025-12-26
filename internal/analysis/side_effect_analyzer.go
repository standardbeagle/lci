package analysis

import (
	"sort"
	"strings"

	"github.com/standardbeagle/lci/internal/types"
)

// SideEffectAnalyzer performs conservative side-effect analysis on functions.
//
// Design principle: If we say it's pure, it IS pure.
// - Unknown functions are marked with SideEffectExternalCall
// - Any uncertainty marks the function as potentially impure
// - Only well-analyzed local code can be marked pure
type SideEffectAnalyzer struct {
	// Language being analyzed (affects known function lookups)
	language string

	// Current function being analyzed
	currentFunc *FunctionAnalysisContext

	// Results for all analyzed functions
	results map[string]*types.SideEffectInfo // keyed by "file:line:col"

	// Configuration
	config *SideEffectAnalyzerConfig
}

// FunctionAnalysisContext tracks state while analyzing a single function
type FunctionAnalysisContext struct {
	// Function identity
	Name      string
	File      string
	StartLine int
	EndLine   int

	// Parameters (names to track for parameter writes)
	Parameters   map[string]int // name -> parameter index
	ReceiverName string         // "this", "self", or Go receiver name
	ReceiverType string         // Type of receiver (for methods)

	// Scope tracking for closure detection
	LocalVariables map[string]int   // name -> declaration line
	ScopeDepth     int              // Current nesting depth
	OuterScopes    []map[string]int // Variables from enclosing scopes (closures)

	// Access tracking
	Accesses []types.FieldAccess
	SeqNum   int // Next sequence number

	// Detected effects
	SideEffects types.SideEffectCategory

	// External calls (truly external: I/O, network, etc.)
	ExternalCalls []types.ExternalCallInfo

	// Unresolved function calls (need Phase 2 resolution)
	UnresolvedCalls []types.UnresolvedCallInfo

	// Throw sites
	ThrowSites []types.ThrowSiteInfo

	// Error handling
	DeferCount      int
	TryFinallyCount int
	ReturnsError    bool

	// Impurity reasons (for debugging)
	ImpurityReasons []string
}

// SideEffectAnalyzerConfig controls analyzer behavior
type SideEffectAnalyzerConfig struct {
	// TrustAnnotations: if true, @pure/@side-effect annotations override analysis
	TrustAnnotations bool

	// StrictMode: if true, any uncertainty marks function as impure
	// (recommended for high-signal analysis)
	StrictMode bool

	// TrackFieldAccess: if true, track field-level access patterns
	TrackFieldAccess bool

	// MaxAccessesPerFunction: limit to prevent memory issues on huge functions
	MaxAccessesPerFunction int
}

// DefaultSideEffectAnalyzerConfig returns conservative default settings
func DefaultSideEffectAnalyzerConfig() *SideEffectAnalyzerConfig {
	return &SideEffectAnalyzerConfig{
		TrustAnnotations:       true,
		StrictMode:             true, // Conservative by default
		TrackFieldAccess:       true,
		MaxAccessesPerFunction: 1000,
	}
}

// NewSideEffectAnalyzer creates a new analyzer for the given language
func NewSideEffectAnalyzer(language string, config *SideEffectAnalyzerConfig) *SideEffectAnalyzer {
	if config == nil {
		config = DefaultSideEffectAnalyzerConfig()
	}
	return &SideEffectAnalyzer{
		language: language,
		results:  make(map[string]*types.SideEffectInfo),
		config:   config,
	}
}

// BeginFunction starts analysis of a new function
func (sa *SideEffectAnalyzer) BeginFunction(name, file string, startLine, endLine int) {
	sa.currentFunc = &FunctionAnalysisContext{
		Name:            name,
		File:            file,
		StartLine:       startLine,
		EndLine:         endLine,
		Parameters:      make(map[string]int),
		LocalVariables:  make(map[string]int),
		OuterScopes:     make([]map[string]int, 0),
		Accesses:        make([]types.FieldAccess, 0, 32),
		ExternalCalls:   make([]types.ExternalCallInfo, 0),
		UnresolvedCalls: make([]types.UnresolvedCallInfo, 0),
		ThrowSites:      make([]types.ThrowSiteInfo, 0),
		ImpurityReasons: make([]string, 0),
	}
}

// EndFunction completes analysis of current function and returns results
func (sa *SideEffectAnalyzer) EndFunction() *types.SideEffectInfo {
	if sa.currentFunc == nil {
		return nil
	}

	ctx := sa.currentFunc
	info := types.NewSideEffectInfo()

	// Copy function identification
	info.FunctionName = ctx.Name
	info.FilePath = ctx.File
	info.StartLine = ctx.StartLine
	info.EndLine = ctx.EndLine

	// Copy detected effects
	info.Categories = ctx.SideEffects
	info.ExternalCalls = ctx.ExternalCalls
	info.UnresolvedCalls = ctx.UnresolvedCalls
	info.ThrowSites = ctx.ThrowSites
	info.ImpurityReasons = ctx.ImpurityReasons

	// Analyze access patterns
	info.AccessPattern = sa.analyzeAccessPattern(ctx.Accesses)

	// Build error handling info
	canThrow := len(ctx.ThrowSites) > 0 || ctx.SideEffects&types.SideEffectThrow != 0
	info.ErrorHandling = &types.ErrorHandlingInfo{
		CanThrow:         canThrow,
		ReturnsError:     ctx.ReturnsError,
		ExceptionNeutral: ctx.DeferCount == 0 && ctx.TryFinallyCount == 0 && !canThrow,
		ExceptionSafe:    ctx.DeferCount > 0 || ctx.TryFinallyCount > 0,
		DeferCount:       ctx.DeferCount,
		TryFinallyCount:  ctx.TryFinallyCount,
		ThrowCount:       len(ctx.ThrowSites),
	}

	// Extract parameter writes from access pattern
	paramIndexSet := make(map[int]bool) // Track unique parameter indices
	if info.AccessPattern != nil {
		for _, access := range ctx.Accesses {
			if access.Type == types.AccessWrite && access.TargetType == types.AccessTargetParameter {
				info.ParameterWrites = append(info.ParameterWrites, types.ParameterWriteInfo{
					ParameterName:  access.BaseIdentifier,
					ParameterIndex: ctx.Parameters[access.BaseIdentifier],
					Line:           access.Line,
					Column:         access.Column,
					FieldPath:      access.FieldPath,
				})
				// Track for classification
				paramIndexSet[ctx.Parameters[access.BaseIdentifier]] = true
			}
		}
	}

	// Populate PurityClassification
	sa.populatePurityClassification(ctx, info, paramIndexSet)

	// Determine confidence
	info.Confidence = sa.determineConfidence(ctx, info)

	// Compute final purity score
	info.ComputePurityScore()

	// Store result
	key := ctx.File + ":" + itoa(ctx.StartLine) + ":" + "0"
	sa.results[key] = info

	sa.currentFunc = nil
	return info
}

// AddParameter registers a function parameter for tracking
func (sa *SideEffectAnalyzer) AddParameter(name string, index int) {
	if sa.currentFunc != nil {
		sa.currentFunc.Parameters[name] = index
	}
}

// SetReceiver sets the receiver name and type for method analysis
func (sa *SideEffectAnalyzer) SetReceiver(name, receiverType string) {
	if sa.currentFunc != nil {
		sa.currentFunc.ReceiverName = name
		sa.currentFunc.ReceiverType = receiverType
	}
}

// AddLocalVariable registers a local variable declaration
func (sa *SideEffectAnalyzer) AddLocalVariable(name string, line int) {
	if sa.currentFunc != nil {
		sa.currentFunc.LocalVariables[name] = line
	}
}

// EnterScope pushes current locals to outer scope (for closure tracking)
func (sa *SideEffectAnalyzer) EnterScope() {
	if sa.currentFunc != nil {
		// Copy current locals to outer scope
		outerScope := make(map[string]int, len(sa.currentFunc.LocalVariables))
		for k, v := range sa.currentFunc.LocalVariables {
			outerScope[k] = v
		}
		sa.currentFunc.OuterScopes = append(sa.currentFunc.OuterScopes, outerScope)
		sa.currentFunc.ScopeDepth++
	}
}

// ExitScope pops scope
func (sa *SideEffectAnalyzer) ExitScope() {
	if sa.currentFunc != nil && len(sa.currentFunc.OuterScopes) > 0 {
		sa.currentFunc.OuterScopes = sa.currentFunc.OuterScopes[:len(sa.currentFunc.OuterScopes)-1]
		sa.currentFunc.ScopeDepth--
	}
}

// RecordAccess records a read or write access
func (sa *SideEffectAnalyzer) RecordAccess(
	identifier string,
	fieldPath []string,
	accessType types.AccessType,
	line, column int,
) {
	if sa.currentFunc == nil {
		return
	}

	// Check access limit
	if len(sa.currentFunc.Accesses) >= sa.config.MaxAccessesPerFunction {
		return
	}

	// Classify the target
	targetType := sa.classifyTarget(identifier)

	// Build canonical target string
	target := sa.buildTargetString(identifier, fieldPath, targetType)

	access := types.FieldAccess{
		Target:         target,
		TargetType:     targetType,
		Type:           accessType,
		Line:           line,
		Column:         column,
		SeqNum:         sa.currentFunc.SeqNum,
		BaseIdentifier: identifier,
		FieldPath:      fieldPath,
	}

	sa.currentFunc.Accesses = append(sa.currentFunc.Accesses, access)
	sa.currentFunc.SeqNum++

	// Record side effects for writes
	if accessType == types.AccessWrite {
		sa.recordWriteSideEffect(targetType, identifier, line)
	}
}

// RecordFunctionCall records a function call and checks if it's known pure/impure
// Phase 1: Record all calls, classify as known pure/impure or unresolved
// Phase 2 will resolve unresolved calls using complete symbol table
func (sa *SideEffectAnalyzer) RecordFunctionCall(
	funcName string,
	qualifier string, // package/module or receiver type
	isMethod bool,
	line, column int,
) {
	if sa.currentFunc == nil {
		return
	}

	// Build qualified name
	qualifiedName := funcName
	if qualifier != "" {
		qualifiedName = qualifier + "." + funcName
	}

	// Check known functions (built-ins, stdlib)
	isPure, confidence := CheckFunctionPurity(sa.language, qualifiedName)

	if confidence == types.ConfidenceProven {
		if !isPure {
			// Known impure - get specific effects (I/O, network, etc.)
			effects := GetKnownSideEffects(sa.language, qualifiedName)
			sa.currentFunc.SideEffects |= effects

			// Only add to ExternalCalls if it's truly external (I/O, network, etc.)
			if effects&(types.SideEffectIO|types.SideEffectNetwork|types.SideEffectDatabase) != 0 {
				sa.currentFunc.ExternalCalls = append(sa.currentFunc.ExternalCalls, types.ExternalCallInfo{
					FunctionName: funcName,
					Line:         line,
					Column:       column,
					IsMethod:     isMethod,
					ReceiverType: qualifier,
					Package:      qualifier,
					Reason:       "known external function",
				})
			}

			sa.currentFunc.ImpurityReasons = append(sa.currentFunc.ImpurityReasons,
				"calls known impure function: "+qualifiedName)
		}
		// Known pure - no effect recorded
		return
	}

	// Unknown function - record for Phase 2 resolution
	// Do NOT mark as SideEffectExternalCall (that's for true external effects)
	sa.currentFunc.UnresolvedCalls = append(sa.currentFunc.UnresolvedCalls, types.UnresolvedCallInfo{
		FunctionName: funcName,
		Qualifier:    qualifier,
		IsMethod:     isMethod,
		Line:         line,
		Column:       column,
	})

	// Note: We do NOT add to ImpurityReasons here because unresolved calls
	// are expected and will be resolved in Phase 2. This allows functions
	// to be classified as Level 2 (InternallyPure) rather than impure.
}

// RecordDynamicCall records a call through interface/function pointer
func (sa *SideEffectAnalyzer) RecordDynamicCall(description string, line, column int) {
	if sa.currentFunc == nil {
		return
	}

	sa.currentFunc.SideEffects |= types.SideEffectDynamicCall
	sa.currentFunc.ExternalCalls = append(sa.currentFunc.ExternalCalls, types.ExternalCallInfo{
		FunctionName: description,
		Line:         line,
		Column:       column,
		Reason:       "dynamic dispatch - cannot determine target",
	})
	sa.currentFunc.ImpurityReasons = append(sa.currentFunc.ImpurityReasons,
		"dynamic call at line "+itoa(line)+": "+description)
}

// RecordThrow records a throw/panic/raise statement
func (sa *SideEffectAnalyzer) RecordThrow(throwType string, line, column int) {
	if sa.currentFunc == nil {
		return
	}

	sa.currentFunc.SideEffects |= types.SideEffectThrow
	sa.currentFunc.ThrowSites = append(sa.currentFunc.ThrowSites, types.ThrowSiteInfo{
		Type:   throwType,
		Line:   line,
		Column: column,
	})
}

// RecordDefer records a defer statement (Go)
func (sa *SideEffectAnalyzer) RecordDefer() {
	if sa.currentFunc != nil {
		sa.currentFunc.DeferCount++
	}
}

// RecordTryFinally records a try-finally block
func (sa *SideEffectAnalyzer) RecordTryFinally() {
	if sa.currentFunc != nil {
		sa.currentFunc.TryFinallyCount++
	}
}

// RecordErrorReturn records that function returns an error
func (sa *SideEffectAnalyzer) RecordErrorReturn() {
	if sa.currentFunc != nil {
		sa.currentFunc.ReturnsError = true
	}
}

// RecordChannelOp records a channel send or receive (Go)
func (sa *SideEffectAnalyzer) RecordChannelOp(line int) {
	if sa.currentFunc != nil {
		sa.currentFunc.SideEffects |= types.SideEffectChannel
		sa.currentFunc.ImpurityReasons = append(sa.currentFunc.ImpurityReasons,
			"channel operation at line "+itoa(line))
	}
}

// classifyTarget determines what kind of variable/field is being accessed
func (sa *SideEffectAnalyzer) classifyTarget(identifier string) types.AccessTarget {
	ctx := sa.currentFunc
	if ctx == nil {
		return types.AccessTargetUnknown
	}

	// Check if it's a parameter
	if _, isParam := ctx.Parameters[identifier]; isParam {
		return types.AccessTargetParameter
	}

	// Check if it's the receiver
	if identifier == ctx.ReceiverName || identifier == "this" || identifier == "self" {
		return types.AccessTargetReceiver
	}

	// Check if it's a local variable
	if _, isLocal := ctx.LocalVariables[identifier]; isLocal {
		return types.AccessTargetLocal
	}

	// Check outer scopes (closure capture)
	for _, scope := range ctx.OuterScopes {
		if _, found := scope[identifier]; found {
			return types.AccessTargetClosure
		}
	}

	// Not found locally - assume global/module-level
	return types.AccessTargetGlobal
}

// buildTargetString creates a canonical target string for pattern analysis
func (sa *SideEffectAnalyzer) buildTargetString(identifier string, fieldPath []string, targetType types.AccessTarget) string {
	var prefix string
	switch targetType {
	case types.AccessTargetParameter:
		prefix = "param:"
	case types.AccessTargetReceiver:
		prefix = "receiver:"
	case types.AccessTargetLocal:
		prefix = "local:"
	case types.AccessTargetGlobal:
		prefix = "global:"
	case types.AccessTargetClosure:
		prefix = "closure:"
	default:
		prefix = "unknown:"
	}

	target := prefix + identifier
	for _, field := range fieldPath {
		target += "." + field
	}
	return target
}

// recordWriteSideEffect adds the appropriate side effect category for a write
func (sa *SideEffectAnalyzer) recordWriteSideEffect(targetType types.AccessTarget, identifier string, line int) {
	ctx := sa.currentFunc
	if ctx == nil {
		return
	}

	switch targetType {
	case types.AccessTargetParameter:
		ctx.SideEffects |= types.SideEffectParamWrite
		ctx.ImpurityReasons = append(ctx.ImpurityReasons,
			"writes to parameter '"+identifier+"' at line "+itoa(line))
	case types.AccessTargetReceiver:
		ctx.SideEffects |= types.SideEffectReceiverWrite
		ctx.ImpurityReasons = append(ctx.ImpurityReasons,
			"writes to receiver at line "+itoa(line))
	case types.AccessTargetGlobal:
		ctx.SideEffects |= types.SideEffectGlobalWrite
		ctx.ImpurityReasons = append(ctx.ImpurityReasons,
			"writes to global '"+identifier+"' at line "+itoa(line))
	case types.AccessTargetClosure:
		ctx.SideEffects |= types.SideEffectClosureWrite
		ctx.ImpurityReasons = append(ctx.ImpurityReasons,
			"writes to closure variable '"+identifier+"' at line "+itoa(line))
	}
}

// analyzeAccessPattern analyzes the collected accesses to find patterns
func (sa *SideEffectAnalyzer) analyzeAccessPattern(accesses []types.FieldAccess) *types.AccessPattern {
	if len(accesses) == 0 {
		return &types.AccessPattern{
			Pattern: types.PatternPure,
		}
	}

	pattern := &types.AccessPattern{
		Accesses:       accesses,
		TargetPatterns: make(map[string]*types.TargetAccessPattern),
	}

	// Group accesses by target
	byTarget := make(map[string][]types.FieldAccess)
	for _, access := range accesses {
		byTarget[access.Target] = append(byTarget[access.Target], access)
	}

	// Analyze each target's pattern
	hasWrite := false
	hasInterleaved := false
	hasWriteThenRead := false

	for target, targetAccesses := range byTarget {
		tap := sa.analyzeTargetAccesses(target, targetAccesses)
		pattern.TargetPatterns[target] = tap

		if tap.WriteCount > 0 {
			hasWrite = true
		}

		switch tap.Pattern {
		case types.PatternInterleaved:
			hasInterleaved = true
			pattern.Violations = append(pattern.Violations, types.PatternViolation{
				Type:        types.ViolationInterleavedAccess,
				Target:      target,
				Line:        tap.FirstWriteLine,
				Description: "interleaved read/write pattern on " + target,
				Severity:    0.7,
			})
		case types.PatternWriteThenRead:
			hasWriteThenRead = true
			pattern.Violations = append(pattern.Violations, types.PatternViolation{
				Type:        types.ViolationWriteBeforeRead,
				Target:      target,
				Line:        tap.FirstWriteLine,
				ReadLine:    tap.FirstReadAfterWriteLine,
				WriteLine:   tap.FirstWriteLine,
				Description: "write before read on " + target,
				Severity:    0.8,
			})
		}

		// Track specific violation types
		if tap.WriteCount > 0 {
			switch targetAccesses[0].TargetType {
			case types.AccessTargetParameter:
				pattern.ParameterWrites++
				pattern.Violations = append(pattern.Violations, types.PatternViolation{
					Type:        types.ViolationMutateParameter,
					Target:      target,
					Line:        tap.FirstWriteLine,
					Description: "mutation of parameter " + target,
					Severity:    0.9,
				})
			case types.AccessTargetReceiver:
				pattern.ReceiverWrites++
				pattern.Violations = append(pattern.Violations, types.PatternViolation{
					Type:        types.ViolationMutateReceiver,
					Target:      target,
					Line:        tap.FirstWriteLine,
					Description: "mutation of receiver",
					Severity:    0.6, // Lower severity - mutating receivers is common
				})
			case types.AccessTargetGlobal:
				pattern.GlobalWrites++
			case types.AccessTargetClosure:
				pattern.ClosureWrites++
			}
		}

		pattern.TotalReads += tap.ReadCount
		pattern.TotalWrites += tap.WriteCount
	}

	pattern.UniqueTargets = len(byTarget)

	// Determine overall pattern
	if !hasWrite {
		pattern.Pattern = types.PatternPure
	} else if hasInterleaved {
		pattern.Pattern = types.PatternInterleaved
	} else if hasWriteThenRead {
		pattern.Pattern = types.PatternWriteThenRead
	} else if pattern.TotalReads == 0 {
		pattern.Pattern = types.PatternWriteOnly
	} else {
		pattern.Pattern = types.PatternReadThenWrite
	}

	return pattern
}

// analyzeTargetAccesses analyzes access pattern for a single target
func (sa *SideEffectAnalyzer) analyzeTargetAccesses(target string, accesses []types.FieldAccess) *types.TargetAccessPattern {
	// Sort by sequence number
	sort.Slice(accesses, func(i, j int) bool {
		return accesses[i].SeqNum < accesses[j].SeqNum
	})

	tap := &types.TargetAccessPattern{
		Target:     target,
		TargetType: accesses[0].TargetType,
	}

	// Build sequence string and gather stats
	var seq strings.Builder
	firstReadSeen := false
	firstWriteSeen := false
	readAfterWrite := false

	for _, access := range accesses {
		if access.Type == types.AccessRead {
			seq.WriteByte('R')
			tap.ReadCount++
			if !firstReadSeen {
				tap.FirstReadLine = access.Line
				firstReadSeen = true
			}
			if firstWriteSeen && !readAfterWrite {
				tap.FirstReadAfterWriteLine = access.Line
				readAfterWrite = true
			}
		} else {
			seq.WriteByte('W')
			tap.WriteCount++
			if !firstWriteSeen {
				tap.FirstWriteLine = access.Line
				firstWriteSeen = true
			}
		}
	}

	tap.Sequence = seq.String()
	tap.Pattern = classifyAccessSequence(tap.Sequence)

	return tap
}

// classifyAccessSequence determines the pattern from a sequence string
func classifyAccessSequence(seq string) types.AccessPatternType {
	if len(seq) == 0 {
		return types.PatternPure
	}

	hasRead := strings.Contains(seq, "R")
	hasWrite := strings.Contains(seq, "W")

	if !hasWrite {
		return types.PatternPure
	}
	if !hasRead {
		return types.PatternWriteOnly
	}

	firstW := strings.Index(seq, "W")
	firstR := strings.Index(seq, "R")
	lastW := strings.LastIndex(seq, "W")
	lastR := strings.LastIndex(seq, "R")

	// All reads before all writes: "RRRWWW"
	if lastR < firstW {
		return types.PatternReadThenWrite
	}

	// All writes before all reads: "WWWRRR"
	if lastW < firstR {
		return types.PatternWriteThenRead
	}

	// Mixed
	return types.PatternInterleaved
}

// populatePurityClassification fills in the fine-grained purity classification
func (sa *SideEffectAnalyzer) populatePurityClassification(ctx *FunctionAnalysisContext, info *types.SideEffectInfo, paramIndexSet map[int]bool) {
	// Mutated parameters (convert set to sorted vector)
	for idx := range paramIndexSet {
		info.PurityClassification.MutatedParameters = append(info.PurityClassification.MutatedParameters, idx)
	}
	// Sort for deterministic output
	sort.Ints(info.PurityClassification.MutatedParameters)

	// Mutated receiver
	info.PurityClassification.MutatesReceiver = info.Categories&types.SideEffectReceiverWrite != 0

	// Mutated globals (extract from GlobalWrites)
	globalSet := make(map[string]bool)
	for _, gw := range info.GlobalWrites {
		globalSet[gw.GlobalName] = true
	}
	for name := range globalSet {
		info.PurityClassification.MutatedGlobals = append(info.PurityClassification.MutatedGlobals, name)
	}
	sort.Strings(info.PurityClassification.MutatedGlobals)

	// Mutated closures (extract from accesses)
	closureSet := make(map[string]bool)
	for _, access := range ctx.Accesses {
		if access.Type == types.AccessWrite && access.TargetType == types.AccessTargetClosure {
			closureSet[access.BaseIdentifier] = true
		}
	}
	for name := range closureSet {
		info.PurityClassification.MutatedClosures = append(info.PurityClassification.MutatedClosures, name)
	}
	sort.Strings(info.PurityClassification.MutatedClosures)

	// Dependent functions (from UnresolvedCalls)
	for _, call := range info.UnresolvedCalls {
		qualifiedName := call.FunctionName
		if call.Qualifier != "" {
			qualifiedName = call.Qualifier + "." + call.FunctionName
		}
		// Mark as unknown purity (will be resolved in Phase 2)
		info.PurityClassification.DependentFunctions[qualifiedName] = false // Conservative: assume impure
	}

	// I/O flags
	info.PurityClassification.PerformsIO = info.Categories&types.SideEffectIO != 0
	info.PurityClassification.PerformsNetwork = info.Categories&types.SideEffectNetwork != 0
	info.PurityClassification.PerformsDatabase = info.Categories&types.SideEffectDatabase != 0
	info.PurityClassification.CanThrow = info.Categories&types.SideEffectThrow != 0 || len(info.ThrowSites) > 0
}

// determineConfidence calculates confidence in the analysis
func (sa *SideEffectAnalyzer) determineConfidence(ctx *FunctionAnalysisContext, info *types.SideEffectInfo) types.PurityConfidence {
	// If we have uncertainty markers, confidence is low
	if info.Categories.HasUncertainty() {
		return types.ConfidenceLow
	}

	// If we have external calls, confidence depends on how many
	if len(ctx.ExternalCalls) > 0 {
		if len(ctx.ExternalCalls) > 5 {
			return types.ConfidenceLow
		}
		return types.ConfidenceMedium
	}

	// If we detected clear side effects, we're confident
	if info.Categories != types.SideEffectNone {
		return types.ConfidenceHigh
	}

	// No side effects detected and no external calls - high confidence
	// (assuming strict mode where unknown calls are flagged)
	if sa.config.StrictMode {
		return types.ConfidenceHigh
	}

	return types.ConfidenceMedium
}

// GetResults returns all analysis results
func (sa *SideEffectAnalyzer) GetResults() map[string]*types.SideEffectInfo {
	return sa.results
}

// GetResult returns analysis for a specific function
func (sa *SideEffectAnalyzer) GetResult(file string, line int) *types.SideEffectInfo {
	key := file + ":" + itoa(line) + ":0"
	return sa.results[key]
}

// Simple itoa to avoid import
func itoa(i int) string {
	if i == 0 {
		return "0"
	}

	neg := i < 0
	if neg {
		i = -i
	}

	var buf [20]byte
	pos := len(buf)

	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}

	if neg {
		pos--
		buf[pos] = '-'
	}

	return string(buf[pos:])
}
