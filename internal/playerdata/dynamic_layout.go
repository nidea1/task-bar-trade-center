package playerdata

import (
	"sort"

	"github.com/nidea1/task-bar-trade-center/internal/il2cpp"
	"github.com/nidea1/task-bar-trade-center/internal/tbhmem"
)

const (
	dynamicObjectFieldStart = 0x10
	dynamicObjectFieldEnd   = 0x180
	dynamicEntryFieldStart  = 0x10
	dynamicEntryFieldEnd    = 0xB0
	dynamicSampleLimit      = 512
)

type saveDataLayout struct {
	currenciesOffset uintptr
	heroesOffset     uintptr
	inventoryOffset  uintptr
	stashOffset      uintptr
	tradeSlotsOffset uintptr
	itemsOffset      uintptr

	currencyKeyOffset      uintptr
	currencyQuantityOffset uintptr
	heroKeyOffset          uintptr
	heroEquippedItems      uintptr
	itemKeyOffset          uintptr
	itemUniqueIDOffset     uintptr
	slotIndexOffset        uintptr
	slotUniqueIDOffset     uintptr
	tradeSlotIndexOffset   uintptr
	tradeSlotCooldown      uintptr
	tradeSlotStateOffset   uintptr
}

func defaultSaveDataLayout() saveDataLayout {
	return saveDataLayout{
		currenciesOffset:       playerCurrencies,
		heroesOffset:           playerHeroes,
		inventoryOffset:        playerInventory,
		stashOffset:            playerStash,
		tradeSlotsOffset:       playerTradeSlots,
		itemsOffset:            playerItems,
		currencyKeyOffset:      currencyKey,
		currencyQuantityOffset: currencyQuantity,
		heroKeyOffset:          heroKey,
		heroEquippedItems:      heroEquippedItems,
		itemKeyOffset:          itemSaveItemKey,
		itemUniqueIDOffset:     itemSaveUniqueID,
		slotIndexOffset:        slotIndex,
		slotUniqueIDOffset:     slotUniqueID,
		tradeSlotIndexOffset:   0x10,
		tradeSlotCooldown:      0x18,
		tradeSlotStateOffset:   0x20,
	}
}

func (layout saveDataLayout) valid() bool {
	return layout.itemsOffset != 0 && layout.itemKeyOffset != 0 && layout.itemUniqueIDOffset != 0
}

type objectListCandidate struct {
	offset       uintptr
	info         listInfo
	elementClass string
	classKnown   bool
}

type itemListShape struct {
	keyOffset      uintptr
	uniqueIDOffset uintptr
	valid          int
}

type slotListShape struct {
	uniqueIDOffset   uintptr
	indexOffset      uintptr
	known            int
	filled           int
	sparseIndexCount int
}

type slotListCandidate struct {
	objectListCandidate
	shape slotListShape
}

type heroListShape struct {
	keyOffset           uintptr
	equippedItemsOffset uintptr
	known               int
}

type heroKeyScore struct {
	offset              uintptr
	score               int
	valid               int
	canonical           int
	compact             int
	exactIndex          int
	knownEquipped       int
	knownEquippedUnique int
	unique              int
}

type currencyListShape struct {
	keyOffset      uintptr
	quantityOffset uintptr
	gold           uint64
}

type tradeSlotListShape struct {
	indexOffset    uintptr
	cooldownOffset uintptr
	stateOffset    uintptr
	valid          int
}

func (resolver *Resolver) discoverObjectLayout(memory Memory, object uintptr) (saveDataLayout, candidate, bool) {
	lists := discoverObjectLists(memory, object)
	if len(lists) == 0 {
		return saveDataLayout{}, candidate{}, false
	}

	itemCandidates := lists
	typedItemList, itemClassKnown := findListByElementClass(lists, "ItemSaveData")
	if itemClassKnown {
		itemCandidates = []objectListCandidate{typedItemList}
	}

	var itemList objectListCandidate
	var itemShape itemListShape
	for _, list := range itemCandidates {
		shape, ok := resolver.analyzeItemList(memory, list.info)
		if !ok {
			continue
		}
		if shape.valid > itemShape.valid {
			itemList = list
			itemShape = shape
		}
	}
	if itemShape.valid < 3 {
		return saveDataLayout{}, candidate{}, false
	}

	layout := defaultSaveDataLayout()
	layout.currenciesOffset = 0
	layout.heroesOffset = 0
	layout.inventoryOffset = 0
	layout.stashOffset = 0
	layout.tradeSlotsOffset = 0
	layout.itemsOffset = itemList.offset
	layout.itemKeyOffset = itemShape.keyOffset
	layout.itemUniqueIDOffset = itemShape.uniqueIDOffset
	uniqueToItem, validItems := resolver.readItemSaveDataListWithLayout(memory, itemList.info, layout)
	if validItems < 3 {
		return saveDataLayout{}, candidate{}, false
	}

	score := validItems
	locationEvidence := 0
	structuralEvidence := 0
	if itemClassKnown {
		structuralEvidence++
	}

	inventoryList, inventoryClassKnown := findListByElementClass(lists, "InventorySaveData")
	stashList, stashClassKnown := findListByElementClass(lists, "StashSaveData")
	if inventoryClassKnown || stashClassKnown {
		var typedSlotShapes []slotListCandidate
		if inventoryClassKnown {
			layout.inventoryOffset = inventoryList.offset
			structuralEvidence++
			if shape, ok := analyzeSlotList(memory, inventoryList.info, uniqueToItem); ok {
				typedSlotShapes = append(typedSlotShapes, slotListCandidate{objectListCandidate: inventoryList, shape: shape})
				score += shape.known * 3
				locationEvidence += shape.known
			}
		}
		if stashClassKnown {
			layout.stashOffset = stashList.offset
			structuralEvidence++
			if shape, ok := analyzeSlotList(memory, stashList.info, uniqueToItem); ok {
				typedSlotShapes = append(typedSlotShapes, slotListCandidate{objectListCandidate: stashList, shape: shape})
				score += shape.known * 3
				locationEvidence += shape.known
			}
		}
		applyTypedSlotShape(&layout, typedSlotShapes)
	} else {
		var slotLists []slotListCandidate
		for _, list := range lists {
			if list.offset == layout.itemsOffset {
				continue
			}
			shape, ok := analyzeSlotList(memory, list.info, uniqueToItem)
			if !ok {
				continue
			}
			slotLists = append(slotLists, slotListCandidate{objectListCandidate: list, shape: shape})
			score += shape.known * 3
			locationEvidence += shape.known
		}
		applySlotListsToLayout(&layout, slotLists)
	}

	heroList, heroClassKnown := findListByElementClass(lists, "HeroSaveData")
	if heroClassKnown {
		layout.heroesOffset = heroList.offset
		structuralEvidence++
		if heroShape, ok := analyzeHeroList(memory, heroList.info, uniqueToItem); ok {
			layout.heroKeyOffset = heroShape.keyOffset
			layout.heroEquippedItems = heroShape.equippedItemsOffset
			score += heroShape.known * 3
			locationEvidence += heroShape.known
		}
	} else if heroList, heroShape, ok := analyzeBestHeroList(memory, lists, uniqueToItem, layout.itemsOffset); ok {
		layout.heroesOffset = heroList.offset
		layout.heroKeyOffset = heroShape.keyOffset
		layout.heroEquippedItems = heroShape.equippedItemsOffset
		score += heroShape.known * 3
		locationEvidence += heroShape.known
	}

	var gold uint64
	currencyList, currencyClassKnown := findListByElementClass(lists, "CurrencySaveData")
	if currencyClassKnown {
		layout.currenciesOffset = currencyList.offset
		structuralEvidence++
		if currencyShape, ok := analyzeCurrencyList(memory, currencyList.info); ok {
			layout.currencyKeyOffset = currencyShape.keyOffset
			layout.currencyQuantityOffset = currencyShape.quantityOffset
			gold = currencyShape.gold
			score += 10
		}
	} else if currencyList, currencyShape, ok := analyzeBestCurrencyList(memory, lists, layout.itemsOffset); ok {
		layout.currenciesOffset = currencyList.offset
		layout.currencyKeyOffset = currencyShape.keyOffset
		layout.currencyQuantityOffset = currencyShape.quantityOffset
		gold = currencyShape.gold
		score += 10
	}

	tradeList, tradeClassKnown := findListByElementClass(lists, "RemakeTradingStashSaveData", "TradingStashSaveData")
	if tradeClassKnown {
		layout.tradeSlotsOffset = tradeList.offset
		structuralEvidence++
		if tradeShape, ok := analyzeTradeSlotList(memory, tradeList.info); ok {
			layout.tradeSlotIndexOffset = tradeShape.indexOffset
			layout.tradeSlotCooldown = tradeShape.cooldownOffset
			layout.tradeSlotStateOffset = tradeShape.stateOffset
		}
	} else if tradeList, tradeShape, ok := analyzeBestTradeSlotList(memory, lists, layout.itemsOffset); ok {
		layout.tradeSlotsOffset = tradeList.offset
		layout.tradeSlotIndexOffset = tradeShape.indexOffset
		layout.tradeSlotCooldown = tradeShape.cooldownOffset
		layout.tradeSlotStateOffset = tradeShape.stateOffset
	}

	score += structuralEvidence * 2
	next := candidate{object: object, score: score, gold: gold, layout: layout}
	return layout, next, locationEvidence > 0 || structuralEvidence >= 2
}

func discoverObjectLists(memory Memory, object uintptr) []objectListCandidate {
	lists := make([]objectListCandidate, 0)
	seen := make(map[uintptr]struct{})
	classNames := make(map[uintptr]string)
	for offset := uintptr(dynamicObjectFieldStart); offset <= dynamicObjectFieldEnd; offset += 8 {
		listPtr := readPtr(memory, object+offset)
		if listPtr == 0 {
			continue
		}
		if _, exists := seen[listPtr]; exists {
			continue
		}
		seen[listPtr] = struct{}{}
		info := readListInfo(memory, listPtr, 200000)
		if !info.ok {
			continue
		}
		className, classKnown := dominantListElementClass(memory, info, classNames)
		lists = append(lists, objectListCandidate{offset: offset, info: info, elementClass: className, classKnown: classKnown})
	}
	return lists
}

func findListByElementClass(lists []objectListCandidate, names ...string) (objectListCandidate, bool) {
	for _, name := range names {
		var best objectListCandidate
		for _, list := range lists {
			if !list.classKnown || list.elementClass != name {
				continue
			}
			if best.offset == 0 || list.info.size > best.info.size {
				best = list
			}
		}
		if best.offset != 0 {
			return best, true
		}
	}
	return objectListCandidate{}, false
}

func dominantListElementClass(memory Memory, list listInfo, cache map[uintptr]string) (string, bool) {
	counts := make(map[string]int)
	limit := sampleLimit(list.size, 16)
	for index := 0; index < limit; index++ {
		object, ok := listObjectAt(memory, list, index)
		if !ok {
			continue
		}
		classPtr, ok := memory.ReadUintptr(object + il2cpp.ObjectClassOffset)
		if !ok || classPtr == 0 {
			continue
		}
		name, exists := cache[classPtr]
		if !exists {
			name, ok = readClassName(memory, classPtr)
			if !ok {
				continue
			}
			cache[classPtr] = name
		}
		counts[name]++
	}

	bestName := ""
	bestCount := 0
	for name, count := range counts {
		if count > bestCount {
			bestName = name
			bestCount = count
		}
	}
	return bestName, bestCount > 0
}

func readClassName(memory Memory, classPtr uintptr) (string, bool) {
	reader, ok := memory.(byteMemory)
	if !ok {
		return "", false
	}
	namePtr, ok := memory.ReadUintptr(classPtr + il2cpp.ClassNameOffset)
	if !ok || !tbhmem.PlausibleAddress(namePtr) {
		return "", false
	}
	buffer := make([]byte, 96)
	if !reader.ReadBytes(namePtr, buffer) {
		return "", false
	}
	for index, value := range buffer {
		if value == 0 {
			return string(buffer[:index]), index > 0
		}
		if value < 0x20 || value > 0x7E {
			return "", false
		}
	}
	return "", false
}

type itemFieldPair struct {
	keyOffset      uintptr
	uniqueIDOffset uintptr
}

func (resolver *Resolver) analyzeItemList(memory Memory, list listInfo) (itemListShape, bool) {
	if !list.ok || list.size <= 0 {
		return itemListShape{}, false
	}
	pairCounts := make(map[itemFieldPair]int)
	limit := sampleLimit(list.size, dynamicSampleLimit)
	for i := 0; i < limit; i++ {
		obj, ok := listObjectAt(memory, list, i)
		if !ok {
			continue
		}
		keyOffsets := resolver.knownItemKeyOffsets(memory, obj)
		if len(keyOffsets) == 0 {
			continue
		}
		uniqueOffsets := nonZeroUint64Offsets(memory, obj)
		for _, keyOffset := range keyOffsets {
			for _, uniqueOffset := range uniqueOffsets {
				if fieldsOverlap(keyOffset, 4, uniqueOffset, 8) {
					continue
				}
				pairCounts[itemFieldPair{keyOffset: keyOffset, uniqueIDOffset: uniqueOffset}]++
			}
		}
	}

	var bestPair itemFieldPair
	bestScore := -1
	bestCount := 0
	for pair, count := range pairCounts {
		score := count * 100
		if pair.uniqueIDOffset == pair.keyOffset+8 {
			score += 20
		} else if pair.uniqueIDOffset > pair.keyOffset {
			score += 10
		}
		if score > bestScore {
			bestScore = score
			bestCount = count
			bestPair = pair
		}
	}
	if bestCount < 3 {
		return itemListShape{}, false
	}

	layout := saveDataLayout{itemKeyOffset: bestPair.keyOffset, itemUniqueIDOffset: bestPair.uniqueIDOffset}
	_, valid := resolver.readItemSaveDataListWithLayout(memory, list, layout)
	if valid < 3 {
		return itemListShape{}, false
	}
	return itemListShape{keyOffset: bestPair.keyOffset, uniqueIDOffset: bestPair.uniqueIDOffset, valid: valid}, true
}

func (resolver *Resolver) knownItemKeyOffsets(memory Memory, obj uintptr) []uintptr {
	offsets := make([]uintptr, 0, 2)
	for offset, end := uintptr(dynamicEntryFieldStart), objectFieldEnd(memory, obj, 4); offset <= end; offset += 4 {
		value, ok := memory.ReadInt32(obj + offset)
		if !ok || !resolver.knownItemID(value) {
			continue
		}
		offsets = append(offsets, offset)
	}
	return offsets
}

func (resolver *Resolver) knownItemID(value int32) bool {
	if value <= 0 {
		return false
	}
	if len(resolver.metadata) == 0 {
		return true
	}
	_, exists := resolver.metadata[int(value)]
	return exists
}

func nonZeroUint64Offsets(memory Memory, obj uintptr) []uintptr {
	offsets := make([]uintptr, 0, 2)
	for offset, end := uintptr(dynamicEntryFieldStart), objectFieldEnd(memory, obj, 8); offset <= end; offset += 8 {
		value, ok := memory.ReadUint64(obj + offset)
		if !ok || value == 0 {
			continue
		}
		offsets = append(offsets, offset)
	}
	return offsets
}

func analyzeSlotList(memory Memory, list listInfo, uniqueToItem map[uint64]int) (slotListShape, bool) {
	if !list.ok || list.size <= 0 || len(uniqueToItem) == 0 {
		return slotListShape{}, false
	}
	knownByOffset := make(map[uintptr]int)
	filledByOffset := make(map[uintptr]int)
	limit := sampleLimit(list.size, 5000)
	for i := 0; i < limit; i++ {
		obj, ok := listObjectAt(memory, list, i)
		if !ok {
			continue
		}
		for offset, end := uintptr(dynamicEntryFieldStart), objectFieldEnd(memory, obj, 8); offset <= end; offset += 8 {
			uniqueID, ok := memory.ReadUint64(obj + offset)
			if !ok || uniqueID == 0 {
				continue
			}
			filledByOffset[offset]++
			if _, exists := uniqueToItem[uniqueID]; exists {
				knownByOffset[offset]++
			}
		}
	}

	var uniqueOffset uintptr
	known := 0
	for offset, count := range knownByOffset {
		if count > known {
			known = count
			uniqueOffset = offset
		}
	}
	if known == 0 {
		return slotListShape{}, false
	}
	indexOffset, sparseIndexCount := detectSlotIndexOffset(memory, list, uniqueOffset, uniqueToItem)
	return slotListShape{
		uniqueIDOffset:   uniqueOffset,
		indexOffset:      indexOffset,
		known:            known,
		filled:           filledByOffset[uniqueOffset],
		sparseIndexCount: sparseIndexCount,
	}, true
}

type indexOffsetStats struct {
	valid          int
	nonZero        int
	listIndexMatch int
	sparse         int
}

func detectSlotIndexOffset(memory Memory, list listInfo, uniqueOffset uintptr, uniqueToItem map[uint64]int) (uintptr, int) {
	statsByOffset := make(map[uintptr]indexOffsetStats)
	limit := sampleLimit(list.size, 5000)
	for i := 0; i < limit; i++ {
		obj, ok := listObjectAt(memory, list, i)
		if !ok {
			continue
		}
		uniqueID, ok := memory.ReadUint64(obj + uniqueOffset)
		if !ok {
			continue
		}
		if _, exists := uniqueToItem[uniqueID]; !exists {
			continue
		}
		for offset, end := uintptr(dynamicEntryFieldStart), objectFieldEnd(memory, obj, 4); offset <= end; offset += 4 {
			if fieldsOverlap(offset, 4, uniqueOffset, 8) {
				continue
			}
			value, ok := memory.ReadInt32(obj + offset)
			if !ok || value < 0 || value > 5000 {
				continue
			}
			stats := statsByOffset[offset]
			stats.valid++
			if value != 0 {
				stats.nonZero++
			}
			if int(value) == i {
				stats.listIndexMatch++
			}
			if int(value) >= list.size {
				stats.sparse++
			}
			statsByOffset[offset] = stats
		}
	}

	var bestOffset uintptr
	bestScore := 0
	bestSparse := 0
	for offset, stats := range statsByOffset {
		score := stats.sparse*6 + stats.nonZero*3 + stats.listIndexMatch*2 + stats.valid
		if score > bestScore {
			bestScore = score
			bestOffset = offset
			bestSparse = stats.sparse
		}
	}
	return bestOffset, bestSparse
}

func applySlotListsToLayout(layout *saveDataLayout, slots []slotListCandidate) {
	if len(slots) == 0 {
		return
	}
	sort.Slice(slots, func(i, j int) bool {
		left := slots[i]
		right := slots[j]
		if left.shape.sparseIndexCount != right.shape.sparseIndexCount {
			return left.shape.sparseIndexCount > right.shape.sparseIndexCount
		}
		if left.info.max != right.info.max {
			return left.info.max > right.info.max
		}
		if left.info.size != right.info.size {
			return left.info.size > right.info.size
		}
		return left.shape.known > right.shape.known
	})

	primary := slots[0]
	layout.slotUniqueIDOffset = primary.shape.uniqueIDOffset
	layout.slotIndexOffset = primary.shape.indexOffset
	if len(slots) == 1 && primary.shape.sparseIndexCount == 0 && primary.info.max <= 100 {
		layout.inventoryOffset = primary.offset
		return
	}
	layout.stashOffset = primary.offset
	if len(slots) > 1 {
		layout.inventoryOffset = slots[1].offset
	}
}

func applyTypedSlotShape(layout *saveDataLayout, slots []slotListCandidate) {
	if len(slots) == 0 {
		return
	}
	sort.Slice(slots, func(i, j int) bool {
		return slots[i].shape.known > slots[j].shape.known
	})
	layout.slotUniqueIDOffset = slots[0].shape.uniqueIDOffset
	layout.slotIndexOffset = slots[0].shape.indexOffset
}

func listElementClassMatches(memory Memory, list listInfo, names ...string) (bool, bool) {
	name, known := dominantListElementClass(memory, list, make(map[uintptr]string))
	if !known {
		return false, false
	}
	for _, expected := range names {
		if name == expected {
			return true, true
		}
	}
	return false, true
}

func analyzeBestHeroList(memory Memory, lists []objectListCandidate, uniqueToItem map[uint64]int, itemListOffset uintptr) (objectListCandidate, heroListShape, bool) {
	var bestList objectListCandidate
	var bestShape heroListShape
	for _, list := range lists {
		if list.offset == itemListOffset {
			continue
		}
		shape, ok := analyzeHeroList(memory, list.info, uniqueToItem)
		if !ok {
			continue
		}
		if shape.known > bestShape.known {
			bestList = list
			bestShape = shape
		}
	}
	return bestList, bestShape, bestShape.known > 0
}

func analyzeHeroList(memory Memory, list listInfo, uniqueToItem map[uint64]int) (heroListShape, bool) {
	if !list.ok || list.size <= 0 || len(uniqueToItem) == 0 {
		return heroListShape{}, false
	}
	knownByArrayOffset := make(map[uintptr]int)
	limit := sampleLimit(list.size, 100)
	for i := 0; i < limit; i++ {
		hero, ok := listObjectAt(memory, list, i)
		if !ok {
			continue
		}
		for offset, end := uintptr(dynamicEntryFieldStart), objectFieldEnd(memory, hero, 8); offset <= end; offset += 8 {
			arrayPtr, ok := memory.ReadUintptr(hero + offset)
			if !ok {
				continue
			}
			knownByArrayOffset[offset] += countKnownUniqueIDsInArray(memory, arrayPtr, uniqueToItem)
		}
	}

	var arrayOffset uintptr
	known := 0
	for offset, count := range knownByArrayOffset {
		if count > known {
			known = count
			arrayOffset = offset
		}
	}
	if known == 0 {
		return heroListShape{}, false
	}
	return heroListShape{
		keyOffset:           detectHeroKeyOffset(memory, list, arrayOffset, uniqueToItem),
		equippedItemsOffset: arrayOffset,
		known:               known,
	}, true
}

func detectHeroKeyOffset(memory Memory, list listInfo, arrayOffset uintptr, uniqueToItem map[uint64]int) uintptr {
	best := heroKeyScore{}
	for offset, end := uintptr(dynamicEntryFieldStart), listFieldEnd(memory, list, 4); offset <= end; offset += 4 {
		if fieldsOverlap(offset, 4, arrayOffset, 8) {
			continue
		}
		next := scoreHeroKeyOffset(memory, list, arrayOffset, offset, uniqueToItem)
		if next.betterThan(best) {
			best = next
		}
	}
	if !best.acceptable() {
		return 0
	}
	return best.offset
}

func scoreHeroKeyOffset(memory Memory, list listInfo, arrayOffset uintptr, offset uintptr, uniqueToItem map[uint64]int) heroKeyScore {
	score := heroKeyScore{offset: offset}
	classes := make(map[int]struct{})
	knownEquippedClasses := make(map[int]struct{})
	limit := sampleLimit(list.size, 100)
	for i := 0; i < limit; i++ {
		hero, ok := listObjectAt(memory, list, i)
		if !ok {
			continue
		}
		value, ok := memory.ReadInt32(hero + offset)
		if !ok {
			continue
		}
		classID, keyKindOK := heroClassIDFromKey(int(value))
		if !keyKindOK {
			continue
		}
		score.valid++
		classes[classID] = struct{}{}
		if classID == i+1 {
			score.exactIndex++
		}
		if value >= 101 {
			score.canonical++
		} else {
			score.compact++
		}
		if heroHasKnownEquippedItem(memory, hero, arrayOffset, uniqueToItem) {
			score.knownEquipped++
			knownEquippedClasses[classID] = struct{}{}
		}
	}
	score.unique = len(classes)
	score.knownEquippedUnique = len(knownEquippedClasses)
	score.score = score.valid + score.compact*2 + score.canonical*4 + score.exactIndex*8 + score.unique*12 + score.knownEquipped*20 + score.knownEquippedUnique*50
	if score.knownEquipped > 1 && score.knownEquippedUnique <= 1 {
		score.score -= 1000
	}
	if score.valid > 1 && score.unique <= 1 {
		score.score -= 100
	}
	return score
}

func (score heroKeyScore) acceptable() bool {
	if score.offset == 0 || score.valid == 0 {
		return false
	}
	if score.knownEquipped > 1 {
		return score.knownEquippedUnique > 1
	}
	if score.valid > 1 {
		return score.unique > 1
	}
	return true
}

func (score heroKeyScore) betterThan(other heroKeyScore) bool {
	scoreAcceptable := score.acceptable()
	otherAcceptable := other.acceptable()
	if scoreAcceptable != otherAcceptable {
		return scoreAcceptable
	}
	if score.score != other.score {
		return score.score > other.score
	}
	if score.knownEquippedUnique != other.knownEquippedUnique {
		return score.knownEquippedUnique > other.knownEquippedUnique
	}
	if score.unique != other.unique {
		return score.unique > other.unique
	}
	if score.exactIndex != other.exactIndex {
		return score.exactIndex > other.exactIndex
	}
	if score.canonical != other.canonical {
		return score.canonical > other.canonical
	}
	return other.offset == 0 || score.offset < other.offset
}

func heroClassIDFromKey(value int) (int, bool) {
	if value >= 101 && value <= 601 && value%100 == 1 {
		return value / 100, true
	}
	if value >= 1 && value <= 6 {
		return value, true
	}
	return 0, false
}

func heroHasKnownEquippedItem(memory Memory, hero uintptr, arrayOffset uintptr, uniqueToItem map[uint64]int) bool {
	arrayPtr, ok := memory.ReadUintptr(hero + arrayOffset)
	if !ok {
		return false
	}
	return countKnownUniqueIDsInArray(memory, arrayPtr, uniqueToItem) > 0
}

func countKnownUniqueIDsInArray(memory Memory, arrayPtr uintptr, uniqueToItem map[uint64]int) int {
	if arrayPtr == 0 || !tbhmem.PlausibleAddress(arrayPtr) {
		return 0
	}
	maxLen, ok := memory.ReadUintptr(arrayPtr + il2cpp.ArrayMaxOffset)
	if !ok || maxLen == 0 || maxLen > 100 {
		return 0
	}
	known := 0
	for slot := uintptr(0); slot < maxLen; slot++ {
		uniqueID, ok := memory.ReadUint64(arrayPtr + il2cpp.ArrayDataOffset + slot*8)
		if !ok || uniqueID == 0 {
			continue
		}
		if _, exists := uniqueToItem[uniqueID]; exists {
			known++
		}
	}
	return known
}

func analyzeBestCurrencyList(memory Memory, lists []objectListCandidate, itemListOffset uintptr) (objectListCandidate, currencyListShape, bool) {
	var bestList objectListCandidate
	var bestShape currencyListShape
	for _, list := range lists {
		if list.offset == itemListOffset {
			continue
		}
		shape, ok := analyzeCurrencyList(memory, list.info)
		if !ok {
			continue
		}
		if bestShape.keyOffset == 0 || shape.gold > bestShape.gold {
			bestList = list
			bestShape = shape
		}
	}
	return bestList, bestShape, bestShape.keyOffset != 0
}

type currencyFieldPair struct {
	keyOffset      uintptr
	quantityOffset uintptr
	gold           uint64
}

func analyzeCurrencyList(memory Memory, list listInfo) (currencyListShape, bool) {
	if !list.ok || list.size <= 0 || list.size > 1000 {
		return currencyListShape{}, false
	}
	pairs := make([]currencyFieldPair, 0)
	limit := sampleLimit(list.size, 1000)
	for i := 0; i < limit; i++ {
		obj, ok := listObjectAt(memory, list, i)
		if !ok {
			continue
		}
		for keyOffset, keyEnd := uintptr(dynamicEntryFieldStart), objectFieldEnd(memory, obj, 4); keyOffset <= keyEnd; keyOffset += 4 {
			key, ok := memory.ReadInt32(obj + keyOffset)
			if !ok || key != goldKey {
				continue
			}
			for quantityOffset, quantityEnd := uintptr(dynamicEntryFieldStart), objectFieldEnd(memory, obj, 8); quantityOffset <= quantityEnd; quantityOffset += 8 {
				if fieldsOverlap(keyOffset, 4, quantityOffset, 8) {
					continue
				}
				gold, ok := memory.ReadUint64(obj + quantityOffset)
				if !ok {
					continue
				}
				pairs = append(pairs, currencyFieldPair{keyOffset: keyOffset, quantityOffset: quantityOffset, gold: gold})
			}
		}
	}
	if len(pairs) == 0 {
		return currencyListShape{}, false
	}
	sort.Slice(pairs, func(i, j int) bool {
		left := currencyPairScore(pairs[i])
		right := currencyPairScore(pairs[j])
		if left != right {
			return left > right
		}
		return pairs[i].gold > pairs[j].gold
	})
	best := pairs[0]
	return currencyListShape{keyOffset: best.keyOffset, quantityOffset: best.quantityOffset, gold: best.gold}, true
}

func currencyPairScore(pair currencyFieldPair) int {
	if pair.quantityOffset == pair.keyOffset+8 {
		return 3
	}
	if pair.quantityOffset > pair.keyOffset && pair.quantityOffset-pair.keyOffset <= 0x20 {
		return 2
	}
	return 1
}

func analyzeBestTradeSlotList(memory Memory, lists []objectListCandidate, itemListOffset uintptr) (objectListCandidate, tradeSlotListShape, bool) {
	var bestList objectListCandidate
	var bestShape tradeSlotListShape
	for _, list := range lists {
		if list.offset == itemListOffset {
			continue
		}
		shape, ok := analyzeTradeSlotList(memory, list.info)
		if !ok {
			continue
		}
		if shape.valid > bestShape.valid {
			bestList = list
			bestShape = shape
		}
	}
	return bestList, bestShape, bestShape.valid > 0
}

func analyzeTradeSlotList(memory Memory, list listInfo) (tradeSlotListShape, bool) {
	if !list.ok || list.size <= 0 || list.size > 100 {
		return tradeSlotListShape{}, false
	}
	indexCounts := make(map[uintptr]int)
	stateCounts := make(map[uintptr]int)
	cooldownCounts := make(map[uintptr]int)
	limit := sampleLimit(list.size, 100)
	for i := 0; i < limit; i++ {
		slot, ok := listObjectAt(memory, list, i)
		if !ok {
			continue
		}
		for offset, end := uintptr(dynamicEntryFieldStart), objectFieldEnd(memory, slot, 4); offset <= end; offset += 4 {
			value, ok := memory.ReadInt32(slot + offset)
			if !ok {
				continue
			}
			if value >= 0 && value <= 99 {
				indexCounts[offset]++
			}
			if value >= 0 && value <= 5 {
				stateCounts[offset]++
			}
		}
		for offset, end := uintptr(dynamicEntryFieldStart), objectFieldEnd(memory, slot, 8); offset <= end; offset += 8 {
			value, ok := memory.ReadUint64(slot + offset)
			if !ok {
				continue
			}
			if value == 0 || looksLikeTradeCooldown(value) {
				cooldownCounts[offset]++
			}
		}
	}

	indexOffset, indexCount := bestCountedOffset(indexCounts, 0)
	stateOffset, stateCount := bestCountedOffset(stateCounts, indexOffset)
	cooldownOffset, cooldownCount := bestCountedOffset(cooldownCounts, indexOffset)
	if indexCount == 0 || stateCount == 0 || cooldownCount == 0 {
		return tradeSlotListShape{}, false
	}
	return tradeSlotListShape{
		indexOffset:    indexOffset,
		cooldownOffset: cooldownOffset,
		stateOffset:    stateOffset,
		valid:          minInt(indexCount, minInt(stateCount, cooldownCount)),
	}, true
}

func looksLikeTradeCooldown(value uint64) bool {
	const (
		constantOffset   = uint64(135194695325352348)
		ticksAtUnixEpoch = uint64(621355968000000000)
		ticksPerSecond   = uint64(10000000)
		minUnix          = uint64(1577836800) // 2020-01-01
		maxUnix          = uint64(2208988800) // 2040-01-01
	)
	if value+constantOffset < ticksAtUnixEpoch {
		return false
	}
	unixSecs := (value + constantOffset - ticksAtUnixEpoch) / ticksPerSecond
	return unixSecs >= minUnix && unixSecs <= maxUnix
}

func bestCountedOffset(counts map[uintptr]int, excluded uintptr) (uintptr, int) {
	var bestOffset uintptr
	bestCount := 0
	for offset, count := range counts {
		if excluded != 0 && offset == excluded {
			continue
		}
		if count > bestCount {
			bestOffset = offset
			bestCount = count
		}
	}
	return bestOffset, bestCount
}

func objectFieldEnd(memory Memory, object uintptr, width uintptr) uintptr {
	fallback := uintptr(dynamicEntryFieldEnd)
	classPtr, ok := memory.ReadUintptr(object + il2cpp.ObjectClassOffset)
	if !ok || classPtr == 0 {
		return fallback
	}
	instanceSize, ok := memory.ReadInt32(classPtr + il2cpp.ClassInstanceSizeOffset)
	if !ok || instanceSize < dynamicEntryFieldStart+int32(width) {
		return fallback
	}
	end := uintptr(instanceSize) - width
	if end > fallback {
		return fallback
	}
	return end
}

func listFieldEnd(memory Memory, list listInfo, width uintptr) uintptr {
	limit := sampleLimit(list.size, 16)
	for index := 0; index < limit; index++ {
		object, ok := listObjectAt(memory, list, index)
		if ok {
			return objectFieldEnd(memory, object, width)
		}
	}
	return dynamicEntryFieldEnd
}

func listObjectAt(memory Memory, list listInfo, index int) (uintptr, bool) {
	obj, ok := memory.ReadUintptr(list.arrayPtr + il2cpp.ArrayDataOffset + uintptr(index*8))
	if !ok || obj == 0 || !tbhmem.PlausibleAddress(obj) {
		return 0, false
	}
	return obj, true
}

func sampleLimit(size int, max int) int {
	if size < max {
		return size
	}
	return max
}

func fieldsOverlap(left uintptr, leftSize uintptr, right uintptr, rightSize uintptr) bool {
	return left < right+rightSize && right < left+leftSize
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}
