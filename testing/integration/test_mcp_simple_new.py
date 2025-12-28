#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Simple LCI MCP Test Runner

This script provides a simpler way to test LCI MCP functionality without requiring
Claude/Gemini CLI setup. It directly tests the MCP server with predefined scenarios.
"""

import json
import subprocess
import time
import os
import sys
from pathlib import Path
import tempfile

class SimpleMCPTester:
    """Simple MCP tester that directly invokes LCI MCP tools"""
    
    def __init__(self, lci_binary_path: str, test_codebase_path: str):
        self.lci_binary = Path(lci_binary_path)
        self.test_codebase = Path(test_codebase_path)
        
        if not self.lci_binary.exists():
            raise FileNotFoundError(f"LCI binary not found: {lci_binary_path}")
        if not self.test_codebase.exists():
            raise FileNotFoundError(f"Test codebase not found: {test_codebase_path}")
    
    def run_mcp_tool(self, tool_name: str, params: dict) -> dict:
        """Run a single MCP tool with given parameters"""
        
        # Create MCP request
        request = {
            "jsonrpc": "2.0",
            "id": 1,
            "method": "tools/call",
            "params": {
                "name": tool_name,
                "arguments": params
            }
        }
        
        try:
            # Start MCP server process
            process = subprocess.Popen(
                [str(self.lci_binary), str(self.test_codebase)],
                stdin=subprocess.PIPE,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                text=True,
                cwd=self.test_codebase
            )
            
            # Send request
            request_json = json.dumps(request) + '\n'
            stdout, stderr = process.communicate(input=request_json, timeout=30)
            
            if process.returncode != 0:
                return {
                    "error": f"Process failed with code {process.returncode}",
                    "stderr": stderr,
                    "stdout": stdout
                }
            
            # Parse response
            for line in stdout.strip().split('\n'):
                if line.strip():
                    try:
                        response = json.loads(line)
                        if "result" in response:
                            return response["result"]
                        elif "error" in response:
                            return {"error": response["error"]}
                    except json.JSONDecodeError:
                        continue
                        
            return {"error": "No valid JSON response received", "raw_output": stdout}
            
        except subprocess.TimeoutExpired:
            process.kill()
            return {"error": "Tool call timed out"}
        except Exception as e:
            return {"error": f"Unexpected error: {str(e)}"}
    
    def test_basic_functionality(self):
        """Test basic LCI functionality"""
        
        print("Testing LCI MCP Basic Functionality")
        print("="*50)
        
        # Test 1: Index start
        print("\n1. Testing index_start...")
        result = self.run_mcp_tool("index_start", {})
        
        if "error" in result:
            print(f"   L FAILED: {result['error']}")
            return False
        else:
            print(f"    SUCCESS: Indexing initiated")
            
        # Test 2: Index stats
        print("\n2. Testing index_stats...")
        result = self.run_mcp_tool("index_stats", {})
        
        if "error" in result:
            print(f"   L FAILED: {result['error']}")
        else:
            print(f"    SUCCESS: Stats retrieved")
            if isinstance(result, dict):
                file_count = result.get("file_count", "unknown")
                print(f"      Files indexed: {file_count}")
        
        # Test 3: Simple search
        print("\n3. Testing basic search...")  
        result = self.run_mcp_tool("search", {
            "pattern": "func",
            "max_results": 5
        })
        
        if "error" in result:
            print(f"   L FAILED: {result['error']}")
        else:
            print(f"    SUCCESS: Search completed")
            if isinstance(result, dict) and "results" in result:
                count = len(result["results"])
                print(f"      Found {count} results")
        
        # Test 4: File discovery (current limitation)
        print("\n4. Testing file discovery pattern...")
        result = self.run_mcp_tool("search", {
            "pattern": ".*",
            "include": "internal/.*\\.go$",
            "use_regex": True,
            "max_results": 10
        })
        
        if "error" in result:
            print(f"   L FAILED: {result['error']}")
            print("   �  This demonstrates the file discovery limitation")
        else:
            print(f"    SUCCESS: Complex file pattern worked")
            if isinstance(result, dict) and "results" in result:
                count = len(result["results"])
                print(f"      Found {count} files")
        
        return True
    
    def test_feedback_scenarios(self):
        """Test scenarios from feedback documents"""
        
        print("\n\nTesting Feedback Scenarios")
        print("="*50)
        
        scenarios = [
            {
                "name": "Case Sensitivity Issue",
                "description": "Test if case-insensitive search works better",
                "tests": [
                    {
                        "label": "Search 'complexity' (case sensitive)",
                        "params": {"pattern": "complexity", "max_results": 5}
                    },
                    {
                        "label": "Search 'complexity' (case insensitive)", 
                        "params": {"pattern": "complexity", "case_insensitive": True, "max_results": 5}
                    }
                ]
            },
            {
                "name": "Token Management",
                "description": "Test large result handling",
                "tests": [
                    {
                        "label": "Broad search with light mode",
                        "params": {"pattern": "func", "max_results": 20}
                    },
                    {
                        "label": "Broad search without light mode (potential overflow)",
                        "params": {"pattern": "func", "max_results": 10}
                    }
                ]
            },
            {
                "name": "Component Discovery",
                "description": "Find architectural components",
                "tests": [
                    {
                        "label": "Find main functions",
                        "params": {"pattern": "func main"}
                    },
                    {
                        "label": "Find handler functions", 
                        "params": {"pattern": "handler", "case_insensitive": True}
                    },
                    {
                        "label": "Find MCP-related code",
                        "params": {"pattern": "mcp", "case_insensitive": True, "include": ".*\\.go$", "use_regex": True}
                    }
                ]
            }
        ]
        
        for scenario in scenarios:
            print(f"\n{scenario['name']}: {scenario['description']}")
            print("-" * 40)
            
            for test in scenario['tests']:
                print(f"\n  {test['label']}")
                result = self.run_mcp_tool("search", test['params'])
                
                if "error" in result:
                    print(f"    L FAILED: {result['error']}")
                else:
                    print(f"     SUCCESS")
                    if isinstance(result, dict):
                        count = len(result.get("results", []))
                        tokens = result.get("token_count", "unknown")
                        print(f"    Results: {count}, Tokens: {tokens}")
    
    def run_all_tests(self):
        """Run all test suites"""
        
        print(f"LCI MCP Testing")
        print(f"Binary: {self.lci_binary}")
        print(f"Codebase: {self.test_codebase}")
        print("="*60)
        
        start_time = time.time()
        
        try:
            # Basic functionality tests
            self.test_basic_functionality()
            
            # Feedback scenario tests
            self.test_feedback_scenarios()
            
            print(f"\n\nTesting completed in {time.time() - start_time:.2f} seconds")
            
        except KeyboardInterrupt:
            print("\nTesting interrupted by user")
        except Exception as e:
            print(f"\nTesting failed with error: {e}")

def main():
    """Main entry point"""
    
    if len(sys.argv) < 3:
        print("Usage: python test_mcp_simple_new.py <lci_binary_path> <test_codebase_path>")
        print("Example: python test_mcp_simple_new.py ./lci .")
        sys.exit(1)
    
    lci_binary = sys.argv[1]
    test_codebase = sys.argv[2]
    
    tester = SimpleMCPTester(lci_binary, test_codebase)
    tester.run_all_tests()

if __name__ == "__main__":
    main()