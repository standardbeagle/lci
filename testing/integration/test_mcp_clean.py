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

class SimpleMCPTester:
    """Simple MCP tester that directly invokes LCI MCP tools"""
    
    def __init__(self, lci_binary_path: str, test_codebase_path: str):
        self.lci_binary = Path(lci_binary_path)
        self.test_codebase = Path(test_codebase_path)
        
        if not self.lci_binary.exists():
            raise FileNotFoundError(f"LCI binary not found: {lci_binary_path}")
        if not self.test_codebase.exists():
            raise FileNotFoundError(f"Test codebase not found: {test_codebase_path}")
    
    def run_lci_cli(self, args: list) -> dict:
        """Run LCI CLI command and return results"""
        try:
            cmd = [str(self.lci_binary)] + args
            process = subprocess.run(
                cmd,
                capture_output=True,
                text=True,
                timeout=30,
                cwd=self.test_codebase
            )
            
            return {
                "success": process.returncode == 0,
                "stdout": process.stdout,
                "stderr": process.stderr,
                "returncode": process.returncode
            }
            
        except subprocess.TimeoutExpired:
            return {"success": False, "error": "Command timed out"}
        except Exception as e:
            return {"success": False, "error": str(e)}
    
    def test_basic_functionality(self):
        """Test basic LCI functionality using CLI commands"""
        
        print("Testing LCI CLI Basic Functionality")
        print("="*50)
        
        # Test 1: Simple search
        print("\n1. Testing basic search...")
        result = self.run_lci_cli(['search', 'func', '--max-results', '5'])
        
        if result["success"]:
            print(f"   SUCCESS: Search completed")
            lines = result["stdout"].split('\n')
            print(f"      Output lines: {len([l for l in lines if l.strip()])}")
        else:
            print(f"   FAILED: {result.get('error', result['stderr'])}")
            
        # Test 2: Definition search
        print("\n2. Testing definition search...")
        result = self.run_lci_cli(['definition', 'main'])
        
        if result["success"]:
            print(f"   SUCCESS: Definition search completed")
        else:
            print(f"   FAILED: {result.get('error', result['stderr'])}")
        
        # Test 3: File discovery pattern (workaround test)
        print("\n3. Testing file discovery pattern...")
        result = self.run_lci_cli(['search', '.*', '--include', 'internal.*\\.go$', '--use-regex', '--max-results', '10'])
        
        if result["success"]:
            print(f"   SUCCESS: Complex file pattern worked")
            lines = result["stdout"].split('\n')
            print(f"      Found results in output")
        else:
            print(f"   FAILED: {result.get('error', result['stderr'])}")
            print("   WARNING: This demonstrates the file discovery limitation")
    
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
                        "args": ['search', 'complexity', '--max-results', '5']
                    },
                    {
                        "label": "Search 'complexity' (case insensitive)", 
                        "args": ['search', 'complexity', '--case-insensitive', '--max-results', '5']
                    }
                ]
            },
            {
                "name": "Component Discovery",
                "description": "Find architectural components",
                "tests": [
                    {
                        "label": "Find main functions",
                        "args": ['search', 'func main']
                    },
                    {
                        "label": "Find handler functions", 
                        "args": ['search', 'handler', '--case-insensitive']
                    },
                    {
                        "label": "Find MCP-related code",
                        "args": ['search', 'mcp', '--case-insensitive', '--include', '.*\\.go$', '--use-regex']
                    }
                ]
            }
        ]
        
        for scenario in scenarios:
            print(f"\n{scenario['name']}: {scenario['description']}")
            print("-" * 40)
            
            for test in scenario['tests']:
                print(f"\n  {test['label']}")
                result = self.run_lci_cli(test['args'])
                
                if result["success"]:
                    print(f"    SUCCESS")
                    lines = result["stdout"].split('\n')
                    result_lines = [l for l in lines if l.strip()]
                    print(f"    Output lines: {len(result_lines)}")
                else:
                    print(f"    FAILED: {result.get('error', result['stderr'])}")
    
    def run_all_tests(self):
        """Run all test suites"""
        
        print(f"LCI CLI Testing")
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
        print("Usage: python test_mcp_clean.py <lci_binary_path> <test_codebase_path>")
        print("Example: python test_mcp_clean.py ./lci .")
        sys.exit(1)
    
    lci_binary = sys.argv[1]
    test_codebase = sys.argv[2]
    
    tester = SimpleMCPTester(lci_binary, test_codebase)
    tester.run_all_tests()

if __name__ == "__main__":
    main()