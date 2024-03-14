package luna

// Item is an AVL tree node.
type Item[K Ordered, V worker] struct {
	Key    K
	Value  V
	height int8
	left   *Item[K, V]
	right  *Item[K, V]
}

func Insert[K Ordered, V worker](root *Item[K, V], insert *Item[K, V]) *Item[K, V] {
	if root == nil {
		return insert
	}
	if insert.Key < root.Key {
		root.left = Insert(root.left, insert)
	} else {
		root.right = Insert(root.right, insert)
	}

	return balanceItem(root)
}

func height[K Ordered, V worker](item *Item[K, V]) int8 {
	if item == nil {
		return 0
	}
	return item.height
}

func balance[K Ordered, V worker](item *Item[K, V]) int8 {
	if item == nil {
		return 0
	}
	return height(item.right) - height(item.left)
}

func fixHeight[K Ordered, V worker](item *Item[K, V]) {
	if item == nil {
		return
	}
	item.height = max(height(item.left), height(item.right)) + 1
}

func rotateRight[K Ordered, V worker](item *Item[K, V]) *Item[K, V] {
	// fmt.Printf("rotateRight: %v\n", item.Key)
	left := item.left
	item.left = left.right
	left.right = item
	fixHeight(item)
	fixHeight(left)
	return left
}

func rotateLeft[K Ordered, V worker](item *Item[K, V]) *Item[K, V] {
	// fmt.Printf("rotateLeft: %v\n", item.Key)
	right := item.right
	item.right = right.left
	right.left = item
	fixHeight(item)
	fixHeight(right)
	return right
}

func balanceItem[K Ordered, V worker](item *Item[K, V]) *Item[K, V] {
	fixHeight(item)
	if balance(item) == 2 {
		if balance(item.right) < 0 {
			item.right = rotateRight(item.right)
		}
		return rotateLeft(item)
	}
	if balance(item) == -2 {
		if balance(item.left) > 0 {
			item.left = rotateLeft(item.left)
		}
		return rotateRight(item)
	}
	return item
}

// Search searches for an item in the tree.
func Search[K Ordered, V worker](item *Item[K, V], key K) *Item[K, V] {
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
		return Search(item.left, key)
	}

	// log.Println("searching right")
	return Search(item.right, key)
}

// Delete deletes an item from the tree.
func Delete[K Ordered, V worker](item *Item[K, V], key K) (*Item[K, V], *Item[K, V]) {
	if item == nil {
		return nil, nil
	}
	var found *Item[K, V]
	if key < item.Key {
		item.left, found = Delete(item.left, key)
	} else if key > item.Key {
		item.right, found = Delete(item.right, key)
	} else {
		// item found
		left := item.left
		right := item.right
		if right == nil {
			return left, item
		}
		minItem := right
		for minItem.left != nil {
			minItem = minItem.left
		}
		minItem.right = deleteMin(right)
		minItem.left = left
		return balanceItem(minItem), item
	}

	return balanceItem(item), found
}

func deleteMin[K Ordered, V worker](item *Item[K, V]) *Item[K, V] {
	if item.left == nil {
		return item.right
	}
	item.left = deleteMin(item.left)

	return balanceItem(item)
}
