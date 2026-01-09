# LCI - Lightning Code Index

Lightning-fast code indexing and search for AI assistants.

[![CI](https://github.com/standardbeagle/lci/actions/workflows/ci.yml/badge.svg)](https://github.com/standardbeagle/lci/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/standardbeagle/lci)](https://goreportcard.com/report/github.com/standardbeagle/lci)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## Features

- **Sub-millisecond search**: Trigram-based indexing with <5ms search guarantee
- **Multi-language support**: Go, TypeScript, JavaScript, Python, Rust, C#, PHP, and more
- **MCP integration**: Model Context Protocol server for AI assistant integration
- **Semantic search**: Natural language queries with intelligent matching
- **Call graph analysis**: Track function calls, references, and dependencies
- **Semantic annotations**: `@lci:` vocabulary for marking up code with metadata

## Installation

### npm (recommended)

```bash
npm install -g @standardbeagle/lci
```

### pip

```bash
pip install lightning-code-index
```

### Homebrew (coming soon)

```bash
brew install standardbeagle/tap/lci
```

### From Source

```bash
go install github.com/standardbeagle/lci/cmd/lci@latest
```

### From Releases

Download pre-built binaries from [GitHub Releases](https://github.com/standardbeagle/lci/releases).

## Quick Start

### CLI Usage

```bash
# Index and search in current directory
lci search "handleRequest"

# Find symbol definitions
lci def UserService

# Find all references to a symbol
lci refs CreateUser

# Display function call hierarchy
lci tree main

# Fast grep-style search
lci grep "TODO|FIXME"

# List files that would be indexed
lci list
```

### MCP Server

Start the MCP server for AI assistant integration:

```bash
lci mcp
```

#### Claude Code Integration

Add to your `.mcp.json`:

```json
{
  "lci": {
    "command": "lci",
    "args": ["mcp"],
    "env": {}
  }
}
```

## Configuration

Create `.lci.kdl` in your project root:

```kdl
project {
    name "my-project"
    root "."
}

index {
    include "**/*.go" "**/*.ts" "**/*.py"
    exclude "**/node_modules/**" "**/vendor/**"
}

search {
    max-results 100
    context-lines 3
}
```

## MCP Tools

When running as an MCP server, LCI exposes these tools:

| Tool | Description |
|------|-------------|
| `search` | Semantic code search with fuzzy matching |
| `get_context` | Get detailed context for a code symbol |
| `find_files` | Fast file path search with glob patterns |
| `code_insight` | Codebase intelligence and analysis |
| `context` | Save/load code context manifests |
| `semantic_annotations` | Query @lci: semantic labels |
| `side_effects` | Analyze function purity and side effects |

## Semantic Annotations

Mark up your code with `@lci:` annotations for enhanced AI understanding:

```go
// @lci:risk[high] @lci:public-api
// @lci:requires[env:DATABASE_URL]
func HandleUserLogin(w http.ResponseWriter, r *http.Request) {
    // ...
}

// @lci:purpose[Validate user credentials against database]
// @lci:must[Return error for invalid credentials]
func ValidateCredentials(username, password string) error {
    // ...
}
```

### Annotation Categories

- **Risk & Safety**: `@lci:risk[low|medium|high|critical]`, `@lci:safe-zone`, `@lci:stability`
- **Dependencies**: `@lci:requires[env:VAR]`, `@lci:requires[db:table]`, `@lci:requires[service:name]`
- **Conventions**: `@lci:convention[pattern]`, `@lci:idiom[name]`, `@lci:template[name]`
- **Contracts**: `@lci:must[behavior]`, `@lci:must-not[behavior]`, `@lci:invariant[condition]`
- **Purpose**: `@lci:purpose[description]`, `@lci:domain[area]`, `@lci:owner[team]`

## Architecture

```
lci/
├── cmd/lci/          # CLI entry point
├── internal/
│   ├── core/         # Trigram index, symbol store, reference tracker
│   ├── parser/       # Tree-sitter based multi-language parsing
│   ├── search/       # Search engine and scoring
│   ├── indexing/     # Master index and pipeline
│   ├── mcp/          # MCP server and tools
│   └── analysis/     # Code analysis and metrics
└── pkg/              # Public API
```

## Performance

LCI is designed for speed:

- **Indexing**: <5s for typical projects (<10k files)
- **Search**: <5ms for most queries
- **Memory**: <100MB for typical web projects
- **Startup**: Near-instant with persistent index

## Development

```bash
# Run tests
go test ./...

# Build
go build ./cmd/lci

# Run with race detector
go test -race ./...
```

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome! Please read the contributing guidelines before submitting PRs.
