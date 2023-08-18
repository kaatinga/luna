package cache

import (
	"fmt"
	"testing"
	"time"

	"github.com/kaatinga/luna/internal/item"
)

// printTree prints the tree in a human-readable format as a rotated left tree.
func printTree[K item.Ordered, V any](t *testing.T, item *item.Item[K, V], prefix string) {
	t.Helper()
	if item == nil {
		t.Log("tree is empty")
		return
	}
	// right node first
	if item.Right != nil {
		printTree(t, item.Right, prefix+"  ")
	} else {
		t.Logf("%s\n", prefix+"┏ <nil>")
	}

	// print current node
	t.Logf("%s%v\n", prefix, item.Key)

	// left node last
	if item.Left != nil {
		printTree(t, item.Left, prefix+"  ")
	} else {
		t.Logf("%s\n", prefix+"┗ <nil>")
	}
}

// checkEvictionList prints the list in a human-readable format.
func checkEvictionList[K item.Ordered, V any](t *testing.T, cache *Cache[K, V]) {
	t.Helper()
	if cache.lastItem == nil {
		t.Log("eviction list is empty")
		return
	}
	t.Logf("List:\n")
	var previousTime time.Time
	for i := cache.firstItem; i != nil; i = i.NextItem {
		if i.ExpirationTime.After(previousTime) && !previousTime.IsZero() {
			t.Fatalf("previousTime %s is after current %s\n", previousTime.Format(time.RFC3339Nano), i.ExpirationTime.Format(time.RFC3339Nano))
		}
		next := "<nil>"
		if i.NextItem != nil {
			next = fmt.Sprint(i.NextItem.Key)
		}
		previous := "<nil>"
		if i.PreviousItem != nil {
			previous = fmt.Sprint(i.PreviousItem.Key)
		}
		t.Logf("%v, next: %v, prev: %s, %s\n", i.Key, next, previous, i.ExpirationTime.Format(time.RFC3339Nano))
		previousTime = i.ExpirationTime
	}
	t.Logf("\n")
}
