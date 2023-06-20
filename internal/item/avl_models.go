package item

// Item is an AVL tree node
type Item[K Ordered, V any] struct {
	Key    K
	Value  V
	Height int
	Left   *Item[K, V]
	Right  *Item[K, V]
}

func height[K Ordered, V any](item *Item[K, V]) int {
	if item == nil {
		return 0
	}
	return item.Height
}

func balance[K Ordered, V any](item *Item[K, V]) int {
	if item == nil {
		return 0
	}
	return height(item.Right) - height(item.Left)
}

func max(a, b int) int {
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
	left := item.Left
	item.Left = left.Right
	left.Right = item
	fixHeight(item)
	fixHeight(left)
	return left
}

func rotateLeft[K Ordered, V any](item *Item[K, V]) *Item[K, V] {
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

// Insert inserts a new item into the tree.
func Insert[K Ordered, V any](item *Item[K, V], key K, value any) *Item[K, V] {
	if item == nil {
		return &Item[K, V]{Key: key, Height: 1, Value: value}
	}
	if key < item.Key {
		item.Left = Insert(item.Left, key, value)
	} else {
		item.Right = Insert(item.Right, key, value)
	}
	return balanceItem(item)
}

// Search searches for an item in the tree.
func Search[K Ordered, V any](item *Item[K, V], key K) *Item[K, V] {
	if item == nil || item.Key == key {
		return item
	}
	if key < item.Key {
		return Search(item.Left, key)
	}
	return Search(item.Right, key)
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
