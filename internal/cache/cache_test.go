package cache

import (
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
			tree := NewCache[string, string](30 * time.Second)
			for i := 0; i < tt.count; i++ {
				name := randomUserName()
				t.Logf("inserting %s\n", name)
				tree.Insert(name, "test")
			}

			printTree(t, tree.Root, "")
			checkEvictionList(t, tree)
		})
	}
}

func TestDelete_Test(t *testing.T) {
	item1 := randomUserName()
	item2 := randomUserName()
	item3 := randomUserName()
	items := []string{item1, item2, item3}
	// add a number of nodes to the tree
	t.Run("test items", func(t *testing.T) {
		tree := NewCache[string, string](time.Second)
		for _, name := range items {
			t.Logf("inserting %s\n", name)
			tree.Insert(name, name)
		}
		time.Sleep(100 * time.Millisecond)
		printTree(t, tree.Root, "")

		// checkEvictionList(t, tree)
		found := tree.Get(item2)
		t.Logf("found item: %v\n", found)
		checkEvictionList(t, tree)

		// delete the second item
		tree.Delete(item2)
		time.Sleep(100 * time.Millisecond)
		printTree(t, tree.Root, "")
		checkEvictionList(t, tree)
		time.Sleep(2 * time.Second)
		printTree(t, tree.Root, "")
		checkEvictionList(t, tree)
	})
}
