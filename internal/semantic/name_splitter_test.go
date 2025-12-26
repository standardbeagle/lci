package semantic

import (
	"reflect"
	"testing"
)

func TestNameSplitter(t *testing.T) {
	splitter := NewNameSplitter()

	tests := []struct {
		input    string
		expected []string
	}{
		// Basic cases
		{"", []string{}},
		{"simple", []string{"simple"}},
		{"Simple", []string{"simple"}},
		{"SIMPLE", []string{"simple"}},

		// CamelCase
		{"camelCase", []string{"camel", "case"}},
		{"getUserName", []string{"get", "user", "name"}},
		{"createTable", []string{"create", "table"}},
		{"parseJSON", []string{"parse", "json"}},

		// PascalCase
		{"PascalCase", []string{"pascal", "case"}},
		{"GetUserName", []string{"get", "user", "name"}},
		{"CreateTable", []string{"create", "table"}},
		{"ParseJSON", []string{"parse", "json"}},

		// Acronyms
		{"HTTPServer", []string{"http", "server"}},
		{"XMLParser", []string{"xml", "parser"}},
		{"JSONData", []string{"json", "data"}},
		{"HTTPSConnection", []string{"https", "connection"}},
		{"XMLHttpRequest", []string{"xml", "http", "request"}},
		{"IDGenerator", []string{"id", "generator"}},
		{"URLPath", []string{"url", "path"}},

		// Snake case
		{"snake_case", []string{"snake", "case"}},
		{"get_user_name", []string{"get", "user", "name"}},
		{"create_table", []string{"create", "table"}},
		{"parse_json", []string{"parse", "json"}},

		// Screaming snake case
		{"SCREAMING_SNAKE_CASE", []string{"screaming", "snake", "case"}},
		{"GET_USER_NAME", []string{"get", "user", "name"}},
		{"CREATE_TABLE", []string{"create", "table"}},

		// Kebab case
		{"kebab-case", []string{"kebab", "case"}},
		{"get-user-name", []string{"get", "user", "name"}},
		{"create-table", []string{"create", "table"}},

		// Dot notation
		{"dot.notation", []string{"dot", "notation"}},
		{"java.util.ArrayList", []string{"java", "util", "array", "list"}},
		{"com.example.MyClass", []string{"com", "example", "my", "class"}},

		// Path notation
		{"path/to/file", []string{"path", "to", "file"}},
		{"src/main/java", []string{"src", "main", "java"}},

		// Mixed cases
		{"get_userName", []string{"get", "user", "name"}},
		{"http_ServerName", []string{"http", "server", "name"}},
		{"parse-JSONData", []string{"parse", "json", "data"}},
		{"java.util.ConcurrentHashMap", []string{"java", "util", "concurrent", "hash", "map"}},

		// Numbers
		{"version2", []string{"version", "2"}},
		{"v2Parser", []string{"v", "2", "parser"}},
		{"getUserByIDv2", []string{"get", "user", "by", "i", "dv", "2"}},
		{"parse2XML", []string{"parse", "2", "xml"}},
		{"base64Encode", []string{"base", "64", "encode"}},

		// Edge cases
		{"a", []string{"a"}},
		{"A", []string{"a"}},
		{"_", []string{}},
		{"__", []string{}},
		{"_leading", []string{"leading"}},
		{"trailing_", []string{"trailing"}},
		{"__double__underscore__", []string{"double", "underscore"}},
		{"Mixed__Case__Style", []string{"mixed", "case", "style"}},

		// Complex real-world examples
		{"AbstractHTTPSConnectionPoolManager", []string{"abstract", "https", "connection", "pool", "manager"}},
		{"IUserAuthenticationService", []string{"i", "user", "authentication", "service"}},
		{"__init__", []string{"init"}},
		{"MAX_RETRY_COUNT", []string{"max", "retry", "count"}},
		{"getUserByID_v2", []string{"get", "user", "by", "id", "v", "2"}},
		{"MyApp.Controllers.UserController", []string{"my", "app", "controllers", "user", "controller"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := splitter.Split(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Split(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func BenchmarkNameSplitter(b *testing.B) {
	inputs := []string{
		"getUserName",
		"HTTPServerConfiguration",
		"parse_json_response",
		"XMLHttpRequestHandler",
		"get_user_by_id_v2",
		"java.util.concurrent.ThreadPoolExecutor",
		"AbstractHTTPSConnectionPoolManager",
		"IUserAuthenticationService",
	}

	splitter := NewNameSplitter()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for _, input := range inputs {
			_ = splitter.Split(input)
		}
	}
}

func BenchmarkNameSplitterSingleAllocation(b *testing.B) {
	splitter := NewNameSplitter()
	input := "AbstractHTTPSConnectionPoolManager"

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = splitter.Split(input)
	}
}