package luna

import (
	"testing"
	"time"
)

// func BenchmarkTTLCache_Add(b *testing.B) {
// 	c := ttlcache.New[string, string](
// 		ttlcache.WithTTL[string, string](time.Second),
// 	)
// 	b.ReportAllocs()
// 	for i := 0; i < b.N; i++ {
// 		k, v := randomUserName(), randomUserName()
// 		c.Set(k, v, ttlcache.DefaultTTL)
// 	}
// }

func BenchmarkLuna_Add(b *testing.B) {
	c := NewCache[string, string](
		WithTTL[string, string](time.Second),
	)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		k, v := randomUserName(), randomUserName()
		c.Insert(k, v)
	}
}
