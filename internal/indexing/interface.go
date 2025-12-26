package indexing

// This file previously contained the Index interface and simple index implementations.
// These have been removed in favor of using MasterIndex directly throughout the system.
//
// The MCP server now uses MasterIndex directly via NewServerWithMasterIndex().
// The CLI has always used MasterIndex directly.
//
// This unification eliminates redundant indexing implementations and ensures
// both CLI and MCP server use the same high-performance indexing engine.
