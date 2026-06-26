package playerdata

import (
	"github.com/nidea1/task-bar-trade-center/internal/il2cpp"
	"github.com/nidea1/task-bar-trade-center/internal/tbhmem"
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
		items = append(items, resolver.ownedItem(itemID, uniqueID, location, 0, i))
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
