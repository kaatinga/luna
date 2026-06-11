package luna

import (
	"strconv"
	"sync"
	"testing"
	"time"
)

// ttlCache is the common surface of all cache variants, for tests.
type ttlCache interface {
	Insert(key string, value int)
	Get(key string) (int, bool)
	Delete(key string)
	Stop()
}

func variants(ttl time.Duration) map[string]ttlCache {
	return map[string]ttlCache{
		"avl":     NewCache[string, int](WithTTL[string, int](ttl)),
		"sharded": NewShardedCache[string, int](WithTTL[string, int](ttl)),
		"swiss":   NewSwissCache[string, int](WithTTL[string, int](ttl)),
	}
}

func TestVariantsBasic_Test(t *testing.T) {
	for name, cache := range variants(time.Hour) {
		t.Run(name, func(t *testing.T) {
			defer cache.Stop()

			const n = 10_000
			for i := 0; i < n; i++ {
				cache.Insert(strconv.Itoa(i), i)
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
			cache.Delete("42")
			if _, ok := cache.Get("42"); ok {
				t.Fatal("key 42 still present after delete")
			}

			for i := 0; i < n; i++ {
				cache.Delete(strconv.Itoa(i))
			}
			for i := 0; i < n; i++ {
				if _, ok := cache.Get(strconv.Itoa(i)); ok {
					t.Fatalf("key %d present after delete", i)
				}
			}
		})
	}
}

func TestVariantsExpiration_Test(t *testing.T) {
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
		})
	}
}

func TestVariantsHammer_Test(t *testing.T) {
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
		})
	}
}

// TestSwissChurn_Test exercises tombstone accumulation and purge rehashes:
// repeated insert/delete cycles over a small keyspace must not corrupt the
// table or grow it unboundedly.
func TestSwissChurn_Test(t *testing.T) {
	cache := NewSwissCache[string, int](WithTTL[string, int](time.Hour))
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
