package item

import (
	"fmt"
	"log"
	"time"
)

// Item is an AVL tree node
type Item[K Ordered, V any] struct {
	Key            K
	Value          V
	Height         int8
	Left           *Item[K, V]
	Right          *Item[K, V]
	ExpirationTime time.Time

	// linked list for eviction

	NextItem     *Item[K, V]
	PreviousItem *Item[K, V]
}

func Insert[K Ordered, V any](root *Item[K, V], insert *Item[K, V]) *Item[K, V] {
	if root == nil {
		return insert
	}
	if insert.Key < root.Key {
		root.Left = Insert(root.Left, insert)
	} else {
		root.Right = Insert(root.Right, insert)
	}

	return balanceItem(root)
}

func height[K Ordered, V any](item *Item[K, V]) int8 {
	if item == nil {
		return 0
	}
	return item.Height
}

func balance[K Ordered, V any](item *Item[K, V]) int8 {
	if item == nil {
		return 0
	}
	return height(item.Right) - height(item.Left)
}

func max(a, b int8) int8 {
	if a > b {
		return a
	}
	return b
}

func fixHeight[K Ordered, V any](item *Item[K, V]) {
	if item == nil {
		return
	}
	item.Height = max(height(item.Left), height(item.Right)) + 1
}

func rotateRight[K Ordered, V any](item *Item[K, V]) *Item[K, V] {
	// fmt.Printf("rotateRight: %v\n", item.Key)
	left := item.Left
	item.Left = left.Right
	left.Right = item
	fixHeight(item)
	fixHeight(left)
	return left
}

func rotateLeft[K Ordered, V any](item *Item[K, V]) *Item[K, V] {
	// fmt.Printf("rotateLeft: %v\n", item.Key)
	right := item.Right
	item.Right = right.Left
	right.Left = item
	fixHeight(item)
	fixHeight(right)
	return right
}

func balanceItem[K Ordered, V any](item *Item[K, V]) *Item[K, V] {
	fixHeight(item)
	if balance(item) == 2 {
		if balance(item.Right) < 0 {
			item.Right = rotateRight(item.Right)
		}
		return rotateLeft(item)
	}
	if balance(item) == -2 {
		if balance(item.Left) > 0 {
			item.Left = rotateLeft(item.Left)
		}
		return rotateRight(item)
	}
	return item
}

// Search searches for an item in the tree.
func Search[K Ordered, V any](item *Item[K, V], key K) *Item[K, V] {
	// if item != nil {
	// log.Println("searching in", item.Key)
	// }

	if item == nil || item.Key == key {
		// if item != nil && item.Key == key {
		// log.Println("item found", item)
		// }
		return item
	}
	// printItem(item)
	if key < item.Key {
		// log.Println("searching left")
		return Search(item.Left, key)
	}

	// log.Println("searching right")
	return Search(item.Right, key)
}

func printItem[K Ordered, V any](item *Item[K, V]) {
	var left = "<nil>"
	if item.Left != nil {
		left = fmt.Sprint(item.Left.Key)
	}

	var right = "<nil>"
	if item.Right != nil {
		right = fmt.Sprint(item.Right.Key)
	}
	log.Printf("item: %s, left: %s, right: %s\n", item.Key, left, right)
}

// Delete deletes an item from the tree.
func Delete[K Ordered, V any](item *Item[K, V], key K) *Item[K, V] {
	if item == nil {
		return nil
	}
	if key < item.Key {
		item.Left = Delete(item.Left, key)
	} else if key > item.Key {
		item.Right = Delete(item.Right, key)
	} else {
		left := item.Left
		right := item.Right
		if right == nil {
			return left
		}
		min := right
		for min.Left != nil {
			min = min.Left
		}
		min.Right = deleteMin(right)
		min.Left = left
		return balanceItem(min)
	}
	return balanceItem(item)
}

func deleteMin[K Ordered, V any](item *Item[K, V]) *Item[K, V] {
	if item.Left == nil {
		return item.Right
	}
	item.Left = deleteMin(item.Left)
	return balanceItem(item)
}
