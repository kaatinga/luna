package item

// Item is an AVL tree node. It also serves as a node of the intrusive
// doubly-linked eviction list, so one allocation covers both structures.
type Item[K Ordered, V any] struct {
	Key    K
	Left   *Item[K, V]
	Right  *Item[K, V]
	Height int8

	Value V

	// janitor fields

	// ExpirationTime is the eviction deadline in unix nanoseconds.
	ExpirationTime int64

	// eviction list links: NextItem points towards older items,
	// PreviousItem towards newer ones.
	NextItem     *Item[K, V]
	PreviousItem *Item[K, V]
}

// Insert inserts a key/value pair into the tree. If the key already exists,
// the value is overwritten in place and no rebalancing happens.
// It returns the new root, the node holding the key, and whether the key
// already existed.
func Insert[K Ordered, V any](root *Item[K, V], key K, value V) (*Item[K, V], *Item[K, V], bool) {
	if root == nil {
		node := &Item[K, V]{Key: key, Value: value, Height: 1}
		return node, node, false
	}

	if key == root.Key {
		root.Value = value
		return root, root, true
	}

	var node *Item[K, V]
	var existed bool
	if key < root.Key {
		root.Left, node, existed = Insert(root.Left, key, value)
	} else {
		root.Right, node, existed = Insert(root.Right, key, value)
	}
	if existed {
		return root, node, true
	}

	return balanceItem(root), node, false
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

// Search searches for an item in the tree.
func Search[K Ordered, V any](item *Item[K, V], key K) *Item[K, V] {
	for item != nil && item.Key != key {
		if key < item.Key {
			item = item.Left
		} else {
			item = item.Right
		}
	}
	return item
}

// Delete deletes an item from the tree. It returns the new root and the
// deleted node, if any.
func Delete[K Ordered, V any](item *Item[K, V], key K) (*Item[K, V], *Item[K, V]) {
	if item == nil {
		return nil, nil
	}
	var found *Item[K, V]
	if key < item.Key {
		item.Left, found = Delete(item.Left, key)
	} else if key > item.Key {
		item.Right, found = Delete(item.Right, key)
	} else {
		// item found
		left := item.Left
		right := item.Right
		item.Left, item.Right = nil, nil
		if right == nil {
			return left, item
		}
		min := right
		for min.Left != nil {
			min = min.Left
		}
		min.Right = deleteMin(right)
		min.Left = left
		return balanceItem(min), item
	}

	return balanceItem(item), found
}

func deleteMin[K Ordered, V any](item *Item[K, V]) *Item[K, V] {
	if item.Left == nil {
		return item.Right
	}
	item.Left = deleteMin(item.Left)

	return balanceItem(item)
}
