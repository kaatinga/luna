package luna

import (
	"github.com/kaatinga/luna/internal/item"
	"time"
)

type options[K item.Ordered, V any] struct {
	ttl time.Duration
	// loader            Loader[K, V]
	disableTouchOnHit bool
}

type Option[K item.Ordered, V any] func(*options[K, V])

// WithTTL sets the TTL of the cache.
// It has no effect when passing into Get().
func WithTTL[K item.Ordered, V any](ttl time.Duration) Option[K, V] {
	return func(opts *options[K, V]) {
		opts.ttl = ttl
	}
}

// WithLoader sets the loader of the cache.
// When passing into Get(), it sets an epheral loader that
// is used instead of the cache's default one.
// func WithLoader[K comparable, V any](l Loader[K, V]) Option[K, V] {
// 	return func(opts *options[K, V]) {
// 		opts.loader = l
// 	}
// }

// WithDisableTouchOnHit prevents the cache instance from
// extending/touching an item's expiration timestamp when it is being
// retrieved.
// When passing into Get(), it overrides the default value of the
// cache.
func WithDisableTouchOnHit[K item.Ordered, V any]() Option[K, V] {
	return func(opts *options[K, V]) {
		opts.disableTouchOnHit = true
	}
}
