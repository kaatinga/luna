package benchmarks

import (
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/kaatinga/luna"
)

// long TTL so eviction never interferes with the measurements
const ttl = time.Hour

var sizes = []int{1_000, 100_000, 1_000_000}

// benchCache is the common surface all implementations are measured
// through, so every engine pays the same call overhead.
type benchCache interface {
	Insert(key string, value int)
	Get(key string) (int, bool)
	Delete(key string)
	Stop()
}

type jellydatorAdapter struct {
	c *ttlcache.Cache[string, int]
}

func (a jellydatorAdapter) Insert(key string, value int) { a.c.Set(key, value, ttlcache.DefaultTTL) }
func (a jellydatorAdapter) Get(key string) (int, bool) {
	item := a.c.Get(key)
	if item == nil {
		return 0, false
	}
	return item.Value(), true
}
func (a jellydatorAdapter) Delete(key string) { a.c.Delete(key) }
func (a jellydatorAdapter) Stop()             {}

var impls = []struct {
	name string
	make func() benchCache
}{
	{"luna-avl", func() benchCache {
		return luna.NewCache[string, int](
			luna.WithTTL[string, int](ttl),
			luna.WithDisableTouchOnHit[string, int](),
		)
	}},
	{"luna-sharded", func() benchCache {
		return luna.NewShardedCache[string, int](
			luna.WithTTL[string, int](ttl),
			luna.WithDisableTouchOnHit[string, int](),
		)
	}},
	{"luna-swiss", func() benchCache {
		return luna.NewSwissCache[string, int](
			luna.WithTTL[string, int](ttl),
			luna.WithDisableTouchOnHit[string, int](),
		)
	}},
	{"luna-sharded-swiss", func() benchCache {
		return luna.NewShardedSwissCache[string, int](
			luna.WithTTL[string, int](ttl),
			luna.WithDisableTouchOnHit[string, int](),
		)
	}},
	{"jellydator", func() benchCache {
		// the jellydator janitor (cache.Start) is intentionally not
		// started — luna's janitor goroutines sleep on a timer too
		return jellydatorAdapter{ttlcache.New[string, int](
			ttlcache.WithTTL[string, int](ttl),
			ttlcache.WithDisableTouchOnHit[string, int](),
		)}
	}},
}

func stringKeys(n int) []string {
	rng := rand.New(rand.NewSource(42))
	keys := make([]string, n)
	for i := range keys {
		keys[i] = strconv.Itoa(rng.Int())
	}
	return keys
}

func missKeys(n int) []string {
	keys := make([]string, n)
	for i := range keys {
		// stringKeys produces non-negative ints, so these never collide
		keys[i] = "-" + strconv.Itoa(i+1)
	}
	return keys
}

// forEach runs fn for every implementation and size.
func forEach(b *testing.B, fn func(b *testing.B, c benchCache, keys []string, n int)) {
	b.Helper()
	for _, impl := range impls {
		b.Run(impl.name, func(b *testing.B) {
			for _, n := range sizes {
				b.Run("n="+strconv.Itoa(n), func(b *testing.B) {
					c := impl.make()
					defer c.Stop()
					fn(b, c, stringKeys(n), n)
				})
			}
		})
	}
}

func BenchmarkGetHit(b *testing.B) {
	forEach(b, func(b *testing.B, c benchCache, keys []string, n int) {
		for i, k := range keys {
			c.Insert(k, i)
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			c.Get(keys[i%n])
		}
	})
}

func BenchmarkGetMiss(b *testing.B) {
	forEach(b, func(b *testing.B, c benchCache, keys []string, n int) {
		miss := missKeys(n)
		for i, k := range keys {
			c.Insert(k, i)
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			c.Get(miss[i%n])
		}
	})
}

func BenchmarkInsert(b *testing.B) {
	forEach(b, func(b *testing.B, c benchCache, keys []string, n int) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			c.Insert(keys[i%n], i)
		}
	})
}

func BenchmarkInsertExisting(b *testing.B) {
	forEach(b, func(b *testing.B, c benchCache, keys []string, n int) {
		for i, k := range keys {
			c.Insert(k, i)
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			c.Insert(keys[i%n], i)
		}
	})
}

func BenchmarkDelete(b *testing.B) {
	forEach(b, func(b *testing.B, c benchCache, keys []string, n int) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if i%n == 0 {
				b.StopTimer()
				for j, k := range keys {
					c.Insert(k, j)
				}
				b.StartTimer()
			}
			c.Delete(keys[i%n])
		}
	})
}

// BenchmarkMixedParallel runs 90% Get / 5% Insert / 5% Delete over a fixed
// keyspace from all available cores. This is where locking strategy shows.
func BenchmarkMixedParallel(b *testing.B) {
	forEach(b, func(b *testing.B, c benchCache, keys []string, n int) {
		for i, k := range keys {
			c.Insert(k, i)
		}
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			rng := rand.New(rand.NewSource(rand.Int63()))
			i := 0
			for pb.Next() {
				key := keys[rng.Intn(n)]
				switch i % 20 {
				case 18:
					c.Insert(key, i)
				case 19:
					c.Delete(key)
				default:
					c.Get(key)
				}
				i++
			}
		})
	})
}
