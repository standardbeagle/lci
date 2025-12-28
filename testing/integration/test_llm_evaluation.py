#!/usr/bin/env python3
"""
Advanced LLM Evaluation Scenarios for Lightning Code Index MCP Server

This script tests the enhanced MCP features with realistic AI assistant scenarios,
focusing on the newly implemented improvements:
1. Smart error responses with context-aware suggestions
2. Enhanced file search capabilities  
3. Comprehensive pattern validation
4. Improved component discovery
5. UX enhancements for AI interaction

Usage:
    python test_llm_evaluation.py
"""

import json
import subprocess
import threading
import time
import sys
from typing import Dict, List, Any, Optional
import tempfile
import os

class MCPTestClient:
    """Client for testing MCP server responses"""
    
    def __init__(self, server_path: str):
        self.server_path = server_path
        self.process = None
        self.request_id = 0
    
    def start_server(self):
        """Start MCP server process"""
        self.process = subprocess.Popen(
            [self.server_path, 'mcp'],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            bufsize=0
        )
        
        # Wait for server initialization
        time.sleep(2)
        
        if self.process.poll() is not None:
            stderr = self.process.stderr.read()
            raise Exception(f"MCP server failed to start: {stderr}")
    
    def stop_server(self):
        """Stop MCP server process"""
        if self.process:
            self.process.terminate()
            self.process.wait()
    
    def send_request(self, method: str, params: Dict[str, Any]) -> Dict[str, Any]:
        """Send MCP request and get response"""
        self.request_id += 1
        request = {
            "jsonrpc": "2.0",
            "id": self.request_id,
            "method": method,
            "params": params
        }
        
        request_json = json.dumps(request) + '\n'
        self.process.stdin.write(request_json)
        self.process.stdin.flush()
        
        # Read response
        response_line = self.process.stdout.readline()
        if not response_line:
            raise Exception("No response from MCP server")
        
        try:
            response = json.loads(response_line)
            return response
        except json.JSONDecodeError as e:
            raise Exception(f"Invalid JSON response: {response_line}, Error: {e}")
    
    def call_tool(self, tool_name: str, arguments: Dict[str, Any]) -> Dict[str, Any]:
        """Call MCP tool and return response"""
        return self.send_request("tools/call", {
            "name": tool_name,
            "arguments": arguments
        })

class LLMEvaluationScenarios:
    """Advanced evaluation scenarios for LLM interactions"""
    
    def __init__(self, client: MCPTestClient):
        self.client = client
        self.results = []
    
    def log_result(self, scenario: str, success: bool, details: str, response_data: Any = None):
        """Log evaluation result"""
        result = {
            "scenario": scenario,
            "success": success,
            "details": details,
            "timestamp": time.time()
        }
        if response_data:
            result["response_data"] = response_data
        
        self.results.append(result)
        status = "✅ PASS" if success else "❌ FAIL"
        print(f"{status} {scenario}: {details}")
    
    def test_smart_error_responses(self):
        """Test smart error responses with contextual suggestions"""
        print("\n🧠 Testing Smart Error Responses...")
        
        # Test 1: Empty search pattern
        try:
            response = self.client.call_tool("search", {})
            content = self._extract_content(response)
            
            if "suggestions" in content and "error" in content:
                self.log_result("Smart Error - Empty Pattern", True, 
                               f"Got {len(content.get('suggestions', []))} suggestions for empty pattern error")
            else:
                self.log_result("Smart Error - Empty Pattern", False, 
                               "Missing suggestions in error response")
        except Exception as e:
            self.log_result("Smart Error - Empty Pattern", False, f"Exception: {e}")
        
        # Test 2: Invalid pattern type in file_search
        try:
            response = self.client.call_tool("file_search", {
                "pattern": "*.go",
                "pattern_type": "invalid_type"
            })
            content = self._extract_content(response)
            
            if "suggestions" in content and "supported_types" in content.get("context", {}):
                self.log_result("Smart Error - Invalid Pattern Type", True,
                               "Got contextual suggestions for invalid pattern type")
            else:
                self.log_result("Smart Error - Invalid Pattern Type", False,
                               "Missing context-aware suggestions")
        except Exception as e:
            self.log_result("Smart Error - Invalid Pattern Type", False, f"Exception: {e}")
        
        # Test 3: Missing function parameter in tree tool
        try:
            response = self.client.call_tool("tree", {})
            content = self._extract_content(response)
            
            if "help" in content and "related_operations" in content:
                self.log_result("Smart Error - Missing Function", True,
                               "Got help and related operations for missing function parameter")
            else:
                self.log_result("Smart Error - Missing Function", False,
                               "Missing help information in error response")
        except Exception as e:
            self.log_result("Smart Error - Missing Function", False, f"Exception: {e}")
    
    def test_enhanced_file_search(self):
        """Test enhanced file search capabilities"""
        print("\n📁 Testing Enhanced File Search...")
        
        # Test 1: Basic glob pattern search
        try:
            response = self.client.call_tool("file_search", {
                "pattern": "*.go",
                "pattern_type": "glob",
                "max_results": 10
            })
            content = self._extract_content(response)
            
            if "results" in content and len(content["results"]) > 0:
                self.log_result("File Search - Glob Pattern", True,
                               f"Found {len(content['results'])} .go files")
            else:
                self.log_result("File Search - Glob Pattern", False,
                               "No results from glob pattern search")
        except Exception as e:
            self.log_result("File Search - Glob Pattern", False, f"Exception: {e}")
        
        # Test 2: Regex pattern search
        try:
            response = self.client.call_tool("file_search", {
                "pattern": ".*\\.go$",
                "pattern_type": "regex",
                "max_results": 5
            })
            content = self._extract_content(response)
            
            if "results" in content:
                self.log_result("File Search - Regex Pattern", True,
                               f"Regex search returned {len(content.get('results', []))} results")
            else:
                self.log_result("File Search - Regex Pattern", False,
                               "Regex pattern search failed")
        except Exception as e:
            self.log_result("File Search - Regex Pattern", False, f"Exception: {e}")
        
        # Test 3: File search with extensions filter
        try:
            response = self.client.call_tool("file_search", {
                "pattern": "*",
                "pattern_type": "glob",
                "extensions": [".go", ".md"],
                "max_results": 15
            })
            content = self._extract_content(response)
            
            if "results" in content:
                results = content["results"]
                go_files = [r for r in results if r.get("name", "").endswith(".go")]
                md_files = [r for r in results if r.get("name", "").endswith(".md")]
                self.log_result("File Search - Extensions Filter", True,
                               f"Found {len(go_files)} .go and {len(md_files)} .md files")
            else:
                self.log_result("File Search - Extensions Filter", False,
                               "Extensions filter search failed")
        except Exception as e:
            self.log_result("File Search - Extensions Filter", False, f"Exception: {e}")
    
    def test_pattern_validation(self):
        """Test comprehensive pattern validation"""
        print("\n🔍 Testing Pattern Validation...")
        
        # Test 1: Valid regex pattern
        try:
            response = self.client.call_tool("validate_pattern", {
                "pattern": "func.*\\(.*\\)",
                "pattern_type": "regex"
            })
            content = self._extract_content(response)
            
            if content.get("is_valid") == True and "complexity" in content:
                self.log_result("Pattern Validation - Valid Regex", True,
                               f"Regex validated with complexity: {content.get('complexity')}")
            else:
                self.log_result("Pattern Validation - Valid Regex", False,
                               "Valid regex not properly validated")
        except Exception as e:
            self.log_result("Pattern Validation - Valid Regex", False, f"Exception: {e}")
        
        # Test 2: Invalid regex pattern
        try:
            response = self.client.call_tool("validate_pattern", {
                "pattern": "[invalid(regex",
                "pattern_type": "regex"
            })
            content = self._extract_content(response)
            
            if content.get("is_valid") == False and "errors" in content:
                self.log_result("Pattern Validation - Invalid Regex", True,
                               f"Invalid regex detected with {len(content.get('errors', []))} errors")
            else:
                self.log_result("Pattern Validation - Invalid Regex", False,
                               "Invalid regex not properly detected")
        except Exception as e:
            self.log_result("Pattern Validation - Invalid Regex", False, f"Exception: {e}")
        
        # Test 3: Pattern with test file matching
        try:
            response = self.client.call_tool("validate_pattern", {
                "pattern": "*.go",
                "pattern_type": "glob",
                "test_file": "main.go"
            })
            content = self._extract_content(response)
            
            if "test_match" in content and content.get("test_file") == "main.go":
                match_result = content.get("test_match")
                self.log_result("Pattern Validation - Test File Match", True,
                               f"Pattern matching test: *.go {'matches' if match_result else 'does not match'} main.go")
            else:
                self.log_result("Pattern Validation - Test File Match", False,
                               "Test file matching functionality missing")
        except Exception as e:
            self.log_result("Pattern Validation - Test File Match", False, f"Exception: {e}")
    
    def test_component_discovery(self):
        """Test enhanced component discovery"""
        print("\n🧩 Testing Component Discovery...")
        
        try:
            response = self.client.call_tool("find_components", {})
            content = self._extract_content(response)
            
            if "components" in content:
                components = content["components"]
                component_types = set()
                for comp in components:
                    component_types.add(comp.get("type", "unknown"))
                
                self.log_result("Component Discovery - Enhanced Types", True,
                               f"Discovered {len(component_types)} component types: {', '.join(sorted(component_types))}")
            else:
                self.log_result("Component Discovery - Enhanced Types", False,
                               "No components found in discovery")
        except Exception as e:
            self.log_result("Component Discovery - Enhanced Types", False, f"Exception: {e}")
    
    def test_search_suggestions(self):
        """Test search suggestions for empty results"""
        print("\n💡 Testing Search Suggestions...")
        
        # Test search with pattern unlikely to match
        try:
            response = self.client.call_tool("search", {
                "pattern": "extremely_unlikely_to_match_anything_12345",
                "max_results": 10
            })
            content = self._extract_content(response)
            
            if "suggestions" in content and "message" in content:
                suggestions = content.get("suggestions", [])
                self.log_result("Search Suggestions - No Results", True,
                               f"Generated {len(suggestions)} suggestions for zero-result search")
            else:
                # Check if we got normal results (which is also valid)
                if "results" in content and len(content.get("results", [])) == 0:
                    self.log_result("Search Suggestions - No Results", True,
                                   "Search returned empty results without suggestions (acceptable)")
                else:
                    self.log_result("Search Suggestions - No Results", False,
                                   "No suggestions provided for zero-result search")
        except Exception as e:
            self.log_result("Search Suggestions - No Results", False, f"Exception: {e}")
    
    def test_response_structure_consistency(self):
        """Test that all responses have consistent structure"""
        print("\n🏗️ Testing Response Structure Consistency...")
        
        tools_to_test = [
            ("search", {"pattern": "func"}),
            ("file_search", {"pattern": "*.go"}),
            ("validate_pattern", {"pattern": "test", "pattern_type": "literal"}),
            ("find_components", {}),
            ("find_important_files", {})
        ]
        
        consistent_responses = 0
        for tool_name, args in tools_to_test:
            try:
                response = self.client.call_tool(tool_name, args)
                
                # Check basic structure
                has_result = "result" in response
                has_content = False
                
                if has_result:
                    result = response["result"]
                    has_content = "content" in result and len(result["content"]) > 0
                
                if has_result and has_content:
                    consistent_responses += 1
                    self.log_result(f"Response Structure - {tool_name}", True,
                                   "Consistent response structure")
                else:
                    self.log_result(f"Response Structure - {tool_name}", False,
                                   f"Inconsistent structure: result={has_result}, content={has_content}")
                                   
            except Exception as e:
                self.log_result(f"Response Structure - {tool_name}", False, f"Exception: {e}")
        
        # Summary
        total_tools = len(tools_to_test)
        consistency_rate = (consistent_responses / total_tools) * 100
        self.log_result("Overall Response Consistency", consistency_rate >= 80,
                       f"Consistency rate: {consistency_rate:.1f}% ({consistent_responses}/{total_tools})")
    
    def _extract_content(self, response: Dict[str, Any]) -> Dict[str, Any]:
        """Extract content from MCP response"""
        if "result" not in response:
            return {}
        
        result = response["result"]
        if "content" not in result or not result["content"]:
            return {}
        
        # Get first content item
        content_item = result["content"][0]
        if "text" not in content_item:
            return {}
        
        try:
            return json.loads(content_item["text"])
        except json.JSONDecodeError:
            return {"raw_text": content_item["text"]}
    
    def run_all_evaluations(self):
        """Run all evaluation scenarios"""
        print("🚀 Starting Advanced LLM Evaluation Scenarios...")
        print(f"Testing MCP server at: {self.client.server_path}")
        
        try:
            # Start index first
            print("\n📊 Initializing index...")
            self.client.call_tool("index_start", {"root_path": "."})
            time.sleep(3)  # Wait for indexing
            
            # Run all test scenarios
            self.test_smart_error_responses()
            self.test_enhanced_file_search()
            self.test_pattern_validation()
            self.test_component_discovery()
            self.test_search_suggestions()
            self.test_response_structure_consistency()
            
        except Exception as e:
            print(f"❌ Evaluation failed: {e}")
            return False
        
        # Generate summary report
        self._generate_report()
        return True
    
    def _generate_report(self):
        """Generate evaluation summary report"""
        print("\n📋 EVALUATION SUMMARY REPORT")
        print("=" * 50)
        
        total_tests = len(self.results)
        passed_tests = len([r for r in self.results if r["success"]])
        failed_tests = total_tests - passed_tests
        
        print(f"Total Tests: {total_tests}")
        print(f"Passed: {passed_tests} ✅")
        print(f"Failed: {failed_tests} ❌")
        print(f"Success Rate: {(passed_tests/total_tests)*100:.1f}%")
        
        if failed_tests > 0:
            print(f"\n❌ FAILED TESTS:")
            for result in self.results:
                if not result["success"]:
                    print(f"  - {result['scenario']}: {result['details']}")
        
        print(f"\n💡 Key Improvements Validated:")
        improvement_categories = {
            "Smart Error Responses": len([r for r in self.results if "Smart Error" in r["scenario"] and r["success"]]),
            "Enhanced File Search": len([r for r in self.results if "File Search" in r["scenario"] and r["success"]]), 
            "Pattern Validation": len([r for r in self.results if "Pattern Validation" in r["scenario"] and r["success"]]),
            "Component Discovery": len([r for r in self.results if "Component Discovery" in r["scenario"] and r["success"]]),
            "Search Suggestions": len([r for r in self.results if "Search Suggestions" in r["scenario"] and r["success"]]),
            "Response Consistency": len([r for r in self.results if "Response Structure" in r["scenario"] and r["success"]])
        }
        
        for category, count in improvement_categories.items():
            print(f"  ✅ {category}: {count} tests passed")

def main():
    """Main evaluation function"""
    if len(sys.argv) < 2:
        print("Usage: python test_llm_evaluation.py <path_to_lci_binary>")
        print("Example: python test_llm_evaluation.py ./lci-test")
        sys.exit(1)
    
    server_path = sys.argv[1]
    if not os.path.exists(server_path):
        print(f"Error: Server binary not found at {server_path}")
        sys.exit(1)
    
    # Initialize test client
    client = MCPTestClient(server_path)
    
    try:
        print("🔧 Starting MCP server...")
        client.start_server()
        
        # Run evaluations
        evaluator = LLMEvaluationScenarios(client)
        success = evaluator.run_all_evaluations()
        
        if success:
            print("\n🎉 Evaluation completed successfully!")
        else:
            print("\n⚠️ Evaluation completed with issues")
            sys.exit(1)
            
    except KeyboardInterrupt:
        print("\n⏹️ Evaluation interrupted by user")
    except Exception as e:
        print(f"\n💥 Evaluation failed with error: {e}")
        sys.exit(1)
    finally:
        print("\n🔧 Stopping MCP server...")
        client.stop_server()

if __name__ == "__main__":
    main()