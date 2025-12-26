package types

// Side Effect Analysis Types
//
// This package provides types for tracking and analyzing side effects in code.
// The analysis uses a TWO-PHASE approach with PURITY LEVELS:
//
// Phase 1: Internal Purity Analysis (per-function)
//   - Analyze each function independently
//   - Classify into 5 purity levels based on direct side effects
//   - Record ALL function calls (including unknown)
//
// Phase 2: Transitive Purity Resolution (cross-function)
//   - Resolve function calls using complete symbol table
//   - Build call graph (including cross-file calls)
//   - Propagate effects through call graph (fixed-point iteration)
//   - Upgrade Level 2 functions to Level 1 if all dependencies pure
//
// Purity Levels:
//   1. Pure - No side effects, all dependencies pure (referentially transparent)
//   2. Internally Pure - No direct side effects, but calls unknown/impure functions
//   3. Object State - Mutates receiver/parameters only (object-local mutations)
//   4. Module/Global - Mutates module-level or global state
//   5. External Dependency - I/O, network, database (affects external world)

// SideEffectCategory represents categories of side effects as a bitfield.
// Multiple categories can be combined with bitwise OR.
type SideEffectCategory uint32

const (
	// SideEffectNone indicates no detected side effects (pure function candidate)
	SideEffectNone SideEffectCategory = 0

	// Write effects - mutations to state
	SideEffectParamWrite    SideEffectCategory = 1 << iota // Writes to function parameters
	SideEffectReceiverWrite                                // Writes to receiver/this/self
	SideEffectGlobalWrite                                  // Writes to global/module-level state
	SideEffectClosureWrite                                 // Writes to captured closure variables
	SideEffectFieldWrite                                   // Writes to object fields (general)

	// I/O effects
	SideEffectIO       // Any I/O operation (file, network, console)
	SideEffectDatabase // Database operations
	SideEffectNetwork  // Network operations specifically

	// Control flow effects
	SideEffectThrow   // Can throw/panic/raise
	SideEffectChannel // Channel send/receive (Go)
	SideEffectAsync   // Async operations that may have deferred effects

	// Uncertainty markers (conservative flags)
	SideEffectExternalCall  // Calls external/unknown function
	SideEffectDynamicCall   // Dynamic dispatch (interface, function pointer)
	SideEffectReflection    // Uses reflection
	SideEffectUncertain     // Analysis couldn't determine (assume side effects)
	SideEffectIndirectWrite // Writes through pointer/reference that may alias
)

// String returns a human-readable representation of the side effect categories
func (s SideEffectCategory) String() string {
	if s == SideEffectNone {
		return "none"
	}

	var parts []string
	if s&SideEffectParamWrite != 0 {
		parts = append(parts, "param-write")
	}
	if s&SideEffectReceiverWrite != 0 {
		parts = append(parts, "receiver-write")
	}
	if s&SideEffectGlobalWrite != 0 {
		parts = append(parts, "global-write")
	}
	if s&SideEffectClosureWrite != 0 {
		parts = append(parts, "closure-write")
	}
	if s&SideEffectFieldWrite != 0 {
		parts = append(parts, "field-write")
	}
	if s&SideEffectIO != 0 {
		parts = append(parts, "io")
	}
	if s&SideEffectDatabase != 0 {
		parts = append(parts, "database")
	}
	if s&SideEffectNetwork != 0 {
		parts = append(parts, "network")
	}
	if s&SideEffectThrow != 0 {
		parts = append(parts, "throw")
	}
	if s&SideEffectChannel != 0 {
		parts = append(parts, "channel")
	}
	if s&SideEffectAsync != 0 {
		parts = append(parts, "async")
	}
	if s&SideEffectExternalCall != 0 {
		parts = append(parts, "external-call")
	}
	if s&SideEffectDynamicCall != 0 {
		parts = append(parts, "dynamic-call")
	}
	if s&SideEffectReflection != 0 {
		parts = append(parts, "reflection")
	}
	if s&SideEffectUncertain != 0 {
		parts = append(parts, "uncertain")
	}
	if s&SideEffectIndirectWrite != 0 {
		parts = append(parts, "indirect-write")
	}

	if len(parts) == 0 {
		return "unknown"
	}

	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += "|" + parts[i]
	}
	return result
}

// HasWriteEffects returns true if any write-related side effects are present
func (s SideEffectCategory) HasWriteEffects() bool {
	writeEffects := SideEffectParamWrite | SideEffectReceiverWrite |
		SideEffectGlobalWrite | SideEffectClosureWrite |
		SideEffectFieldWrite | SideEffectIndirectWrite
	return s&writeEffects != 0
}

// HasIOEffects returns true if any I/O-related side effects are present
func (s SideEffectCategory) HasIOEffects() bool {
	ioEffects := SideEffectIO | SideEffectDatabase | SideEffectNetwork
	return s&ioEffects != 0
}

// HasUncertainty returns true if any uncertainty markers are present
func (s SideEffectCategory) HasUncertainty() bool {
	uncertainEffects := SideEffectExternalCall | SideEffectDynamicCall |
		SideEffectReflection | SideEffectUncertain
	return s&uncertainEffects != 0
}

// IsPure returns true only if no side effects AND no uncertainty
func (s SideEffectCategory) IsPure() bool {
	return s == SideEffectNone
}

// PurityLevel represents the purity classification of a function
// Based on two-phase analysis: internal effects + transitive dependencies
type PurityLevel int

const (
	// PurityLevelPure - Level 1: True purity
	// No side effects, all dependencies are pure (referentially transparent)
	// Can only be achieved after Phase 2 resolves all dependencies
	PurityLevelPure PurityLevel = 1

	// PurityLevelInternallyPure - Level 2: Internally pure
	// No direct side effects in function body
	// But calls unknown/unresolved functions (before Phase 2)
	// OR calls functions that are Level 3+ (after Phase 2)
	PurityLevelInternallyPure PurityLevel = 2

	// PurityLevelObjectState - Level 3: Object-local mutations
	// Mutates receiver (methods) or parameters only
	// No global state or I/O
	// Examples: c.value++, array[i] = x
	PurityLevelObjectState PurityLevel = 3

	// PurityLevelModuleGlobal - Level 4: Module/global state
	// Mutates global variables or module-level state
	// No I/O or external dependencies
	// Examples: globalCounter++, cache[key] = value
	PurityLevelModuleGlobal PurityLevel = 4

	// PurityLevelExternalDependency - Level 5: External side effects
	// Performs I/O, network, database operations
	// Affects state outside the program
	// Examples: fmt.Println(), db.Execute(), http.Get()
	PurityLevelExternalDependency PurityLevel = 5
)

// String returns a human-readable representation of the purity level
func (p PurityLevel) String() string {
	switch p {
	case PurityLevelPure:
		return "Pure"
	case PurityLevelInternallyPure:
		return "InternallyPure"
	case PurityLevelObjectState:
		return "ObjectState"
	case PurityLevelModuleGlobal:
		return "ModuleGlobal"
	case PurityLevelExternalDependency:
		return "ExternalDependency"
	default:
		return "Unknown"
	}
}

// ComputePurityLevel determines the purity level based on side effect categories
// This is used in Phase 1 (internal analysis)
func ComputePurityLevel(categories SideEffectCategory, hasUnresolvedCalls bool) PurityLevel {
	// Level 5: External dependencies (I/O, network, database)
	if categories&(SideEffectIO|SideEffectNetwork|SideEffectDatabase) != 0 {
		return PurityLevelExternalDependency
	}

	// Level 4: Module/global state mutations
	if categories&(SideEffectGlobalWrite|SideEffectClosureWrite) != 0 {
		return PurityLevelModuleGlobal
	}

	// Level 3: Object-local state mutations
	if categories&(SideEffectParamWrite|SideEffectReceiverWrite|SideEffectFieldWrite) != 0 {
		return PurityLevelObjectState
	}

	// Level 2: Internally pure but has unresolved/external calls
	if hasUnresolvedCalls || categories.HasUncertainty() {
		return PurityLevelInternallyPure
	}

	// Level 1: Pure (no side effects, no unresolved calls)
	// Note: This is tentative in Phase 1, needs Phase 2 confirmation
	return PurityLevelPure
}

// PurityConfidence represents confidence level in purity classification
type PurityConfidence uint8

const (
	// ConfidenceNone - cannot determine purity
	ConfidenceNone PurityConfidence = iota

	// ConfidenceLow - weak evidence, likely has side effects
	ConfidenceLow

	// ConfidenceMedium - some evidence but uncertainty remains
	ConfidenceMedium

	// ConfidenceHigh - strong evidence, analysis is reliable
	ConfidenceHigh

	// ConfidenceProven - proven pure/impure (e.g., user annotation, known stdlib)
	ConfidenceProven
)

func (c PurityConfidence) String() string {
	switch c {
	case ConfidenceNone:
		return "none"
	case ConfidenceLow:
		return "low"
	case ConfidenceMedium:
		return "medium"
	case ConfidenceHigh:
		return "high"
	case ConfidenceProven:
		return "proven"
	default:
		return "unknown"
	}
}

// AccessType distinguishes reads from writes
type AccessType uint8

const (
	AccessRead  AccessType = 1
	AccessWrite AccessType = 2
)

func (a AccessType) String() string {
	switch a {
	case AccessRead:
		return "read"
	case AccessWrite:
		return "write"
	default:
		return "unknown"
	}
}

// AccessTarget represents what is being accessed
type AccessTarget uint8

const (
	AccessTargetLocal     AccessTarget = iota // Local variable
	AccessTargetParameter                     // Function parameter
	AccessTargetReceiver                      // Method receiver (this/self)
	AccessTargetGlobal                        // Global/module-level variable
	AccessTargetClosure                       // Captured closure variable
	AccessTargetField                         // Object/struct field
	AccessTargetIndex                         // Array/slice/map index
	AccessTargetUnknown                       // Cannot determine
)

func (t AccessTarget) String() string {
	switch t {
	case AccessTargetLocal:
		return "local"
	case AccessTargetParameter:
		return "parameter"
	case AccessTargetReceiver:
		return "receiver"
	case AccessTargetGlobal:
		return "global"
	case AccessTargetClosure:
		return "closure"
	case AccessTargetField:
		return "field"
	case AccessTargetIndex:
		return "index"
	case AccessTargetUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

// FieldAccess tracks a single access to a variable/field
type FieldAccess struct {
	// What is being accessed (canonical form: "param:user.Name", "global:counter")
	Target string

	// Classification of the target
	TargetType AccessTarget

	// Read or write
	Type AccessType

	// Location in source
	Line   int
	Column int

	// Order within function (0, 1, 2, ...) for sequence analysis
	SeqNum int

	// The base identifier (e.g., "user" in "user.Name")
	BaseIdentifier string

	// Field path (e.g., ["Name"] in "user.Name", ["addr", "city"] in "user.addr.city")
	FieldPath []string
}

// AccessPatternType classifies the overall read/write pattern of a function
type AccessPatternType uint8

const (
	// PatternPure - no writes at all (reads only or no accesses)
	PatternPure AccessPatternType = iota

	// PatternReadThenWrite - all reads occur before all writes (clean pattern)
	PatternReadThenWrite

	// PatternWriteOnly - only writes, no reads (initializer pattern)
	PatternWriteOnly

	// PatternWriteThenRead - writes before reads to same target (suspicious)
	PatternWriteThenRead

	// PatternInterleaved - mixed read/write ordering (complex/suspicious)
	PatternInterleaved

	// PatternUnknown - cannot determine pattern
	PatternUnknown
)

func (p AccessPatternType) String() string {
	switch p {
	case PatternPure:
		return "pure"
	case PatternReadThenWrite:
		return "read-then-write"
	case PatternWriteOnly:
		return "write-only"
	case PatternWriteThenRead:
		return "write-then-read"
	case PatternInterleaved:
		return "interleaved"
	case PatternUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

// IsClean returns true if the pattern is considered clean/expected
func (p AccessPatternType) IsClean() bool {
	return p == PatternPure || p == PatternReadThenWrite || p == PatternWriteOnly
}

// TargetAccessPattern tracks access pattern to a single variable/field
type TargetAccessPattern struct {
	// The target being accessed
	Target string

	// Classification of the target
	TargetType AccessTarget

	// Pattern for this specific target
	Pattern AccessPatternType

	// Access counts
	ReadCount  int
	WriteCount int

	// Sequence string: "RRW" = read, read, write; "WRR" = write, read, read
	Sequence string

	// Lines of first read and first write (for violation reporting)
	FirstReadLine  int
	FirstWriteLine int

	// Line of first read after a write (for WriteThenRead detection)
	FirstReadAfterWriteLine int
}

// ViolationType categorizes concerning access patterns
type ViolationType uint8

const (
	// ViolationWriteBeforeRead - write to target before reading from it
	ViolationWriteBeforeRead ViolationType = iota

	// ViolationReadAfterWrite - read from aggregate after write to part of it
	ViolationReadAfterWrite

	// ViolationInterleavedAccess - R-W-R or W-R-W pattern on same target
	ViolationInterleavedAccess

	// ViolationSelfInterference - function result depends on its own mutation
	ViolationSelfInterference

	// ViolationMutateParameter - writes to a function parameter
	ViolationMutateParameter

	// ViolationMutateReceiver - writes to method receiver
	ViolationMutateReceiver
)

func (v ViolationType) String() string {
	switch v {
	case ViolationWriteBeforeRead:
		return "write-before-read"
	case ViolationReadAfterWrite:
		return "read-after-write"
	case ViolationInterleavedAccess:
		return "interleaved-access"
	case ViolationSelfInterference:
		return "self-interference"
	case ViolationMutateParameter:
		return "mutate-parameter"
	case ViolationMutateReceiver:
		return "mutate-receiver"
	default:
		return "unknown"
	}
}

// PatternViolation flags a specific concerning pattern
type PatternViolation struct {
	Type ViolationType

	// The target involved
	Target string

	// Location information
	Line      int // Primary line (usually the problematic access)
	ReadLine  int // Line of the read involved
	WriteLine int // Line of the write involved

	// Human-readable description
	Description string

	// Severity from 0.0 (informational) to 1.0 (definitely problematic)
	Severity float64
}

// AccessPattern summarizes the read/write pattern for a function
type AccessPattern struct {
	// All detected accesses in order
	Accesses []FieldAccess

	// Overall pattern classification
	Pattern AccessPatternType

	// Per-target analysis
	TargetPatterns map[string]*TargetAccessPattern

	// Detected violations/concerns
	Violations []PatternViolation

	// Summary statistics
	TotalReads      int
	TotalWrites     int
	UniqueTargets   int
	ParameterWrites int
	ReceiverWrites  int
	GlobalWrites    int
	ClosureWrites   int
}

// HasViolations returns true if any violations were detected
func (ap *AccessPattern) HasViolations() bool {
	return len(ap.Violations) > 0
}

// MaxSeverity returns the maximum severity among all violations
func (ap *AccessPattern) MaxSeverity() float64 {
	max := 0.0
	for _, v := range ap.Violations {
		if v.Severity > max {
			max = v.Severity
		}
	}
	return max
}

// SideEffectInfo captures complete side-effect analysis for a function
type SideEffectInfo struct {
	// Function identification
	FunctionName string
	FilePath     string
	StartLine    int
	EndLine      int

	// Bitfield of detected side effect categories
	Categories SideEffectCategory

	// Confidence in the analysis
	Confidence PurityConfidence

	// Purity level (two-phase analysis result)
	PurityLevel PurityLevel

	// Purity classification: track WHAT affects purity
	PurityClassification PurityClassification

	// Access pattern analysis
	AccessPattern *AccessPattern

	// Specific details about detected effects
	ParameterWrites []ParameterWriteInfo
	GlobalWrites    []GlobalWriteInfo
	ExternalCalls   []ExternalCallInfo
	ThrowSites      []ThrowSiteInfo

	// Unresolved function calls (for Phase 2 propagation)
	UnresolvedCalls []UnresolvedCallInfo

	// Error handling analysis
	ErrorHandling *ErrorHandlingInfo

	// Transitive effects (from callees) - populated by propagation
	TransitiveCategories SideEffectCategory
	TransitiveConfidence PurityConfidence

	// Combined assessment
	IsPure           bool    // True ONLY if locally pure AND all callees pure
	PurityScore      float64 // 1.0 = definitely pure, 0.0 = definitely impure
	PurityConfidence float64 // Confidence in the purity score (0.0-1.0)

	// Reasons for impurity (for debugging/reporting)
	ImpurityReasons []string
}

// PurityClassification tracks WHAT affects purity (fine-grained analysis)
// This is a vector-based classification instead of a single level
type PurityClassification struct {
	// Parameters mutated (vector of parameter indices)
	// e.g., [0, 2] means parameters 0 and 2 are mutated
	MutatedParameters []int

	// Object/receiver mutated
	MutatesReceiver bool

	// Globals mutated (list of global variable names)
	MutatedGlobals []string

	// Closures mutated (list of captured variable names)
	MutatedClosures []string

	// Dependent functions and their purity status
	// Maps function name to its purity (true = pure, false = impure)
	DependentFunctions map[string]bool

	// I/O operations performed
	PerformsIO bool

	// Network operations performed
	PerformsNetwork bool

	// Database operations performed
	PerformsDatabase bool

	// Exception/panic operations
	CanThrow bool
}

// ParameterWriteInfo details a write to a function parameter
type ParameterWriteInfo struct {
	ParameterName  string
	ParameterIndex int
	Line           int
	Column         int
	FieldPath      []string // Empty for direct write, populated for field access
	IsPointer      bool     // Write through pointer dereference
}

// GlobalWriteInfo details a write to global state
type GlobalWriteInfo struct {
	GlobalName string
	Line       int
	Column     int
	FieldPath  []string
	IsPackage  bool // Package-level vs module-level
}

// ExternalCallInfo details a call to an external/unknown function
type ExternalCallInfo struct {
	FunctionName string
	Line         int
	Column       int
	IsMethod     bool
	ReceiverType string // For method calls
	Package      string // If determinable
	Reason       string // Why it's considered external
}

// UnresolvedCallInfo tracks a function call that needs Phase 2 resolution
type UnresolvedCallInfo struct {
	FunctionName string
	Qualifier    string // Package/module or receiver type
	IsMethod     bool
	Line         int
	Column       int
}

// ThrowSiteInfo details a throw/panic/raise site
type ThrowSiteInfo struct {
	Type   string // "panic", "throw", "raise", etc.
	Line   int
	Column int
}

// ErrorHandlingInfo captures exception safety characteristics
type ErrorHandlingInfo struct {
	// Can the function throw/panic?
	CanThrow bool

	// Does it return errors? (Go pattern)
	ReturnsError bool

	// Exception safety level
	ExceptionNeutral bool // No cleanup needed (no resources acquired)
	ExceptionSafe    bool // Cleanup guaranteed (defer, try-finally present)

	// Details
	DeferCount       int   // Number of defer statements
	TryFinallyCount  int   // Number of try-finally blocks
	ThrowCount       int   // Number of throw/panic sites
	ErrorReturnLines []int // Lines with error returns
}

// NewSideEffectInfo creates a new SideEffectInfo with default values
func NewSideEffectInfo() *SideEffectInfo {
	return &SideEffectInfo{
		Categories:  SideEffectNone,
		Confidence:  ConfidenceNone,
		PurityLevel: PurityLevelInternallyPure, // Conservative default (Phase 1)
		PurityClassification: PurityClassification{
			MutatedParameters:  make([]int, 0),
			MutatedGlobals:     make([]string, 0),
			MutatedClosures:    make([]string, 0),
			DependentFunctions: make(map[string]bool),
			MutatesReceiver:    false,
			PerformsIO:         false,
			PerformsNetwork:    false,
			PerformsDatabase:   false,
			CanThrow:           false,
		},
		AccessPattern:    nil,
		ParameterWrites:  make([]ParameterWriteInfo, 0),
		GlobalWrites:     make([]GlobalWriteInfo, 0),
		ExternalCalls:    make([]ExternalCallInfo, 0),
		ThrowSites:       make([]ThrowSiteInfo, 0),
		UnresolvedCalls:  make([]UnresolvedCallInfo, 0),
		ErrorHandling:    nil,
		IsPure:           false, // Conservative default
		PurityScore:      0.0,   // Conservative default
		PurityConfidence: 0.0,
		ImpurityReasons:  make([]string, 0),
	}
}

// AddCategory adds a side effect category
func (si *SideEffectInfo) AddCategory(cat SideEffectCategory) {
	si.Categories |= cat
}

// AddImpurityReason adds a reason why the function is not pure
func (si *SideEffectInfo) AddImpurityReason(reason string) {
	si.ImpurityReasons = append(si.ImpurityReasons, reason)
}

// ComputePurityScore calculates the purity score and level based on categories and confidence.
// This uses a CONSERVATIVE approach: any uncertainty results in low purity score.
// This is called after Phase 1 (internal analysis) and after Phase 2 (propagation).
func (si *SideEffectInfo) ComputePurityScore() {
	// Check if Phase 2 has run (TransitiveCategories populated or TransitiveConfidence set)
	phase2Completed := si.TransitiveCategories != SideEffectNone || si.TransitiveConfidence != ConfidenceNone

	// Only recompute purity level if Phase 2 hasn't set it explicitly
	if !phase2Completed {
		// Phase 1: Compute initial purity level
		hasUnresolvedCalls := len(si.UnresolvedCalls) > 0
		si.PurityLevel = ComputePurityLevel(si.Categories, hasUnresolvedCalls)
	}
	// Otherwise, Phase 2 has already set PurityLevel in updatePurityAssessment

	// Start with assumption of impure
	si.IsPure = false
	si.PurityScore = 0.0
	si.PurityConfidence = 0.0

	// Compute purity score based on level
	switch si.PurityLevel {
	case PurityLevelPure:
		// Level 1: Truly pure (no effects, no unresolved calls OR all resolved to pure)
		si.IsPure = true
		si.PurityScore = 1.0
		si.PurityConfidence = float64(si.Confidence) / float64(ConfidenceProven)

	case PurityLevelInternallyPure:
		// Level 2: No direct effects, but has unresolved calls
		// Cannot claim purity until Phase 2 resolves dependencies
		si.IsPure = false
		si.PurityScore = 0.8 // High potential, pending resolution
		si.PurityConfidence = 0.6
		if len(si.UnresolvedCalls) > 0 && !phase2Completed {
			si.AddImpurityReason("has unresolved function calls (pending Phase 2)")
		}

	case PurityLevelObjectState:
		// Level 3: Object-local mutations only
		si.IsPure = false
		si.PurityScore = 0.6
		si.PurityConfidence = 0.8

	case PurityLevelModuleGlobal:
		// Level 4: Global state mutations
		si.IsPure = false
		si.PurityScore = 0.3
		si.PurityConfidence = 0.8

	case PurityLevelExternalDependency:
		// Level 5: I/O and external effects
		si.IsPure = false
		si.PurityScore = 0.0
		si.PurityConfidence = 0.9
	}

	// If we have transitive side effects (from Phase 2), override
	if si.TransitiveCategories != SideEffectNone {
		si.IsPure = false
		si.PurityScore = 0.0
		si.PurityConfidence = float64(si.TransitiveConfidence) / float64(ConfidenceProven)
		si.AddImpurityReason("callee has side effects")
	}
}

// KnownPureFunction represents a function known to be pure from external knowledge
type KnownPureFunction struct {
	Package  string // e.g., "strings", "math", "fmt"
	Function string // e.g., "ToLower", "Sqrt"
	Language string // e.g., "go", "javascript", "python"
}

// KnownIOFunction represents a function known to perform I/O
type KnownIOFunction struct {
	Package  string
	Function string
	Language string
	IOType   SideEffectCategory // SideEffectIO, SideEffectNetwork, etc.
}
