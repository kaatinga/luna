package cache

import (
	"fmt"
	"github.com/kaatinga/luna/internal/item"
	"log"
	"time"
)

type Janitor[K item.Ordered, V any] struct {
	update    chan *Action[K, V]
	lastItem  *item.Item[K, V]
	firstItem *item.Item[K, V]
}

func NewJanitor[K item.Ordered, V any]() *Janitor[K, V] {
	return &Janitor[K, V]{
		update: make(chan *Action[K, V]),
	}
}

const year = time.Hour * 24 * 365

func (c *Cache[K, V]) evictor() {
	fmt.Println("evictor started")
	// default time is in one year
	var soonestTime = time.Now().Add(year)
	for {
		select {
		case action := <-c.Janitor.update:
			switch action.actionType {
			case insert:
				action.Item.NextItem, c.Janitor.firstItem = c.Janitor.firstItem, action.Item
				c.Janitor.firstItem = action.Item
			case search:
				// delete item from the linked list
				c.deleteItem(action.Item)

				// move the item back to the front of the linked list
				action.Item.NextItem, action.Item.PreviousItem = c.Janitor.firstItem, nil
				c.Janitor.firstItem = action.Item
			case remove:
				// delete item from the linked list
				c.deleteItem(action.Item)
			default:
				log.Println("unknown action type in the janitor")
			}

			if c.lastItem == nil {
				soonestTime = time.Now().Add(year) // set one year if there are no items in the list
			} else {
				soonestTime = c.lastItem.ExpirationTime
			}

		case <-time.After(time.Until(soonestTime)):
			// delete action from the tree
			c.Root = item.Delete(c.Root, c.Janitor.firstItem.Key)
		}
	}
}

func (c *Cache[K, V]) deleteItem(item *item.Item[K, V]) {
	if item.NextItem != nil {
		item.NextItem.PreviousItem = item.PreviousItem
	}

	if item.PreviousItem != nil {
		item.PreviousItem.NextItem = item.NextItem
	}

	// if the item is the first item, update the first item
	if c.firstItem == item {
		c.firstItem = item.NextItem
	}

	// if the item is the last item, update the last item
	if c.lastItem == item {
		c.lastItem = item.PreviousItem
	}
}
