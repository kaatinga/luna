[![Tests](https://github.com/kaatinga/luna/actions/workflows/test.yml/badge.svg?branch=main)](https://github.com/kaatinga/luna/actions/workflows/test.yml)
[![codecov](https://codecov.io/gh/kaatinga/luna/graph/badge.svg?token=277RYDJB2J)](https://codecov.io/gh/kaatinga/luna)
[![lint workflow](https://github.com/kaatinga/luna/actions/workflows/golangci-lint.yml/badge.svg)](https://github.com/kaatinga/luna/actions?query=workflow%3Alinter)
[![MIT license](https://img.shields.io/badge/License-MIT-blue.svg)](https://github.com/kaatinga/luna/blob/main/LICENSE)

# Luna

Luna is a fast, dependency-free TTL cache for Go with a deliberately small API.

It stores entries in a hand-rolled open-addressing hash table in the style of
swiss tables (control bytes probed a word at a time, like the Go 1.24+ runtime
map). Each entry is allocated once and doubles as a node of an intrusive
eviction list, so there is no separate map entry, list element and item object
per key. Eviction is driven by a single timer armed for the oldest entry's
deadline — no polling, no channels on the hot path.

## Features

- **Zero dependencies** — stdlib only, `go.sum` is empty.
- **Generic** — any `comparable` key (strings, ints, structs…), any value type.
- **TTL with touch-on-hit** — retrieving an entry extends its life
  (disable with `WithDisableTouchOnHit`). Pass `luna.NoTTL` to `WithTTL`
  and entries never expire — no timer, no eviction goroutine at all.
- **Loaders** — `WithLoader` fills the cache on a `Get` miss; the loader
  runs outside the lock so a slow load never blocks other keys.
- **One-time values** — `GetAndDelete` fetches and removes atomically,
  handy for secrets and PRG-style form state.
- **Two flavours** — `Cache` for single-goroutine-dominated workloads,
  `ShardedCache` (16 independent shards) for heavy concurrent use.
- **Small API on purpose** — `Insert`, `Get`, `GetAndDelete`, `Delete`,
  `Len`, `Stop`. No callbacks, capacity limits, metrics or per-item TTLs.

## Installation

```sh
go get github.com/kaatinga/luna
```

## Usage

```go
package main

import (
	"fmt"
	"time"

	"github.com/kaatinga/luna"
)

func main() {
	cache := luna.NewCache[string, int](
		luna.WithTTL[string, int](time.Minute),
	)
	defer cache.Stop() // releases the eviction goroutine

	cache.Insert("answer", 42)

	if v, ok := cache.Get("answer"); ok {
		fmt.Println(v) // 42
	}

	cache.Delete("answer")
}
```

For workloads where many goroutines hit the cache at once, use the sharded
variant — same API:

```go
cache := luna.NewShardedCache[string, int](
	luna.WithTTL[string, int](time.Minute),
)
```

A loader turns the cache into a read-through cache — `Get` calls it on a
miss and caches the result. The loader runs outside the cache lock, so
concurrent `Get`s of the same cold key may each load it; the last result
wins. Return `false` to report a miss without caching anything:

```go
limiters := luna.NewCache[string, *rate.Limiter](
	luna.WithTTL[string, *rate.Limiter](15*time.Minute),
	luna.WithLoader[string, *rate.Limiter](func(ip string) (*rate.Limiter, bool) {
		return rate.NewLimiter(10, 30), true
	}),
)
```

For one-time values — secrets, PRG form state — `GetAndDelete` fetches and
removes under a single lock acquisition (it never calls the loader):

```go
if secret, ok := cache.GetAndDelete(token); ok {
	// secret can never be retrieved again
}
```

A cache that should keep entries forever skips the eviction machinery
entirely — no timer and no background goroutine are created:

```go
ids := luna.NewCache[string, string](luna.WithTTL[string, string](luna.NoTTL))
```

## Benchmarks

Measured against [jellydator/ttlcache/v3](https://github.com/jellydator/ttlcache)
on an Apple M1 (8 threads), Go 1.26, string keys, touch-on-hit disabled in all
caches, 6 runs each. The mixed workload is 90% Get / 5% Insert / 5% Delete
across all cores via `b.RunParallel`. Full suite and raw results live in
[benchmarks/](benchmarks/), which is a separate module so the root module
stays dependency-free.

ns/op at n=1,000 / 100,000 / 1,000,000 entries — lower is better:

| Benchmark | luna | luna-sharded | jellydator |
|---|---|---|---|
| Get (hit) | **49 / 55 / 127** | 57 / 70 / 140 | 92 / 112 / 201 |
| Get (miss) | **15 / 29 / 23** | 24 / 36 / 29 | 38 / 49 / 105 |
| Insert (new) | **51 / 62 / 172** | 58 / 68 / 144 | 286 / 438 / 523 |
| Insert (overwrite) | **50 / 55 / 123** | 57 / 68 / 140 | 280 / 436 / 592 |
| Delete | **20 / 27 / 73** | 31 / 34 / 78 | 202 / 370 / 559 |
| Mixed parallel | 143 / 227 / 507 | **39 / 50 / 155** | 261 / 429 / 793 |

All luna operations are allocation-free except inserting a new key
(one allocation — the entry itself).

Honest framing: jellydator/ttlcache offers more functionality (capacity
limits, per-item TTLs, metrics, eviction callbacks, singleflight loader
suppression). Luna trades that for a smaller, faster core. If you need
those features, use ttlcache.

## How it works

- `internal/swiss` is a minimal swiss table: one control byte per slot
  holding seven bits of the hash, probed in groups of eight with plain word
  operations (SWAR), tombstones on delete, growth at 7/8 load factor.
  Hashing is stdlib `hash/maphash.Comparable`.
- Entries form a doubly-linked list ordered by expiration. Insert and touch
  move an entry to the front; the janitor goroutine sleeps on one
  `time.Timer` armed for the tail's deadline and evicts from the tail.
- `Get` never returns an expired entry, even before the janitor collects it.
- `ShardedCache` hashes the key once more to pick one of 16 independent
  `Cache` instances, dividing lock contention accordingly.

The AVL-tree engine this project started with, and the other bake-off
prototypes, are preserved on the
[`prototypes`](https://github.com/kaatinga/luna/tree/prototypes) branch.

## Testing

```sh
go test -race ./...
cd benchmarks && go test -run xxx -bench . -count 6
```

## Contributing

Contributions are welcome! Please feel free to submit a pull request or open
issues to discuss potential improvements or features.

## License

[MIT License](LICENSE)
