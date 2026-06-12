package benchmarks

import (
	"sync"
	"time"
)

// naiveEntry is a heap-allocated wrapper around each cached value.
// Unlike luna's arena, every insert allocates one of these and the
// map retains its high-water bucket count after deletes.
type naiveEntry struct {
	value     int
	expiresAt time.Time
}

// naiveCache is a minimal TTL cache: map[string]*naiveEntry + RWMutex.
// It mirrors what many hand-rolled caches look like before reaching
// for a library.
type naiveCache struct {
	mu      sync.RWMutex
	entries map[string]*naiveEntry
	ttl     time.Duration
}

func newNaiveCache() *naiveCache {
	return &naiveCache{
		entries: make(map[string]*naiveEntry),
		ttl:     ttl,
	}
}

func (c *naiveCache) Insert(key string, value int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[key]; ok {
		e.value = value
		e.expiresAt = time.Now().Add(c.ttl)
		return
	}
	c.entries[key] = &naiveEntry{value: value, expiresAt: time.Now().Add(c.ttl)}
}

func (c *naiveCache) Get(key string) (int, bool) {
	now := time.Now()
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok {
		return 0, false
	}
	if now.After(e.expiresAt) {
		c.mu.Lock()
		delete(c.entries, key)
		c.mu.Unlock()
		return 0, false
	}
	return e.value, true
}

func (c *naiveCache) Delete(key string) {
	c.mu.Lock()
	delete(c.entries, key)
	c.mu.Unlock()
}

func (c *naiveCache) Stop() {}
