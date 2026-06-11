package luna

import (
	"strconv"
	"sync"
	"testing"
	"time"
)

// ttlCache is the common surface of both cache types, for tests.
type ttlCache interface {
	Insert(key string, value int)
	Get(key string) (int, bool)
	Delete(key string)
	Len() int
	Stop()
}

func variants(ttl time.Duration) map[string]ttlCache {
	return map[string]ttlCache{
		"cache":   NewCache[string, int](WithTTL[string, int](ttl)),
		"sharded": NewShardedCache[string, int](WithTTL[string, int](ttl)),
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

	if mustBeEmpty && (c.firstEntry != nil || c.lastEntry != nil) {
		t.Fatalf("eviction list must be empty, first: %v, last: %v", c.firstEntry, c.lastEntry)
	}
	if !mustBeEmpty && (c.firstEntry == nil || c.lastEntry == nil) {
		t.Fatal("eviction list must not be empty")
	}

	var previousTime int64
	count := 0
	for e := c.firstEntry; e != nil; e = e.NextEntry {
		if previousTime != 0 && e.ExpirationTime > previousTime {
			t.Fatalf("entry %v expires after its newer neighbour", e.Key)
		}
		previousTime = e.ExpirationTime
		if e.NextEntry == nil && e != c.lastEntry {
			t.Fatalf("list tail %v is not lastEntry %v", e.Key, c.lastEntry.Key)
		}
		if e.NextEntry != nil && e.NextEntry.PreviousEntry != e {
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
