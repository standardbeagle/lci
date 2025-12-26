package parser

import (
	"context"
	"testing"
	"time"

	"github.com/standardbeagle/lci/internal/types"
)

// BenchmarkExtractJSReferences benchmarks the optimized context-retention implementation
func BenchmarkExtractJSReferences(b *testing.B) {
	// JavaScript code with multiple nested calls and member expressions
	// This creates the worst-case scenario for parent chain walking
	jsCode := `
function calculateTotal(items, tax, discount) {
    const subtotal = items.reduce((sum, item) => sum + item.price, 0);
    const taxAmount = subtotal * tax;
    const discountAmount = subtotal * discount;
    return subtotal + taxAmount - discountAmount;
}

class ShoppingCart {
    constructor(user) {
        this.user = user;
        this.items = [];
    }

    addItem(item) {
        this.items.push(item);
        this.updateTotal();
    }

    removeItem(itemId) {
        this.items = this.items.filter(item => item.id !== itemId);
        this.updateTotal();
    }

    updateTotal() {
        this.total = calculateTotal(this.items, 0.08, 0.10);
    }

    checkout() {
        return this.user.processPayment(this.total);
    }
}

class User {
    constructor(name, email) {
        this.name = name;
        this.email = email;
    }

    processPayment(amount) {
        console.log("Processing payment of $" + amount + " for " + this.name);
        return true;
    }
}

const cart = new ShoppingCart(new User("John", "john@example.com"));
cart.addItem({id: 1, name: "Product 1", price: 29.99});
cart.addItem({id: 2, name: "Product 2", price: 49.99});
const result = cart.checkout();
`

	// Initialize parser
	parser := NewTreeSitterParser()
	fileID := types.FileID(1)

	// Parse the code once to get the AST
	tree, _, _, _, _, _, _ := parser.ParseFileEnhancedWithASTAndContextStringRef(context.TODO(), "test.js", []byte(jsCode), fileID)

	if tree == nil {
		b.Fatal("Parse result tree is nil")
	}

	// Reset the benchmark timer to exclude parsing overhead
	b.ResetTimer()

	// Benchmark the reference extraction with our optimized UnifiedExtractor implementation
	for i := 0; i < b.N; i++ {
		// Use UnifiedExtractor for single-pass extraction
		extractor := NewUnifiedExtractor(parser, []byte(jsCode), fileID, ".js", "test.js")
		extractor.Extract(tree)
		_, references, _, _ := extractor.GetResults()

		// Ensure we're actually getting results (prevent optimization)
		if len(references) == 0 {
			b.Fatal("No references extracted")
		}
	}
}

// BenchmarkContextOperations benchmarks just the context operations to ensure they're fast
func BenchmarkContextOperations(b *testing.B) {
	ctx := NewVisitContext()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Simulate the operations that would happen during traversal
		ctx.PushParent("call_expression")
		ctx.MarkFunctionHandled(uintptr(i))

		ctx.PushParent("member_expression")
		ctx.MarkPropertyHandled(uintptr(i + 1))

		// Simulate context checks (these should be O(1))
		_ = ctx.IsFunctionHandled(uintptr(i))
		_ = ctx.IsPropertyHandled(uintptr(i + 1))
		_ = ctx.inImportStatement
		_ = ctx.GetImmediateParent()
		_ = ctx.IsInParentType("call_expression")

		ctx.PopParent()
		ctx.PopParent()
	}
}

// Test optimized performance with realistic data
func TestOptimizedPerformance(t *testing.T) {
	// JavaScript code with realistic complexity
	jsCode := `
import React, { useState, useEffect } from 'react';
import axios from 'axios';

const UserProfile = ({ userId }) => {
    const [user, setUser] = useState(null);
    const [posts, setPosts] = useState([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState(null);

    useEffect(() => {
        const fetchUserData = async () => {
            try {
                setLoading(true);
                const userResponse = await axios.get("/api/users/" + userId);
                const postsResponse = await axios.get("/api/users/" + userId + "/posts");

                setUser(userResponse.data);
                setPosts(postsResponse.data);
                setError(null);
            } catch (err) {
                setError(err.message);
                setUser(null);
                setPosts([]);
            } finally {
                setLoading(false);
            }
        };

        if (userId) {
            fetchUserData();
        }
    }, [userId]);

    const handleRefresh = () => {
        fetchUserData();
    };

    const handleDeletePost = async (postId) => {
        try {
            await axios.delete("/api/posts/" + postId);
            setPosts(posts.filter(post => post.id !== postId));
        } catch (err) {
            setError(err.message);
        }
    };

    if (loading) {
        return <div className="loading">Loading...</div>;
    }

    if (error) {
        return <div className="error">Error: {error}</div>;
    }

    if (!user) {
        return <div className="not-found">User not found</div>;
    }

    return (
        <div className="user-profile">
            <h1>{user.name}</h1>
            <p>{user.email}</p>
            <button onClick={handleRefresh}>Refresh</button>
            <div className="posts">
                <h2>Posts</h2>
                {posts.map(post => (
                    <div key={post.id} className="post">
                        <h3>{post.title}</h3>
                        <p>{post.content}</p>
                        <button onClick={() => handleDeletePost(post.id)}>
                            Delete
                        </button>
                    </div>
                ))}
            </div>
        </div>
    );
};

export default UserProfile;
`

	parser := NewTreeSitterParser()
	fileID := types.FileID(1)

	start := time.Now()

	// Parse the code
	tree, _, _, _, _, _, _ := parser.ParseFileEnhancedWithASTAndContextStringRef(context.TODO(), "test.js", []byte(jsCode), fileID)

	if tree == nil {
		t.Fatal("Parse result tree is nil")
	}

	parseTime := time.Since(start)

	start = time.Now()

	// Extract references using optimized UnifiedExtractor implementation
	extractor := NewUnifiedExtractor(parser, []byte(jsCode), fileID, ".js", "test.js")
	extractor.Extract(tree)
	_, references, _, _ := extractor.GetResults()

	extractionTime := time.Since(start)

	t.Logf("Performance Results:")
	t.Logf("  Parse time: %v", parseTime)
	t.Logf("  Reference extraction time: %v", extractionTime)
	t.Logf("  References extracted: %d", len(references))

	// Ensure extraction is reasonably fast (should be under 10ms for this file)
	// Retry with backoff to handle timing variance
	maxRetries := 2
	for attempt := 0; attempt < maxRetries; attempt++ {
		if extractionTime <= 10*time.Millisecond {
			break // Success
		}
		if attempt < maxRetries-1 {
			t.Logf("Reference extraction attempt %d/%d: %v (threshold: 10ms), retrying...", attempt+1, maxRetries, extractionTime)
			time.Sleep(100 * time.Millisecond)
			// Re-run extraction with fresh timer
			start = time.Now()
			extractor := NewUnifiedExtractor(parser, []byte(jsCode), fileID, ".js", "test.js")
			extractor.Extract(tree)
			_, references, _, _ = extractor.GetResults()
			extractionTime = time.Since(start)
			continue
		}
		t.Errorf("Reference extraction too slow: %v (expected < 10ms)", extractionTime)
	}

	// Verify we're extracting a reasonable number of references
	if len(references) < 20 {
		t.Errorf("Too few references extracted: %d (expected > 20)", len(references))
	}
}
