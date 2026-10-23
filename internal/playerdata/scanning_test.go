package playerdata

import (
	"testing"
	"time"
)

type countingMemory struct {
	fakeMemory
	scans int
}

func (m *countingMemory) ScanPattern(pattern []byte, limit int) ([]uintptr, uint64) {
	m.scans++
	return m.fakeMemory.ScanPattern(pattern, limit)
}

func (m *countingMemory) ScanPatterns(patterns [][]byte, _ int) ([][]uintptr, uint64) {
	m.scans++
	results := make([][]uintptr, len(patterns))
	for i, pattern := range patterns {
		results[i] = m.patterns[string(pattern)]
	}
	return results, 0
}

func TestWarmSnapshotReadsDoNotRescanMemory(t *testing.T) {
	m := &countingMemory{fakeMemory: fakeMemory{ptrs: map[uintptr]uintptr{}, ints: map[uintptr]int32{}, uints: map[uintptr]uint64{}, patterns: map[string][]uintptr{}}}
	writeClass(m.fakeMemory, "PlayerSaveData", 0x200000, 0x300000, 0x400000)
	writeClass(m.fakeMemory, "InventorySaveData", 0x210000, 0x310000)
	writeClass(m.fakeMemory, "RuneSaveData", 0x220000, 0x320000)
	writeCandidateSaveData(m.fakeMemory, 0x400000, 0x500000, 100, 12345)
	r := NewResolver(nil)
	if _, ok := r.ReadSnapshotCore(m, time.Now()); !ok {
		t.Fatal("initial snapshot unavailable")
	}
	m.scans = 0
	if snapshot, ok := r.ReadSnapshotCore(m, time.Now()); !ok || snapshot.Gold != 12345 {
		t.Fatalf("warm snapshot=%+v ok=%t", snapshot, ok)
	}
	if m.scans != 0 {
		t.Fatalf("warm read performed %d memory scans", m.scans)
	}
}

func TestRuntimeTradeSlotsReuseLiveObjectsAndRecoverAfterDestruction(t *testing.T) {
	m := &countingMemory{fakeMemory: fakeMemory{ptrs: map[uintptr]uintptr{}, ints: map[uintptr]int32{}, uints: map[uintptr]uint64{}, patterns: map[string][]uintptr{}}}
	writeClass(m.fakeMemory, "TradingStashSlot", 0x210000, 0x310000, 0x400000)
	writeClass(m.fakeMemory, "TradingStashCache", 0x220000, 0x320000)
	m.ptrs[0x500000] = 0x320000
	now := time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)
	writeSlot := func(object uintptr, index int32, until time.Time) {
		m.ptrs[object] = 0x310000
		m.ptrs[object+0x10] = 0x700000
		m.ints[object+runtimeTradeSlotIndex] = index
		m.ptrs[object+runtimeTradeSlotCache] = 0x500000
		m.ptrs[object+runtimeTradeSlotCountdownText] = 0x600000
		m.uints[object+runtimeTradeSlotCooldownUntil] = dotnetDateData(until)
	}
	writeSlot(0x400000, 0, now.Add(time.Hour))
	r := NewResolver(nil)
	first := r.readRuntimeTradeSlots(m, now, 1)
	if len(first) != 1 {
		t.Fatalf("initial slots=%v", first)
	}
	m.scans = 0
	writeSlot(0x400000, 0, now.Add(2*time.Hour))
	second := r.readRuntimeTradeSlots(m, now.Add(time.Second), 1)
	if len(second) != 1 || !second[0].CooldownUntil.Equal(now.Add(2*time.Hour)) || m.scans != 0 {
		t.Fatalf("live slots=%v scans=%d", second, m.scans)
	}
	m.ptrs[0x400000+0x10] = 0 // Destroyed Unity component.
	if got := r.readRuntimeTradeSlots(m, now.Add(2*time.Second), 1); len(got) != 0 || m.scans != 0 {
		t.Fatalf("destroyed slots=%v scans=%d", got, m.scans)
	}
	writeSlot(0x410000, 0, now.Add(3*time.Hour))
	m.patterns[pointerPattern(0x310000)] = []uintptr{0x400000, 0x410000}
	recovered := r.readRuntimeTradeSlots(m, now.Add(6*time.Second), 1)
	if len(recovered) != 1 || !recovered[0].CooldownUntil.Equal(now.Add(3*time.Hour)) || m.scans != 1 {
		t.Fatalf("recovered=%v scans=%d", recovered, m.scans)
	}
}

func TestImportedClassPointersAreValidatedBeforeUse(t *testing.T) {
	m := fakeMemory{ptrs: map[uintptr]uintptr{}, ints: map[uintptr]int32{}, uints: map[uintptr]uint64{}, patterns: map[string][]uintptr{}}
	writeClass(m, "WrongClass", 0x200000, 0x300000)
	writeClass(m, "PlayerSaveData", 0x210000, 0x310000)
	r := NewResolver(nil)
	r.ImportCache(ResolverCache{ClassCache: map[string][]uintptr{"PlayerSaveData": {0x300000}}})
	classes, ok := r.resolveClassByName(m, "PlayerSaveData")
	if !ok || len(classes) != 1 || classes[0] != 0x310000 {
		t.Fatalf("classes=%v ok=%t", classes, ok)
	}
}

func TestRuntimeTradeSlotsReadCurrentNamedLayout(t *testing.T) {
	m := &countingMemory{fakeMemory: fakeMemory{ptrs: map[uintptr]uintptr{}, ints: map[uintptr]int32{}, uints: map[uintptr]uint64{}, patterns: map[string][]uintptr{}}}
	writeClass(m.fakeMemory, "TradingStashSlot", 0x210000, 0x310000, 0x400000)
	writeClass(m.fakeMemory, "TradingStashCache", 0x220000, 0x320000)
	m.ptrs[0x500000] = 0x320000
	m.ptrs[0x400010] = 0x700000
	m.ints[0x4000A8] = 2
	m.ptrs[0x400118] = 0x500000
	m.ptrs[0x4000F8] = 0x600000
	now := time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)
	m.uints[0x400100] = dotnetDateData(now.Add(time.Hour))
	r := NewResolver(nil)
	r.fieldOffsets = map[uintptr]map[string]uintptr{0x310000: {"Index": 0xA8, "m_coolTimeText": 0xF8, "bkkz": 0x100, "bklc": 0x118}}
	got := r.readRuntimeTradeSlots(m, now, 1)
	if len(got) != 1 || got[0].Index != 2 || !got[0].CooldownUntil.Equal(now.Add(time.Hour)) {
		t.Fatalf("named layout slots=%v", got)
	}
	m.scans = 0
	m.uints[0x400100] = dotnetDateData(now.Add(2 * time.Hour))
	got = r.readRuntimeTradeSlots(m, now.Add(time.Minute), 1)
	if len(got) != 1 || !got[0].CooldownUntil.Equal(now.Add(2*time.Hour)) || m.scans != 0 {
		t.Fatalf("warm named slots=%v scans=%d", got, m.scans)
	}
}

func TestAbsentRuntimeTradeUIUsesLongerDiscoveryBackoff(t *testing.T) {
	m := &countingMemory{fakeMemory: fakeMemory{ptrs: map[uintptr]uintptr{}, ints: map[uintptr]int32{}, uints: map[uintptr]uint64{}, patterns: map[string][]uintptr{}}}
	writeClass(m.fakeMemory, "TradingStashSlot", 0x210000, 0x310000)
	writeClass(m.fakeMemory, "TradingStashCache", 0x220000, 0x320000)
	r := NewResolver(nil)
	now := time.Now()
	r.readRuntimeTradeSlots(m, now, 10)
	m.scans = 0
	r.readRuntimeTradeSlots(m, now.Add(6*time.Second), 10)
	if m.scans != 0 {
		t.Fatalf("absent UI rescanned too soon: %d", m.scans)
	}
	r.readRuntimeTradeSlots(m, now.Add(31*time.Second), 10)
	if m.scans != 1 {
		t.Fatalf("absent UI did not retry: %d", m.scans)
	}
}

func TestCurrentTradeSaveDoesNotDecodeItemIDAsDeadline(t *testing.T) {
	m := fakeMemory{ptrs: map[uintptr]uintptr{}, ints: map[uintptr]int32{}, uints: map[uintptr]uint64{}, patterns: map[string][]uintptr{}}
	writeClass(m, "RemakeTradingStashSaveData", 0x210000, 0x310000, 0x400000)
	writeList(m, 0x500000, 0x510000, 1)
	m.ptrs[0x510020] = 0x400000
	m.ints[0x400010] = 2
	m.ints[0x400020] = 1
	m.uints[0x400018] = 600000000000000000 // Unique ID, never a date.
	r := NewResolver(nil)
	r.fieldOffsets = map[uintptr]map[string]uintptr{0x310000: {"Index": 0x10, "ItemUniqueId": 0x18, "IsUnLock": 0x20}}
	got := r.readTradeSlotsWithLayout(m, readListInfo(m, 0x500000, 100), defaultSaveDataLayout())
	if len(got) != 1 || got[0].Index != 2 || got[0].State != 1 || !got[0].CooldownUntil.IsZero() {
		t.Fatalf("current save slots=%v; item ID was interpreted as a deadline", got)
	}
}

func TestWarmSnapshotsDoNotRescanForLockedTradeBackingSlots(t *testing.T) {
	m := &countingMemory{fakeMemory: fakeMemory{ptrs: map[uintptr]uintptr{}, ints: map[uintptr]int32{}, uints: map[uintptr]uint64{}, patterns: map[string][]uintptr{}}}
	writeClass(m.fakeMemory, "PlayerSaveData", 0x200000, 0x300000, 0x400000)
	writeClass(m.fakeMemory, "InventorySaveData", 0x210000, 0x310000)
	writeClass(m.fakeMemory, "RuneSaveData", 0x220000, 0x320000)
	writeClass(m.fakeMemory, "TradingStashSlot", 0x230000, 0x330000, 0x600000)
	writeClass(m.fakeMemory, "TradingStashCache", 0x240000, 0x340000)
	writeCandidateSaveData(m.fakeMemory, 0x400000, 0x500000, 100, 12345)
	writeList(m.fakeMemory, 0x700000, 0x710000, 10)
	m.ptrs[0x400000+playerTradeSlots] = 0x700000
	now := time.Now()
	for i := 0; i < 10; i++ {
		object := uintptr(0x720000 + i*0x100)
		m.ptrs[0x710000+0x20+uintptr(i*8)] = object
		m.ints[object+0x10] = int32(i)
		m.uints[object+0x18] = 0
		m.ints[object+0x20] = 0
	}
	m.ints[0x720020] = 1 // Only one unlocked slot is instantiated in the UI.
	m.ptrs[0x600010] = 0x800000
	m.ints[0x600000+runtimeTradeSlotIndex] = 0
	m.ptrs[0x600000+runtimeTradeSlotCache] = 0x610000
	m.ptrs[0x610000] = 0x340000
	m.ptrs[0x600000+runtimeTradeSlotCountdownText] = 0x620000
	m.uints[0x600000+runtimeTradeSlotCooldownUntil] = dotnetDateData(now.Add(time.Hour))
	r := NewResolver(nil)
	if _, ok := r.ReadSnapshot(m, now); !ok {
		t.Fatal("initial snapshot unavailable")
	}
	m.scans = 0
	if _, ok := r.ReadSnapshot(m, now.Add(time.Minute)); !ok {
		t.Fatal("warm snapshot unavailable")
	}
	if m.scans != 0 {
		t.Fatalf("locked backing slots caused %d rediscovery scans", m.scans)
	}
}
