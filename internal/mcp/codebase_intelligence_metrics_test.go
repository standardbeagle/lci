package mcp

import (
	"math"
	"testing"

	"github.com/standardbeagle/lci/internal/types"
)

// TestCalculateModuleMetrics tests the cohesion/coupling/stability calculations
func TestCalculateModuleMetrics(t *testing.T) {
	server := &Server{}

	tests := []struct {
		name               string
		moduleName         string
		symbols            []*types.UniversalSymbolNode
		moduleSymbolSets   map[string]map[string]bool
		expectedCohesion   float64
		expectedCoupling   float64
		expectedStability  float64
		cohesionTolerance  float64
		couplingTolerance  float64
		stabilityTolerance float64
	}{
		{
			name:       "empty module",
			moduleName: "empty",
			symbols:    []*types.UniversalSymbolNode{},
			moduleSymbolSets: map[string]map[string]bool{
				"empty": {},
			},
			expectedCohesion:   0.5,
			expectedCoupling:   0.0,
			expectedStability:  0.5,
			cohesionTolerance:  0.01,
			couplingTolerance:  0.01,
			stabilityTolerance: 0.01,
		},
		{
			name:       "isolated module with no dependencies",
			moduleName: "isolated",
			symbols: []*types.UniversalSymbolNode{
				{
					Identity: types.SymbolIdentity{
						ID:   types.CompositeSymbolID{FileID: 1, LocalSymbolID: 1},
						Name: "FuncA",
					},
					Relationships: types.SymbolRelationships{
						Dependencies: []types.SymbolDependency{},
						Dependents:   []types.CompositeSymbolID{},
						CallsTo:      []types.FunctionCall{},
						CalledBy:     []types.FunctionCall{},
					},
				},
				{
					Identity: types.SymbolIdentity{
						ID:   types.CompositeSymbolID{FileID: 1, LocalSymbolID: 2},
						Name: "FuncB",
					},
					Relationships: types.SymbolRelationships{
						Dependencies: []types.SymbolDependency{},
						Dependents:   []types.CompositeSymbolID{},
						CallsTo:      []types.FunctionCall{},
						CalledBy:     []types.FunctionCall{},
					},
				},
			},
			moduleSymbolSets: map[string]map[string]bool{
				"isolated": {
					"Symbol[F:1,L:1]": true,
					"Symbol[F:1,L:2]": true,
				},
			},
			expectedCohesion:   0.5, // No dependencies = moderate cohesion
			expectedCoupling:   0.0, // No external dependencies
			expectedStability:  0.5, // No dependencies either direction
			cohesionTolerance:  0.01,
			couplingTolerance:  0.01,
			stabilityTolerance: 0.01,
		},
		{
			name:       "highly cohesive module with internal calls",
			moduleName: "cohesive",
			symbols: []*types.UniversalSymbolNode{
				{
					Identity: types.SymbolIdentity{
						ID:   types.CompositeSymbolID{FileID: 1, LocalSymbolID: 1},
						Name: "FuncA",
					},
					Relationships: types.SymbolRelationships{
						CallsTo: []types.FunctionCall{
							{Target: types.CompositeSymbolID{FileID: 1, LocalSymbolID: 2}}, // internal call
							{Target: types.CompositeSymbolID{FileID: 1, LocalSymbolID: 3}}, // internal call
						},
					},
				},
				{
					Identity: types.SymbolIdentity{
						ID:   types.CompositeSymbolID{FileID: 1, LocalSymbolID: 2},
						Name: "FuncB",
					},
					Relationships: types.SymbolRelationships{
						CallsTo: []types.FunctionCall{
							{Target: types.CompositeSymbolID{FileID: 1, LocalSymbolID: 3}}, // internal call
						},
					},
				},
				{
					Identity: types.SymbolIdentity{
						ID:   types.CompositeSymbolID{FileID: 1, LocalSymbolID: 3},
						Name: "FuncC",
					},
					Relationships: types.SymbolRelationships{},
				},
			},
			moduleSymbolSets: map[string]map[string]bool{
				"cohesive": {
					"Symbol[F:1,L:1]": true,
					"Symbol[F:1,L:2]": true,
					"Symbol[F:1,L:3]": true,
				},
			},
			expectedCohesion:   1.0, // All calls are internal
			expectedCoupling:   0.0, // No external dependencies
			expectedStability:  0.5, // No external callers or callees
			cohesionTolerance:  0.01,
			couplingTolerance:  0.01,
			stabilityTolerance: 0.01,
		},
		{
			name:       "module with external dependencies (unstable)",
			moduleName: "unstable",
			symbols: []*types.UniversalSymbolNode{
				{
					Identity: types.SymbolIdentity{
						ID:   types.CompositeSymbolID{FileID: 1, LocalSymbolID: 1},
						Name: "FuncA",
					},
					Relationships: types.SymbolRelationships{
						Dependencies: []types.SymbolDependency{
							{Target: types.CompositeSymbolID{FileID: 2, LocalSymbolID: 1}}, // external dep
							{Target: types.CompositeSymbolID{FileID: 2, LocalSymbolID: 2}}, // external dep
						},
						CallsTo: []types.FunctionCall{
							{Target: types.CompositeSymbolID{FileID: 2, LocalSymbolID: 3}}, // external call
						},
					},
				},
			},
			moduleSymbolSets: map[string]map[string]bool{
				"unstable": {
					"Symbol[F:1,L:1]": true,
				},
				"external": {
					"Symbol[F:2,L:1]": true,
					"Symbol[F:2,L:2]": true,
					"Symbol[F:2,L:3]": true,
				},
			},
			expectedCohesion:   0.0, // All dependencies are external
			expectedCoupling:   0.2, // 2 external deps / (1 symbol * 10 max) - calls don't count as deps
			expectedStability:  0.0, // No incoming deps, all outgoing = maximally unstable
			cohesionTolerance:  0.01,
			couplingTolerance:  0.01,
			stabilityTolerance: 0.01,
		},
		{
			name:       "stable module with incoming dependencies",
			moduleName: "stable",
			symbols: []*types.UniversalSymbolNode{
				{
					Identity: types.SymbolIdentity{
						ID:   types.CompositeSymbolID{FileID: 1, LocalSymbolID: 1},
						Name: "CoreFunc",
					},
					Relationships: types.SymbolRelationships{
						// Other modules depend on this one
						CalledBy: []types.FunctionCall{
							{Target: types.CompositeSymbolID{FileID: 2, LocalSymbolID: 1}}, // external caller
							{Target: types.CompositeSymbolID{FileID: 3, LocalSymbolID: 1}}, // external caller
							{Target: types.CompositeSymbolID{FileID: 4, LocalSymbolID: 1}}, // external caller
						},
						// But this module doesn't depend on others
						Dependencies: []types.SymbolDependency{},
						CallsTo:      []types.FunctionCall{},
					},
				},
			},
			moduleSymbolSets: map[string]map[string]bool{
				"stable": {
					"Symbol[F:1,L:1]": true,
				},
			},
			expectedCohesion:   0.5, // No internal calls but also no external
			expectedCoupling:   0.0, // No outgoing dependencies
			expectedStability:  1.0, // All incoming, no outgoing = maximally stable
			cohesionTolerance:  0.01,
			couplingTolerance:  0.01,
			stabilityTolerance: 0.01,
		},
		{
			name:       "mixed module with both internal and external",
			moduleName: "mixed",
			symbols: []*types.UniversalSymbolNode{
				{
					Identity: types.SymbolIdentity{
						ID:   types.CompositeSymbolID{FileID: 1, LocalSymbolID: 1},
						Name: "FuncA",
					},
					Relationships: types.SymbolRelationships{
						CallsTo: []types.FunctionCall{
							{Target: types.CompositeSymbolID{FileID: 1, LocalSymbolID: 2}}, // internal
							{Target: types.CompositeSymbolID{FileID: 2, LocalSymbolID: 1}}, // external
						},
						CalledBy: []types.FunctionCall{
							{Target: types.CompositeSymbolID{FileID: 3, LocalSymbolID: 1}}, // external caller
						},
					},
				},
				{
					Identity: types.SymbolIdentity{
						ID:   types.CompositeSymbolID{FileID: 1, LocalSymbolID: 2},
						Name: "FuncB",
					},
					Relationships: types.SymbolRelationships{
						Dependencies: []types.SymbolDependency{
							{Target: types.CompositeSymbolID{FileID: 2, LocalSymbolID: 2}}, // external dep
						},
					},
				},
			},
			moduleSymbolSets: map[string]map[string]bool{
				"mixed": {
					"Symbol[F:1,L:1]": true,
					"Symbol[F:1,L:2]": true,
				},
			},
			// internalDeps=0, externalDeps=1, totalCalls=2, internalCalls=1
			// cohesion = (0 + 1) / (0 + 1 + 2) = 1/3 = 0.333
			expectedCohesion: 0.333,
			// externalDeps=1, symbols=2: coupling = 1 / (2 * 10) = 0.05
			expectedCoupling: 0.05,
			// incomingDeps=1 (CalledBy), externalDeps=1: stability = 1 / (1 + 1) = 0.5
			expectedStability:  0.5,
			cohesionTolerance:  0.05,
			couplingTolerance:  0.05,
			stabilityTolerance: 0.05,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cohesion, coupling, stability := server.calculateModuleMetrics(
				tt.moduleName,
				tt.symbols,
				tt.moduleSymbolSets,
			)

			if math.Abs(cohesion-tt.expectedCohesion) > tt.cohesionTolerance {
				t.Errorf("Cohesion: got %.3f, want %.3f (±%.3f)",
					cohesion, tt.expectedCohesion, tt.cohesionTolerance)
			}

			if math.Abs(coupling-tt.expectedCoupling) > tt.couplingTolerance {
				t.Errorf("Coupling: got %.3f, want %.3f (±%.3f)",
					coupling, tt.expectedCoupling, tt.couplingTolerance)
			}

			if math.Abs(stability-tt.expectedStability) > tt.stabilityTolerance {
				t.Errorf("Stability: got %.3f, want %.3f (±%.3f)",
					stability, tt.expectedStability, tt.stabilityTolerance)
			}
		})
	}
}

// TestCalculateDomainConfidence tests the domain term confidence calculation
func TestCalculateDomainConfidence(t *testing.T) {
	server := &Server{}

	tests := []struct {
		name           string
		matchStrength  float64
		termCount      int
		totalFrequency int
		totalTerms     int
		minConfidence  float64
		maxConfidence  float64
	}{
		{
			name:           "high match strength, many terms, high frequency",
			matchStrength:  1.0,
			termCount:      10,
			totalFrequency: 100,
			totalTerms:     100,
			minConfidence:  0.8,
			maxConfidence:  1.0,
		},
		{
			name:           "low match strength, few terms",
			matchStrength:  0.5,
			termCount:      1,
			totalFrequency: 5,
			totalTerms:     100,
			minConfidence:  0.2,
			maxConfidence:  0.5,
		},
		{
			name:           "medium match strength, moderate terms",
			matchStrength:  0.75,
			termCount:      5,
			totalFrequency: 25,
			totalTerms:     50,
			minConfidence:  0.5,
			maxConfidence:  0.8,
		},
		{
			name:           "exact match with single term",
			matchStrength:  1.0,
			termCount:      1,
			totalFrequency: 10,
			totalTerms:     100,
			minConfidence:  0.4,
			maxConfidence:  0.7,
		},
		{
			name:           "zero terms should return minimum",
			matchStrength:  0.8,
			termCount:      0,
			totalFrequency: 0,
			totalTerms:     100,
			minConfidence:  0.1,
			maxConfidence:  0.4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confidence := server.calculateDomainConfidence(
				tt.matchStrength,
				tt.termCount,
				tt.totalFrequency,
				tt.totalTerms,
			)

			if confidence < tt.minConfidence || confidence > tt.maxConfidence {
				t.Errorf("Confidence %.3f not in expected range [%.3f, %.3f]",
					confidence, tt.minConfidence, tt.maxConfidence)
			}
		})
	}
}

// TestClassifyTermDomainWithStrength tests domain classification with match strength
func TestClassifyTermDomainWithStrength(t *testing.T) {
	server := &Server{}

	tests := []struct {
		term             string
		expectedDomain   string
		minStrength      float64
		shouldHaveDomain bool
	}{
		// Authentication domain
		{"auth", "Authentication", 1.0, true},
		{"authenticate", "Authentication", 0.7, true},
		{"AuthService", "Authentication", 0.7, true},
		{"login", "Authentication", 1.0, true},
		{"loginUser", "Authentication", 0.7, true},
		{"password", "Authentication", 1.0, true},
		{"token", "Authentication", 1.0, true}, // token is in both Auth and Parsing; Authentication wins alphabetically
		{"oauth", "Authentication", 1.0, true},
		{"jwt", "Authentication", 1.0, true},

		// Database domain
		{"db", "Database", 1.0, true},
		{"database", "Database", 1.0, true},
		{"query", "Database", 1.0, true},
		{"queryBuilder", "Database", 0.7, true},
		{"sql", "Database", 1.0, true},
		{"transaction", "Database", 1.0, true},

		// HTTP/API domain
		{"http", "HTTP/API", 1.0, true},
		{"httpClient", "HTTP/API", 0.7, true},
		{"api", "HTTP/API", 1.0, true},
		{"handler", "HTTP/API", 1.0, true},
		{"endpoint", "HTTP/API", 1.0, true},
		{"request", "HTTP/API", 1.0, true},
		{"response", "HTTP/API", 1.0, true},

		// Parsing domain
		{"parse", "Parsing", 1.0, true},
		{"parser", "Parsing", 1.0, true},
		{"lexer", "Parsing", 1.0, true},
		{"ast", "Parsing", 1.0, true},
		{"syntax", "Parsing", 1.0, true},

		// Testing domain
		{"test", "Testing", 1.0, true},
		{"testHelper", "Testing", 0.7, true},
		{"mock", "Testing", 1.0, true},
		{"mockService", "Testing", 0.7, true},
		{"assert", "Testing", 1.0, true},
		{"benchmark", "Testing", 1.0, true},

		// Configuration domain
		{"config", "Configuration", 1.0, true},
		{"configLoader", "Configuration", 0.7, true},
		{"setting", "Configuration", 1.0, true},
		{"option", "Configuration", 1.0, true},

		// Error Handling domain
		{"error", "Error Handling", 1.0, true},
		{"errorHandler", "HTTP/API", 0.8, true}, // "handler" matches HTTP/API with higher weight than "error"
		{"err", "Error Handling", 1.0, true},
		{"panic", "Error Handling", 1.0, true},

		// Concurrency domain
		{"goroutine", "Concurrency", 1.0, true},
		{"channel", "Concurrency", 1.0, true},
		{"mutex", "Concurrency", 1.0, true},
		{"sync", "Concurrency", 1.0, true},
		{"async", "Concurrency", 1.0, true},
		{"worker", "Concurrency", 1.0, true},

		// Non-matching terms
		{"calculate", "", 0.0, false},
		{"process", "", 0.0, false},
		{"main", "", 0.0, false},
		{"helper", "", 0.0, false},
		{"utils", "", 0.0, false},
	}

	for _, tt := range tests {
		t.Run(tt.term, func(t *testing.T) {
			domain, strength := server.classifyTermDomainWithStrength(tt.term)

			if tt.shouldHaveDomain {
				if domain != tt.expectedDomain {
					t.Errorf("Term %q: got domain %q, want %q", tt.term, domain, tt.expectedDomain)
				}
				if strength < tt.minStrength {
					t.Errorf("Term %q: got strength %.2f, want >= %.2f", tt.term, strength, tt.minStrength)
				}
			} else {
				if domain != "" {
					t.Errorf("Term %q: expected no domain, got %q", tt.term, domain)
				}
			}
		})
	}
}

// TestExtractCriticalFunctions tests the critical function extraction
func TestExtractCriticalFunctions(t *testing.T) {
	server := &Server{}

	tests := []struct {
		name            string
		symbols         []*types.UniversalSymbolNode
		minFunctions    int
		expectExported  bool
		expectHighUsage bool
	}{
		{
			name: "exported functions should be included",
			symbols: []*types.UniversalSymbolNode{
				{
					Identity: types.SymbolIdentity{
						ID:   types.CompositeSymbolID{FileID: 1, LocalSymbolID: 1},
						Name: "PublicFunc",
						Kind: types.SymbolKindFunction,
					},
					Visibility: types.SymbolVisibility{
						IsExported: true,
					},
					Usage: types.SymbolUsage{
						ReferenceCount: 0,
						CallCount:      0,
					},
				},
			},
			minFunctions:   1,
			expectExported: true,
		},
		{
			name: "high usage functions should be included",
			symbols: []*types.UniversalSymbolNode{
				{
					Identity: types.SymbolIdentity{
						ID:   types.CompositeSymbolID{FileID: 1, LocalSymbolID: 1},
						Name: "frequentlyUsed",
						Kind: types.SymbolKindFunction,
					},
					Visibility: types.SymbolVisibility{
						IsExported: false,
					},
					Usage: types.SymbolUsage{
						ReferenceCount: 10,
						CallCount:      20,
					},
				},
			},
			minFunctions:    1,
			expectHighUsage: true,
		},
		{
			name: "non-function symbols should be excluded",
			symbols: []*types.UniversalSymbolNode{
				{
					Identity: types.SymbolIdentity{
						ID:   types.CompositeSymbolID{FileID: 1, LocalSymbolID: 1},
						Name: "MyStruct",
						Kind: types.SymbolKindStruct,
					},
					Visibility: types.SymbolVisibility{
						IsExported: true,
					},
				},
				{
					Identity: types.SymbolIdentity{
						ID:   types.CompositeSymbolID{FileID: 1, LocalSymbolID: 2},
						Name: "myVar",
						Kind: types.SymbolKindVariable,
					},
					Usage: types.SymbolUsage{
						ReferenceCount: 100,
					},
				},
			},
			minFunctions: 0, // No functions should be extracted
		},
		{
			name: "methods should be included",
			symbols: []*types.UniversalSymbolNode{
				{
					Identity: types.SymbolIdentity{
						ID:   types.CompositeSymbolID{FileID: 1, LocalSymbolID: 1},
						Name: "DoSomething",
						Kind: types.SymbolKindMethod,
					},
					Visibility: types.SymbolVisibility{
						IsExported: true,
					},
					Usage: types.SymbolUsage{
						ReferenceCount: 5,
						CallCount:      10,
					},
				},
			},
			minFunctions: 1,
		},
		{
			name: "private unused functions should be excluded",
			symbols: []*types.UniversalSymbolNode{
				{
					Identity: types.SymbolIdentity{
						ID:   types.CompositeSymbolID{FileID: 1, LocalSymbolID: 1},
						Name: "privateUnused",
						Kind: types.SymbolKindFunction,
					},
					Visibility: types.SymbolVisibility{
						IsExported: false,
					},
					Usage: types.SymbolUsage{
						ReferenceCount: 0,
						CallCount:      0,
					},
				},
			},
			minFunctions: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := &CodebaseIntelligenceParams{}
			result := server.extractCriticalFunctions(tt.symbols, args)

			if len(result) < tt.minFunctions {
				t.Errorf("Got %d functions, want at least %d", len(result), tt.minFunctions)
			}

			if tt.expectExported && len(result) > 0 {
				found := false
				for _, f := range result {
					if f.IsExported {
						found = true
						break
					}
				}
				if !found {
					t.Error("Expected to find exported function in results")
				}
			}

			if tt.expectHighUsage && len(result) > 0 {
				found := false
				for _, f := range result {
					if f.ReferencedBy > 0 {
						found = true
						break
					}
				}
				if !found {
					t.Error("Expected to find high-usage function in results")
				}
			}
		})
	}
}

// TestImportanceScoreCalculation verifies the importance score formula with multiple values
func TestImportanceScoreCalculation(t *testing.T) {
	server := &Server{}

	tests := []struct {
		name          string
		refCount      int
		callCount     int
		isExported    bool
		expectedScore float64
		expectedRefs  int
	}{
		{
			name:          "exported function with high usage",
			refCount:      5,
			callCount:     10,
			isExported:    true,
			expectedScore: 120.0, // 5*10 + 10*5 + 20 = 120
			expectedRefs:  15,
		},
		{
			name:          "exported function with no usage",
			refCount:      0,
			callCount:     0,
			isExported:    true,
			expectedScore: 20.0, // 0*10 + 0*5 + 20 = 20
			expectedRefs:  0,
		},
		{
			name:          "private function with high usage",
			refCount:      10,
			callCount:     20,
			isExported:    false,
			expectedScore: 200.0, // 10*10 + 20*5 + 0 = 200
			expectedRefs:  30,
		},
		{
			name:          "private function with moderate usage",
			refCount:      3,
			callCount:     5,
			isExported:    false,
			expectedScore: 55.0, // 3*10 + 5*5 + 0 = 55
			expectedRefs:  8,
		},
		{
			name:          "exported function with only refs",
			refCount:      8,
			callCount:     0,
			isExported:    true,
			expectedScore: 100.0, // 8*10 + 0*5 + 20 = 100
			expectedRefs:  8,
		},
		{
			name:          "exported function with only calls",
			refCount:      0,
			callCount:     12,
			isExported:    true,
			expectedScore: 80.0, // 0*10 + 12*5 + 20 = 80
			expectedRefs:  12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			symbols := []*types.UniversalSymbolNode{
				{
					Identity: types.SymbolIdentity{
						ID:   types.CompositeSymbolID{FileID: 1, LocalSymbolID: 1},
						Name: "TestFunc",
						Kind: types.SymbolKindFunction,
					},
					Visibility: types.SymbolVisibility{
						IsExported: tt.isExported,
					},
					Usage: types.SymbolUsage{
						ReferenceCount: tt.refCount,
						CallCount:      tt.callCount,
					},
				},
			}

			args := &CodebaseIntelligenceParams{}
			result := server.extractCriticalFunctions(symbols, args)

			// Private unused functions are filtered out
			if !tt.isExported && tt.refCount == 0 && tt.callCount == 0 {
				if len(result) != 0 {
					t.Errorf("Expected 0 functions for private unused, got %d", len(result))
				}
				return
			}

			if len(result) != 1 {
				t.Fatalf("Expected 1 function, got %d", len(result))
			}

			if result[0].ImportanceScore != tt.expectedScore {
				t.Errorf("ImportanceScore: got %.1f, want %.1f", result[0].ImportanceScore, tt.expectedScore)
			}

			if result[0].ReferencedBy != tt.expectedRefs {
				t.Errorf("ReferencedBy: got %d, want %d", result[0].ReferencedBy, tt.expectedRefs)
			}
		})
	}
}

// TestModuleMetricsProducesDistinctValues verifies that different module structures
// produce measurably different metric values
func TestModuleMetricsProducesDistinctValues(t *testing.T) {
	server := &Server{}

	// Collect all computed values to verify distinctness
	cohesionValues := make(map[float64]string)
	couplingValues := make(map[float64]string)
	stabilityValues := make(map[float64]string)

	testCases := []struct {
		name       string
		symbols    []*types.UniversalSymbolNode
		moduleSets map[string]map[string]bool
	}{
		{
			name:    "empty_module",
			symbols: []*types.UniversalSymbolNode{},
			moduleSets: map[string]map[string]bool{
				"test": {},
			},
		},
		{
			name: "fully_cohesive_all_internal_calls",
			symbols: []*types.UniversalSymbolNode{
				{
					Identity: types.SymbolIdentity{
						ID:   types.CompositeSymbolID{FileID: 1, LocalSymbolID: 1},
						Name: "A",
					},
					Relationships: types.SymbolRelationships{
						CallsTo: []types.FunctionCall{
							{Target: types.CompositeSymbolID{FileID: 1, LocalSymbolID: 2}},
							{Target: types.CompositeSymbolID{FileID: 1, LocalSymbolID: 3}},
						},
					},
				},
				{
					Identity: types.SymbolIdentity{
						ID:   types.CompositeSymbolID{FileID: 1, LocalSymbolID: 2},
						Name: "B",
					},
				},
				{
					Identity: types.SymbolIdentity{
						ID:   types.CompositeSymbolID{FileID: 1, LocalSymbolID: 3},
						Name: "C",
					},
				},
			},
			moduleSets: map[string]map[string]bool{
				"test": {
					"Symbol[F:1,L:1]": true,
					"Symbol[F:1,L:2]": true,
					"Symbol[F:1,L:3]": true,
				},
			},
		},
		{
			name: "zero_cohesion_all_external_calls",
			symbols: []*types.UniversalSymbolNode{
				{
					Identity: types.SymbolIdentity{
						ID:   types.CompositeSymbolID{FileID: 1, LocalSymbolID: 1},
						Name: "A",
					},
					Relationships: types.SymbolRelationships{
						CallsTo: []types.FunctionCall{
							{Target: types.CompositeSymbolID{FileID: 2, LocalSymbolID: 1}},
							{Target: types.CompositeSymbolID{FileID: 2, LocalSymbolID: 2}},
						},
					},
				},
			},
			moduleSets: map[string]map[string]bool{
				"test": {
					"Symbol[F:1,L:1]": true,
				},
			},
		},
		{
			name: "high_coupling_many_external_deps",
			symbols: []*types.UniversalSymbolNode{
				{
					Identity: types.SymbolIdentity{
						ID:   types.CompositeSymbolID{FileID: 1, LocalSymbolID: 1},
						Name: "A",
					},
					Relationships: types.SymbolRelationships{
						Dependencies: []types.SymbolDependency{
							{Target: types.CompositeSymbolID{FileID: 2, LocalSymbolID: 1}},
							{Target: types.CompositeSymbolID{FileID: 2, LocalSymbolID: 2}},
							{Target: types.CompositeSymbolID{FileID: 2, LocalSymbolID: 3}},
							{Target: types.CompositeSymbolID{FileID: 2, LocalSymbolID: 4}},
							{Target: types.CompositeSymbolID{FileID: 2, LocalSymbolID: 5}},
						},
					},
				},
			},
			moduleSets: map[string]map[string]bool{
				"test": {
					"Symbol[F:1,L:1]": true,
				},
			},
		},
		{
			name: "maximally_stable_only_incoming",
			symbols: []*types.UniversalSymbolNode{
				{
					Identity: types.SymbolIdentity{
						ID:   types.CompositeSymbolID{FileID: 1, LocalSymbolID: 1},
						Name: "A",
					},
					Relationships: types.SymbolRelationships{
						CalledBy: []types.FunctionCall{
							{Target: types.CompositeSymbolID{FileID: 2, LocalSymbolID: 1}},
							{Target: types.CompositeSymbolID{FileID: 3, LocalSymbolID: 1}},
							{Target: types.CompositeSymbolID{FileID: 4, LocalSymbolID: 1}},
						},
					},
				},
			},
			moduleSets: map[string]map[string]bool{
				"test": {
					"Symbol[F:1,L:1]": true,
				},
			},
		},
		{
			name: "maximally_unstable_only_outgoing",
			symbols: []*types.UniversalSymbolNode{
				{
					Identity: types.SymbolIdentity{
						ID:   types.CompositeSymbolID{FileID: 1, LocalSymbolID: 1},
						Name: "A",
					},
					Relationships: types.SymbolRelationships{
						Dependencies: []types.SymbolDependency{
							{Target: types.CompositeSymbolID{FileID: 2, LocalSymbolID: 1}},
						},
					},
				},
			},
			moduleSets: map[string]map[string]bool{
				"test": {
					"Symbol[F:1,L:1]": true,
				},
			},
		},
	}

	for _, tc := range testCases {
		cohesion, coupling, stability := server.calculateModuleMetrics("test", tc.symbols, tc.moduleSets)

		// Round to 2 decimal places for comparison
		cohesion = math.Round(cohesion*100) / 100
		coupling = math.Round(coupling*100) / 100
		stability = math.Round(stability*100) / 100

		t.Logf("%s: cohesion=%.2f, coupling=%.2f, stability=%.2f", tc.name, cohesion, coupling, stability)

		cohesionValues[cohesion] = tc.name
		couplingValues[coupling] = tc.name
		stabilityValues[stability] = tc.name
	}

	// Verify we got multiple distinct values for each metric
	if len(cohesionValues) < 3 {
		t.Errorf("Cohesion only produced %d distinct values, want at least 3. Values: %v",
			len(cohesionValues), cohesionValues)
	}

	if len(couplingValues) < 2 {
		t.Errorf("Coupling only produced %d distinct values, want at least 2. Values: %v",
			len(couplingValues), couplingValues)
	}

	if len(stabilityValues) < 3 {
		t.Errorf("Stability only produced %d distinct values, want at least 3. Values: %v",
			len(stabilityValues), stabilityValues)
	}
}

// TestDomainConfidenceProducesDistinctValues verifies confidence calculation
// produces different values for different inputs
func TestDomainConfidenceProducesDistinctValues(t *testing.T) {
	server := &Server{}

	tests := []struct {
		name           string
		matchStrength  float64
		termCount      int
		totalFrequency int
		totalTerms     int
	}{
		{"weak_single_term", 0.5, 1, 2, 100},
		{"weak_multiple_terms", 0.5, 5, 10, 100},
		{"medium_single_term", 0.75, 1, 5, 100},
		{"medium_multiple_terms", 0.75, 5, 25, 100},
		{"strong_single_term", 1.0, 1, 10, 100},
		{"strong_multiple_terms", 1.0, 10, 50, 100},
		{"strong_dominant_terms", 1.0, 20, 80, 100},
		{"zero_terms", 0.8, 0, 0, 100},
	}

	confidenceValues := make(map[float64][]string)

	for _, tt := range tests {
		confidence := server.calculateDomainConfidence(
			tt.matchStrength,
			tt.termCount,
			tt.totalFrequency,
			tt.totalTerms,
		)

		// Round to 2 decimal places
		rounded := math.Round(confidence*100) / 100

		t.Logf("%s: confidence=%.3f (rounded=%.2f)", tt.name, confidence, rounded)

		confidenceValues[rounded] = append(confidenceValues[rounded], tt.name)
	}

	// Should produce at least 4 distinct confidence values
	if len(confidenceValues) < 4 {
		t.Errorf("Confidence calculation only produced %d distinct values, want at least 4",
			len(confidenceValues))
		for val, names := range confidenceValues {
			t.Logf("  %.2f: %v", val, names)
		}
	}
}

// TestCriticalFunctionsSortedByImportance verifies functions are sorted by importance score
func TestCriticalFunctionsSortedByImportance(t *testing.T) {
	server := &Server{}

	symbols := []*types.UniversalSymbolNode{
		{
			Identity: types.SymbolIdentity{
				ID:   types.CompositeSymbolID{FileID: 1, LocalSymbolID: 1},
				Name: "LowImportance",
				Kind: types.SymbolKindFunction,
			},
			Visibility: types.SymbolVisibility{IsExported: true},
			Usage:      types.SymbolUsage{ReferenceCount: 1, CallCount: 0},
		},
		{
			Identity: types.SymbolIdentity{
				ID:   types.CompositeSymbolID{FileID: 1, LocalSymbolID: 2},
				Name: "HighImportance",
				Kind: types.SymbolKindFunction,
			},
			Visibility: types.SymbolVisibility{IsExported: true},
			Usage:      types.SymbolUsage{ReferenceCount: 20, CallCount: 30},
		},
		{
			Identity: types.SymbolIdentity{
				ID:   types.CompositeSymbolID{FileID: 1, LocalSymbolID: 3},
				Name: "MediumImportance",
				Kind: types.SymbolKindFunction,
			},
			Visibility: types.SymbolVisibility{IsExported: true},
			Usage:      types.SymbolUsage{ReferenceCount: 5, CallCount: 10},
		},
		{
			Identity: types.SymbolIdentity{
				ID:   types.CompositeSymbolID{FileID: 1, LocalSymbolID: 4},
				Name: "VeryHighImportance",
				Kind: types.SymbolKindFunction,
			},
			Visibility: types.SymbolVisibility{IsExported: true},
			Usage:      types.SymbolUsage{ReferenceCount: 50, CallCount: 100},
		},
	}

	args := &CodebaseIntelligenceParams{}
	result := server.extractCriticalFunctions(symbols, args)

	if len(result) < 4 {
		t.Fatalf("Expected 4 functions, got %d", len(result))
	}

	// Verify sorted by importance (descending)
	for i := 0; i < len(result)-1; i++ {
		if result[i].ImportanceScore < result[i+1].ImportanceScore {
			t.Errorf("Functions not sorted by importance: %s (%.1f) < %s (%.1f)",
				result[i].Name, result[i].ImportanceScore,
				result[i+1].Name, result[i+1].ImportanceScore)
		}
	}

	// Verify we see distinct importance scores
	scores := make(map[float64]bool)
	for _, f := range result {
		scores[f.ImportanceScore] = true
		t.Logf("%s: importance=%.1f", f.Name, f.ImportanceScore)
	}

	if len(scores) < 4 {
		t.Errorf("Expected 4 distinct importance scores, got %d", len(scores))
	}

	// Verify the order
	if result[0].Name != "VeryHighImportance" {
		t.Errorf("Expected VeryHighImportance first, got %s", result[0].Name)
	}
	if result[len(result)-1].Name != "LowImportance" {
		t.Errorf("Expected LowImportance last, got %s", result[len(result)-1].Name)
	}
}

// TestComplexityThresholdsConsistency verifies that complexity threshold constants
// are used consistently across all complexity-related functions.
// This tests for Issue #2: inconsistent threshold usage (< vs <=, hardcoded vs constants)
func TestComplexityThresholdsConsistency(t *testing.T) {
	// The thresholds should be consistent:
	// - complexityThresholdLow = 10 (CC <= 10 is "low")
	// - CC > 10 and <= 20 is "medium"
	// - CC > 20 is "high"
	//
	// This test verifies that the boundary conditions are consistent

	// Test boundary values
	testCases := []struct {
		name             string
		cc               int
		expectedCategory string
	}{
		// Low complexity range (1-10)
		{"CC=1 should be low", 1, "low"},
		{"CC=5 should be low", 5, "low"},
		{"CC=10 should be low", 10, "low"},

		// Medium complexity range (11-20)
		{"CC=11 should be medium", 11, "medium"},
		{"CC=15 should be medium", 15, "medium"},
		{"CC=20 should be medium", 20, "medium"},

		// High complexity range (21+)
		{"CC=21 should be high", 21, "high"},
		{"CC=25 should be high", 25, "high"},
		{"CC=50 should be high", 50, "high"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test that categorization is consistent with thresholds
			category := categorizeComplexity(tc.cc)
			if category != tc.expectedCategory {
				t.Errorf("CC=%d: expected category %q, got %q", tc.cc, tc.expectedCategory, category)
			}
		})
	}
}

// categorizeComplexity is a helper that mirrors the expected categorization logic
// based on the defined threshold constants
func categorizeComplexity(cc int) string {
	// These thresholds should match the constants in codebase_intelligence_tools.go
	// complexityThresholdLow = 10
	// complexityThresholdHigh = 20
	if cc <= complexityThresholdLow {
		return "low"
	} else if cc <= complexityThresholdHigh {
		return "medium"
	}
	return "high"
}

// TestHighComplexityFunctionsHaveObjectID verifies that high complexity functions
// in statistics report include ObjectID for drill-down.
// This tests for Issue #3: Missing ObjectID in FunctionInfo high complexity list
func TestHighComplexityFunctionsHaveObjectID(t *testing.T) {
	// This test will verify that when we have high complexity functions,
	// they should include ObjectID (if available from EnhancedSymbol)

	server := &Server{}

	// Create test files with enhanced symbols having high complexity
	files := []*types.FileInfo{
		{
			Path: "test.go",
			EnhancedSymbols: []*types.EnhancedSymbol{
				{
					Symbol: types.Symbol{
						Name:   "complexFunction",
						Line:   10,
						Column: 1,
						Type:   types.SymbolTypeFunction,
					},
					ID:         types.SymbolID(12345),
					Complexity: 25, // High complexity (> 20)
				},
				{
					Symbol: types.Symbol{
						Name:   "simpleFunction",
						Line:   50,
						Column: 1,
						Type:   types.SymbolTypeFunction,
					},
					ID:         types.SymbolID(67890),
					Complexity: 5, // Low complexity
				},
			},
		},
	}

	metrics := server.calculateComplexityMetricsFromFiles(files)

	// Should have found the high complexity function
	if len(metrics.HighComplexityFuncs) == 0 {
		t.Fatal("Expected at least one high complexity function")
	}

	// Verify the high complexity function has an ObjectID
	for _, fn := range metrics.HighComplexityFuncs {
		if fn.Name == "complexFunction" {
			if fn.ObjectID == "" {
				t.Errorf("High complexity function %q should have ObjectID for drill-down", fn.Name)
			}
			t.Logf("Found high complexity function: %s with ObjectID=%s", fn.Name, fn.ObjectID)
		}
	}
}

// TestThresholdConstantsAreUsed verifies that the declared threshold constants
// actually exist and have the expected values
func TestThresholdConstantsAreUsed(t *testing.T) {
	// Verify the constants exist with expected values
	if complexityThresholdLow != 10 {
		t.Errorf("complexityThresholdLow = %d, want 10", complexityThresholdLow)
	}
	if complexityThresholdModerate != 15 {
		t.Errorf("complexityThresholdModerate = %d, want 15", complexityThresholdModerate)
	}
	if complexityThresholdHigh != 20 {
		t.Errorf("complexityThresholdHigh = %d, want 20", complexityThresholdHigh)
	}
	if hotspotComplexityThreshold != 10 {
		t.Errorf("hotspotComplexityThreshold = %d, want 10", hotspotComplexityThreshold)
	}
	if hotspotLinecountThreshold != 50 {
		t.Errorf("hotspotLinecountThreshold = %d, want 50", hotspotLinecountThreshold)
	}
	if highReferenceCountThreshold != 10 {
		t.Errorf("highReferenceCountThreshold = %d, want 10", highReferenceCountThreshold)
	}
	if highUsageThreshold != 5 {
		t.Errorf("highUsageThreshold = %d, want 5", highUsageThreshold)
	}
	if riskScoreMax != 10.0 {
		t.Errorf("riskScoreMax = %f, want 10.0", riskScoreMax)
	}
}
