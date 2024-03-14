package luna

import (
	"sync"
)

// workerPool is a trait for the workers that can be added to the pool.
type worker interface {
	Start()
	Stop()
}

type WorkerPool[K Ordered, V worker] struct {
	Root *Item[K, V]

	me sync.Mutex
}

func NewWorkerPool[K Ordered, V worker]() *WorkerPool[K, V] {
	return &WorkerPool[K, V]{}
}

func (c *WorkerPool[K, V]) Add(key K, value V) {
	c.me.Lock()
	newItem := &Item[K, V]{
		Key:   key,
		Value: value,
	}
	c.Root = Insert(c.Root, newItem)
	newItem.Value.Start()
	c.me.Unlock()
}

func (c *WorkerPool[K, V]) Delete(key K) {
	c.me.Lock()
	var found *Item[K, V]
	c.Root, found = Delete(c.Root, key)
	found.Value.Stop()
	c.me.Unlock()
}

func (c *WorkerPool[K, V]) Get(key K) *Item[K, V] {
	c.me.Lock()
	itm := Search(c.Root, key)
	c.me.Unlock()
	return itm
}

// Do executes a function on the item with the given key and updates the item atomically if necessary.
func (c *WorkerPool[K, V]) Do(key K, f func(*Item[K, V])) (executed bool) {
	c.me.Lock()
	itm := Search(c.Root, key)
	if itm != nil {
		f(itm)
		executed = true
	}
	c.me.Unlock()
	return
}
