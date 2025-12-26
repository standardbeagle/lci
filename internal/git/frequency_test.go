package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTimeWindowToDuration(t *testing.T) {
	tests := []struct {
		window   TimeWindow
		expected time.Duration
	}{
		{Window7Days, 7 * 24 * time.Hour},
		{Window30Days, 30 * 24 * time.Hour},
		{Window90Days, 90 * 24 * time.Hour},
		{Window1Year, 365 * 24 * time.Hour},
		{"invalid", 30 * 24 * time.Hour}, // Default
	}

	for _, tt := range tests {
		t.Run(string(tt.window), func(t *testing.T) {
			got := TimeWindowToDuration(tt.window)
			if got != tt.expected {
				t.Errorf("TimeWindowToDuration(%s) = %v, want %v", tt.window, got, tt.expected)
			}
		})
	}
}

func TestParseTimeWindow(t *testing.T) {
	tests := []struct {
		input    string
		expected TimeWindow
	}{
		{"7d", Window7Days},
		{"7days", Window7Days},
		{"week", Window7Days},
		{"30d", Window30Days},
		{"30days", Window30Days},
		{"month", Window30Days},
		{"90d", Window90Days},
		{"90days", Window90Days},
		{"quarter", Window90Days},
		{"1y", Window1Year},
		{"1year", Window1Year},
		{"year", Window1Year},
		{"365d", Window1Year},
		{"invalid", Window30Days}, // Default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseTimeWindow(tt.input)
			if got != tt.expected {
				t.Errorf("ParseTimeWindow(%s) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestCalculateVolatilityScore(t *testing.T) {
	tests := []struct {
		name          string
		changeCount   int
		linesChanged  int
		uniqueAuthors int
		windowDays    float64
		minExpected   float64
		maxExpected   float64
	}{
		{
			name:          "no changes",
			changeCount:   0,
			linesChanged:  0,
			uniqueAuthors: 0,
			windowDays:    30,
			minExpected:   0.0,
			maxExpected:   0.1,
		},
		{
			name:          "moderate activity",
			changeCount:   15,
			linesChanged:  500,
			uniqueAuthors: 2,
			windowDays:    30,
			minExpected:   0.2,
			maxExpected:   0.6,
		},
		{
			name:          "high activity",
			changeCount:   60,
			linesChanged:  5000,
			uniqueAuthors: 10,
			windowDays:    30,
			minExpected:   0.8,
			maxExpected:   1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateVolatilityScore(tt.changeCount, tt.linesChanged, tt.uniqueAuthors, tt.windowDays)
			if got < tt.minExpected || got > tt.maxExpected {
				t.Errorf("CalculateVolatilityScore() = %v, want between %v and %v", got, tt.minExpected, tt.maxExpected)
			}
		})
	}
}

func TestCalculateCollisionScore(t *testing.T) {
	tests := []struct {
		name          string
		contributors  []ContributorActivity
		recentChanges int
		minExpected   float64
		maxExpected   float64
	}{
		{
			name:          "single contributor",
			contributors:  []ContributorActivity{{AuthorName: "Alice", ChangeCount: 10}},
			recentChanges: 5,
			minExpected:   0.0,
			maxExpected:   0.1,
		},
		{
			name: "two contributors",
			contributors: []ContributorActivity{
				{AuthorName: "Alice", ChangeCount: 10, OwnershipShare: 0.7},
				{AuthorName: "Bob", ChangeCount: 5, OwnershipShare: 0.3},
			},
			recentChanges: 5,
			minExpected:   0.2,
			maxExpected:   0.6,
		},
		{
			name: "many contributors with high activity",
			contributors: []ContributorActivity{
				{AuthorName: "Alice", ChangeCount: 10, OwnershipShare: 0.4},
				{AuthorName: "Bob", ChangeCount: 8, OwnershipShare: 0.32},
				{AuthorName: "Carol", ChangeCount: 5, OwnershipShare: 0.2},
				{AuthorName: "Dave", ChangeCount: 2, OwnershipShare: 0.08},
			},
			recentChanges: 15,
			minExpected:   0.6,
			maxExpected:   1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateCollisionScore(tt.contributors, tt.recentChanges)
			if got < tt.minExpected || got > tt.maxExpected {
				t.Errorf("CalculateCollisionScore() = %v, want between %v and %v", got, tt.minExpected, tt.maxExpected)
			}
		})
	}
}

func TestDetermineCollisionSeverity(t *testing.T) {
	tests := []struct {
		score    float64
		expected FindingSeverity
	}{
		{0.0, SeverityInfo},
		{0.3, SeverityInfo},
		{0.4, SeverityWarning},
		{0.6, SeverityWarning},
		{0.7, SeverityCritical},
		{1.0, SeverityCritical},
	}

	for _, tt := range tests {
		t.Run(string(tt.expected), func(t *testing.T) {
			got := DetermineCollisionSeverity(tt.score)
			if got != tt.expected {
				t.Errorf("DetermineCollisionSeverity(%v) = %v, want %v", tt.score, got, tt.expected)
			}
		})
	}
}

func TestChangeFrequencyParams(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		params := DefaultChangeFrequencyParams()

		if params.TimeWindow != string(Window30Days) {
			t.Errorf("Default TimeWindow = %v, want %v", params.TimeWindow, Window30Days)
		}
		if params.Granularity != string(GranularityFile) {
			t.Errorf("Default Granularity = %v, want %v", params.Granularity, GranularityFile)
		}
		if params.MinChanges != 2 {
			t.Errorf("Default MinChanges = %v, want 2", params.MinChanges)
		}
	})

	t.Run("HasFocus", func(t *testing.T) {
		params := ChangeFrequencyParams{
			Focus: []string{"hotspots", "collisions"},
		}

		if !params.HasFocus(FocusHotspots) {
			t.Error("HasFocus(hotspots) should be true")
		}
		if !params.HasFocus(FocusCollisions) {
			t.Error("HasFocus(collisions) should be true")
		}
		if params.HasFocus(FocusPatterns) {
			t.Error("HasFocus(patterns) should be false")
		}

		// Test "all" focus
		params.Focus = []string{"all"}
		if !params.HasFocus(FocusPatterns) {
			t.Error("HasFocus(patterns) with 'all' should be true")
		}
	})
}

func TestFrequencyCache(t *testing.T) {
	cache := NewFrequencyCache(100 * time.Millisecond)

	t.Run("set and get", func(t *testing.T) {
		cache.Set("key1", "value1")

		got, ok := cache.Get("key1")
		if !ok {
			t.Error("Get should return true for existing key")
		}
		if got != "value1" {
			t.Errorf("Get = %v, want value1", got)
		}
	})

	t.Run("miss", func(t *testing.T) {
		_, ok := cache.Get("nonexistent")
		if ok {
			t.Error("Get should return false for nonexistent key")
		}
	})

	t.Run("expiration", func(t *testing.T) {
		cache.Set("expiring", "value")
		time.Sleep(150 * time.Millisecond)

		_, ok := cache.Get("expiring")
		if ok {
			t.Error("Get should return false for expired key")
		}
	})

	t.Run("stats", func(t *testing.T) {
		stats := cache.Stats()
		if stats.HitRate < 0 || stats.HitRate > 1 {
			t.Errorf("HitRate = %v, want 0-1", stats.HitRate)
		}
	})
}

func TestPatternDetector(t *testing.T) {
	detector := NewPatternDetector()

	t.Run("detect registration function", func(t *testing.T) {
		content := []byte(`
func registerTools() {
	server.AddTool("tool1", handler1)
	server.AddTool("tool2", handler2)
	server.AddTool("tool3", handler3)
	server.AddTool("tool4", handler4)
	server.AddTool("tool5", handler5)
	server.AddTool("tool6", handler6)
	server.AddTool("tool7", handler7)
	server.AddTool("tool8", handler8)
	server.AddTool("tool9", handler9)
	server.AddTool("tool10", handler10)
	server.AddTool("tool11", handler11)
}
`)
		patterns := detector.DetectPatterns(content, "server.go")

		found := false
		for _, p := range patterns {
			if p.Type == PatternRegistrationFunction {
				found = true
				break
			}
		}
		if !found {
			t.Error("Should detect registration function pattern")
		}
	})

	t.Run("detect enum aggregation", func(t *testing.T) {
		// Multiple const blocks and individual consts trigger detection
		content := []byte(`
const A = 1
const B = 2
const C = 3
const D = 4
const E = 5
const (
	F = iota
	G
	H
)
const (
	I = "i"
	J = "j"
	K = "k"
	L = "l"
)
`)
		patterns := detector.DetectPatterns(content, "types.go")

		found := false
		for _, p := range patterns {
			if p.Type == PatternEnumAggregation {
				found = true
				break
			}
		}
		if !found {
			t.Error("Should detect enum aggregation pattern")
		}
	})

	t.Run("detect switch factory", func(t *testing.T) {
		content := []byte(`
func factory(t string) Handler {
	switch t {
	case "a": return handlerA()
	case "b": return handlerB()
	case "c": return handlerC()
	case "d": return handlerD()
	case "e": return handlerE()
	case "f": return handlerF()
	case "g": return handlerG()
	case "h": return handlerH()
	case "i": return handlerI()
	case "j": return handlerJ()
	case "k": return handlerK()
	default: return nil
	}
}
`)
		patterns := detector.DetectPatterns(content, "factory.go")

		found := false
		for _, p := range patterns {
			if p.Type == PatternSwitchFactory {
				found = true
				break
			}
		}
		if !found {
			t.Error("Should detect switch factory pattern")
		}
	})
}

func TestHistoryProvider(t *testing.T) {
	// Skip if not in a git repo
	_, err := os.Stat(".git")
	if err != nil {
		// Try parent directories
		dir, _ := os.Getwd()
		for i := 0; i < 5; i++ {
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
			if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
				break
			}
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
			t.Skip("Not in a git repository")
		}
	}

	provider, err := NewProvider(".")
	if err != nil {
		// Try to find git root
		dir, _ := os.Getwd()
		for i := 0; i < 5; i++ {
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
			provider, err = NewProvider(dir)
			if err == nil {
				break
			}
		}
		if err != nil {
			t.Skip("Cannot create provider: " + err.Error())
		}
	}

	histProvider := NewHistoryProvider(provider)
	ctx := context.Background()

	t.Run("GetCommitHistory", func(t *testing.T) {
		since := time.Now().Add(-30 * 24 * time.Hour)
		commits, err := histProvider.GetCommitHistory(ctx, since)
		if err != nil {
			t.Fatalf("GetCommitHistory failed: %v", err)
		}

		// Should have at least some commits in last 30 days
		// (assuming this is run in an active repo)
		t.Logf("Found %d commits in last 30 days", len(commits))
	})

	t.Run("GetCurrentAuthor", func(t *testing.T) {
		name, email, err := histProvider.GetCurrentAuthor(ctx)
		if err != nil {
			t.Logf("GetCurrentAuthor skipped: %v", err)
			return
		}

		if name == "" {
			t.Error("Expected non-empty author name")
		}
		t.Logf("Current author: %s <%s>", name, email)
	})
}

func TestFrequencyAnalyzer(t *testing.T) {
	// Skip if not in a git repo
	provider, err := NewProvider(".")
	if err != nil {
		// Try parent directories
		dir, _ := os.Getwd()
		for i := 0; i < 5; i++ {
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
			provider, err = NewProvider(dir)
			if err == nil {
				break
			}
		}
		if err != nil {
			t.Skip("Cannot create provider: " + err.Error())
		}
	}

	analyzer := NewFrequencyAnalyzer(provider)
	ctx := context.Background()

	t.Run("Analyze", func(t *testing.T) {
		params := ChangeFrequencyParams{
			TimeWindow:  "7d",
			Granularity: "file",
			Focus:       []string{"hotspots"},
			MinChanges:  1,
			TopN:        10,
		}

		report, err := analyzer.Analyze(ctx, params)
		if err != nil {
			t.Fatalf("Analyze failed: %v", err)
		}

		if report == nil {
			t.Fatal("Expected non-nil report")
		}

		t.Logf("Analyzed %d files, found %d hotspots in %dms",
			report.Summary.TotalFilesAnalyzed,
			report.Summary.HotspotsFound,
			report.Metadata.ComputeTimeMs)
	})

	t.Run("AnalyzeFile", func(t *testing.T) {
		// Analyze a file that should exist
		freq, err := analyzer.AnalyzeFile(ctx, "go.mod", Window30Days)
		if err != nil {
			t.Logf("AnalyzeFile skipped: %v", err)
			return
		}

		if freq == nil {
			t.Fatal("Expected non-nil frequency")
		}

		t.Logf("go.mod: %d changes, %d contributors",
			freq.Metrics[Window30Days].ChangeCount,
			len(freq.Contributors))
	})
}

func TestCacheKeyFunctions(t *testing.T) {
	t.Run("CacheKeyFile", func(t *testing.T) {
		key := CacheKeyFile("path/to/file.go", Window30Days)
		if key != "freq:path/to/file.go:30d:file" {
			t.Errorf("Unexpected key: %s", key)
		}
	})

	t.Run("CacheKeySymbol", func(t *testing.T) {
		key := CacheKeySymbol("path/to/file.go", "MyFunction", Window7Days)
		expected := "freq:path/to/file.go:MyFunction:7d:symbol"
		if key != expected {
			t.Errorf("Got %s, want %s", key, expected)
		}
	})

	t.Run("CacheKeyPattern", func(t *testing.T) {
		key := CacheKeyPattern("internal/**/*.go", Window90Days)
		expected := "freq:pattern:internal/**/*.go:90d"
		if key != expected {
			t.Errorf("Got %s, want %s", key, expected)
		}
	})
}

func BenchmarkCalculateVolatilityScore(b *testing.B) {
	for i := 0; i < b.N; i++ {
		CalculateVolatilityScore(30, 1500, 5, 30)
	}
}

func BenchmarkPatternDetector(b *testing.B) {
	detector := NewPatternDetector()
	content := []byte(`
func registerTools() {
	server.AddTool("tool1", handler1)
	server.AddTool("tool2", handler2)
	server.AddTool("tool3", handler3)
	server.AddTool("tool4", handler4)
	server.AddTool("tool5", handler5)
}
`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detector.DetectPatterns(content, "server.go")
	}
}

func BenchmarkFrequencyCache(b *testing.B) {
	cache := NewFrequencyCache(10 * time.Minute)

	// Pre-populate
	for i := 0; i < 100; i++ {
		cache.Set(CacheKeyFile("file"+string(rune(i))+".go", Window30Days), &FileChangeFrequency{})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := CacheKeyFile("file"+string(rune(i%100))+".go", Window30Days)
		cache.Get(key)
	}
}

func TestShouldExcludeFromChurn(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		want     bool
	}{
		// Files that SHOULD be excluded
		{"changelog", "CHANGELOG.md", true},
		{"changelog lowercase", "changelog.md", true},
		{"history", "HISTORY.md", true},
		{"changes", "CHANGES.md", true},
		{"readme", "README.md", true},
		{"any markdown", "docs/guide.md", true},
		{"rst docs", "docs/index.rst", true},
		{"txt file", "notes.txt", true},
		{"minified js", "app.min.js", true},
		{"minified css", "styles.min.css", true},
		{"bundle js", "vendor.bundle.js", true},
		{"bundle css", "main.bundle.css", true},
		{"d.ts file", "types/index.d.ts", true},
		{"generated file", "api.generated.go", true},
		{"dart freezed", "model.freezed.dart", true},
		{"dart generated", "model.g.dart", true},
		{"package-lock.json", "package-lock.json", true},
		{"yarn.lock", "yarn.lock", true},
		{"pnpm-lock", "pnpm-lock.yaml", true},
		{"go.sum", "go.sum", true},
		{"Cargo.lock", "Cargo.lock", true},
		{"poetry.lock", "poetry.lock", true},
		{"composer.lock", "composer.lock", true},
		{"Gemfile.lock", "Gemfile.lock", true},
		{"dist folder", "dist/bundle.js", true},
		{"build folder", "build/output.js", true},
		{"vendor folder", "vendor/github.com/pkg/errors/errors.go", true},
		{"node_modules", "node_modules/lodash/index.js", true},
		{"third_party", "third_party/protobuf/proto.go", true},
		{"idea folder", ".idea/workspace.xml", true},
		{"vscode folder", ".vscode/settings.json", true},
		{"iml file", "project.iml", true},
		{"github folder", ".github/workflows/ci.yml", true},
		{"gitlab ci", ".gitlab-ci.yml", true},
		{"travis", ".travis.yml", true},
		{"jenkinsfile", "Jenkinsfile", true},
		{"docs folder", "docs/api.md", true},
		{"doc folder", "doc/readme.md", true},

		// Binary build directories
		{"bin folder", "bin/myapp", true},
		{"obj folder", "obj/Debug/app.dll", true},
		{"Debug folder", "Debug/app.exe", true},
		{"Release folder", "Release/app.exe", true},
		{"x64 folder", "x64/Release/lib.dll", true},
		{"artifacts folder", "artifacts/package.zip", true},
		{"pycache", "__pycache__/module.pyc", true},

		// Binary executables and libraries
		{"exe file", "app.exe", true},
		{"dll file", "library.dll", true},
		{"so file", "libfoo.so", true},
		{"dylib file", "libbar.dylib", true},
		{"static lib", "libfoo.a", true},
		{"windows lib", "foo.lib", true},
		{"object file .o", "main.o", true},
		{"object file .obj", "main.obj", true},
		{"python bytecode", "module.pyc", true},
		{"java class", "Main.class", true},
		{"java jar", "app.jar", true},
		{"wasm file", "module.wasm", true},

		// Database files
		{"sqlite db", "data.sqlite", true},
		{"sqlite3 db", "data.sqlite3", true},
		{"bin file", "data.bin", true},

		// Images and media
		{"png image", "logo.png", true},
		{"jpg image", "photo.jpg", true},
		{"svg image", "icon.svg", true},
		{"pdf doc", "manual.pdf", true},
		{"mp3 audio", "sound.mp3", true},
		{"mp4 video", "video.mp4", true},

		// Fonts
		{"woff font", "font.woff", true},
		{"woff2 font", "font.woff2", true},
		{"ttf font", "font.ttf", true},

		// Archives
		{"zip archive", "package.zip", true},
		{"tar archive", "backup.tar", true},
		{"gz archive", "file.gz", true},
		{"tgz archive", "release.tgz", true},

		// Package files
		{"nuget package", "lib.nupkg", true},
		{"ruby gem", "mygem.gem", true},
		{"python wheel", "package.whl", true},

		// Coverage output
		{"coverage folder", "coverage/lcov.info", true},
		{"lcov file", "coverage.lcov", true},
		{"nyc output", ".nyc_output/data.json", true},

		// Files that should NOT be excluded (actual code)
		{"go file", "internal/server/handler.go", false},
		{"ts file", "src/components/Button.tsx", false},
		{"js file", "src/utils/helpers.js", false},
		{"py file", "app/models.py", false},
		{"rust file", "src/lib.rs", false},
		{"java file", "src/main/java/App.java", false},
		{"c file", "src/main.c", false},
		{"cpp file", "src/main.cpp", false},
		{"go test file", "internal/server/handler_test.go", false},
		{"css file", "src/styles/main.css", false},
		{"scss file", "src/styles/main.scss", false},
		{"html file", "templates/index.html", false},
		{"json config", "config.json", false},
		{"yaml config", "config.yaml", false},
		{"toml config", "Cargo.toml", false},
		{"go.mod", "go.mod", false},
		{"package.json", "package.json", false},
		{"tsconfig", "tsconfig.json", false},
		{"Makefile", "Makefile", false},
		{"Dockerfile", "Dockerfile", false},
		{"proto file", "api/service.proto", false},
		{"sql file", "migrations/001_init.sql", false},
		{"shell script", "scripts/build.sh", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldExcludeFromChurn(tt.filePath)
			if got != tt.want {
				t.Errorf("shouldExcludeFromChurn(%q) = %v, want %v", tt.filePath, got, tt.want)
			}
		})
	}
}

func TestShouldExcludeFromChurn_PathVariants(t *testing.T) {
	// Test that both forward and back slashes work
	tests := []struct {
		name  string
		paths []string
		want  bool
	}{
		{
			"vendor with forward slash",
			[]string{"vendor/pkg/file.go", "vendor\\pkg\\file.go"},
			true,
		},
		{
			"dist with mixed slashes",
			[]string{"dist/js/app.js", "dist\\js\\app.js"},
			true,
		},
		{
			"normal code path",
			[]string{"src/app/main.go", "src\\app\\main.go"},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, path := range tt.paths {
				got := shouldExcludeFromChurn(path)
				if got != tt.want {
					t.Errorf("shouldExcludeFromChurn(%q) = %v, want %v", path, got, tt.want)
				}
			}
		})
	}
}

func BenchmarkShouldExcludeFromChurn(b *testing.B) {
	paths := []string{
		"internal/server/handler.go",
		"CHANGELOG.md",
		"vendor/github.com/pkg/errors/errors.go",
		"src/components/Button.tsx",
		"node_modules/lodash/index.js",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, path := range paths {
			shouldExcludeFromChurn(path)
		}
	}
}

func TestShouldExcludeFromChurnWithConfig(t *testing.T) {
	t.Run("include patterns only", func(t *testing.T) {
		config := ChurnFilterConfig{
			IncludePatterns: []string{"*.go", "*.py"},
		}

		tests := []struct {
			path string
			want bool
		}{
			{"main.go", false},            // matches *.go, include
			{"app.py", false},             // matches *.py, include
			{"main.js", true},             // doesn't match, exclude
			{"README.md", true},           // doesn't match, exclude
			{"internal/server.go", false}, // matches *.go, include
		}

		for _, tt := range tests {
			got := shouldExcludeFromChurnWithConfig(tt.path, config)
			if got != tt.want {
				t.Errorf("shouldExcludeFromChurnWithConfig(%q, include=[*.go,*.py]) = %v, want %v", tt.path, got, tt.want)
			}
		}
	})

	t.Run("custom exclude patterns", func(t *testing.T) {
		config := ChurnFilterConfig{
			ExcludePatterns: []string{"*_test.go", "mocks/*"},
		}

		tests := []struct {
			path string
			want bool
		}{
			{"handler.go", false},           // not excluded
			{"handler_test.go", true},       // matches *_test.go
			{"mocks/mock_service.go", true}, // matches mocks/*
			{"pkg/mocks/m.go", true},        // matches */mocks/*
		}

		for _, tt := range tests {
			got := shouldExcludeFromChurnWithConfig(tt.path, config)
			if got != tt.want {
				t.Errorf("shouldExcludeFromChurnWithConfig(%q, exclude=[*_test.go,mocks/*]) = %v, want %v", tt.path, got, tt.want)
			}
		}
	})

	t.Run("skip default exclusions", func(t *testing.T) {
		config := ChurnFilterConfig{
			SkipDefaultExclusions: true,
		}

		tests := []struct {
			path string
			want bool
		}{
			{"CHANGELOG.md", false},       // normally excluded, but defaults skipped
			{"go.sum", false},             // normally excluded, but defaults skipped
			{"vendor/pkg/file.go", false}, // normally excluded, but defaults skipped
			{"main.go", false},            // not excluded
		}

		for _, tt := range tests {
			got := shouldExcludeFromChurnWithConfig(tt.path, config)
			if got != tt.want {
				t.Errorf("shouldExcludeFromChurnWithConfig(%q, skipDefaults=true) = %v, want %v", tt.path, got, tt.want)
			}
		}
	})

	t.Run("include and exclude combined", func(t *testing.T) {
		config := ChurnFilterConfig{
			IncludePatterns: []string{"*.go"},
			ExcludePatterns: []string{"*_test.go"},
		}

		tests := []struct {
			path string
			want bool
		}{
			{"main.go", false},     // matches include, not exclude
			{"main_test.go", true}, // matches include AND exclude, exclude wins
			{"app.py", true},       // doesn't match include
		}

		for _, tt := range tests {
			got := shouldExcludeFromChurnWithConfig(tt.path, config)
			if got != tt.want {
				t.Errorf("shouldExcludeFromChurnWithConfig(%q, include=[*.go], exclude=[*_test.go]) = %v, want %v", tt.path, got, tt.want)
			}
		}
	})
}
