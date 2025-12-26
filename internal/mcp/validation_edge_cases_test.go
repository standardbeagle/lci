package mcp

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestValidateSearchParams_EdgeCases tests extreme and boundary conditions for search params
func TestValidateSearchParams_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		params  SearchParams
		wantErr bool
		errMsg  string
	}{
		{
			name: "very large max_results",
			params: SearchParams{
				Pattern: "test",
				Max:     999999999,
			},
			wantErr: false, // Should allow very large values
		},
		{
			name: "max results at limit",
			params: SearchParams{
				Pattern: "test",
				Max:     2147483647, // Max int32
			},
			wantErr: false,
		},
		{
			name: "zero max_results",
			params: SearchParams{
				Pattern: "test",
				Max:     0,
			},
			wantErr: false, // Zero should be allowed
		},
		{
			name: "pattern with unicode characters",
			params: SearchParams{
				Pattern: "测试🚀", // Chinese characters and emoji
			},
			wantErr: false, // Unicode should be allowed
		},
		{
			name: "pattern with control characters",
			params: SearchParams{
				Pattern: "test\x00\x01\x02", // Contains null bytes and control chars
			},
			wantErr: false, // Control chars should be allowed (up to the implementation)
		},
		{
			name: "extremely long pattern",
			params: SearchParams{
				Pattern: strings.Repeat("a", 10000), // 10K characters
			},
			wantErr: false, // Should allow long patterns
		},
		{
			name: "pattern with only newlines and tabs",
			params: SearchParams{
				Pattern: "\n\t\n\t",
			},
			wantErr: true,
			errMsg:  "validation error for field 'pattern': pattern cannot be empty",
		},
		{
			name: "pattern with mixed whitespace",
			params: SearchParams{
				Pattern: " \t\n\r test \t\n\r ",
			},
			wantErr: false, // Should trim whitespace and find "test"
		},
		{
			name: "pattern with SQL injection attempt",
			params: SearchParams{
				Pattern: "'; DROP TABLE users; --",
			},
			wantErr: false, // Should be treated as literal string
		},
		{
			name: "pattern with path traversal attempt",
			params: SearchParams{
				Pattern: "../../../etc/passwd",
			},
			wantErr: false, // Should be treated as literal string
		},
		{
			name: "max_results overflow",
			params: SearchParams{
				Pattern: "test",
				Max:     2147483648, // Max int32 + 1
			},
			wantErr: true, // Should fail for overflow
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSearchParams(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSearchParams() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" && err.Error() != tt.errMsg {
				t.Errorf("validateSearchParams() error = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

// TestValidateSymbolParams_EdgeCases tests extreme and boundary conditions for symbol params
func TestValidateSymbolParams_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		params  SymbolParams
		wantErr bool
		errMsg  string
	}{
		{
			name: "symbol with unicode characters",
			params: SymbolParams{
				Symbol: "测试函数名", // Chinese function name
			},
			wantErr: false,
		},
		{
			name: "symbol with emoji",
			params: SymbolParams{
				Symbol: "🚀_test_function", // Emoji in symbol name
			},
			wantErr: false,
		},
		{
			name: "symbol with special characters",
			params: SymbolParams{
				Symbol: "test-function_var$123", // Common programming symbols
			},
			wantErr: false,
		},
		{
			name: "very long symbol name",
			params: SymbolParams{
				Symbol: strings.Repeat("a", 1000), // 1K character symbol name
			},
			wantErr: false,
		},
		{
			name: "symbol with control characters",
			params: SymbolParams{
				Symbol: "test\x00\x01", // Null byte and control chars
			},
			wantErr: false,
		},
		{
			name: "symbol with only whitespace variants",
			params: SymbolParams{
				Symbol: "\t\n\r", // Various whitespace characters
			},
			wantErr: true,
			errMsg:  "validation error for field 'symbol': symbol cannot be empty",
		},
		{
			name: "symbol starting with numbers",
			params: SymbolParams{
				Symbol: "123testFunction", // Numbers at start
			},
			wantErr: false,
		},
		{
			name: "symbol with dots and dashes",
			params: SymbolParams{
				Symbol: "package.Class-method", // Common OOP pattern
			},
			wantErr: false,
		},
		{
			name: "symbol with namespace",
			params: SymbolParams{
				Symbol: "std::vector::push_back", // C++ style
			},
			wantErr: false,
		},
		{
			name: "symbol with generic parameters",
			params: SymbolParams{
				Symbol: "Map<String, Integer>", // Java style generics
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSymbolParams(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSymbolParams() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" && err.Error() != tt.errMsg {
				t.Errorf("validateSymbolParams() error = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

// TestValidateObjectContextParams_EdgeCases tests extreme boundary conditions
func TestValidateObjectContextParams_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		params  ObjectContextParams
		wantErr bool
		errMsg  string
	}{
		{
			name: "id with many comma-separated values",
			params: ObjectContextParams{
				ID: "VE,tG,Ab,Cd,Ef,Gh,Ij,Kl,Mn,Op",
			},
			wantErr: false, // Should allow many IDs
		},
		{
			name: "id with very long base-63 encoded values",
			params: ObjectContextParams{
				ID: "ABCDEFGHIJKLMNOP,QRSTUVWXYZ123456",
			},
			wantErr: false,
		},
		{
			name: "name lookup with file_id",
			params: ObjectContextParams{
				Name:   "getUserName",
				FileID: 123,
			},
			wantErr: false,
		},
		{
			name: "conflict between id and name",
			params: ObjectContextParams{
				ID:   "VE",
				Name: "test",
			},
			wantErr: true,
			errMsg:  "validation error for field 'id,name': parameter conflict: use either 'id' OR 'name', not both",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateObjectContextParams(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateObjectContextParams() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" && err.Error() != tt.errMsg {
				t.Errorf("validateObjectContextParams() error = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

// TestValidationPerformance tests validation performance with large inputs
func TestValidationPerformance(t *testing.T) {
	// Test validation doesn't crash with extremely large inputs
	t.Run("large comma-separated id validation", func(t *testing.T) {
		// Build a comma-separated ID string with 10K IDs
		ids := make([]string, 10000)
		for i := 0; i < 10000; i++ {
			ids[i] = fmt.Sprintf("ID%d", i)
		}
		params := ObjectContextParams{
			ID: strings.Join(ids, ","),
		}

		start := time.Now()
		err := validateObjectContextParams(params)
		duration := time.Since(start)

		if err != nil {
			t.Errorf("Large comma-separated ID validation failed: %v", err)
		}

		if duration > 100*time.Millisecond {
			t.Logf("Large comma-separated ID validation took %v (consider optimizing if too slow)", duration)
		}
	})

	t.Run("very long pattern validation", func(t *testing.T) {
		params := SearchParams{
			Pattern: strings.Repeat("test_pattern_", 10000), // ~140K characters
		}

		start := time.Now()
		err := validateSearchParams(params)
		duration := time.Since(start)

		// Should fail with max length error
		if err == nil {
			t.Errorf("Very long pattern validation should fail: got nil error")
		}

		if duration > 10*time.Millisecond {
			t.Logf("Very long pattern validation took %v (consider optimizing if too slow)", duration)
		}
	})
}
