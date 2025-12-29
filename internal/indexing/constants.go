package indexing

// Project marker constants for detecting project roots.
// These are checked in priority order when determining the project root directory.

// LCI configuration markers - highest priority
// These take precedence over all other markers to ensure user-defined
// project boundaries and exclusion patterns are respected.
var LCIConfigMarkers = []string{
	".lci.kdl",   // LCI KDL config (preferred format)
	".lciconfig", // Legacy LCI config
}

// Primary project markers - strong indicators of project roots
// These are well-known project definition files that reliably indicate
// the root of a software project.
var PrimaryProjectMarkers = []string{
	".git",             // Git repository (strongest general indicator)
	"go.mod",           // Go module
	"package.json",     // Node.js project
	"Cargo.toml",       // Rust project
	"requirements.txt", // Python project
	"pyproject.toml",   // Modern Python project
	"pom.xml",          // Maven project
	"build.gradle",     // Gradle project
	"setup.py",         // Python setup
	"composer.json",    // PHP project
	"Gemfile",          // Ruby project
}

// Secondary project markers - weaker indicators but still useful
// These files often exist at project roots but are less definitive.
var SecondaryProjectMarkers = []string{
	"Makefile",           // Make-based project
	"CMakeLists.txt",     // CMake project
	"docker-compose.yml", // Docker compose
	"Dockerfile",         // Docker project
	".dockerignore",      // Docker project
	"README.md",          // Documented project
	"README.rst",         // Documented project
	"README",             // Documented project
	"LICENSE",            // Licensed project
	"LICENSE.md",         // Licensed project
	".gitignore",         // Git-aware project (even if no .git)
	"tsconfig.json",      // TypeScript project
	"webpack.config.js",  // Webpack project
	"vite.config.ts",     // Vite project
	".eslintrc.js",       // ESLint project
	".prettierrc",        // Prettier project
}

// Source directory names - used as tertiary fallback for project detection
// When no explicit project markers are found, the presence of multiple
// source directories suggests a project root.
var SourceDirectoryNames = []string{
	"src", "lib", "app", "components", "pages", "utils",
	"handlers", "services", "models", "controllers", "views",
}

// SourceDirectoryThreshold is the minimum number of source directories
// required to consider a path as a project root when no other markers are found.
const SourceDirectoryThreshold = 2

// SourceFileExtensions defines file extensions eligible for indexing.
// This is the canonical list of extensions that the indexer will process.
// For search ranking extensions (which may include additional languages),
// see internal/search/engine.go:codeExtensions.
var SourceFileExtensions = map[string]bool{
	// Go
	".go": true,
	// JavaScript/TypeScript
	".js": true, ".ts": true, ".jsx": true, ".tsx": true,
	// Python
	".py": true,
	// Rust
	".rs": true,
	// Java/JVM
	".java": true, ".kt": true, ".scala": true,
	// C/C++
	".c": true, ".cpp": true, ".cc": true, ".cxx": true,
	".h": true, ".hpp": true, ".hxx": true,
	// C#
	".cs": true,
	// PHP
	".php": true,
	// Ruby
	".rb": true,
	// Swift
	".swift": true,
	// Zig
	".zig": true,
	// Frontend frameworks
	".vue": true, ".svelte": true,
	// Dart
	".dart": true,
	// Configuration files (needed for grep compatibility)
	".json": true, ".toml": true, ".yaml": true, ".yml": true,
	".mod": true, // Go modules
	".xml": true, ".ini": true, ".conf": true, ".config": true,
	".lock": true, // Package lock files
	".md": true, ".txt": true, // Documentation
}
