package luna

import (
	"github.com/kaatinga/luna/internal/item"
	"sync"
	"time"
)

type Cache[K item.Ordered, V any] struct {
	Root *item.Item[K, V]
	*Janitor[K, V]
	//jobs chan *Action[K, V]
	options[K, V]

	me sync.Mutex
}

// NewCache creates a new cache instance. The default TTL is 30 minutes.
func NewCache[K item.Ordered, V any](opts ...Option[K, V]) *Cache[K, V] {
	c := &Cache[K, V]{
		//jobs:    make(chan *Action[K, V], 100),
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

func (c *Cache[K, V]) Insert(key K, value V) {
	c.me.Lock()
	itm := &item.Item[K, V]{
		Key:   key,
		Value: value,
	}
	c.Root = item.Insert(c.Root, itm)
	c.addToEvictionList(itm)
	c.me.Unlock()
}

func (c *Cache[K, V]) Delete(key K) {
	c.me.Lock()
	var deletedItem *item.Item[K, V]
	c.Root, deletedItem = item.Delete(c.Root, key)
	c.deleteFromEvictionList(deletedItem)
	c.me.Unlock()
}

func (c *Cache[K, V]) Get(key K) *item.Item[K, V] {
	c.me.Lock()
	itm := item.Search(c.Root, key)
	//c.moveToTheEnd(itm)
	c.me.Unlock()
	return itm
}
