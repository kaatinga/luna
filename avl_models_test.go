package luna

import (
	"testing"
)

type dummyWorker bool

func (v dummyWorker) Start() error {
	return nil
}

func (v dummyWorker) Stop() error {
	return nil
}

// Test helper functions
func checkBalanceFactor[K ordered, V worker](t *testing.T, item *Item[K, V]) {
	if item == nil {
		return
	}
	bal := balance(item)
	if bal < -1 || bal > 1 {
		t.Errorf("Balance factor of node %v is %d, which is outside the range [-1, 1]", item.Key, bal)
	}
	checkBalanceFactor(t, item.left)
	checkBalanceFactor(t, item.right)
}

func TestInsertions(t *testing.T) {
	var tree = new(Item[int, dummyWorker])
	items := []int{3, 2, 1, 4, 5}

	for _, key := range items {
		item := &Item[int, dummyWorker]{Key: key, Value: dummyWorker(true)}
		tree = Insert(tree, item)
		checkBalanceFactor(t, tree)
	}
}

func TestDeletions(t *testing.T) {
	var tree = new(Item[int, dummyWorker])
	for _, key := range []int{5, 2, 1, 4, 3, 6} {
		tree = Insert(tree, &Item[int, dummyWorker]{Key: key, Value: true})
	}

	keysToDelete := []int{1, 6}
	for _, key := range keysToDelete {
		var found *Item[int, dummyWorker]
		tree, found = Delete(tree, key)
		if found == nil {
			t.Errorf("Did not find item with key %d to delete", key)
		} else {
			checkBalanceFactor(t, tree)
		}
	}
}
