#!/usr/bin/env python3
"""
Simple test script to validate the new MCP architecture discovery tools
"""

import json
import subprocess
import sys
import time

def test_mcp_tool(tool_name, params=None):
    """Test an MCP tool by sending a request and parsing the response"""
    
    # Build MCP request
    request = {
        "jsonrpc": "2.0",
        "id": 1,
        "method": "tools/call",
        "params": {
            "name": tool_name,
            "arguments": params or {}
        }
    }
    
    try:
        # Start MCP server process
        process = subprocess.Popen([
            './cmd/lci/lci-binary', 'mcp'
        ], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        
        # Send the request
        request_str = json.dumps(request) + '\n'
        stdout, stderr = process.communicate(request_str, timeout=30)
        
        print(f"\n=== Testing {tool_name} ===")
        print(f"Request: {json.dumps(request, indent=2)}")
        print(f"STDOUT: {stdout}")
        if stderr:
            print(f"STDERR: {stderr}")
            
        return stdout, stderr
        
    except subprocess.TimeoutExpired:
        process.kill()
        print(f"Timeout testing {tool_name}")
        return None, "Timeout"
    except Exception as e:
        print(f"Error testing {tool_name}: {e}")
        return None, str(e)

def main():
    """Test the new architecture discovery MCP tools"""
    
    print("Testing new MCP architecture discovery tools...")
    
    # Test 1: Index the codebase first
    print("\n=== Step 1: Index the codebase ===")
    test_mcp_tool("index_start", {"root_path": "/home/beagle/work/lightning-docs/lightning-code-index"})
    
    # Wait a bit for indexing to complete
    time.sleep(2)
    
    # Test 2: Find important files
    print("\n=== Step 2: Find important files ===")
    test_mcp_tool("find_important_files", {})
    
    # Test 3: Find components
    print("\n=== Step 3: Find components ===")
    test_mcp_tool("find_components", {"technology": "go"})
    
    # Test 4: Project structure analysis
    print("\n=== Step 4: Project structure analysis ===")
    test_mcp_tool("project_structure", {})
    
    # Test 5: AST search
    print("\n=== Step 5: AST search ===")
    test_mcp_tool("ast_search", {"query": "functions that call SearchWithOptions"})
    
    print("\n=== Testing completed ===")

if __name__ == "__main__":
    main()