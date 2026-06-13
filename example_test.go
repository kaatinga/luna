package luna_test

import (
	"fmt"
	"time"

	"github.com/kaatinga/luna"
)

// The basic flow: create a cache with a TTL, insert, read back, delete.
// Always Stop a TTL cache when you are done so the eviction goroutine exits.
func Example() {
	cache := luna.NewCache[string, int](
		luna.WithTTL[string, int](time.Minute),
	)
	defer cache.Stop()

	cache.Insert("answer", 42)

	if v, ok := cache.Get("answer"); ok {
		fmt.Println(v)
	}
	// Output: 42
}

// A loader turns the cache into a read-through cache: Get calls it on a miss
// and caches the result. Return false to report a miss without caching.
func ExampleWithLoader() {
	calls := 0
	cache := luna.NewCache[string, int](
		luna.WithTTL[string, int](time.Minute),
		luna.WithLoader[string, int](func(key string) (int, bool) {
			calls++
			return len(key), true // pretend this is an expensive lookup
		}),
	)
	defer cache.Stop()

	v1, _ := cache.Get("hello") // miss -> loads and caches
	v2, _ := cache.Get("hello") // hit  -> served from cache

	fmt.Println(v1, v2, "loader calls:", calls)
	// Output: 5 5 loader calls: 1
}

// GetAndDelete fetches and removes an entry under a single lock acquisition,
// so a value can be consumed exactly once — handy for secrets or PRG-style
// one-time form state. It never calls the loader.
func ExampleCache_GetAndDelete() {
	cache := luna.NewCache[string, string](
		luna.WithTTL[string, string](time.Minute),
	)
	defer cache.Stop()

	cache.Insert("token", "s3cret")

	first, ok1 := cache.GetAndDelete("token")
	_, ok2 := cache.GetAndDelete("token") // already gone

	fmt.Println(first, ok1, ok2)
	// Output: s3cret true false
}

// Passing luna.NoTTL keeps entries forever: no timer and no background
// goroutine are created. Stop is still safe to call.
func ExampleNoTTL() {
	cache := luna.NewCache[string, string](
		luna.WithTTL[string, string](luna.NoTTL),
	)
	defer cache.Stop()

	cache.Insert("k", "v")
	v, ok := cache.Get("k")

	fmt.Println(v, ok)
	// Output: v true
}

// ShardedCache has the same API as Cache but spreads keys across 16
// independent shards, dividing lock contention for heavily concurrent use.
func ExampleShardedCache() {
	cache := luna.NewShardedCache[string, int](
		luna.WithTTL[string, int](time.Minute),
	)
	defer cache.Stop()

	cache.Insert("requests", 1)
	v, _ := cache.Get("requests")

	fmt.Println(v)
	// Output: 1
}
