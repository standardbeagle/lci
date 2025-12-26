package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	kdl "github.com/sblinch/kdl-go"
	"github.com/sblinch/kdl-go/document"
)

// LoadKDL attempts to load configuration from .lci.kdl file
func LoadKDL(projectRoot string) (*Config, error) {
	kdlPath := filepath.Join(projectRoot, ".lci.kdl")

	// Check if .lci.kdl exists
	if _, err := os.Stat(kdlPath); os.IsNotExist(err) {
		return nil, nil // No KDL config found, use defaults
	}

	content, err := os.ReadFile(kdlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read .lci.kdl: %v", err)
	}

	cfg, err := parseKDL(string(content))
	if err != nil {
		return nil, err
	}

	// Ensure root path is absolute for consistent path handling
	// Resolve relative paths relative to the directory containing the .lci.kdl file
	if cfg != nil && cfg.Project.Root != "" {
		var absRoot string
		if filepath.IsAbs(cfg.Project.Root) {
			absRoot = cfg.Project.Root
		} else {
			// Resolve relative to the projectRoot directory (where .lci.kdl is)
			absRoot = filepath.Join(projectRoot, cfg.Project.Root)
		}
		// Clean the path to resolve . and ..
		cfg.Project.Root = filepath.Clean(absRoot)
	} else if cfg != nil {
		// If no root specified in KDL, use the projectRoot parameter
		absRoot, err := filepath.Abs(projectRoot)
		if err == nil {
			cfg.Project.Root = absRoot
		} else {
			cfg.Project.Root = projectRoot
		}
	}

	return cfg, nil
}

// Simple KDL parser for LCI configuration
func parseKDL(content string) (*Config, error) {
	// Default to absolute current working directory
	defaultRoot, _ := os.Getwd()
	if defaultRoot == "" {
		defaultRoot = "."
	}

	cfg := &Config{
		Version: 1,
		Project: Project{Root: defaultRoot},
		Index: Index{
			MaxFileSize:      10 * 1024 * 1024,
			MaxTotalSizeMB:   500,
			MaxFileCount:     10000,
			FollowSymlinks:   false,
			SmartSizeControl: true,
			PriorityMode:     "recent",
		},
		Performance: Performance{
			MaxMemoryMB:   500,
			MaxGoroutines: 4,
			DebounceMs:    100,
		},
		Search: Search{
			DefaultContextLines:    0,
			MaxResults:             100,
			EnableFuzzy:            true,
			MaxContextLines:        100,
			MergeFileResults:       true,
			EnsureCompleteStmt:     false,
			IncludeLeadingComments: true,
			Ranking: SearchRanking{
				Enabled:          true,
				CodeFileBoost:    DefaultCodeFileBoost,
				DocFilePenalty:   DefaultDocFilePenalty,
				ConfigFileBoost:  DefaultConfigFileBoost,
				RequireSymbol:    false,
				NonSymbolPenalty: DefaultNonSymbolPenalty,
			},
		},
		Include: []string{}, // No include patterns - include everything by default, filtered only by exclusions
		Exclude: []string{}, // Minimal exclusions - add test data and build output exclusions in project .lci.kdl if needed
	}

	doc, err := kdl.Parse(strings.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse KDL config: %w", err)
	}

	for _, n := range doc.Nodes {
		switch nodeName(n) {
		case "project":
			for _, cn := range n.Children { // project { root "." name "foo" }
				assignSimpleString(cn, "root", func(v string) { cfg.Project.Root = v })
				assignSimpleString(cn, "name", func(v string) { cfg.Project.Name = v })
			}
		case "index":
			for _, cn := range n.Children {
				name := nodeName(cn)
				switch name {
				case "max_file_size":
					if v, ok := firstIntArg(cn); ok {
						cfg.Index.MaxFileSize = int64(v)
					}
					if s, ok := firstStringArg(cn); ok {
						if sz, err := parseSize(s); err == nil {
							cfg.Index.MaxFileSize = sz
						}
					}
				case "max_total_size_mb":
					if v, ok := firstIntArg(cn); ok {
						cfg.Index.MaxTotalSizeMB = int64(v)
					}
				case "max_file_count":
					if v, ok := firstIntArg(cn); ok {
						cfg.Index.MaxFileCount = v
					}
				case "follow_symlinks":
					if b, ok := firstBoolArg(cn); ok {
						cfg.Index.FollowSymlinks = b
					}
				case "smart_size_control":
					if b, ok := firstBoolArg(cn); ok {
						cfg.Index.SmartSizeControl = b
					}
				case "priority_mode":
					if s, ok := firstStringArg(cn); ok {
						cfg.Index.PriorityMode = s
					}
				case "respect_gitignore":
					if b, ok := firstBoolArg(cn); ok {
						cfg.Index.RespectGitignore = b
					}
				case "watch_mode":
					if b, ok := firstBoolArg(cn); ok {
						cfg.Index.WatchMode = b
					}
				case "watch_debounce_ms":
					if v, ok := firstIntArg(cn); ok {
						cfg.Index.WatchDebounceMs = v
					}
				case "cache_dir":
					// cache_dir removed - persistence no longer supported
				}
			}
		case "performance":
			for _, cn := range n.Children {
				switch nodeName(cn) {
				case "max_memory_mb":
					if v, ok := firstIntArg(cn); ok {
						cfg.Performance.MaxMemoryMB = v
					}
				case "max_goroutines":
					if v, ok := firstIntArg(cn); ok {
						cfg.Performance.MaxGoroutines = v
					}
				case "debounce_ms":
					if v, ok := firstIntArg(cn); ok {
						cfg.Performance.DebounceMs = v
					}
				case "startup_delay_ms":
					if v, ok := firstIntArg(cn); ok {
						cfg.Performance.StartupDelayMs = v
					}
				}
			}
		case "search":
			for _, cn := range n.Children {
				switch nodeName(cn) {
				case "max_results":
					if v, ok := firstIntArg(cn); ok {
						cfg.Search.MaxResults = v
					}
				case "max_context_lines":
					if v, ok := firstIntArg(cn); ok {
						cfg.Search.MaxContextLines = v
					}
				case "enable_fuzzy":
					if b, ok := firstBoolArg(cn); ok {
						cfg.Search.EnableFuzzy = b
					}
				case "merge_file_results":
					if b, ok := firstBoolArg(cn); ok {
						cfg.Search.MergeFileResults = b
					}
				case "ensure_complete_stmt":
					if b, ok := firstBoolArg(cn); ok {
						cfg.Search.EnsureCompleteStmt = b
					}
				case "include_leading_comments":
					if b, ok := firstBoolArg(cn); ok {
						cfg.Search.IncludeLeadingComments = b
					}
				case "ranking":
					// Parse ranking block for file type and symbol preferences
					for _, rn := range cn.Children {
						switch nodeName(rn) {
						case "enabled":
							if b, ok := firstBoolArg(rn); ok {
								cfg.Search.Ranking.Enabled = b
							}
						case "code_file_boost":
							if v, ok := firstFloatArg(rn); ok {
								cfg.Search.Ranking.CodeFileBoost = v
							}
						case "doc_file_penalty":
							if v, ok := firstFloatArg(rn); ok {
								cfg.Search.Ranking.DocFilePenalty = v
							}
						case "config_file_boost":
							if v, ok := firstFloatArg(rn); ok {
								cfg.Search.Ranking.ConfigFileBoost = v
							}
						case "require_symbol":
							if b, ok := firstBoolArg(rn); ok {
								cfg.Search.Ranking.RequireSymbol = b
							}
						case "non_symbol_penalty":
							if v, ok := firstFloatArg(rn); ok {
								cfg.Search.Ranking.NonSymbolPenalty = v
							}
						}
					}
				}
			}
		case "include":
			cfg.Include = append(cfg.Include, collectStringArgs(n)...)
		case "exclude":
			// Replace default exclusions if exclude block is present
			// This allows global config to specify its own exclusions
			cfg.Exclude = collectStringArgs(n)
		case "propagation_config_dir":
			if s, ok := firstStringArg(n); ok {
				cfg.PropagationConfigDir = s
			}
		}
	}

	// Enrich exclusions with language-specific build artifacts
	cfg.EnrichExclusionsWithBuildArtifacts()

	return cfg, nil
}

// Helper functions leveraging kdl-go document model (simple copies from propagation config helpers)
func nodeName(n *document.Node) string {
	if n == nil || n.Name == nil {
		return ""
	}
	return n.Name.NodeNameString()
}
func firstIntArg(n *document.Node) (int, bool) {
	if len(n.Arguments) == 0 {
		return 0, false
	}
	switch v := n.Arguments[0].Value.(type) {
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}
func firstStringArg(n *document.Node) (string, bool) {
	if len(n.Arguments) == 0 {
		return "", false
	}
	if s, ok := n.Arguments[0].Value.(string); ok {
		return s, true
	}
	return "", false
}
func firstBoolArg(n *document.Node) (bool, bool) {
	if len(n.Arguments) == 0 {
		return false, false
	}
	if b, ok := n.Arguments[0].Value.(bool); ok {
		return b, true
	}
	return false, false
}
func firstFloatArg(n *document.Node) (float64, bool) {
	if len(n.Arguments) == 0 {
		return 0, false
	}
	switch v := n.Arguments[0].Value.(type) {
	case float64:
		return v, true
	case int64:
		return float64(v), true
	default:
		nodeName := nodeName(n)
		log.Printf("WARNING: invalid float value for '%s' in KDL config, expected number but got %T", nodeName, n.Arguments[0].Value)
		return 0, false
	}
}
func collectStringArgs(n *document.Node) []string {
	if n == nil {
		return nil
	}
	// First try to collect from arguments (for inline format)
	out := make([]string, 0, len(n.Arguments))
	for _, a := range n.Arguments {
		if s, ok := a.Value.(string); ok {
			out = append(out, s)
		}
	}

	// If no arguments, collect from children (for block format like exclude { "pattern" })
	// In KDL block format, strings are child nodes where the node name is the string value
	if len(out) == 0 && len(n.Children) > 0 {
		out = make([]string, 0, len(n.Children))
		for _, child := range n.Children {
			// Try to get string from arguments first
			if s, ok := firstStringArg(child); ok {
				out = append(out, s)
			} else if child.Name != nil {
				// If no arguments, the node name itself is the string value
				if s, ok := child.Name.Value.(string); ok {
					out = append(out, s)
				}
			}
		}
	}

	return out
}
func assignSimpleString(n *document.Node, target string, set func(string)) {
	if nodeName(n) == target {
		if s, ok := firstStringArg(n); ok {
			set(s)
		}
	}
}

func parseIndexSection(cfg *Config, key, value string) error {
	switch key {
	case "max_file_size":
		if size, err := parseSize(value); err == nil {
			cfg.Index.MaxFileSize = size
		}
	case "max_total_size_mb":
		if mb, err := strconv.ParseInt(value, 10, 64); err == nil {
			cfg.Index.MaxTotalSizeMB = mb
		}
	case "max_file_count":
		if count, err := strconv.Atoi(value); err == nil {
			cfg.Index.MaxFileCount = count
		}
	case "smart_size_control":
		cfg.Index.SmartSizeControl = parseBool(value)
	case "priority_mode":
		cfg.Index.PriorityMode = value
	case "follow_symlinks":
		cfg.Index.FollowSymlinks = parseBool(value)
	case "respect_gitignore":
		cfg.Index.RespectGitignore = parseBool(value)
	}
	return nil
}

// Legacy helper removals: index/performance/search/project parsing now performed via KDL AST traversal

// parseSize handles size strings like "10MB", "500KB", "1GB"
func parseSize(s string) (int64, error) {
	s = strings.ToUpper(strings.TrimSpace(s))

	var multiplier int64 = 1
	var numStr string

	switch {
	case strings.HasSuffix(s, "GB"):
		multiplier = 1024 * 1024 * 1024
		numStr = strings.TrimSuffix(s, "GB")
	case strings.HasSuffix(s, "MB"):
		multiplier = 1024 * 1024
		numStr = strings.TrimSuffix(s, "MB")
	case strings.HasSuffix(s, "KB"):
		multiplier = 1024
		numStr = strings.TrimSuffix(s, "KB")
	case strings.HasSuffix(s, "B"):
		multiplier = 1
		numStr = strings.TrimSuffix(s, "B")
	default:
		numStr = s
	}

	num, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0, err
	}

	return num * multiplier, nil
}

func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "true" || s == "yes" || s == "1" || s == "on"
}

func getDefaultExclusions() []string {
	return []string{
		// Hidden directories (catch-all for dot directories)
		"**/.*/**", // All hidden directories

		// Package managers & dependencies
		"**/node_modules/**",
		"**/vendor/**",
		"**/bower_components/**",
		"**/jspm_packages/**",
		"**/.bundle/**",       // Ruby bundler
		"**/packages/**",      // NuGet, Composer
		"**/.gradle/**",       // Gradle cache
		"**/.m2/**",           // Maven cache
		"**/.ivy2/**",         // Ivy cache
		"**/.cargo/**",        // Rust cargo cache
		"**/venv/**",          // Python virtual environments
		"**/virtualenv/**",    // Python virtual environments
		"**/.venv/**",         // Python virtual environments
		"**/conda/**",         // Conda environments
		"**/site-packages/**", // Python packages
		"**/pkg/**",           // Go packages (some projects)
		"**/Pods/**",          // CocoaPods (iOS)
		"**/Carthage/**",      // Carthage (iOS)
		"**/SPM/**",           // Swift Package Manager

		// Build artifacts & output
		"**/dist/**",
		"**/build/**",
		"**/out/**",
		"**/target/**", // Rust, Java
		"**/bin/**",
		"**/obj/**",    // .NET
		"**/ui/**",     // Web UI build artifacts
		"**/public/**", // Static assets
		"**/output/**",
		"**/Release/**", // Visual Studio
		"**/Debug/**",   // Visual Studio
		"**/*.min.js",
		"**/*.min.css",
		"**/*.bundle.js",
		"**/*.chunk.js",
		"**/CMakeFiles/**", // CMake
		"**/*.build/**",    // Xcode

		// Editor temp files
		"**/*.swp",
		"**/*.swo",
		"**/*~",
		"**/*.tmp",
		"**/*.temp",
		"**/*.bak",
		"**/*.orig",

		// Python compiled files
		"**/__pycache__/**",
		"**/*.pyc",
		"**/*.pyo",
		"**/*.pyd",
		"**/*.egg-info/**",
		"**/*.eggs/**",
		"**/.pytest_cache/**",
		"**/.mypy_cache/**",
		"**/.ruff_cache/**",

		// OS-specific files - Windows
		"**/Thumbs.db",
		"**/desktop.ini",
		"**/*.lnk",
		"**/*.sys",
		"**/*.msi",
		"**/*.msix",
		"**/*.appx",
		"**/*.cab",
		"**/*.msp",
		"**/*.msm",
		"**/System Volume Information/**",
		"**/$RECYCLE.BIN/**",

		// OS-specific files - macOS
		"**/.DS_Store",
		"**/.AppleDouble",
		"**/.LSOverride",
		"**/._*",
		"**/.DocumentRevisions-V100/**",
		"**/.fseventsd/**",
		"**/.Spotlight-V100/**",
		"**/.TemporaryItems/**",
		"**/.Trashes/**",
		"**/.VolumeIcon.icns",
		"**/.com.apple.timemachine.donotpresent/**",
		"**/.AppleDB/**",
		"**/.AppleDesktop/**",

		// OS-specific files - Linux
		"**/.directory",
		"**/.Trash-*/**",

		// Windows development artifacts
		"**/*.exe",
		"**/*.dll",
		"**/*.pdb", // Debug symbols
		"**/*.ilk", // Incremental linking
		"**/*.lib",
		"**/*.exp",
		"**/*.manifest",
		"**/*.res",
		"**/*.obj",
		"**/*.suo",  // Visual Studio user options
		"**/*.user", // Visual Studio user settings
		"**/*.sln.docstates",
		"**/*.vspscc",
		"**/*.vssscc",
		"**/.vs/**",         // Visual Studio cache
		"**/.vscode/**",     // VS Code settings (often user-specific)
		"**/ipch/**",        // Intellisense precompiled headers
		"**/*.aps",          // Resource compiler
		"**/*.ncb",          // Intellisense database
		"**/*.opendb",       // VS database
		"**/*.opensdf",      // VS database
		"**/*.sdf",          // VS database
		"**/*.cachefile",    // VS cache
		"**/*.VC.db",        // VS database
		"**/*.VC.opendb",    // VS database
		"**/x64/**",         // Build output
		"**/x86/**",         // Build output
		"**/ARM/**",         // Build output
		"**/ARM64/**",       // Build output
		"**/.nuget/**",      // NuGet cache
		"**/TestResults/**", // Test results
		"**/*.nupkg",        // NuGet packages
		"**/*.snupkg",       // NuGet symbol packages

		// Linux/Unix development artifacts
		"**/*.so",        // Shared objects
		"**/*.so.*",      // Versioned shared objects
		"**/*.a",         // Static libraries
		"**/*.o",         // Object files
		"**/*.ko",        // Kernel modules
		"**/*.dylib",     // Dynamic libraries (also macOS)
		"**/core",        // Core dumps
		"**/core.*",      // Core dumps
		"**/*.core",      // Core dumps
		"**/vgcore.*",    // Valgrind core dumps
		"**/*.stackdump", // Cygwin stack dumps

		// macOS/iOS development artifacts
		"**/*.app/**",                  // Application bundles
		"**/*.framework/**",            // Framework bundles
		"**/*.xcodeproj/**",            // Xcode project (contains user data)
		"**/*.xcworkspace/**",          // Xcode workspace (contains user data)
		"**/*.xcuserstate",             // Xcode user state
		"**/xcuserdata/**",             // Xcode user data
		"**/*.dSYM/**",                 // Debug symbols
		"**/*.ipa",                     // iOS app archive/package
		"**/*.xcarchive/**",            // Xcode archive
		"**/DerivedData/**",            // Xcode derived data
		"**/*.hmap",                    // Xcode header map
		"**/*.pbxuser",                 // Xcode user data
		"**/*.perspectivev3",           // Xcode perspective
		"**/Breakpoints_v2.xcbkptlist", // Xcode breakpoints

		// Android development artifacts
		"**/*.apk",                    // Android package
		"**/*.aab",                    // Android app bundle
		"**/*.dex",                    // Dalvik executable
		"**/*.class",                  // Java class files
		"**/local.properties",         // Local SDK paths
		"**/captures/**",              // Android Studio captures
		"**/*.jks",                    // Java keystore
		"**/*.keystore",               // Android keystore
		"**/google-services.json",     // Firebase config (may contain secrets)
		"**/GoogleService-Info.plist", // Firebase config iOS

		// Game Development - Unity
		"**/Library/**",
		"**/Temp/**",
		"**/Obj/**",
		"**/UnityGenerated/**",
		"**/*.unitypackage",
		"**/*.pidb",
		"**/*.booproj",
		"**/*.svd",
		"**/*.userprefs",
		"**/ExportedObj/**",
		"**/*.csproj",
		"**/*.unityproj",
		"**/*.sln",
		"**/*.suo",
		"**/*.user",
		"**/*.tmp",
		"**/*.svd",
		"**/*.pidb",
		"**/*.booproj",
		"**/UpgradeLog*.XML",
		"**/UpgradeLog*.htm",

		// Game Development - Unreal Engine
		"**/Binaries/**",
		"**/DerivedDataCache/**",
		"**/Intermediate/**",
		"**/Saved/**",
		"**/*.uasset", // Large binary assets
		"**/*.umap",   // Map files
		"**/*.bnk",    // Wwise sound banks
		"**/*.upk",    // Unreal package
		"**/*.udk",    // Unreal development kit

		// Game Development - Godot
		"**/.import/**",
		"**/export_presets.cfg",
		"**/*.import",

		// 3D Media & Assets
		"**/*.blend",  // Blender
		"**/*.blend1", // Blender backup
		"**/*.fbx",    // Autodesk FBX
		"**/*.obj",    // Wavefront OBJ
		"**/*.max",    // 3ds Max
		"**/*.ma",     // Maya ASCII
		"**/*.mb",     // Maya Binary
		"**/*.c4d",    // Cinema 4D
		"**/*.3ds",    // 3D Studio
		"**/*.dae",    // Collada
		"**/*.skp",    // SketchUp
		"**/*.ztl",    // ZBrush
		"**/*.zpr",    // ZBrush project
		"**/*.spp",    // Substance Painter
		"**/*.sbsar",  // Substance archive
		"**/*.sbs",    // Substance Designer
		"**/*.exr",    // OpenEXR (high dynamic range)
		"**/*.hdr",    // HDR image
		"**/*.dds",    // DirectDraw Surface
		"**/*.tga",    // Targa
		"**/*.bmp",    // Bitmap
		"**/*.psd",    // Photoshop
		"**/*.ai",     // Adobe Illustrator
		"**/*.indd",   // Adobe InDesign
		"**/*.xcf",    // GIMP

		// Video & Audio files
		"**/*.mp4",
		"**/*.avi",
		"**/*.mov",
		"**/*.wmv",
		"**/*.flv",
		"**/*.mkv",
		"**/*.webm",
		"**/*.m4v",
		"**/*.mpg",
		"**/*.mpeg",
		"**/*.3gp",
		"**/*.ogv",
		"**/*.mp3",
		"**/*.wav",
		"**/*.flac",
		"**/*.aac",
		"**/*.ogg",
		"**/*.wma",
		"**/*.m4a",
		"**/*.aiff",
		"**/*.ape",

		// AI/ML artifacts
		"**/*.h5",              // Keras/HDF5 models
		"**/*.hdf5",            // HDF5 data
		"**/*.pb",              // TensorFlow protobuf
		"**/*.pbtxt",           // TensorFlow protobuf text
		"**/*.onnx",            // ONNX models
		"**/*.pt",              // PyTorch models
		"**/*.pth",             // PyTorch models
		"**/*.pkl",             // Pickle files
		"**/*.pickle",          // Pickle files
		"**/*.joblib",          // Joblib files
		"**/*.safetensors",     // SafeTensors format
		"**/*.ckpt",            // Model checkpoints
		"**/*.index",           // TensorFlow checkpoint index
		"**/*.meta",            // TensorFlow metadata
		"**/checkpoints/**",    // Model checkpoints directory
		"**/lightning_logs/**", // PyTorch Lightning logs
		"**/runs/**",           // TensorBoard runs
		"**/wandb/**",          // Weights & Biases
		"**/.neptune/**",       // Neptune.ai
		"**/mlruns/**",         // MLflow
		"**/*.tflite",          // TensorFlow Lite
		"**/*.trt",             // TensorRT
		"**/*.engine",          // TensorRT engine
		"**/*.plan",            // TensorRT plan
		"**/*.caffemodel",      // Caffe model
		"**/*.npy",             // NumPy array (can be large)
		"**/*.npz",             // NumPy compressed arrays

		// Databases & data files
		"**/*.sqlite",
		"**/*.sqlite3",
		"**/*.db",
		"**/*.db3",
		"**/*.mdb",
		"**/*.accdb",
		"**/*.rdb", // Redis
		"**/*.dbf", // dBase
		"**/*.dat", // Generic data files
		"**/*.data",

		// Archives & compressed files
		"**/*.zip",
		"**/*.tar",
		"**/*.tar.gz",
		"**/*.tgz",
		"**/*.tar.bz2",
		"**/*.tbz2",
		"**/*.tar.xz",
		"**/*.rar",
		"**/*.7z",
		"**/*.gz",
		"**/*.bz2",
		"**/*.xz",
		"**/*.lz",
		"**/*.lzma",
		"**/*.z",
		"**/*.jar", // Java archives
		"**/*.war", // Web archives
		"**/*.ear", // Enterprise archives
		"**/*.sar", // Service archives

		// Cache directories
		"**/.cache/**",
		"**/cache/**",
		"**/.next/**",         // Next.js
		"**/.nuxt/**",         // Nuxt.js
		"**/.parcel-cache/**", // Parcel
		"**/.turbo/**",        // Turborepo
		"**/.docusaurus/**",   // Docusaurus
		"**/.astro/**",        // Astro
		"**/.vite/**",         // Vite cache
		"**/.rollup.cache/**", // Rollup cache
		"**/.yarn/**",         // Yarn cache

		// Logs & temporary files
		"**/logs/**",
		"**/*.log",
		"**/*.log.*", // Rotated logs
		"**/tmp/**",
		"**/temp/**",
		"**/.tmp/**",
		"**/.temp/**",

		// Coverage & test artifacts
		"**/coverage/**",
		"**/.coverage",
		"**/.nyc_output/**",
		"**/htmlcov/**",
		"**/.tox/**",
		"**/.nox/**",
		"**/*.cover",
		"**/*.coverage",
		"**/junit.xml",
		"**/test-results/**",

		// Documentation build outputs
		"**/_build/**",     // Sphinx
		"**/site/**",       // MkDocs
		"**/.docz/**",      // Docz
		"**/docs/_site/**", // Jekyll docs
	}
}
