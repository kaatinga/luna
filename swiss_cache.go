package luna

import (
	"sync"
	"time"

	"github.com/kaatinga/luna/internal/item"
	"github.com/kaatinga/luna/internal/swiss"
)

// SwissCache is a TTL cache backed by an open-addressing swiss table
// instead of an AVL tree. Same API, same eviction model.
type SwissCache[K item.Ordered, V any] struct {
	table *swiss.Table[K, V]
	options[K, V]

	timer *time.Timer
	stop  chan struct{}

	// eviction list: firstEntry is the newest, lastEntry the oldest.
	firstEntry *swiss.Entry[K, V]
	lastEntry  *swiss.Entry[K, V]

	me sync.Mutex
}

// NewSwissCache creates a new swiss-table cache. The default TTL is
// 30 minutes. Call Stop when the cache is no longer needed.
func NewSwissCache[K item.Ordered, V any](opts ...Option[K, V]) *SwissCache[K, V] {
	c := &SwissCache[K, V]{
		table:   swiss.NewTable[K, V](),
		options: defaultOptions[K, V](),
		timer:   time.NewTimer(year),
		stop:    make(chan struct{}),
	}

	for _, o := range opts {
		o(&c.options)
	}

	go c.evictionWorker()

	return c
}

func (c *SwissCache[K, V]) Insert(key K, value V) {
	hash := c.table.Hash(key)
	c.me.Lock()
	entry, existed := c.table.Insert(hash, key)
	entry.Value = value
	if existed {
		c.touch(entry)
	} else {
		c.addToEvictionList(entry)
	}
	c.me.Unlock()
}

func (c *SwissCache[K, V]) Delete(key K) {
	hash := c.table.Hash(key)
	c.me.Lock()
	c.deleteFromEvictionList(c.table.Delete(hash, key))
	c.me.Unlock()
}

func (c *SwissCache[K, V]) Get(key K) (V, bool) {
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

// Stop terminates the eviction goroutine. The cache must not be used after
// Stop has been called.
func (c *SwissCache[K, V]) Stop() {
	close(c.stop)
}

func (c *SwissCache[K, V]) evictionWorker() {
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

func (c *SwissCache[K, V]) evictExpired() {
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

func (c *SwissCache[K, V]) addToEvictionList(e *swiss.Entry[K, V]) {
	e.ExpirationTime = time.Now().UnixNano() + int64(c.ttl)
	if c.firstEntry == nil {
		c.firstEntry, c.lastEntry = e, e
		c.timer.Reset(c.ttl)
		return
	}
	e.NextEntry = c.firstEntry
	c.firstEntry.PreviousEntry = e
	c.firstEntry = e
}

func (c *SwissCache[K, V]) deleteFromEvictionList(e *swiss.Entry[K, V]) {
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
}

func (c *SwissCache[K, V]) touch(e *swiss.Entry[K, V]) {
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
