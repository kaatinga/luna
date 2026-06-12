package benchmarks

import (
	"math/rand"
	"runtime"
	"strconv"
	"testing"
	"time"

	expirable "github.com/go-pkgz/expirable-cache/v3"
	"github.com/jellydator/ttlcache/v3"
	"github.com/kaatinga/luna"
	gocache "github.com/patrickmn/go-cache"
)

// long TTL so eviction never interferes with the measurements
const ttl = time.Hour

var sizes = []int{1_000, 100_000, 1_000_000}

// memorySizes are the entry counts for the footprint benchmark.
var memorySizes = []int{100_000, 1_000_000}

// benchCache is the common surface all implementations are measured
// through, so every engine pays the same call overhead.
type benchCache interface {
	Insert(key string, value int)
	Get(key string) (int, bool)
	Delete(key string)
	Stop()
}

var impls = []struct {
	name string
	make func() benchCache
}{
	{"luna", func() benchCache {
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
	{"naive-map", func() benchCache { return newNaiveCache() }},
	{"jellydator", func() benchCache {
		// the jellydator janitor (cache.Start) is intentionally not
		// started — luna's janitor goroutines sleep on a timer too
		return jellydatorAdapter{ttlcache.New[string, int](
			ttlcache.WithTTL[string, int](ttl),
			ttlcache.WithDisableTouchOnHit[string, int](),
		)}
	}},
	{"otter", func() benchCache {
		c, err := makeOtter()
		if err != nil {
			panic(err)
		}
		return c
	}},
	{"theine", func() benchCache {
		c, err := makeTheine()
		if err != nil {
			panic(err)
		}
		return c
	}},
	{"expirable", func() benchCache {
		return expirableAdapter{c: expirable.NewCache[string, int]()}
	}},
	{"go-cache", func() benchCache {
		// cleanup interval 0: no background janitor goroutine
		return goCacheAdapter{c: gocache.New(ttl, 0)}
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

func runtimeGC() {
	runtime.GC()
	runtime.GC()
}

func heapInuse() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapInuse
}

// measureRetainedHeap returns incremental heap still held by a live,
// empty cache after filling to len(keys) and deleting every entry.
// GC is deliberately not run after the deletes: map-backed caches keep
// their high-water bucket tables and freed entry objects may not yet be
// returned to the runtime.
func measureRetainedHeap(makeCache func() benchCache, keys []string) uint64 {
	runtimeGC()
	base := heapInuse()

	c := makeCache()
	for j, k := range keys {
		c.Insert(k, j)
	}
	for _, k := range keys {
		c.Delete(k)
	}
	runtime.KeepAlive(c)

	after := heapInuse()
	c.Stop()
	if after < base {
		return 0
	}
	return after - base
}

// measurePeakHeap returns incremental heap held by a full cache.
func measurePeakHeap(makeCache func() benchCache, keys []string) uint64 {
	runtimeGC()
	base := heapInuse()

	c := makeCache()
	for j, k := range keys {
		c.Insert(k, j)
	}
	runtime.KeepAlive(c)

	after := heapInuse()
	c.Stop()
	if after < base {
		return 0
	}
	return after - base
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

// BenchmarkInsertFresh measures a delete-then-insert cycle so every op
// hits an empty slot — the allocation profile of growing a cold cache.
func BenchmarkInsertFresh(b *testing.B) {
	forEach(b, func(b *testing.B, c benchCache, keys []string, n int) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := keys[i%n]
			c.Delete(key)
			c.Insert(key, i)
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

// BenchmarkMemoryAtPeak reports incremental heap while the cache holds all
// entries.
func BenchmarkMemoryAtPeak(b *testing.B) {
	for _, impl := range impls {
		b.Run(impl.name, func(b *testing.B) {
			for _, n := range memorySizes {
				b.Run("n="+strconv.Itoa(n), func(b *testing.B) {
					keys := stringKeys(n)
					b.StopTimer()
					peak := measurePeakHeap(impl.make, keys)
					b.ReportMetric(float64(peak)/(1024*1024), "heap-peak-MiB")
				})
			}
		})
	}
}

// BenchmarkMemoryAfterDelete fills the cache, deletes every entry, and
// reports incremental heap retained by the still-live cache without a
// post-delete GC. Map-backed caches keep their high-water bucket tables
// until the runtime collects them; naive-map also allocates one heap object
// per fresh insert. The workload runs once per sub-benchmark.
func BenchmarkMemoryAfterDelete(b *testing.B) {
	for _, impl := range impls {
		b.Run(impl.name, func(b *testing.B) {
			for _, n := range memorySizes {
				b.Run("n="+strconv.Itoa(n), func(b *testing.B) {
					keys := stringKeys(n)
					b.StopTimer()

					retained := measureRetainedHeap(impl.make, keys)
					b.ReportMetric(float64(retained)/(1024*1024), "heap-retained-MiB")
				})
			}
		})
	}
}
