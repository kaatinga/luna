package luna

// AVL Tree Implementation
//
// The AVL tree is a self-balancing binary searchNode tree algorithm, ensuring that the tree remains balanced at all times. This implementation is based on the original concept of AVL trees as introduced by Georgy Adelson-Velsky and Evgenii Landis in 1962.
//
// Developers: Georgy Adelson-Velsky and Evgenii Landis
// Year Introduced: 1962
// Publication: "An algorithm for the organization of information."
//
// This code is an implementation of the AVL tree algorithm, adhering to the original principles of maintaining balanced heights to ensure O(log n) time complexity for insertions, deletions, and lookups. Modifications and optimizations may have been applied to adapt to specific use cases.

// Item is an AVL tree node.
type Item[K ordered, V worker] struct {
	key    K
	value  V
	height int8
	left   *Item[K, V]
	right  *Item[K, V]
}

func (i *Item[K, V]) Key() K {
	return i.key
}

func (i *Item[K, V]) Value() V {
	return i.value
}

func insertNode[K ordered, V worker](root *Item[K, V], ins *Item[K, V]) *Item[K, V] {
	if root == nil {
		return ins
	}
	if ins.key < root.key {
		root.left = insertNode(root.left, ins)
	} else {
		root.right = insertNode(root.right, ins)
	}

	return balanceItem(root)
}

func height[K ordered, V worker](item *Item[K, V]) int8 {
	if item == nil {
		return 0
	}
	return item.height
}

func balance[K ordered, V worker](item *Item[K, V]) int8 {
	if item == nil {
		return 0
	}
	return height(item.right) - height(item.left)
}

func fixHeight[K ordered, V worker](item *Item[K, V]) {
	if item == nil {
		return
	}
	item.height = max(height(item.left), height(item.right)) + 1
}

func rotateRight[K ordered, V worker](item *Item[K, V]) *Item[K, V] {
	left := item.left
	item.left = left.right
	left.right = item
	fixHeight(item)
	fixHeight(left)
	return left
}

func rotateLeft[K ordered, V worker](item *Item[K, V]) *Item[K, V] {
	right := item.right
	item.right = right.left
	right.left = item
	fixHeight(item)
	fixHeight(right)
	return right
}

func balanceItem[K ordered, V worker](item *Item[K, V]) *Item[K, V] {
	fixHeight(item)
	switch balance(item) {
	case 2:
		if balance(item.right) < 0 {
			item.right = rotateRight(item.right)
		}
		item = rotateLeft(item)
	case -2:
		if balance(item.left) > 0 {
			item.left = rotateLeft(item.left)
		}
		item = rotateRight(item)
	}

	return item
}

// searchNode searches for an item in the tree.
func searchNode[K ordered, V worker](item *Item[K, V], key K) *Item[K, V] {
	// if item != nil {
	// log.Println("searching in", item.key)
	// }

	if item == nil || item.key == key {
		// if item != nil && item.key == key {
		// log.Println("item found", item)
		// }
		return item
	}
	// printItem(item)
	if key < item.key {
		// log.Println("searching left")
		return searchNode(item.left, key)
	}

	// log.Println("searching right")
	return searchNode(item.right, key)
}

// deleteNode deletes an item from the tree.
func deleteNode[K ordered, V worker](item *Item[K, V], key K) (*Item[K, V], *Item[K, V]) {
	if item == nil {
		return nil, nil
	}
	var found *Item[K, V]
	if key < item.key {
		item.left, found = deleteNode(item.left, key)
	} else if key > item.key {
		item.right, found = deleteNode(item.right, key)
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

func deleteMin[K ordered, V worker](item *Item[K, V]) *Item[K, V] {
	if item.left == nil {
		return item.right
	}
	item.left = deleteMin(item.left)

	return balanceItem(item)
}
