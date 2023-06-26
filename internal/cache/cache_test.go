package cache

import (
	"github.com/kaatinga/luna/internal/item"
	"testing"
	"time"
)

func TestInsert_Test(t *testing.T) {
	type testCases struct {
		name  string
		count int
	}
	tests := []testCases{
		{"3 random nodes", 3},
		{"5 random nodes", 5},
		{"10 random nodes", 10},
		{"20 random nodes", 20},
	}
	for _, tt := range tests {
		// add a number of nodes to the tree
		t.Run(tt.name, func(t *testing.T) {
			tree := NewCache[string, string]()
			for i := 0; i < tt.count; i++ {
				tree.Insert(randomUserName(), "test")
			}

			printTree(tree.Root, "")
			checkEvictionList(t, tree)
		})
	}
}

// checkEvictionList prints the list in a human-readable format.
func checkEvictionList[K item.Ordered, V any](t *testing.T, cache *Cache[K, V]) {
	t.Helper()
	t.Logf("List:\n")
	var previousTime time.Time
	for i := cache.firstItem; i != nil; i = i.NextItem {
		if i.ExpirationTime.After(previousTime) && !previousTime.IsZero() {
			t.Fatalf("previousTime %s is after current %s\n", previousTime.Format(time.RFC3339Nano), i.ExpirationTime.Format(time.RFC3339Nano))
		}
		t.Logf("%v %v %s\n", i.Key, i.Value, i.ExpirationTime.Format(time.RFC3339Nano))
		previousTime = i.ExpirationTime
	}
	t.Logf("\n")
}
