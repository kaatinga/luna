// Package swiss is a minimal open-addressing hash table in the style of
// swiss tables (and the Go 1.24+ runtime map): one control byte per slot
// holding seven bits of the hash, probed a group of eight at a time with
// plain word operations (SWAR), so most probes resolve without touching
// the entries at all.
package swiss

import (
	"encoding/binary"
	"hash/maphash"
	"math/bits"
)

const (
	groupSize = 8

	ctrlEmpty   = uint8(0x80) // high bit set, low bits zero
	ctrlDeleted = uint8(0xFE) // tombstone

	lsb = 0x0101010101010101
	msb = 0x8080808080808080
)

// Entry is a table entry. It is allocated once per item and doubles as the
// eviction-list node, so the table costs one allocation per item.
type Entry[K comparable, V any] struct {
	Key   K
	Value V

	// ExpirationTime is the eviction deadline in unix nanoseconds.
	ExpirationTime int64

	// eviction list links: NextEntry points towards older entries,
	// PreviousEntry towards newer ones.
	NextEntry     *Entry[K, V]
	PreviousEntry *Entry[K, V]

	hash uint64
}

// Table is the hash table. It is not safe for concurrent use; the caller
// locks.
type Table[K comparable, V any] struct {
	ctrl   []uint8        // one control byte per slot
	slots  []*Entry[K, V] // len(slots) == len(ctrl) == groups*8
	groups uint64         // power of two
	size   int            // live entries
	dead   int            // tombstones
	seed   maphash.Seed
}

func NewTable[K comparable, V any]() *Table[K, V] {
	t := &Table[K, V]{seed: maphash.MakeSeed()}
	t.rehash(2)
	return t
}

// Hash returns the hash of a key. It is exposed so callers can compute it
// outside the lock.
func (t *Table[K, V]) Hash(key K) uint64 {
	return maphash.Comparable(t.seed, key)
}

func (t *Table[K, V]) Len() int { return t.size }

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

// Get returns the entry holding the key, or nil.
func (t *Table[K, V]) Get(hash uint64, key K) *Entry[K, V] {
	g := t.group(hash)
	for i := uint64(1); ; i++ {
		word := t.word(g)
		for m := matchByte(word, h2(hash)); m != 0; m &= m - 1 {
			e := t.slots[g*groupSize+uint64(bits.TrailingZeros64(m))/8]
			if e.hash == hash && e.Key == key {
				return e
			}
		}
		if matchByte(word, ctrlEmpty) != 0 {
			// the group has a never-used slot, so the key cannot
			// live in a later group
			return nil
		}
		g = (g + i) & (t.groups - 1)
	}
}

// Insert returns the entry for the key and whether it already existed. For
// a new key the returned entry has only Key and hash set; the caller fills
// in the rest.
func (t *Table[K, V]) Insert(hash uint64, key K) (*Entry[K, V], bool) {
	if (t.size+t.dead+1)*8 > len(t.slots)*7 {
		t.grow()
	}

	g := t.group(hash)
	freeIdx := int64(-1)
	for i := uint64(1); ; i++ {
		word := t.word(g)
		for m := matchByte(word, h2(hash)); m != 0; m &= m - 1 {
			e := t.slots[g*groupSize+uint64(bits.TrailingZeros64(m))/8]
			if e.hash == hash && e.Key == key {
				return e, true
			}
		}
		if freeIdx < 0 {
			if m := matchFree(word); m != 0 {
				freeIdx = int64(g*groupSize + uint64(bits.TrailingZeros64(m))/8)
			}
		}
		if matchByte(word, ctrlEmpty) != 0 {
			// key is absent; reuse the first free slot seen
			idx := uint64(freeIdx)
			if t.ctrl[idx] == ctrlDeleted {
				t.dead--
			}
			e := &Entry[K, V]{Key: key, hash: hash}
			t.ctrl[idx] = h2(hash)
			t.slots[idx] = e
			t.size++
			return e, false
		}
		g = (g + i) & (t.groups - 1)
	}
}

// Delete removes the key and returns its entry, or nil.
func (t *Table[K, V]) Delete(hash uint64, key K) *Entry[K, V] {
	g := t.group(hash)
	for i := uint64(1); ; i++ {
		word := t.word(g)
		for m := matchByte(word, h2(hash)); m != 0; m &= m - 1 {
			idx := g*groupSize + uint64(bits.TrailingZeros64(m))/8
			e := t.slots[idx]
			if e.hash == hash && e.Key == key {
				t.ctrl[idx] = ctrlDeleted
				t.slots[idx] = nil
				t.size--
				t.dead++
				return e
			}
		}
		if matchByte(word, ctrlEmpty) != 0 {
			return nil
		}
		g = (g + i) & (t.groups - 1)
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

func (t *Table[K, V]) rehash(groups uint64) {
	oldSlots := t.slots
	t.ctrl = make([]uint8, groups*groupSize)
	for i := range t.ctrl {
		t.ctrl[i] = ctrlEmpty
	}
	t.slots = make([]*Entry[K, V], groups*groupSize)
	t.groups = groups
	t.dead = 0
	for _, e := range oldSlots {
		if e != nil {
			t.place(e)
		}
	}
}

// place inserts a known-absent entry into a table without tombstones.
func (t *Table[K, V]) place(e *Entry[K, V]) {
	g := t.group(e.hash)
	for i := uint64(1); ; i++ {
		if m := matchByte(t.word(g), ctrlEmpty); m != 0 {
			idx := g*groupSize + uint64(bits.TrailingZeros64(m))/8
			t.ctrl[idx] = h2(e.hash)
			t.slots[idx] = e
			return
		}
		g = (g + i) & (t.groups - 1)
	}
}
