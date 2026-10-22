package playerdata

import (
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/il2cpp"
)

// Verified against the game's EAccountStatus and OpenBoxStats getter. Plague
// chests have their own unlock times and reductions, separate from normal runes.
func (resolver *Resolver) readPlagueChestAutomation(memory Memory, now time.Time) ChestAutomation {
	if automation, ok := readPlagueAccountStatus(memory, resolver.accountStatusObject); ok {
		return automation
	}
	if resolver.accountStatusAttemptedAt.IsZero() || now.Sub(resolver.accountStatusAttemptedAt) >= 5*time.Second {
		resolver.accountStatusAttemptedAt = now
		classes, ok := resolver.resolveClassByName(memory, "AccountStatus")
		if ok {
			for _, refs := range resolver.scanClassReferences(memory, classes) {
				for _, object := range refs {
					if automation, ok := readPlagueAccountStatus(memory, object); ok {
						resolver.accountStatusObject = object
						return automation
					}
				}
			}
		}
	}
	return ChestAutomation{Diagnostics: []string{"PlagueLands chest automation could not be resolved"}}
}

func readPlagueAccountStatus(memory Memory, object uintptr) (ChestAutomation, bool) {
	if object == 0 {
		return ChestAutomation{}, false
	}
	class := readPtr(memory, object)
	if name, ok := readClassName(memory, class); !ok || name != "AccountStatus" {
		return ChestAutomation{}, false
	}
	dictionary := readPtr(memory, object+0x10)
	if name, ok := readClassName(memory, readPtr(memory, dictionary)); !ok || name != "Dictionary`2" {
		return ChestAutomation{}, false
	}
	entries := readPtr(memory, dictionary+0x18)
	count, ok := memory.ReadInt32(dictionary + 0x20)
	capacity, capacityOK := memory.ReadUintptr(entries + il2cpp.ArrayMaxOffset)
	if !ok || !capacityOK || count <= 0 || count > 256 || capacity < uintptr(count) {
		return ChestAutomation{}, false
	}
	values := make(map[int32]int32, 4)
	for index := int32(0); index < count; index++ {
		entry := entries + il2cpp.ArrayDataOffset + uintptr(index)*28
		hash, hashOK := memory.ReadInt32(entry)
		key, keyOK := memory.ReadInt32(entry + 8)
		if !hashOK || !keyOK || hash < 0 || key < 45 || key > 48 {
			continue
		}
		hidden, hiddenOK := memory.ReadInt32(entry + 16)
		cryptoKey, cryptoOK := memory.ReadInt32(entry + 20)
		if !hiddenOK || !cryptoOK {
			return ChestAutomation{}, false
		}
		// Read-only decoding of the game's inline ObscuredInt value.
		value := int32((uint32(hidden) - uint32(cryptoKey)) ^ uint32(cryptoKey))
		if value < 0 || value > 86400 {
			return ChestAutomation{}, false
		}
		values[key] = value
	}
	if len(values) != 4 {
		return ChestAutomation{}, false
	}
	return ChestAutomation{
		CommonBaseAutoOpenSeconds:  int(values[45]),
		StageBaseAutoOpenSeconds:   int(values[47]),
		CommonRuneReductionSeconds: int(values[46]),
		StageRuneReductionSeconds:  int(values[48]),
		HasCommonAutoOpenRune:      values[45] > 0,
		HasStageAutoOpenRune:       values[47] > 0,
		RuneResolved:               true,
	}, true
}
