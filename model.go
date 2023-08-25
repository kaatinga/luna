package luna

import (
	"github.com/kaatinga/luna/internal/item"
)

type Cache[K item.Ordered, V any] struct {
	Root *item.Item[K, V]
	*Janitor[K, V]
	jobs chan *Action[K, V]
	options[K, V]
}

func NewCache[K item.Ordered, V any](opts ...Option[K, V]) *Cache[K, V] {
	c := &Cache[K, V]{
		Janitor: NewJanitor[K, V](),
		jobs:    make(chan *Action[K, V], 100),
	}

	for _, o := range opts {
		o(&c.options)
	}

	go c.cacheWorker()
	go c.evictionWorker()

	return c
}

func (c *Cache[K, V]) Insert(key K, value V) {
	action := &Action[K, V]{
		actionType: insert,
		Item: &item.Item[K, V]{
			Key:   key,
			Value: value,
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
	// fmt.Println("main worker started")

	for action := range c.jobs {
		// jobs the item in the tree
		switch action.actionType {
		case insert:
			c.Root = item.Insert(c.Root, action.Item)
			c.addToEvictionList(action.Item)
		case remove:
			var deletedItem *item.Item[K, V]
			c.Root, deletedItem = item.Delete(c.Root, action.Key)
			c.deleteFromEvictionList(deletedItem)
		case search:
			found := item.Search(c.Root, action.Key)
			c.deleteFromEvictionList(found)
			c.addToEvictionList(found)
			action.output <- found
		}
	}
}
