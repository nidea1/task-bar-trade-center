package playerdata

import (
	"strings"
	"testing"
	"time"
)

func plagueRuneMemory(levels map[int32]int32) (fakeMemory, uintptr) {
	mem := regionTestMemory()
	player, class := uintptr(0x400000), uintptr(0x310000)
	list, array := uintptr(0x600000), uintptr(0x610000)
	writeClass(mem, "RuneSaveData", 0x210000, class)
	mem.ptrs[player+0xB8] = list
	writeList(mem, list, array, len(levels))
	index := 0
	for key, level := range levels {
		writeRuneSave(mem, array, index, 0x620000+uintptr(index)*0x100, class, key, level)
		index++
	}
	return mem, player
}

func TestPlagueChestRuneLevels(t *testing.T) {
	for _, tt := range []struct {
		name                  string
		levels                map[int32]int32
		common, stage         int
		commonOpen, stageOpen bool
	}{
		{"locked", map[int32]int32{1163: 0, 1165: 0, 11631: 0, 11651: 0}, 0, 0, false, false},
		{"unlock only", map[int32]int32{1163: 1, 1165: 1}, 0, 0, true, true},
		{"partial", map[int32]int32{1163: 1, 1165: 1, 11631: 2, 11632: 3, 11633: 1, 11651: 1, 11652: 2, 11653: 4}, 24, 56, true, true},
		{"max", map[int32]int32{1163: 1, 1165: 1, 11631: 5, 11632: 5, 11633: 5, 11651: 5, 11652: 5, 11653: 5}, 60, 120, true, true},
		{"independent unlocks", map[int32]int32{1163: 1, 1165: 0, 11631: 5, 11651: 5}, 20, 40, true, false},
		{"invalid levels", map[int32]int32{1163: 1, 1165: 1, 11631: -1, 11632: 6, 11651: 6, 11652: -1}, 0, 0, true, true},
		{"unrelated runes", map[int32]int32{1163: 1, 1165: 1, 130021: 3, 150011: 3, 1905011: 5, 1905021: 5, 11671: 5, 11672: 5, 11673: 5}, 0, 0, true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mem, player := plagueRuneMemory(tt.levels)
			got := NewResolver(nil).readPlagueChestAutomation(mem, player, time.Now())
			if !got.RuneResolved || got.CommonRuneReductionSeconds != tt.common || got.StageRuneReductionSeconds != tt.stage || got.HasCommonAutoOpenRune != tt.commonOpen || got.HasStageAutoOpenRune != tt.stageOpen {
				t.Fatalf("automation=%+v", got)
			}
			if got.CommonBaseAutoOpenSeconds != 600 || got.StageBaseAutoOpenSeconds != 1200 {
				t.Fatalf("Plague base durations=%d/%d, want 600/1200", got.CommonBaseAutoOpenSeconds, got.StageBaseAutoOpenSeconds)
			}
			if strings.Contains(got.ReductionBuffSummary, "1163 lv") || strings.Contains(got.ReductionBuffSummary, "1165 lv") {
				t.Fatalf("unlock counted as a reduction: %s", got.ReductionBuffSummary)
			}
		})
	}
}

func TestPlagueRuneFallbackDoesNotInheritNormalRunes(t *testing.T) {
	mem, player := plagueRuneMemory(map[int32]int32{130021: 3, 150011: 3, 1905011: 5, 1905021: 5})
	resolver := NewResolver(nil)
	got := resolver.readPlagueChestAutomation(mem, player, time.Now())
	if got.RuneResolved || got.HasCommonAutoOpenRune || got.HasStageAutoOpenRune || got.CommonRuneReductionSeconds != 0 || got.StageRuneReductionSeconds != 0 {
		t.Fatalf("normal runes leaked into Plague forecast: %+v", got)
	}
	mem, player = plagueRuneMemory(map[int32]int32{1163: 1, 1165: 1, 11631: 5, 11651: 5})
	got = NewResolver(nil).readChestAutomationWithLayout(mem, player, defaultSaveDataLayout())
	if got.RuneResolved || got.HasCommonAutoOpenRune || got.HasStageAutoOpenRune || got.CommonRuneReductionSeconds != 0 || got.StageRuneReductionSeconds != 0 {
		t.Fatalf("Plague runes leaked into normal forecast: %+v", got)
	}
}

func TestSnapshotSelectsChestRunesAfterRegionChanges(t *testing.T) {
	mem, player := plagueRuneMemory(map[int32]int32{
		13002: 1, 15001: 1, 130021: 3, 150011: 3,
		1163: 1, 1165: 1, 11631: 5, 11632: 5, 11633: 5, 11651: 5, 11652: 5, 11653: 5,
	})
	writeClass(mem, "PlayerSaveData", 0x200000, 0x300000, player)
	writeCandidateSaveData(mem, player, 0x800000, 100, 1000)
	common, normal, plague := uintptr(0x900000), uintptr(0x910000), uintptr(0x920000)
	mem.ptrs[player+0x10], mem.ptrs[common] = common, 0x340000
	writeClass(mem, "StageInfoData", 0x240000, 0x350000, normal, plague)
	mem.ints[normal+0x30], mem.ints[normal+0x40] = 1201, 0
	mem.ints[plague+0x30], mem.ints[plague+0x40] = 201201, 2
	resolver := NewResolver(map[int]ItemMetadata{100: {}, 101: {}, 102: {}})
	resolver.fieldOffsets = map[uintptr]map[string]uintptr{
		0x300000: {"commonSaveData": 0x10}, 0x340000: {"currentStageKey": 0x64},
		0x350000: {"StageKey": 0x30, "STAGETYPE": 0x40},
	}
	for _, tt := range []struct {
		stage        int32
		region       Region
		common, blue int
	}{{1201, RegionNormal, 9, 15}, {201201, RegionPlaguelands, 60, 120}, {1201, RegionNormal, 9, 15}} {
		mem.ints[common+0x64] = tt.stage
		got, ok := resolver.ReadSnapshotCore(mem, time.Now())
		if !ok || got.Region != tt.region || got.ChestAutomation.CommonRuneReductionSeconds != tt.common || got.ChestAutomation.StageRuneReductionSeconds != tt.blue {
			t.Fatalf("stage=%d ok=%t region=%s automation=%+v", tt.stage, ok, got.Region, got.ChestAutomation)
		}
	}
}

func TestPlagueAccountStatsAreNotAddedToRuneReductions(t *testing.T) {
	mem, player := plagueRuneMemory(map[int32]int32{1163: 1, 1165: 1, 11631: 5, 11632: 5, 11633: 5, 11651: 5, 11652: 5, 11653: 5})
	object, dict, entries := uintptr(0x700000), uintptr(0x710000), uintptr(0x720000)
	writeClass(mem, "AccountStatus", 0x220000, 0x320000, object)
	writeClass(mem, "Dictionary`2", 0x230000, 0x330000, dict)
	mem.ptrs[object+16], mem.ptrs[dict+24], mem.ptrs[entries+24] = dict, entries, 4
	mem.ints[dict+32] = 4
	for index, value := range []int32{600, 60, 1200, 120} {
		entry := entries + 32 + uintptr(index)*28
		key := uint32(0x67890123)
		mem.ints[entry], mem.ints[entry+8] = int32(index), int32(45+index)
		mem.ints[entry+16], mem.ints[entry+20] = int32((uint32(value)^key)+key), int32(key)
	}
	resolver := NewResolver(nil)
	resolver.accountStatusObject = object
	got := resolver.readPlagueChestAutomation(mem, player, time.Now())
	if !got.RuneResolved || got.CommonRuneReductionSeconds != 60 || got.StageRuneReductionSeconds != 120 {
		t.Fatalf("aggregate account stats must be used once: %+v", got)
	}
	// A cached account preview can be valid but belong to a different rune state.
	mem.ints[entries+32+28+16] = mem.ints[entries+32+28+20] * 2
	got = resolver.readPlagueChestAutomation(mem, player, time.Now())
	if !got.RuneResolved || got.CommonRuneReductionSeconds != 60 || got.StageRuneReductionSeconds != 120 || !strings.Contains(strings.Join(got.Diagnostics, ";"), "matching PlagueLands AccountStatus unavailable") {
		t.Fatalf("unrelated account values replaced the player's runes: %+v", got)
	}
}
