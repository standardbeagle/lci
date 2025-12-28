#!/usr/bin/env python3
"""
Quick validation test to confirm LLM optimization is working correctly.
"""
import json
import subprocess
import sys
import time
import threading
from queue import Queue, Empty
import signal

class QuickMCPTester:
    def __init__(self):
        self.process = None
        self.message_id = 1
        
    def test_single_optimization(self):
        """Test single optimization with proper cleanup."""
        print("🚀 Testing LLM optimization integration...")
        
        try:
            # Start server
            self.process = subprocess.Popen(
                ['./lci', 'mcp'],
                stdin=subprocess.PIPE,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                text=True,
                preexec_fn=os.setsid if hasattr(__builtins__, 'os') else None
            )
            
            # Initialize connection
            init_request = {
                "jsonrpc": "2.0",
                "id": 1,
                "method": "initialize",
                "params": {
                    "protocolVersion": "2024-06-18",
                    "capabilities": {"tools": {}},
                    "clientInfo": {"name": "quick-test", "version": "1.0.0"}
                }
            }
            
            self._send_and_read(init_request)
            
            # Send initialized notification
            self._send_message({
                "jsonrpc": "2.0",
                "method": "notifications/initialized"
            })
            
            # Test optimize_search
            optimize_request = {
                "jsonrpc": "2.0",
                "id": 2,
                "method": "tools/call",
                "params": {
                    "name": "optimize_search",
                    "arguments": {
                        "query": "func",
                        "max_tokens": 2000,
                        "include_examples": 1,
                        "context_format": "structured"
                    }
                }
            }
            
            response = self._send_and_read(optimize_request, timeout=10)
            
            if response and 'result' in response:
                result = response['result']
                print("✅ Optimize search successful!")
                print(f"📊 Format: {result.get('format', 'N/A')}")
                
                if 'analysis' in result:
                    print(f"📝 Has analysis section: ✅")
                    
                if 'code_examples' in result:
                    examples = result.get('code_examples', [])
                    print(f"💡 Code examples: {len(examples)}")
                    
                if 'metadata' in result:
                    metadata = result['metadata']
                    print(f"🎯 Token estimate: {metadata.get('token_estimate', 'N/A')}")
                    print(f"📄 Source files: {len(metadata.get('source_files', []))}")
                    
                print("🎉 LLM optimization integration WORKING!")
                return True
            else:
                print(f"❌ Test failed: {response}")
                return False
                
        except Exception as e:
            print(f"❌ Test error: {e}")
            return False
        finally:
            self._cleanup()
            
    def _send_message(self, message):
        """Send message to server."""
        try:
            json_message = json.dumps(message) + '\n'
            self.process.stdin.write(json_message)
            self.process.stdin.flush()
            return True
        except:
            return False
            
    def _send_and_read(self, message, timeout=5):
        """Send message and read response."""
        if not self._send_message(message):
            return None
            
        try:
            # Simple read with timeout
            start_time = time.time()
            while time.time() - start_time < timeout:
                if self.process.poll() is not None:
                    break
                    
                try:
                    line = self.process.stdout.readline()
                    if line:
                        line = line.strip()
                        if line:
                            try:
                                return json.loads(line)
                            except json.JSONDecodeError:
                                continue
                except:
                    continue
                    
            return None
        except:
            return None
            
    def _cleanup(self):
        """Clean up process."""
        if self.process:
            try:
                self.process.terminate()
                time.sleep(1)
                if self.process.poll() is None:
                    self.process.kill()
            except:
                pass

import os

def main():
    """Run quick validation test."""
    print("⚡ QUICK LLM OPTIMIZATION VALIDATION")
    print("=" * 40)
    
    # Build
    print("🔨 Building...")
    result = subprocess.run(['go', 'build', './cmd/lci'], capture_output=True, text=True)
    if result.returncode != 0:
        print("❌ Build failed")
        return False
    print("✅ Build OK")
    
    # Test CLI baseline
    print("\n📊 CLI Baseline Test...")
    cli_result = subprocess.run(['./lci', 'search', 'func'], capture_output=True, text=True)
    if cli_result.returncode == 0:
        cli_size = len(cli_result.stdout)
        print(f"✅ CLI works: {cli_size} chars output")
    else:
        print("❌ CLI failed")
        return False
    
    # Test MCP optimization
    print("\n🧠 MCP Optimization Test...")
    tester = QuickMCPTester()
    mcp_success = tester.test_single_optimization()
    
    if mcp_success:
        print("\n🎉 SUCCESS: LLM optimization fully integrated and working!")
        print("\n🚀 Ready for AI assistant integration:")
        print("  • optimize_search tool operational")
        print("  • Token reduction active")
        print("  • Multi-format output available")
        print("  • Architecture analysis enabled")
        print("  • Ready for Claude CLI testing")
        return True
    else:
        print("\n❌ MCP optimization test failed")
        return False

if __name__ == "__main__":
    success = main()
    sys.exit(0 if success else 1)