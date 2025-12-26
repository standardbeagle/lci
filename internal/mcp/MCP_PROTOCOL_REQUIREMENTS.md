# MCP Protocol Compliance Requirements

## CRITICAL: Clean stdio Protocol Requirement

The Model Context Protocol (MCP) uses JSON-RPC 2.0 over stdio for bidirectional communication with AI assistants. **ANY output to stdout or stderr will break the protocol** and cause connection failures.

### Error Message When Protocol is Violated
```
connection closed
error=calling "tools/call": connection closed
```

This error occurs when:
1. Any text is written to stdout (the protocol channel)
2. Any diagnostic output is written to stderr during server operation
3. The stdio stream becomes corrupted or non-compliant

## Implementation: DiagnosticLogger

All diagnostic output MUST use `DiagnosticLogger` instead of writing directly to stderr.

### Usage in MCP Server

```go
// ✅ CORRECT: Use diagnostic logger
s.diagnosticLogger.Printf("Operation started")

// ❌ WRONG: Direct stderr write
fmt.Fprintf(os.Stderr, "Operation started\n")

// ❌ WRONG: Standard logger
log.Printf("Operation started")
```

### How It Works

1. **Detection**: When `isMCP=true`, all logging goes to file
2. **File Location**:
   - Unix/Linux: `/tmp/lci-mcp-logs/mcp-YYYY-MM-DDTHHMSS.log`
   - macOS: `/var/folders/.../T/lci-mcp-logs/mcp-YYYY-MM-DDTHHMSS.log`
   - Windows: `%TEMP%\lci-mcp-logs\mcp-YYYY-MM-DDTHHMSS.log`
   - Fallback: `~/.lci-mcp-logs/mcp-YYYY-MM-DDTHHMSS.log`

3. **Graceful Degradation**: If file creation fails, logging is disabled (not stderr)

## Error Handling: MCP Error Responses

ALL errors MUST be returned through the MCP protocol using proper error responses.

### ✅ CORRECT: Return MCP Error Response

```go
func (s *Server) handleGetObjectContext(ctx context.Context, ss *mcp.ServerSession,
    params *mcp.CallToolParams) (*mcp.CallToolResultFor[any], error) {

    // Manual deserialization to avoid protocol errors
    var args ObjectContextParams
    if err := json.Unmarshal(params.Arguments.([]byte), &args); err != nil {
        return createErrorResponse("INVALID_REQUEST", "Invalid parameters: " + err.Error()), nil
    }

    if err != nil {
        // Return proper MCP error response
        return createErrorResponse("INVALID_REQUEST", "Symbol not found: " + err.Error()), nil
    }

    return &mcp.CallToolResultFor[any]{
        Content: [...],
    }, nil
}
```

### ❌ WRONG: Don't Return Go Error

```go
func (s *Server) handleGetObjectContext(ctx context.Context, ss *mcp.ServerSession,
    params *mcp.CallToolParams) (*mcp.CallToolResultFor[any], error) {

    if err != nil {
        // ❌ WRONG: This breaks the protocol
        return nil, err  // Client sees "connection closed"
    }
}
```

## Error Response Format

Use `createErrorResponse()` or `createSmartErrorResponse()` to generate proper MCP error messages:

```go
// Simple error response
createErrorResponse("INVALID_PATTERN", "Pattern cannot be empty")

// Smart error response with context
createSmartErrorResponse("search", err, map[string]interface{}{
    "pattern": pattern,
    "timestamp": time.Now().Format(time.RFC3339),
})
```

## Checklist for MCP Compliance

- [ ] No `fmt.Printf()` or `fmt.Println()` to stdout
- [ ] No `fmt.Fprintf(os.Stdout, ...)` calls
- [ ] No `os.Stderr` writes except through `diagnosticLogger`
- [ ] All errors wrapped in MCP error responses
- [ ] Logger cleanup in `Shutdown()` method
- [ ] No panics that terminate the server (use `recoverFromPanic`)
- [ ] Async operations properly tracked and reported
- [ ] Context timeouts properly handled

## Testing for Protocol Compliance

Run MCP tests with no protocol violations:

```bash
# Should pass - clean stdio protocol
make test-mcp

# Should pass - all errors returned as MCP responses
go test -v ./internal/mcp/...
```

## Diagnostic Log Access

To debug MCP issues, check the diagnostic log file:

### In CLI code:
```go
if dl.isMCP {
    fmt.Fprintf(os.Stderr, "Diagnostics: %s\n", dl.GetLogPath())
}
```

### Direct access:
```bash
# Linux/macOS
tail -f /tmp/lci-mcp-logs/mcp-*.log

# Or from home directory
tail -f ~/.lci-mcp-logs/mcp-*.log
```

## Common Pitfalls

### 1. Debug Output During MCP Operation
```go
// ❌ WRONG: This breaks the protocol
if debugFlag {
    fmt.Println("Debug info...")  // Goes to stdout!
}

// ✅ CORRECT: Use diagnostic logger
if debugFlag {
    s.diagnosticLogger.Printf("Debug info...")  // Goes to file
}
```

### 2. Unhandled Panics
```go
// ❌ WRONG: Panic crashes the server
panic("index not initialized")

// ✅ CORRECT: Wrapped in error response
return createErrorResponse("INTERNAL_ERROR", "index not initialized"), nil
```

### 3. Async Operations Without Tracking
```go
// ❌ WRONG: Background error goes nowhere
go func() {
    if err := s.autoIndexManager.startAutoIndexing(rootPath, s.cfg); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)  // Lost!
    }
}()

// ✅ CORRECT: Proper error tracking
go func() {
    if err := s.autoIndexManager.startAutoIndexing(rootPath, s.cfg); err != nil {
        s.diagnosticLogger.Printf("Auto-indexing error: %v", err)
    }
}()
```

## Implementation Rules

1. **FileService Implementation**: Any filesystem operations must use FileService
2. **No Raw Stderr**: Never `fmt.Fprintf(os.Stderr, ...)` in MCP context
3. **Error Always Returns**: Every error in a handler should return MCP response
4. **Logging Always File-Based**: All logs go through DiagnosticLogger
5. **Graceful Shutdown**: Diagnostic logger closed in Shutdown()

## References

- MCP Specification: https://modelcontextprotocol.io
- Go MCP SDK: https://github.com/modelcontextprotocol/go-sdk
- Error Response Format: See `mcp.CallToolResultFor` documentation
