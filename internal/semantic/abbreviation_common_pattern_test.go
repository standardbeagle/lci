package semantic

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCommonPattern_SearchFullWord_FindAbbreviation tests the most common usage:
// Developer searches with full word (e.g., "transaction"), code uses abbreviation (e.g., "txn")
func TestCommonPattern_SearchFullWord_FindAbbreviation(t *testing.T) {
	dict := DefaultTranslationDictionary()
	matcher := NewAbbreviationMatcher(dict, NewNameSplitter())
	config := DefaultScoreLayers

	// COMMON PATTERN: Search uses full word, code uses abbreviation
	tests := []struct {
		name        string
		searchQuery string // What developer searches for (full word)
		codeSymbol  string // What's actually in the code (abbreviation)
		shouldMatch bool
		description string
	}{
		// Variables using abbreviations
		{
			name:        "transaction_finds_txn",
			searchQuery: "transaction",
			codeSymbol:  "processTxn",
			shouldMatch: true,
			description: "Search 'transaction' should find variable 'processTxn'",
		},
		{
			name:        "transaction_finds_TxnManager",
			searchQuery: "transaction",
			codeSymbol:  "TxnManager",
			shouldMatch: true,
			description: "Search 'transaction' should find class 'TxnManager'",
		},
		{
			name:        "index_finds_idx",
			searchQuery: "index",
			codeSymbol:  "getIdx",
			shouldMatch: true,
			description: "Search 'index' should find variable 'getIdx'",
		},
		{
			name:        "count_finds_cnt",
			searchQuery: "count",
			codeSymbol:  "totalCnt",
			shouldMatch: true,
			description: "Search 'count' should find variable 'totalCnt'",
		},
		{
			name:        "configuration_finds_cfg",
			searchQuery: "configuration",
			codeSymbol:  "loadCfg",
			shouldMatch: true,
			description: "Search 'configuration' should find function 'loadCfg'",
		},
		{
			name:        "error_finds_err",
			searchQuery: "error",
			codeSymbol:  "handleErr",
			shouldMatch: true,
			description: "Search 'error' should find function 'handleErr'",
		},
		{
			name:        "message_finds_msg",
			searchQuery: "message",
			codeSymbol:  "sendMsg",
			shouldMatch: true,
			description: "Search 'message' should find function 'sendMsg'",
		},
		{
			name:        "request_finds_req",
			searchQuery: "request",
			codeSymbol:  "processReq",
			shouldMatch: true,
			description: "Search 'request' should find function 'processReq'",
		},
		{
			name:        "response_finds_resp",
			searchQuery: "response",
			codeSymbol:  "buildResp",
			shouldMatch: true,
			description: "Search 'response' should find function 'buildResp'",
		},
		{
			name:        "context_finds_ctx",
			searchQuery: "context",
			codeSymbol:  "getCtx",
			shouldMatch: true,
			description: "Search 'context' should find function 'getCtx'",
		},
		{
			name:        "manager_finds_mgr",
			searchQuery: "manager",
			codeSymbol:  "connMgr",
			shouldMatch: true,
			description: "Search 'manager' should find variable 'connMgr'",
		},
		{
			name:        "service_finds_svc",
			searchQuery: "service",
			codeSymbol:  "authSvc",
			shouldMatch: true,
			description: "Search 'service' should find variable 'authSvc'",
		},

		// camelCase symbols with abbreviations
		{
			name:        "transaction_finds_ProcessTxnRequest",
			searchQuery: "transaction",
			codeSymbol:  "ProcessTxnRequest",
			shouldMatch: true,
			description: "Search 'transaction' should find 'ProcessTxnRequest' (camelCase)",
		},
		{
			name:        "configuration_finds_LoadCfgFile",
			searchQuery: "configuration",
			codeSymbol:  "LoadCfgFile",
			shouldMatch: true,
			description: "Search 'configuration' should find 'LoadCfgFile' (camelCase)",
		},

		// snake_case symbols with abbreviations
		{
			name:        "transaction_finds_process_txn",
			searchQuery: "transaction",
			codeSymbol:  "process_txn",
			shouldMatch: true,
			description: "Search 'transaction' should find 'process_txn' (snake_case)",
		},
		{
			name:        "configuration_finds_load_cfg",
			searchQuery: "configuration",
			codeSymbol:  "load_cfg",
			shouldMatch: true,
			description: "Search 'configuration' should find 'load_cfg' (snake_case)",
		},

		// REVERSE: Also support abbreviation search finding full words
		{
			name:        "txn_finds_TransactionManager",
			searchQuery: "txn",
			codeSymbol:  "TransactionManager",
			shouldMatch: true,
			description: "Search 'txn' should also find 'TransactionManager'",
		},
		{
			name:        "cfg_finds_ConfigurationLoader",
			searchQuery: "cfg",
			codeSymbol:  "ConfigurationLoader",
			shouldMatch: true,
			description: "Search 'cfg' should also find 'ConfigurationLoader'",
		},

		// Case insensitivity
		{
			name:        "TRANSACTION_finds_txnManager",
			searchQuery: "TRANSACTION",
			codeSymbol:  "txnManager",
			shouldMatch: true,
			description: "Search is case-insensitive (TRANSACTION → txnManager)",
		},
		{
			name:        "Transaction_finds_TXN_HANDLER",
			searchQuery: "Transaction",
			codeSymbol:  "TXN_HANDLER",
			shouldMatch: true,
			description: "Search is case-insensitive (Transaction → TXN_HANDLER)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched, score, justification, details := matcher.Detect(tt.searchQuery, tt.codeSymbol, strings.ToLower(tt.searchQuery), strings.ToLower(tt.codeSymbol), config)

			if tt.shouldMatch {
				assert.True(t, matched, "Expected match for: %s", tt.description)
				assert.Greater(t, score, 0.0, "Score should be positive for a match")
				assert.NotEmpty(t, justification, "Justification should be provided")

				t.Logf("✓ %s", tt.description)
				t.Logf("  Query: '%s' → Symbol: '%s'", tt.searchQuery, tt.codeSymbol)
				t.Logf("  Score: %.2f", score)
				t.Logf("  Justification: %s", justification)
				t.Logf("  Details: %+v", details)
			} else {
				assert.False(t, matched, "Expected no match for: %s", tt.description)
			}
		})
	}
}

// TestCamelCaseSplitting_WithAbbreviations verifies that camelCase splitting works with abbreviations
func TestCamelCaseSplitting_WithAbbreviations(t *testing.T) {
	dict := DefaultTranslationDictionary()
	matcher := NewAbbreviationMatcher(dict, NewNameSplitter())
	config := DefaultScoreLayers

	tests := []struct {
		name        string
		query       string
		symbol      string
		shouldMatch bool
	}{
		// Query should be split into words, each word checked for abbreviation expansion
		{
			name:        "ProcessTxnRequest_splits_to_process_txn_request",
			query:       "transaction",
			symbol:      "ProcessTxnRequest",
			shouldMatch: true,
		},
		{
			name:        "GetIdxValue_splits_to_get_idx_value",
			query:       "index",
			symbol:      "GetIdxValue",
			shouldMatch: true,
		},
		{
			name:        "LoadCfgFromFile_splits_to_load_cfg_from_file",
			query:       "configuration",
			symbol:      "LoadCfgFromFile",
			shouldMatch: true,
		},
		{
			name:        "HandleErrWithContext_splits_to_handle_err_with_context",
			query:       "error",
			symbol:      "HandleErrWithContext",
			shouldMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched, score, justification, _ := matcher.Detect(tt.query, tt.symbol, strings.ToLower(tt.query), strings.ToLower(tt.symbol), config)

			if tt.shouldMatch {
				assert.True(t, matched, "Should match: query '%s' → symbol '%s'", tt.query, tt.symbol)
				t.Logf("✓ Query '%s' matched symbol '%s'", tt.query, tt.symbol)
				t.Logf("  Score: %.2f", score)
				t.Logf("  Justification: %s", justification)

				// Verify the justification mentions the expansion
				lowerJust := strings.ToLower(justification)
				assert.True(t,
					strings.Contains(lowerJust, "abbreviation") ||
						strings.Contains(lowerJust, "expansion"),
					"Justification should mention abbreviation/expansion")
			} else {
				assert.False(t, matched, "Should not match")
			}
		})
	}
}

// TestNormalization_AllLowercase verifies all matching is case-insensitive
func TestNormalization_AllLowercase(t *testing.T) {
	dict := DefaultTranslationDictionary()
	matcher := NewAbbreviationMatcher(dict, NewNameSplitter())
	config := DefaultScoreLayers

	// All these variants should match (case-insensitive)
	variants := []struct {
		query  string
		symbol string
	}{
		{"transaction", "txnManager"},
		{"TRANSACTION", "txnManager"},
		{"Transaction", "txnManager"},
		{"TrAnSaCtIoN", "txnManager"},
		{"transaction", "TxnManager"},
		{"transaction", "TXNMANAGER"},
		{"transaction", "TXN_MANAGER"},
	}

	for i, v := range variants {
		matched, score, justification, _ := matcher.Detect(v.query, v.symbol, strings.ToLower(v.query), strings.ToLower(v.symbol), config)
		assert.True(t, matched, "Variant %d should match: '%s' → '%s'", i, v.query, v.symbol)
		assert.Greater(t, score, 0.0, "Matched variant should have positive score")

		t.Logf("Variant %d: '%s' → '%s' (score: %.3f): %s",
			i, v.query, v.symbol, score, justification)
	}

	// Verify query case doesn't matter (same query different cases on same symbol)
	sameSymbolVariants := []string{"transaction", "TRANSACTION", "Transaction", "TrAnSaCtIoN"}
	var firstSameSymbolScore float64
	for i, query := range sameSymbolVariants {
		matched, score, _, _ := matcher.Detect(query, "txnManager", strings.ToLower(query), strings.ToLower("txnManager"), config)
		assert.True(t, matched, "Query variant %d should match: '%s' → 'txnManager'", i, query)

		if i == 0 {
			firstSameSymbolScore = score
		} else {
			// Query case variations should produce identical scores (same symbol)
			assert.Equal(t, firstSameSymbolScore, score,
				"Different query cases should produce identical scores on same symbol")
		}
	}
}
