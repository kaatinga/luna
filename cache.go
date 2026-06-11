package luna

import (
	"sync"
	"time"

	"github.com/kaatinga/luna/internal/item"
)

type Cache[K item.Ordered, V any] struct {
	Root *item.Item[K, V]
	*Janitor[K, V]
	options[K, V]

	me sync.Mutex
}

// NewCache creates a new cache instance. The default TTL is 30 minutes.
// Call Stop when the cache is no longer needed to release the eviction
// goroutine.
func NewCache[K item.Ordered, V any](opts ...Option[K, V]) *Cache[K, V] {
	c := &Cache[K, V]{
		Janitor: NewJanitor[K, V](),
		options: defaultOptions[K, V](),
	}

	for _, o := range opts {
		o(&c.options)
	}

	go c.evictionWorker()

	return c
}

func defaultOptions[K item.Ordered, V any]() options[K, V] {
	return options[K, V]{
		ttl: 30 * time.Minute,
	}
}

// Insert adds a key/value pair to the cache. If the key already exists, the
// value is overwritten and the expiration is refreshed.
func (c *Cache[K, V]) Insert(key K, value V) {
	c.me.Lock()
	var node *item.Item[K, V]
	var existed bool
	c.Root, node, existed = item.Insert(c.Root, key, value)
	if existed {
		c.touch(node)
	} else {
		c.addToEvictionList(node)
	}
	c.me.Unlock()
}

// Delete removes a key from the cache.
func (c *Cache[K, V]) Delete(key K) {
	c.me.Lock()
	var deletedItem *item.Item[K, V]
	c.Root, deletedItem = item.Delete(c.Root, key)
	c.deleteFromEvictionList(deletedItem)
	c.me.Unlock()
}

// Get returns the value stored under the key. Expired items are reported as
// missing even if they are not evicted yet. Unless WithDisableTouchOnHit is
// set, a hit refreshes the item's expiration.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.me.Lock()
	itm := item.Search(c.Root, key)
	if itm == nil || itm.ExpirationTime <= time.Now().UnixNano() {
		c.me.Unlock()
		var zero V
		return zero, false
	}
	if !c.disableTouchOnHit {
		c.touch(itm)
	}
	value := itm.Value
	c.me.Unlock()
	return value, true
}

// Stop terminates the eviction goroutine. The cache must not be used after
// Stop has been called.
func (c *Cache[K, V]) Stop() {
	close(c.stop)
}
