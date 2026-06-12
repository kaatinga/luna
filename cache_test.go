package luna

import (
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kaatinga/luna/internal/swiss"
)

// ttlCache is the common surface of both cache types, for tests.
type ttlCache interface {
	Insert(key string, value int)
	Get(key string) (int, bool)
	GetAndDelete(key string) (int, bool)
	Delete(key string)
	Len() int
	Stop()
}

func variants(ttl time.Duration, opts ...Option[string, int]) map[string]ttlCache {
	opts = append([]Option[string, int]{WithTTL[string, int](ttl)}, opts...)
	return map[string]ttlCache{
		"cache":   NewCache[string, int](opts...),
		"sharded": NewShardedCache[string, int](opts...),
	}
}

func TestBasic_Test(t *testing.T) {
	for name, cache := range variants(time.Hour) {
		t.Run(name, func(t *testing.T) {
			defer cache.Stop()

			const n = 10_000
			for i := 0; i < n; i++ {
				cache.Insert(strconv.Itoa(i), i)
			}
			if got := cache.Len(); got != n {
				t.Fatalf("Len() = %d, want %d", got, n)
			}
			for i := 0; i < n; i++ {
				v, ok := cache.Get(strconv.Itoa(i))
				if !ok || v != i {
					t.Fatalf("key %d: got %v (ok=%v), want %d", i, v, ok, i)
				}
			}

			// overwrite must not duplicate
			cache.Insert("42", 100500)
			if v, _ := cache.Get("42"); v != 100500 {
				t.Fatalf("overwrite failed, got %v", v)
			}
			if got := cache.Len(); got != n {
				t.Fatalf("Len() = %d after overwrite, want %d", got, n)
			}
			cache.Delete("42")
			if _, ok := cache.Get("42"); ok {
				t.Fatal("key 42 still present after delete")
			}

			for i := 0; i < n; i++ {
				cache.Delete(strconv.Itoa(i))
			}
			if got := cache.Len(); got != 0 {
				t.Fatalf("Len() = %d after deleting everything", got)
			}
		})
	}
}

func TestExpiration_Test(t *testing.T) {
	for name, cache := range variants(20 * time.Millisecond) {
		t.Run(name, func(t *testing.T) {
			defer cache.Stop()

			for i := 0; i < 100; i++ {
				cache.Insert(strconv.Itoa(i), i)
			}
			if _, ok := cache.Get("0"); !ok {
				t.Fatal("fresh item not found")
			}
			time.Sleep(60 * time.Millisecond)
			for i := 0; i < 100; i++ {
				if _, ok := cache.Get(strconv.Itoa(i)); ok {
					t.Fatalf("key %d survived expiration", i)
				}
			}
			// the janitor must actually evict, not just hide expired items
			if got := cache.Len(); got != 0 {
				t.Fatalf("Len() = %d after expiration, want 0", got)
			}
		})
	}
}

func TestTouchOnHit_Test(t *testing.T) {
	cache := NewCache[string, int](WithTTL[string, int](60 * time.Millisecond))
	defer cache.Stop()

	cache.Insert("touched", 1)
	cache.Insert("ignored", 2)

	// keep touching one key past the original deadline
	for i := 0; i < 6; i++ {
		time.Sleep(20 * time.Millisecond)
		if _, ok := cache.Get("touched"); !ok {
			t.Fatal("touched item expired despite hits")
		}
	}

	if _, ok := cache.Get("ignored"); ok {
		t.Fatal("untouched item should have expired")
	}
}

func TestEvictionListOrder_Test(t *testing.T) {
	cache := NewCache[string, int](WithTTL[string, int](time.Hour))
	defer cache.Stop()

	for i := 0; i < 100; i++ {
		cache.Insert(strconv.Itoa(i), i)
	}
	// touch some keys to move them to the front
	cache.Get("5")
	cache.Get("50")

	cache.checkEvictionList(t, false)

	for i := 0; i < 100; i++ {
		cache.Delete(strconv.Itoa(i))
	}
	cache.checkEvictionList(t, true)
}

// checkEvictionList verifies list integrity: no cycles, consistent links,
// non-increasing expiration times from newest to oldest.
func (c *Cache[K, V]) checkEvictionList(t *testing.T, mustBeEmpty bool) {
	t.Helper()
	c.me.Lock()
	defer c.me.Unlock()

	if mustBeEmpty && (c.firstEntry != swiss.NoIndex || c.lastEntry != swiss.NoIndex) {
		t.Fatalf("eviction list must be empty, first: %v, last: %v", c.firstEntry, c.lastEntry)
	}
	if !mustBeEmpty && (c.firstEntry == swiss.NoIndex || c.lastEntry == swiss.NoIndex) {
		t.Fatal("eviction list must not be empty")
	}

	var previousTime int64
	count := 0
	for idx := c.firstEntry; idx != swiss.NoIndex; idx = c.table.At(idx).Next {
		e := c.table.At(idx)
		if previousTime != 0 && e.ExpirationTime > previousTime {
			t.Fatalf("entry %v expires after its newer neighbour", e.Key)
		}
		previousTime = e.ExpirationTime
		if e.Next == swiss.NoIndex && idx != c.lastEntry {
			t.Fatalf("list tail %v is not lastEntry %v", e.Key, c.table.At(c.lastEntry).Key)
		}
		if e.Next != swiss.NoIndex && c.table.At(e.Next).Prev != idx {
			t.Fatalf("broken back-link at %v", e.Key)
		}
		if count++; count > c.table.Len() {
			t.Fatal("eviction list is longer than the table — cycle?")
		}
	}
}

func TestHammer_Test(t *testing.T) {
	for name, cache := range variants(10 * time.Millisecond) {
		t.Run(name, func(t *testing.T) {
			defer cache.Stop()

			keys := make([]string, 100)
			for i := range keys {
				keys[i] = randomUserName()
			}

			var wg sync.WaitGroup
			for g := 0; g < 4; g++ {
				wg.Add(3)
				go func(seed int) {
					defer wg.Done()
					for i := 0; i < 5000; i++ {
						cache.Insert(keys[(i+seed)%len(keys)], i)
					}
				}(g)
				go func(seed int) {
					defer wg.Done()
					for i := 0; i < 5000; i++ {
						cache.Get(keys[(i+seed*7)%len(keys)])
					}
				}(g)
				go func(seed int) {
					defer wg.Done()
					for i := 0; i < 5000; i++ {
						cache.Delete(keys[(i+seed*13)%len(keys)])
					}
				}(g)
			}
			wg.Wait()

			// let the janitor evict the leftovers
			time.Sleep(50 * time.Millisecond)
			if got := cache.Len(); got != 0 {
				t.Fatalf("Len() = %d after hammer and expiration, want 0", got)
			}
		})
	}
}

// TestChurn_Test exercises tombstone accumulation and purge rehashes:
// repeated insert/delete cycles over a small keyspace must not corrupt the
// table or grow it unboundedly.
func TestChurn_Test(t *testing.T) {
	cache := NewCache[string, int](WithTTL[string, int](time.Hour))
	defer cache.Stop()

	keys := make([]string, 64)
	for i := range keys {
		keys[i] = strconv.Itoa(i)
	}

	for round := 0; round < 10_000; round++ {
		k := keys[round%len(keys)]
		cache.Insert(k, round)
		if v, ok := cache.Get(k); !ok || v != round {
			t.Fatalf("round %d: got %v (ok=%v)", round, v, ok)
		}
		cache.Delete(k)
		if _, ok := cache.Get(k); ok {
			t.Fatalf("round %d: key %s survived delete", round, k)
		}
	}
}

func TestGetAndDelete_Test(t *testing.T) {
	for name, cache := range variants(time.Hour) {
		t.Run(name, func(t *testing.T) {
			defer cache.Stop()

			cache.Insert("once", 7)
			if v, ok := cache.GetAndDelete("once"); !ok || v != 7 {
				t.Fatalf("GetAndDelete = %v (ok=%v), want 7", v, ok)
			}
			if got := cache.Len(); got != 0 {
				t.Fatalf("Len() = %d after GetAndDelete, want 0", got)
			}
			if _, ok := cache.GetAndDelete("once"); ok {
				t.Fatal("second GetAndDelete must miss")
			}
			if _, ok := cache.GetAndDelete("never"); ok {
				t.Fatal("GetAndDelete of absent key must miss")
			}
		})
	}

	// expired-but-not-yet-evicted entries are removed but reported missing
	t.Run("expired", func(t *testing.T) {
		cache := NewCache[string, int](WithTTL[string, int](time.Hour))
		defer cache.Stop()

		cache.Insert("stale", 1)
		cache.me.Lock()
		cache.table.At(cache.table.Get(cache.table.Hash("stale"), "stale")).ExpirationTime = time.Now().UnixNano() - 1
		cache.me.Unlock()

		if _, ok := cache.GetAndDelete("stale"); ok {
			t.Fatal("expired entry must be reported as missing")
		}
		if got := cache.Len(); got != 0 {
			t.Fatalf("Len() = %d, expired entry must be physically removed", got)
		}
	})
}

func TestLoader_Test(t *testing.T) {
	var calls atomic.Int64
	loader := func(key string) (int, bool) {
		calls.Add(1)
		if key == "reject" {
			return 0, false
		}
		return len(key), true
	}

	for name, cache := range variants(time.Hour, WithLoader[string, int](loader)) {
		t.Run(name, func(t *testing.T) {
			defer cache.Stop()
			calls.Store(0)

			if v, ok := cache.Get("loaded"); !ok || v != 6 {
				t.Fatalf("Get = %v (ok=%v), want loader value 6", v, ok)
			}
			if got := calls.Load(); got != 1 {
				t.Fatalf("loader called %d times, want 1", got)
			}
			// second Get is a cache hit
			if v, ok := cache.Get("loaded"); !ok || v != 6 {
				t.Fatalf("Get after load = %v (ok=%v)", v, ok)
			}
			if got := calls.Load(); got != 1 {
				t.Fatalf("loader called %d times after hit, want 1", got)
			}

			// loader miss: nothing cached, no negative caching
			if _, ok := cache.Get("reject"); ok {
				t.Fatal("rejected key must miss")
			}
			if got := cache.Len(); got != 1 {
				t.Fatalf("Len() = %d, loader miss must not insert", got)
			}
			if _, ok := cache.Get("reject"); ok {
				t.Fatal("rejected key must miss again")
			}
			if got := calls.Load(); got != 3 {
				t.Fatalf("loader called %d times, want 3 (no negative caching)", got)
			}

			// GetAndDelete never invokes the loader
			if _, ok := cache.GetAndDelete("absent"); ok {
				t.Fatal("GetAndDelete must not load")
			}
			if got := calls.Load(); got != 3 {
				t.Fatalf("loader called %d times after GetAndDelete, want 3", got)
			}
		})
	}
}

func TestLoaderConcurrent_Test(t *testing.T) {
	var calls atomic.Int64
	loader := func(key string) (int, bool) {
		calls.Add(1)
		time.Sleep(10 * time.Millisecond)
		return 42, true
	}

	for name, cache := range variants(time.Hour, WithLoader[string, int](loader)) {
		t.Run(name, func(t *testing.T) {
			defer cache.Stop()
			calls.Store(0)

			var wg sync.WaitGroup
			for g := 0; g < 16; g++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if v, ok := cache.Get("cold"); !ok || v != 42 {
						t.Errorf("Get = %v (ok=%v), want 42", v, ok)
					}
				}()
			}
			wg.Wait()

			// duplicate loads are the documented contract; at least one
			// must have happened and the key must now be cached
			if calls.Load() < 1 {
				t.Fatal("loader never called")
			}
			before := calls.Load()
			if _, ok := cache.Get("cold"); !ok {
				t.Fatal("key not cached after concurrent loads")
			}
			if calls.Load() != before {
				t.Fatal("warm Get must not call the loader")
			}
		})
	}
}

// TestLoaderUsesCache_Test proves the loader runs outside the lock: a
// loader that calls back into the cache must not deadlock.
func TestLoaderUsesCache_Test(t *testing.T) {
	var cache *Cache[string, int]
	cache = NewCache[string, int](
		WithTTL[string, int](time.Hour),
		WithLoader[string, int](func(key string) (int, bool) {
			v, _ := cache.Get("seed")
			return v + 1, true
		}),
	)
	defer cache.Stop()

	cache.Insert("seed", 10)
	if v, ok := cache.Get("derived"); !ok || v != 11 {
		t.Fatalf("Get = %v (ok=%v), want 11", v, ok)
	}
}

func TestNoTTL_Test(t *testing.T) {
	for name, cache := range variants(NoTTL) {
		t.Run(name, func(t *testing.T) {
			for i := 0; i < 100; i++ {
				cache.Insert(strconv.Itoa(i), i)
			}
			time.Sleep(50 * time.Millisecond)
			for i := 0; i < 100; i++ {
				if v, ok := cache.Get(strconv.Itoa(i)); !ok || v != i {
					t.Fatalf("key %d: got %v (ok=%v), NoTTL entry must survive", i, v, ok)
				}
			}
			if got := cache.Len(); got != 100 {
				t.Fatalf("Len() = %d, want 100", got)
			}

			// overwrite and delete paths must work without an eviction list
			cache.Insert("42", 100500)
			if v, _ := cache.Get("42"); v != 100500 {
				t.Fatalf("overwrite failed, got %v", v)
			}
			cache.Delete("0")
			if _, ok := cache.Get("0"); ok {
				t.Fatal("key 0 survived delete")
			}
			if v, ok := cache.GetAndDelete("1"); !ok || v != 1 {
				t.Fatalf("GetAndDelete = %v (ok=%v), want 1", v, ok)
			}

			done := make(chan struct{})
			go func() {
				cache.Stop()
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("Stop hung on a NoTTL cache")
			}
		})
	}

	// white-box: NoTTL entries never join the eviction list
	t.Run("no eviction list", func(t *testing.T) {
		cache := NewCache[string, int](WithTTL[string, int](NoTTL))
		defer cache.Stop()

		cache.Insert("a", 1)
		cache.Insert("a", 2)
		cache.Get("a")

		cache.me.Lock()
		first, last := cache.firstEntry, cache.lastEntry
		cache.me.Unlock()
		if first != swiss.NoIndex || last != swiss.NoIndex {
			t.Fatalf("eviction list not empty: first=%v last=%v", first, last)
		}
	})
}

func TestNoTTLHammer_Test(t *testing.T) {
	for name, cache := range variants(NoTTL) {
		t.Run(name, func(t *testing.T) {
			defer cache.Stop()

			keys := make([]string, 100)
			for i := range keys {
				keys[i] = randomUserName()
			}

			var wg sync.WaitGroup
			for g := 0; g < 4; g++ {
				wg.Add(4)
				go func(seed int) {
					defer wg.Done()
					for i := 0; i < 5000; i++ {
						cache.Insert(keys[(i+seed)%len(keys)], i)
					}
				}(g)
				go func(seed int) {
					defer wg.Done()
					for i := 0; i < 5000; i++ {
						cache.Get(keys[(i+seed*7)%len(keys)])
					}
				}(g)
				go func(seed int) {
					defer wg.Done()
					for i := 0; i < 5000; i++ {
						cache.Delete(keys[(i+seed*13)%len(keys)])
					}
				}(g)
				go func(seed int) {
					defer wg.Done()
					for i := 0; i < 5000; i++ {
						cache.GetAndDelete(keys[(i+seed*17)%len(keys)])
					}
				}(g)
			}
			wg.Wait()
		})
	}
}

// TestStructKeys_Test verifies that any comparable type works as a key.
func TestStructKeys_Test(t *testing.T) {
	type point struct{ X, Y int }

	cache := NewCache[point, string](WithTTL[point, string](time.Hour))
	defer cache.Stop()

	cache.Insert(point{1, 2}, "a")
	cache.Insert(point{3, 4}, "b")

	if v, ok := cache.Get(point{1, 2}); !ok || v != "a" {
		t.Fatalf("struct key lookup failed: %v (ok=%v)", v, ok)
	}
	if _, ok := cache.Get(point{5, 6}); ok {
		t.Fatal("absent struct key found")
	}
}

// TestZeroAllocSteadyState asserts the documented allocation behavior: no
// allocations on hits, misses, overwrites and delete/re-insert cycles (the
// arena recycles freed entries).
func TestZeroAllocSteadyState_Test(t *testing.T) {
	cache := NewCache[string, int]()
	defer cache.Stop()
	cache.Insert("hot", 1)
	cache.Insert("churn", 1) // warm the arena's free list
	cache.Delete("churn")

	for name, op := range map[string]func(){
		"get hit":   func() { cache.Get("hot") },
		"get miss":  func() { cache.Get("absent") },
		"overwrite": func() { cache.Insert("hot", 2) },
		"delete and re-insert": func() {
			cache.Insert("churn", 1)
			cache.Delete("churn")
		},
	} {
		if avg := testing.AllocsPerRun(1000, op); avg != 0 {
			t.Errorf("%s: %v allocs/op, want 0", name, avg)
		}
	}
}
