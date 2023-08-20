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
			tree.checkEvictionList(t, false)
		})
	}
}

func TestDelete_Test(t *testing.T) {
	item1 := randomUserName()
	item2 := randomUserName()
	item3 := randomUserName()
	items := []string{item1, item2, item3}
	tree := NewCache[string, string](300 * time.Millisecond)
	// add a number of nodes to the tree
	t.Run("delete item in the center", func(t *testing.T) {
		for _, name := range items {
			t.Logf("inserting %s\n", name)
			tree.Insert(name, name)
		}
		time.Sleep(tree.ttl / 10)
		printTree(t, tree.Root, "")
		tree.checkEvictionList(t, false)
		found := tree.Get(item2)
		if found == nil {
			t.Fatalf("item %s was not found\n", item2)
		}
		time.Sleep(tree.ttl / 10)
		printTree(t, tree.Root, "")
		tree.checkEvictionList(t, false)

		// delete the second item
		tree.Delete(item2)
		time.Sleep(tree.ttl / 10)
		printTree(t, tree.Root, "")
		tree.checkEvictionList(t, false)
		time.Sleep(tree.ttl)
		printTree(t, tree.Root, "")
		tree.checkEvictionList(t, true)
	})
	t.Run("delete item in the end", func(t *testing.T) {
		for _, name := range items {
			t.Logf("inserting %s\n", name)
			tree.Insert(name, name)
		}
		tree.Delete(item1)
		time.Sleep(tree.ttl / 10)
		printTree(t, tree.Root, "")
		tree.checkEvictionList(t, false)

		time.Sleep(tree.ttl)
		found := tree.Get(item2)
		if found != nil {
			t.Fatalf("item %s was found\n", item2)
		}
	})

	t.Run("addToEvictionList is empty after passing ttl", func(t *testing.T) {
		for _, name := range items {
			t.Logf("inserting %s\n", name)
			tree.Insert(name, name)
		}

		time.Sleep(tree.ttl * 2)
		for _, name := range items {
			found := tree.Get(name)
			if found != nil {
				t.Fatalf("item %s was found\n", name)
			}
		}
	})

	t.Run("addToEvictionList is empty after all the items are deleted", func(t *testing.T) {
		for _, name := range items {
			t.Logf("inserting %s\n", name)
			tree.Insert(name, name)
		}

		for _, name := range items {
			tree.Delete(name)
		}
		for _, name := range items {
			found := tree.Get(name)
			if found != nil {
				t.Fatalf("item %s was found\n", name)
			}
		}
	})

	t.Run("last item in the addToEvictionList", func(t *testing.T) {
		for _, name := range items {
			t.Logf("inserting %s\n", name)
			tree.Insert(name, name)
		}

		for _, name := range items[1:] {
			tree.Delete(name)
		}
		found := tree.Get(items[0])
		if found == nil {
			t.Fatalf("item %s was not found\n", items[0])
		}

		time.Sleep(tree.ttl / 10)
		printTree(t, tree.Root, "")
		tree.checkEvictionList(t, false)
		for _, name := range items {
			tree.Delete(name)
		}
	})

	t.Run("delete first and last", func(t *testing.T) {
		for _, name := range items {
			t.Logf("inserting %s\n", name)
			tree.Insert(name, name)
		}

		tree.Delete(items[0])
		tree.Delete(items[2])
		found := tree.Get(items[1])
		if found == nil {
			t.Fatalf("item %s was not found\n", items[1])
		}

		time.Sleep(tree.ttl / 10)
		printTree(t, tree.Root, "")
		tree.checkEvictionList(t, false)
		for _, name := range items {
			tree.Delete(name)
		}
	})

	t.Run("get item, check the first item", func(t *testing.T) {
		for _, name := range items {
			t.Logf("inserting %s\n", name)
			tree.Insert(name, name)
		}

		found := tree.Get(items[1])
		if found == nil {
			t.Fatalf("item %s was not found\n", items[1])
		}

		if tree.firstItem.Key != items[1] {
			t.Fatalf("first item is not %s\n", items[1])
		}

		time.Sleep(tree.ttl / 10)
		printTree(t, tree.Root, "")
		tree.checkEvictionList(t, false)

		found = tree.Get(items[2])
		if found == nil {
			t.Fatalf("item %s was not found\n", items[2])
		}

		if tree.firstItem.Key != items[2] {
			t.Fatalf("first item is not %s\n", items[2])
		}
		time.Sleep(tree.ttl / 10)
		printTree(t, tree.Root, "")
		tree.checkEvictionList(t, false)
	})
}
