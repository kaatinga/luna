package luna

import (
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
	// therefore the next to expire.
	firstEntry *swiss.Entry[K, V]
	lastEntry  *swiss.Entry[K, V]

	me sync.Mutex
}

// NewCache creates a new cache instance. The default TTL is 30 minutes.
// Call Stop when the cache is no longer needed to release the eviction
// goroutine.
func NewCache[K comparable, V any](opts ...Option[K, V]) *Cache[K, V] {
	c := &Cache[K, V]{
		table:   swiss.NewTable[K, V](),
		options: defaultOptions[K, V](),
		stop:    make(chan struct{}),
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
	hash := c.table.Hash(key)
	c.me.Lock()
	entry, existed := c.table.Insert(hash, key)
	entry.Value = value
	switch {
	case c.noTTL:
		// the entry never joins the eviction list; an overwrite keeps
		// the sentinel already in place
		if !existed {
			entry.ExpirationTime = math.MaxInt64
		}
	case existed:
		c.touch(entry)
	default:
		c.addToEvictionList(entry)
	}
	c.me.Unlock()
}

// Delete removes a key from the cache.
func (c *Cache[K, V]) Delete(key K) {
	hash := c.table.Hash(key)
	c.me.Lock()
	c.deleteFromEvictionList(c.table.Delete(hash, key))
	c.me.Unlock()
}

// Get returns the value stored under the key. Expired items are reported as
// missing even if they are not evicted yet. Unless WithDisableTouchOnHit is
// set, a hit refreshes the item's expiration. On a miss, a loader set via
// WithLoader is invoked outside the lock; see the option for the contract.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	value, ok := c.lookup(key)
	if ok || c.loader == nil {
		return value, ok
	}
	value, ok = c.loader(key)
	if !ok {
		var zero V
		return zero, false
	}
	c.Insert(key, value)
	return value, true
}

// lookup retrieves the value under the key, touching the entry on a hit
// unless touch-on-hit is disabled.
func (c *Cache[K, V]) lookup(key K) (V, bool) {
	hash := c.table.Hash(key)
	c.me.Lock()
	entry := c.table.Get(hash, key)
	if entry == nil || entry.ExpirationTime <= time.Now().UnixNano() {
		c.me.Unlock()
		var zero V
		return zero, false
	}
	if !c.disableTouchOnHit {
		c.touch(entry)
	}
	value := entry.Value
	c.me.Unlock()
	return value, true
}

// GetAndDelete removes the key and returns the value it held, all under one
// lock acquisition. Expired items are removed but reported as missing,
// consistent with Get. The loader is never invoked.
func (c *Cache[K, V]) GetAndDelete(key K) (V, bool) {
	hash := c.table.Hash(key)
	c.me.Lock()
	entry := c.table.Delete(hash, key)
	if entry == nil {
		c.me.Unlock()
		var zero V
		return zero, false
	}
	c.deleteFromEvictionList(entry)
	value := entry.Value
	expired := entry.ExpirationTime <= time.Now().UnixNano()
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
	for c.lastEntry != nil && c.lastEntry.ExpirationTime <= now {
		oldest := c.lastEntry
		c.table.Delete(c.table.Hash(oldest.Key), oldest.Key)
		c.deleteFromEvictionList(oldest)
	}
	if c.lastEntry != nil {
		c.timer.Reset(time.Duration(c.lastEntry.ExpirationTime - now))
	}
	c.me.Unlock()
}

// addToEvictionList puts a new entry at the front (newest end) of the list.
// Must be called with the cache locked.
func (c *Cache[K, V]) addToEvictionList(e *swiss.Entry[K, V]) {
	e.ExpirationTime = time.Now().UnixNano() + int64(c.ttl)
	if c.firstEntry == nil {
		c.firstEntry, c.lastEntry = e, e
		// the list was empty, so the timer is parked far in the future
		c.timer.Reset(c.ttl)
		return
	}
	e.NextEntry = c.firstEntry
	c.firstEntry.PreviousEntry = e
	c.firstEntry = e
}

// deleteFromEvictionList unlinks an entry from the list.
// Must be called with the cache locked.
func (c *Cache[K, V]) deleteFromEvictionList(e *swiss.Entry[K, V]) {
	if e == nil {
		return
	}

	if e.PreviousEntry != nil {
		e.PreviousEntry.NextEntry = e.NextEntry
	} else if c.firstEntry == e {
		c.firstEntry = e.NextEntry
	}

	if e.NextEntry != nil {
		e.NextEntry.PreviousEntry = e.PreviousEntry
	} else if c.lastEntry == e {
		c.lastEntry = e.PreviousEntry
	}

	e.NextEntry, e.PreviousEntry = nil, nil
	// no timer reset: if the oldest entry was removed, the timer fires
	// early, finds nothing expired and re-arms for the new deadline
}

// touch refreshes an entry's expiration and moves it to the front (newest
// end) of the list. Must be called with the cache locked.
func (c *Cache[K, V]) touch(e *swiss.Entry[K, V]) {
	e.ExpirationTime = time.Now().UnixNano() + int64(c.ttl)
	if e == c.firstEntry {
		return
	}

	// e is not the first entry, so PreviousEntry is set
	e.PreviousEntry.NextEntry = e.NextEntry
	if e.NextEntry != nil {
		e.NextEntry.PreviousEntry = e.PreviousEntry
	} else {
		c.lastEntry = e.PreviousEntry
	}

	e.PreviousEntry = nil
	e.NextEntry = c.firstEntry
	c.firstEntry.PreviousEntry = e
	c.firstEntry = e
}
