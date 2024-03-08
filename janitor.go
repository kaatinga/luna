package luna

import (
	"time"

	"github.com/kaatinga/luna/internal/item"
)

type Janitor[K item.Ordered, V any] struct {
	reloadEvictionWorker chan struct{}
	lastItem             *item.Item[K, V]
	firstItem            *item.Item[K, V]
}

func NewJanitor[K item.Ordered, V any]() *Janitor[K, V] {
	return &Janitor[K, V]{
		reloadEvictionWorker: make(chan struct{}, 20),
	}
}

const year = time.Hour * 24 * 365

// evictionWorker
func (c *Cache[K, V]) evictionWorker() {
	for {
		select {
		case <-c.reloadEvictionWorker:
			//fmt.Println("--- evictionWorker: reloadEvictionWorker")
		case <-time.After(time.Until(c.soonestTime())):
			//fmt.Println("--- evictionWorker: time to evict", c.lastItem.Key)
			c.Delete(c.lastItem.Key)
		}
	}
}

func (c *Cache[K, V]) soonestTime() time.Time {
	if c.lastItem == nil {
		return time.Now().Add(year) // set one year if there are no items in the list
	}
	return c.lastItem.ExpirationTime
}

func (c *Cache[K, V]) addToEvictionList(item *item.Item[K, V]) {
	if item == nil {
		//fmt.Println("evictionWorker: no need to update eviction list: item is nil")
		return
	}
	item.ExpirationTime = time.Now().Add(c.ttl)
	if c.lastItem == nil {
		c.lastItem, c.firstItem = item, item
		c.Janitor.reloadEvictionWorker <- struct{}{}
		//fmt.Println("evictionWorker: reloadEvictionWorker sent as the last item is nil")
	} else {
		c.firstItem, c.firstItem.PreviousItem, item.NextItem = item, nil, c.firstItem
		item.NextItem.PreviousItem = item
	}

	//fmt.Printf("item added: %v\n", item.Key)
}

func (c *Cache[K, V]) deleteFromEvictionList(item *item.Item[K, V]) {
	if item == nil {
		//fmt.Println("evictionWorker: no need to update eviction list: item is nil")
		return
	}

	if item == c.firstItem {
		c.firstItem = item.NextItem
	}

	if item == c.lastItem {
		c.lastItem = item.PreviousItem
		c.Janitor.reloadEvictionWorker <- struct{}{}
		//fmt.Println("evictionWorker: reloadEvictionWorker sent as last item was deleted")
	}

	if item.NextItem != nil {
		item.NextItem.PreviousItem = item.PreviousItem
	}

	if item.PreviousItem != nil {
		item.PreviousItem.NextItem = item.NextItem
	}

	//fmt.Printf("item deleted: %v\n", item.Key)
}

func (c *Cache[K, V]) moveToTheEnd(item *item.Item[K, V]) {
	if item == nil {
		//fmt.Println("evictionWorker: no need to update eviction list: item is nil")
		return
	}
	c.firstItem, c.firstItem.PreviousItem, item.NextItem = item, nil, c.firstItem
	item.NextItem.PreviousItem = item

	if item == c.lastItem {
		c.lastItem = item.PreviousItem
		c.lastItem.NextItem = nil
		c.Janitor.reloadEvictionWorker <- struct{}{}
		//fmt.Println("evictionWorker: reloadEvictionWorker sent as last item was touched")
	}
}
