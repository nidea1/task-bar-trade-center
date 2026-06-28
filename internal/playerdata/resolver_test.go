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

func (memory fakeMemory) ScanPattern(pattern []byte, _ int) ([]uintptr, uint64) {
	return memory.patterns[string(pattern)], 0
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

func writeHero(memory fakeMemory, array uintptr, index int, hero uintptr, key int32, unique uint64) {
	equipped := uintptr(0x800000)
	memory.ptrs[array+il2cpp.ArrayDataOffset+uintptr(index*8)] = hero
	memory.ints[hero+heroKey] = key
	memory.ptrs[hero+heroEquippedItems] = equipped
	memory.ptrs[equipped+il2cpp.ArrayMaxOffset] = 1
	memory.uints[equipped+il2cpp.ArrayDataOffset] = unique
}

func writeCurrency(memory fakeMemory, array uintptr, index int, obj uintptr, key int32, value uint64) {
	memory.ptrs[array+il2cpp.ArrayDataOffset+uintptr(index*8)] = obj
	memory.ints[obj+currencyKey] = key
	memory.uints[obj+currencyQuantity] = value
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
	mem.ints[slot0Obj+0x20] = 1                  // state (active)

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

