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
	// fmt.Println("evict started")

	// default time is in one year
	var soonestTime = time.Now().Add(year)

	for {
		select {
		case <-c.reloadEvictionWorker:
			//if c.lastItem != nil {
			//	log.Println("--- evictionWorker: reloadEvictionWorker, soonest item:", c.lastItem.Key)
			//} else {
			//	log.Println("--- evictionWorker: reloadEvictionWorker, soonest item: not present")
			//}
			if c.lastItem == nil {
				soonestTime = time.Now().Add(year) // set one year if there are no items in the list
			} else {
				soonestTime = c.lastItem.ExpirationTime
			}
		case <-time.After(time.Until(soonestTime)):
			//log.Println("--- evictionWorker: time to evict", c.lastItem.Key)
			// delete action from the tree
			soonestTime = time.Now().Add(year)
			c.Delete(c.lastItem.Key)
		}
	}
}

func (c *Cache[K, V]) addToEvictionList(item *item.Item[K, V]) {
	if item == nil {
		// log.Println("evictionWorker: no need to update eviction list: item is nil")
		return
	}
	item.ExpirationTime = time.Now().Add(c.ttl)
	if c.firstItem == nil {
		c.lastItem, c.firstItem = item, item
		c.Janitor.reloadEvictionWorker <- struct{}{}
	} else {
		c.firstItem, c.firstItem.PreviousItem, item.NextItem = item, item, c.firstItem
	}
}

func (c *Cache[K, V]) deleteFromEvictionList(found *item.Item[K, V]) {
	if found == nil {
		// log.Println("evictionWorker: no need to update eviction list: found is nil")
		return
	}
	// var next string
	// if found.NextItem != nil {
	// 	next = fmt.Sprint(found.NextItem.Key)
	// } else {
	// 	next = "<nil>"
	// }

	// var previous string
	// if found.PreviousItem != nil {
	// 	previous = fmt.Sprint(found.PreviousItem.Key)
	// } else {
	// 	previous = "<nil>"
	// }
	// log.Printf("evicting %v, next: %v, previous: %v", found.Key, next, previous)
	if found.NextItem != nil {
		found.NextItem.PreviousItem = found.PreviousItem
	}

	if found.PreviousItem != nil {
		found.PreviousItem.NextItem = found.NextItem
	}

	// if the item is the first item, update the first item
	if c.firstItem == found {
		c.firstItem = found.NextItem
	}

	// if the item is the last item, update the last item
	if c.lastItem == found {
		c.lastItem = found.PreviousItem
		if c.lastItem != nil {
			// log.Println("eviction list must be reloaded as c.lastItem = found, key:", found.Key)
			c.Janitor.reloadEvictionWorker <- struct{}{}
		}
	}

	// clean up bonds in case the item is reinserted
	found.PreviousItem, found.NextItem = nil, nil
}
