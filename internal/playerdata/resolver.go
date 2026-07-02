package playerdata

import (
	"encoding/binary"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/il2cpp"
	"github.com/nidea1/task-bar-trade-center/internal/tbhmem"
)

const (
	stashSlotsPerPage = 100

	playerCurrencies = 0x48
	playerHeroes     = 0x50
	playerInventory  = 0x78
	playerStash      = 0x80
	playerTradeSlots = 0x88
	playerItems      = 0xA0

	currencyKey      = 0x10
	currencyQuantity = 0x18
	goldKey          = 100001

	heroKey           = 0x10
	heroEquippedItems = 0x28

	itemSaveItemKey  = 0x10
	itemSaveUniqueID = 0x18
	slotIndex        = 0x10
	slotUniqueID     = 0x18

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

type Resolver struct {
	metadata               map[int]ItemMetadata
	cachedObject           uintptr
	cachedObjectResolvedAt time.Time
	classCache             map[string][]uintptr
	layout                 saveDataLayout
}

type ResolverCache struct {
	CachedObject uintptr
	ClassCache   map[string][]uintptr
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
	stashPageCount := 0
	seen := make(map[uint64]struct{})
	if stash := readObjectListInfo(memory, object, layout.stashOffset, 200000); stash.ok {
		stashPageCount = pageCountForSlotCount(stash.size)
		owned = append(owned, resolver.readSlotItemsWithLayout(memory, stash, uniqueToItem, seen, LocationStash, layout)...)
	}
	if inventory := readObjectListInfo(memory, object, layout.inventoryOffset, 200000); inventory.ok {
		owned = append(owned, resolver.readSlotItemsWithLayout(memory, inventory, uniqueToItem, seen, LocationInventory, layout)...)
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

	return InventorySnapshot{ReadAt: now, Gold: gold, StashPageCount: stashPageCount, Items: owned, TradeSlots: tradeSlots}, true
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
