package types

import (
	"encoding/json"
	"testing"
	"time"
)

func TestContextManifest_Validate(t *testing.T) {
	tests := []struct {
		name     string
		manifest *ContextManifest
		wantErr  bool
		errMsg   string
	}{
		{
			name: "Valid manifest with symbol",
			manifest: &ContextManifest{
				Refs: []ContextRef{
					{F: "main.go", S: "main"},
				},
			},
			wantErr: false,
		},
		{
			name: "Valid manifest with line range",
			manifest: &ContextManifest{
				Refs: []ContextRef{
					{F: "main.go", L: &LineRange{Start: 10, End: 20}},
				},
			},
			wantErr: false,
		},
		{
			name: "Valid manifest with both symbol and line range",
			manifest: &ContextManifest{
				Refs: []ContextRef{
					{F: "main.go", S: "main", L: &LineRange{Start: 1, End: 10}},
				},
			},
			wantErr: false,
		},
		{
			name: "Empty manifest is valid",
			manifest: &ContextManifest{
				Refs: []ContextRef{},
			},
			wantErr: false,
		},
		{
			name: "Missing file path",
			manifest: &ContextManifest{
				Refs: []ContextRef{
					{S: "main"},
				},
			},
			wantErr: true,
			errMsg:  "file path is required",
		},
		{
			name: "Missing symbol and line range",
			manifest: &ContextManifest{
				Refs: []ContextRef{
					{F: "main.go"},
				},
			},
			wantErr: true,
			errMsg:  "either symbol name (s) or line range (l) is required",
		},
		{
			name: "Invalid line range - negative start",
			manifest: &ContextManifest{
				Refs: []ContextRef{
					{F: "main.go", L: &LineRange{Start: -1, End: 10}},
				},
			},
			wantErr: true,
			errMsg:  "line numbers must be positive",
		},
		{
			name: "Invalid line range - start > end",
			manifest: &ContextManifest{
				Refs: []ContextRef{
					{F: "main.go", L: &LineRange{Start: 20, End: 10}},
				},
			},
			wantErr: true,
			errMsg:  "start line must be <= end line",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if _, ok := err.(*ValidationError); !ok {
					t.Errorf("Expected ValidationError, got %T", err)
				}
			}
		})
	}
}

func TestContextManifest_ComputeStats(t *testing.T) {
	manifest := &ContextManifest{
		Refs: []ContextRef{
			{F: "main.go", S: "main", Role: "modify"},
			{F: "main.go", S: "helper", Role: "modify"},
			{F: "types.go", S: "User", L: &LineRange{Start: 10, End: 30}, Role: "contract"},
			{F: "service.go", L: &LineRange{Start: 1, End: 50}}, // No role
		},
	}

	stats := manifest.ComputeStats()

	if stats.RefCount != 4 {
		t.Errorf("Expected RefCount = 4, got %d", stats.RefCount)
	}

	if stats.FileCount != 3 {
		t.Errorf("Expected FileCount = 3 (unique files), got %d", stats.FileCount)
	}

	// TotalLines: main(1) + helper(1) + User(21) + service(50) = 73
	expectedLines := 73
	if stats.TotalLines != expectedLines {
		t.Errorf("Expected TotalLines = %d, got %d", expectedLines, stats.TotalLines)
	}

	if stats.RoleBreakdown["modify"] != 2 {
		t.Errorf("Expected 2 'modify' roles, got %d", stats.RoleBreakdown["modify"])
	}

	if stats.RoleBreakdown["contract"] != 1 {
		t.Errorf("Expected 1 'contract' role, got %d", stats.RoleBreakdown["contract"])
	}
}

func TestContextManifest_JSONRoundtrip(t *testing.T) {
	original := &ContextManifest{
		Task:    "Add discount support",
		Created: time.Date(2025, 12, 5, 10, 0, 0, 0, time.UTC),
		Refs: []ContextRef{
			{
				F:    "cart/pricing.go",
				S:    "calculateTotal",
				X:    []string{"callers", "callees:1"},
				Role: "modify",
				Note: "Main pricing function",
			},
			{
				F:    "cart/types.go",
				S:    "PricingRule",
				X:    []string{"implementations"},
				Role: "contract",
			},
		},
	}

	// Marshal
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Unmarshal
	var decoded ContextManifest
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify
	if decoded.Task != original.Task {
		t.Errorf("Task mismatch: got %q, want %q", decoded.Task, original.Task)
	}

	if len(decoded.Refs) != len(original.Refs) {
		t.Errorf("Refs count mismatch: got %d, want %d", len(decoded.Refs), len(original.Refs))
	}

	if decoded.Version != "1.0" {
		t.Errorf("Version not set during marshal: got %q, want %q", decoded.Version, "1.0")
	}

	// Check first ref
	ref0 := decoded.Refs[0]
	if ref0.F != "cart/pricing.go" {
		t.Errorf("Ref[0].F mismatch: got %q, want %q", ref0.F, "cart/pricing.go")
	}
	if ref0.S != "calculateTotal" {
		t.Errorf("Ref[0].S mismatch: got %q, want %q", ref0.S, "calculateTotal")
	}
	if ref0.Role != "modify" {
		t.Errorf("Ref[0].Role mismatch: got %q, want %q", ref0.Role, "modify")
	}
	if ref0.Note != "Main pricing function" {
		t.Errorf("Ref[0].Note mismatch: got %q, want %q", ref0.Note, "Main pricing function")
	}
}

func TestContextManifest_CompactJSON(t *testing.T) {
	manifest := &ContextManifest{
		Task: "Test task",
		Refs: []ContextRef{
			{F: "a.go", S: "foo"},
		},
	}

	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Check that compact field names are used
	jsonStr := string(data)
	if !contains(jsonStr, `"t":"Test task"`) {
		t.Error("Expected compact field 't' for Task")
	}
	if !contains(jsonStr, `"r":[`) {
		t.Error("Expected compact field 'r' for Refs")
	}
	if !contains(jsonStr, `"f":"a.go"`) {
		t.Error("Expected compact field 'f' for file")
	}
	if !contains(jsonStr, `"s":"foo"`) {
		t.Error("Expected compact field 's' for symbol")
	}
}

func TestFormatType_IsValid(t *testing.T) {
	tests := []struct {
		format FormatType
		valid  bool
	}{
		{FormatFull, true},
		{FormatSignatures, true},
		{FormatOutline, true},
		{FormatType("invalid"), false},
		{FormatType(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			if got := tt.format.IsValid(); got != tt.valid {
				t.Errorf("IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestLineRange_Validation(t *testing.T) {
	tests := []struct {
		name      string
		lineRange *LineRange
		valid     bool
	}{
		{"Valid range", &LineRange{Start: 1, End: 10}, true},
		{"Single line", &LineRange{Start: 5, End: 5}, true},
		{"Zero start", &LineRange{Start: 0, End: 10}, false},
		{"Negative", &LineRange{Start: -1, End: 10}, false},
		{"Inverted", &LineRange{Start: 10, End: 1}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := &ContextManifest{
				Refs: []ContextRef{
					{F: "test.go", L: tt.lineRange},
				},
			}
			err := manifest.Validate()
			if (err == nil) != tt.valid {
				t.Errorf("Validation = %v, want valid = %v", err, tt.valid)
			}
		})
	}
}

func TestHydratedContext_Structure(t *testing.T) {
	// Test that HydratedContext can be marshaled/unmarshaled
	ctx := &HydratedContext{
		Task: "Test task",
		Refs: []HydratedRef{
			{
				File:       "main.go",
				Symbol:     "main",
				Lines:      LineRange{Start: 1, End: 10},
				Role:       "modify",
				Note:       "Entry point",
				Source:     "func main() {\n\t// code\n}",
				SymbolType: "function",
				Signature:  "func main()",
				Expanded: map[string][]HydratedRef{
					"callees": {
						{
							File:   "helper.go",
							Symbol: "helper",
							Lines:  LineRange{Start: 5, End: 15},
							Source: "func helper() {}",
						},
					},
				},
			},
		},
		Stats: HydrationStats{
			RefsLoaded:        1,
			SymbolsHydrated:   2,
			TokensApprox:      500,
			ExpansionsApplied: 1,
		},
	}

	data, err := json.Marshal(ctx)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded HydratedContext
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Task != ctx.Task {
		t.Errorf("Task mismatch")
	}
	if len(decoded.Refs) != 1 {
		t.Errorf("Expected 1 ref, got %d", len(decoded.Refs))
	}
	if decoded.Stats.SymbolsHydrated != 2 {
		t.Errorf("Expected 2 symbols hydrated, got %d", decoded.Stats.SymbolsHydrated)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
