package playerdata

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/il2cpp"
)

type fakeMemory struct {
	ptrs     map[uintptr]uintptr
	ints     map[uintptr]int32
	uints    map[uintptr]uint64
	patterns map[string][]uintptr
}

func (memory fakeMemory) ReadUintptr(address uintptr) (uintptr, bool) {
	value, ok := memory.ptrs[address]
	return value, ok
}

func (memory fakeMemory) ReadUint64(address uintptr) (uint64, bool) {
	value, ok := memory.uints[address]
	return value, ok
}

func (memory fakeMemory) ReadInt32(address uintptr) (int32, bool) {
	value, ok := memory.ints[address]
	return value, ok
}

func (memory fakeMemory) ReadBytes(address uintptr, destination []byte) bool {
	for index := range destination {
		value, ok := memory.ints[address+uintptr(index)]
		if !ok {
			return false
		}
		destination[index] = byte(value)
	}
	return true
}

func (memory fakeMemory) ScanPattern(pattern []byte, _ int) ([]uintptr, uint64) {
	return memory.patterns[string(pattern)], 0
}

func TestDefaultSaveDataLayoutMatchesCurrentGameLayout(t *testing.T) {
	layout := defaultSaveDataLayout()
	if layout.currenciesOffset != 0x58 || layout.heroesOffset != 0x60 ||
		layout.inventoryOffset != 0x88 || layout.stashOffset != 0x90 ||
		layout.tradeSlotsOffset != 0x98 || layout.itemsOffset != 0xB0 {
		t.Fatalf("PlayerSaveData offsets = %+v, want July 2026 layout", layout)
	}
	if layout.heroEquippedItems != 0x30 {
		t.Fatalf("HeroSaveData equipped items offset = 0x%X, want 0x30", layout.heroEquippedItems)
	}
}

func TestResolverReadsPlayerSaveDataSnapshot(t *testing.T) {
	nameAddr := uintptr(0x200000)
	classPtr := uintptr(0x300000)
	object := uintptr(0x400000)
	mem := fakeMemory{
		ptrs:     make(map[uintptr]uintptr),
		ints:     make(map[uintptr]int32),
		uints:    make(map[uintptr]uint64),
		patterns: make(map[string][]uintptr),
	}

	mem.patterns["PlayerSaveData\x00"] = []uintptr{nameAddr}
	mem.patterns[pointerPattern(nameAddr)] = []uintptr{classPtr + il2cpp.ClassNameOffset}
	mem.patterns[pointerPattern(classPtr)] = []uintptr{object}
	mem.ptrs[classPtr+il2cpp.ClassNameOffset] = nameAddr
	mem.ptrs[classPtr+il2cpp.ClassElementClassOffset] = classPtr
	mem.ptrs[classPtr+il2cpp.ClassCastClassOffset] = classPtr

	itemsList, itemsArray := uintptr(0x500000), uintptr(0x510000)
	stashList, stashArray := uintptr(0x520000), uintptr(0x530000)
	heroesList, heroesArray := uintptr(0x540000), uintptr(0x550000)
	currencyList, currencyArray := uintptr(0x560000), uintptr(0x570000)
	mem.ptrs[object+playerItems] = itemsList
	mem.ptrs[object+playerStash] = stashList
	mem.ptrs[object+playerHeroes] = heroesList
	mem.ptrs[object+playerCurrencies] = currencyList
	writeList(mem, itemsList, itemsArray, 3)
	writeList(mem, stashList, stashArray, 2)
	writeList(mem, heroesList, heroesArray, 1)
	writeList(mem, currencyList, currencyArray, 1)

	writeItem(mem, itemsArray, 0, 100, 1000)
	writeItem(mem, itemsArray, 1, 200, 2000)
	writeItem(mem, itemsArray, 2, 300, 3000)
	writeSlot(mem, stashArray, 0, 0x610000, 1000)
	writeSlot(mem, stashArray, 1, 0x620000, 2000)
	writeHero(mem, heroesArray, 0, 0x630000, 601, 3000)
	writeCurrency(mem, currencyArray, 0, 0x640000, goldKey, 999)

	resolver := NewResolver(map[int]ItemMetadata{
		100: {Marketable: true},
		200: {Marketable: false},
		300: {Marketable: true},
	})
	snapshot, ok := resolver.ReadSnapshot(mem, time.Unix(1700000000, 0))
	if !ok {
		t.Fatal("expected snapshot")
	}
	if snapshot.Gold != 999 {
		t.Fatalf("gold = %d, want 999", snapshot.Gold)
	}
	if len(snapshot.Items) != 3 {
		t.Fatalf("items = %+v, want 3", snapshot.Items)
	}
	equipped, ok := findOwnedItem(snapshot.Items, 300)
	if !ok || equipped.Location != LocationEquipped {
		t.Fatalf("equipped item = %+v, ok=%v, want item 300 equipped", equipped, ok)
	}
}

func TestResolverPrefersSlotLocationOverStaleEquippedReference(t *testing.T) {
	nameAddr := uintptr(0x210000)
	classPtr := uintptr(0x310000)
	object := uintptr(0x410000)
	mem := fakeMemory{
		ptrs:     make(map[uintptr]uintptr),
		ints:     make(map[uintptr]int32),
		uints:    make(map[uintptr]uint64),
		patterns: make(map[string][]uintptr),
	}

	mem.patterns["PlayerSaveData\x00"] = []uintptr{nameAddr}
	mem.patterns[pointerPattern(nameAddr)] = []uintptr{classPtr + il2cpp.ClassNameOffset}
	mem.patterns[pointerPattern(classPtr)] = []uintptr{object}
	mem.ptrs[classPtr+il2cpp.ClassNameOffset] = nameAddr
	mem.ptrs[classPtr+il2cpp.ClassElementClassOffset] = classPtr
	mem.ptrs[classPtr+il2cpp.ClassCastClassOffset] = classPtr

	itemsList, itemsArray := uintptr(0x580000), uintptr(0x590000)
	inventoryList, inventoryArray := uintptr(0x5A0000), uintptr(0x5B0000)
	heroesList, heroesArray := uintptr(0x5C0000), uintptr(0x5D0000)
	mem.ptrs[object+playerItems] = itemsList
	mem.ptrs[object+playerInventory] = inventoryList
	mem.ptrs[object+playerHeroes] = heroesList
	writeList(mem, itemsList, itemsArray, 3)
	writeList(mem, inventoryList, inventoryArray, 1)
	writeList(mem, heroesList, heroesArray, 1)

	writeItem(mem, itemsArray, 0, 100, 1000)
	writeItem(mem, itemsArray, 1, 200, 2000)
	writeItem(mem, itemsArray, 2, 300, 3000)
	writeSlot(mem, inventoryArray, 0, 0x650000, 3000)
	writeHero(mem, heroesArray, 0, 0x660000, 601, 3000)

	resolver := NewResolver(map[int]ItemMetadata{
		100: {Marketable: true},
		200: {Marketable: true},
		300: {Marketable: true},
	})
	snapshot, ok := resolver.ReadSnapshot(mem, time.Unix(1700000000, 0))
	if !ok {
		t.Fatal("expected snapshot")
	}
	if len(snapshot.Items) != 1 {
		t.Fatalf("items = %+v, want only the current slot location", snapshot.Items)
	}
	item, ok := findOwnedItem(snapshot.Items, 300)
	if !ok || item.Location != LocationInventory {
		t.Fatalf("item = %+v, ok=%v, want inventory location", item, ok)
	}
}

func TestResolverReadsInventorySlotSummary(t *testing.T) {
	nameAddr := uintptr(0x215000)
	classPtr := uintptr(0x315000)
	object := uintptr(0x415000)
	mem := fakeMemory{
		ptrs:     make(map[uintptr]uintptr),
		ints:     make(map[uintptr]int32),
		uints:    make(map[uintptr]uint64),
		patterns: make(map[string][]uintptr),
	}
	writeClass(mem, "PlayerSaveData", nameAddr, classPtr, object)

	itemsList, itemsArray := uintptr(0x5E0000), uintptr(0x5F0000)
	inventoryList, inventoryArray := uintptr(0x600000), uintptr(0x601000)
	mem.ptrs[object+playerItems] = itemsList
	mem.ptrs[object+playerInventory] = inventoryList
	writeList(mem, itemsList, itemsArray, 3)
	writeList(mem, inventoryList, inventoryArray, 3)

	writeItem(mem, itemsArray, 0, 100, 1000)
	writeItem(mem, itemsArray, 1, 200, 2000)
	writeItem(mem, itemsArray, 2, 300, 3000)
	writeSlot(mem, inventoryArray, 0, 0x602000, 1000)

	resolver := NewResolver(map[int]ItemMetadata{
		100: {Marketable: true},
		200: {Marketable: true},
		300: {Marketable: true},
	})
	snapshot, ok := resolver.ReadSnapshotCore(mem, time.Unix(1700000000, 0))
	if !ok {
		t.Fatal("expected snapshot")
	}
	if snapshot.InventorySlots.Total != 3 || snapshot.InventorySlots.Used != 1 || snapshot.InventorySlots.Free != 2 {
		t.Fatalf("inventory slots = %+v, want total=3 used=1 free=2", snapshot.InventorySlots)
	}
	if snapshot.InventorySlots.EmptySlotRanges != "1-2" {
		t.Fatalf("empty slot ranges = %q, want 1-2", snapshot.InventorySlots.EmptySlotRanges)
	}
	if snapshot.InventorySlots.Source != "layout" {
		t.Fatalf("inventory slot source = %q, want layout", snapshot.InventorySlots.Source)
	}
}

func TestResolverCountsUnknownFilledInventorySlots(t *testing.T) {
	nameAddr := uintptr(0x215800)
	classPtr := uintptr(0x315800)
	object := uintptr(0x415800)
	mem := fakeMemory{
		ptrs:     make(map[uintptr]uintptr),
		ints:     make(map[uintptr]int32),
		uints:    make(map[uintptr]uint64),
		patterns: make(map[string][]uintptr),
	}
	writeClass(mem, "PlayerSaveData", nameAddr, classPtr, object)

	itemsList, itemsArray := uintptr(0x61A000), uintptr(0x61B000)
	inventoryList, inventoryArray := uintptr(0x61C000), uintptr(0x61D000)
	mem.ptrs[object+playerItems] = itemsList
	mem.ptrs[object+playerInventory] = inventoryList
	writeList(mem, itemsList, itemsArray, 4)
	writeList(mem, inventoryList, inventoryArray, 4)

	writeItem(mem, itemsArray, 0, 100, 1000)
	writeItem(mem, itemsArray, 1, 200, 2000)
	writeItem(mem, itemsArray, 2, 300, 3000)
	writeItem(mem, itemsArray, 3, 999, 9990)
	writeSlot(mem, inventoryArray, 0, 0x61E000, 1000)
	writeSlot(mem, inventoryArray, 1, 0x61F000, 9990)

	resolver := NewResolver(map[int]ItemMetadata{
		100: {Marketable: true},
		200: {Marketable: true},
		300: {Marketable: true},
	})
	snapshot, ok := resolver.ReadSnapshotCore(mem, time.Unix(1700000000, 0))
	if !ok {
		t.Fatal("expected snapshot")
	}
	if snapshot.InventorySlots.Total != 4 || snapshot.InventorySlots.Used != 2 || snapshot.InventorySlots.Free != 2 {
		t.Fatalf("inventory slots = %+v, want total=4 used=2 free=2", snapshot.InventorySlots)
	}
	if snapshot.InventorySlots.EmptySlotRanges != "2-3" {
		t.Fatalf("empty slot ranges = %q, want 2-3", snapshot.InventorySlots.EmptySlotRanges)
	}
	if len(snapshot.Items) != 1 {
		t.Fatalf("known inventory items = %+v, want only metadata-backed item exposed", snapshot.Items)
	}
}

func TestResolverUsesCurrentInventorySlotsOffsetInsteadOfStash(t *testing.T) {
	nameAddr := uintptr(0x215A00)
	classPtr := uintptr(0x315A00)
	object := uintptr(0x415A00)
	mem := fakeMemory{
		ptrs:     make(map[uintptr]uintptr),
		ints:     make(map[uintptr]int32),
		uints:    make(map[uintptr]uint64),
		patterns: make(map[string][]uintptr),
	}
	writeClass(mem, "PlayerSaveData", nameAddr, classPtr, object)

	itemsList, itemsArray := uintptr(0x620000), uintptr(0x621000)
	inventoryList, inventoryArray := uintptr(0x622000), uintptr(0x623000)
	stashList, stashArray := uintptr(0x624000), uintptr(0x625000)
	mem.ptrs[object+0x80] = inventoryList
	mem.ptrs[object+0x88] = stashList
	mem.ptrs[object+0xA8] = itemsList
	writeList(mem, itemsList, itemsArray, 3)
	writeList(mem, inventoryList, inventoryArray, 104)
	writeList(mem, stashList, stashArray, 260)

	writeItem(mem, itemsArray, 0, 100, 1000)
	writeItem(mem, itemsArray, 1, 200, 2000)
	writeItem(mem, itemsArray, 2, 300, 3000)
	writeSlot(mem, inventoryArray, 0, 0x626000, 1000)
	writeSlot(mem, stashArray, 0, 0x627000, 2000)

	resolver := NewResolver(map[int]ItemMetadata{
		100: {Marketable: true},
		200: {Marketable: true},
		300: {Marketable: true},
	})
	snapshot, ok := resolver.ReadSnapshotCore(mem, time.Unix(1700000000, 0))
	if !ok {
		t.Fatal("expected snapshot")
	}
	if snapshot.InventorySlots.Total != 104 {
		t.Fatalf("inventory total slots = %d, want 104", snapshot.InventorySlots.Total)
	}
	if snapshot.InventorySlots.Free != 103 {
		t.Fatalf("inventory free slots = %d, want 103", snapshot.InventorySlots.Free)
	}
	stashItem, ok := findOwnedItem(snapshot.Items, 200)
	if !ok || stashItem.Location != LocationStash {
		t.Fatalf("stash item = %+v, ok=%v, want item 200 in stash", stashItem, ok)
	}
	inventoryItem, ok := findOwnedItem(snapshot.Items, 100)
	if !ok || inventoryItem.Location != LocationInventory {
		t.Fatalf("inventory item = %+v, ok=%v, want item 100 in inventory", inventoryItem, ok)
	}
}

func TestResolverReadsInventorySlotsFromInventorySaveDataClass(t *testing.T) {
	nameAddr := uintptr(0x215C00)
	classPtr := uintptr(0x315C00)
	inventoryNameAddr := uintptr(0x216C00)
	inventoryClassPtr := uintptr(0x316C00)
	object := uintptr(0x415C00)
	mem := fakeMemory{
		ptrs:     make(map[uintptr]uintptr),
		ints:     make(map[uintptr]int32),
		uints:    make(map[uintptr]uint64),
		patterns: make(map[string][]uintptr),
	}
	writeClass(mem, "PlayerSaveData", nameAddr, classPtr, object)
	writeClass(mem, "InventorySaveData", inventoryNameAddr, inventoryClassPtr)

	itemsList, itemsArray := uintptr(0x620000), uintptr(0x621000)
	inventoryList, inventoryArray := uintptr(0x622000), uintptr(0x623000)
	mem.ptrs[object+playerItems] = itemsList
	mem.ptrs[object+0xB8] = inventoryList
	writeList(mem, itemsList, itemsArray, 3)
	writeList(mem, inventoryList, inventoryArray, 4)

	writeItem(mem, itemsArray, 0, 100, 1000)
	writeItem(mem, itemsArray, 1, 200, 2000)
	writeItem(mem, itemsArray, 2, 300, 3000)
	writeInventorySaveSlot(mem, inventoryArray, 0, 0x624000, inventoryClassPtr, 1000)
	writeInventorySaveSlot(mem, inventoryArray, 1, 0x625000, inventoryClassPtr, 0)
	writeInventorySaveSlot(mem, inventoryArray, 2, 0x626000, inventoryClassPtr, 3000)
	writeInventorySaveSlot(mem, inventoryArray, 3, 0x627000, inventoryClassPtr, 0)

	resolver := NewResolver(map[int]ItemMetadata{
		100: {Marketable: true},
		200: {Marketable: true},
		300: {Marketable: true},
	})
	snapshot, ok := resolver.ReadSnapshotCore(mem, time.Unix(1700000000, 0))
	if !ok {
		t.Fatal("expected snapshot")
	}
	if snapshot.InventorySlots.Total != 4 || snapshot.InventorySlots.Used != 2 || snapshot.InventorySlots.Free != 2 {
		t.Fatalf("inventory slots = %+v, want total=4 used=2 free=2", snapshot.InventorySlots)
	}
	if snapshot.InventorySlots.EmptySlotRanges != "1,3" {
		t.Fatalf("empty slot ranges = %q, want 1,3", snapshot.InventorySlots.EmptySlotRanges)
	}
	if snapshot.InventorySlots.Source != "InventorySaveData" {
		t.Fatalf("inventory slot source = %q, want InventorySaveData", snapshot.InventorySlots.Source)
	}
	if item, ok := findOwnedItem(snapshot.Items, 300); !ok || item.Location != LocationInventory {
		t.Fatalf("inventory item = %+v, ok=%v, want item 300 from InventorySaveData list", item, ok)
	}
}

func TestResolverCountsOnlyUnlockedInventorySaveSlots(t *testing.T) {
	nameAddr := uintptr(0x215D00)
	classPtr := uintptr(0x315D00)
	inventoryNameAddr := uintptr(0x216D00)
	inventoryClassPtr := uintptr(0x316D00)
	object := uintptr(0x415D00)
	mem := fakeMemory{
		ptrs:     make(map[uintptr]uintptr),
		ints:     make(map[uintptr]int32),
		uints:    make(map[uintptr]uint64),
		patterns: make(map[string][]uintptr),
	}
	writeClass(mem, "PlayerSaveData", nameAddr, classPtr, object)
	writeClass(mem, "InventorySaveData", inventoryNameAddr, inventoryClassPtr)

	itemsList, itemsArray := uintptr(0x628000), uintptr(0x629000)
	inventoryList, inventoryArray := uintptr(0x62A000), uintptr(0x62B000)
	mem.ptrs[object+playerItems] = itemsList
	mem.ptrs[object+playerInventory] = inventoryList
	writeList(mem, itemsList, itemsArray, 3)
	writeList(mem, inventoryList, inventoryArray, 260)

	writeItem(mem, itemsArray, 0, 100, 1000)
	writeItem(mem, itemsArray, 1, 200, 2000)
	writeItem(mem, itemsArray, 2, 300, 3000)
	for index := 0; index < 260; index++ {
		uniqueID := uint64(0)
		if index == 0 {
			uniqueID = 1000
		}
		writeInventorySaveSlotWithUnlock(mem, inventoryArray, index, 0x630000+uintptr(index*0x100), inventoryClassPtr, uniqueID, index < 104, false)
	}

	resolver := NewResolver(map[int]ItemMetadata{
		100: {Marketable: true},
		200: {Marketable: true},
		300: {Marketable: true},
	})
	snapshot, ok := resolver.ReadSnapshotCore(mem, time.Unix(1700000000, 0))
	if !ok {
		t.Fatal("expected snapshot")
	}
	if snapshot.InventorySlots.Total != 104 || snapshot.InventorySlots.BackingTotal != 260 {
		t.Fatalf("inventory totals = %+v, want total=104 backing_total=260", snapshot.InventorySlots)
	}
	if snapshot.InventorySlots.Used != 1 || snapshot.InventorySlots.Free != 103 {
		t.Fatalf("inventory slots = %+v, want used=1 free=103", snapshot.InventorySlots)
	}
	if snapshot.InventorySlots.EmptySlotRanges != "1-103" {
		t.Fatalf("empty slot ranges = %q, want 1-103", snapshot.InventorySlots.EmptySlotRanges)
	}
}

func TestResolverReadsChestAutomationRuneReductions(t *testing.T) {
	nameAddr := uintptr(0x216000)
	classPtr := uintptr(0x316000)
	runeNameAddr := uintptr(0x216800)
	runeClassPtr := uintptr(0x316800)
	object := uintptr(0x416000)
	mem := fakeMemory{
		ptrs:     make(map[uintptr]uintptr),
		ints:     make(map[uintptr]int32),
		uints:    make(map[uintptr]uint64),
		patterns: make(map[string][]uintptr),
	}
	writeClass(mem, "PlayerSaveData", nameAddr, classPtr, object)
	writeClass(mem, "RuneSaveData", runeNameAddr, runeClassPtr)

	itemsList, itemsArray := uintptr(0x603000), uintptr(0x604000)
	inventoryList, inventoryArray := uintptr(0x605000), uintptr(0x606000)
	runeList, runeArray := uintptr(0x607000), uintptr(0x608000)
	mem.ptrs[object+playerItems] = itemsList
	mem.ptrs[object+playerInventory] = inventoryList
	mem.ptrs[object+0xB8] = runeList
	writeList(mem, itemsList, itemsArray, 3)
	writeList(mem, inventoryList, inventoryArray, 1)
	writeList(mem, runeList, runeArray, 10)

	writeItem(mem, itemsArray, 0, 100, 1000)
	writeItem(mem, itemsArray, 1, 200, 2000)
	writeItem(mem, itemsArray, 2, 300, 3000)
	writeSlot(mem, inventoryArray, 0, 0x609000, 1000)
	writeRuneSave(mem, runeArray, 0, 0x60A000, runeClassPtr, 130021, 3)
	writeRuneSave(mem, runeArray, 1, 0x60B000, runeClassPtr, 190301, 5)
	writeRuneSave(mem, runeArray, 2, 0x60C000, runeClassPtr, 190501, 5)
	writeRuneSave(mem, runeArray, 3, 0x60D000, runeClassPtr, 1905011, 5)
	writeRuneSave(mem, runeArray, 4, 0x60E000, runeClassPtr, 150011, 3)
	writeRuneSave(mem, runeArray, 5, 0x60F000, runeClassPtr, 1905021, 5)
	writeRuneSave(mem, runeArray, 6, 0x610000, runeClassPtr, 190502, 5)
	writeRuneSave(mem, runeArray, 7, 0x611000, runeClassPtr, 190302, 5)
	writeRuneSave(mem, runeArray, 8, 0x612000, runeClassPtr, 13002, 1)
	writeRuneSave(mem, runeArray, 9, 0x613000, runeClassPtr, 15001, 1)

	resolver := NewResolver(map[int]ItemMetadata{
		100: {Marketable: true},
		200: {Marketable: true},
		300: {Marketable: true},
	})
	snapshot, ok := resolver.ReadSnapshotCore(mem, time.Unix(1700000000, 0))
	if !ok {
		t.Fatal("expected snapshot")
	}
	if snapshot.ChestAutomation.CommonRuneReductionSeconds != 39 {
		t.Fatalf("common reduction = %d, want 39", snapshot.ChestAutomation.CommonRuneReductionSeconds)
	}
	if snapshot.ChestAutomation.StageRuneReductionSeconds != 75 {
		t.Fatalf("stage reduction = %d, want 75", snapshot.ChestAutomation.StageRuneReductionSeconds)
	}
	if !snapshot.ChestAutomation.RuneResolved {
		t.Fatalf("rune reductions were not fully resolved: %+v", snapshot.ChestAutomation.Diagnostics)
	}
	if !snapshot.ChestAutomation.HasCommonAutoOpenRune {
		t.Fatal("expected HasCommonAutoOpenRune to be true")
	}
	if !snapshot.ChestAutomation.HasStageAutoOpenRune {
		t.Fatal("expected HasStageAutoOpenRune to be true")
	}
	wantCommonBuffs := "130021 lv3 -9s,190301 lv5 -10s,190501 lv5 -10s,1905011 lv5 -10s"
	if snapshot.ChestAutomation.CommonReductionBuffSummary != wantCommonBuffs {
		t.Fatalf("common buff summary = %q, want %q", snapshot.ChestAutomation.CommonReductionBuffSummary, wantCommonBuffs)
	}
	wantStageBuffs := "150011 lv3 -15s,190302 lv5 -20s,190502 lv5 -20s,1905021 lv5 -20s"
	if snapshot.ChestAutomation.StageReductionBuffSummary != wantStageBuffs {
		t.Fatalf("stage buff summary = %q, want %q", snapshot.ChestAutomation.StageReductionBuffSummary, wantStageBuffs)
	}
	wantBuffs := "common:130021 lv3 -9s,190301 lv5 -10s,190501 lv5 -10s,1905011 lv5 -10s; stage:150011 lv3 -15s,190302 lv5 -20s,190502 lv5 -20s,1905021 lv5 -20s"
	if snapshot.ChestAutomation.ReductionBuffSummary != wantBuffs {
		t.Fatalf("buff summary = %q, want %q", snapshot.ChestAutomation.ReductionBuffSummary, wantBuffs)
	}
}

func TestResolverIgnoresNonRuneSaveDataStageOffset(t *testing.T) {
	nameAddr := uintptr(0x217000)
	classPtr := uintptr(0x317000)
	runeNameAddr := uintptr(0x217800)
	runeClassPtr := uintptr(0x317800)
	object := uintptr(0x417000)
	mem := fakeMemory{
		ptrs:     make(map[uintptr]uintptr),
		ints:     make(map[uintptr]int32),
		uints:    make(map[uintptr]uint64),
		patterns: make(map[string][]uintptr),
	}
	writeClass(mem, "PlayerSaveData", nameAddr, classPtr, object)
	writeClass(mem, "RuneSaveData", runeNameAddr, runeClassPtr)

	itemsList, itemsArray := uintptr(0x612000), uintptr(0x613000)
	inventoryList, inventoryArray := uintptr(0x614000), uintptr(0x615000)
	legacyLikeList, legacyLikeArray := uintptr(0x616000), uintptr(0x617000)
	legacyLikeRune := uintptr(0x618000)
	mem.ptrs[object+playerItems] = itemsList
	mem.ptrs[object+playerInventory] = inventoryList
	mem.ptrs[object+0xB8] = legacyLikeList
	writeList(mem, itemsList, itemsArray, 3)
	writeList(mem, inventoryList, inventoryArray, 1)
	writeList(mem, legacyLikeList, legacyLikeArray, 1)

	writeItem(mem, itemsArray, 0, 100, 1000)
	writeItem(mem, itemsArray, 1, 200, 2000)
	writeItem(mem, itemsArray, 2, 300, 3000)
	writeSlot(mem, inventoryArray, 0, 0x619000, 1000)
	mem.ptrs[legacyLikeArray+il2cpp.ArrayDataOffset] = legacyLikeRune
	mem.ints[legacyLikeRune+0x10] = 130021
	mem.ints[legacyLikeRune+0x18] = 3

	resolver := NewResolver(map[int]ItemMetadata{
		100: {Marketable: true},
		200: {Marketable: true},
		300: {Marketable: true},
	})
	snapshot, ok := resolver.ReadSnapshotCore(mem, time.Unix(1700000000, 0))
	if !ok {
		t.Fatal("expected snapshot")
	}
	if snapshot.ChestAutomation.CommonRuneReductionSeconds != 0 {
		t.Fatalf("common reduction = %d, want 0", snapshot.ChestAutomation.CommonRuneReductionSeconds)
	}
	if snapshot.ChestAutomation.RuneResolved {
		t.Fatalf("rune reductions resolved from non-RuneSaveData shape: %+v", snapshot.ChestAutomation.Diagnostics)
	}
}

func TestResolverUsesSaveSlotIndexForStashPages(t *testing.T) {
	nameAddr := uintptr(0x220000)
	classPtr := uintptr(0x320000)
	object := uintptr(0x420000)
	mem := fakeMemory{
		ptrs:     make(map[uintptr]uintptr),
		ints:     make(map[uintptr]int32),
		uints:    make(map[uintptr]uint64),
		patterns: make(map[string][]uintptr),
	}

	mem.patterns["PlayerSaveData\x00"] = []uintptr{nameAddr}
	mem.patterns[pointerPattern(nameAddr)] = []uintptr{classPtr + il2cpp.ClassNameOffset}
	mem.patterns[pointerPattern(classPtr)] = []uintptr{object}
	mem.ptrs[classPtr+il2cpp.ClassNameOffset] = nameAddr
	mem.ptrs[classPtr+il2cpp.ClassElementClassOffset] = classPtr
	mem.ptrs[classPtr+il2cpp.ClassCastClassOffset] = classPtr

	itemsList, itemsArray := uintptr(0x670000), uintptr(0x680000)
	stashList, stashArray := uintptr(0x690000), uintptr(0x6A0000)
	mem.ptrs[object+playerItems] = itemsList
	mem.ptrs[object+playerStash] = stashList
	writeList(mem, itemsList, itemsArray, 3)
	writeList(mem, stashList, stashArray, 2)

	writeItem(mem, itemsArray, 0, 100, 1000)
	writeItem(mem, itemsArray, 1, 200, 2000)
	writeItem(mem, itemsArray, 2, 300, 3000)
	writeSlotWithIndex(mem, stashArray, 0, 0x6B0000, 1000, 401)
	writeSlotWithIndex(mem, stashArray, 1, 0x6C0000, 2000, 650)

	resolver := NewResolver(map[int]ItemMetadata{
		100: {Marketable: true},
		200: {Marketable: true},
		300: {Marketable: true},
	})
	snapshot, ok := resolver.ReadSnapshot(mem, time.Unix(1700000000, 0))
	if !ok {
		t.Fatal("expected snapshot")
	}
	first, ok := findOwnedItem(snapshot.Items, 100)
	if !ok || first.SlotIndex != 401 {
		t.Fatalf("first stash item = %+v, ok=%v, want slot index 401", first, ok)
	}
	second, ok := findOwnedItem(snapshot.Items, 200)
	if !ok || second.SlotIndex != 650 {
		t.Fatalf("second stash item = %+v, ok=%v, want slot index 650", second, ok)
	}
}

func TestResolverKeepsListSlotIndexWhenSaveIndexIsNotSparse(t *testing.T) {
	nameAddr := uintptr(0x230000)
	classPtr := uintptr(0x330000)
	object := uintptr(0x430000)
	mem := fakeMemory{
		ptrs:     make(map[uintptr]uintptr),
		ints:     make(map[uintptr]int32),
		uints:    make(map[uintptr]uint64),
		patterns: make(map[string][]uintptr),
	}

	mem.patterns["PlayerSaveData\x00"] = []uintptr{nameAddr}
	mem.patterns[pointerPattern(nameAddr)] = []uintptr{classPtr + il2cpp.ClassNameOffset}
	mem.patterns[pointerPattern(classPtr)] = []uintptr{object}
	mem.ptrs[classPtr+il2cpp.ClassNameOffset] = nameAddr
	mem.ptrs[classPtr+il2cpp.ClassElementClassOffset] = classPtr
	mem.ptrs[classPtr+il2cpp.ClassCastClassOffset] = classPtr

	itemsList, itemsArray := uintptr(0x6D0000), uintptr(0x6E0000)
	stashList, stashArray := uintptr(0x6F0000), uintptr(0x710000)
	mem.ptrs[object+playerItems] = itemsList
	mem.ptrs[object+playerStash] = stashList
	writeList(mem, itemsList, itemsArray, 3)
	writeList(mem, stashList, stashArray, 451)

	writeItem(mem, itemsArray, 0, 100, 1000)
	writeItem(mem, itemsArray, 1, 200, 2000)
	writeItem(mem, itemsArray, 2, 300, 3000)
	writeSlotWithIndex(mem, stashArray, 450, 0x720000, 1000, 1)

	resolver := NewResolver(map[int]ItemMetadata{
		100: {Marketable: true},
		200: {Marketable: true},
		300: {Marketable: true},
	})
	snapshot, ok := resolver.ReadSnapshot(mem, time.Unix(1700000000, 0))
	if !ok {
		t.Fatal("expected snapshot")
	}
	item, ok := findOwnedItem(snapshot.Items, 100)
	if !ok || item.SlotIndex != 450 {
		t.Fatalf("stash item = %+v, ok=%v, want list slot index 450", item, ok)
	}
}

func TestResolverPrefersHighestGoldPlayerSaveDataCandidate(t *testing.T) {
	nameAddr := uintptr(0x240000)
	classPtr := uintptr(0x340000)
	staleObject := uintptr(0x440000)
	liveObject := uintptr(0x450000)
	mem := fakeMemory{
		ptrs:     make(map[uintptr]uintptr),
		ints:     make(map[uintptr]int32),
		uints:    make(map[uintptr]uint64),
		patterns: make(map[string][]uintptr),
	}

	mem.patterns["PlayerSaveData\x00"] = []uintptr{nameAddr}
	mem.patterns[pointerPattern(nameAddr)] = []uintptr{classPtr + il2cpp.ClassNameOffset}
	mem.patterns[pointerPattern(classPtr)] = []uintptr{staleObject, liveObject}
	mem.ptrs[classPtr+il2cpp.ClassNameOffset] = nameAddr
	mem.ptrs[classPtr+il2cpp.ClassElementClassOffset] = classPtr
	mem.ptrs[classPtr+il2cpp.ClassCastClassOffset] = classPtr

	writeCandidateSaveData(mem, staleObject, 0x730000, 100, 100)
	writeCandidateSaveData(mem, liveObject, 0x830000, 200, 500)

	resolver := NewResolver(map[int]ItemMetadata{
		100: {Marketable: true},
		101: {Marketable: true},
		102: {Marketable: true},
		200: {Marketable: true},
		201: {Marketable: true},
		202: {Marketable: true},
	})
	snapshot, ok := resolver.ReadSnapshot(mem, time.Unix(1700000000, 0))
	if !ok {
		t.Fatal("expected snapshot")
	}
	if snapshot.Gold != 500 {
		t.Fatalf("gold = %d, want live candidate gold 500", snapshot.Gold)
	}
	if item, ok := findOwnedItem(snapshot.Items, 200); !ok || item.Location != LocationStash {
		t.Fatalf("live stash item = %+v, ok=%v, want item 200 from highest-gold candidate", item, ok)
	}
}

func TestResolverUsesDistinctHeroKeysWhenOldHeroKeyFieldIsShared(t *testing.T) {
	nameAddr := uintptr(0x260000)
	classPtr := uintptr(0x360000)
	object := uintptr(0x470000)
	mem := fakeMemory{
		ptrs:     make(map[uintptr]uintptr),
		ints:     make(map[uintptr]int32),
		uints:    make(map[uintptr]uint64),
		patterns: make(map[string][]uintptr),
	}
	writeClass(mem, "PlayerSaveData", nameAddr, classPtr, object)

	itemsList, itemsArray := uintptr(0xA00000), uintptr(0xA01000)
	heroesList, heroesArray := uintptr(0xA02000), uintptr(0xA03000)
	mem.ptrs[object+playerItems] = itemsList
	mem.ptrs[object+playerHeroes] = heroesList
	writeList(mem, itemsList, itemsArray, 3)
	writeList(mem, heroesList, heroesArray, 3)

	writeItem(mem, itemsArray, 0, 100, 1000)
	writeItem(mem, itemsArray, 1, 200, 2000)
	writeItem(mem, itemsArray, 2, 300, 3000)
	writeHeroWithOldAndActualKey(mem, heroesArray, 0, 0xA10000, 1, 101, 1000)
	writeHeroWithOldAndActualKey(mem, heroesArray, 1, 0xA11000, 1, 201, 2000)
	writeHeroWithOldAndActualKey(mem, heroesArray, 2, 0xA12000, 1, 301, 3000)

	resolver := NewResolver(map[int]ItemMetadata{
		100: {Marketable: true},
		200: {Marketable: true},
		300: {Marketable: true},
	})
	snapshot, ok := resolver.ReadSnapshotCore(mem, time.Unix(1700000000, 0))
	if !ok {
		t.Fatal("expected snapshot")
	}

	for itemID, wantHero := range map[int]int{100: 101, 200: 201, 300: 301} {
		item, ok := findOwnedItem(snapshot.Items, itemID)
		if !ok || item.Location != LocationEquipped || item.EquippedHeroKey != wantHero {
			t.Fatalf("item %d = %+v, ok=%v, want equipped hero %d", itemID, item, ok, wantHero)
		}
	}
}

func TestResolverDiscoversShiftedSaveDataLayout(t *testing.T) {
	nameAddr := uintptr(0x250000)
	classPtr := uintptr(0x350000)
	object := uintptr(0x460000)
	mem := fakeMemory{
		ptrs:     make(map[uintptr]uintptr),
		ints:     make(map[uintptr]int32),
		uints:    make(map[uintptr]uint64),
		patterns: make(map[string][]uintptr),
	}
	writeClass(mem, "PlayerSaveData", nameAddr, classPtr, object)

	shifted := saveDataLayout{
		itemsOffset:            0x38,
		stashOffset:            0x48,
		inventoryOffset:        0x58,
		heroesOffset:           0x68,
		currenciesOffset:       0x78,
		itemKeyOffset:          0x24,
		itemUniqueIDOffset:     0x30,
		slotIndexOffset:        0x28,
		slotUniqueIDOffset:     0x40,
		heroKeyOffset:          0x34,
		heroEquippedItems:      0x50,
		currencyKeyOffset:      0x2C,
		currencyQuantityOffset: 0x38,
	}

	itemsList, itemsArray := uintptr(0x900000), uintptr(0x901000)
	stashList, stashArray := uintptr(0x902000), uintptr(0x903000)
	inventoryList, inventoryArray := uintptr(0x904000), uintptr(0x905000)
	heroesList, heroesArray := uintptr(0x906000), uintptr(0x907000)
	currencyList, currencyArray := uintptr(0x908000), uintptr(0x909000)
	mem.ptrs[object+shifted.itemsOffset] = itemsList
	mem.ptrs[object+shifted.stashOffset] = stashList
	mem.ptrs[object+shifted.inventoryOffset] = inventoryList
	mem.ptrs[object+shifted.heroesOffset] = heroesList
	mem.ptrs[object+shifted.currenciesOffset] = currencyList
	writeList(mem, itemsList, itemsArray, 4)
	writeList(mem, stashList, stashArray, 2)
	writeList(mem, inventoryList, inventoryArray, 1)
	writeList(mem, heroesList, heroesArray, 1)
	writeList(mem, currencyList, currencyArray, 1)

	writeItemWithOffsets(mem, itemsArray, 0, 0x910000, shifted.itemKeyOffset, shifted.itemUniqueIDOffset, 100, 1000)
	writeItemWithOffsets(mem, itemsArray, 1, 0x911000, shifted.itemKeyOffset, shifted.itemUniqueIDOffset, 200, 2000)
	writeItemWithOffsets(mem, itemsArray, 2, 0x912000, shifted.itemKeyOffset, shifted.itemUniqueIDOffset, 300, 3000)
	writeItemWithOffsets(mem, itemsArray, 3, 0x913000, shifted.itemKeyOffset, shifted.itemUniqueIDOffset, 400, 4000)
	writeSlotWithOffsets(mem, stashArray, 0, 0x920000, shifted.slotIndexOffset, shifted.slotUniqueIDOffset, 250, 1000)
	writeSlotWithOffsets(mem, stashArray, 1, 0x921000, shifted.slotIndexOffset, shifted.slotUniqueIDOffset, 251, 2000)
	writeSlotWithOffsets(mem, inventoryArray, 0, 0x922000, shifted.slotIndexOffset, shifted.slotUniqueIDOffset, 0, 3000)
	writeHeroWithOffsets(mem, heroesArray, 0, 0x930000, shifted.heroKeyOffset, shifted.heroEquippedItems, 601, 4000)
	writeCurrencyWithOffsets(mem, currencyArray, 0, 0x940000, shifted.currencyKeyOffset, shifted.currencyQuantityOffset, goldKey, 777)

	resolver := NewResolver(map[int]ItemMetadata{
		100: {Marketable: true},
		200: {Marketable: true},
		300: {Marketable: true},
		400: {Marketable: true},
	})
	snapshot, ok := resolver.ReadSnapshotCore(mem, time.Unix(1700000000, 0))
	if !ok {
		t.Fatal("expected snapshot from discovered layout")
	}
	if snapshot.Gold != 777 {
		t.Fatalf("gold = %d, want 777", snapshot.Gold)
	}
	if len(snapshot.Items) != 4 {
		t.Fatalf("items = %+v, want 4", snapshot.Items)
	}
	stashItem, ok := findOwnedItem(snapshot.Items, 100)
	if !ok || stashItem.Location != LocationStash || stashItem.SlotIndex != 250 {
		t.Fatalf("stash item = %+v, ok=%v, want item 100 in shifted stash slot 250", stashItem, ok)
	}
	inventoryItem, ok := findOwnedItem(snapshot.Items, 300)
	if !ok || inventoryItem.Location != LocationInventory {
		t.Fatalf("inventory item = %+v, ok=%v, want item 300 in shifted inventory", inventoryItem, ok)
	}
	equippedItem, ok := findOwnedItem(snapshot.Items, 400)
	if !ok || equippedItem.Location != LocationEquipped || equippedItem.EquippedHeroKey != 601 {
		t.Fatalf("equipped item = %+v, ok=%v, want item 400 equipped by hero 601", equippedItem, ok)
	}
}

func TestResolverPrefersRuntimeElementClassesOverLargerFalseItemList(t *testing.T) {
	object := uintptr(0x480000)
	mem := fakeMemory{
		ptrs:     make(map[uintptr]uintptr),
		ints:     make(map[uintptr]int32),
		uints:    make(map[uintptr]uint64),
		patterns: make(map[string][]uintptr),
	}
	writeClass(mem, "PlayerSaveData", 0x270000, 0x370000, object)

	falseList, falseArray := uintptr(0xA20000), uintptr(0xA21000)
	itemsList, itemsArray := uintptr(0xA22000), uintptr(0xA23000)
	stashList, stashArray := uintptr(0xA24000), uintptr(0xA25000)
	inventoryList, inventoryArray := uintptr(0xA26000), uintptr(0xA27000)
	currencyList, currencyArray := uintptr(0xA28000), uintptr(0xA29000)
	mem.ptrs[object+playerItems] = falseList
	mem.ptrs[object+0xC0] = itemsList
	mem.ptrs[object+0xD0] = stashList
	mem.ptrs[object+0xD8] = inventoryList
	mem.ptrs[object+0xE0] = currencyList
	writeList(mem, falseList, falseArray, 5)
	writeList(mem, itemsList, itemsArray, 3)
	writeList(mem, stashList, stashArray, 1)
	writeList(mem, inventoryList, inventoryArray, 1)
	writeList(mem, currencyList, currencyArray, 1)

	falseObjects := []uintptr{0xA30000, 0xA30100, 0xA30200, 0xA30300, 0xA30400}
	itemObjects := []uintptr{0xA31000, 0xA31100, 0xA31200}
	stashSlot := uintptr(0xA32000)
	inventorySlot := uintptr(0xA32100)
	currencyObject := uintptr(0xA33000)
	writeClass(mem, "AttributeSaveData", 0x271000, 0x371000, falseObjects...)
	writeClass(mem, "ItemSaveData", 0x272000, 0x372000, itemObjects...)
	writeClass(mem, "StashSaveData", 0x273000, 0x373000, stashSlot)
	writeClass(mem, "InventorySaveData", 0x274000, 0x374000, inventorySlot)
	writeClass(mem, "CurrencySaveData", 0x275000, 0x375000, currencyObject)
	mem.ints[0x371000+il2cpp.ClassInstanceSizeOffset] = 0x20
	mem.ints[0x372000+il2cpp.ClassInstanceSizeOffset] = 0x38
	mem.ints[0x373000+il2cpp.ClassInstanceSizeOffset] = 0x48
	mem.ints[0x374000+il2cpp.ClassInstanceSizeOffset] = 0x48
	mem.ints[0x375000+il2cpp.ClassInstanceSizeOffset] = 0x40

	for index, entry := range falseObjects {
		writeItemWithOffsets(mem, falseArray, index, entry, 0x10, 0x18, int32(100+index), uint64(1000+index))
	}
	for index, entry := range itemObjects {
		writeItemWithOffsets(mem, itemsArray, index, entry, 0x24, 0x30, int32(200+index), uint64(2000+index))
	}
	writeSlotWithOffsets(mem, stashArray, 0, stashSlot, 0x28, 0x40, 250, 2000)
	writeSlotWithOffsets(mem, inventoryArray, 0, inventorySlot, 0x28, 0x40, 0, 0)
	writeCurrencyWithOffsets(mem, currencyArray, 0, currencyObject, 0x2C, 0x38, goldKey, 777)

	resolver := NewResolver(map[int]ItemMetadata{
		100: {}, 101: {}, 102: {}, 103: {}, 104: {},
		200: {Marketable: true}, 201: {Marketable: true}, 202: {Marketable: true},
	})
	snapshot, ok := resolver.ReadSnapshotCore(mem, time.Unix(1700000000, 0))
	if !ok {
		t.Fatal("expected class-aware snapshot")
	}
	item, ok := findOwnedItem(snapshot.Items, 200)
	if !ok || item.Location != LocationStash || item.SlotIndex != 250 {
		t.Fatalf("stash item = %+v, ok=%v, want class-identified ItemSaveData item", item, ok)
	}
	if snapshot.Gold != 777 {
		t.Fatalf("gold = %d, want 777", snapshot.Gold)
	}
	cache := resolver.ExportCache()
	if cache.Layout.ItemsOffset != 0xC0 || cache.Layout.StashOffset != 0xD0 || cache.Layout.InventoryOffset != 0xD8 {
		t.Fatalf("class-aware layout = %+v", cache.Layout)
	}
	if cache.Layout.ItemKeyOffset != 0x24 || cache.Layout.ItemUniqueIDOffset != 0x30 ||
		cache.Layout.SlotIndexOffset != 0x28 || cache.Layout.SlotUniqueIDOffset != 0x40 {
		t.Fatalf("instance-bounded field layout = %+v", cache.Layout)
	}
}

func findOwnedItem(items []OwnedItem, itemID int) (OwnedItem, bool) {
	for _, item := range items {
		if item.ItemID == itemID {
			return item, true
		}
	}
	return OwnedItem{}, false
}

func pointerPattern(address uintptr) string {
	bytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(bytes, uint64(address))
	return string(bytes)
}

func writeClass(memory fakeMemory, name string, nameAddr uintptr, classPtr uintptr, objects ...uintptr) {
	memory.patterns[name+"\x00"] = []uintptr{nameAddr}
	for index := 0; index < 96; index++ {
		memory.ints[nameAddr+uintptr(index)] = 0
	}
	for index, value := range []byte(name) {
		memory.ints[nameAddr+uintptr(index)] = int32(value)
	}
	memory.patterns[pointerPattern(nameAddr)] = []uintptr{classPtr + il2cpp.ClassNameOffset}
	memory.ptrs[classPtr+il2cpp.ClassNameOffset] = nameAddr
	memory.ptrs[classPtr+il2cpp.ClassElementClassOffset] = classPtr
	memory.ptrs[classPtr+il2cpp.ClassCastClassOffset] = classPtr
	if len(objects) == 0 {
		return
	}
	memory.patterns[pointerPattern(classPtr)] = objects
	for _, object := range objects {
		memory.ptrs[object] = classPtr
	}
}

func dotnetDateData(value time.Time) uint64 {
	const ticksAtUnixEpoch = uint64(621355968000000000)
	const ticksPerSecond = uint64(10000000)

	return (uint64(value.Unix())*ticksPerSecond + ticksAtUnixEpoch) | 0x4000000000000000
}

func writeList(memory fakeMemory, list uintptr, array uintptr, size int) {
	memory.ptrs[list+il2cpp.ListItemsOffset] = array
	memory.ints[list+il2cpp.ListSizeOffset] = int32(size)
	memory.ptrs[array+il2cpp.ArrayMaxOffset] = uintptr(size)
}

func writeItem(memory fakeMemory, array uintptr, index int, key int32, unique uint64) {
	obj := uintptr(0x700000 + index*0x1000)
	memory.ptrs[array+il2cpp.ArrayDataOffset+uintptr(index*8)] = obj
	memory.ints[obj+itemSaveItemKey] = key
	memory.uints[obj+itemSaveUniqueID] = unique
}

func writeSlot(memory fakeMemory, array uintptr, index int, slot uintptr, unique uint64) {
	memory.ptrs[array+il2cpp.ArrayDataOffset+uintptr(index*8)] = slot
	memory.uints[slot+slotUniqueID] = unique
}

func writeSlotWithIndex(memory fakeMemory, array uintptr, index int, slot uintptr, unique uint64, savedIndex int32) {
	writeSlot(memory, array, index, slot, unique)
	memory.ints[slot+slotIndex] = savedIndex
}

func writeInventorySaveSlot(memory fakeMemory, array uintptr, index int, slot uintptr, classPtr uintptr, unique uint64) {
	writeSlot(memory, array, index, slot, unique)
	memory.ptrs[slot] = classPtr
}

func writeInventorySaveSlotWithUnlock(memory fakeMemory, array uintptr, index int, slot uintptr, classPtr uintptr, unique uint64, unlocked bool, unlockedByRune bool) {
	writeInventorySaveSlot(memory, array, index, slot, classPtr, unique)
	memory.ints[slot+slotIndex] = int32(index)
	if unlocked {
		memory.ints[slot+slotIsUnlock] = 1
	} else {
		memory.ints[slot+slotIsUnlock] = 0
	}
	if unlockedByRune {
		memory.ints[slot+slotIsRuneUnlock] = 1
	} else {
		memory.ints[slot+slotIsRuneUnlock] = 0
	}
}

func writeHero(memory fakeMemory, array uintptr, index int, hero uintptr, key int32, unique uint64) {
	equipped := uintptr(0x800000)
	memory.ptrs[array+il2cpp.ArrayDataOffset+uintptr(index*8)] = hero
	memory.ints[hero+heroKey] = key
	memory.ptrs[hero+heroEquippedItems] = equipped
	memory.ptrs[equipped+il2cpp.ArrayMaxOffset] = 1
	memory.uints[equipped+il2cpp.ArrayDataOffset] = unique
}

func writeHeroWithOldAndActualKey(memory fakeMemory, array uintptr, index int, hero uintptr, oldKey int32, actualKey int32, unique uint64) {
	equipped := hero + 0x1000
	memory.ptrs[array+il2cpp.ArrayDataOffset+uintptr(index*8)] = hero
	memory.ints[hero+heroKey] = oldKey
	memory.ints[hero+0x3C] = actualKey
	memory.ptrs[hero+heroEquippedItems] = equipped
	memory.ptrs[equipped+il2cpp.ArrayMaxOffset] = 1
	memory.uints[equipped+il2cpp.ArrayDataOffset] = unique
}

func writeCurrency(memory fakeMemory, array uintptr, index int, obj uintptr, key int32, value uint64) {
	memory.ptrs[array+il2cpp.ArrayDataOffset+uintptr(index*8)] = obj
	memory.ints[obj+currencyKey] = key
	memory.uints[obj+currencyQuantity] = value
}

func writeRuneSave(memory fakeMemory, array uintptr, index int, obj uintptr, classPtr uintptr, key int32, stage int32) {
	memory.ptrs[array+il2cpp.ArrayDataOffset+uintptr(index*8)] = obj
	memory.ptrs[obj] = classPtr
	memory.ints[obj+0x10] = key
	memory.ints[obj+0x14] = stage
}

func writeCandidateSaveData(memory fakeMemory, object uintptr, base uintptr, firstItemID int32, gold uint64) {
	itemsList, itemsArray := base, base+0x1000
	stashList, stashArray := base+0x2000, base+0x3000
	currencyList, currencyArray := base+0x4000, base+0x5000
	memory.ptrs[object+playerItems] = itemsList
	memory.ptrs[object+playerStash] = stashList
	memory.ptrs[object+playerCurrencies] = currencyList
	writeList(memory, itemsList, itemsArray, 3)
	writeList(memory, stashList, stashArray, 1)
	writeList(memory, currencyList, currencyArray, 1)

	writeItem(memory, itemsArray, 0, firstItemID, uint64(firstItemID)*10)
	writeItem(memory, itemsArray, 1, firstItemID+1, uint64(firstItemID+1)*10)
	writeItem(memory, itemsArray, 2, firstItemID+2, uint64(firstItemID+2)*10)
	writeSlot(memory, stashArray, 0, base+0x6000, uint64(firstItemID)*10)
	writeCurrency(memory, currencyArray, 0, base+0x7000, goldKey, gold)
}

func writeItemWithOffsets(memory fakeMemory, array uintptr, index int, obj uintptr, keyOffset uintptr, uniqueOffset uintptr, key int32, unique uint64) {
	memory.ptrs[array+il2cpp.ArrayDataOffset+uintptr(index*8)] = obj
	memory.ints[obj+keyOffset] = key
	memory.uints[obj+uniqueOffset] = unique
}

func writeSlotWithOffsets(memory fakeMemory, array uintptr, index int, slot uintptr, indexOffset uintptr, uniqueOffset uintptr, savedIndex int32, unique uint64) {
	memory.ptrs[array+il2cpp.ArrayDataOffset+uintptr(index*8)] = slot
	memory.ints[slot+indexOffset] = savedIndex
	memory.uints[slot+uniqueOffset] = unique
}

func writeHeroWithOffsets(memory fakeMemory, array uintptr, index int, hero uintptr, keyOffset uintptr, equippedOffset uintptr, key int32, unique uint64) {
	equipped := hero + 0x1000
	memory.ptrs[array+il2cpp.ArrayDataOffset+uintptr(index*8)] = hero
	memory.ints[hero+keyOffset] = key
	memory.ptrs[hero+equippedOffset] = equipped
	memory.ptrs[equipped+il2cpp.ArrayMaxOffset] = 1
	memory.uints[equipped+il2cpp.ArrayDataOffset] = unique
}

func writeCurrencyWithOffsets(memory fakeMemory, array uintptr, index int, obj uintptr, keyOffset uintptr, quantityOffset uintptr, key int32, value uint64) {
	memory.ptrs[array+il2cpp.ArrayDataOffset+uintptr(index*8)] = obj
	memory.ints[obj+keyOffset] = key
	memory.uints[obj+quantityOffset] = value
}

func TestResolverReadsTradeSlots(t *testing.T) {
	nameAddr := uintptr(0x200000)
	classPtr := uintptr(0x300000)
	object := uintptr(0x400000)
	mem := fakeMemory{
		ptrs:     make(map[uintptr]uintptr),
		ints:     make(map[uintptr]int32),
		uints:    make(map[uintptr]uint64),
		patterns: make(map[string][]uintptr),
	}

	mem.patterns["PlayerSaveData\x00"] = []uintptr{nameAddr}
	mem.patterns[pointerPattern(nameAddr)] = []uintptr{classPtr + il2cpp.ClassNameOffset}
	mem.patterns[pointerPattern(classPtr)] = []uintptr{object}
	mem.ptrs[classPtr+il2cpp.ClassNameOffset] = nameAddr
	mem.ptrs[classPtr+il2cpp.ClassElementClassOffset] = classPtr
	mem.ptrs[classPtr+il2cpp.ClassCastClassOffset] = classPtr

	itemsList, itemsArray := uintptr(0x500000), uintptr(0x510000)
	stashList, stashArray := uintptr(0x520000), uintptr(0x530000)
	tradeList, tradeArray := uintptr(0x580000), uintptr(0x590000)
	mem.ptrs[object+playerItems] = itemsList
	mem.ptrs[object+playerStash] = stashList
	mem.ptrs[object+playerTradeSlots] = tradeList
	writeList(mem, itemsList, itemsArray, 3)
	writeList(mem, stashList, stashArray, 1)
	writeList(mem, tradeList, tradeArray, 2)

	writeItem(mem, itemsArray, 0, 100, 1000)
	writeItem(mem, itemsArray, 1, 200, 2000)
	writeItem(mem, itemsArray, 2, 300, 3000)
	writeSlot(mem, stashArray, 0, 0x610000, 1000)

	// Write mock trade slots:
	// Slot 1: index 0, state 1, cooldownRaw = 503987864624647652
	slot0Obj := uintptr(0x650000)
	mem.ptrs[tradeArray+il2cpp.ArrayDataOffset+0] = slot0Obj
	mem.ints[slot0Obj+0x10] = 0 // index
	// actualTicks = cooldownRaw + 135194695325352348 = 639182559950000000
	// 639182559950000000 is June 28, 2026 15:06:35 UTC
	mem.uints[slot0Obj+0x18] = 503987864624647652 // cooldownUntil raw ticks
	mem.ints[slot0Obj+0x20] = 1                   // state (active)

	// Slot 2: index 1, state 0, cooldownRaw = 0
	slot1Obj := uintptr(0x660000)
	mem.ptrs[tradeArray+il2cpp.ArrayDataOffset+8] = slot1Obj
	mem.ints[slot1Obj+0x10] = 1 // index
	mem.uints[slot1Obj+0x18] = 0
	mem.ints[slot1Obj+0x20] = 0 // state (inactive)

	resolver := NewResolver(map[int]ItemMetadata{
		100: {Marketable: true},
		200: {Marketable: false},
		300: {Marketable: true},
	})
	snapshot, ok := resolver.ReadSnapshot(mem, time.Unix(1700000000, 0))
	if !ok {
		t.Fatal("expected snapshot")
	}

	if len(snapshot.TradeSlots) != 2 {
		t.Fatalf("len(TradeSlots) = %d, want 2", len(snapshot.TradeSlots))
	}

	slot0 := snapshot.TradeSlots[0]
	if slot0.Index != 0 || slot0.State != 1 {
		t.Fatalf("slot0 = %+v, want index 0 state 1", slot0)
	}
	expectedTime := time.Date(2026, 6, 28, 15, 6, 35, 0, time.UTC)
	if !slot0.CooldownUntil.Equal(expectedTime) {
		t.Fatalf("slot0.CooldownUntil = %v, want %v", slot0.CooldownUntil, expectedTime)
	}

	slot1 := snapshot.TradeSlots[1]
	if slot1.Index != 1 || slot1.State != 0 || !slot1.CooldownUntil.IsZero() {
		t.Fatalf("slot1 = %+v, want index 1 state 0 cooldown zero", slot1)
	}
}

func TestResolverPrefersRuntimeTradeSlotCooldowns(t *testing.T) {
	nameAddr := uintptr(0x200000)
	classPtr := uintptr(0x300000)
	object := uintptr(0x400000)
	slotNameAddr := uintptr(0x210000)
	slotClassPtr := uintptr(0x310000)
	cacheNameAddr := uintptr(0x220000)
	cacheClassPtr := uintptr(0x320000)
	mem := fakeMemory{
		ptrs:     make(map[uintptr]uintptr),
		ints:     make(map[uintptr]int32),
		uints:    make(map[uintptr]uint64),
		patterns: make(map[string][]uintptr),
	}

	writeClass(mem, "PlayerSaveData", nameAddr, classPtr, object)
	writeClass(mem, "TradingStashSlot", slotNameAddr, slotClassPtr)
	writeClass(mem, "TradingStashCache", cacheNameAddr, cacheClassPtr)

	itemsList, itemsArray := uintptr(0x500000), uintptr(0x510000)
	stashList, stashArray := uintptr(0x520000), uintptr(0x530000)
	tradeList, tradeArray := uintptr(0x580000), uintptr(0x590000)
	mem.ptrs[object+playerItems] = itemsList
	mem.ptrs[object+playerStash] = stashList
	mem.ptrs[object+playerTradeSlots] = tradeList
	writeList(mem, itemsList, itemsArray, 3)
	writeList(mem, stashList, stashArray, 1)
	writeList(mem, tradeList, tradeArray, 1)
	writeItem(mem, itemsArray, 0, 100, 1000)
	writeItem(mem, itemsArray, 1, 200, 2000)
	writeItem(mem, itemsArray, 2, 300, 3000)
	writeSlot(mem, stashArray, 0, 0x610000, 1000)

	saveSlot := uintptr(0x650000)
	mem.ptrs[tradeArray+il2cpp.ArrayDataOffset] = saveSlot
	mem.ints[saveSlot+0x10] = 0
	mem.uints[saveSlot+0x18] = 503987864624647652
	mem.ints[saveSlot+0x20] = 1

	runtimeSlot0 := uintptr(0x670000)
	runtimeSlot1 := uintptr(0x671000)
	cache0 := uintptr(0x680000)
	cache1 := uintptr(0x681000)
	mem.patterns[pointerPattern(slotClassPtr)] = []uintptr{runtimeSlot0, runtimeSlot1}
	mem.ptrs[runtimeSlot0] = slotClassPtr
	mem.ptrs[runtimeSlot0+0x10] = 0x700000
	mem.ptrs[runtimeSlot1] = slotClassPtr
	mem.ptrs[runtimeSlot1+0x10] = 0x701000
	mem.ptrs[cache0] = cacheClassPtr
	mem.ptrs[cache1] = cacheClassPtr

	expected0 := time.Date(2026, 6, 29, 15, 27, 29, 0, time.UTC)
	expected1 := time.Date(2026, 6, 29, 15, 27, 32, 0, time.UTC)
	mem.ints[runtimeSlot0+runtimeTradeSlotIndex] = 0
	mem.ptrs[runtimeSlot0+runtimeTradeSlotCountdownText] = 0x690000
	mem.uints[runtimeSlot0+runtimeTradeSlotCooldownUntil] = dotnetDateData(expected0)
	mem.ptrs[runtimeSlot0+runtimeTradeSlotCache] = cache0
	mem.ints[runtimeSlot1+runtimeTradeSlotIndex] = 1
	mem.ptrs[runtimeSlot1+runtimeTradeSlotCountdownText] = 0x690100
	mem.uints[runtimeSlot1+runtimeTradeSlotCooldownUntil] = dotnetDateData(expected1)
	mem.ptrs[runtimeSlot1+runtimeTradeSlotCache] = cache1

	resolver := NewResolver(map[int]ItemMetadata{
		100: {Marketable: true},
		200: {Marketable: false},
		300: {Marketable: true},
	})
	snapshot, ok := resolver.ReadSnapshot(mem, time.Date(2026, 6, 29, 9, 0, 0, 0, time.UTC))
	if !ok {
		t.Fatal("expected snapshot")
	}

	if len(snapshot.TradeSlots) != 2 {
		t.Fatalf("len(TradeSlots) = %d, want 2", len(snapshot.TradeSlots))
	}
	if snapshot.TradeSlots[0].Index != 0 || snapshot.TradeSlots[0].State != 1 || !snapshot.TradeSlots[0].CooldownUntil.Equal(expected0) {
		t.Fatalf("slot0 = %+v, want runtime cooldown %v", snapshot.TradeSlots[0], expected0)
	}
	if snapshot.TradeSlots[1].Index != 1 || snapshot.TradeSlots[1].State != 1 || !snapshot.TradeSlots[1].CooldownUntil.Equal(expected1) {
		t.Fatalf("slot1 = %+v, want runtime cooldown %v", snapshot.TradeSlots[1], expected1)
	}
}

func TestResolverCacheImportExport(t *testing.T) {
	resolver := NewResolver(map[int]ItemMetadata{
		100: {Marketable: true},
		101: {Marketable: false},
		102: {Marketable: true},
	})

	// Setup mock data
	nameAddr := uintptr(0x200000)
	classPtr := uintptr(0x300000)
	object := uintptr(0x400000)
	mem := fakeMemory{
		ptrs:     make(map[uintptr]uintptr),
		ints:     make(map[uintptr]int32),
		uints:    make(map[uintptr]uint64),
		patterns: make(map[string][]uintptr),
	}

	// Setup classes and candidate save data
	writeClass(mem, "PlayerSaveData", nameAddr, classPtr, object)
	writeCandidateSaveData(mem, object, 0x500000, 100, 12345)

	// First scan to populate the cache
	snapshot, ok := resolver.ReadSnapshot(mem, time.Unix(1700000000, 0))
	if !ok {
		t.Fatal("expected snapshot")
	}
	if snapshot.Gold != 12345 {
		t.Fatalf("snapshot.Gold = %d, want 12345", snapshot.Gold)
	}

	// Export the cache
	cache := resolver.ExportCache()
	if cache.CachedObject != object {
		t.Fatalf("cache.CachedObject = 0x%X, want 0x%X", cache.CachedObject, object)
	}
	if cache.Layout.ItemsOffset != playerItems {
		t.Fatalf("cache.Layout.ItemsOffset = 0x%X, want 0x%X", cache.Layout.ItemsOffset, playerItems)
	}
	if len(cache.ClassCache) == 0 {
		t.Fatal("expected ClassCache to be populated")
	}

	// Import the cache into a new resolver
	resolver2 := NewResolver(map[int]ItemMetadata{
		100: {Marketable: true},
		101: {Marketable: false},
		102: {Marketable: true},
	})
	resolver2.ImportCache(cache)

	// We clear the memory patterns so that a full scan would FAIL.
	// If it successfully reads, it must have used the cachedObject and Layout directly!
	memNoPatterns := fakeMemory{
		ptrs:     mem.ptrs,
		ints:     mem.ints,
		uints:    mem.uints,
		patterns: make(map[string][]uintptr), // NO PATTERNS for scanning!
	}

	snapshot2, ok := resolver2.ReadSnapshot(memNoPatterns, time.Unix(1700000000, 0))
	if !ok {
		t.Fatal("expected snapshot2 from cache")
	}
	if snapshot2.Gold != 12345 {
		t.Fatalf("snapshot2.Gold = %d, want 12345", snapshot2.Gold)
	}

	// Now we test cache invalidation/fallback:
	// We corrupt the memory at cachedObject (e.g. set itemsList ptr to 0)
	// so validation fails.
	memCorrupted := fakeMemory{
		ptrs:     make(map[uintptr]uintptr),
		ints:     make(map[uintptr]int32),
		uints:    make(map[uintptr]uint64),
		patterns: make(map[string][]uintptr),
	}
	// Copy over some pointers but NOT the valid structure at object
	for k, v := range mem.ptrs {
		if k != object+playerItems {
			memCorrupted.ptrs[k] = v
		}
	}
	// Add valid PlayerSaveData patterns so it can fall back to a full scan and succeed there.
	newObject := uintptr(0x900000)
	writeClass(memCorrupted, "PlayerSaveData", nameAddr, classPtr, newObject)
	writeCandidateSaveData(memCorrupted, newObject, 0x950000, 100, 99999)

	// Try reading snapshot. It should:
	// 1. Try reading from cachedObject (0x400000) -> fails because itemsList is missing/corrupted.
	// 2. Clear cachedObject and rediscover instances using validated classes.
	// 3. Perform a full scan and resolve newObject (0x900000) -> succeeds!
	snapshot3, ok := resolver2.ReadSnapshot(memCorrupted, time.Unix(1700000000, 0))
	if !ok {
		t.Fatal("expected snapshot3 after fallback scan")
	}
	if snapshot3.Gold != 99999 {
		t.Fatalf("snapshot3.Gold = %d, want 99999", snapshot3.Gold)
	}
	if resolver2.cachedObject != newObject {
		t.Fatalf("resolver2.cachedObject = 0x%X, want 0x%X", resolver2.cachedObject, newObject)
	}
}
