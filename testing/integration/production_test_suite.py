#!/usr/bin/env python3
"""
Comprehensive Production Test Suite for LCI LLM Optimization
Tests real-world scenarios that AI assistants encounter when analyzing codebases.
"""
import json
import subprocess
import sys
import time
import os
import threading
from pathlib import Path

class LCIProductionTester:
    def __init__(self):
        self.mcp_server = None
        self.test_results = []
        
    def start_mcp_server(self):
        """Start MCP server in background for testing."""
        print("🚀 Starting MCP server...")
        self.mcp_server = subprocess.Popen(
            ['./lci', 'mcp'], 
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True
        )
        time.sleep(2)  # Give server time to start
        return self.mcp_server is not None
        
    def stop_mcp_server(self):
        """Stop MCP server."""
        if self.mcp_server:
            self.mcp_server.terminate()
            self.mcp_server = None
            
    def run_cli_benchmark(self, query, description):
        """Run CLI search and measure performance."""
        print(f"⚡ Testing CLI: {description}")
        
        start_time = time.time()
        result = subprocess.run(
            ['./lci', 'search', query], 
            capture_output=True, 
            text=True
        )
        elapsed = (time.time() - start_time) * 1000
        
        if result.returncode == 0:
            # Extract match count from output
            lines = result.stdout.split('\n')
            match_info = next((line for line in lines if 'matches' in line.lower()), '')
            
            return {
                'success': True,
                'time_ms': elapsed,
                'output_length': len(result.stdout),
                'match_info': match_info.strip(),
                'description': description
            }
        else:
            return {
                'success': False,
                'error': result.stderr,
                'description': description
            }
    
    def create_mcp_request(self, tool_name, params):
        """Create properly formatted MCP request."""
        return {
            "jsonrpc": "2.0",
            "id": int(time.time()),
            "method": "tools/call",
            "params": {
                "name": tool_name,
                "arguments": params
            }
        }
    
    def run_mcp_optimization_test(self, query, params, description):
        """Run optimize_search MCP tool and measure results."""
        print(f"🧠 Testing MCP Optimization: {description}")
        
        mcp_request = self.create_mcp_request("optimize_search", {
            "query": query,
            **params
        })
        
        try:
            # Create new process for this test to avoid state issues
            proc = subprocess.Popen(
                ['./lci', 'mcp'],
                stdin=subprocess.PIPE,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                text=True
            )
            
            start_time = time.time()
            stdout, stderr = proc.communicate(
                json.dumps(mcp_request) + '\n', 
                timeout=30
            )
            elapsed = (time.time() - start_time) * 1000
            
            if proc.returncode == 0:
                try:
                    response = json.loads(stdout.strip())
                    if 'result' in response:
                        result_data = response['result']
                        
                        # Analyze optimization metrics
                        token_estimate = result_data.get('metadata', {}).get('token_estimate', 0)
                        examples_count = result_data.get('metadata', {}).get('total_examples', 0)
                        findings_count = result_data.get('metadata', {}).get('total_findings', 0)
                        
                        return {
                            'success': True,
                            'time_ms': elapsed,
                            'token_estimate': token_estimate,
                            'examples_count': examples_count,
                            'findings_count': findings_count,
                            'output_format': result_data.get('format', 'unknown'),
                            'description': description,
                            'response_size': len(json.dumps(result_data))
                        }
                    else:
                        return {
                            'success': False,
                            'error': response.get('error', 'Unknown error'),
                            'description': description
                        }
                        
                except json.JSONDecodeError as e:
                    return {
                        'success': False,
                        'error': f"JSON decode error: {e}",
                        'raw_output': stdout[:200],
                        'description': description
                    }
            else:
                return {
                    'success': False,
                    'error': f"Process failed: {stderr}",
                    'description': description
                }
                
        except subprocess.TimeoutExpired:
            proc.kill()
            return {
                'success': False,
                'error': "Test timed out",
                'description': description
            }
        except Exception as e:
            return {
                'success': False,
                'error': f"Exception: {e}",
                'description': description
            }
    
    def run_comparative_analysis(self):
        """Run comparative tests between CLI and optimized MCP."""
        print("\n" + "=" * 80)
        print("🔬 COMPARATIVE ANALYSIS: CLI vs MCP Optimization")
        print("=" * 80)
        
        test_queries = [
            ("func.*Handle", "Function handler patterns"),
            ("error", "Error handling analysis"),
            ("config", "Configuration management"),
            ("interface", "Interface definitions"),
            ("struct.*User", "User data structures")
        ]
        
        results = []
        
        for query, description in test_queries:
            print(f"\n📊 Analyzing: {description}")
            
            # CLI baseline test
            cli_result = self.run_cli_benchmark(query, f"CLI: {description}")
            
            # MCP optimization test - structured format
            mcp_structured = self.run_mcp_optimization_test(
                query, 
                {
                    "max_tokens": 4000,
                    "include_examples": 2,
                    "context_format": "structured"
                },
                f"MCP Structured: {description}"
            )
            
            # MCP optimization test - markdown format  
            mcp_markdown = self.run_mcp_optimization_test(
                query,
                {
                    "max_tokens": 4000,
                    "include_examples": 2,
                    "context_format": "markdown"
                },
                f"MCP Markdown: {description}"
            )
            
            # Compare results
            comparison = {
                'query': query,
                'description': description,
                'cli': cli_result,
                'mcp_structured': mcp_structured,
                'mcp_markdown': mcp_markdown
            }
            
            results.append(comparison)
            
            # Print immediate results
            if cli_result['success'] and mcp_structured['success']:
                cli_size = cli_result['output_length']
                mcp_size = mcp_structured['response_size']
                mcp_tokens = mcp_structured['token_estimate']
                
                compression_ratio = ((cli_size - mcp_size) / cli_size * 100) if cli_size > 0 else 0
                
                print(f"  📈 CLI output: {cli_size:,} chars")
                print(f"  🎯 MCP output: {mcp_size:,} chars ({mcp_tokens} tokens)")
                print(f"  💾 Compression: {compression_ratio:.1f}% reduction")
                print(f"  ⚡ MCP examples: {mcp_structured.get('examples_count', 0)}")
                print(f"  🔍 MCP findings: {mcp_structured.get('findings_count', 0)}")
        
        return results
    
    def run_advanced_optimization_tests(self):
        """Test advanced LLM optimization features."""
        print("\n" + "=" * 80)
        print("🚀 ADVANCED OPTIMIZATION FEATURES")
        print("=" * 80)
        
        advanced_tests = [
            {
                "name": "Intent Analysis Integration",
                "query": "render",
                "params": {
                    "intent": "size_management",
                    "max_tokens": 3000,
                    "context_format": "structured"
                }
            },
            {
                "name": "Pattern Verification Integration",
                "query": "error",
                "params": {
                    "verify_pattern": "security_patterns",
                    "max_tokens": 5000,
                    "context_format": "json"
                }
            },
            {
                "name": "High Token Limit Test",
                "query": "func",
                "params": {
                    "max_tokens": 8000,
                    "include_examples": 5,
                    "context_format": "markdown"
                }
            },
            {
                "name": "Low Token Limit Test",
                "query": "interface",
                "params": {
                    "max_tokens": 1000,
                    "include_examples": 1,
                    "context_format": "structured"
                }
            }
        ]
        
        results = []
        
        for test in advanced_tests:
            print(f"\n🧪 Testing: {test['name']}")
            result = self.run_mcp_optimization_test(
                test['query'],
                test['params'],
                test['name']
            )
            
            if result['success']:
                print(f"  ✅ Success!")
                print(f"  🎯 Token estimate: {result['token_estimate']}")
                print(f"  📊 Examples: {result['examples_count']}")
                print(f"  🔍 Findings: {result['findings_count']}")
                print(f"  📝 Format: {result['output_format']}")
                print(f"  ⚡ Time: {result['time_ms']:.1f}ms")
            else:
                print(f"  ❌ Failed: {result.get('error', 'Unknown error')}")
                
            results.append(result)
            
        return results
    
    def generate_performance_report(self, comparative_results, advanced_results):
        """Generate comprehensive performance report."""
        print("\n" + "=" * 80)
        print("📊 PERFORMANCE ANALYSIS REPORT")
        print("=" * 80)
        
        # Analyze comparative results
        successful_comparisons = [r for r in comparative_results 
                                if r['cli']['success'] and r['mcp_structured']['success']]
        
        if successful_comparisons:
            total_cli_size = sum(r['cli']['output_length'] for r in successful_comparisons)
            total_mcp_size = sum(r['mcp_structured']['response_size'] for r in successful_comparisons)
            total_tokens = sum(r['mcp_structured']['token_estimate'] for r in successful_comparisons)
            avg_compression = ((total_cli_size - total_mcp_size) / total_cli_size * 100) if total_cli_size > 0 else 0
            
            print(f"\n🎯 OPTIMIZATION METRICS:")
            print(f"  📈 Total CLI output: {total_cli_size:,} characters")
            print(f"  🎯 Total MCP output: {total_mcp_size:,} characters")
            print(f"  💰 Total tokens estimated: {total_tokens:,} tokens")
            print(f"  💾 Average compression: {avg_compression:.1f}% size reduction")
            print(f"  🚀 Successful tests: {len(successful_comparisons)}/{len(comparative_results)}")
        
        # Advanced features analysis
        successful_advanced = [r for r in advanced_results if r['success']]
        
        print(f"\n🚀 ADVANCED FEATURES:")
        print(f"  ✅ Advanced tests passed: {len(successful_advanced)}/{len(advanced_results)}")
        
        if successful_advanced:
            avg_tokens = sum(r['token_estimate'] for r in successful_advanced) / len(successful_advanced)
            avg_time = sum(r['time_ms'] for r in successful_advanced) / len(successful_advanced)
            avg_examples = sum(r['examples_count'] for r in successful_advanced) / len(successful_advanced)
            avg_findings = sum(r['findings_count'] for r in successful_advanced) / len(successful_advanced)
            
            print(f"  🎯 Average token usage: {avg_tokens:.0f} tokens")
            print(f"  ⚡ Average processing time: {avg_time:.1f}ms")
            print(f"  📊 Average examples per result: {avg_examples:.1f}")
            print(f"  🔍 Average findings per result: {avg_findings:.1f}")
        
        # Overall assessment
        total_tests = len(comparative_results) * 3 + len(advanced_results)  # CLI + 2 MCP formats + advanced
        successful_tests = len(successful_comparisons) * 3 + len(successful_advanced)
        success_rate = (successful_tests / total_tests * 100) if total_tests > 0 else 0
        
        print(f"\n🎉 OVERALL ASSESSMENT:")
        print(f"  📊 Success Rate: {success_rate:.1f}% ({successful_tests}/{total_tests} tests)")
        
        if success_rate >= 90:
            print(f"  🚀 EXCELLENT: Production ready for AI assistant integration!")
        elif success_rate >= 80:
            print(f"  ✅ GOOD: Minor optimizations recommended before production")
        elif success_rate >= 70:
            print(f"  ⚠️ FAIR: Some issues need addressing before production")
        else:
            print(f"  ❌ NEEDS WORK: Significant issues require resolution")
            
        return {
            'success_rate': success_rate,
            'total_tests': total_tests,
            'successful_tests': successful_tests,
            'compression_ratio': avg_compression if successful_comparisons else 0,
            'average_tokens': avg_tokens if successful_advanced else 0
        }
    
    def run_full_production_test_suite(self):
        """Run complete production test suite."""
        print("🚀 LIGHTNING CODE INDEX - PRODUCTION TEST SUITE")
        print("=" * 80)
        print("Testing LLM optimization features for real-world AI assistant integration")
        print("=" * 80)
        
        # Build first
        print("🔨 Building LCI...")
        build_result = subprocess.run(['go', 'build', './cmd/lci'], 
                                    capture_output=True, text=True)
        
        if build_result.returncode != 0:
            print(f"❌ Build failed: {build_result.stderr}")
            return False
        print("✅ Build successful")
        
        try:
            # Run test suites
            comparative_results = self.run_comparative_analysis()
            advanced_results = self.run_advanced_optimization_tests()
            
            # Generate comprehensive report
            final_metrics = self.generate_performance_report(comparative_results, advanced_results)
            
            # Save detailed results
            detailed_report = {
                'timestamp': time.time(),
                'comparative_tests': comparative_results,
                'advanced_tests': advanced_results,
                'final_metrics': final_metrics
            }
            
            with open('production_test_results.json', 'w') as f:
                json.dump(detailed_report, f, indent=2)
            
            print(f"\n📄 Detailed results saved to: production_test_results.json")
            print(f"🎯 Ready for AI assistant integration testing!")
            
            return final_metrics['success_rate'] >= 80
            
        except Exception as e:
            print(f"❌ Test suite failed with error: {e}")
            return False

def main():
    """Run the complete production test suite."""
    tester = LCIProductionTester()
    success = tester.run_full_production_test_suite()
    sys.exit(0 if success else 1)

if __name__ == "__main__":
    main()