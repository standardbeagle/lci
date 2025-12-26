package semantic

import (
	"container/list"
	"sync"
)

// LRUCache is a thread-safe least-recently-used cache with a maximum size
type LRUCache struct {
	maxSize int
	mu      sync.RWMutex
	items   map[string]*list.Element
	order   *list.List
}

// cacheEntry represents an entry in the cache
type cacheEntry struct {
	key   string
	value *normalizedQuery
}

// NewLRUCache creates a new LRU cache with the specified maximum size
func NewLRUCache(maxSize int) *LRUCache {
	if maxSize <= 0 {
		maxSize = 100 // Default size
	}
	return &LRUCache{
		maxSize: maxSize,
		items:   make(map[string]*list.Element),
		order:   list.New(),
	}
}

// Get retrieves a value from the cache and marks it as recently used
func (c *LRUCache) Get(key string) (*normalizedQuery, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		// Move to front (mark as recently used)
		c.order.MoveToFront(elem)
		return elem.Value.(*cacheEntry).value, true
	}
	return nil, false
}

// Set adds or updates a value in the cache
func (c *LRUCache) Set(key string, value *normalizedQuery) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if key already exists
	if elem, ok := c.items[key]; ok {
		// Update existing entry and move to front
		c.order.MoveToFront(elem)
		elem.Value.(*cacheEntry).value = value
		return
	}

	// Add new entry
	entry := &cacheEntry{key: key, value: value}
	elem := c.order.PushFront(entry)
	c.items[key] = elem

	// Evict oldest if over capacity
	if c.order.Len() > c.maxSize {
		oldest := c.order.Back()
		if oldest != nil {
			c.order.Remove(oldest)
			delete(c.items, oldest.Value.(*cacheEntry).key)
		}
	}
}

// Clear removes all entries from the cache
func (c *LRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element)
	c.order = list.New()
}

// Size returns the current number of items in the cache
func (c *LRUCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.order.Len()
}