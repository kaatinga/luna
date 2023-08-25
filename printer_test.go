package luna

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
func (c *Cache[K, V]) checkEvictionList(t *testing.T, mustBeEmpty bool) {
	t.Helper()
	if c.lastItem == nil {
		t.Log("eviction list is empty")
		return
	}
	t.Logf("List:\n")
	var previousTime time.Time
	for i := c.firstItem; i != nil; i = i.NextItem {
		if i.ExpirationTime.After(previousTime) && !previousTime.IsZero() {
			t.Fatalf("previousTime %s is after current %s\n", previousTime.Format(time.RFC3339Nano), i.ExpirationTime.Format(time.RFC3339Nano))
		}
		next := "<nil>"
		if i.NextItem != nil {
			next = fmt.Sprint(i.NextItem.Key)
			if i.NextItem == i {
				t.Fatalf("item %v points to itself\n", i.Key)
			}
		}
		previous := "<nil>"
		if i.PreviousItem != nil {
			previous = fmt.Sprint(i.PreviousItem.Key)
		}
		t.Logf("%v, next: %v, prev: %s, %s\n", i.Key, next, previous, i.ExpirationTime.Format(time.RFC3339Nano))
		previousTime = i.ExpirationTime
	}

	var first string
	if c.firstItem != nil {
		first = fmt.Sprint(c.firstItem.Key)
	} else {
		first = "<nil>"
	}
	var last string
	if c.lastItem != nil {
		last = fmt.Sprint(c.lastItem.Key)
	} else {
		last = "<nil>"
	}
	t.Logf("first: %v, last: %v\n", first, last)
	if mustBeEmpty && (c.firstItem != nil || c.lastItem != nil) {
		t.Fatalf("eviction list must be empty, but first item is %v\n", c.firstItem.Key)
	}
	if !mustBeEmpty && (c.firstItem == nil || c.lastItem == nil) {
		t.Fatalf("eviction list must not be empty")
	}
	t.Logf("\n")
}
