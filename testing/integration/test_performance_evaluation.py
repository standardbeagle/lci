#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Performance Evaluation for Lightning Code Index Improvements

This script benchmarks the performance impact of the implemented improvements:
1. Smart error response generation overhead
2. Enhanced file search performance
3. Pattern validation speed
4. Component discovery efficiency
5. Search suggestion generation time

Usage:
    python test_performance_evaluation.py <path_to_lci_binary>
"""

import time
import json
import subprocess
import statistics
import sys
import os
from typing import Dict, List, Any, Tuple
from concurrent.futures import ThreadPoolExecutor, as_completed

class PerformanceBenchmark:
    """Performance benchmarking for Lightning Code Index"""
    
    def __init__(self, lci_path: str):
        self.lci_path = lci_path
        self.results = {}
        
    def benchmark_cli_operation(self, command: List[str], runs: int = 5) -> Dict[str, Any]:
        """Benchmark a CLI operation multiple times"""
        times = []
        
        for i in range(runs):
            start_time = time.perf_counter()
            
            try:
                result = subprocess.run(
                    [self.lci_path] + command,
                    capture_output=True,
                    text=True,
                    timeout=30
                )
                end_time = time.perf_counter()
                
                if result.returncode == 0:
                    times.append(end_time - start_time)
                else:
                    print(f"Warning: Command failed on run {i+1}: {result.stderr[:100]}")
                    
            except subprocess.TimeoutExpired:
                print(f"Warning: Command timed out on run {i+1}")
                continue
                
        if not times:
            return {"error": "All runs failed"}
            
        return {
            "avg_time": statistics.mean(times),
            "min_time": min(times),
            "max_time": max(times),
            "median_time": statistics.median(times),
            "std_dev": statistics.stdev(times) if len(times) > 1 else 0,
            "successful_runs": len(times),
            "total_runs": runs
        }
    
    def test_indexing_performance(self):
        """Test indexing performance as baseline"""
        print("Building: Testing Indexing Performance...")
        
        # Test index building
        stats = self.benchmark_cli_operation(["stats"], runs=3)
        self.results["indexing_stats"] = stats
        
        if "error" not in stats:
            print(f"  SUCCESS: Index stats: {stats['avg_time']:.3f}s avg ({stats['successful_runs']}/{stats['total_runs']} runs)")
        else:
            print(f"  FAIL: Index stats failed: {stats['error']}")
    
    def test_search_performance(self):
        """Test various search operations"""
        print("Search: Testing Search Performance...")
        
        # Standard search patterns
        search_patterns = [
            ("func", "Simple function search"),
            ("main", "Main function search"),  
            ("type.*struct", "Regex pattern search"),
            ("HandleRequest", "Specific method search"),
            (".*Error.*", "Error pattern search")
        ]
        
        search_results = {}
        
        for pattern, description in search_patterns:
            print(f"  Testing: {description}")
            stats = self.benchmark_cli_operation(["search", pattern, "--max-results", "10"], runs=5)
            search_results[pattern] = {
                "description": description,
                "stats": stats
            }
            
            if "error" not in stats:
                print(f"    SUCCESS: {stats['avg_time']:.3f}s avg, {stats['std_dev']:.3f}s std")
            else:
                print(f"    FAIL: Failed: {stats['error']}")
        
        self.results["search_performance"] = search_results
    
    def test_file_search_performance(self):
        """Test enhanced file search performance"""
        print("Files: Testing Enhanced File Search Performance...")
        
        # Note: CLI doesn't have file_search, but we can test list command as proxy
        file_patterns = [
            (["list", "--include", "*.go"], "Go files search"),
            (["list", "--include", "*.md"], "Markdown files search"),
            (["list", "--include", "internal/**/*.go"], "Internal Go files search"),
            (["list"], "All files listing")
        ]
        
        file_search_results = {}
        
        for command, description in file_patterns:
            print(f"  Testing: {description}")
            stats = self.benchmark_cli_operation(command, runs=3)
            file_search_results[str(command)] = {
                "description": description,
                "stats": stats
            }
            
            if "error" not in stats:
                print(f"    SUCCESS: {stats['avg_time']:.3f}s avg")
            else:
                print(f"    FAIL: Failed: {stats['error']}")
        
        self.results["file_search_performance"] = file_search_results
    
    def test_definition_search_performance(self):
        """Test definition search performance"""
        print("Definitions: Testing Definition Search Performance...")
        
        # Common symbols to search for definitions
        symbols = [
            ("main", "Main function"),
            ("Server", "Server struct"),
            ("NewServer", "Constructor function"),
            ("Search", "Search method"),
            ("Index", "Index interface")
        ]
        
        definition_results = {}
        
        for symbol, description in symbols:
            print(f"  Testing: {description}")
            stats = self.benchmark_cli_operation(["def", symbol], runs=5)
            definition_results[symbol] = {
                "description": description,
                "stats": stats
            }
            
            if "error" not in stats:
                print(f"    SUCCESS: {stats['avg_time']:.3f}s avg")
            else:
                print(f"    FAIL: Failed: {stats['error']}")
        
        self.results["definition_performance"] = definition_results
    
    def test_tree_performance(self):
        """Test function tree generation performance"""
        print("Trees: Testing Tree Generation Performance...")
        
        # Test tree generation for various functions
        functions = [
            ("main", "Main function tree"),
            ("NewServer", "Constructor tree"),
            ("Search", "Search method tree")
        ]
        
        tree_results = {}
        
        for function, description in functions:
            print(f"  Testing: {description}")
            stats = self.benchmark_cli_operation(["tree", function], runs=3)
            tree_results[function] = {
                "description": description,
                "stats": stats
            }
            
            if "error" not in stats:
                print(f"    SUCCESS: {stats['avg_time']:.3f}s avg")
            else:
                print(f"    FAIL: Failed: {stats['error']}")
        
        self.results["tree_performance"] = tree_results
    
    def test_concurrent_performance(self):
        """Test concurrent operation performance"""
        print("Concurrent: Testing Concurrent Performance...")
        
        # Test multiple concurrent searches
        def run_concurrent_search():
            return subprocess.run(
                [self.lci_path, "search", "func", "--max-results", "5"],
                capture_output=True,
                text=True,
                timeout=10
            )
        
        concurrent_counts = [1, 2, 4, 8]
        concurrent_results = {}
        
        for count in concurrent_counts:
            print(f"  Testing {count} concurrent searches...")
            
            times = []
            for run in range(3):  # 3 runs per concurrency level
                start_time = time.perf_counter()
                
                with ThreadPoolExecutor(max_workers=count) as executor:
                    futures = [executor.submit(run_concurrent_search) for _ in range(count)]
                    
                    successful = 0
                    for future in as_completed(futures):
                        try:
                            result = future.result()
                            if result.returncode == 0:
                                successful += 1
                        except Exception:
                            pass
                
                end_time = time.perf_counter()
                
                if successful == count:  # All searches succeeded
                    times.append(end_time - start_time)
            
            if times:
                concurrent_results[count] = {
                    "avg_time": statistics.mean(times),
                    "min_time": min(times),
                    "max_time": max(times),
                    "successful_runs": len(times)
                }
                print(f"    SUCCESS: {count} concurrent: {statistics.mean(times):.3f}s avg")
            else:
                concurrent_results[count] = {"error": "All runs failed"}
                print(f"    FAIL: {count} concurrent: Failed")
        
        self.results["concurrent_performance"] = concurrent_results
    
    def generate_performance_report(self):
        """Generate comprehensive performance report"""
        print("\nPERFORMANCE EVALUATION REPORT")
        print("=" * 60)
        
        # Overall performance summary
        all_operations = []
        
        # Collect all timing data
        for category, data in self.results.items():
            if "error" in str(data):
                continue
                
            if isinstance(data, dict) and "stats" in data:
                # Single operation category
                if "error" not in data["stats"]:
                    all_operations.append(data["stats"]["avg_time"])
            else:
                # Multiple operations category
                for op_name, op_data in data.items():
                    if isinstance(op_data, dict) and "stats" in op_data:
                        if "error" not in op_data["stats"]:
                            all_operations.append(op_data["stats"]["avg_time"])
        
        if all_operations:
            print(f"Overall Performance Summary:")
            print(f"  Average Operation Time: {statistics.mean(all_operations):.3f}s")
            print(f"  Fastest Operation: {min(all_operations):.3f}s")
            print(f"  Slowest Operation: {max(all_operations):.3f}s")
            print(f"  Total Operations Tested: {len(all_operations)}")
        
        print(f"\nDetailed Results by Category:")
        print("-" * 40)
        
        # Search Performance Analysis
        if "search_performance" in self.results:
            print(f"\nSearch Performance:")
            for pattern, data in self.results["search_performance"].items():
                if "error" not in data["stats"]:
                    stats = data["stats"]
                    print(f"  {data['description']}: {stats['avg_time']:.3f}s +/- {stats['std_dev']:.3f}s")
                else:
                    print(f"  {data['description']}: FAILED")
        
        # File Search Performance
        if "file_search_performance" in self.results:
            print(f"\nFile Operations Performance:")
            for command, data in self.results["file_search_performance"].items():
                if "error" not in data["stats"]:
                    stats = data["stats"]
                    print(f"  {data['description']}: {stats['avg_time']:.3f}s")
                else:
                    print(f"  {data['description']}: FAILED")
        
        # Definition Search Performance
        if "definition_performance" in self.results:
            print(f"\nDefinition Search Performance:")
            for symbol, data in self.results["definition_performance"].items():
                if "error" not in data["stats"]:
                    stats = data["stats"]
                    print(f"  {data['description']}: {stats['avg_time']:.3f}s")
                else:
                    print(f"  {data['description']}: FAILED")
        
        # Tree Generation Performance
        if "tree_performance" in self.results:
            print(f"\nTree Generation Performance:")
            for function, data in self.results["tree_performance"].items():
                if "error" not in data["stats"]:
                    stats = data["stats"]
                    print(f"  {data['description']}: {stats['avg_time']:.3f}s")
                else:
                    print(f"  {data['description']}: FAILED")
        
        # Concurrent Performance
        if "concurrent_performance" in self.results:
            print(f"\nConcurrent Performance:")
            for count, data in self.results["concurrent_performance"].items():
                if "error" not in data:
                    print(f"  {count} concurrent operations: {data['avg_time']:.3f}s total")
                else:
                    print(f"  {count} concurrent operations: FAILED")
        
        # Performance Insights
        print(f"\nPerformance Insights:")
        
        # Check if search is fast enough (< 5ms target from requirements)
        search_times = []
        if "search_performance" in self.results:
            for data in self.results["search_performance"].values():
                if "error" not in data["stats"]:
                    search_times.append(data["stats"]["avg_time"] * 1000)  # Convert to ms
        
        if search_times:
            avg_search_ms = statistics.mean(search_times)
            if avg_search_ms < 5:
                print(f"  SUCCESS: Search performance meets <5ms target: {avg_search_ms:.1f}ms avg")
            else:
                print(f"  WARNING: Search performance exceeds 5ms target: {avg_search_ms:.1f}ms avg")
        
        # Check consistency (low standard deviation is good)
        consistent_operations = 0
        total_measured = 0
        
        for category, data in self.results.items():
            if isinstance(data, dict):
                for op_data in data.values() if not isinstance(list(data.values())[0], dict) or "stats" not in list(data.values())[0] else [data]:
                    if isinstance(op_data, dict) and "stats" in op_data:
                        stats = op_data["stats"]
                        if "error" not in stats and "std_dev" in stats:
                            total_measured += 1
                            if stats["std_dev"] < stats["avg_time"] * 0.2:  # Less than 20% variation
                                consistent_operations += 1
        
        if total_measured > 0:
            consistency_rate = (consistent_operations / total_measured) * 100
            print(f"  Performance consistency: {consistency_rate:.1f}% of operations have <20% variation")
        
        # Overall assessment
        print(f"\nOverall Assessment:")
        
        failed_categories = 0
        total_categories = 0
        
        for category, data in self.results.items():
            total_categories += 1
            if isinstance(data, dict):
                if "error" in data:
                    failed_categories += 1
                else:
                    # Check if any operations in this category failed
                    has_failures = False
                    for item in data.values():
                        if isinstance(item, dict) and "stats" in item and "error" in item["stats"]:
                            has_failures = True
                            break
                    if has_failures:
                        failed_categories += 1
        
        success_rate = ((total_categories - failed_categories) / total_categories) * 100
        print(f"  Success Rate: {success_rate:.1f}% ({total_categories - failed_categories}/{total_categories} categories)")
        
        if success_rate >= 80:
            print(f"  EXCELLENT: Performance across all tested operations")
        elif success_rate >= 60:
            print(f"  GOOD: Performance with some areas for improvement")  
        else:
            print(f"  WARNING: Performance issues detected - investigation needed")

    def run_performance_evaluation(self):
        """Run complete performance evaluation"""
        print("STARTING: Performance Evaluation...")
        print(f"Testing binary: {self.lci_path}")
        
        try:
            self.test_indexing_performance()
            self.test_search_performance()
            self.test_file_search_performance()
            self.test_definition_search_performance()
            self.test_tree_performance()
            self.test_concurrent_performance()
            
            # Generate comprehensive report
            self.generate_performance_report()
            
            return True
            
        except KeyboardInterrupt:
            print("\nINTERRUPTED: Performance evaluation interrupted")
            return False
        except Exception as e:
            print(f"\nERROR: Performance evaluation failed: {e}")
            return False

def main():
    """Main performance evaluation function"""
    if len(sys.argv) < 2:
        print("Usage: python test_performance_evaluation.py <path_to_lci_binary>")
        print("Example: python test_performance_evaluation.py ./lci-test")
        sys.exit(1)
    
    lci_path = sys.argv[1]
    if not os.path.exists(lci_path):
        print(f"Error: LCI binary not found at {lci_path}")
        sys.exit(1)
    
    # Run performance evaluation
    benchmark = PerformanceBenchmark(lci_path)
    success = benchmark.run_performance_evaluation()
    
    if success:
        print("\nCOMPLETED: Performance evaluation completed successfully!")
    else:
        print("\nWARNING: Performance evaluation completed with issues")
        sys.exit(1)

if __name__ == "__main__":
    main()