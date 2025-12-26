package mcp

import (
	"reflect"
	"sort"
	"testing"
)

func TestExtractTerms(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		// camelCase splitting
		{
			name:     "simple camelCase",
			input:    "getUserName",
			expected: []string{"get", "user", "name"},
		},
		{
			name:     "camelCase with acronym",
			input:    "parseHTTPResponse",
			expected: []string{"parse", "http", "response"}, // HTTP is split as acronym
		},
		{
			name:     "PascalCase",
			input:    "UserAuthentication",
			expected: []string{"user", "authentication"},
		},
		{
			name:     "mixed case handler",
			input:    "ServeHTTP",
			expected: []string{"serve", "http"},
		},
		{
			name:     "acronym at start",
			input:    "HTTPHandler",
			expected: []string{"http", "handler"},
		},
		{
			name:     "multiple acronyms",
			input:    "XMLToJSONConverter",
			expected: []string{"xml", "json", "converter"},
		},

		// snake_case splitting
		{
			name:     "simple snake_case",
			input:    "get_user_name",
			expected: []string{"get", "user", "name"},
		},
		{
			name:     "snake_case with numbers",
			input:    "parse_v2_response",
			expected: []string{"parse", "response"},
		},

		// kebab-case splitting
		{
			name:     "simple kebab-case",
			input:    "get-user-name",
			expected: []string{"get", "user", "name"},
		},

		// dot notation splitting
		{
			name:     "dot notation",
			input:    "config.database.host",
			expected: []string{"config", "database", "host"},
		},

		// Mixed conventions
		{
			name:     "mixed snake and camel",
			input:    "get_userName",
			expected: []string{"get", "user", "name"},
		},
		{
			name:     "complex mixed",
			input:    "HTTP_request_Handler",
			expected: []string{"http", "request", "handler"},
		},
		{
			name:     "acronym in middle",
			input:    "getAPIResponse",
			expected: []string{"get", "api", "response"},
		},

		// Short terms filtered out (< 3 chars)
		{
			name:     "filters short terms",
			input:    "getID",
			expected: []string{"get"},
		},
		{
			name:     "all short terms",
			input:    "a_b_c",
			expected: []string{},
		},

		// Edge cases
		{
			name:     "single word",
			input:    "handler",
			expected: []string{"handler"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "only uppercase short",
			input:    "HTTP",
			expected: []string{"http"},
		},
		{
			name:     "only uppercase long",
			input:    "AUTHENTICATION",
			expected: []string{"authentication"},
		},
		{
			name:     "lowercase with numbers",
			input:    "handler123",
			expected: []string{"handler123"},
		},

		// Real-world examples from chi codebase
		{
			name:     "chi Router",
			input:    "NewRouter",
			expected: []string{"new", "router"},
		},
		{
			name:     "chi middleware",
			input:    "WithValue",
			expected: []string{"with", "value"},
		},
		{
			name:     "chi URLParam",
			input:    "URLParam",
			expected: []string{"url", "param"},
		},
		{
			name:     "chi ServeHTTP",
			input:    "ServeHTTP",
			expected: []string{"serve", "http"},
		},
		{
			name:     "chi findRoute",
			input:    "findRoute",
			expected: []string{"find", "route"},
		},
		{
			name:     "chi ResponseWriter",
			input:    "ResponseWriter",
			expected: []string{"response", "writer"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTerms(tt.input)

			// Sort both for comparison since order may vary
			sort.Strings(got)
			sort.Strings(tt.expected)

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("extractTerms(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestExtractTerms_NoGarbledOutput(t *testing.T) {
	// This test specifically checks that we don't produce garbled terms
	// like "uth" instead of "auth", "ser" instead of "user", etc.
	garbledTerms := []string{
		"uth", "ser", "ame", "oute", "ndpoint", "esponse",
		"andler", "iddleware", "ession", "oken",
	}

	testInputs := []string{
		"getUserName",
		"AuthenticationService",
		"handleRequest",
		"ResponseWriter",
		"MiddlewareHandler",
		"SessionToken",
		"RouteEndpoint",
	}

	for _, input := range testInputs {
		terms := extractTerms(input)
		for _, term := range terms {
			for _, garbled := range garbledTerms {
				if term == garbled {
					t.Errorf("extractTerms(%q) produced garbled term %q", input, term)
				}
			}
		}
	}
}

func TestClassifyTermSimple(t *testing.T) {
	tests := []struct {
		term     string
		expected string
	}{
		// Auth domain
		{"auth", "auth"},
		{"authentication", "auth"},
		{"login", "auth"},
		{"user", "auth"},
		{"session", "auth"},
		{"token", "auth"},

		// Database domain
		{"database", "db"},
		{"sql", "db"},
		{"model", "db"},
		{"data", "db"},

		// API domain
		{"api", "api"},
		{"http", "api"},
		{"request", "api"},
		{"response", "api"},

		// Test domain
		{"test", "test"},
		{"spec", "test"},
		{"mock", "test"},

		// File domain
		{"file", "file"},
		{"read", "file"},
		{"write", "file"},

		// Cache domain
		{"cache", "cache"},
		{"store", "cache"},
		{"memo", "cache"},

		// API domain (handler, route now classified as API)
		{"handler", "api"},
		{"route", "api"},

		// Config domain
		{"config", "config"},
		{"setting", "config"},

		// Service domain
		{"service", "service"},
		{"manager", "service"},

		// Log domain
		{"log", "log"},
		{"debug", "log"},

		// API domain - router contains "route"
		{"router", "api"},

		// General (no match)
		{"middleware", "general"},
		{"context", "general"},
	}

	for _, tt := range tests {
		t.Run(tt.term, func(t *testing.T) {
			got := classifyTermSimple(tt.term)
			if got != tt.expected {
				t.Errorf("classifyTermSimple(%q) = %q, want %q", tt.term, got, tt.expected)
			}
		})
	}
}

func TestExtractTerms_ConsistentOutput(t *testing.T) {
	// Same input should always produce same output
	input := "getUserAuthenticationToken"

	first := extractTerms(input)
	for i := 0; i < 10; i++ {
		got := extractTerms(input)
		sort.Strings(first)
		sort.Strings(got)
		if !reflect.DeepEqual(got, first) {
			t.Errorf("extractTerms not consistent: first=%v, iteration %d=%v", first, i, got)
		}
	}
}

func BenchmarkExtractTerms(b *testing.B) {
	inputs := []string{
		"getUserName",
		"AuthenticationServiceHandler",
		"parse_http_response_body",
		"config.database.connection.pool",
		"VeryLongCamelCaseSymbolNameWithManyParts",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, input := range inputs {
			extractTerms(input)
		}
	}
}
