# Real Projects Test Data

This directory contains real-world open-source projects used for testing the Lightning Code Index (LCI) performance and functionality.

## Git Submodule Structure

Each project in this directory is a **git submodule**, not a regular directory. This approach allows us to:

- 🔄 Keep test projects up-to-date with their latest versions
- 💾 Save significant repository space (avoids copying hundreds of MB of code)
- 🚀 Enable faster cloning and updates of the LCI repository
- 📦 Test against real, evolving codebases

## Initial Setup

After cloning the LCI repository, you must initialize and update the submodules:

```bash
# Initialize all submodules
git submodule update --init --recursive

# Or update existing submodules to latest versions
git submodule update --recursive --remote
```

## Directory Structure

```
real_projects/
├── go/                    # Go language projects
│   ├── chi/              # Chi web framework
│   ├── go-github/        # GitHub Go client library
│   └── pocketbase/       # PocketBase database
├── python/                # Python language projects
│   ├── fastapi/          # FastAPI web framework
│   ├── httpx/            # HTTP client library
│   └── pydantic/         # Data validation library
├── typescript/            # TypeScript projects
│   ├── next.js/          # Next.js React framework
│   ├── shadcn-ui/        # UI component library
│   └── trpc/             # TypeScript RPC framework
└── README.md             # This file
```

## Project Selection Criteria

Projects were chosen based on:

- **Size**: Medium to large codebases (hundreds to thousands of files)
- **Complexity**: Real-world architecture with multiple modules
- **Activity**: Actively maintained projects
- **Variety**: Different programming languages and frameworks
- **Quality**: Well-structured codebases with good patterns

## Usage in Tests

### Workflow Tests

The workflow test system uses these projects for realistic testing:

```go
// Example: Test with a Go project
ctx, err := SetupRealProject(t, "go", "chi")
require.NoError(t, err)

// Run queries against real code
results, err := ctx.Search("router middleware")
```

### Performance Tests

Memory and performance benchmarks test indexing against actual codebases:

```bash
# Run performance tests on all real projects
go test -v ./tests/benchmarks/...

# Profile memory usage during indexing
go test -v -run TestCurrentProjectMemoryUsage ./internal/indexing/
```

## Adding New Projects

To add a new test project:

1. **Choose a suitable project** that meets the criteria above
2. **Add as submodule** in the appropriate language directory:

```bash
# Example: Add a new Go project
git submodule add https://github.com/user/project.git real_projects/go/project-name
```

3. **Update test configurations** if needed
4. **Document the project** in this README

### Guidelines for New Projects

- 📏 **Size**: 500-5000 files (good balance of realism vs test time)
- 🏷️ **License**: Permissive licenses preferred (MIT, Apache 2.0, BSD)
- 🔧 **Dependencies**: Reasonable dependency count (avoid extremely complex setups)
- 📚 **Documentation**: Well-documented codebases help validate search quality

## Maintenance

### Updating Projects

Periodically update submodules to test against newer code:

```bash
# Update all submodules to latest
git submodule update --recursive --remote

# Commit the updates
git add real_projects/
git commit -m "test: update real project submodules"
```

### Monitoring

- **Repository size**: Monitor that submodule updates don't make the LCI repo too large
- **Test performance**: Ensure indexing tests complete in reasonable time
- **Compatibility**: Verify projects still work with current LCI features

## Troubleshooting

### Submodule Issues

```bash
# If submodule is in detached HEAD state
cd real_projects/go/chi
git checkout main  # or appropriate branch

# If submodule directory is empty
git submodule update --init --recursive

# Reset submodule to current commit recorded in LCI
git submodule update --recursive
```

### Large File Warnings

Some projects may include large files (assets, docs, etc.). These are typically excluded by the LCI configuration in `.lci.kdl`:

```kdl
exclude {
    "**/node_modules/**"
    "**/dist/**"
    "**/*.min.js"
    "**/documentation/**"
    # ... more exclusions
}
```

## Test Data Usage

### Performance Benchmarks

- **Memory usage**: Track indexing memory consumption
- **Speed**: Measure indexing time per file
- **Search performance**: Validate <5ms search guarantee
- **Concurrent operations**: Test with multiple simultaneous operations

### Qualitative Tests

The qualitative testing framework compares LCI-assisted responses vs. baseline:

- **Code navigation accuracy**
- **Feature explanation quality**
- **Architecture understanding**
- **Debugging assistance**

## Contributing

When adding or modifying test projects:

1. **Follow the submodule pattern** - don't copy code directly
2. **Update relevant tests** if project structure changes
3. **Document any special considerations** in project READMEs
4. **Verify CI performance** - ensure tests don't timeout

## Security Considerations

- All projects are public open-source repositories
- No proprietary or confidential code included
- Regular security audits of submodules recommended
- Monitor for any security vulnerabilities in test projects

---

**Note**: This directory is intentionally excluded from normal LCI indexing via `.lci.kdl` to avoid indexing test data during development.