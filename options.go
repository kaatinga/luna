package luna

import (
	"time"
)

// NoTTL, passed to WithTTL, makes the cache keep entries forever. No
// eviction goroutine or timer is started for such a cache.
const NoTTL time.Duration = 0

type options[K comparable, V any] struct {
	ttl               time.Duration
	disableTouchOnHit bool
	loader            func(key K) (V, bool)
	initialSize       int  // expected entry count; presizes table and arena
	noTTL             bool // derived from ttl by NewCache, not set by options
}

type Option[K comparable, V any] func(*options[K, V])

// WithTTL sets the TTL of the cache. A non-positive ttl (see NoTTL) makes
// entries never expire and skips the eviction goroutine entirely.
func WithTTL[K comparable, V any](ttl time.Duration) Option[K, V] {
	return func(opts *options[K, V]) {
		opts.ttl = ttl
	}
}

// WithDisableTouchOnHit prevents the cache instance from
// extending/touching an item's expiration timestamp when it is being
// retrieved.
func WithDisableTouchOnHit[K comparable, V any]() Option[K, V] {
	return func(opts *options[K, V]) {
		opts.disableTouchOnHit = true
	}
}

// WithLoader sets a loader that Get calls on a miss. If the loader returns
// true, the value is inserted into the cache and returned to the caller;
// on false nothing is cached and Get reports a miss. The loader runs
// outside the cache lock, so a slow load never blocks other operations and
// the loader may itself use the cache. Consequently, concurrent Gets of
// the same cold key may each invoke the loader; the last result wins. The
// loader must be safe for concurrent use. GetAndDelete never invokes it.
func WithLoader[K comparable, V any](loader func(key K) (V, bool)) Option[K, V] {
	return func(opts *options[K, V]) {
		opts.loader = loader
	}
}

// WithInitialSize presizes the cache for an expected number of live entries,
// so filling up to n inserts triggers no table rehash and no arena
// reallocation — every insert during the fill is allocation-free, not just
// steady-state churn. n is a hint for the high-water mark, not a cap: the
// cache still grows past it on demand. A non-positive n is ignored.
//
// For ShardedCache, n is the total across all shards; it is divided evenly
// between them.
func WithInitialSize[K comparable, V any](n int) Option[K, V] {
	return func(opts *options[K, V]) {
		if n > 0 {
			opts.initialSize = n
		}
	}
}
