package cache

import (
	"fmt"
	"log"
	"time"

	"github.com/kaatinga/luna/internal/item"
)

type Cache[K item.Ordered, V any] struct {
	Root *item.Item[K, V]
	*Janitor[K, V]
	jobs chan *Action[K, V]
	ttl  time.Duration
}

func NewCache[K item.Ordered, V any](ttl time.Duration) *Cache[K, V] {
	c := &Cache[K, V]{
		Janitor: NewJanitor[K, V](),
		jobs:    make(chan *Action[K, V]),
		ttl:     ttl,
	}

	go c.cacheWorker()
	go c.evictionWorker()

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
	action := &Action[K, V]{
		actionType: remove,
		Item: &item.Item[K, V]{
			Key: key,
		},
	}

	c.jobs <- action
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

			if c.firstItem == nil {
				c.lastItem, c.firstItem = action.Item, action.Item
				c.Janitor.reloadEvictionWorker <- struct{}{}
			} else {
				c.firstItem, c.firstItem.PreviousItem, action.Item.NextItem = action.Item, action.Item, c.firstItem
			}
		case remove:
			found := item.Search(c.Root, action.Key)
			if found == nil || found.Key != action.Item.Key {
				log.Println("item not found", action.Key)
				continue
			}

			c.evict(found)
			c.Root = item.Delete(c.Root, action.Key)
		case search:
			found := item.Search(c.Root, action.Key)
			if found == nil || found.Key != action.Item.Key {
				action.output <- nil
				continue
			}

			action.output <- found
		}
	}
}
