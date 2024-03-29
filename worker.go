package luna

import (
	"sync"
)

// worker is a trait for the workers that can be added to the pool.
type worker interface {
	Start() error
	Stop() error
}

// WorkerPool is a pool of workers.
type WorkerPool[K ordered, V worker] struct {
	Root *Item[K, V]

	me sync.Mutex
}

// NewWorkerPool creates a new instance of WorkerPool.
func NewWorkerPool[K ordered, V worker]() *WorkerPool[K, V] {
	return &WorkerPool[K, V]{}
}

// Add adds a new worker to the pool.
func (c *WorkerPool[K, V]) Add(key K, value V) error {
	c.me.Lock()
	defer c.me.Unlock()
	newItem := &Item[K, V]{
		key:   key,
		value: value,
	}
	c.Root = insertNode(c.Root, newItem)
	if err := newItem.value.Start(); err != nil {
		c.Root, _ = deleteNode(c.Root, key)
		return err
	}

	return nil
}

// AddUnlessExists adds a new worker to the pool if it does not exist.
func (c *WorkerPool[K, V]) AddUnlessExists(key K, value V) (*Item[K, V], error) {
	c.me.Lock()
	defer c.me.Unlock()
	if item := searchNode(c.Root, key); item != nil {
		return item, nil
	}

	newItem := &Item[K, V]{
		key:   key,
		value: value,
	}
	c.Root = insertNode(c.Root, newItem)
	if err := newItem.value.Start(); err != nil {
		c.Root, _ = deleteNode(c.Root, key)
		return nil, err
	}

	return newItem, nil
}

// Delete removes a worker from the pool.
func (c *WorkerPool[K, V]) Delete(key K) error {
	c.me.Lock()
	defer c.me.Unlock()
	var found *Item[K, V]
	c.Root, found = deleteNode(c.Root, key)
	if found != nil {
		return found.value.Stop()
	}

	return nil
}

// Get returns a worker from the pool.
func (c *WorkerPool[K, V]) Get(key K) *Item[K, V] {
	c.me.Lock()
	itm := searchNode(c.Root, key)
	c.me.Unlock()
	return itm
}

// Do executes a function on the item with the given key and updates the item atomically if necessary.
func (c *WorkerPool[K, V]) Do(key K, f func(*Item[K, V])) (executed bool) {
	c.me.Lock()
	itm := searchNode(c.Root, key)
	if itm != nil {
		f(itm)
		executed = true
	}
	c.me.Unlock()
	return
}
