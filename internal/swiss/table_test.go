package swiss

import "testing"

func key(i int) string {
	return string(rune('a'+i%26)) + string(rune('0'+i%10)) + string(rune('A'+i%13)) + string(rune('a'+i/338))
}

func TestMaybeShrink(t *testing.T) {
	tbl := NewTable[int, int]()
	const n = 100_000
	for i := range n {
		tbl.Insert(tbl.Hash(i), i)
	}
	grown := len(tbl.slots)
	if grown < n {
		t.Fatalf("table did not grow: %d slots for %d entries", grown, n)
	}

	// shrinking a near-full table must be a no-op
	tbl.MaybeShrink()
	if len(tbl.slots) != grown {
		t.Fatalf("MaybeShrink shrank a full table: %d -> %d slots", grown, len(tbl.slots))
	}

	for i := 100; i < n; i++ {
		if idx := tbl.Delete(tbl.Hash(i), i); idx != NoIndex {
			tbl.Free(idx)
		}
	}
	tbl.MaybeShrink()
	if len(tbl.slots) >= grown {
		t.Fatalf("MaybeShrink did not shrink: still %d slots for %d entries", len(tbl.slots), tbl.Len())
	}
	// the surviving load must sit at or above 1/8 so shrinking has converged
	if tbl.Len()*8 < len(tbl.slots) && len(tbl.slots) > 2*groupSize {
		t.Fatalf("MaybeShrink stopped early: %d entries in %d slots", tbl.Len(), len(tbl.slots))
	}

	// all remaining entries must still be reachable after the rehash
	for i := range 100 {
		idx := tbl.Get(tbl.Hash(i), i)
		if idx == NoIndex || tbl.At(idx).Key != i {
			t.Fatalf("entry %d lost after shrink", i)
		}
	}
}

func TestDeleteIndex(t *testing.T) {
	tbl := NewTable[string, int]()
	const n = 10_000
	indices := make(map[int]int32, n)
	for i := range n {
		idx, existed := tbl.Insert(tbl.Hash(key(i)), key(i))
		if existed {
			continue // key() collides occasionally; skip duplicates
		}
		tbl.At(idx).Value = i
		indices[i] = idx
	}

	for i, idx := range indices {
		tbl.DeleteIndex(idx)
		tbl.Free(idx)
		if got := tbl.Get(tbl.Hash(key(i)), key(i)); got != NoIndex {
			t.Fatalf("entry %q still in table after DeleteIndex", key(i))
		}
	}
	if tbl.Len() != 0 {
		t.Fatalf("table not empty after deleting all entries: %d left", tbl.Len())
	}
}

// TestArenaReuse verifies the free list recycles entries instead of growing
// the arena.
func TestArenaReuse(t *testing.T) {
	tbl := NewTable[int, int]()
	const n = 10_000
	for i := range n {
		tbl.Insert(tbl.Hash(i), i)
	}
	arena := len(tbl.entries)
	for i := range n {
		idx := tbl.Delete(tbl.Hash(i), i)
		if idx == NoIndex {
			t.Fatalf("entry %d missing", i)
		}
		tbl.Free(idx)
	}
	for i := n; i < 2*n; i++ {
		tbl.Insert(tbl.Hash(i), i)
	}
	if len(tbl.entries) != arena {
		t.Fatalf("arena grew despite free list: %d -> %d cells", arena, len(tbl.entries))
	}
	if tbl.Len() != n {
		t.Fatalf("wrong size after refill: %d", tbl.Len())
	}
}

// TestReserve checks that presizing lets a fill of n entries complete with
// no rehash (groups unchanged) and no arena reallocation (cap unchanged).
func TestReserve(t *testing.T) {
	const n = 10_000
	tbl := NewTable[int, int]()
	tbl.Reserve(n)

	groups := tbl.groups
	arenaCap := cap(tbl.entries)
	if arenaCap < n {
		t.Fatalf("arena not reserved: cap %d < %d", arenaCap, n)
	}

	for i := range n {
		tbl.Insert(tbl.Hash(i), i)
	}

	if tbl.groups != groups {
		t.Fatalf("table rehashed during reserved fill: %d -> %d groups", groups, tbl.groups)
	}
	if cap(tbl.entries) != arenaCap {
		t.Fatalf("arena reallocated during reserved fill: %d -> %d cap", arenaCap, cap(tbl.entries))
	}
	if tbl.Len() != n {
		t.Fatalf("wrong size after fill: %d", tbl.Len())
	}
	for i := range n {
		if tbl.Get(tbl.Hash(i), i) == NoIndex {
			t.Fatalf("entry %d missing after reserved fill", i)
		}
	}
}

// TestReserveNoShrink checks Reserve only grows: a smaller hint is a no-op.
func TestReserveNoShrink(t *testing.T) {
	tbl := NewTable[int, int]()
	tbl.Reserve(10_000)
	groups, arenaCap := tbl.groups, cap(tbl.entries)
	tbl.Reserve(10)
	if tbl.groups != groups || cap(tbl.entries) != arenaCap {
		t.Fatalf("Reserve shrank the table: groups %d->%d, cap %d->%d",
			groups, tbl.groups, arenaCap, cap(tbl.entries))
	}
}

// TestFreeZeroesEntry guards the GC-release contract of Free.
func TestFreeZeroesEntry(t *testing.T) {
	tbl := NewTable[string, *int]()
	v := new(int)
	idx, _ := tbl.Insert(tbl.Hash("a"), "a")
	e := tbl.At(idx)
	e.Value = v
	e.ExpirationTime = 42

	got := tbl.Delete(tbl.Hash("a"), "a")
	if got != idx {
		t.Fatalf("Delete returned %d, want %d", got, idx)
	}
	tbl.Free(idx)
	e = tbl.At(idx)
	if e.Key != "" || e.Value != nil || e.ExpirationTime != 0 {
		t.Fatalf("freed entry not zeroed: %+v", e)
	}
}

// TestChurnIntegrity hammers insert/delete/get and checks that the free
// list and the live count always partition the arena.
func TestChurnIntegrity(t *testing.T) {
	tbl := NewTable[int, int]()
	live := map[int]bool{}
	rng := uint64(1)
	for step := range 200_000 {
		rng = rng*6364136223846793005 + 1442695040888963407
		k := int(rng>>33) % 500
		switch step % 3 {
		case 0, 1:
			idx, existed := tbl.Insert(tbl.Hash(k), k)
			if existed != live[k] {
				t.Fatalf("step %d: Insert(%d) existed=%v, want %v", step, k, existed, live[k])
			}
			tbl.At(idx).Value = step
			live[k] = true
		case 2:
			idx := tbl.Delete(tbl.Hash(k), k)
			if (idx != NoIndex) != live[k] {
				t.Fatalf("step %d: Delete(%d) found=%v, want %v", step, k, idx != NoIndex, live[k])
			}
			if idx != NoIndex {
				tbl.Free(idx)
			}
			delete(live, k)
		}
	}

	if tbl.Len() != len(live) {
		t.Fatalf("size mismatch: table %d, model %d", tbl.Len(), len(live))
	}
	for k := range live {
		if tbl.Get(tbl.Hash(k), k) == NoIndex {
			t.Fatalf("live key %d missing", k)
		}
	}
	// free list + live entries must account for the whole arena
	free := 0
	for idx := tbl.freeHead; idx != NoIndex; idx = tbl.entries[idx].Next {
		if free++; free > len(tbl.entries) {
			t.Fatal("free list cycle")
		}
	}
	if free+tbl.Len() != len(tbl.entries) {
		t.Fatalf("arena leak: %d free + %d live != %d cells", free, tbl.Len(), len(tbl.entries))
	}
}
