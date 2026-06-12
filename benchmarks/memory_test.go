package benchmarks

import (
	"strconv"
	"testing"
)

// TestMemoryFootprint logs heap retained by each implementation after
// filling to n entries and deleting them all (no post-delete GC).
func TestMemoryFootprint(t *testing.T) {
	for _, impl := range impls {
		for _, n := range memorySizes {
			t.Run(impl.name+"/n="+strconv.Itoa(n), func(t *testing.T) {
				keys := stringKeys(n)
				retained := measureRetainedHeap(impl.make, keys)
				peak := measurePeakHeap(impl.make, keys)
				t.Logf("peak=%.2f MiB retained-after-delete=%.2f MiB",
					float64(peak)/(1024*1024),
					float64(retained)/(1024*1024),
				)
			})
		}
	}
}
