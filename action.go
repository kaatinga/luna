package luna

import "github.com/kaatinga/luna/internal/item"

type actionType byte

const (
	insert actionType = iota
	remove
	search
)

type Action[K item.Ordered, V any] struct {
	actionType
	*item.Item[K, V]
	output chan *item.Item[K, V]
}
