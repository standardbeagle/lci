#!/usr/bin/env python3
"""
Simple MCP protocol test for the architecture discovery tools
"""

import json
import subprocess
import sys
import os

def test_mcp_client():
    """Test MCP server with proper protocol initialization"""
    
    # Start MCP server
    print("Starting MCP server...")
    process = subprocess.Popen([
        './cmd/lci/lci-binary', 'mcp'
    ], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    
    def send_request(request):
        """Send a request and get response"""
        request_str = json.dumps(request) + '\n'
        process.stdin.write(request_str)
        process.stdin.flush()
        
        # Read response
        response_line = process.stdout.readline()
        if response_line:
            try:
                return json.loads(response_line.strip())
            except json.JSONDecodeError:
                return {"error": "Invalid JSON response: " + response_line}
        return {"error": "No response"}
    
    try:
        # 1. Initialize the MCP session
        print("\n1. Initializing MCP session...")
        init_request = {
            "jsonrpc": "2.0",
            "id": 1,
            "method": "initialize",
            "params": {
                "protocolVersion": "2024-11-05",
                "capabilities": {},
                "clientInfo": {"name": "test-client", "version": "1.0.0"}
            }
        }
        
        init_response = send_request(init_request)
        print(f"Initialize response: {json.dumps(init_response, indent=2)}")
        
        if "error" in init_response:
            print("Initialization failed")
            return False
            
        # 2. Send initialized notification
        print("\n2. Sending initialized notification...")
        initialized_request = {
            "jsonrpc": "2.0",
            "method": "notifications/initialized"
        }
        
        send_request(initialized_request)
        
        # 3. List available tools
        print("\n3. Listing available tools...")
        tools_request = {
            "jsonrpc": "2.0",
            "id": 2,
            "method": "tools/list"
        }
        
        tools_response = send_request(tools_request)
        print(f"Tools response: {json.dumps(tools_response, indent=2)}")
        
        if "result" in tools_response and "tools" in tools_response["result"]:
            tools = tools_response["result"]["tools"]
            print(f"\nFound {len(tools)} tools:")
            for tool in tools:
                print(f"  - {tool['name']}: {tool.get('description', 'No description')}")
            
            # Check if our new tools are available
            tool_names = [tool['name'] for tool in tools]
            expected_tools = ['find_important_files', 'find_components', 'project_structure', 'ast_search']
            
            found_tools = [tool for tool in expected_tools if tool in tool_names]
            missing_tools = [tool for tool in expected_tools if tool not in tool_names]
            
            print(f"\n✅ Found new architecture tools: {found_tools}")
            if missing_tools:
                print(f"❌ Missing expected tools: {missing_tools}")
            
            return len(found_tools) > 0
        else:
            print("Failed to get tools list")
            return False
            
    except Exception as e:
        print(f"Error: {e}")
        return False
    finally:
        process.terminate()
        process.wait()

if __name__ == "__main__":
    print("Testing MCP architecture discovery tools...")
    success = test_mcp_client()
    
    if success:
        print("\n✅ Test PASSED: New architecture tools are available via MCP")
    else:
        print("\n❌ Test FAILED: Could not verify architecture tools")
        
    sys.exit(0 if success else 1)