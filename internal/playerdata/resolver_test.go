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
	if snapshot.Items[0].Location != LocationEquipped || snapshot.Items[0].ItemID != 300 {
		t.Fatalf("first item = %+v, want equipped item 300", snapshot.Items[0])
	}
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
