#!/usr/bin/env python3
"""
Test script for the find_components MCP tool using the sample project
"""

import json
import subprocess
import sys
import os
import time

def test_find_components():
    """Test the find_components MCP tool functionality"""
    
    # First initialize the session
    init_request = {
        "jsonrpc": "2.0",
        "id": 1,
        "method": "initialize",
        "params": {
            "protocolVersion": "2025-06-18",
            "capabilities": {},
            "clientInfo": {
                "name": "test_client",
                "version": "1.0.0"
            }
        }
    }
    
    # Then send index start request
    index_request = {
        "jsonrpc": "2.0",
        "id": 2,
        "method": "tools/call",
        "params": {
            "name": "index_start",
            "arguments": {}
        }
    }
    
    # Finally send find_components request
    find_request = {
        "jsonrpc": "2.0",
        "id": 3,
        "method": "tools/call",
        "params": {
            "name": "find_components",
            "arguments": {}
        }
    }
    
    # Start MCP server in background
    print("Starting MCP server...")
    proc = subprocess.Popen(
        ["./lci", "mcp"],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        cwd="."
    )
    
    try:
        # Send initialize request
        init_json = json.dumps(init_request) + "\n"
        print("Sending initialize request...")
        proc.stdin.write(init_json)
        proc.stdin.flush()
        
        # Send initialized notification
        initialized_notification = {
            "jsonrpc": "2.0",
            "method": "notifications/initialized"
        }
        init_notif_json = json.dumps(initialized_notification) + "\n"
        proc.stdin.write(init_notif_json)
        proc.stdin.flush()
        
        # Send index start request
        index_json = json.dumps(index_request) + "\n"
        print("Sending index_start request...")
        proc.stdin.write(index_json)
        proc.stdin.flush()
        
        # Send find_components request
        find_json = json.dumps(find_request) + "\n"
        print("Sending find_components request...")
        proc.stdin.write(find_json)
        proc.stdin.flush()
        
        # Wait for responses with timeout
        try:
            stdout, stderr = proc.communicate(timeout=15)
        except subprocess.TimeoutExpired:
            proc.kill()
            stdout, stderr = proc.communicate()
            print("ERROR: MCP server timed out")
            return False
            
        if stderr:
            print(f"STDERR: {stderr}")
            
        if stdout:
            print(f"STDOUT: {stdout}")
            
            # Try to parse JSON responses
            lines = stdout.strip().split('\n')
            for line in lines:
                if line.strip():
                    try:
                        response = json.loads(line)
                        if response.get("method") == "notifications/initialized":
                            print("✅ MCP server initialized")
                            continue
                        elif response.get("id") == 1:
                            print("✅ Received initialize response")
                        elif response.get("id") == 2:
                            print("✅ Received index_start response")
                            result = response.get("result", {})
                            if result.get("status") == "completed":
                                print("✅ Index building completed")
                        elif response.get("id") == 3:
                            print("✅ Received find_components response")
                            result = response.get("result", {})
                            content = result.get("content", [])
                            if content and len(content) > 0:
                                text_content = content[0].get("text", "{}")
                                try:
                                    data = json.loads(text_content)
                                    components = data.get("components", [])
                                    print(f"✅ Found {len(components)} components")
                                    
                                    # Display some components
                                    for i, comp in enumerate(components[:5]):
                                        print(f"  {i+1}. {comp.get('name')} ({comp.get('type')}) - Confidence: {comp.get('confidence'):.2f}")
                                    
                                    if len(components) > 5:
                                        print(f"  ... and {len(components) - 5} more")
                                        
                                    return len(components) > 0
                                except json.JSONDecodeError:
                                    print("❌ Failed to parse components data")
                                    return False
                            else:
                                print("❌ No content in find_components response")
                                return False
                    except json.JSONDecodeError as e:
                        print(f"Failed to parse JSON: {e}")
                        print(f"Raw line: {line}")
            
        return False
        
    finally:
        if proc.poll() is None:
            proc.terminate()
            proc.wait()

if __name__ == "__main__":
    # Change to the project directory
    os.chdir("/home/beagle/work/lightning-docs/lightning-code-index")
    
    print("Testing find_components MCP tool...")
    print("=" * 50)
    
    success = test_find_components()
    
    if success:
        print("=" * 50)
        print("✅ find_components test PASSED")
        sys.exit(0)
    else:
        print("=" * 50) 
        print("❌ find_components test FAILED")
        sys.exit(1)