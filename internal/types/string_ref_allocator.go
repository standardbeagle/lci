package types

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

const (
	// StringRefsPerBlock - optimized for cache efficiency
	// 20 bytes per StringRef, ~3 refs per 64-byte cache line
	// 4096 StringRefs = ~80KB per block
	StringRefsPerBlock = 4096
)

// StringRefBlock is a block of pre-allocated StringRefs
type StringRefBlock struct {
	refs [StringRefsPerBlock]StringRef
	used int
	next *StringRefBlock
}

// StringRefAllocator provides efficient block-based allocation for StringRefs
type StringRefAllocator struct {
	mu          sync.Mutex
	current     *StringRefBlock
	freeList    *StringRefBlock
	totalBlocks atomic.Int32
	activeRefs  atomic.Int64

	// Pool of blocks for reuse
	blockPool sync.Pool
}

// GlobalStringRefAllocator is the application-wide StringRef allocator
var GlobalStringRefAllocator = &StringRefAllocator{
	blockPool: sync.Pool{
		New: func() interface{} {
			return &StringRefBlock{}
		},
	},
}

// Allocate returns a pointer to a new StringRef
func (a *StringRefAllocator) Allocate() *StringRef {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Need new block?
	if a.current == nil || a.current.used >= StringRefsPerBlock {
		a.allocateNewBlock()
	}

	ref := &a.current.refs[a.current.used]
	a.current.used++
	a.activeRefs.Add(1)
	return ref
}

// AllocateSlice pre-allocates a slice of StringRefs from the block allocator
func (a *StringRefAllocator) AllocateSlice(n int) []StringRef {
	if n <= 0 {
		return nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// For very large allocations, fall back to regular allocation
	if n > StringRefsPerBlock/2 {
		a.activeRefs.Add(int64(n))
		return make([]StringRef, n)
	}

	// Can we fit in current block?
	if a.current != nil && a.current.used+n <= StringRefsPerBlock {
		start := a.current.used
		a.current.used += n
		a.activeRefs.Add(int64(n))
		return a.current.refs[start : start+n]
	}

	// Need new block
	a.allocateNewBlock()
	a.current.used = n
	a.activeRefs.Add(int64(n))
	return a.current.refs[0:n]
}

// AllocateBatch allocates multiple StringRefs and returns them individually
func (a *StringRefAllocator) AllocateBatch(n int) []*StringRef {
	if n <= 0 {
		return nil
	}

	refs := make([]*StringRef, n)

	a.mu.Lock()
	defer a.mu.Unlock()

	for i := 0; i < n; i++ {
		// Need new block?
		if a.current == nil || a.current.used >= StringRefsPerBlock {
			a.allocateNewBlock()
		}

		refs[i] = &a.current.refs[a.current.used]
		a.current.used++
	}

	a.activeRefs.Add(int64(n))
	return refs
}

// allocateNewBlock gets a block from the free list or creates a new one
func (a *StringRefAllocator) allocateNewBlock() {
	var block *StringRefBlock

	// Try free list first
	if a.freeList != nil {
		block = a.freeList
		a.freeList = block.next
		block.used = 0
		block.next = nil
		// Clear the block to ensure clean state
		for i := range block.refs {
			block.refs[i] = StringRef{}
		}
	} else {
		// Get from pool or allocate new
		block = a.blockPool.Get().(*StringRefBlock)
		a.totalBlocks.Add(1)
	}

	block.next = a.current
	a.current = block
}

// Reset returns all blocks to the free list for reuse
func (a *StringRefAllocator) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Move all blocks to free list
	for a.current != nil {
		block := a.current
		a.current = block.next
		block.next = a.freeList
		a.freeList = block
	}

	a.activeRefs.Store(0)
}

// ReturnToPool returns blocks to the sync.Pool to be GC'd if needed
func (a *StringRefAllocator) ReturnToPool() {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Return current blocks
	for a.current != nil {
		block := a.current
		a.current = block.next
		a.blockPool.Put(block)
	}

	// Return free list blocks
	for a.freeList != nil {
		block := a.freeList
		a.freeList = block.next
		a.blockPool.Put(block)
	}

	a.activeRefs.Store(0)
}

// Stats represents allocator statistics
type AllocatorStats struct {
	TotalBlocks      int
	ActiveRefs       int64
	BlockSizeBytes   int
	TotalMemoryBytes int
	RefsPerBlock     int
}

// Stats returns current allocator statistics
func (a *StringRefAllocator) Stats() AllocatorStats {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Count blocks in use
	blocksInUse := 0
	for block := a.current; block != nil; block = block.next {
		blocksInUse++
	}

	// Count blocks in free list
	freeBlocks := 0
	for block := a.freeList; block != nil; block = block.next {
		freeBlocks++
	}

	totalBlocks := int(a.totalBlocks.Load())
	blockSize := int(unsafe.Sizeof(StringRefBlock{}))

	return AllocatorStats{
		TotalBlocks:      totalBlocks,
		ActiveRefs:       a.activeRefs.Load(),
		BlockSizeBytes:   blockSize,
		TotalMemoryBytes: totalBlocks * blockSize,
		RefsPerBlock:     StringRefsPerBlock,
	}
}

// Utility functions for creating StringRefs with the allocator

// NewStringRef allocates and initializes a new StringRef
func (a *StringRefAllocator) NewStringRef(fileID FileID, start, length uint32, hash uint64) *StringRef {
	ref := a.Allocate()
	ref.FileID = fileID
	ref.Offset = start
	ref.Length = length
	ref.Hash = hash
	return ref
}

// NewStringRefSlice allocates and initializes a slice of StringRefs
func (a *StringRefAllocator) NewStringRefSlice(fileID FileID, offsets []uint32) []StringRef {
	if len(offsets) == 0 {
		return nil
	}

	refs := a.AllocateSlice(len(offsets) - 1)
	for i := 0; i < len(offsets)-1; i++ {
		refs[i].FileID = fileID
		refs[i].Offset = offsets[i]
		refs[i].Length = offsets[i+1] - offsets[i]
		// Hash will be computed later if needed
	}

	return refs
}
