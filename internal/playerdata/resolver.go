package playerdata

import (
	"encoding/binary"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/il2cpp"
	"github.com/nidea1/task-bar-trade-center/internal/tbhmem"
)

const (
	stashSlotsPerPage  = 100
	cachedObjectMaxAge = 5 * time.Minute

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

type Resolver struct {
	metadata               map[int]ItemMetadata
	cachedObject           uintptr
	cachedObjectResolvedAt time.Time
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
}

func NewResolver(metadata map[int]ItemMetadata) *Resolver {
	copied := make(map[int]ItemMetadata, len(metadata))
	for key, value := range metadata {
		copied[key] = value
	}
	return &Resolver{metadata: copied}
}

func (resolver *Resolver) ReadSnapshot(memory Memory, now time.Time) (InventorySnapshot, bool) {
	if now.IsZero() {
		now = time.Now()
	}
	if resolver.cachedObject != 0 && now.Sub(resolver.cachedObjectResolvedAt) < cachedObjectMaxAge {
		if snapshot, ok := resolver.readObject(memory, resolver.cachedObject, now); ok {
			return snapshot, true
		}
		resolver.cachedObject = 0
	}

	if snapshot, ok := resolver.resolveAndReadObject(memory, now); ok {
		return snapshot, true
	}

	if resolver.cachedObject != 0 {
		if snapshot, ok := resolver.readObject(memory, resolver.cachedObject, now); ok {
			return snapshot, true
		}
		resolver.cachedObject = 0
	}
	return InventorySnapshot{}, false
}

func (resolver *Resolver) resolveAndReadObject(memory Memory, now time.Time) (InventorySnapshot, bool) {
	classes, ok := il2cpp.ResolveClassByName(memory, "PlayerSaveData")
	if !ok {
		return InventorySnapshot{}, false
	}

	var best candidate
	for _, classPtr := range classes {
		ptrBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(ptrBytes, uint64(classPtr))
		refs, _ := memory.ScanPattern(ptrBytes, maxClassRefs)
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
	return resolver.readObject(memory, best.object, now)
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
	items := readListInfo(memory, readPtr(memory, object+playerItems), 200000)
	if !items.ok || items.size <= 0 {
		return candidate{}, false
	}
	uniqueToItem, validItems := resolver.readItemSaveDataList(memory, items)
	if validItems < 3 {
		return candidate{}, false
	}

	score := validItems
	if inventory := readListInfo(memory, readPtr(memory, object+playerInventory), 200000); inventory.ok {
		_, known := countSlotMatches(memory, inventory, uniqueToItem)
		score += known * 3
	}
	if stash := readListInfo(memory, readPtr(memory, object+playerStash), 200000); stash.ok {
		_, known := countSlotMatches(memory, stash, uniqueToItem)
		score += known * 3
	}
	if heroes := readListInfo(memory, readPtr(memory, object+playerHeroes), 1000); heroes.ok {
		_, known := countEquippedMatches(memory, heroes, uniqueToItem)
		score += known * 3
	}
	var gold uint64
	if currencies := readListInfo(memory, readPtr(memory, object+playerCurrencies), 1000); currencies.ok {
		if value, ok := readGold(memory, currencies); ok {
			gold = value
			score += 10
		}
	}
	return candidate{object: object, score: score, gold: gold}, score > validItems
}

func (resolver *Resolver) readObject(memory Memory, object uintptr, now time.Time) (InventorySnapshot, bool) {
	items := readListInfo(memory, readPtr(memory, object+playerItems), 200000)
	if !items.ok {
		return InventorySnapshot{}, false
	}
	uniqueToItem, validItems := resolver.readItemSaveDataList(memory, items)
	if validItems < 3 {
		return InventorySnapshot{}, false
	}

	var owned []OwnedItem
	stashPageCount := 0
	seen := make(map[uint64]struct{})
	if stash := readListInfo(memory, readPtr(memory, object+playerStash), 200000); stash.ok {
		stashPageCount = pageCountForSlotCount(stash.size)
		owned = append(owned, resolver.readSlotItems(memory, stash, uniqueToItem, seen, LocationStash)...)
	}
	if inventory := readListInfo(memory, readPtr(memory, object+playerInventory), 200000); inventory.ok {
		owned = append(owned, resolver.readSlotItems(memory, inventory, uniqueToItem, seen, LocationInventory)...)
	}
	// Storage slots are the stronger current-location signal when a save-backed hero
	// equipped array lags after an unequip.
	if heroes := readListInfo(memory, readPtr(memory, object+playerHeroes), 1000); heroes.ok {
		owned = append(owned, resolver.readEquippedItems(memory, heroes, uniqueToItem, seen)...)
	}

	var gold uint64
	if currencies := readListInfo(memory, readPtr(memory, object+playerCurrencies), 1000); currencies.ok {
		gold, _ = readGold(memory, currencies)
	}

	var tradeSlots []TradeShipSlot
	if trade := readListInfo(memory, readPtr(memory, object+playerTradeSlots), 100); trade.ok {
		tradeSlots = resolver.readTradeSlots(memory, trade)
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
