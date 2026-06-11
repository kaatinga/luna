package luna

// Loader is an interface that handles missing data loading.
// type Loader[K comparable, V any] interface {
// 	// Load should execute a custom item retrieval logic and
// 	// return the item that is associated with the key.
// 	// It should return nil if the item is not found/valid.
// 	// The method is allowed to fetch data from the cache instance
// 	// or update it for future use.
// 	Load(c *Cache[K, V], key K) *Item[K, V]
// }
//
// LoaderFunc type is an adapter that allows the use of ordinary
// functions as data loaders.
// type LoaderFunc[K comparable, V any] func(*Cache[K, V], K) *Item[K, V]
