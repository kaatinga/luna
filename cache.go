package luna

import (
	"hash/maphash"
	"math"
	"sync"
	"time"

	"github.com/kaatinga/luna/internal/swiss"
)

const year = time.Hour * 24 * 365

// Cache is a TTL cache backed by an open-addressing swiss table. Entries
// expire a fixed TTL after they were inserted or, unless disabled, last
// retrieved. A single timer armed for the oldest entry's deadline drives
// eviction, so Insert, Get and Delete never block on background work.
type Cache[K comparable, V any] struct {
	table *swiss.Table[K, V]
	options[K, V]

	timer *time.Timer
	stop  chan struct{}

	// eviction list: firstEntry is the newest, lastEntry the oldest and
	// therefore the next to expire; swiss.NoIndex when the list is empty.
	firstEntry int32
	lastEntry  int32

	me sync.Mutex
}

// NewCache creates a new cache instance. The default TTL is 30 minutes.
// Call Stop when the cache is no longer needed to release the eviction
// goroutine.
func NewCache[K comparable, V any](opts ...Option[K, V]) *Cache[K, V] {
	return newCache(maphash.MakeSeed(), opts...)
}

// newCache creates a cache whose table hashes with the given seed.
// ShardedCache shares one seed across its shards so a key is hashed once.
func newCache[K comparable, V any](seed maphash.Seed, opts ...Option[K, V]) *Cache[K, V] {
	c := &Cache[K, V]{
		table:   swiss.NewSeededTable[K, V](seed),
		options: defaultOptions[K, V](),
		stop:    make(chan struct{}),
		// the zero value 0 is a valid arena index, not an empty list
		firstEntry: swiss.NoIndex,
		lastEntry:  swiss.NoIndex,
	}

	for _, o := range opts {
		o(&c.options)
	}

	if c.ttl <= 0 {
		// entries never expire: no timer, no eviction goroutine, and
		// nothing to touch on hit
		c.noTTL = true
		c.disableTouchOnHit = true
	} else {
		c.timer = time.NewTimer(year)
		go c.evictionWorker()
	}

	return c
}

func defaultOptions[K comparable, V any]() options[K, V] {
	return options[K, V]{
		ttl: 30 * time.Minute,
	}
}

// Insert adds a key/value pair to the cache. If the key already exists, the
// value is overwritten and the expiration is refreshed.
func (c *Cache[K, V]) Insert(key K, value V) {
	c.insertHashed(c.table.Hash(key), key, value)
}

// insertHashed is Insert for a key whose hash is already computed.
func (c *Cache[K, V]) insertHashed(hash uint64, key K, value V) {
	// reading the clock before taking the lock keeps the critical section
	// short; under contention the deadline lands marginally earlier, which
	// only makes expiry more conservative
	var now int64
	if !c.noTTL {
		now = time.Now().UnixNano()
	}
	c.me.Lock()
	idx, existed := c.table.Insert(hash, key)
	entry := c.table.At(idx)
	entry.Value = value
	switch {
	case c.noTTL:
		// the entry never joins the eviction list; an overwrite keeps
		// the sentinel already in place
		if !existed {
			entry.ExpirationTime = math.MaxInt64
		}
	case existed:
		c.touch(idx, now)
	default:
		c.addToEvictionList(idx, now)
	}
	c.me.Unlock()
}

// Delete removes a key from the cache.
func (c *Cache[K, V]) Delete(key K) {
	c.deleteHashed(c.table.Hash(key), key)
}

// deleteHashed is Delete for a key whose hash is already computed.
func (c *Cache[K, V]) deleteHashed(hash uint64, key K) {
	c.me.Lock()
	if idx := c.table.Delete(hash, key); idx != swiss.NoIndex {
		c.deleteFromEvictionList(idx)
		c.table.Free(idx)
	}
	c.me.Unlock()
}

// Get returns the value stored under the key. Expired items are reported as
// missing even if they are not evicted yet. Unless WithDisableTouchOnHit is
// set, a hit refreshes the item's expiration. On a miss, a loader set via
// WithLoader is invoked outside the lock; see the option for the contract.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	return c.getHashed(c.table.Hash(key), key)
}

// getHashed is Get for a key whose hash is already computed. The hash is
// reused for the insert after a successful load.
func (c *Cache[K, V]) getHashed(hash uint64, key K) (V, bool) {
	value, ok := c.lookupHashed(hash, key)
	if ok || c.loader == nil {
		return value, ok
	}
	value, ok = c.loader(key)
	if !ok {
		var zero V
		return zero, false
	}
	c.insertHashed(hash, key, value)
	return value, true
}

// lookupHashed retrieves the value under the key, touching the entry on a
// hit unless touch-on-hit is disabled.
func (c *Cache[K, V]) lookupHashed(hash uint64, key K) (V, bool) {
	c.me.Lock()
	idx := c.table.Get(hash, key)
	if idx == swiss.NoIndex {
		c.me.Unlock()
		var zero V
		return zero, false
	}
	entry := c.table.At(idx)
	// the clock is read only on a hit; misses skip it entirely
	now := time.Now().UnixNano()
	if entry.ExpirationTime <= now {
		c.me.Unlock()
		var zero V
		return zero, false
	}
	if !c.disableTouchOnHit {
		c.touch(idx, now)
	}
	value := entry.Value
	c.me.Unlock()
	return value, true
}

// GetAndDelete removes the key and returns the value it held, all under one
// lock acquisition. Expired items are removed but reported as missing,
// consistent with Get. The loader is never invoked.
func (c *Cache[K, V]) GetAndDelete(key K) (V, bool) {
	return c.getAndDeleteHashed(c.table.Hash(key), key)
}

// getAndDeleteHashed is GetAndDelete for a key whose hash is already
// computed.
func (c *Cache[K, V]) getAndDeleteHashed(hash uint64, key K) (V, bool) {
	c.me.Lock()
	idx := c.table.Delete(hash, key)
	if idx == swiss.NoIndex {
		c.me.Unlock()
		var zero V
		return zero, false
	}
	entry := c.table.At(idx)
	value := entry.Value
	expired := entry.ExpirationTime <= time.Now().UnixNano()
	c.deleteFromEvictionList(idx)
	c.table.Free(idx)
	c.me.Unlock()
	if expired {
		var zero V
		return zero, false
	}
	return value, true
}

// Len returns the number of items in the cache, including expired but not
// yet evicted ones.
func (c *Cache[K, V]) Len() int {
	c.me.Lock()
	defer c.me.Unlock()
	return c.table.Len()
}

// Stop terminates the eviction goroutine. The cache must not be used after
// Stop has been called. A NoTTL cache has no goroutine, but Stop remains
// safe to call.
func (c *Cache[K, V]) Stop() {
	close(c.stop)
}

// evictionWorker runs until Stop is called, deleting expired entries
// whenever the timer fires.
func (c *Cache[K, V]) evictionWorker() {
	for {
		select {
		case <-c.stop:
			c.timer.Stop()
			return
		case <-c.timer.C:
			c.evictExpired()
		}
	}
}

// evictExpired removes all expired entries and re-arms the timer for the
// next deadline, if any.
func (c *Cache[K, V]) evictExpired() {
	c.me.Lock()
	now := time.Now().UnixNano()
	for c.lastEntry != swiss.NoIndex && c.table.At(c.lastEntry).ExpirationTime <= now {
		oldest := c.lastEntry
		c.table.DeleteIndex(oldest)
		c.deleteFromEvictionList(oldest)
		c.table.Free(oldest)
	}
	// the sweep is the only place the cache shrinks: a Delete-driven
	// rehash would spike latency on the hot path, so explicit-Delete-only
	// and NoTTL caches keep their high-water table size
	c.table.MaybeShrink()
	if c.lastEntry != swiss.NoIndex {
		c.timer.Reset(time.Duration(c.table.At(c.lastEntry).ExpirationTime - now))
	}
	c.me.Unlock()
}

// addToEvictionList puts a new entry at the front (newest end) of the list.
// Must be called with the cache locked.
func (c *Cache[K, V]) addToEvictionList(idx int32, now int64) {
	e := c.table.At(idx)
	e.ExpirationTime = now + int64(c.ttl)
	if c.firstEntry == swiss.NoIndex {
		c.firstEntry, c.lastEntry = idx, idx
		// the list was empty, so the timer is parked far in the future
		c.timer.Reset(c.ttl)
		return
	}
	e.Next = c.firstEntry
	c.table.At(c.firstEntry).Prev = idx
	c.firstEntry = idx
}

// deleteFromEvictionList unlinks an entry from the list.
// Must be called with the cache locked.
func (c *Cache[K, V]) deleteFromEvictionList(idx int32) {
	e := c.table.At(idx)

	if e.Prev != swiss.NoIndex {
		c.table.At(e.Prev).Next = e.Next
	} else if c.firstEntry == idx {
		c.firstEntry = e.Next
	}

	if e.Next != swiss.NoIndex {
		c.table.At(e.Next).Prev = e.Prev
	} else if c.lastEntry == idx {
		c.lastEntry = e.Prev
	}

	e.Next, e.Prev = swiss.NoIndex, swiss.NoIndex
	// no timer reset: if the oldest entry was removed, the timer fires
	// early, finds nothing expired and re-arms for the new deadline
}

// touch refreshes an entry's expiration and moves it to the front (newest
// end) of the list. Must be called with the cache locked.
func (c *Cache[K, V]) touch(idx int32, now int64) {
	e := c.table.At(idx)
	e.ExpirationTime = now + int64(c.ttl)
	if idx == c.firstEntry {
		return
	}

	// e is not the first entry, so Prev is set
	c.table.At(e.Prev).Next = e.Next
	if e.Next != swiss.NoIndex {
		c.table.At(e.Next).Prev = e.Prev
	} else {
		c.lastEntry = e.Prev
	}

	e.Prev = swiss.NoIndex
	e.Next = c.firstEntry
	c.table.At(c.firstEntry).Prev = idx
	c.firstEntry = idx
}
