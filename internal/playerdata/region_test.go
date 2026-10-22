package playerdata

import (
	"testing"
	"time"
)

func regionTestMemory() fakeMemory {
	return fakeMemory{ptrs: map[uintptr]uintptr{}, ints: map[uintptr]int32{}, uints: map[uintptr]uint64{}, patterns: map[string][]uintptr{}}
}

func writeNamedFields(memory fakeMemory, class, array uintptr, fields map[string]uintptr) {
	memory.ptrs[class+0x80] = array
	memory.ints[class+0xF8] = 512
	index := uintptr(0)
	for name, offset := range fields {
		entry := array + index*32
		nameAddress := array + 0x1000 + index*128
		memory.ptrs[entry] = nameAddress
		memory.ptrs[entry+16] = class
		memory.ints[entry+24] = int32(offset)
		for i := 0; i < 96; i++ {
			memory.ints[nameAddress+uintptr(i)] = 0
		}
		for i, b := range []byte(name) {
			memory.ints[nameAddress+uintptr(i)] = int32(b)
		}
		index++
	}
}

func TestRegionFollowsCurrentStageUsingMovedNamedFields(t *testing.T) {
	mem := regionTestMemory()
	player, common, normal, plague, boss := uintptr(0x100000), uintptr(0x110000), uintptr(0x120000), uintptr(0x130000), uintptr(0x140000)
	pc, cc, sc := uintptr(0x200000), uintptr(0x210000), uintptr(0x220000)
	mem.ptrs[player], mem.ptrs[common] = pc, cc
	mem.ptrs[player+0x28] = common
	writeNamedFields(mem, pc, 0x300000, map[string]uintptr{"commonSaveData": 0x28})
	writeNamedFields(mem, cc, 0x310000, map[string]uintptr{"currentStageKey": 0x84})
	writeClass(mem, "StageInfoData", 0x500000, sc, normal, plague, boss)
	writeNamedFields(mem, sc, 0x320000, map[string]uintptr{"StageKey": 0x48, "STAGETYPE": 0x60})
	mem.ints[normal+0x48], mem.ints[normal+0x60] = 1201, 0
	mem.ints[plague+0x48], mem.ints[plague+0x60] = 9001, 2
	mem.ints[boss+0x48], mem.ints[boss+0x60] = 9002, 3
	r := NewResolver(nil)
	now := time.Now()
	for _, tt := range []struct {
		key    int32
		region Region
	}{{1201, RegionNormal}, {9001, RegionPlaguelands}, {9002, RegionPlaguelands}, {1201, RegionNormal}, {9999, ""}} {
		mem.ints[common+0x84] = tt.key
		got, key := r.readRegion(mem, player, now)
		if got != tt.region || key != int(tt.key) {
			t.Fatalf("stage %d: region=%s key=%d want=%s", tt.key, got, key, tt.region)
		}
	}
	mem.ptrs[player+0x28] = 0
	if got, _ := r.readRegion(mem, player, now); got != "" {
		t.Fatalf("missing common save data returned stale region %s", got)
	}
}

func TestPlagueChestAutomationUsesSeparateAccountStats(t *testing.T) {
	mem := regionTestMemory()
	object, dict, entries := uintptr(0x100000), uintptr(0x110000), uintptr(0x120000)
	writeClass(mem, "AccountStatus", 0x300000, 0x200000, object)
	writeClass(mem, "Dictionary`2", 0x310000, 0x210000, dict)
	mem.ptrs[object+16] = dict
	mem.ptrs[dict+24] = entries
	mem.ints[dict+32] = 4
	mem.ptrs[entries+24] = 4
	for i, value := range []int32{420, 39, 720, 75} {
		a := entries + 32 + uintptr(i)*28
		key := uint32(0x67890123)
		mem.ints[a], mem.ints[a+8] = int32(i), int32(45+i)
		mem.ints[a+16] = int32((uint32(value) ^ key) + key)
		mem.ints[a+20] = int32(key)
	}
	a, ok := readPlagueAccountStatus(mem, object)
	if !ok || !a.RuneResolved || !a.HasCommonAutoOpenRune || !a.HasStageAutoOpenRune || a.CommonBaseAutoOpenSeconds != 420 || a.StageBaseAutoOpenSeconds != 720 || a.CommonRuneReductionSeconds != 39 || a.StageRuneReductionSeconds != 75 {
		t.Fatalf("automation=%+v ok=%t", a, ok)
	}
	mem.ints[entries+32+16] = mem.ints[entries+32+20] * 2
	a, ok = readPlagueAccountStatus(mem, object)
	if !ok || a.HasCommonAutoOpenRune || !a.HasStageAutoOpenRune {
		t.Fatalf("locked plague chest incorrectly inherited an unlock: %+v", a)
	}
	mem.ints[dict+32] = 257
	if _, ok := readPlagueAccountStatus(mem, object); ok {
		t.Fatal("accepted corrupt dictionary count")
	}
}

func TestPlaguelandsNightmareHellTormentStages(t *testing.T) {
	mem := regionTestMemory()
	player, common, class := uintptr(0x100000), uintptr(0x110000), uintptr(0x220000)
	mem.ptrs[player], mem.ptrs[common] = 0x200000, 0x210000
	mem.ptrs[player+0x10] = common
	r := NewResolver(nil)
	r.fieldOffsets = map[uintptr]map[string]uintptr{
		0x200000: {"commonSaveData": 0x10}, 0x210000: {"currentStageKey": 0x64}, class: {"StageKey": 0x30, "STAGETYPE": 0x40},
	}
	var stages []uintptr
	for _, first := range []int32{201201, 201301, 201401} {
		for key := first; key < first+20; key++ {
			object := uintptr(0x400000 + len(stages)*0x100)
			mem.ints[object+0x30], mem.ints[object+0x40] = key, 2
			stages = append(stages, object)
		}
	}
	writeClass(mem, "StageInfoData", 0x600000, class, stages...)
	for _, first := range []int32{201201, 201301, 201401} {
		for key := first; key < first+20; key++ {
			mem.ints[common+0x64] = key
			if region, _ := r.readRegion(mem, player, time.Now()); region != RegionPlaguelands {
				t.Fatalf("stage=%d region=%s want PlagueLands", key, region)
			}
		}
	}
}
