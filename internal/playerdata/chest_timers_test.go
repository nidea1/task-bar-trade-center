package playerdata

import (
	"math"
	"testing"
	"time"
)

func stageBoxMemory() (fakeMemory, *Resolver, uintptr) {
	mem := regionTestMemory()
	box, slider := uintptr(0x400000), uintptr(0x500000)
	writeClass(mem, "StageBox", 0x200000, 0x300000, box)
	mem.ptrs[box+0x10] = 0x900000
	mem.ptrs[box+0xE8], mem.ptrs[slider] = slider, 0x310000
	resolver := NewResolver(nil)
	resolver.fieldOffsets = map[uintptr]map[string]uintptr{
		0x300000: {"m_boxType": 0x48, "m_contentType": 0x4C, "m_autoOpenSliderView": 0xE8, "bkmu": 0x128, "bkmv": 0x12C, "bkmw": 0x130, "bkmx": 0x134},
		0x310000: {"m_MinValue": 0x100, "m_MaxValue": 0x104, "m_Value": 0x108},
	}
	mem.ints[box+0x48], mem.ints[box+0x4C], mem.ints[box+0x130] = 0, 0, 1
	mem.ints[box+0x128] = int32(math.Float32bits(300))
	mem.ints[box+0x12C] = int32(math.Float32bits(300))
	mem.ints[box+0x134] = int32(math.Float32bits(120))
	mem.ints[slider+0x100] = 0
	mem.ints[slider+0x104] = int32(math.Float32bits(1))
	mem.ints[slider+0x108] = int32(math.Float32bits(0.4))
	return mem, resolver, box
}

func TestChestTimerReadsProgressAndRegion(t *testing.T) {
	mem, resolver, box := stageBoxMemory()
	timer := resolver.readChestTimers(mem, RegionNormal, time.Now()).Common
	if !timer.Available || !timer.Enabled || timer.DurationSeconds != 300 || timer.RemainingSeconds != 180 {
		t.Fatalf("timer=%+v", timer)
	}
	if got := resolver.readChestTimers(mem, RegionPlaguelands, time.Now()); got.Common.Available {
		t.Fatal("normal timer leaked into Plague region")
	}
	mem.ints[box+0x4C] = 1
	mem.ints[box+0x48] = 1
	if got := resolver.readChestTimers(mem, RegionPlaguelands, time.Now()); !got.Stage.Available || got.Common.Available {
		t.Fatalf("wrong Plague timer kind: %+v", got)
	}
	mem.ints[box+0x130] = 0
	if got := resolver.readChestTimers(mem, RegionPlaguelands, time.Now()); !got.Stage.Available || got.Stage.Enabled {
		t.Fatalf("disabled timer=%+v", got)
	}
}

func TestChestTimerRejectsInvalidOrUninitializedData(t *testing.T) {
	for _, tt := range []struct {
		name    string
		address uintptr
		value   int32
	}{
		{"prefab base", 0x400128, 0},
		{"invalid enabled flag", 0x400130, 255},
		{"nonfinite duration", 0x40012C, int32(math.Float32bits(float32(math.NaN())))},
		{"elapsed out of range", 0x400134, int32(math.Float32bits(3010))},
		{"inconsistent slider", 0x500108, int32(math.Float32bits(0.9))},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mem, resolver, box := stageBoxMemory()
			mem.ints[tt.address] = tt.value
			if _, _, ok := resolver.readStageBoxTimer(mem, box, RegionNormal); ok {
				t.Fatal("invalid timer accepted")
			}
		})
	}
}

func TestNormalUnlockDoesNotRequireReductionRune(t *testing.T) {
	mem, player := plagueRuneMemory(map[int32]int32{13002: 1, 130021: 0, 15001: 0, 150011: 0})
	got := NewResolver(nil).readChestAutomationWithLayout(mem, player, defaultSaveDataLayout())
	if !got.HasCommonAutoOpenRune || got.HasStageAutoOpenRune || got.CommonRuneReductionSeconds != 0 || got.CommonBaseAutoOpenSeconds != 300 {
		t.Fatalf("unlock-only automation=%+v", got)
	}
}

func TestChestCountTextRejectsPlaceholders(t *testing.T) {
	for _, tt := range []struct {
		text  string
		count int
		ok    bool
	}{{"10/40", 10, true}, {"0/40", 0, true}, {" 2 / 8 ", 2, true}, {"0/0", 0, false}, {"99/40", 99, false}, {"-1/40", -1, false}, {"<b>2</b>/8", 0, false}, {"Loading", 0, false}} {
		count, ok := parseChestCountText(tt.text)
		if ok != tt.ok || (ok && count != tt.count) {
			t.Fatalf("count %q=%d/%t", tt.text, count, ok)
		}
	}
}
