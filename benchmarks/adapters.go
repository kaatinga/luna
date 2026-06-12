package benchmarks

import (
	"github.com/Yiling-J/theine-go"
	expirable "github.com/go-pkgz/expirable-cache/v3"
	"github.com/jellydator/ttlcache/v3"
	"github.com/maypok86/otter/v2"
	gocache "github.com/patrickmn/go-cache"
)

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

type otterAdapter struct {
	c *otter.Cache[string, int]
}

func (a otterAdapter) Insert(key string, value int) { a.c.Set(key, value) }
func (a otterAdapter) Get(key string) (int, bool)   { return a.c.GetIfPresent(key) }
func (a otterAdapter) Delete(key string)            { a.c.Invalidate(key) }
func (a otterAdapter) Stop()                        {}

type theineAdapter struct {
	c *theine.Cache[string, int]
}

func (a theineAdapter) Insert(key string, value int) { a.c.SetWithTTL(key, value, 1, ttl) }
func (a theineAdapter) Get(key string) (int, bool)   { return a.c.Get(key) }
func (a theineAdapter) Delete(key string)            { a.c.Delete(key) }
func (a theineAdapter) Stop()                        { a.c.Close() }

type expirableAdapter struct {
	c expirable.Cache[string, int]
}

func (a expirableAdapter) Insert(key string, value int) { a.c.Set(key, value, ttl) }
func (a expirableAdapter) Get(key string) (int, bool)   { return a.c.Get(key) }
func (a expirableAdapter) Delete(key string)            { a.c.Remove(key) }
func (a expirableAdapter) Stop()                        {}

type goCacheAdapter struct {
	c *gocache.Cache
}

func (a goCacheAdapter) Insert(key string, value int) { a.c.Set(key, value, gocache.DefaultExpiration) }
func (a goCacheAdapter) Get(key string) (int, bool) {
	v, ok := a.c.Get(key)
	if !ok {
		return 0, false
	}
	return v.(int), true
}
func (a goCacheAdapter) Delete(key string) { a.c.Delete(key) }
func (a goCacheAdapter) Stop()             {}

func makeOtter() (benchCache, error) {
	c, err := otter.New(&otter.Options[string, int]{
		// ExpiryCreating: TTL fixed at insert, not extended on read
		// (equivalent to jellydator's WithDisableTouchOnHit).
		ExpiryCalculator: otter.ExpiryCreating[string, int](ttl),
	})
	if err != nil {
		return nil, err
	}
	return otterAdapter{c: c}, nil
}

func makeTheine() (benchCache, error) {
	c, err := theine.NewBuilder[string, int](2_000_000).Build()
	if err != nil {
		return nil, err
	}
	return theineAdapter{c: c}, nil
}
