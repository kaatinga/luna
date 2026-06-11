package luna

import (
	"hash/maphash"

	"github.com/kaatinga/luna/internal/item"
)

// ShardedSwissCache splits the keyspace by hash into independent swiss-table
// caches, each with its own lock, table and eviction list.
type ShardedSwissCache[K item.Ordered, V any] struct {
	shards [shardCount]*SwissCache[K, V]
	seed   maphash.Seed
}

// NewShardedSwissCache creates a sharded swiss-table cache. The default TTL
// is 30 minutes. Call Stop when the cache is no longer needed.
func NewShardedSwissCache[K item.Ordered, V any](opts ...Option[K, V]) *ShardedSwissCache[K, V] {
	c := &ShardedSwissCache[K, V]{seed: maphash.MakeSeed()}
	for i := range c.shards {
		c.shards[i] = NewSwissCache[K, V](opts...)
	}
	return c
}

func (c *ShardedSwissCache[K, V]) shard(key K) *SwissCache[K, V] {
	return c.shards[maphash.Comparable(c.seed, key)&(shardCount-1)]
}

func (c *ShardedSwissCache[K, V]) Insert(key K, value V) {
	c.shard(key).Insert(key, value)
}

func (c *ShardedSwissCache[K, V]) Delete(key K) {
	c.shard(key).Delete(key)
}

func (c *ShardedSwissCache[K, V]) Get(key K) (V, bool) {
	return c.shard(key).Get(key)
}

// Stop terminates the eviction goroutines of all shards.
func (c *ShardedSwissCache[K, V]) Stop() {
	for _, s := range c.shards {
		s.Stop()
	}
}
