package cache

import (
	"fmt"
	"github.com/kaatinga/luna/internal/item"
)

// printTree prints the tree in a human-readable format as a rotated left tree.
func printTree[K item.Ordered, V any](item *item.Item[K, V], prefix string) {
	// right node first
	if item.Right != nil {
		printTree(item.Right, prefix+"  ")
	} else {
		fmt.Printf("%s\n", prefix+"┏ <nil>")
	}

	// print current node
	fmt.Printf("%s%v\n", prefix, item.Key)

	// left node last
	if item.Left != nil {
		printTree(item.Left, prefix+"  ")
	} else {
		fmt.Printf("%s\n", prefix+"┗ <nil>")
	}
}
