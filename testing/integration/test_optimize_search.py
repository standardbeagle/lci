#!/usr/bin/env python3
"""
Test script to validate the optimize_search MCP tool integration.
This tests both the CLI and MCP functionality for LLM optimization.
"""
import json
import subprocess
import sys
import time

def run_lci_cli_test():
    """Test the CLI functionality to ensure basic indexing and search works."""
    print("🔍 Testing CLI search functionality...")
    
    # Test basic search
    result = subprocess.run(['./lci', 'search', 'func.*Handle'], 
                          capture_output=True, text=True, cwd='.')
    
    if result.returncode != 0:
        print(f"❌ CLI search failed: {result.stderr}")
        return False
        
    if 'Building index' in result.stderr and 'matches found' in result.stdout:
        print("✅ CLI search working correctly")
        return True
    else:
        print(f"⚠️ Unexpected CLI output: {result.stdout[:200]}...")
        return False

def run_mcp_server_test():
    """Test the MCP server with optimize_search tool."""
    print("🚀 Testing MCP optimize_search tool...")
    
    # Create a simple MCP test request
    mcp_request = {
        "jsonrpc": "2.0",
        "id": 1,
        "method": "tools/call",
        "params": {
            "name": "optimize_search",
            "arguments": {
                "query": "error handling",
                "max_tokens": 4000,
                "include_examples": 2,
                "context_format": "structured"
            }
        }
    }
    
    # Start MCP server process
    try:
        proc = subprocess.Popen(['./lci', 'mcp'], 
                              stdin=subprocess.PIPE, 
                              stdout=subprocess.PIPE, 
                              stderr=subprocess.PIPE,
                              text=True, cwd='.')
        
        # Send request and get response
        stdout, stderr = proc.communicate(json.dumps(mcp_request), timeout=30)
        
        if proc.returncode == 0:
            try:
                response = json.loads(stdout)
                if 'result' in response and 'analysis' in response['result']:
                    print("✅ MCP optimize_search tool working correctly")
                    print(f"📊 Token estimate: {response['result'].get('metadata', {}).get('token_estimate', 'N/A')}")
                    return True
            except json.JSONDecodeError:
                pass
        
        print(f"❌ MCP test failed - stdout: {stdout[:200]}...")
        print(f"❌ MCP test failed - stderr: {stderr[:200]}...")
        return False
        
    except subprocess.TimeoutExpired:
        proc.kill()
        print("❌ MCP test timed out")
        return False
    except Exception as e:
        print(f"❌ MCP test error: {e}")
        return False

def test_llm_optimization_features():
    """Test specific LLM optimization features."""
    print("🧠 Testing LLM optimization features...")
    
    # Test with intent analysis
    mcp_request = {
        "jsonrpc": "2.0", 
        "id": 2,
        "method": "tools/call",
        "params": {
            "name": "optimize_search",
            "arguments": {
                "query": "config",
                "intent": "configuration",
                "max_tokens": 2000,
                "context_format": "json"
            }
        }
    }
    
    try:
        proc = subprocess.Popen(['./lci', 'mcp'],
                              stdin=subprocess.PIPE,
                              stdout=subprocess.PIPE, 
                              stderr=subprocess.PIPE,
                              text=True, cwd='.')
        
        stdout, stderr = proc.communicate(json.dumps(mcp_request), timeout=30)
        
        if proc.returncode == 0:
            try:
                response = json.loads(stdout)
                result = response.get('result', {})
                
                # Check for key optimization features
                has_summary = 'summary' in result
                has_findings = 'key_findings' in result and len(result['key_findings']) > 0
                has_examples = 'code_examples' in result
                has_token_estimate = 'token_estimate' in result
                
                if has_summary and has_findings and has_examples and has_token_estimate:
                    print("✅ LLM optimization features working")
                    print(f"📝 Summary length: {len(result.get('summary', ''))}")
                    print(f"🔍 Key findings: {len(result.get('key_findings', []))}")
                    print(f"💡 Code examples: {len(result.get('code_examples', []))}")
                    print(f"🎯 Token estimate: {result.get('token_estimate', 'N/A')}")
                    return True
                    
            except json.JSONDecodeError:
                pass
        
        print("❌ LLM optimization features test failed")
        return False
        
    except subprocess.TimeoutExpired:
        proc.kill()
        print("❌ LLM optimization test timed out")
        return False
    except Exception as e:
        print(f"❌ LLM optimization test error: {e}")
        return False

def main():
    """Run all tests and report results."""
    print("🧪 Lightning Code Index - LLM Optimization Testing")
    print("=" * 60)
    
    # Build the project first
    print("🔨 Building LCI...")
    build_result = subprocess.run(['go', 'build', './cmd/lci'], 
                                capture_output=True, text=True)
    
    if build_result.returncode != 0:
        print(f"❌ Build failed: {build_result.stderr}")
        return False
    
    print("✅ Build successful")
    print()
    
    # Run tests
    tests = [
        ("CLI Search", run_lci_cli_test),
        ("MCP Optimize Search", run_mcp_server_test), 
        ("LLM Features", test_llm_optimization_features)
    ]
    
    passed = 0
    total = len(tests)
    
    for test_name, test_func in tests:
        print(f"Running {test_name} test...")
        if test_func():
            passed += 1
        print()
    
    # Summary
    print("=" * 60)
    print(f"📊 Test Results: {passed}/{total} tests passed")
    
    if passed == total:
        print("🎉 All tests passed! LLM optimization integration is working.")
        print("\n💡 Ready for production testing with Claude and Gemini CLIs")
        return True
    else:
        print(f"⚠️ {total - passed} tests failed. Check the output above.")
        return False

if __name__ == "__main__":
    success = main()
    sys.exit(0 if success else 1)