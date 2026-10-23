package playerdata

import "testing"

func TestMaterialStashReadsQuantitiesWithoutDuplicateItems(t *testing.T) {
	mem := regionTestMemory()
	player, list, array, stack := uintptr(0x400000), uintptr(0x500000), uintptr(0x510000), uintptr(0x520000)
	mem.ptrs[player], mem.ptrs[stack], mem.ptrs[player+0xA0] = 0x300000, 0x310000, list
	writeList(mem, list, array, 1)
	mem.ptrs[array+0x20] = stack
	mem.ints[stack+0x10], mem.uints[stack+0x18], mem.ints[stack+0x20] = 110001, 1234, 25
	r := NewResolver(map[int]ItemMetadata{110001: {Marketable: true}})
	r.fieldOffsets = map[uintptr]map[string]uintptr{0x300000: {"materialStashSaveDatas": 0xA0}, 0x310000: {"ItemKey": 0x10, "ItemUniqueId": 0x18, "Quantity": 0x20}}
	seen := map[uint64]struct{}{}
	items, ok := r.readMaterialStash(mem, player, seen)
	if !ok || len(items) != 1 || items[0].Units() != 25 || items[0].Location != LocationMaterialStash || !items[0].Marketable {
		t.Fatalf("items=%+v ok=%t", items, ok)
	}
	if items, ok = r.readMaterialStash(mem, player, seen); !ok || len(items) != 0 {
		t.Fatal("duplicate stack was included")
	}
	delete(mem.ints, stack+0x20)
	if _, ok = r.readMaterialStash(mem, player, map[uint64]struct{}{}); ok {
		t.Fatal("unreadable quantity was accepted as an empty stash")
	}
	mem.ints[stack+0x20] = 25
	delete(mem.ptrs, array+0x20)
	if _, ok = r.readMaterialStash(mem, player, map[uint64]struct{}{}); ok {
		t.Fatal("unreadable stack pointer was accepted as an empty stash")
	}
}
