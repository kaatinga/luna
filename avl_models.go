package luna

// Item is an AVL tree node
type Item[K Ordered, V any] struct {
	Key    K
	Value  V
	Height int8
	Left   *Item[K, V]
	Right  *Item[K, V]
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

// Delete deletes an item from the tree.
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
