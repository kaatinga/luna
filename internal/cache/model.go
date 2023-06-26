package cache

import (
	"fmt"
	"github.com/kaatinga/luna/internal/item"
	"time"
)

const defaultTTL = time.Minute * 30

type Cache[K item.Ordered, V any] struct {
	Root *item.Item[K, V]
	*Janitor[K, V]
	jobs chan *Action[K, V]
	ttl  time.Duration
}

func NewCache[K item.Ordered, V any]() *Cache[K, V] {
	c := &Cache[K, V]{
		Janitor: NewJanitor[K, V](),
		jobs:    make(chan *Action[K, V]),
		ttl:     defaultTTL,
	}

	go c.cacheWorker()
	go c.evictor()

	return c
}

func (c *Cache[K, V]) Insert(key K, value V) {
	action := &Action[K, V]{
		actionType: insert,
		Item: &item.Item[K, V]{
			Key:            key,
			Value:          value,
			ExpirationTime: time.Now().Add(c.ttl),
		},
	}

	c.jobs <- action
}

func (c *Cache[K, V]) Delete(key K) {
	c.Root = item.Delete(c.Root, key)
}

func (c *Cache[K, V]) Get(key K) *item.Item[K, V] {
	action := &Action[K, V]{
		actionType: search,
		Item: &item.Item[K, V]{
			Key: key,
		},
		output: make(chan *item.Item[K, V]),
	}

	c.jobs <- action

	return <-action.output
}

// cacheWorker is a worker that updates the tree
func (c *Cache[K, V]) cacheWorker() {
	fmt.Println("main worker started")

	for action := range c.jobs {
		// jobs the item in the tree
		switch action.actionType {
		case insert:
			c.Root = item.Insert(c.Root, action.Item)
		case remove:
			c.Root = item.Delete(c.Root, action.Key)
		case search:
			found := item.Search(c.Root, action.Key)
			if found.Key != action.Item.Key {
				action.output <- nil
				continue
			}
			action.output <- found
			action.Item = found
		}

		c.Janitor.update <- action
	}
}
