package luna

import (
	"sync"
)

type Worker interface {
	Start()
	Stop()
}

type WorkerPool[K Ordered, V Worker] struct {
	Root *Item[K, V]

	me sync.Mutex
}

func NewWorkerPool[K Ordered, V Worker]() *WorkerPool[K, V] {
	return &WorkerPool[K, V]{}
}

func (c *WorkerPool[K, V]) Add(key K, value V) {
	c.me.Lock()
	newItem := &Item[K, V]{
		Key:   key,
		Value: value,
	}
	c.Root = Insert(c.Root, newItem)
	c.me.Unlock()
	newItem.Value.Start()
}

func (c *WorkerPool[K, V]) Delete(key K) {
	c.me.Lock()
	var found *Item[K, V]
	c.Root, found = Delete(c.Root, key)
	c.me.Unlock()
	found.Value.Stop()
}

func (c *WorkerPool[K, V]) Get(key K) *Item[K, V] {
	c.me.Lock()
	itm := Search(c.Root, key)
	c.me.Unlock()
	return itm
}

// Do executes a function on the item with the given key and updates the item with the result of the function.
func (c *WorkerPool[K, V]) Do(key K, f func(*Item[K, V]) *Item[K, V]) (executed bool) {
	c.me.Lock()
	itm := Search(c.Root, key)
	if itm != nil {
		itm = f(itm)
		executed = true
	}
	c.me.Unlock()
	return
}

// Read executes a function on the item with the given key but does not update the item with the result of the function.
func (c *WorkerPool[K, V]) Read(key K, f func(*Item[K, V])) (executed bool) {
	c.me.Lock()
	itm := Search(c.Root, key)
	if itm != nil {
		f(itm)
		executed = true
	}
	c.me.Unlock()
	return
}
