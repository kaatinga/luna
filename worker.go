package luna

import (
	"sync"

	"github.com/kaatinga/luna/internal/item"
)

type Worker interface {
	Start()
	Stop()
}

type WorkerPool[K item.Ordered, V Worker] struct {
	Root *item.Item[K, V]

	me sync.Mutex
}

func NewWorkerPool[K item.Ordered, V Worker]() *WorkerPool[K, V] {
	return &WorkerPool[K, V]{}
}

func (c *WorkerPool[K, V]) Add(key K, value V) {
	c.me.Lock()
	newItem := &item.Item[K, V]{
		Key:   key,
		Value: value,
	}
	c.Root = item.Insert(c.Root, newItem)
	c.me.Unlock()
	newItem.Value.Start()
}

func (c *WorkerPool[K, V]) Delete(key K) {
	c.me.Lock()
	var found *item.Item[K, V]
	c.Root, found = item.Delete(c.Root, key)
	c.me.Unlock()
	found.Value.Stop()
}

func (c *WorkerPool[K, V]) Get(key K) *item.Item[K, V] {
	c.me.Lock()
	itm := item.Search(c.Root, key)
	c.me.Unlock()
	return itm
}

func (c *WorkerPool[K, V]) Do(key K, f func(*item.Item[K, V]) *item.Item[K, V]) (executed bool) {
	c.me.Lock()
	itm := item.Search(c.Root, key)
	if itm != nil {
		itm = f(itm)
		executed = true
	}
	c.me.Unlock()
	return
}
