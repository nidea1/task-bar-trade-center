package playerdata

import (
	"bytes"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/il2cpp"
	"github.com/nidea1/task-bar-trade-center/internal/tbhmem"
)

// IL2CPP FieldInfo keeps the declaring class and actual field offset, allowing
// these named fields to move without changing the save-data layout constants.
func (resolver *Resolver) instanceFields(memory Memory, class uintptr) map[string]uintptr {
	if fields, ok := resolver.fieldOffsets[class]; ok {
		return fields
	}
	reader, ok := memory.(byteMemory)
	if !ok || !tbhmem.PlausibleAddress(class) {
		return nil
	}
	array := readPtr(memory, class+0x80)
	size, ok := memory.ReadInt32(class + il2cpp.ClassInstanceSizeOffset)
	if array == 0 || !ok || size < 0x10 || size > 1<<20 {
		return nil
	}
	fields := make(map[string]uintptr)
	for index := uintptr(0); index < 256; index++ {
		entry := array + index*0x20
		if readPtr(memory, entry+0x10) != class {
			break
		}
		offset, ok := memory.ReadInt32(entry + 0x18)
		if !ok || offset < 0x10 || offset >= size {
			continue
		}
		name := readPtr(memory, entry)
		var buffer [96]byte
		if name == 0 || !reader.ReadBytes(name, buffer[:]) {
			continue
		}
		if end := bytes.IndexByte(buffer[:], 0); end > 0 {
			fields[string(buffer[:end])] = uintptr(offset)
		}
	}
	if len(fields) > 0 {
		if resolver.fieldOffsets == nil {
			resolver.fieldOffsets = make(map[uintptr]map[string]uintptr)
		}
		resolver.fieldOffsets[class] = fields
	}
	return fields
}

func (resolver *Resolver) readRegion(memory Memory, player uintptr, now time.Time) (Region, int) {
	fields := resolver.instanceFields(memory, readPtr(memory, player))
	commonOffset := fields["commonSaveData"]
	if commonOffset == 0 {
		return "", 0
	}
	common := readPtr(memory, player+commonOffset)
	fields = resolver.instanceFields(memory, readPtr(memory, common))
	keyOffset := fields["currentStageKey"]
	if common == 0 || keyOffset == 0 {
		return "", 0
	}
	key, ok := memory.ReadInt32(common + keyOffset)
	if !ok || key <= 0 {
		return "", 0
	}
	if object := resolver.stageInfoObjects[int(key)]; object != 0 {
		if region, ok := resolver.regionFromStageInfo(memory, object, key); ok {
			return region, int(key)
		}
	}
	if !resolver.stageResolveAttemptedAt.IsZero() && now.Sub(resolver.stageResolveAttemptedAt) < 5*time.Second {
		return "", int(key)
	}
	resolver.stageResolveAttemptedAt = now
	classes, ok := resolver.resolveClassByName(memory, "StageInfoData")
	if !ok {
		return "", int(key)
	}
	resolver.stageInfoObjects = make(map[int]uintptr)
	for _, refs := range resolver.scanClassReferences(memory, classes) {
		for _, object := range refs {
			fields := resolver.instanceFields(memory, readPtr(memory, object))
			if fields["StageKey"] == 0 || fields["STAGETYPE"] == 0 {
				continue
			}
			stageKey, ok := memory.ReadInt32(object + fields["StageKey"])
			if !ok || stageKey <= 0 {
				continue
			}
			if _, ok := resolver.regionFromStageInfo(memory, object, stageKey); ok {
				resolver.stageInfoObjects[int(stageKey)] = object
			}
		}
	}
	region, _ := resolver.regionFromStageInfo(memory, resolver.stageInfoObjects[int(key)], key)
	return region, int(key)
}

func (resolver *Resolver) regionFromStageInfo(memory Memory, object uintptr, key int32) (Region, bool) {
	if object == 0 {
		return "", false
	}
	fields := resolver.instanceFields(memory, readPtr(memory, object))
	if fields["StageKey"] == 0 || fields["STAGETYPE"] == 0 {
		return "", false
	}
	actual, ok := memory.ReadInt32(object + fields["StageKey"])
	if !ok || actual != key {
		return "", false
	}
	typ, ok := memory.ReadInt32(object + fields["STAGETYPE"])
	if !ok {
		return "", false
	}
	switch typ {
	case 0, 1:
		return RegionNormal, true // NORMAL, ACTBOSS
	case 2, 3:
		return RegionPlaguelands, true // PLAGUE, CONTAMINACTBOSS
	default:
		return "", false
	}
}
