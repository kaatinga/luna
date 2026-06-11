package luna

import (
	"time"
)

type options[K comparable, V any] struct {
	ttl               time.Duration
	disableTouchOnHit bool
}

type Option[K comparable, V any] func(*options[K, V])

// WithTTL sets the TTL of the cache.
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
