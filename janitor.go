package luna

import (
	"time"

	"github.com/kaatinga/luna/internal/item"
)

// Janitor evicts expired items. A single timer is armed for the oldest
// item's deadline; inserts and deletes re-arm it only when the deadline
// moves, so the hot path never blocks on channels.
type Janitor[K item.Ordered, V any] struct {
	timer *time.Timer
	stop  chan struct{}

	// eviction list: firstItem is the newest, lastItem is the oldest and
	// therefore the next to expire.
	firstItem *item.Item[K, V]
	lastItem  *item.Item[K, V]
}

const year = time.Hour * 24 * 365

func NewJanitor[K item.Ordered, V any]() *Janitor[K, V] {
	return &Janitor[K, V]{
		timer: time.NewTimer(year),
		stop:  make(chan struct{}),
	}
}

// evictionWorker runs until Stop is called, deleting expired items whenever
// the timer fires.
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

// evictExpired removes all expired items and re-arms the timer for the next
// deadline, if any.
func (c *Cache[K, V]) evictExpired() {
	c.me.Lock()
	now := time.Now().UnixNano()
	for c.lastItem != nil && c.lastItem.ExpirationTime <= now {
		oldest := c.lastItem
		c.Root, _ = item.Delete(c.Root, oldest.Key)
		c.deleteFromEvictionList(oldest)
	}
	if c.lastItem != nil {
		c.timer.Reset(time.Duration(c.lastItem.ExpirationTime - now))
	}
	c.me.Unlock()
}

// addToEvictionList puts a new item at the front (newest end) of the list.
// Must be called with the cache locked.
func (c *Cache[K, V]) addToEvictionList(itm *item.Item[K, V]) {
	itm.ExpirationTime = time.Now().UnixNano() + int64(c.ttl)
	if c.firstItem == nil {
		c.firstItem, c.lastItem = itm, itm
		// the list was empty, so the timer is parked far in the future
		c.timer.Reset(c.ttl)
		return
	}
	itm.NextItem = c.firstItem
	c.firstItem.PreviousItem = itm
	c.firstItem = itm
}

// deleteFromEvictionList unlinks an item from the list.
// Must be called with the cache locked.
func (c *Cache[K, V]) deleteFromEvictionList(itm *item.Item[K, V]) {
	if itm == nil {
		return
	}

	if itm.PreviousItem != nil {
		itm.PreviousItem.NextItem = itm.NextItem
	} else if c.firstItem == itm {
		c.firstItem = itm.NextItem
	}

	if itm.NextItem != nil {
		itm.NextItem.PreviousItem = itm.PreviousItem
	} else if c.lastItem == itm {
		c.lastItem = itm.PreviousItem
	}

	itm.NextItem, itm.PreviousItem = nil, nil
	// no timer reset: if the oldest item was removed, the timer fires
	// early, finds nothing expired and re-arms for the new deadline
}

// touch refreshes an item's expiration and moves it to the front (newest
// end) of the list. Must be called with the cache locked.
func (c *Cache[K, V]) touch(itm *item.Item[K, V]) {
	itm.ExpirationTime = time.Now().UnixNano() + int64(c.ttl)
	if itm == c.firstItem {
		return
	}

	// itm is not the first item, so PreviousItem is set
	itm.PreviousItem.NextItem = itm.NextItem
	if itm.NextItem != nil {
		itm.NextItem.PreviousItem = itm.PreviousItem
	} else {
		c.lastItem = itm.PreviousItem
	}

	itm.PreviousItem = nil
	itm.NextItem = c.firstItem
	c.firstItem.PreviousItem = itm
	c.firstItem = itm
}
