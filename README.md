[![Tests](https://github.com/kaatinga/luna/actions/workflows/test.yml/badge.svg?branch=main)](https://github.com/kaatinga/luna/actions/workflows/test.yml)
[![codecov](https://codecov.io/gh/kaatinga/luna/graph/badge.svg?token=277RYDJB2J)](https://codecov.io/gh/kaatinga/luna)
[![lint workflow](https://github.com/kaatinga/luna/actions/workflows/golangci-lint.yml/badge.svg)](https://github.com/kaatinga/luna/actions?query=workflow%3Alinter)
[![MIT license](https://img.shields.io/badge/License-MIT-blue.svg)](https://github.com/kaatinga/luna/blob/main/LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/kaatinga/luna.svg)](https://pkg.go.dev/github.com/kaatinga/luna)

# Luna

Luna is a fast, dependency-free TTL cache for Go with a deliberately small API.

**Why Luna:** it stores entries in a dense arena instead of a `map`, so steady
state — even cold inserts after a delete — is **allocation-free** and the GC
sees three slices rather than one object per key. At 1M entries that means
**0 allocs/op** on insert and roughly **half the retained heap** of
`go-cache`, `theine` or `jellydator` (see [Benchmarks](#benchmarks)), with
get/insert/delete latency at or below every cache benchmarked. If you want a
small, memory-frugal core and don't need capacity limits, per-item TTLs or
eviction callbacks, Luna is for you.

It stores entries in a hand-rolled open-addressing hash table in the style of
swiss tables (control bytes probed a word at a time, like the Go 1.24+ runtime
map). Entries live inline in a dense arena indexed by `int32` and double as
nodes of an intrusive eviction list, so there is no per-item allocation at
all — the GC sees three slices instead of an object per key. Eviction is
driven by a single timer armed for the oldest entry's deadline — no polling,
no channels on the hot path.

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
- **Presizing** — `WithInitialSize(n)` reserves the table and arena up front,
  so even the initial fill is allocation-free, not just steady-state churn.
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

If you know roughly how many entries the cache will hold, reserve them up
front with `WithInitialSize` — the fill then triggers no rehash and no arena
reallocation, so every insert during warm-up is allocation-free too (for
`ShardedCache` the size is the total across all shards):

```go
sessions := luna.NewCache[string, Session](
	luna.WithTTL[string, Session](30*time.Minute),
	luna.WithInitialSize[string, Session](100_000),
)
```

## Benchmarks

Measured on an Apple M1 (8 threads), Go 1.26, string keys, touch-on-hit
disabled in all caches, 6 runs each. Compared against a hand-rolled
`map` + `RWMutex` baseline (`naive-map`) and five TTL caches:
[jellydator/ttlcache/v3](https://github.com/jellydator/ttlcache),
[maypok86/otter/v2](https://github.com/maypok86/otter),
[Yiling-J/theine-go](https://github.com/Yiling-J/theine-go),
[go-pkgz/expirable-cache/v3](https://github.com/go-pkgz/expirable-cache),
and [patrickmn/go-cache](https://github.com/patrickmn/go-cache). The mixed
workload is 90% Get / 5% Insert / 5% Delete across all cores via
`b.RunParallel`. Full suite lives in [benchmarks/](benchmarks/) — a
separate module so the root module stays dependency-free.

Reproduce it yourself — every number below comes from one command:

```sh
cd benchmarks
make bench     # full latency + allocs suite, 6 runs
make memory    # heap retained after fill+delete
make stat      # benchstat new.txt against the committed baseline
```

ns/op at n=1,000 / 100,000 / 1,000,000 entries — lower is better:

| Benchmark | luna | sharded | naive-map | otter | theine | expirable | go-cache | jellydator |
|---|---|---|---|---|---|---|---|---|
| Get (hit) | **50 / 57 / 122** | 51 / 62 / 129 | 48 / 59 / 158 | 75 / 93 / 184 | 109 / 155 / 278 | 54 / 73 / 168 | 51 / 68 / 180 | 92 / 120 / 214 |
| Get (miss) | **17 / 30 / 25** | 21 / 33 / 26 | 48 / 60 / 132 | 25 / 31 / 106 | 45 / 65 / 135 | 16 / 33 / 66 | 15 / 30 / 51 | 39 / 49 / 122 |
| Insert (overwrite) | **52 / 55 / 124** | 52 / 57 / 128 | 60 / 72 / 175 | 197 / 222 / 298 | 181 / 196 / 292 | 56 / 69 / 164 | 67 / 79 / 202 | 288 / 454 / 603 |
| Insert (fresh slot) | **74 / 102 / 187** | 82 / 113 / 208 | 116 / 197 / 304 | 412 / 416 / 471 | 431 / 494 / 573 | 196 / 268 / 315 | 104 / 143 / 238 | 429 / 664 / 701 |
| Delete | **23 / 25 / 50** | 30 / 30 / 54 | 32 / 39 / 85 | 156 / 169 / 241 | 192 / 216 / 288 | 63 / 80 / 167 | 44 / 52 / 100 | 193 / 346 / 534 |
| Mixed parallel | 142 / 172 / 481 | **39 / 50 / 166** | 119 / 193 / 358 | 28 / 36 / 90 | 65 / 97 / 187 | 166 / 396 / 543 | 103 / 167 / 362 | 247 / 445 / 787 |

Allocations on a delete-then-insert cycle (`BenchmarkInsertFresh`, n=1,000,000)
— lower is better:

| | luna | sharded | naive-map | otter | theine | expirable | go-cache | jellydator |
|---|---|---|---|---|---|---|---|---|
| allocs/op | **0** | **0** | 1 | 1 | 1 | 2 | 1 | 3 |
| B/op | **37** | 35 | 60 | 92 | 141 | 126 | 52 | 234 |

Luna recycles arena slots, so even cold inserts are allocation-free. The
`naive-map` baseline allocates one heap object per new key; map-backed
libraries allocate at least once per fresh entry as well.

Heap retained after filling to n entries and deleting them all, without a
post-delete GC (MiB — lower is better). Map-backed caches keep their
high-water bucket tables until the runtime collects them; luna's arena and
swiss table hold similar dense storage but avoid per-entry pointers.

| | n=100,000 | n=1,000,000 |
|---|---|---|
| luna | 11.4 | **89.4** |
| luna-sharded | 16.9 | 88.4 |
| naive-map | 9.9 | 93.6 |
| otter | 14.5 | 67.2 |
| theine | 18.5 | 183.1 |
| expirable | 15.5 | 200.4 |
| go-cache | 12.8 | 199.5 |
| jellydator | 28.5 | 243.9 |

At n=1,000,000 the naive `map` baseline retains ~94 MiB of bucket storage
after every key is deleted — the same high-water mark as at peak — and each
insert still allocated a `*entry` on the heap during the fill phase. Luna
matches that footprint with zero per-entry allocations and recycles slots
in-place; several libraries retain substantially more.

Honest framing: jellydator, otter and theine offer more functionality
(capacity limits, per-item TTLs, metrics, eviction callbacks, loaders).
Luna trades that for a smaller, faster core. If you need those features,
use one of the libraries above.

## How it works

- `internal/swiss` is a minimal swiss table: one control byte per slot
  holding seven bits of the hash, probed in groups of eight with plain word
  operations (SWAR), tombstones on delete, growth at 7/8 load factor.
  Hashing is stdlib `hash/maphash.Comparable`.
- Entries are stored inline in a dense arena and addressed by `int32`
  indices, which stay stable across table growth; deleted entries feed an
  intrusive free list, so steady-state operation never allocates. After a
  mass expiry the table shrinks back down (explicit Delete-only workloads
  keep their high-water size — shrinking happens off the hot path).
- Entries form a doubly-linked list ordered by expiration. Insert and touch
  move an entry to the front; the janitor goroutine sleeps on one
  `time.Timer` armed for the tail's deadline and evicts from the tail.
- `Get` never returns an expired entry, even before the janitor collects it.
- `ShardedCache` hashes the key once and uses the high bits to pick one of
  16 independent `Cache` instances (the tables consume the low bits),
  dividing lock contention accordingly.

The AVL-tree engine this project started with, and the other bake-off
prototypes, are preserved on the
[`prototypes`](https://github.com/kaatinga/luna/tree/prototypes) branch.

## Testing

```sh
go test -race ./...
cd benchmarks && go test -run xxx -bench . -benchmem -count 6
go test -run TestMemoryFootprint -v   # heap retained after fill+delete
```

## Contributing

Contributions are welcome! Please feel free to submit a pull request or open
issues to discuss potential improvements or features.

## License

[MIT License](LICENSE)
