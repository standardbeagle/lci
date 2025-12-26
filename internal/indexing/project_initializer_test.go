package indexing

import (
	"io/fs"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/standardbeagle/lci/internal/core"
)

// MockFileSystem implements core.FileSystemInterface for testing
type MockFileSystem struct {
	// files maps path -> content (nil content means directory)
	files map[string][]byte
	// dirs tracks which paths are directories
	dirs map[string]bool
}

// NewMockFileSystem creates a new mock filesystem
func NewMockFileSystem() *MockFileSystem {
	return &MockFileSystem{
		files: make(map[string][]byte),
		dirs:  make(map[string]bool),
	}
}

// AddFile adds a file to the mock filesystem
func (m *MockFileSystem) AddFile(path string, content []byte) {
	m.files[path] = content
}

// AddDir adds a directory to the mock filesystem
func (m *MockFileSystem) AddDir(path string) {
	m.dirs[path] = true
}

// mockFileInfo implements fs.FileInfo for testing
type mockFileInfo struct {
	name  string
	size  int64
	isDir bool
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return m.size }
func (m *mockFileInfo) Mode() fs.FileMode  { return 0644 }
func (m *mockFileInfo) ModTime() time.Time { return time.Now() }
func (m *mockFileInfo) IsDir() bool        { return m.isDir }
func (m *mockFileInfo) Sys() interface{}   { return nil }

func (m *MockFileSystem) Stat(path string) (fs.FileInfo, error) {
	if m.dirs[path] {
		return &mockFileInfo{name: path, isDir: true}, nil
	}
	if content, ok := m.files[path]; ok {
		return &mockFileInfo{name: path, size: int64(len(content)), isDir: false}, nil
	}
	return nil, fs.ErrNotExist
}

func (m *MockFileSystem) ReadFile(path string) ([]byte, error) {
	if content, ok := m.files[path]; ok {
		return content, nil
	}
	return nil, fs.ErrNotExist
}

func (m *MockFileSystem) ReadDir(path string) ([]fs.DirEntry, error) {
	return nil, nil // Not needed for these tests
}

func (m *MockFileSystem) Exists(path string) bool {
	if m.dirs[path] {
		return true
	}
	_, ok := m.files[path]
	return ok
}

func (m *MockFileSystem) IsDir(path string) bool {
	return m.dirs[path]
}

func (m *MockFileSystem) IsFile(path string) bool {
	_, ok := m.files[path]
	return ok
}

// Helper to create a ProjectInitializer with mock filesystem
func newTestProjectInitializer(mockFS *MockFileSystem) *ProjectInitializer {
	fileService := core.NewFileServiceWithOptions(core.FileServiceOptions{
		FileSystem: mockFS,
	})
	return NewProjectInitializerWithFileService(fileService)
}

// TestDetectProjectRoot_LCIConfigPriority verifies that .lci.kdl takes
// precedence over other project markers like .git
func TestDetectProjectRoot_LCIConfigPriority(t *testing.T) {
	tests := []struct {
		name       string
		setupFS    func(*MockFileSystem)
		path       string
		wantIsRoot bool
		wantMarker string
	}{
		{
			name: "lci.kdl takes priority over git",
			setupFS: func(m *MockFileSystem) {
				m.AddDir("/project")
				m.AddFile("/project/.lci.kdl", []byte("project {}"))
				m.AddDir("/project/.git")
			},
			path:       "/project",
			wantIsRoot: true,
			wantMarker: ".lci.kdl",
		},
		{
			name: "lciconfig takes priority over git",
			setupFS: func(m *MockFileSystem) {
				m.AddDir("/project")
				m.AddFile("/project/.lciconfig", []byte("{}"))
				m.AddDir("/project/.git")
			},
			path:       "/project",
			wantIsRoot: true,
			wantMarker: ".lciconfig",
		},
		{
			name: "lci.kdl preferred over lciconfig",
			setupFS: func(m *MockFileSystem) {
				m.AddDir("/project")
				m.AddFile("/project/.lci.kdl", []byte("project {}"))
				m.AddFile("/project/.lciconfig", []byte("{}"))
			},
			path:       "/project",
			wantIsRoot: true,
			wantMarker: ".lci.kdl",
		},
		{
			name: "git detected when no lci config",
			setupFS: func(m *MockFileSystem) {
				m.AddDir("/project")
				m.AddDir("/project/.git")
			},
			path:       "/project",
			wantIsRoot: true,
			wantMarker: ".git",
		},
		{
			name: "go.mod detected when no lci config or git",
			setupFS: func(m *MockFileSystem) {
				m.AddDir("/project")
				m.AddFile("/project/go.mod", []byte("module example"))
			},
			path:       "/project",
			wantIsRoot: true,
			wantMarker: "go.mod",
		},
		{
			name: "package.json detected",
			setupFS: func(m *MockFileSystem) {
				m.AddDir("/project")
				m.AddFile("/project/package.json", []byte("{}"))
			},
			path:       "/project",
			wantIsRoot: true,
			wantMarker: "package.json",
		},
		{
			name: "empty directory is not project root",
			setupFS: func(m *MockFileSystem) {
				m.AddDir("/empty")
			},
			path:       "/empty",
			wantIsRoot: false,
			wantMarker: "",
		},
		{
			name: "empty path returns false",
			setupFS: func(m *MockFileSystem) {
				m.AddDir("/project")
			},
			path:       "",
			wantIsRoot: false,
			wantMarker: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFS := NewMockFileSystem()
			tt.setupFS(mockFS)

			pi := newTestProjectInitializer(mockFS)
			gotIsRoot, gotMarker := pi.DetectProjectRoot(tt.path)

			assert.Equal(t, tt.wantIsRoot, gotIsRoot, "isRoot mismatch")
			assert.Equal(t, tt.wantMarker, gotMarker, "marker mismatch")
		})
	}
}

// TestFindProjectRoot_LCIConfigInParent verifies that LCI config in parent
// directory takes precedence over .git in child directory
func TestFindProjectRoot_LCIConfigInParent(t *testing.T) {
	tests := []struct {
		name       string
		setupFS    func(*MockFileSystem)
		startPath  string
		wantRoot   string
		wantMarker string
		wantErr    bool
	}{
		{
			name: "parent lci.kdl takes precedence over child git",
			setupFS: func(m *MockFileSystem) {
				// Parent has .lci.kdl
				m.AddDir("/parent")
				m.AddFile("/parent/.lci.kdl", []byte("exclude { \"**/child/**\" }"))
				// Child has .git (like real_projects subdirectories)
				m.AddDir("/parent/child")
				m.AddDir("/parent/child/.git")
			},
			startPath:  "/parent/child",
			wantRoot:   "/parent",
			wantMarker: ".lci.kdl",
			wantErr:    false,
		},
		{
			name: "deeply nested path finds parent lci.kdl",
			setupFS: func(m *MockFileSystem) {
				m.AddDir("/root")
				m.AddFile("/root/.lci.kdl", []byte("project {}"))
				m.AddDir("/root/a")
				m.AddDir("/root/a/b")
				m.AddDir("/root/a/b/c")
				m.AddDir("/root/a/b/c/.git") // Nested git repo
			},
			startPath:  "/root/a/b/c",
			wantRoot:   "/root",
			wantMarker: ".lci.kdl",
			wantErr:    false,
		},
		{
			name: "no lci config falls back to nearest git",
			setupFS: func(m *MockFileSystem) {
				m.AddDir("/parent")
				m.AddDir("/parent/.git")
				m.AddDir("/parent/child")
				m.AddDir("/parent/child/.git")
			},
			startPath:  "/parent/child",
			wantRoot:   "/parent/child",
			wantMarker: ".git",
			wantErr:    false,
		},
		{
			name: "lciconfig in grandparent",
			setupFS: func(m *MockFileSystem) {
				m.AddDir("/gp")
				m.AddFile("/gp/.lciconfig", []byte("{}"))
				m.AddDir("/gp/parent")
				m.AddDir("/gp/parent/child")
				m.AddFile("/gp/parent/child/go.mod", []byte("module x"))
			},
			startPath:  "/gp/parent/child",
			wantRoot:   "/gp",
			wantMarker: ".lciconfig",
			wantErr:    false,
		},
		{
			name: "no project root found",
			setupFS: func(m *MockFileSystem) {
				m.AddDir("/lonely")
				m.AddDir("/lonely/path")
			},
			startPath:  "/lonely/path",
			wantRoot:   "",
			wantMarker: "",
			wantErr:    true,
		},
		{
			name: "empty start path returns error",
			setupFS: func(m *MockFileSystem) {
				m.AddDir("/project")
				m.AddFile("/project/.lci.kdl", []byte("{}"))
			},
			startPath:  "",
			wantRoot:   "",
			wantMarker: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFS := NewMockFileSystem()
			tt.setupFS(mockFS)

			pi := newTestProjectInitializer(mockFS)
			gotRoot, gotMarker, err := pi.FindProjectRoot(tt.startPath)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantRoot, gotRoot, "root path mismatch")
			assert.Equal(t, tt.wantMarker, gotMarker, "marker mismatch")
		})
	}
}

// TestFindProjectRoot_RealProjectsScenario tests the exact scenario that was
// causing issues: starting from real_projects/typescript/trpc which has its
// own .git, but the parent lightning-code-index has .lci.kdl with exclusions
func TestFindProjectRoot_RealProjectsScenario(t *testing.T) {
	mockFS := NewMockFileSystem()

	// Setup: lightning-code-index project structure
	m := mockFS
	m.AddDir("/home/user/lightning-code-index")
	m.AddFile("/home/user/lightning-code-index/.lci.kdl", []byte(`
exclude {
    "**/real_projects/**"
    "**/testdata/**"
}
`))
	m.AddDir("/home/user/lightning-code-index/.git")
	m.AddFile("/home/user/lightning-code-index/go.mod", []byte("module lci"))

	// Nested real_projects with their own git repos
	m.AddDir("/home/user/lightning-code-index/real_projects")
	m.AddDir("/home/user/lightning-code-index/real_projects/typescript")
	m.AddDir("/home/user/lightning-code-index/real_projects/typescript/trpc")
	m.AddDir("/home/user/lightning-code-index/real_projects/typescript/trpc/.git")
	m.AddFile("/home/user/lightning-code-index/real_projects/typescript/trpc/package.json", []byte("{}"))

	pi := newTestProjectInitializer(mockFS)

	// When starting from the nested trpc directory
	root, marker, err := pi.FindProjectRoot("/home/user/lightning-code-index/real_projects/typescript/trpc")

	require.NoError(t, err)
	// Should find the parent .lci.kdl, NOT the nested .git
	assert.Equal(t, "/home/user/lightning-code-index", root,
		"should find parent directory with .lci.kdl, not nested git repo")
	assert.Equal(t, ".lci.kdl", marker,
		"should detect .lci.kdl marker, not .git")
}

// TestDetectProjectRoot_SecondaryMarkers tests detection of secondary markers
// like Makefile, README.md, etc.
func TestDetectProjectRoot_SecondaryMarkers(t *testing.T) {
	tests := []struct {
		name       string
		setupFS    func(*MockFileSystem)
		path       string
		wantMarker string
	}{
		{
			name: "Makefile detected",
			setupFS: func(m *MockFileSystem) {
				m.AddDir("/project")
				m.AddFile("/project/Makefile", []byte("all:"))
			},
			path:       "/project",
			wantMarker: "Makefile",
		},
		{
			name: "README.md detected",
			setupFS: func(m *MockFileSystem) {
				m.AddDir("/project")
				m.AddFile("/project/README.md", []byte("# Project"))
			},
			path:       "/project",
			wantMarker: "README.md",
		},
		{
			name: "tsconfig.json detected",
			setupFS: func(m *MockFileSystem) {
				m.AddDir("/project")
				m.AddFile("/project/tsconfig.json", []byte("{}"))
			},
			path:       "/project",
			wantMarker: "tsconfig.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFS := NewMockFileSystem()
			tt.setupFS(mockFS)

			pi := newTestProjectInitializer(mockFS)
			gotIsRoot, gotMarker := pi.DetectProjectRoot(tt.path)

			assert.True(t, gotIsRoot, "should detect as project root")
			assert.Equal(t, tt.wantMarker, gotMarker, "marker mismatch")
		})
	}
}

// TestConstants_ProjectMarkers validates that the marker constants are properly defined
// and contain the expected values. This ensures constants aren't accidentally modified.
func TestConstants_ProjectMarkers(t *testing.T) {
	t.Run("LCI config markers defined and ordered correctly", func(t *testing.T) {
		require.NotEmpty(t, LCIConfigMarkers, "LCIConfigMarkers should not be empty")
		assert.Equal(t, ".lci.kdl", LCIConfigMarkers[0], ".lci.kdl should be first (preferred)")
		assert.Equal(t, ".lciconfig", LCIConfigMarkers[1], ".lciconfig should be second (legacy)")
	})

	t.Run("primary markers include essential project files", func(t *testing.T) {
		require.NotEmpty(t, PrimaryProjectMarkers, "PrimaryProjectMarkers should not be empty")

		// Check that critical markers are present
		essentialMarkers := []string{".git", "go.mod", "package.json", "Cargo.toml"}
		for _, marker := range essentialMarkers {
			assert.Contains(t, PrimaryProjectMarkers, marker,
				"PrimaryProjectMarkers should contain %s", marker)
		}
	})

	t.Run("secondary markers include common project files", func(t *testing.T) {
		require.NotEmpty(t, SecondaryProjectMarkers, "SecondaryProjectMarkers should not be empty")

		commonMarkers := []string{"Makefile", "README.md", "LICENSE", "tsconfig.json"}
		for _, marker := range commonMarkers {
			assert.Contains(t, SecondaryProjectMarkers, marker,
				"SecondaryProjectMarkers should contain %s", marker)
		}
	})

	t.Run("source directory names defined", func(t *testing.T) {
		require.NotEmpty(t, SourceDirectoryNames, "SourceDirectoryNames should not be empty")
		assert.Contains(t, SourceDirectoryNames, "src", "should include 'src'")
		assert.Contains(t, SourceDirectoryNames, "lib", "should include 'lib'")
	})

	t.Run("source file extensions defined", func(t *testing.T) {
		require.NotEmpty(t, SourceFileExtensions, "SourceFileExtensions should not be empty")

		// Check common extensions
		commonExts := []string{".go", ".js", ".ts", ".py", ".rs", ".java"}
		for _, ext := range commonExts {
			assert.True(t, SourceFileExtensions[ext],
				"SourceFileExtensions should include %s", ext)
		}
	})

	t.Run("source directory threshold is reasonable", func(t *testing.T) {
		assert.GreaterOrEqual(t, SourceDirectoryThreshold, 1,
			"threshold should be at least 1")
		assert.LessOrEqual(t, SourceDirectoryThreshold, 5,
			"threshold should not be too high")
	})
}
