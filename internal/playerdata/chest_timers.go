package playerdata

import (
	"encoding/binary"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
)

func (resolver *Resolver) readChestTimers(memory Memory, region Region, now time.Time) ChestTimers {
	if region != RegionNormal && region != RegionPlaguelands {
		return ChestTimers{}
	}
	if len(resolver.chestTimerObjects) == 0 && (resolver.chestTimerAttemptedAt.IsZero() || now.Sub(resolver.chestTimerAttemptedAt) >= 5*time.Second) {
		resolver.chestTimerAttemptedAt = now
		classes, ok := resolver.resolveClassByName(memory, "StageBox")
		if ok {
			for _, refs := range resolver.scanClassReferences(memory, classes) {
				resolver.chestTimerObjects = append(resolver.chestTimerObjects, refs...)
			}
		}
	}
	var result ChestTimers
	var commonSeen, stageSeen, commonConflict, stageConflict bool
	validObjects := 0
	for _, object := range resolver.chestTimerObjects {
		timer, kind, ok := resolver.readStageBoxTimer(memory, object, region)
		if !ok {
			continue
		}
		validObjects++
		target, seen, conflict := &result.Common, &commonSeen, &commonConflict
		if kind == 1 {
			target, seen, conflict = &result.Stage, &stageSeen, &stageConflict
		}
		if *seen && (target.Enabled != timer.Enabled || target.DurationSeconds != timer.DurationSeconds || math.Abs(target.RemainingSeconds-timer.RemainingSeconds) > 2) {
			*conflict = true
		}
		if !*seen {
			*target = timer
			*seen = true
		}
	}
	if commonConflict {
		result.Common = ChestTimer{}
	}
	if stageConflict {
		result.Stage = ChestTimer{}
	}
	if validObjects == 0 && now.Sub(resolver.chestTimerAttemptedAt) >= 5*time.Second {
		resolver.chestTimerObjects = nil
	}
	return result
}

func readFloat(memory Memory, address uintptr) (float64, bool) {
	bits, ok := memory.ReadInt32(address)
	value := float64(math.Float32frombits(uint32(bits)))
	return value, ok && !math.IsNaN(value) && !math.IsInf(value, 0)
}

// Named fields and slider agreement guard this 1.3.0 layout. No game methods
// are invoked. Unknown layouts return an unavailable observation.
func (resolver *Resolver) readStageBoxTimer(memory Memory, object uintptr, region Region) (ChestTimer, int32, bool) {
	class := readPtr(memory, object)
	if readPtr(memory, object+0x10) == 0 {
		return ChestTimer{}, 0, false
	}
	if name, ok := readClassName(memory, class); !ok || name != "StageBox" {
		return ChestTimer{}, 0, false
	}
	f := resolver.instanceFields(memory, class)
	for _, name := range []string{"m_boxType", "m_contentType", "m_autoOpenSliderView", "bkmu", "bkmv", "bkmx", "bkmw"} {
		if f[name] == 0 {
			return ChestTimer{}, 0, false
		}
	}
	kind, ok := memory.ReadInt32(object + f["m_boxType"])
	if !ok || (kind != 0 && kind != 1) {
		return ChestTimer{}, 0, false
	}
	content, ok := memory.ReadInt32(object + f["m_contentType"])
	expected := int32(0)
	if region == RegionPlaguelands {
		expected = 1
	}
	if !ok || content != expected {
		return ChestTimer{}, 0, false
	}
	base, baseOK := readFloat(memory, object+f["bkmu"])
	duration, durationOK := readFloat(memory, object+f["bkmv"])
	elapsed, elapsedOK := readFloat(memory, object+f["bkmx"])
	enabled, enabledOK := readBoolByte(memory, object+f["bkmw"])
	if reader, ok := memory.(byteMemory); ok {
		var raw [1]byte
		if !reader.ReadBytes(object+f["bkmw"], raw[:]) || raw[0] > 1 {
			enabledOK = false
		}
	}
	if !baseOK || !durationOK || !elapsedOK || !enabledOK || base < 1 || base > 86400 || duration < 1 || duration > base || elapsed < 0 || elapsed > duration+1 {
		return ChestTimer{}, 0, false
	}
	slider := readPtr(memory, object+f["m_autoOpenSliderView"])
	sf := resolver.instanceFields(memory, readPtr(memory, slider))
	for _, name := range []string{"m_Value", "m_MinValue", "m_MaxValue"} {
		if sf[name] == 0 {
			return ChestTimer{}, 0, false
		}
	}
	value, valueOK := readFloat(memory, slider+sf["m_Value"])
	min, minOK := readFloat(memory, slider+sf["m_MinValue"])
	max, maxOK := readFloat(memory, slider+sf["m_MaxValue"])
	if !valueOK || !minOK || !maxOK || max <= min || value < min || value > max || math.Abs((value-min)/(max-min)-elapsed/duration) > 0.02 {
		return ChestTimer{}, 0, false
	}
	count, countOK := resolver.readStageBoxCount(memory, object, f)
	return ChestTimer{Available: true, Enabled: enabled, DurationSeconds: int(math.Round(duration)), RemainingSeconds: math.Max(0, duration-elapsed), QueueCount: count, QueueResolved: countOK}, kind, true
}

func (resolver *Resolver) readStageBoxCount(memory Memory, object uintptr, fields map[string]uintptr) (int, bool) {
	if fields["countText"] == 0 {
		return 0, false
	}
	textObject := readPtr(memory, object+fields["countText"])
	var offset uintptr
	for class, depth := readPtr(memory, textObject), 0; class != 0 && depth < 16; class, depth = readPtr(memory, class+0x58), depth+1 {
		if field := resolver.instanceFields(memory, class)["m_text"]; field > 0 {
			offset = field
			break
		}
	}
	if offset == 0 {
		return 0, false
	}
	value := readPtr(memory, textObject+offset)
	length, ok := memory.ReadInt32(value + 0x10)
	reader, readable := memory.(byteMemory)
	if !ok || !readable || length < 1 || length > 64 {
		return 0, false
	}
	data := make([]byte, int(length)*2)
	if !reader.ReadBytes(value+0x14, data) {
		return 0, false
	}
	units := make([]uint16, length)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(data[i*2:])
	}
	return parseChestCountText(string(utf16.Decode(units)))
}

func parseChestCountText(text string) (int, bool) {
	parts := strings.Split(strings.TrimSpace(text), "/")
	if len(parts) != 2 {
		return 0, false
	}
	count, e1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	capacity, e2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	return count, e1 == nil && e2 == nil && count >= 0 && capacity > 0 && count <= capacity && capacity <= 100000
}
