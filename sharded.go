package luna

import (
	"hash/maphash"
)

const (
	shardBits  = 4
	shardCount = 1 << shardBits
)

// ShardedCache splits the keyspace by hash into independent caches, each
// with its own lock, table and eviction list. All shards share one hash
// seed, so a key is hashed exactly once per operation; under concurrent
// load the reduced lock contention wins by a wide margin.
type ShardedCache[K comparable, V any] struct {
	shards [shardCount]*Cache[K, V]
	seed   maphash.Seed
}

// NewShardedCache creates a sharded cache. The default TTL is 30 minutes.
// Call Stop when the cache is no longer needed.
func NewShardedCache[K comparable, V any](opts ...Option[K, V]) *ShardedCache[K, V] {
	c := &ShardedCache[K, V]{seed: maphash.MakeSeed()}
	for i := range c.shards {
		c.shards[i] = newCache(c.seed, opts...)
	}
	return c
}

// shard hashes the key and picks a shard from the hash's high bits; the
// tables consume the low bits (h2 and group), so reusing the same hash for
// both costs no entropy.
func (c *ShardedCache[K, V]) shard(key K) (*Cache[K, V], uint64) {
	hash := maphash.Comparable(c.seed, key)
	return c.shards[hash>>(64-shardBits)], hash
}

// Insert adds a key/value pair to the cache. If the key already exists, the
// value is overwritten and the expiration is refreshed.
func (c *ShardedCache[K, V]) Insert(key K, value V) {
	s, hash := c.shard(key)
	s.insertHashed(hash, key, value)
}

// Delete removes a key from the cache.
func (c *ShardedCache[K, V]) Delete(key K) {
	s, hash := c.shard(key)
	s.deleteHashed(hash, key)
}

// Get returns the value stored under the key. Expired items are reported as
// missing even if they are not evicted yet. Unless WithDisableTouchOnHit is
// set, a hit refreshes the item's expiration.
func (c *ShardedCache[K, V]) Get(key K) (V, bool) {
	s, hash := c.shard(key)
	return s.getHashed(hash, key)
}

// GetAndDelete removes the key and returns the value it held. Expired items
// are removed but reported as missing, consistent with Get. The loader is
// never invoked.
func (c *ShardedCache[K, V]) GetAndDelete(key K) (V, bool) {
	s, hash := c.shard(key)
	return s.getAndDeleteHashed(hash, key)
}

// Len returns the number of items in the cache, including expired but not
// yet evicted ones.
func (c *ShardedCache[K, V]) Len() int {
	var n int
	for _, s := range c.shards {
		n += s.Len()
	}
	return n
}

// Stop terminates the eviction goroutines of all shards.
func (c *ShardedCache[K, V]) Stop() {
	for _, s := range c.shards {
		s.Stop()
	}
}
