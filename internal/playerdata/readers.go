package playerdata

import (
	"fmt"
	"sort"
	"strings"
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
	return resolver.readItemSaveDataListWithLayout(memory, list, defaultSaveDataLayout())
}

func (resolver *Resolver) readItemSaveDataListWithLayout(memory Memory, list listInfo, layout saveDataLayout) (map[uint64]int, int) {
	uniqueToItem := make(map[uint64]int)
	if layout.itemKeyOffset == 0 || layout.itemUniqueIDOffset == 0 {
		return uniqueToItem, 0
	}
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
		key, ok := memory.ReadInt32(obj + layout.itemKeyOffset)
		if !ok || key <= 0 {
			continue
		}
		if _, exists := resolver.metadata[int(key)]; !exists && len(resolver.metadata) > 0 {
			continue
		}
		uniqueID, ok := memory.ReadUint64(obj + layout.itemUniqueIDOffset)
		if !ok || uniqueID == 0 {
			continue
		}
		uniqueToItem[uniqueID] = int(key)
		valid++
	}
	return uniqueToItem, valid
}

func (resolver *Resolver) readSlotItems(memory Memory, list listInfo, uniqueToItem map[uint64]int, seen map[uint64]struct{}, location Location) []OwnedItem {
	return resolver.readSlotItemsWithLayout(memory, list, uniqueToItem, seen, location, defaultSaveDataLayout())
}

func (resolver *Resolver) readSlotItemsWithLayout(memory Memory, list listInfo, uniqueToItem map[uint64]int, seen map[uint64]struct{}, location Location, layout saveDataLayout) []OwnedItem {
	if layout.slotUniqueIDOffset == 0 {
		return nil
	}
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
		uniqueID, ok := memory.ReadUint64(slot + layout.slotUniqueIDOffset)
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
		item := resolver.ownedItem(itemID, uniqueID, location, 0, readSlotPositionWithLayout(memory, slot, i, list.size, layout))
		if field := resolver.instanceFields(memory, readPtr(memory, slot))["Quantity"]; field > 0 {
			if quantity, ok := memory.ReadInt32(slot + field); ok && quantity > 0 {
				item.Quantity = int(quantity)
			}
		}
		items = append(items, item)
	}
	return items
}

func readInventorySlotSummaryWithLayout(memory Memory, list listInfo, layout saveDataLayout) InventorySlotSummary {
	if !list.ok || list.size <= 0 || layout.slotUniqueIDOffset == 0 {
		return InventorySlotSummary{}
	}
	limit := list.size
	if limit > 5000 {
		limit = 5000
	}
	if summary, ok := readUnlockedInventorySlotSummary(memory, list, layout, limit); ok {
		return summary
	}

	used := 0
	emptySlots := make([]int, 0)
	for i := 0; i < limit; i++ {
		slot, ok := memory.ReadUintptr(list.arrayPtr + il2cpp.ArrayDataOffset + uintptr(i*8))
		if !ok || slot == 0 || !tbhmem.PlausibleAddress(slot) {
			emptySlots = append(emptySlots, i)
			continue
		}
		uniqueID, ok := memory.ReadUint64(slot + layout.slotUniqueIDOffset)
		if ok && uniqueID != 0 {
			used++
			continue
		}
		emptySlots = append(emptySlots, readSlotPositionWithLayout(memory, slot, i, list.size, layout))
	}
	free := list.size - used
	if free < 0 {
		free = 0
	}
	return InventorySlotSummary{
		Total:           list.size,
		Used:            used,
		Free:            free,
		EmptySlotRanges: formatSlotRanges(emptySlots),
	}
}

func readUnlockedInventorySlotSummary(memory Memory, list listInfo, layout saveDataLayout, limit int) (InventorySlotSummary, bool) {
	unlockStateRead := false
	total := 0
	used := 0
	emptySlots := make([]int, 0)
	for i := 0; i < limit; i++ {
		slot, ok := listObjectAt(memory, list, i)
		if !ok {
			continue
		}
		unlocked, ok := readInventorySlotUnlocked(memory, slot)
		if !ok {
			continue
		}
		unlockStateRead = true
		if !unlocked {
			continue
		}
		total++
		uniqueID, ok := memory.ReadUint64(slot + layout.slotUniqueIDOffset)
		if ok && uniqueID != 0 {
			used++
			continue
		}
		emptySlots = append(emptySlots, readSlotPositionWithLayout(memory, slot, i, list.size, layout))
	}
	if !unlockStateRead {
		return InventorySlotSummary{}, false
	}
	free := total - used
	if free < 0 {
		free = 0
	}
	return InventorySlotSummary{
		Total:           total,
		BackingTotal:    backingTotal(list.size, total),
		Used:            used,
		Free:            free,
		EmptySlotRanges: formatSlotRanges(emptySlots),
	}, true
}

func readInventorySlotUnlocked(memory Memory, slot uintptr) (bool, bool) {
	baseUnlocked, baseOK := readBoolByte(memory, slot+slotIsUnlock)
	runeUnlocked, runeOK := readBoolByte(memory, slot+slotIsRuneUnlock)
	if !baseOK && !runeOK {
		return false, false
	}
	return (baseOK && baseUnlocked) || (runeOK && runeUnlocked), true
}

func readBoolByte(memory Memory, address uintptr) (bool, bool) {
	reader, ok := memory.(byteMemory)
	if !ok {
		return false, false
	}
	var data [1]byte
	if !reader.ReadBytes(address, data[:]) {
		return false, false
	}
	return data[0] != 0, true
}

func backingTotal(rawTotal int, usableTotal int) int {
	if rawTotal == usableTotal {
		return 0
	}
	return rawTotal
}

func (resolver *Resolver) readEquippedItems(memory Memory, heroes listInfo, uniqueToItem map[uint64]int, seen map[uint64]struct{}) []OwnedItem {
	return resolver.readEquippedItemsWithLayout(memory, heroes, uniqueToItem, seen, defaultSaveDataLayout())
}

func (resolver *Resolver) readEquippedItemsWithLayout(memory Memory, heroes listInfo, uniqueToItem map[uint64]int, seen map[uint64]struct{}, layout saveDataLayout) []OwnedItem {
	if layout.heroEquippedItems == 0 {
		return nil
	}
	heroKeyOffset := detectHeroKeyOffset(memory, heroes, layout.heroEquippedItems, uniqueToItem)
	if heroKeyOffset == 0 {
		heroKeyOffset = layout.heroKeyOffset
	}
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
		var heroKeyValue int32
		if heroKeyOffset != 0 {
			heroKeyValue, _ = memory.ReadInt32(hero + heroKeyOffset)
		}
		arrayPtr, ok := memory.ReadUintptr(hero + layout.heroEquippedItems)
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
	return readSlotPositionWithLayout(memory, slot, fallback, listSize, defaultSaveDataLayout())
}

func readSlotPositionWithLayout(memory Memory, slot uintptr, fallback int, listSize int, layout saveDataLayout) int {
	if layout.slotIndexOffset == 0 {
		return fallback
	}
	value, ok := memory.ReadInt32(slot + layout.slotIndexOffset)
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

func formatSlotRanges(slots []int) string {
	if len(slots) == 0 {
		return ""
	}
	sort.Ints(slots)
	uniqueSlots := slots[:0]
	previous := -1
	for _, slot := range slots {
		if slot == previous {
			continue
		}
		uniqueSlots = append(uniqueSlots, slot)
		previous = slot
	}

	parts := make([]string, 0)
	start := uniqueSlots[0]
	end := start
	for _, slot := range uniqueSlots[1:] {
		if slot == end+1 {
			end = slot
			continue
		}
		parts = append(parts, formatSlotRange(start, end))
		start = slot
		end = slot
	}
	parts = append(parts, formatSlotRange(start, end))
	return strings.Join(parts, ",")
}

func formatSlotRange(start int, end int) string {
	if start == end {
		return fmt.Sprintf("%d", start)
	}
	return fmt.Sprintf("%d-%d", start, end)
}

func countSlotMatches(memory Memory, list listInfo, uniqueToItem map[uint64]int) (filled int, known int) {
	return countSlotMatchesWithLayout(memory, list, uniqueToItem, defaultSaveDataLayout())
}

func countSlotMatchesWithLayout(memory Memory, list listInfo, uniqueToItem map[uint64]int, layout saveDataLayout) (filled int, known int) {
	if layout.slotUniqueIDOffset == 0 {
		return 0, 0
	}
	limit := list.size
	if limit > 5000 {
		limit = 5000
	}
	for i := 0; i < limit; i++ {
		slot, ok := memory.ReadUintptr(list.arrayPtr + il2cpp.ArrayDataOffset + uintptr(i*8))
		if !ok || slot == 0 || !tbhmem.PlausibleAddress(slot) {
			continue
		}
		uniqueID, ok := memory.ReadUint64(slot + layout.slotUniqueIDOffset)
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
	return countEquippedMatchesWithLayout(memory, heroes, uniqueToItem, defaultSaveDataLayout())
}

func countEquippedMatchesWithLayout(memory Memory, heroes listInfo, uniqueToItem map[uint64]int, layout saveDataLayout) (filled int, known int) {
	if layout.heroEquippedItems == 0 {
		return 0, 0
	}
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
		arrayPtr, ok := memory.ReadUintptr(hero + layout.heroEquippedItems)
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
	return readGoldWithLayout(memory, currencies, defaultSaveDataLayout())
}

func readGoldWithLayout(memory Memory, currencies listInfo, layout saveDataLayout) (uint64, bool) {
	if layout.currencyKeyOffset == 0 || layout.currencyQuantityOffset == 0 {
		return 0, false
	}
	limit := currencies.size
	if limit > 1000 {
		limit = 1000
	}
	for i := 0; i < limit; i++ {
		currency, ok := memory.ReadUintptr(currencies.arrayPtr + il2cpp.ArrayDataOffset + uintptr(i*8))
		if !ok || currency == 0 || !tbhmem.PlausibleAddress(currency) {
			continue
		}
		key, ok := memory.ReadInt32(currency + layout.currencyKeyOffset)
		if !ok || key != goldKey {
			continue
		}
		return memory.ReadUint64(currency + layout.currencyQuantityOffset)
	}
	return 0, false
}

func (resolver *Resolver) readTradeSlots(memory Memory, list listInfo) []TradeShipSlot {
	return resolver.readTradeSlotsWithLayout(memory, list, defaultSaveDataLayout())
}

func (resolver *Resolver) readTradeSlotsWithLayout(memory Memory, list listInfo, layout saveDataLayout) []TradeShipSlot {
	if layout.tradeSlotIndexOffset == 0 || layout.tradeSlotCooldown == 0 || layout.tradeSlotStateOffset == 0 {
		return nil
	}
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
		fields := resolver.instanceFields(memory, readPtr(memory, slotPtr))
		if fields["Index"] > 0 && fields["ItemUniqueId"] > 0 && fields["IsUnLock"] > 0 {
			// The current RemakeTradingStashSaveData stores an item ID here,
			// not the old encoded deadline. Actual deadlines come from live UI.
			index, indexOK := memory.ReadInt32(slotPtr + fields["Index"])
			unlocked, unlockOK := readBoolByte(memory, slotPtr+fields["IsUnLock"])
			if !indexOK || !unlockOK || index < 0 || index > 99 {
				continue
			}
			state := 0
			if unlocked {
				state = 1
			}
			slots = append(slots, TradeShipSlot{Index: int(index), State: state})
			continue
		}
		indexVal, ok := memory.ReadInt32(slotPtr + layout.tradeSlotIndexOffset)
		if !ok {
			continue
		}
		cooldownRaw, ok := memory.ReadUint64(slotPtr + layout.tradeSlotCooldown)
		if !ok {
			continue
		}
		stateVal, ok := memory.ReadInt32(slotPtr + layout.tradeSlotStateOffset)
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

func (resolver *Resolver) readRuntimeTradeSlots(memory Memory, now time.Time, expectedSlots int) []TradeShipSlot {
	// Keep object references, not a snapshot of their cooldowns. Every poll
	// rereads and validates the objects, so active countdown changes stay live.
	readObjects := func() ([]TradeShipSlot, int, []uintptr) {
		byIndex := make(map[int]TradeShipSlot)
		validIndexes := make(map[int32]struct{})
		var validObjects []uintptr
		for _, object := range resolver.runtimeTradeObjects {
			class := readPtr(memory, object)
			if !containsClass(resolver.classCache["TradingStashSlot"], class) || !tbhmem.PlausibleAddress(readPtr(memory, object+0x10)) {
				continue
			}
			indexOffset, textOffset := uintptr(runtimeTradeSlotIndex), uintptr(runtimeTradeSlotCountdownText)
			cooldownOffset, cacheOffset := uintptr(runtimeTradeSlotCooldownUntil), uintptr(runtimeTradeSlotCache)
			fields := resolver.instanceFields(memory, class)
			if len(fields) > 0 {
				// REA/current FieldInfo: Index=0xA8, m_coolTimeText=0xF8,
				// DateTime bkkz=0x100, TradingStashCache bklc=0x118.
				indexOffset, textOffset = fields["Index"], fields["m_coolTimeText"]
				cooldownOffset, cacheOffset = fields["bkkz"], fields["bklc"]
				if indexOffset == 0 || textOffset == 0 || cooldownOffset == 0 || cacheOffset == 0 {
					continue // Unknown named layout; retain save-backed deadlines.
				}
			}
			index, ok := memory.ReadInt32(object + indexOffset)
			if !ok || index < 0 || index > 99 {
				continue
			}
			cache := readPtr(memory, object+cacheOffset)
			if !containsClass(resolver.classCache["TradingStashCache"], readPtr(memory, cache)) {
				continue
			}
			text := readPtr(memory, object+textOffset)
			if !tbhmem.PlausibleAddress(text) {
				continue
			}
			validIndexes[index] = struct{}{}
			validObjects = append(validObjects, object)
			raw, ok := memory.ReadUint64(object + cooldownOffset)
			if !ok {
				continue
			}
			until, ok := dotnetDateTime(raw)
			if !ok || until.Before(now.Add(-24*time.Hour)) || until.After(now.Add(7*24*time.Hour)) {
				continue
			}
			slot := TradeShipSlot{Index: int(index), State: 1, CooldownUntil: until}
			if previous, exists := byIndex[slot.Index]; !exists || slot.CooldownUntil.After(previous.CooldownUntil) {
				byIndex[slot.Index] = slot
			}
		}
		slots := make([]TradeShipSlot, 0, len(byIndex))
		for _, slot := range byIndex {
			slots = append(slots, slot)
		}
		sort.Slice(slots, func(i, j int) bool { return slots[i].Index < slots[j].Index })
		return slots, len(validIndexes), validObjects
	}
	slots, valid, _ := readObjects()
	if valid > 0 && valid >= expectedSlots {
		return slots
	}
	retry := 5 * time.Second
	if len(resolver.runtimeTradeObjects) == 0 {
		retry = 30 * time.Second // Optional UI may be absent; save data still works.
	}
	if !resolver.runtimeTradeAttemptedAt.IsZero() && now.Sub(resolver.runtimeTradeAttemptedAt) < retry {
		return slots
	}
	resolver.runtimeTradeAttemptedAt = now
	classes, ok := resolver.resolveClassByName(memory, "TradingStashSlot")
	if !ok {
		return slots
	}
	if _, ok := resolver.resolveClassByName(memory, "TradingStashCache"); !ok {
		return slots
	}
	resolver.runtimeTradeObjects = nil
	for _, refs := range resolver.scanClassReferences(memory, classes) {
		resolver.runtimeTradeObjects = append(resolver.runtimeTradeObjects, refs...)
	}
	var validObjects []uintptr
	slots, _, validObjects = readObjects()
	resolver.runtimeTradeObjects = validObjects
	return slots
}

func containsClass(classes []uintptr, class uintptr) bool {
	for _, candidate := range classes {
		if candidate == class {
			return true
		}
	}
	return false
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
