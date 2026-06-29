package playerdata

import (
	"encoding/binary"
	"sort"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/il2cpp"
	"github.com/nidea1/task-bar-trade-center/internal/tbhmem"
)

const (
	runtimeTradeSlotIndex         = 0x80
	runtimeTradeSlotCountdownText = 0xE0
	runtimeTradeSlotCooldownUntil = 0xE8
	runtimeTradeSlotCache         = 0x100
)

func (resolver *Resolver) readItemSaveDataList(memory Memory, list listInfo) (map[uint64]int, int) {
	uniqueToItem := make(map[uint64]int)
	valid := 0
	limit := list.size
	if limit > 5000 {
		limit = 5000
	}
	for i := 0; i < limit; i++ {
		obj, ok := memory.ReadUintptr(list.arrayPtr + il2cpp.ArrayDataOffset + uintptr(i*8))
		if !ok || obj == 0 || !tbhmem.PlausibleAddress(obj) {
			continue
		}
		key, ok := memory.ReadInt32(obj + itemSaveItemKey)
		if !ok || key <= 0 {
			continue
		}
		if _, exists := resolver.metadata[int(key)]; !exists && len(resolver.metadata) > 0 {
			continue
		}
		uniqueID, ok := memory.ReadUint64(obj + itemSaveUniqueID)
		if !ok || uniqueID == 0 {
			continue
		}
		uniqueToItem[uniqueID] = int(key)
		valid++
	}
	return uniqueToItem, valid
}

func (resolver *Resolver) readSlotItems(memory Memory, list listInfo, uniqueToItem map[uint64]int, seen map[uint64]struct{}, location Location) []OwnedItem {
	limit := list.size
	if limit > 5000 {
		limit = 5000
	}
	items := make([]OwnedItem, 0)
	for i := 0; i < limit; i++ {
		slot, ok := memory.ReadUintptr(list.arrayPtr + il2cpp.ArrayDataOffset + uintptr(i*8))
		if !ok || slot == 0 || !tbhmem.PlausibleAddress(slot) {
			continue
		}
		uniqueID, ok := memory.ReadUint64(slot + slotUniqueID)
		if !ok || uniqueID == 0 {
			continue
		}
		itemID, exists := uniqueToItem[uniqueID]
		if !exists {
			continue
		}
		if _, exists := seen[uniqueID]; exists {
			continue
		}
		seen[uniqueID] = struct{}{}
		items = append(items, resolver.ownedItem(itemID, uniqueID, location, 0, readSlotPosition(memory, slot, i, list.size)))
	}
	return items
}

func (resolver *Resolver) readEquippedItems(memory Memory, heroes listInfo, uniqueToItem map[uint64]int, seen map[uint64]struct{}) []OwnedItem {
	limit := heroes.size
	if limit > 100 {
		limit = 100
	}
	items := make([]OwnedItem, 0)
	for i := 0; i < limit; i++ {
		hero, ok := memory.ReadUintptr(heroes.arrayPtr + il2cpp.ArrayDataOffset + uintptr(i*8))
		if !ok || hero == 0 || !tbhmem.PlausibleAddress(hero) {
			continue
		}
		heroKeyValue, _ := memory.ReadInt32(hero + heroKey)
		arrayPtr, ok := memory.ReadUintptr(hero + heroEquippedItems)
		if !ok || arrayPtr == 0 || !tbhmem.PlausibleAddress(arrayPtr) {
			continue
		}
		maxLen, ok := memory.ReadUintptr(arrayPtr + il2cpp.ArrayMaxOffset)
		if !ok || maxLen > 100 {
			continue
		}
		for slot := uintptr(0); slot < maxLen; slot++ {
			uniqueID, ok := memory.ReadUint64(arrayPtr + il2cpp.ArrayDataOffset + slot*8)
			if !ok || uniqueID == 0 {
				continue
			}
			itemID, exists := uniqueToItem[uniqueID]
			if !exists {
				continue
			}
			if _, exists := seen[uniqueID]; exists {
				continue
			}
			seen[uniqueID] = struct{}{}
			items = append(items, resolver.ownedItem(itemID, uniqueID, LocationEquipped, int(heroKeyValue), int(slot)))
		}
	}
	return items
}

func (resolver *Resolver) ownedItem(itemID int, uniqueID uint64, location Location, heroKey int, slotIndex int) OwnedItem {
	meta := resolver.metadata[itemID]
	return OwnedItem{
		ItemID:          itemID,
		UniqueID:        uniqueID,
		Location:        location,
		EquippedHeroKey: heroKey,
		Marketable:      meta.Marketable,
		SlotIndex:       slotIndex,
	}
}

func readSlotPosition(memory Memory, slot uintptr, fallback int, listSize int) int {
	value, ok := memory.ReadInt32(slot + slotIndex)
	if !ok || value < 0 {
		return fallback
	}
	// The save slot list is normally the physical slot order. Some builds also
	// expose a sparse Index field; use it only when the list is compressed.
	if int(value) < listSize {
		return fallback
	}
	if value > 5000 {
		return fallback
	}
	return int(value)
}

func countSlotMatches(memory Memory, list listInfo, uniqueToItem map[uint64]int) (filled int, known int) {
	limit := list.size
	if limit > 5000 {
		limit = 5000
	}
	for i := 0; i < limit; i++ {
		slot, ok := memory.ReadUintptr(list.arrayPtr + il2cpp.ArrayDataOffset + uintptr(i*8))
		if !ok || slot == 0 || !tbhmem.PlausibleAddress(slot) {
			continue
		}
		uniqueID, ok := memory.ReadUint64(slot + slotUniqueID)
		if !ok || uniqueID == 0 {
			continue
		}
		filled++
		if _, exists := uniqueToItem[uniqueID]; exists {
			known++
		}
	}
	return filled, known
}

func countEquippedMatches(memory Memory, heroes listInfo, uniqueToItem map[uint64]int) (filled int, known int) {
	seen := make(map[uint64]struct{})
	limit := heroes.size
	if limit > 100 {
		limit = 100
	}
	for i := 0; i < limit; i++ {
		hero, ok := memory.ReadUintptr(heroes.arrayPtr + il2cpp.ArrayDataOffset + uintptr(i*8))
		if !ok || hero == 0 || !tbhmem.PlausibleAddress(hero) {
			continue
		}
		arrayPtr, ok := memory.ReadUintptr(hero + heroEquippedItems)
		if !ok || arrayPtr == 0 || !tbhmem.PlausibleAddress(arrayPtr) {
			continue
		}
		maxLen, ok := memory.ReadUintptr(arrayPtr + il2cpp.ArrayMaxOffset)
		if !ok || maxLen > 100 {
			continue
		}
		for slot := uintptr(0); slot < maxLen; slot++ {
			uniqueID, ok := memory.ReadUint64(arrayPtr + il2cpp.ArrayDataOffset + slot*8)
			if !ok || uniqueID == 0 {
				continue
			}
			if _, duplicate := seen[uniqueID]; duplicate {
				continue
			}
			seen[uniqueID] = struct{}{}
			filled++
			if _, exists := uniqueToItem[uniqueID]; exists {
				known++
			}
		}
	}
	return filled, known
}

func readGold(memory Memory, currencies listInfo) (uint64, bool) {
	limit := currencies.size
	if limit > 1000 {
		limit = 1000
	}
	for i := 0; i < limit; i++ {
		currency, ok := memory.ReadUintptr(currencies.arrayPtr + il2cpp.ArrayDataOffset + uintptr(i*8))
		if !ok || currency == 0 || !tbhmem.PlausibleAddress(currency) {
			continue
		}
		key, ok := memory.ReadInt32(currency + currencyKey)
		if !ok || key != goldKey {
			continue
		}
		return memory.ReadUint64(currency + currencyQuantity)
	}
	return 0, false
}

func (resolver *Resolver) readTradeSlots(memory Memory, list listInfo) []TradeShipSlot {
	limit := list.size
	if limit > 100 {
		limit = 100
	}
	slots := make([]TradeShipSlot, 0, limit)
	for i := 0; i < limit; i++ {
		slotPtr, ok := memory.ReadUintptr(list.arrayPtr + il2cpp.ArrayDataOffset + uintptr(i*8))
		if !ok || slotPtr == 0 || !tbhmem.PlausibleAddress(slotPtr) {
			continue
		}
		indexVal, ok := memory.ReadInt32(slotPtr + 0x10) // slotIndex
		if !ok {
			continue
		}
		cooldownRaw, ok := memory.ReadUint64(slotPtr + 0x18) // cooldownUntil (ticks)
		if !ok {
			continue
		}
		stateVal, ok := memory.ReadInt32(slotPtr + 0x20) // state
		if !ok {
			continue
		}

		var cooldownUntil time.Time
		if cooldownRaw > 0 {
			// Convert ticks with custom epoch offset
			const constantOffset = 135194695325352348
			const ticksPerSecond = 10000000
			const ticksAtUnixEpoch = 621355968000000000

			actualTicks := cooldownRaw + constantOffset
			if actualTicks >= ticksAtUnixEpoch {
				unixSecs := int64(actualTicks-ticksAtUnixEpoch) / ticksPerSecond
				cooldownUntil = time.Unix(unixSecs, 0).UTC()
			}
		}

		slots = append(slots, TradeShipSlot{
			Index:         int(indexVal),
			State:         int(stateVal),
			CooldownUntil: cooldownUntil,
		})
	}
	return slots
}

func (resolver *Resolver) readRuntimeTradeSlots(memory Memory, now time.Time) []TradeShipSlot {
	slotClasses, ok := il2cpp.ResolveClassByName(memory, "TradingStashSlot")
	if !ok || len(slotClasses) == 0 {
		return nil
	}
	cacheClasses, ok := il2cpp.ResolveClassByName(memory, "TradingStashCache")
	if !ok || len(cacheClasses) == 0 {
		return nil
	}
	cacheClassSet := make(map[uintptr]struct{}, len(cacheClasses))
	for _, classPtr := range cacheClasses {
		cacheClassSet[classPtr] = struct{}{}
	}

	byIndex := make(map[int]TradeShipSlot)
	for _, classPtr := range slotClasses {
		ptrBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(ptrBytes, uint64(classPtr))
		refs, _ := memory.ScanPattern(ptrBytes, 50000)
		for _, object := range refs {
			if readPtr(memory, object) != classPtr {
				continue
			}
			indexVal, ok := memory.ReadInt32(object + runtimeTradeSlotIndex)
			if !ok || indexVal < 0 || indexVal > 99 {
				continue
			}
			cachePtr := readPtr(memory, object+runtimeTradeSlotCache)
			cacheClass := readPtr(memory, cachePtr)
			if _, ok := cacheClassSet[cacheClass]; !ok {
				continue
			}
			textPtr := readPtr(memory, object+runtimeTradeSlotCountdownText)
			if textPtr == 0 || !tbhmem.PlausibleAddress(textPtr) {
				continue
			}
			rawUntil, ok := memory.ReadUint64(object + runtimeTradeSlotCooldownUntil)
			if !ok {
				continue
			}
			cooldownUntil, ok := dotnetDateTime(rawUntil)
			if !ok || cooldownUntil.Before(now.Add(-24*time.Hour)) || cooldownUntil.After(now.Add(7*24*time.Hour)) {
				continue
			}

			slot := TradeShipSlot{Index: int(indexVal), State: 1, CooldownUntil: cooldownUntil}
			if existing, exists := byIndex[slot.Index]; !exists || slot.CooldownUntil.After(existing.CooldownUntil) {
				byIndex[slot.Index] = slot
			}
		}
	}
	if len(byIndex) == 0 {
		return nil
	}
	slots := make([]TradeShipSlot, 0, len(byIndex))
	for _, slot := range byIndex {
		slots = append(slots, slot)
	}
	sort.Slice(slots, func(i, j int) bool {
		return slots[i].Index < slots[j].Index
	})
	return slots
}

func dotnetDateTime(value uint64) (time.Time, bool) {
	const ticksAtUnixEpoch = uint64(621355968000000000)
	const ticksPerSecond = uint64(10000000)

	ticks := value & 0x3FFFFFFFFFFFFFFF
	if ticks < ticksAtUnixEpoch {
		return time.Time{}, false
	}
	unixSecs := int64((ticks - ticksAtUnixEpoch) / ticksPerSecond)
	return time.Unix(unixSecs, 0).UTC(), true
}
