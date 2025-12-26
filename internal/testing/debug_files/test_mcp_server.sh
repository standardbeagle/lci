#!/bin/bash

# Test script for MCP server functionality

echo "Building the project..."
go build ./cmd/lci || exit 1

echo "Starting MCP server in the background..."
./lci mcp serve > mcp_server.log 2>&1 &
MCP_PID=$!

# Give the server time to start
sleep 2

echo "MCP server started with PID: $MCP_PID"
echo "Server logs will be in mcp_server.log"

# Function to test MCP commands
test_mcp() {
    local tool_name=$1
    local params=$2
    
    echo "Testing $tool_name..."
    
    # Create a test request
    cat > test_request.json <<EOF
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "$tool_name",
    "arguments": $params
  }
}
EOF
    
    # Send the request (this would normally be done via stdio)
    echo "Request: $tool_name with params: $params"
}

# Test the search_stats tool
test_mcp "search_stats" '{"patterns": ["func", "test"]}'

echo ""
echo "To stop the MCP server, run: kill $MCP_PID"
echo "To view server logs, run: tail -f mcp_server.log"