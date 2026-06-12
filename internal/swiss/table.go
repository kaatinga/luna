// Package swiss is a minimal open-addressing hash table in the style of
// swiss tables (and the Go 1.24+ runtime map): one control byte per slot
// holding seven bits of the hash, probed a group of eight at a time with
// plain word operations (SWAR), so most probes resolve without touching
// the entries at all.
//
// Entries live in a dense arena indexed by int32: the table performs no
// per-item allocation and the GC sees three slices instead of one object
// per item. Indices stay stable across arena growth and rehashes, so
// callers may keep them across operations; *Entry pointers obtained via At
// are only valid until the next Insert.
package swiss

import (
	"encoding/binary"
	"hash/maphash"
	"math"
	"math/bits"
)

const (
	groupSize = 8

	ctrlEmpty   = uint8(0x80) // high bit set, low bits zero
	ctrlDeleted = uint8(0xFE) // tombstone

	lsb = 0x0101010101010101
	msb = 0x8080808080808080
)

// NoIndex marks the absence of an entry index: a miss, an empty free list
// or the end of the caller's eviction list.
const NoIndex int32 = -1

// Entry is a table entry, stored inline in the arena. It doubles as the
// caller's eviction-list node; freed entries reuse Next as the free-list
// link.
type Entry[K comparable, V any] struct {
	Key   K
	Value V

	// ExpirationTime is the eviction deadline in unix nanoseconds.
	ExpirationTime int64

	// eviction list links: Next points towards older entries, Prev
	// towards newer ones; NoIndex terminates the list.
	Next, Prev int32
}

// Table is the hash table. It is not safe for concurrent use; the caller
// locks. A table holds at most math.MaxInt32 entries.
type Table[K comparable, V any] struct {
	ctrl     []uint8       // one control byte per slot
	slots    []int32       // arena indices, valid only where ctrl holds an h2
	entries  []Entry[K, V] // dense arena
	freeHead int32         // intrusive free list of deleted entries
	groups   uint64        // power of two
	size     int           // live entries
	dead     int           // tombstones
	seed     maphash.Seed
}

func NewTable[K comparable, V any]() *Table[K, V] {
	return NewSeededTable[K, V](maphash.MakeSeed())
}

// NewSeededTable creates a table hashing with the given seed, so several
// tables can share hashes computed once by the caller. The caller must not
// derive any table-visible bits (group, h2) from the same hash itself; shard
// selection uses the high bits, which the table ignores.
func NewSeededTable[K comparable, V any](seed maphash.Seed) *Table[K, V] {
	t := &Table[K, V]{seed: seed, freeHead: NoIndex}
	t.rehash(2)
	return t
}

// Hash returns the hash of a key. It is exposed so callers can compute it
// outside the lock.
func (t *Table[K, V]) Hash(key K) uint64 {
	return maphash.Comparable(t.seed, key)
}

func (t *Table[K, V]) Len() int { return t.size }

// At returns the entry stored under an index obtained from Get, Insert or
// Delete. The pointer is invalidated by any subsequent Insert, which may
// move the arena; the index itself stays valid until Free.
func (t *Table[K, V]) At(idx int32) *Entry[K, V] {
	return &t.entries[idx]
}

func h2(hash uint64) uint8 { return uint8(hash & 0x7F) }

func (t *Table[K, V]) group(hash uint64) uint64 {
	return (hash >> 7) & (t.groups - 1)
}

func (t *Table[K, V]) word(g uint64) uint64 {
	return binary.LittleEndian.Uint64(t.ctrl[g*groupSize:])
}

// matchByte returns a mask with the high bit set in every byte of word that
// equals b.
func matchByte(word uint64, b uint8) uint64 {
	x := word ^ (lsb * uint64(b))
	return (x - lsb) &^ x & msb
}

// matchFree returns a mask of empty or deleted slots (high bit set).
func matchFree(word uint64) uint64 {
	return word & msb
}

// Get returns the index of the entry holding the key, or NoIndex.
func (t *Table[K, V]) Get(hash uint64, key K) int32 {
	g := t.group(hash)
	for i := uint64(1); ; i++ {
		word := t.word(g)
		for m := matchByte(word, h2(hash)); m != 0; m &= m - 1 {
			idx := t.slots[g*groupSize+uint64(bits.TrailingZeros64(m))/8]
			// h2 already filtered 7 hash bits, so a false positive
			// reaches the key comparison once per ~128 candidates
			if t.entries[idx].Key == key {
				return idx
			}
		}
		if matchByte(word, ctrlEmpty) != 0 {
			// the group has a never-used slot, so the key cannot
			// live in a later group
			return NoIndex
		}
		g = (g + i) & (t.groups - 1)
	}
}

// Insert returns the index of the entry for the key and whether it already
// existed. For a new key the returned entry has only Key, hash and reset
// list links; the caller fills in the rest.
func (t *Table[K, V]) Insert(hash uint64, key K) (int32, bool) {
	if (t.size+t.dead+1)*8 > len(t.slots)*7 {
		t.grow()
	}

	g := t.group(hash)
	freeSlot := int64(-1)
	for i := uint64(1); ; i++ {
		word := t.word(g)
		for m := matchByte(word, h2(hash)); m != 0; m &= m - 1 {
			idx := t.slots[g*groupSize+uint64(bits.TrailingZeros64(m))/8]
			if t.entries[idx].Key == key {
				return idx, true
			}
		}
		if freeSlot < 0 {
			if m := matchFree(word); m != 0 {
				freeSlot = int64(g*groupSize + uint64(bits.TrailingZeros64(m))/8)
			}
		}
		if matchByte(word, ctrlEmpty) != 0 {
			// key is absent; reuse the first free slot seen
			s := uint64(freeSlot)
			if t.ctrl[s] == ctrlDeleted {
				t.dead--
			}
			idx := t.alloc()
			e := &t.entries[idx]
			e.Key = key
			// a recycled entry carries the free-list link in Next
			e.Next, e.Prev = NoIndex, NoIndex
			t.ctrl[s] = h2(hash)
			t.slots[s] = idx
			t.size++
			return idx, false
		}
		g = (g + i) & (t.groups - 1)
	}
}

// alloc takes an entry off the free list or extends the arena. Growth moves
// the arena, so any *Entry obtained earlier is invalid after alloc returns.
func (t *Table[K, V]) alloc() int32 {
	if idx := t.freeHead; idx != NoIndex {
		t.freeHead = t.entries[idx].Next
		return idx
	}
	if len(t.entries) >= math.MaxInt32 {
		panic("swiss: table over math.MaxInt32 entries")
	}
	t.entries = append(t.entries, Entry[K, V]{})
	return int32(len(t.entries) - 1)
}

// Free returns an entry to the free list, zeroing it so pointers held in
// the key or value are released to the GC. The caller must have removed
// the index from the table (Delete or DeleteIndex) and from its own list
// links first; the index must not be used afterwards.
func (t *Table[K, V]) Free(idx int32) {
	t.entries[idx] = Entry[K, V]{Next: t.freeHead, Prev: NoIndex}
	t.freeHead = idx
}

// Delete removes the key from the table and returns the index of its entry,
// or NoIndex. The entry itself is left intact so the caller can read it and
// unlink it from its eviction list; the caller must then release it with
// Free.
func (t *Table[K, V]) Delete(hash uint64, key K) int32 {
	g := t.group(hash)
	for i := uint64(1); ; i++ {
		word := t.word(g)
		for m := matchByte(word, h2(hash)); m != 0; m &= m - 1 {
			s := g*groupSize + uint64(bits.TrailingZeros64(m))/8
			idx := t.slots[s]
			if t.entries[idx].Key == key {
				// the stale index left in slots is unreachable:
				// reads are gated by the ctrl byte
				t.ctrl[s] = ctrlDeleted
				t.size--
				t.dead++
				return idx
			}
		}
		if matchByte(word, ctrlEmpty) != 0 {
			return NoIndex
		}
		g = (g + i) & (t.groups - 1)
	}
}

// DeleteIndex removes an entry known to be in the table, located by
// re-hashing its key and matched by index, skipping the key comparison.
// Like Delete it leaves the entry intact; the caller releases it with Free.
func (t *Table[K, V]) DeleteIndex(target int32) {
	hash := t.Hash(t.entries[target].Key)
	g := t.group(hash)
	for i := uint64(1); ; i++ {
		word := t.word(g)
		for m := matchByte(word, h2(hash)); m != 0; m &= m - 1 {
			s := g*groupSize + uint64(bits.TrailingZeros64(m))/8
			if t.slots[s] == target {
				t.ctrl[s] = ctrlDeleted
				t.size--
				t.dead++
				return
			}
		}
		if matchByte(word, ctrlEmpty) != 0 {
			return
		}
		g = (g + i) & (t.groups - 1)
	}
}

// MaybeShrink halves the table while the load is below 1/8, landing at
// roughly 1/4 — far enough from the 7/8 grow threshold that expiry/refill
// cycles cannot thrash between rehashes. Callers invoke it off the hot
// path, after a mass eviction. The arena is not compacted: freed cells are
// reused by later inserts.
func (t *Table[K, V]) MaybeShrink() {
	groups := t.groups
	for groups > 2 && t.size*8 < int(groups)*groupSize {
		groups /= 2
	}
	if groups != t.groups {
		t.rehash(groups)
	}
}

func (t *Table[K, V]) grow() {
	groups := t.groups
	if (t.size+1)*8 > len(t.slots)*7/2 {
		// genuinely getting full: double
		groups *= 2
	} // else: mostly tombstones, rehash in place to purge them
	t.rehash(groups)
}

// rehash rebuilds ctrl and slots; the arena and free list are untouched,
// so indices held by the caller survive.
func (t *Table[K, V]) rehash(groups uint64) {
	oldCtrl, oldSlots := t.ctrl, t.slots
	t.ctrl = make([]uint8, groups*groupSize)
	for i := range t.ctrl {
		t.ctrl[i] = ctrlEmpty
	}
	t.slots = make([]int32, groups*groupSize)
	t.groups = groups
	t.dead = 0
	for s, c := range oldCtrl {
		// a full slot's ctrl byte is the entry's h2, high bit clear;
		// tombstoned slots hold stale indices and must be skipped
		if c < ctrlEmpty {
			t.place(oldSlots[s])
		}
	}
}

// place inserts a known-absent entry into a table without tombstones,
// re-hashing its key: entries do not cache their hash, trading rehash
// time for eight bytes per item.
func (t *Table[K, V]) place(idx int32) {
	hash := t.Hash(t.entries[idx].Key)
	g := t.group(hash)
	for i := uint64(1); ; i++ {
		if m := matchByte(t.word(g), ctrlEmpty); m != 0 {
			s := g*groupSize + uint64(bits.TrailingZeros64(m))/8
			t.ctrl[s] = h2(hash)
			t.slots[s] = idx
			return
		}
		g = (g + i) & (t.groups - 1)
	}
}
