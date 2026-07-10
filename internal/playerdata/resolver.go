package playerdata

import (
	"encoding/binary"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/il2cpp"
	"github.com/nidea1/task-bar-trade-center/internal/tbhmem"
)

const (
	stashSlotsPerPage = 100

	playerCurrencies = 0x58
	playerHeroes     = 0x60
	playerInventory  = 0x88
	playerStash      = 0x90
	playerTradeSlots = 0x98
	playerItems      = 0xB0

	currencyKey      = 0x10
	currencyQuantity = 0x18
	goldKey          = 100001

	heroKey           = 0x10
	heroEquippedItems = 0x30

	itemSaveItemKey  = 0x10
	itemSaveUniqueID = 0x18
	slotIndex        = 0x10
	slotUniqueID     = 0x18
	slotIsUnlock     = 0x20
	slotIsRuneUnlock = 0x21

	playerSerializedBoxData = 0x20
	sBoxDataBoxTypes        = 0x10
	sBoxDataBoxQuantity     = 0x20

	maxClassRefs = 50000
)

type Memory interface {
	ReadUintptr(address uintptr) (uintptr, bool)
	ReadUint64(address uintptr) (uint64, bool)
	ReadInt32(address uintptr) (int32, bool)
	ScanPattern(pattern []byte, maxResults int) ([]uintptr, uint64)
}

type multiPatternMemory interface {
	ScanPatterns(patterns [][]byte, maxResults int) ([][]uintptr, uint64)
}

type byteMemory interface {
	ReadBytes(address uintptr, destination []byte) bool
}

type Resolver struct {
	metadata               map[int]ItemMetadata
	cachedObject           uintptr
	cachedObjectResolvedAt time.Time
	classCache             map[string][]uintptr
	layout                 saveDataLayout
}

type SaveDataLayout struct {
	CurrenciesOffset       uintptr `json:"currencies_offset"`
	HeroesOffset           uintptr `json:"heroes_offset"`
	InventoryOffset        uintptr `json:"inventory_offset"`
	StashOffset            uintptr `json:"stash_offset"`
	TradeSlotsOffset       uintptr `json:"trade_slots_offset"`
	ItemsOffset            uintptr `json:"items_offset"`
	CurrencyKeyOffset      uintptr `json:"currency_key_offset"`
	CurrencyQuantityOffset uintptr `json:"currency_quantity_offset"`
	HeroKeyOffset          uintptr `json:"hero_key_offset"`
	HeroEquippedItems      uintptr `json:"hero_equipped_items"`
	ItemKeyOffset          uintptr `json:"item_key_offset"`
	ItemUniqueIDOffset     uintptr `json:"item_unique_id_offset"`
	SlotIndexOffset        uintptr `json:"slot_index_offset"`
	SlotUniqueIDOffset     uintptr `json:"slot_unique_id_offset"`
	TradeSlotIndexOffset   uintptr `json:"trade_slot_index_offset"`
	TradeSlotCooldown      uintptr `json:"trade_slot_cooldown"`
	TradeSlotStateOffset   uintptr `json:"trade_slot_state_offset"`
}

type ResolverCache struct {
	CachedObject uintptr
	ClassCache   map[string][]uintptr
	Layout       SaveDataLayout
}

type listInfo struct {
	ptr      uintptr
	arrayPtr uintptr
	size     int
	max      int
	ok       bool
}

type candidate struct {
	object uintptr
	score  int
	gold   uint64
	layout saveDataLayout
}

func NewResolver(metadata map[int]ItemMetadata) *Resolver {
	copied := make(map[int]ItemMetadata, len(metadata))
	for key, value := range metadata {
		copied[key] = value
	}
	return &Resolver{metadata: copied}
}

func (resolver *Resolver) ReadSnapshot(memory Memory, now time.Time) (InventorySnapshot, bool) {
	return resolver.readSnapshot(memory, now, true)
}

func (resolver *Resolver) ReadSnapshotCore(memory Memory, now time.Time) (InventorySnapshot, bool) {
	return resolver.readSnapshot(memory, now, false)
}

func (resolver *Resolver) readSnapshot(memory Memory, now time.Time, includeRuntimeTradeSlots bool) (InventorySnapshot, bool) {
	if now.IsZero() {
		now = time.Now()
	}
	if resolver.cachedObject != 0 {
		if snapshot, ok := resolver.readObject(memory, resolver.cachedObject, now, includeRuntimeTradeSlots); ok {
			return snapshot, true
		}
		resolver.cachedObject = 0
		resolver.classCache = nil // Clear class cache so we perform a full scan
	}

	if snapshot, ok := resolver.resolveAndReadObject(memory, now, includeRuntimeTradeSlots); ok {
		return snapshot, true
	}

	if resolver.cachedObject != 0 {
		if snapshot, ok := resolver.readObject(memory, resolver.cachedObject, now, includeRuntimeTradeSlots); ok {
			return snapshot, true
		}
		resolver.cachedObject = 0
	}
	return InventorySnapshot{}, false
}

func (resolver *Resolver) resolveAndReadObject(memory Memory, now time.Time, includeRuntimeTradeSlots bool) (InventorySnapshot, bool) {
	classes, ok := resolver.resolveClassByName(memory, "PlayerSaveData")
	if !ok {
		return InventorySnapshot{}, false
	}

	var best candidate
	for _, refs := range resolver.scanClassReferences(memory, classes) {
		for _, ref := range refs {
			if ref < il2cpp.ObjectClassOffset {
				continue
			}
			object := ref - il2cpp.ObjectClassOffset
			next, ok := resolver.validateObject(memory, object)
			if ok && betterCandidate(next, best) {
				best = next
			}
		}
	}
	if best.object == 0 {
		return InventorySnapshot{}, false
	}
	resolver.cachedObject = best.object
	resolver.cachedObjectResolvedAt = now
	resolver.layout = best.layout
	return resolver.readObject(memory, best.object, now, includeRuntimeTradeSlots)
}

func (resolver *Resolver) scanClassReferences(memory Memory, classes []uintptr) [][]uintptr {
	patterns := make([][]byte, 0, len(classes))
	for _, classPtr := range classes {
		ptrBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(ptrBytes, uint64(classPtr))
		patterns = append(patterns, ptrBytes)
	}
	if scanner, ok := memory.(multiPatternMemory); ok && len(patterns) > 1 {
		refs, _ := scanner.ScanPatterns(patterns, maxClassRefs)
		return refs
	}
	refs := make([][]uintptr, len(patterns))
	for index, pattern := range patterns {
		refs[index], _ = memory.ScanPattern(pattern, maxClassRefs)
	}
	return refs
}

func (resolver *Resolver) resolveClassByName(memory Memory, name string) ([]uintptr, bool) {
	if resolver.classCache == nil {
		resolver.classCache = make(map[string][]uintptr)
	}
	if cached, exists := resolver.classCache[name]; exists {
		return append([]uintptr(nil), cached...), len(cached) > 0
	}
	classes, ok := il2cpp.ResolveClassByName(memory, name)
	if !ok || len(classes) == 0 {
		return nil, false
	}
	resolver.classCache[name] = append([]uintptr(nil), classes...)
	return classes, true
}

func (resolver *Resolver) ExportCache() ResolverCache {
	cache := ResolverCache{
		CachedObject: resolver.cachedObject,
		ClassCache:   make(map[string][]uintptr, len(resolver.classCache)),
		Layout: SaveDataLayout{
			CurrenciesOffset:       resolver.layout.currenciesOffset,
			HeroesOffset:           resolver.layout.heroesOffset,
			InventoryOffset:        resolver.layout.inventoryOffset,
			StashOffset:            resolver.layout.stashOffset,
			TradeSlotsOffset:       resolver.layout.tradeSlotsOffset,
			ItemsOffset:            resolver.layout.itemsOffset,
			CurrencyKeyOffset:      resolver.layout.currencyKeyOffset,
			CurrencyQuantityOffset: resolver.layout.currencyQuantityOffset,
			HeroKeyOffset:          resolver.layout.heroKeyOffset,
			HeroEquippedItems:      resolver.layout.heroEquippedItems,
			ItemKeyOffset:          resolver.layout.itemKeyOffset,
			ItemUniqueIDOffset:     resolver.layout.itemUniqueIDOffset,
			SlotIndexOffset:        resolver.layout.slotIndexOffset,
			SlotUniqueIDOffset:     resolver.layout.slotUniqueIDOffset,
			TradeSlotIndexOffset:   resolver.layout.tradeSlotIndexOffset,
			TradeSlotCooldown:      resolver.layout.tradeSlotCooldown,
			TradeSlotStateOffset:   resolver.layout.tradeSlotStateOffset,
		},
	}
	for name, addresses := range resolver.classCache {
		cache.ClassCache[name] = append([]uintptr(nil), addresses...)
	}
	return cache
}

func (resolver *Resolver) ImportCache(cache ResolverCache) {
	resolver.cachedObject = cache.CachedObject
	if cache.CachedObject != 0 {
		resolver.cachedObjectResolvedAt = time.Now()
	}
	resolver.layout = saveDataLayout{
		currenciesOffset:       cache.Layout.CurrenciesOffset,
		heroesOffset:           cache.Layout.HeroesOffset,
		inventoryOffset:        cache.Layout.InventoryOffset,
		stashOffset:            cache.Layout.StashOffset,
		tradeSlotsOffset:       cache.Layout.TradeSlotsOffset,
		itemsOffset:            cache.Layout.ItemsOffset,
		currencyKeyOffset:      cache.Layout.CurrencyKeyOffset,
		currencyQuantityOffset: cache.Layout.CurrencyQuantityOffset,
		heroKeyOffset:          cache.Layout.HeroKeyOffset,
		heroEquippedItems:      cache.Layout.HeroEquippedItems,
		itemKeyOffset:          cache.Layout.ItemKeyOffset,
		itemUniqueIDOffset:     cache.Layout.ItemUniqueIDOffset,
		slotIndexOffset:        cache.Layout.SlotIndexOffset,
		slotUniqueIDOffset:     cache.Layout.SlotUniqueIDOffset,
		tradeSlotIndexOffset:   cache.Layout.TradeSlotIndexOffset,
		tradeSlotCooldown:      cache.Layout.TradeSlotCooldown,
		tradeSlotStateOffset:   cache.Layout.TradeSlotStateOffset,
	}
	if len(cache.ClassCache) == 0 {
		return
	}
	if resolver.classCache == nil {
		resolver.classCache = make(map[string][]uintptr, len(cache.ClassCache))
	}
	for name, addresses := range cache.ClassCache {
		if len(addresses) == 0 {
			continue
		}
		resolver.classCache[name] = append([]uintptr(nil), addresses...)
	}
}

func betterCandidate(next candidate, best candidate) bool {
	if best.object == 0 {
		return true
	}
	if next.gold != best.gold {
		return next.gold > best.gold
	}
	return next.score > best.score
}

func (resolver *Resolver) validateObject(memory Memory, object uintptr) (candidate, bool) {
	if resolver.layout.valid() {
		if next, ok := resolver.validateObjectWithLayout(memory, object, resolver.layout); ok {
			return next, true
		}
	}

	if next, ok := resolver.validateObjectWithLayout(memory, object, defaultSaveDataLayout()); ok {
		return next, true
	}

	_, next, ok := resolver.discoverObjectLayout(memory, object)
	return next, ok
}

func (resolver *Resolver) validateObjectWithLayout(memory Memory, object uintptr, layout saveDataLayout) (candidate, bool) {
	items := readObjectListInfo(memory, object, layout.itemsOffset, 200000)
	if !items.ok || items.size <= 0 {
		return candidate{}, false
	}
	uniqueToItem, validItems := resolver.readItemSaveDataListWithLayout(memory, items, layout)
	if validItems < 3 {
		return candidate{}, false
	}

	score := validItems
	if inventory := readObjectListInfo(memory, object, layout.inventoryOffset, 200000); inventory.ok {
		_, known := countSlotMatchesWithLayout(memory, inventory, uniqueToItem, layout)
		score += known * 3
	}
	if stash := readObjectListInfo(memory, object, layout.stashOffset, 200000); stash.ok {
		_, known := countSlotMatchesWithLayout(memory, stash, uniqueToItem, layout)
		score += known * 3
	}
	if heroes := readObjectListInfo(memory, object, layout.heroesOffset, 1000); heroes.ok {
		_, known := countEquippedMatchesWithLayout(memory, heroes, uniqueToItem, layout)
		score += known * 3
	}
	var gold uint64
	if currencies := readObjectListInfo(memory, object, layout.currenciesOffset, 1000); currencies.ok {
		if value, ok := readGoldWithLayout(memory, currencies, layout); ok {
			gold = value
			score += 10
		}
	}
	return candidate{object: object, score: score, gold: gold, layout: layout}, score > validItems
}

func (resolver *Resolver) readObject(memory Memory, object uintptr, now time.Time, includeRuntimeTradeSlots bool) (InventorySnapshot, bool) {
	if resolver.layout.valid() {
		if snapshot, ok := resolver.readObjectWithLayout(memory, object, now, includeRuntimeTradeSlots, resolver.layout); ok {
			return snapshot, true
		}
		resolver.layout = saveDataLayout{}
	}

	if snapshot, ok := resolver.readObjectWithLayout(memory, object, now, includeRuntimeTradeSlots, defaultSaveDataLayout()); ok {
		return snapshot, true
	}

	if layout, _, ok := resolver.discoverObjectLayout(memory, object); ok {
		resolver.layout = layout
		return resolver.readObjectWithLayout(memory, object, now, includeRuntimeTradeSlots, layout)
	}
	return InventorySnapshot{}, false
}

func (resolver *Resolver) readObjectWithLayout(memory Memory, object uintptr, now time.Time, includeRuntimeTradeSlots bool, layout saveDataLayout) (InventorySnapshot, bool) {
	items := readObjectListInfo(memory, object, layout.itemsOffset, 200000)
	if !items.ok {
		return InventorySnapshot{}, false
	}
	uniqueToItem, validItems := resolver.readItemSaveDataListWithLayout(memory, items, layout)
	if validItems < 3 {
		return InventorySnapshot{}, false
	}

	var owned []OwnedItem
	var inventorySlots InventorySlotSummary
	stashPageCount := 0
	seen := make(map[uint64]struct{})
	if stash := readObjectListInfo(memory, object, layout.stashOffset, 200000); stash.ok {
		stashPageCount = pageCountForSlotCount(stash.size)
		owned = append(owned, resolver.readSlotItemsWithLayout(memory, stash, uniqueToItem, seen, LocationStash, layout)...)
	}
	if inventory, inventoryLayout, inventorySource, ok := resolver.readInventorySaveDataList(memory, object, layout); ok {
		inventoryItems := resolver.readSlotItemsWithLayout(memory, inventory.info, uniqueToItem, seen, LocationInventory, inventoryLayout)
		inventorySlots = readInventorySlotSummaryWithLayout(memory, inventory.info, inventoryLayout)
		inventorySlots.Source = inventorySource
		owned = append(owned, inventoryItems...)
	}
	// Storage slots are the stronger current-location signal when a save-backed hero
	// equipped array lags after an unequip.
	if heroes := readObjectListInfo(memory, object, layout.heroesOffset, 1000); heroes.ok {
		owned = append(owned, resolver.readEquippedItemsWithLayout(memory, heroes, uniqueToItem, seen, layout)...)
	}

	var gold uint64
	if currencies := readObjectListInfo(memory, object, layout.currenciesOffset, 1000); currencies.ok {
		gold, _ = readGoldWithLayout(memory, currencies, layout)
	}

	var tradeSlots []TradeShipSlot
	if trade := readObjectListInfo(memory, object, layout.tradeSlotsOffset, 100); trade.ok {
		tradeSlots = resolver.readTradeSlotsWithLayout(memory, trade, layout)
	}
	if includeRuntimeTradeSlots {
		if runtimeTradeSlots := resolver.readRuntimeTradeSlots(memory, now); len(runtimeTradeSlots) > 0 {
			tradeSlots = runtimeTradeSlots
		}
	}

	return InventorySnapshot{
		ReadAt:          now,
		Gold:            gold,
		StashPageCount:  stashPageCount,
		InventorySlots:  inventorySlots,
		ChestAutomation: resolver.readChestAutomationWithLayout(memory, object, layout),
		ChestCounts:     readChestCounts(memory, object),
		Items:           owned,
		TradeSlots:      tradeSlots,
	}, true
}

func readChestCounts(memory Memory, object uintptr) ChestCounts {
	sBoxDataPtr := readPtr(memory, object+playerSerializedBoxData)
	if sBoxDataPtr == 0 {
		return ChestCounts{}
	}
	boxTypesPtr := readPtr(memory, sBoxDataPtr+sBoxDataBoxTypes)
	boxQtyPtr := readPtr(memory, sBoxDataPtr+sBoxDataBoxQuantity)
	if boxTypesPtr == 0 || boxQtyPtr == 0 {
		return ChestCounts{}
	}
	typesInfo := readListInfo(memory, boxTypesPtr, 200)
	qtyInfo := readListInfo(memory, boxQtyPtr, 200)
	if !typesInfo.ok || !qtyInfo.ok || typesInfo.size == 0 {
		return ChestCounts{}
	}
	normal, stage := 0, 0
	for i := 0; i < typesInfo.size; i++ {
		boxType, ok := memory.ReadInt32(typesInfo.arrayPtr + il2cpp.ArrayDataOffset + uintptr(i)*4)
		if !ok {
			continue
		}
		qty := int32(1)
		if i < qtyInfo.size {
			if q, ok := memory.ReadInt32(qtyInfo.arrayPtr + il2cpp.ArrayDataOffset + uintptr(i)*4); ok {
				qty = q
			}
		}
		if qty <= 0 {
			continue
		}
		if boxType == 0 {
			normal += int(qty)
		} else if boxType == 1 {
			stage += int(qty)
		}
	}
	return ChestCounts{
		NormalChests: normal,
		StageChests:  stage,
		TotalChests:  normal + stage,
	}
}

func pageCountForSlotCount(slotCount int) int {
	if slotCount <= 0 {
		return 0
	}
	return (slotCount + stashSlotsPerPage - 1) / stashSlotsPerPage
}

func readPtr(memory Memory, address uintptr) uintptr {
	value, _ := memory.ReadUintptr(address)
	return value
}

func readObjectListInfo(memory Memory, object uintptr, offset uintptr, maxAllowed int) listInfo {
	if offset == 0 {
		return listInfo{}
	}
	return readListInfo(memory, readPtr(memory, object+offset), maxAllowed)
}

func (resolver *Resolver) readInventorySaveDataList(memory Memory, object uintptr, layout saveDataLayout) (objectListCandidate, saveDataLayout, string, bool) {
	if list, ok := resolver.findInventorySaveDataList(memory, object); ok {
		inventoryLayout := layout
		inventoryLayout.slotUniqueIDOffset = slotUniqueID
		inventoryLayout.slotIndexOffset = 0
		return list, inventoryLayout, "InventorySaveData", true
	}
	info := readObjectListInfo(memory, object, layout.inventoryOffset, 200000)
	if !info.ok {
		return objectListCandidate{}, layout, "", false
	}
	return objectListCandidate{offset: layout.inventoryOffset, info: info}, layout, "layout", true
}

func (resolver *Resolver) findInventorySaveDataList(memory Memory, object uintptr) (objectListCandidate, bool) {
	classes, ok := resolver.resolveClassByName(memory, "InventorySaveData")
	if !ok {
		return objectListCandidate{}, false
	}
	classSet := make(map[uintptr]struct{}, len(classes))
	for _, classPtr := range classes {
		classSet[classPtr] = struct{}{}
	}

	if direct := readObjectListInfo(memory, object, playerInventory, 200000); direct.ok {
		matches := countListObjectsWithClass(memory, direct, classSet)
		if matches > 0 {
			return objectListCandidate{offset: playerInventory, info: direct}, true
		}
	}

	var best objectListCandidate
	bestMatches := 0
	for _, list := range discoverObjectLists(memory, object) {
		matches := countListObjectsWithClass(memory, list.info, classSet)
		if matches > bestMatches || (matches == bestMatches && matches > 0 && list.offset == playerInventory) {
			best = list
			bestMatches = matches
		}
	}
	return best, bestMatches > 0
}

func countListObjectsWithClass(memory Memory, list listInfo, classes map[uintptr]struct{}) int {
	if !list.ok || list.size <= 0 || len(classes) == 0 {
		return 0
	}
	matches := 0
	limit := sampleLimit(list.size, 5000)
	for index := 0; index < limit; index++ {
		object, ok := listObjectAt(memory, list, index)
		if !ok {
			continue
		}
		if objectHasAnyClass(memory, object, classes) {
			matches++
		}
	}
	return matches
}

func readListInfo(memory Memory, listPtr uintptr, maxAllowed int) listInfo {
	if listPtr == 0 || !tbhmem.PlausibleAddress(listPtr) {
		return listInfo{ptr: listPtr}
	}
	arrayPtr, ok := memory.ReadUintptr(listPtr + il2cpp.ListItemsOffset)
	if !ok || arrayPtr == 0 || !tbhmem.PlausibleAddress(arrayPtr) {
		return listInfo{ptr: listPtr}
	}
	size32, ok := memory.ReadInt32(listPtr + il2cpp.ListSizeOffset)
	if !ok || size32 < 0 || int(size32) > maxAllowed {
		return listInfo{ptr: listPtr, arrayPtr: arrayPtr}
	}
	max64, ok := memory.ReadUintptr(arrayPtr + il2cpp.ArrayMaxOffset)
	if !ok || max64 > uintptr(maxAllowed) || int(max64) < int(size32) {
		return listInfo{ptr: listPtr, arrayPtr: arrayPtr, size: int(size32)}
	}
	return listInfo{ptr: listPtr, arrayPtr: arrayPtr, size: int(size32), max: int(max64), ok: true}
}
