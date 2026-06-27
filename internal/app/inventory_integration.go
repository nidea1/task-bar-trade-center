package app

import (
	"context"
	"encoding/binary"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/game"
	"github.com/nidea1/task-bar-trade-center/internal/il2cpp"
	"github.com/nidea1/task-bar-trade-center/internal/inventory"
	"github.com/nidea1/task-bar-trade-center/internal/market"
	"github.com/nidea1/task-bar-trade-center/internal/playerdata"
	filestore "github.com/nidea1/task-bar-trade-center/internal/storage"
	"github.com/nidea1/task-bar-trade-center/internal/tbhmem"
)

type cacheQuoteProvider struct {
	scope market.MarketScope
}

const (
	inventoryNotificationActiveInterval = 2 * time.Second
	inventoryNotificationIdleInterval   = 5 * time.Second
	inventoryDashboardRebuildThrottle   = 1500 * time.Millisecond
)

func refreshInventoryDashboardState(reason string) {
	if !activeApp.inventoryDashboardBuildMu.TryLock() {
		logPrintf("[INVENTORY] dashboard refresh skipped (%s): refresh already running\n", reason)
		return
	}
	defer activeApp.inventoryDashboardBuildMu.Unlock()

	state, err := readInventoryDashboardState()
	if err != nil {
		logPrintf("[INVENTORY] dashboard refresh failed (%s): %v\n", reason, err)
		return
	}
	publishInventoryDashboardState(state, reason)
}

func publishInventoryDashboardState(state inventory.DashboardState, reason string) {
	state.Refresh = currentInventoryRefreshStatus()
	activeApp.inventoryMu.Lock()
	activeApp.inventoryDashboardState = state
	activeApp.inventoryMu.Unlock()
	if err := writeInventoryDashboardState(state); err != nil {
		logPrintf("[INVENTORY] dashboard state write failed: %v\n", err)
		return
	}
	callDashboardUpdated(state)
	// Silenced verbose refresh log to avoid clutter
}

func storeInventoryDashboardState(state inventory.DashboardState) {
	activeApp.inventoryMu.Lock()
	activeApp.inventoryDashboardState = state
	activeApp.inventoryMu.Unlock()
}

func currentInventoryDashboardState() inventory.DashboardState {
	activeApp.inventoryMu.Lock()
	defer activeApp.inventoryMu.Unlock()
	return activeApp.inventoryDashboardState
}

func currentInventoryDashboardShellState() inventory.DashboardState {
	return withCurrentDashboardRuntimeFields(inventory.DashboardState{})
}

func withCurrentDashboardRuntimeFields(state inventory.DashboardState) inventory.DashboardState {
	scope := market.CurrentScope()
	state.MarketScope = market.FormatScope(scope)
	state.CurrencyCode = scope.Currency.Code
	state.PricePrefix = scope.Currency.PricePrefix
	state.PriceSuffix = scope.Currency.PriceSuffix
	state.Refresh = currentInventoryRefreshStatus()
	state.Translations = currentTranslations()
	return state
}

func canReadInventorySnapshot() bool {
	return activeApp.gameProcessHandle != 0 && activeApp.gameProcessID != 0
}

func readInventoryDashboardStateLocked() (inventory.DashboardState, error) {
	activeApp.inventoryDashboardBuildMu.Lock()
	defer activeApp.inventoryDashboardBuildMu.Unlock()
	return readInventoryDashboardState()
}

func readInventoryDashboardState() (inventory.DashboardState, error) {
	snapshot, err := readInventorySnapshot()
	if err != nil {
		return inventory.DashboardState{}, err
	}
	storeInventorySnapshotAndNotify(snapshot)

	scope := market.CurrentScope()
	state := inventory.BuildDashboard(snapshot, inventoryItemCatalog(scope), cacheQuoteProvider{scope: scope}, inventory.DashboardOptions{
		MarketScope:  market.FormatScope(scope),
		CurrencyCode: scope.Currency.Code,
		PricePrefix:  scope.Currency.PricePrefix,
		PriceSuffix:  scope.Currency.PriceSuffix,
		Refresh:      currentInventoryRefreshStatus(),
		Now:          time.Now(),
	})
	state.Translations = currentTranslations()
	return state, nil
}

func readInventorySnapshot() (playerdata.InventorySnapshot, error) {
	if !canReadInventorySnapshot() {
		return playerdata.InventorySnapshot{}, fmt.Errorf("game process is not attached")
	}
	memory := tbhmem.FromHandle(activeApp.gameProcessID, activeApp.gameProcessHandle)
	if memory == nil {
		return playerdata.InventorySnapshot{}, fmt.Errorf("game process handle is unavailable")
	}

	resolver := currentInventoryResolver()
	snapshot, ok := resolver.ReadSnapshot(memory, time.Now())
	if !ok {
		return playerdata.InventorySnapshot{}, fmt.Errorf("PlayerSaveData could not be resolved")
	}
	return snapshot, nil
}

func storeInventorySnapshotAndNotify(snapshot playerdata.InventorySnapshot) bool {
	activeApp.inventoryMu.Lock()
	activeApp.lastSnapshot = &snapshot
	activeApp.inventoryMu.Unlock()
	newItems := recordMarketableInventoryItems(snapshot)
	processNewMarketableInventoryItems(newItems)
	return len(newItems) > 0
}

func currentInventoryResolver() *playerdata.Resolver {
	activeApp.inventoryMu.Lock()
	defer activeApp.inventoryMu.Unlock()
	if activeApp.inventoryResolver == nil {
		activeApp.inventoryResolver = playerdata.NewResolver(playerItemMetadata())
	}
	return activeApp.inventoryResolver
}

func playerItemMetadata() map[int]playerdata.ItemMetadata {
	metadata := make(map[int]playerdata.ItemMetadata, len(activeApp.allItemMap))
	for id := range activeApp.allItemMap {
		_, marketable := activeApp.itemMap[id]
		metadata[id] = playerdata.ItemMetadata{Marketable: marketable}
	}
	return metadata
}

func inventoryItemCatalog(scope market.MarketScope) map[int]inventory.ItemDescriptor {
	catalog := make(map[int]inventory.ItemDescriptor, len(activeApp.allItemMap))
	for id, config := range activeApp.allItemMap {
		name := config.Name[currentDisplayLanguage()]
		if name == "" {
			name = config.Name["en-US"]
		}
		gear := ""
		if config.Gear != nil {
			gear = *config.Gear
		}
		marketableConfig, marketable := activeApp.itemMap[id]
		descriptor := inventory.ItemDescriptor{
			Name:       name,
			Grade:      config.Grade,
			Type:       config.Type,
			Gear:       gear,
			Marketable: marketable,
		}
		if marketable {
			hashName := buildMarketHashName(marketableConfig)
			descriptor.MarketHashName = hashName
			descriptor.MarketURL = steamMarketListingURLForScope(marketableConfig, scope)
			if iconPath, ok := marketIconPath(hashName); ok {
				descriptor.IconURL = steamIconImageURL(iconPath)
			} else if cacheData, exists := marketCacheEntry(scope, hashName); exists && cacheData.Analysis.IconURL != "" {
				descriptor.IconURL = steamIconImageURL(cacheData.Analysis.IconURL)
			}
		}
		catalog[id] = descriptor
	}
	return catalog
}

func (provider cacheQuoteProvider) Quote(itemID int) (inventory.PriceQuote, bool) {
	config, exists := activeApp.itemMap[itemID]
	if !exists {
		return inventory.PriceQuote{}, false
	}
	data, exists := marketCacheEntry(provider.scope, buildMarketHashName(config))
	if !exists || data.Analysis.UpdatedAt.IsZero() {
		return inventory.PriceQuote{}, false
	}
	analysis := data.Analysis
	quote := inventory.PriceQuote{
		Suggested:          analysis.SuggestedPrice,
		Instant:            analysis.HighestBuyPrice,
		WeeklyAveragePrice: analysis.WeeklyAveragePrice,
		SpreadPercent:      analysis.SpreadPercent,
		DailySalesVolume:   analysis.DailySalesVolume,
		BuyOrderCount:      analysis.BuyOrderCount,
		SellOrderCount:     analysis.SellOrderCount,
		HasSuggested:       analysis.HasSuggested,
		HasInstant:         analysis.HasHighestBuy,
		HasWeeklyAverage:   analysis.HasWeeklyAverage,
		HasSpread:          analysis.HasSpread,
		HasDailySales:      analysis.HasDailySales,
		HasOrderBook:       analysis.HasOrderBook,
		Confidence:         analysis.Confidence,
		HasConfidence:      analysis.HasConfidence,
		PricePrefix:        analysis.PricePrefix,
		PriceSuffix:        analysis.PriceSuffix,
		UpdatedAt:          analysis.UpdatedAt.Format(time.RFC3339),
	}
	return quote, quote.HasSuggested || quote.HasInstant
}

func writeInventoryDashboardState(state inventory.DashboardState) error {
	if activeApp.inventoryStateFilePath == "" {
		return nil
	}
	return filestore.WriteJSONAtomic(activeApp.inventoryStateFilePath, state)
}

func openInventoryDashboard() {
	go refreshInventoryDashboardState("open-dashboard")
	callOpenDashboard()
}

func monitorInventoryNotifications(processHandle uintptr) {
	const logPollInterval = 2 * time.Second

	// Run first save poll immediately to seed the initial dashboard state on attach
	pollInventoryNotifications()

	for {
		time.Sleep(logPollInterval)
		if processHandle == 0 || game.HasProcessExited(processHandle) || activeApp.gameProcessHandle != processHandle {
			return
		}

		// 1. Instant LogManager poll (extremely fast)
		pollLogManagerNotifications()

		// 2. Event-driven Save snapshot poll (only when stage ends)
		if triggerSavePollNow {
			pollInventoryNotifications()
			refreshInventoryPricesFromDashboard()
			triggerSavePollNow = false
		}
	}
}

var lastPollErr error

func pollInventoryNotifications() bool {
	snapshot, err := readInventorySnapshot()
	if err != nil {
		if lastPollErr == nil || lastPollErr.Error() != err.Error() {
			logPrintf("[INVENTORY:poll] Failed to read inventory snapshot: %v\n", err)
			lastPollErr = err
		}
		return false
	}
	if lastPollErr != nil {
		logPrintf("[INVENTORY:poll] Recovered: Successfully read inventory snapshot\n")
		lastPollErr = nil
	}

	activeApp.inventoryMu.Lock()
	prevSnapshot := activeApp.lastSnapshot
	activeApp.inventoryMu.Unlock()

	changed := false
	if prevSnapshot == nil || inventorySnapshotChanged(prevSnapshot, &snapshot) {
		changed = true
	}

	if changed {
		prevCount := 0
		if prevSnapshot != nil {
			prevCount = len(prevSnapshot.Items)
		}
		logPrintf("[INVENTORY:poll] Snapshot changed. Previous items count: %d, current items count: %d\n", prevCount, len(snapshot.Items))
	}

	newItems := storeInventorySnapshotAndNotify(snapshot)

	if changed {
		go refreshInventoryDashboardState("inventory-changed")
	}

	return newItems || changed
}

func inventorySnapshotChanged(a, b *playerdata.InventorySnapshot) bool {
	if a == nil || b == nil {
		return a != b
	}
	if a.Gold != b.Gold || a.StashPageCount != b.StashPageCount {
		return true
	}
	if len(a.Items) != len(b.Items) {
		return true
	}
	aMap := make(map[uint64]playerdata.OwnedItem, len(a.Items))
	for _, item := range a.Items {
		aMap[item.UniqueID] = item
	}
	for _, item := range b.Items {
		match, exists := aMap[item.UniqueID]
		if !exists {
			return true
		}
		if match.ItemID != item.ItemID ||
			match.Location != item.Location ||
			match.EquippedHeroKey != item.EquippedHeroKey ||
			match.SlotIndex != item.SlotIndex ||
			match.Marketable != item.Marketable {
			return true
		}
	}
	return false
}

func refreshInventoryPricesFromDashboard() {
	state, err := readInventoryDashboardState()
	if err != nil {
		logPrintf("[INVENTORY] cannot queue price refresh: %v\n", err)
		return
	}
	added := queueInventoryPriceRefresh(state)
	logPrintf("[INVENTORY] queued inventory price refresh: added=%d\n", added)
	publishInventoryDashboardState(state, "price-refresh-queued")
}

func forceRefreshInventoryPricesFromDashboard() {
	state, err := readInventoryDashboardState()
	if err != nil {
		logPrintf("[INVENTORY] cannot queue force price refresh: %v\n", err)
		return
	}
	added := queueForceInventoryPriceRefresh(state)
	logPrintf("[INVENTORY] queued force inventory price refresh: added=%d\n", added)
	publishInventoryDashboardState(state, "force-price-refresh-queued")
}

func queueInventoryPriceRefresh(state inventory.DashboardState) int {
	ids := missingOrStaleDashboardItemIDs(state, 5*time.Minute)
	if len(ids) == 0 {
		logPrintln("[INVENTORY] no inventory prices need refresh.")
		return 0
	}
	added := currentInventoryPriceQueue().Enqueue(ids)
	logPrintf("[INVENTORY] queued inventory price refresh: added=%d total_candidates=%d\n", added, len(ids))
	return added
}

func queueForceInventoryPriceRefresh(state inventory.DashboardState) int {
	ids := dashboardMarketableItemIDs(state)
	if len(ids) == 0 {
		logPrintln("[INVENTORY] no inventory prices available for force refresh.")
		return 0
	}
	added := currentInventoryPriceQueue().Enqueue(ids)
	logPrintf("[INVENTORY] queued force inventory price refresh: added=%d total_candidates=%d\n", added, len(ids))
	return added
}

func dashboardMarketableItemIDs(state inventory.DashboardState) []int {
	ids := make([]int, 0, len(state.Items))
	seen := make(map[int]struct{}, len(state.Items))
	for _, item := range state.Items {
		if item.ItemID <= 0 {
			continue
		}
		if _, exists := seen[item.ItemID]; exists {
			continue
		}
		seen[item.ItemID] = struct{}{}
		ids = append(ids, item.ItemID)
	}
	return ids
}

func missingOrStaleDashboardItemIDs(state inventory.DashboardState, maxAge time.Duration) []int {
	now := time.Now()
	ids := make([]int, 0)
	for _, item := range state.Items {
		if item.MarketHashName == "" || !marketIconMetadataFresh(item.MarketHashName, now) {
			ids = append(ids, item.ItemID)
			continue
		}
		if item.IconURL == "" || !item.HasPrice || item.UpdatedAt == "" {
			ids = append(ids, item.ItemID)
			continue
		}
		if parsed, err := time.Parse(time.RFC3339, item.UpdatedAt); err != nil || now.Sub(parsed) > maxAge {
			ids = append(ids, item.ItemID)
		}
	}
	return ids
}

func currentInventoryPriceQueue() *inventory.RefreshQueue {
	activeApp.inventoryMu.Lock()
	defer activeApp.inventoryMu.Unlock()
	if activeApp.inventoryPriceQueue == nil {
		activeApp.inventoryPriceQueue = inventory.NewRefreshQueue(fetchInventoryMarketPrice, isSteamRateLimitError)
	}
	return activeApp.inventoryPriceQueue
}

func currentInventoryRefreshStatus() inventory.RefreshStatus {
	queue := currentInventoryPriceQueue()
	if queue == nil {
		return inventory.RefreshStatus{}
	}
	return queue.Status()
}

func fetchInventoryMarketPrice(_ context.Context, itemID int) error {
	config, exists := activeApp.itemMap[itemID]
	if !exists {
		return nil
	}
	scope := market.CurrentScope()
	marketHashName := buildMarketHashName(config)
	now := time.Now()
	data, err := fetchMarketData(config, marketHashName, now, scope)
	if err != nil {
		return err
	}
	if data.Analysis.IconURL != "" {
		recordMarketIcon(marketHashName, data.Analysis.IconURL, now)
	}
	data = retainIconMetadataURL(data, marketHashName)
	activeApp.priceCacheMu.Lock()
	activeApp.priceCache[market.CacheKey(scope, marketHashName)] = data
	schedulePriceCacheWriteLocked()
	activeApp.priceCacheMu.Unlock()
	requestInventoryDashboardRebuild("price-refreshed")
	return nil
}

func isSteamRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "429") || strings.Contains(message, "too many")
}

func rebuildDashboardState(reason string) (inventory.DashboardState, bool) {
	activeApp.inventoryMu.Lock()
	snapshot := activeApp.lastSnapshot
	activeApp.inventoryMu.Unlock()

	if snapshot == nil {
		// If we don't have a cached snapshot in memory, and the game is running, we can read memory.
		if activeApp.gameProcessHandle != 0 && activeApp.gameProcessID != 0 {
			go refreshInventoryDashboardState(reason)
			return currentInventoryDashboardState(), false
		}
		// If not running, update the translations and scope on the cached dashboard state.
		activeApp.inventoryMu.Lock()
		state := activeApp.inventoryDashboardState
		scope := market.CurrentScope()
		state.MarketScope = market.FormatScope(scope)
		state.CurrencyCode = scope.Currency.Code
		state.PricePrefix = scope.Currency.PricePrefix
		state.PriceSuffix = scope.Currency.PriceSuffix
		state.Translations = currentTranslations()
		activeApp.inventoryDashboardState = state
		activeApp.inventoryMu.Unlock()

		writeInventoryDashboardState(state)
		callDashboardUpdated(state)
		return state, state.UpdatedAt != ""
	}

	scope := market.CurrentScope()
	state := inventory.BuildDashboard(*snapshot, inventoryItemCatalog(scope), cacheQuoteProvider{scope: scope}, inventory.DashboardOptions{
		MarketScope:  market.FormatScope(scope),
		CurrencyCode: scope.Currency.Code,
		PricePrefix:  scope.Currency.PricePrefix,
		PriceSuffix:  scope.Currency.PriceSuffix,
		Refresh:      currentInventoryRefreshStatus(),
		Now:          time.Now(),
	})
	state.Translations = currentTranslations()

	activeApp.inventoryMu.Lock()
	activeApp.inventoryDashboardState = state
	activeApp.inventoryMu.Unlock()

	writeInventoryDashboardState(state)
	callDashboardUpdated(state)
	logPrintf("[INVENTORY] dashboard state rebuilt (%s): items=%d marketable=%d priced=%d\n",
		reason, state.Totals.TotalItemCount, state.Totals.MarketableItemCount, state.Totals.PricedItemCount)
	return state, true
}

func requestInventoryDashboardRebuild(reason string) {
	activeApp.inventoryDashboardRebuildMu.Lock()
	activeApp.pendingInventoryRebuildReason = reason
	if activeApp.inventoryDashboardRebuildTimer == nil {
		activeApp.inventoryDashboardRebuildTimer = time.AfterFunc(inventoryDashboardRebuildThrottle, flushInventoryDashboardRebuild)
	} else {
		activeApp.inventoryDashboardRebuildTimer.Reset(inventoryDashboardRebuildThrottle)
	}
	activeApp.inventoryDashboardRebuildMu.Unlock()
}

func flushInventoryDashboardRebuild() {
	activeApp.inventoryDashboardRebuildMu.Lock()
	reason := activeApp.pendingInventoryRebuildReason
	activeApp.pendingInventoryRebuildReason = ""
	activeApp.inventoryDashboardRebuildTimer = nil
	activeApp.inventoryDashboardRebuildMu.Unlock()

	if reason == "" {
		reason = "price-refreshed"
	}
	rebuildDashboardState(reason)
}

func flushInventoryDashboardRebuildNow() {
	activeApp.inventoryDashboardRebuildMu.Lock()
	if activeApp.inventoryDashboardRebuildTimer != nil {
		activeApp.inventoryDashboardRebuildTimer.Stop()
		activeApp.inventoryDashboardRebuildTimer = nil
	}
	reason := activeApp.pendingInventoryRebuildReason
	activeApp.pendingInventoryRebuildReason = ""
	activeApp.inventoryDashboardRebuildMu.Unlock()

	if reason == "" {
		return
	}
	rebuildDashboardState(reason)
}

type boxNotificationTracker struct {
	count        int
	lastNotified time.Time
}

var (
	logManagerInstance       uintptr
	lastLogCount             int
	recentlyNotifiedBoxItems = make(map[int]boxNotificationTracker)
	notifiedBoxItemsMu       sync.Mutex
	lastProcessHandle        uintptr
	triggerSavePollNow       bool
)

type localListInfo struct {
	ptr      uintptr
	arrayPtr uintptr
	size     int
	max      int
	ok       bool
}

func readStringLocal(p *tbhmem.Process, addr uintptr) string {
	if addr == 0 {
		return ""
	}
	buf := make([]byte, 256)
	if !p.ReadBytes(addr, buf) {
		return ""
	}
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i])
		}
	}
	return string(buf)
}

func readCSharpStringLocal(p *tbhmem.Process, strPtr uintptr) string {
	if strPtr == 0 {
		return ""
	}
	length, ok := p.ReadInt32(strPtr + 0x10)
	if !ok || length <= 0 || length > 1000 {
		return ""
	}
	buf := make([]byte, length*2)
	if !p.ReadBytes(strPtr+0x14, buf) {
		return ""
	}
	runes := make([]rune, length)
	for i := 0; i < int(length); i++ {
		runes[i] = rune(binary.LittleEndian.Uint16(buf[i*2 : i*2+2]))
	}
	return string(runes)
}

func pickListSingletonLocal(p *tbhmem.Process, candidates []uintptr, offset uintptr, cap int) uintptr {
	best, bestSz := uintptr(0), -1
	for _, a := range candidates {
		listPtr, _ := p.ReadUintptr(a + offset)
		if listPtr == 0 {
			continue
		}
		info := localReadListInfo(p, listPtr, cap)
		if info.ok && info.size > bestSz {
			best = a
			bestSz = info.size
		}
	}
	return best
}

func localReadListInfo(p *tbhmem.Process, listPtr uintptr, maxAllowed int) localListInfo {
	if listPtr == 0 || !tbhmem.PlausibleAddress(listPtr) {
		return localListInfo{ptr: listPtr}
	}
	arrayPtr, ok := p.ReadUintptr(listPtr + 0x10)
	if !ok || arrayPtr == 0 || !tbhmem.PlausibleAddress(arrayPtr) {
		return localListInfo{ptr: listPtr}
	}
	size32, ok := p.ReadInt32(listPtr + 0x18)
	if !ok || size32 < 0 || int(size32) > maxAllowed {
		return localListInfo{ptr: listPtr, arrayPtr: arrayPtr}
	}
	max64, ok := p.ReadUintptr(arrayPtr + 0x18)
	if !ok || max64 > uintptr(maxAllowed) || int(max64) < int(size32) {
		return localListInfo{ptr: listPtr, arrayPtr: arrayPtr, size: int(size32)}
	}
	return localListInfo{ptr: listPtr, arrayPtr: arrayPtr, size: int(size32), max: int(max64), ok: true}
}

func pollLogManagerNotifications() {
	if !canReadInventorySnapshot() {
		return
	}
	memory := tbhmem.FromHandle(activeApp.gameProcessID, activeApp.gameProcessHandle)
	if memory == nil {
		return
	}

	if activeApp.gameProcessHandle != lastProcessHandle {
		logManagerInstance = 0
		lastLogCount = 0
		lastProcessHandle = activeApp.gameProcessHandle
	}

	// 1. If we don't have logManagerInstance, resolve it
	if logManagerInstance == 0 {
		classes, ok := il2cpp.ResolveClassByName(memory, "LogManager")
		if !ok || len(classes) == 0 {
			return
		}
		var instances []uintptr
		for _, classPtr := range classes {
			ptrBytes := make([]byte, 8)
			binary.LittleEndian.PutUint64(ptrBytes, uint64(classPtr))
			refs, _ := memory.ScanPattern(ptrBytes, 1000)
			instances = append(instances, refs...)
		}
		if len(instances) == 0 {
			return
		}
		lm := pickListSingletonLocal(memory, instances, 0x20, 100000)
		if lm == 0 {
			return
		}
		logManagerInstance = lm
		logPrintf("[NOTIFY] Resolved LogManager instance at 0x%X\n", logManagerInstance)

		// Seed initial size so we don't notify old chest opens
		logListPtr, _ := memory.ReadUintptr(logManagerInstance + 0x20)
		if logListPtr != 0 {
			if arrayPtr, ok := memory.ReadUintptr(logListPtr + 0x10); ok && arrayPtr != 0 {
				if size32, ok := memory.ReadInt32(logListPtr + 0x18); ok && size32 >= 0 {
					lastLogCount = int(size32)
					logPrintf("[NOTIFY] Seeded LogManager LOG_LIST size: %d\n", lastLogCount)
				}
			}
		}
		return
	}

	// 2. Read LOG_LIST size
	logListPtr, _ := memory.ReadUintptr(logManagerInstance + 0x20)
	if logListPtr == 0 {
		logManagerInstance = 0 // lost handle? Reset
		return
	}

	arrayPtr, ok1 := memory.ReadUintptr(logListPtr + 0x10)
	size32, ok2 := memory.ReadInt32(logListPtr + 0x18)
	if !ok1 || !ok2 || arrayPtr == 0 || size32 < 0 {
		return
	}

	size := int(size32)
	if size == lastLogCount {
		return
	}

	if size < lastLogCount {
		lastLogCount = size
		return
	}

	// 3. Process new logs
	newItems := make([]marketableInventoryItem, 0)
	for i := lastLogCount; i < size; i++ {
		logDataPtr, ok := memory.ReadUintptr(arrayPtr + 0x20 + uintptr(i)*8)
		if !ok || logDataPtr == 0 {
			continue
		}
		classPtr, ok := memory.ReadUintptr(logDataPtr + 0x0)
		if !ok || classPtr == 0 {
			continue
		}
		namePtr, ok := memory.ReadUintptr(classPtr + 0x10)
		if !ok || namePtr == 0 {
			continue
		}
		className := readStringLocal(memory, namePtr)
		if className == "BoxOpenLog" {
			itemNameStrPtr, ok := memory.ReadUintptr(logDataPtr + 0x40)
			if !ok || itemNameStrPtr == 0 {
				continue
			}
			itemNameStr := readCSharpStringLocal(memory, itemNameStrPtr)
			var itemID int
			_, err := fmt.Sscanf(itemNameStr, "ItemName_%d", &itemID)
			if err != nil || itemID == 0 {
				continue
			}

			// Check if marketable
			activeApp.inventoryMu.Lock()
			config, exists := activeApp.itemMap[itemID]
			activeApp.inventoryMu.Unlock()

			if exists {
				notifiedBoxItemsMu.Lock()
				tracker := recentlyNotifiedBoxItems[itemID]
				if time.Since(tracker.lastNotified) > 5*time.Minute {
					tracker.count = 0
				}
				tracker.count++
				tracker.lastNotified = time.Now()
				recentlyNotifiedBoxItems[itemID] = tracker
				// Cleanup old entries
				for k, v := range recentlyNotifiedBoxItems {
					if time.Since(v.lastNotified) > 5*time.Minute {
						delete(recentlyNotifiedBoxItems, k)
					}
				}
				notifiedBoxItemsMu.Unlock()

				logPrintf("[NOTIFY] Instant BoxOpenLog item detected: itemID=%d name=%s\n", itemID, config.Name["en-US"])
				newItems = append(newItems, marketableInventoryItem{
					itemID: itemID,
				})
			}
		} else if className == "StageClearLog" || className == "StageFailedLog" {
			logPrintf("[NOTIFY] Stage end event detected (%s). Triggering inventory sync on next tick.\n", className)
			triggerSavePollNow = true
		}
	}

	lastLogCount = size

	if len(newItems) > 0 {
		for index := range newItems {
			fillMarketableInventoryItemDetails(&newItems[index])
		}
		processNewMarketableInventoryItems(newItems)
	}
}
