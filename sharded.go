package luna

import (
	"hash/maphash"

	"github.com/kaatinga/luna/internal/item"
)

const shardCount = 16 // power of two

// ShardedCache splits the keyspace by hash into independent caches, each
// with its own lock, tree and eviction list — "lock only the part of the
// tree that needs locking", done safely.
type ShardedCache[K item.Ordered, V any] struct {
	shards [shardCount]*Cache[K, V]
	seed   maphash.Seed
}

// NewShardedCache creates a sharded cache. The default TTL is 30 minutes.
// Call Stop when the cache is no longer needed.
func NewShardedCache[K item.Ordered, V any](opts ...Option[K, V]) *ShardedCache[K, V] {
	c := &ShardedCache[K, V]{seed: maphash.MakeSeed()}
	for i := range c.shards {
		c.shards[i] = NewCache[K, V](opts...)
	}
	return c
}

func (c *ShardedCache[K, V]) shard(key K) *Cache[K, V] {
	return c.shards[maphash.Comparable(c.seed, key)&(shardCount-1)]
}

func (c *ShardedCache[K, V]) Insert(key K, value V) {
	c.shard(key).Insert(key, value)
}

func (c *ShardedCache[K, V]) Delete(key K) {
	c.shard(key).Delete(key)
}

func (c *ShardedCache[K, V]) Get(key K) (V, bool) {
	return c.shard(key).Get(key)
}

// Stop terminates the eviction goroutines of all shards.
func (c *ShardedCache[K, V]) Stop() {
	for _, s := range c.shards {
		s.Stop()
	}
}
