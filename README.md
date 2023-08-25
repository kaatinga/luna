# luna

Thread-Safe Read-Through Cache for Golang.

A robust package for Golang that provides a thread-safe read-through caching mechanism with TTL (Time To Live) and touch
functionalities.

## Features:

- **Thread-Safe**: Efficiently handles concurrent reads and writes without data corruption.
- **Read-Through**: Auto-fetching of values using the provided loader when not found in the cache.
- **TTL**: Set expiry duration for items in the cache.
- **Touch**: Ability to extend an item's expiration time upon retrieval.

## Installation

```bash
go get github.com/kaatinga/luna
```

## Usage

### Initializing the Cache

```go
cache := NewCache(WithTTL(time.Minute * 10), WithLoader(customLoaderFunction))
```

### Inserting an Item

```go
cache.Insert("key", "value")
```

### Deleting an Item

```go
cache.Delete("key")
```

### Retrieving an Item

```go
item := cache.Get("key")
```

## Configuration Options

### WithTTL

Sets the TTL of the cache. Note: It has no effect when passed into `Get()`.

```go
WithTTL(time.Duration)
```

### WithLoader

Sets the loader of the cache. If used with `Get()`, it sets an ephemeral loader that is used instead of the cache's
default one.

```go
WithLoader(loaderFunction)
```

### WithDisableTouchOnHit

Prevents the cache instance from extending/touching an item's expiration timestamp when it's being retrieved. If passed
into `Get()`, it overrides the default value of the cache.

```go
WithDisableTouchOnHit()
```

## Contribution

Pull requests are welcome! For major changes, please open an issue first to discuss the change you'd like to make.

## License

[MIT License](LICENSE.md)