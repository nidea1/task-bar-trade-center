package app

import (
	"context"
	"encoding/binary"
	"fmt"
	"strconv"
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
	directInventoryNotificationWindow   = 30 * time.Minute
	inventorySnapshotResolveBackoff     = 5 * time.Second
)

func refreshInventoryDashboardState(reason string) {
	if !activeApp.inventoryDashboardBuildMu.TryLock() {
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
	return buildInventoryDashboardState(snapshot), nil
}

func readInventoryDashboardStateCore() (inventory.DashboardState, error) {
	snapshot, err := readInventorySnapshotCore()
	if err != nil {
		return inventory.DashboardState{}, err
	}
	return buildInventoryDashboardState(snapshot), nil
}

func buildInventoryDashboardState(snapshot playerdata.InventorySnapshot) inventory.DashboardState {
	storeInventorySnapshot(snapshot)

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
	return state
}

func readInventorySnapshot() (playerdata.InventorySnapshot, error) {
	return readInventorySnapshotWithRuntime(true)
}

func readInventorySnapshotCore() (playerdata.InventorySnapshot, error) {
	return readInventorySnapshotWithRuntime(false)
}

func readInventorySnapshotWithRuntime(includeRuntimeTradeSlots bool) (playerdata.InventorySnapshot, error) {
	if !canReadInventorySnapshot() {
		return playerdata.InventorySnapshot{}, fmt.Errorf("game process is not attached")
	}

	processID := activeApp.gameProcessID
	processHandle := activeApp.gameProcessHandle
	gameAssemblyBase := activeApp.gameAssemblyBase
	now := time.Now()
	activeApp.inventorySnapshotMu.Lock()
	if activeApp.inventorySnapshotBackoffUntil.After(now) {
		errText := activeApp.inventorySnapshotLastErr
		if errText == "" {
			errText = "PlayerSaveData could not be resolved"
		}
		backoffUntil := activeApp.inventorySnapshotBackoffUntil
		activeApp.inventorySnapshotMu.Unlock()
		return playerdata.InventorySnapshot{}, fmt.Errorf("%s; retry after %s", errText, backoffUntil.Format(time.RFC3339))
	}
	if call := activeApp.inventorySnapshotInFlight; call != nil {
		activeApp.inventorySnapshotMu.Unlock()
		<-call.done
		if activeApp.gameProcessID != call.processID || activeApp.gameProcessHandle != call.processHandle {
			return playerdata.InventorySnapshot{}, fmt.Errorf("game process changed during inventory snapshot")
		}
		return call.snapshot, call.err
	}
	call := &inventorySnapshotCall{done: make(chan struct{}), processID: processID, processHandle: processHandle}
	activeApp.inventorySnapshotInFlight = call
	activeApp.inventorySnapshotMu.Unlock()

	call.snapshot, call.err = readInventorySnapshotUncoordinated(includeRuntimeTradeSlots, processID, processHandle, gameAssemblyBase)
	staleProcess := activeApp.gameProcessID != processID || activeApp.gameProcessHandle != processHandle
	if staleProcess && call.err == nil {
		call.err = fmt.Errorf("game process changed during inventory snapshot")
	}

	activeApp.inventorySnapshotMu.Lock()
	if activeApp.inventorySnapshotInFlight == call {
		activeApp.inventorySnapshotInFlight = nil
	}
	if staleProcess {
		activeApp.inventorySnapshotMu.Unlock()
		close(call.done)
		return playerdata.InventorySnapshot{}, call.err
	}
	if call.err != nil {
		activeApp.inventorySnapshotLastErr = call.err.Error()
		activeApp.inventorySnapshotBackoffUntil = time.Now().Add(inventorySnapshotResolveBackoff)
	} else {
		activeApp.inventorySnapshotLastErr = ""
		activeApp.inventorySnapshotBackoffUntil = time.Time{}
	}
	close(call.done)
	activeApp.inventorySnapshotMu.Unlock()

	return call.snapshot, call.err
}

func readInventorySnapshotUncoordinated(includeRuntimeTradeSlots bool, processID uint32, processHandle uintptr, gameAssemblyBase uintptr) (playerdata.InventorySnapshot, error) {
	memory := tbhmem.FromHandle(processID, processHandle)
	if memory == nil {
		return playerdata.InventorySnapshot{}, fmt.Errorf("game process handle is unavailable")
	}
	loggingMemory := inventoryScanMemory{Process: memory, reason: "inventory-snapshot"}
	resolver := currentInventoryResolver()
	startedAt := time.Now()
	var snapshot playerdata.InventorySnapshot
	var ok bool
	if includeRuntimeTradeSlots {
		snapshot, ok = resolver.ReadSnapshot(loggingMemory, startedAt)
	} else {
		snapshot, ok = resolver.ReadSnapshotCore(loggingMemory, startedAt)
	}
	status := "ok"
	if !ok {
		status = "failed"
		logPrintf("[INVENTORY:resolve] snapshot include_runtime_trade_slots=%t duration=%s status=%s\n", includeRuntimeTradeSlots, time.Since(startedAt), status)
		return playerdata.InventorySnapshot{}, fmt.Errorf("PlayerSaveData could not be resolved")
	}
	saveInventoryResolverCache(processHandle, gameAssemblyBase, resolver)
	logPrintf("[INVENTORY:resolve] snapshot include_runtime_trade_slots=%t duration=%s status=%s items=%d trade_slots=%d\n", includeRuntimeTradeSlots, time.Since(startedAt), status, len(snapshot.Items), len(snapshot.TradeSlots))
	return snapshot, nil
}

type inventoryScanMemory struct {
	*tbhmem.Process
	reason string
}

func (memory inventoryScanMemory) ScanPattern(pattern []byte, maxResults int) ([]uintptr, uint64) {
	startedAt := time.Now()
	results, scanned := memory.Process.ScanPattern(pattern, maxResults)
	logPrintf("[MEMSCAN] reason=%s pattern_len=%d max_results=%d results=%d scanned_bytes=%d duration=%s\n",
		memory.reason,
		len(pattern),
		maxResults,
		len(results),
		scanned,
		time.Since(startedAt),
	)
	return results, scanned
}

func (memory inventoryScanMemory) ScanPatterns(patterns [][]byte, maxResults int) ([][]uintptr, uint64) {
	startedAt := time.Now()
	results, scanned := memory.Process.ScanPatterns(patterns, maxResults)
	resultCount := 0
	for _, matches := range results {
		resultCount += len(matches)
	}
	logPrintf("[MEMSCAN] reason=%s pattern_count=%d max_results=%d results=%d scanned_bytes=%d duration=%s\n",
		memory.reason,
		len(patterns),
		maxResults,
		resultCount,
		scanned,
		time.Since(startedAt),
	)
	return results, scanned
}

func storeInventorySnapshot(snapshot playerdata.InventorySnapshot) {
	activeApp.inventoryMu.Lock()
	activeApp.lastSnapshot = &snapshot
	activeApp.inventoryMu.Unlock()
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
	if runtimeReady() {
		go refreshInventoryDashboardState("open-dashboard")
	}
	callOpenDashboard()
}

var (
	observedTradeSlots = make(map[int]time.Time)
	notifiedTradeSlots = make(map[int]time.Time)
	tradeSlotsNotifyMu sync.Mutex
)

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

		// 3. Trade Ship cooldown check (non-blocking, memory-only)
		checkTradeSlotCooldowns()
	}
}

func checkTradeSlotCooldowns() {
	activeApp.inventoryMu.Lock()
	snapshot := activeApp.lastSnapshot
	activeApp.inventoryMu.Unlock()

	if snapshot == nil {
		return
	}

	now := time.Now()
	for _, slot := range snapshot.TradeSlots {
		if slot.State != 1 || slot.CooldownUntil.IsZero() {
			forgetTradeSlotCooldown(slot.Index)
			continue
		}

		if !shouldNotifyTradeSlotCooldown(slot.Index, slot.CooldownUntil, now) {
			continue
		}

		title := tr("notification.trade_ship_title")
		body := tr("notification.trade_ship_body", slot.Index+1)
		if title == "" || title == "notification.trade_ship_title" {
			title = "Steam Trade Ship"
		}
		if body == "" || body == "notification.trade_ship_body" {
			body = fmt.Sprintf("Slot %d Voyage Completed!", slot.Index+1)
		}
		queueRawTrayNotification(fmt.Sprintf("%s\n%s", title, body))
	}
}

func shouldNotifyTradeSlotCooldown(index int, cooldownUntil time.Time, now time.Time) bool {
	tradeSlotsNotifyMu.Lock()
	defer tradeSlotsNotifyMu.Unlock()

	if !now.After(cooldownUntil) {
		observedTradeSlots[index] = cooldownUntil
		return false
	}

	if lastNotified, exists := notifiedTradeSlots[index]; exists && lastNotified.Equal(cooldownUntil) {
		return false
	}

	lastObserved, observed := observedTradeSlots[index]
	if !observed || !lastObserved.Equal(cooldownUntil) {
		observedTradeSlots[index] = cooldownUntil
		notifiedTradeSlots[index] = cooldownUntil
		logPrintf("[NOTIFY] Trade Ship slot %d cooldown %s was already due on first observation. Suppressing startup/stale notification.\n", index+1, cooldownUntil.Format(time.RFC3339))
		return false
	}

	notifiedTradeSlots[index] = cooldownUntil
	return true
}

func forgetTradeSlotCooldown(index int) {
	tradeSlotsNotifyMu.Lock()
	delete(observedTradeSlots, index)
	delete(notifiedTradeSlots, index)
	tradeSlotsNotifyMu.Unlock()
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

	storeInventorySnapshot(snapshot)

	if changed {
		go refreshInventoryDashboardState("inventory-changed")
	}

	return changed
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
	if len(a.TradeSlots) != len(b.TradeSlots) {
		return true
	}
	for i := range a.TradeSlots {
		if a.TradeSlots[i].Index != b.TradeSlots[i].Index ||
			a.TradeSlots[i].State != b.TradeSlots[i].State ||
			!a.TradeSlots[i].CooldownUntil.Equal(b.TradeSlots[i].CooldownUntil) {
			return true
		}
	}
	return false
}

func refreshInventoryPricesFromDashboard() {
	if !runtimeReady() {
		logPrintln("[INVENTORY] price refresh skipped: runtime is still preparing.")
		return
	}
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
	if !runtimeReady() {
		logPrintln("[INVENTORY] force price refresh skipped: runtime is still preparing.")
		return
	}
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
		activeApp.inventoryPriceQueue.OnBackoff = func(itemID int, err error, queueRemaining int, backoffUntil time.Time) {
			var marketHashName string
			config, exists := activeApp.itemMap[itemID]
			if exists {
				marketHashName = buildMarketHashName(config)
			}

			endpoint := "unknown"
			retryAfter := ""
			status := 429
			if se, ok := err.(*market.SteamError); ok {
				endpoint = se.Endpoint
				retryAfter = se.RetryAfter
				status = se.StatusCode
			}

			logPrintf("[MARKET] request failed:\nitem_id=%d\nmarket_hash_name=%q\nendpoint=%s\nstatus=%d\nqueue_remaining=%d\nretry_after=%q\nbackoff_until=%s\n",
				itemID,
				marketHashName,
				endpoint,
				status,
				queueRemaining,
				retryAfter,
				backoffUntil.Format(time.RFC3339),
			)
		}
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
	priority := market.RequestPriorityNormal
	if activeApp.showOverlay.Load() && activeApp.activeItemID.Load() == int32(itemID) {
		priority = market.RequestPriorityHigh
	}
	data, err := fetchMarketDataWithPriority(config, marketHashName, now, scope, priority)
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
	updatePriceOverlay(itemID, scope, data.Analysis)
	requestInventoryDashboardRebuild("price-refreshed")
	return nil
}

func isSteamRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	if se, ok := err.(*market.SteamError); ok {
		return se.StatusCode == 429
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

func inventoryInteractionResultSource(className string) (string, bool) {
	switch className {
	case "SynthesisResultLog", "SynthesisResult", "SynthesisLog", "Synthesis":
		return notificationSourceSynthesis, true
	case "CraftingResultLog", "CraftingResult", "CraftingLog", "Crafting":
		return notificationSourceCraft, true
	case "CraftResultLog", "CraftResult", "CubeResultLog", "CubeResult":
		return notificationSourceCraft, true
	case "CubeDecorationLog", "CubeEngravingLog", "CubeInscriptionLog", "CubeExtractionLog":
		return "", false
	case "OfferingResultLog", "OfferingResult", "OfferingLog", "Offering":
		return notificationSourceOffering, true
	}
	lowerClassName := strings.ToLower(className)
	switch {
	case strings.Contains(lowerClassName, "synthesis"):
		return notificationSourceSynthesis, true
	case strings.Contains(lowerClassName, "cubedecoration"), strings.Contains(lowerClassName, "cubeengraving"), strings.Contains(lowerClassName, "cubeinscription"), strings.Contains(lowerClassName, "cubeextraction"):
		return "", false
	case strings.Contains(lowerClassName, "craft"), strings.Contains(lowerClassName, "cube"):
		return notificationSourceCraft, true
	case strings.Contains(lowerClassName, "offering"):
		return notificationSourceOffering, true
	}
	return "", false
}

func itemIDFromItemNameKey(value string) (int, bool) {
	suffix, ok := strings.CutPrefix(value, "ItemName_")
	if !ok || suffix == "" {
		return 0, false
	}
	itemID, err := strconv.Atoi(suffix)
	return itemID, err == nil && itemID > 0
}

func readLogItemPayload(p *tbhmem.Process, logDataPtr uintptr) (int, string, string, bool) {
	for offset := uintptr(0x40); offset <= 0xA0; offset += 8 {
		strPtr, ok := p.ReadUintptr(logDataPtr + offset)
		if !ok || strPtr == 0 {
			continue
		}
		value := readCSharpStringLocal(p, strPtr)
		itemID, ok := itemIDFromItemNameKey(value)
		if ok {
			return itemID, value, fmt.Sprintf("str@0x%X", offset), true
		}
		if itemID, ok := knownMarketableItemID(p, logDataPtr+offset); ok {
			return itemID, fmt.Sprintf("%d", itemID), fmt.Sprintf("i32@0x%X", offset), true
		}
		if itemID, value, path, ok := readNestedLogItemPayload(p, strPtr, fmt.Sprintf("ptr@0x%X", offset), 1); ok {
			return itemID, value, path, true
		}
	}
	return 0, "", "", false
}

func readNestedLogItemPayload(p *tbhmem.Process, objectPtr uintptr, basePath string, depth int) (int, string, string, bool) {
	if depth < 0 || objectPtr == 0 || !tbhmem.PlausibleAddress(objectPtr) {
		return 0, "", "", false
	}
	for offset := uintptr(0x10); offset <= 0xC0; offset += 8 {
		fieldAddr := objectPtr + offset
		strPtr, ok := p.ReadUintptr(fieldAddr)
		if ok && strPtr != 0 {
			value := readCSharpStringLocal(p, strPtr)
			if itemID, ok := itemIDFromItemNameKey(value); ok {
				return itemID, value, fmt.Sprintf("%s.str@0x%X", basePath, offset), true
			}
			if depth > 0 {
				if itemID, value, path, ok := readNestedLogItemPayload(p, strPtr, fmt.Sprintf("%s.ptr@0x%X", basePath, offset), depth-1); ok {
					return itemID, value, path, true
				}
			}
		}
	}
	for offset := uintptr(0x10); offset <= 0xC0; offset += 4 {
		if itemID, ok := knownMarketableItemID(p, objectPtr+offset); ok {
			return itemID, fmt.Sprintf("%d", itemID), fmt.Sprintf("%s.i32@0x%X", basePath, offset), true
		}
	}
	return 0, "", "", false
}

func knownMarketableItemID(p *tbhmem.Process, address uintptr) (int, bool) {
	value, ok := p.ReadInt32(address)
	if !ok || value <= 0 {
		return 0, false
	}
	itemID := int(value)
	activeApp.inventoryMu.Lock()
	_, exists := activeApp.itemMap[itemID]
	activeApp.inventoryMu.Unlock()
	return itemID, exists
}

func logInventoryInteractionPayloadMiss(p *tbhmem.Process, logDataPtr uintptr, className string) {
	candidates := make([]string, 0)
	for offset := uintptr(0x40); offset <= 0xA0; offset += 8 {
		if ptr, ok := p.ReadUintptr(logDataPtr + offset); ok && ptr != 0 && tbhmem.PlausibleAddress(ptr) {
			candidates = append(candidates, fmt.Sprintf("ptr@0x%X=0x%X", offset, ptr))
			if value := readCSharpStringLocal(p, ptr); value != "" {
				candidates = append(candidates, fmt.Sprintf("str@0x%X=%q", offset, value))
			}
			candidates = append(candidates, nestedLogPayloadCandidates(p, ptr, fmt.Sprintf("ptr@0x%X", offset))...)
			continue
		}
		if value, ok := p.ReadInt32(logDataPtr + offset); ok && value > 0 {
			candidates = append(candidates, fmt.Sprintf("i32@0x%X=%d", offset, value))
		}
	}
	if len(candidates) == 0 {
		candidates = append(candidates, "none")
	}
	logPrintf("[NOTIFY] %s did not expose an ItemName_* payload. Candidates: %s\n", className, strings.Join(candidates, ", "))

	lower := strings.ToLower(className)
	if strings.Contains(lower, "synthesis") || strings.Contains(lower, "offering") {
		logPrintf("[NOTIFY] Running deep recursive payload scan for %s...\n", className)
		visited := make(map[uintptr]bool)
		visitCount := 0
		var deepScan func(addr uintptr, path string, depth int)
		deepScan = func(addr uintptr, path string, depth int) {
			if depth > 4 || addr == 0 || visited[addr] || !tbhmem.PlausibleAddress(addr) || visitCount > 1500 {
				return
			}
			visited[addr] = true
			visitCount++

			for offset := uintptr(0); offset <= 0x150; offset += 4 {
				val, ok := p.ReadInt32(addr + offset)
				if ok && val >= 100000 && val <= 999999 {
					activeApp.inventoryMu.Lock()
					_, exists := activeApp.itemMap[int(val)]
					_, existsAll := activeApp.allItemMap[int(val)]
					activeApp.inventoryMu.Unlock()
					if exists {
						logPrintf("[NOTIFY:DEEP] FOUND marketable item ID %d (*) at %s.offset@0x%X\n", val, path, offset)
					} else if existsAll {
						logPrintf("[NOTIFY:DEEP] FOUND non-marketable item ID %d at %s.offset@0x%X\n", val, path, offset)
					}
				}
			}

			for offset := uintptr(0); offset <= 0x150; offset += 8 {
				ptr, ok := p.ReadUintptr(addr + offset)
				if ok && ptr != 0 && tbhmem.PlausibleAddress(ptr) {
					cName := ""
					if cPtr, ok2 := p.ReadUintptr(ptr); ok2 && cPtr != 0 && tbhmem.PlausibleAddress(cPtr) {
						if nPtr, ok3 := p.ReadUintptr(cPtr + 0x10); ok3 && nPtr != 0 && tbhmem.PlausibleAddress(nPtr) {
							cName = readStringLocal(p, nPtr)
						}
					}

					if cName == "String" {
						if strVal := readCSharpStringLocal(p, ptr); strVal != "" {
							if strings.Contains(strVal, "ItemName_") {
								logPrintf("[NOTIFY:DEEP] FOUND localization string key %q (*) at %s.ptr@0x%X\n", strVal, path, offset)
							} else {
								logPrintf("[NOTIFY:DEEP] FOUND string value %q at %s.ptr@0x%X\n", strVal, path, offset)
							}
						}
						continue
					}

					lowerClassName := strings.ToLower(cName)
					shouldSkip := false
					for _, skip := range []string{"hero", "monster", "unit", "dictionary", "comparer", "enumerable", "sorter", "list", "invokable"} {
						if strings.Contains(lowerClassName, skip) {
							if !strings.Contains(lowerClassName, "slot") && !strings.Contains(lowerClassName, "item") {
								shouldSkip = true
								break
							}
						}
					}
					if shouldSkip {
						continue
					}

					pName := fmt.Sprintf("%s.ptr@0x%X", path, offset)
					if cName != "" {
						pName = fmt.Sprintf("%s(%s)", pName, cName)
					}
					deepScan(ptr, pName, depth+1)
				}
			}
		}
		deepScan(logDataPtr, className, 0)
		logPrintf("[NOTIFY] Deep recursive payload scan finished. Visited %d nodes.\n", visitCount)
	}
}

func nestedLogPayloadCandidates(p *tbhmem.Process, objectPtr uintptr, basePath string) []string {
	candidates := make([]string, 0)
	for offset := uintptr(0x10); offset <= 0xC0 && len(candidates) < 16; offset += 8 {
		ptr, ok := p.ReadUintptr(objectPtr + offset)
		if !ok || ptr == 0 || !tbhmem.PlausibleAddress(ptr) {
			continue
		}
		if value := readCSharpStringLocal(p, ptr); value != "" {
			candidates = append(candidates, fmt.Sprintf("%s.str@0x%X=%q", basePath, offset, value))
		}
	}
	for offset := uintptr(0x10); offset <= 0xC0 && len(candidates) < 24; offset += 4 {
		value, ok := p.ReadInt32(objectPtr + offset)
		if !ok || value <= 0 || value > 10000000 {
			continue
		}
		marker := ""
		activeApp.inventoryMu.Lock()
		if _, exists := activeApp.itemMap[int(value)]; exists {
			marker = "*"
		}
		activeApp.inventoryMu.Unlock()
		candidates = append(candidates, fmt.Sprintf("%s.i32@0x%X=%d%s", basePath, offset, value, marker))
	}
	return candidates
}

func recordDirectMarketableItemNotification(itemID int, source string) (marketableInventoryItem, bool) {
	activeApp.inventoryMu.Lock()
	config, exists := activeApp.itemMap[itemID]
	activeApp.inventoryMu.Unlock()
	if !exists {
		logPrintf("[NOTIFY] Ignored %s item ID %d because it is not marketable\n", source, itemID)
		return marketableInventoryItem{}, false
	}

	notifiedBoxItemsMu.Lock()
	tracker := recentlyNotifiedBoxItems[itemID]
	if time.Since(tracker.lastNotified) > directInventoryNotificationWindow {
		tracker.count = 0
	}
	tracker.count++
	tracker.lastNotified = time.Now()
	recentlyNotifiedBoxItems[itemID] = tracker
	for k, v := range recentlyNotifiedBoxItems {
		if time.Since(v.lastNotified) > directInventoryNotificationWindow {
			delete(recentlyNotifiedBoxItems, k)
		}
	}
	notifiedBoxItemsMu.Unlock()

	itemName := config.Name["en-US"]
	if itemName == "" {
		itemName = inventoryNotificationItemName(itemID)
	}
	logPrintf("[NOTIFY] Instant %s item detected: itemID=%d name=%s\n", source, itemID, itemName)
	return marketableInventoryItem{itemID: itemID}, true
}

func appendLogManagerItemNotification(items []marketableInventoryItem, itemID int, source string) []marketableInventoryItem {
	item, ok := recordDirectMarketableItemNotification(itemID, source)
	if !ok {
		return items
	}
	return append(items, item)
}

func resolveLogItemID(p *tbhmem.Process, logDataPtr uintptr, itemID int, className string) int {
	lower := strings.ToLower(className)
	if strings.Contains(lower, "craft") || strings.Contains(lower, "cube") || strings.Contains(lower, "offering") || strings.Contains(lower, "synthesis") || strings.Contains(lower, "box") {
		if grade, ok := p.ReadInt32(logDataPtr + 0x48); ok && grade > 0 && grade <= 10 {
			resolved := resolveGradedItemID(itemID, int(grade))
			logPrintf("[NOTIFY] Resolved base item ID %d with grade %d to %d\n", itemID, grade, resolved)
			return resolved
		}
	}
	return itemID
}

func resolveGradedItemID(baseItemID int, grade int) int {
	if grade <= 0 {
		return baseItemID
	}
	if baseItemID < 300000 || baseItemID > 650000 {
		return baseItemID
	}
	xx := baseItemID / 10000
	middle := (baseItemID / 100) % 100
	if middle != 0 {
		return baseItemID
	}
	y := (baseItemID / 10) % 10
	z := baseItemID % 10
	return xx*10000 + grade*1000 + y*100 + z*10 + 1
}

var (
	pendingLogIndex   = -1
	pendingLogRetries = 0
)

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

		// Seed initial size so we don't notify old LogManager entries.
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
			if notificationSourceEnabled(notificationSourceBox) {
				itemID, _, _, ok := readLogItemPayload(memory, logDataPtr)
				if ok {
					itemID = resolveLogItemID(memory, logDataPtr, itemID, className)
					newItems = appendLogManagerItemNotification(newItems, itemID, notificationSourceBox)
				}
			}
		} else if className == "StageClearLog" || className == "StageFailedLog" {
			logPrintf("[NOTIFY] Stage end event detected (%s). Triggering inventory sync on next tick.\n", className)
			triggerSavePollNow = true
		} else if source, ok := inventoryInteractionResultSource(className); ok {
			if notificationSourceEnabled(source) {
				if source == notificationSourceSynthesis {
					triggerSavePollNow = true
					val, ok := memory.ReadInt32(logDataPtr + 0xB0)
					isReady := ok && val > 0

					if isReady {
						activeApp.inventoryMu.Lock()
						_, isValidMarketable := activeApp.itemMap[int(val)]
						activeApp.inventoryMu.Unlock()

						if isValidMarketable {
							logPrintf("[NOTIFY] Synthesis item ID detected at offset 0xB0: %d\n", val)
							newItems = appendLogManagerItemNotification(newItems, int(val), source)
						} else {
							logPrintf("[NOTIFY] Ignored synthesized item ID %d because it is not marketable\n", val)
						}
						pendingLogIndex = -1
						pendingLogRetries = 0
					} else {
						if pendingLogIndex != i {
							pendingLogIndex = i
							pendingLogRetries = 0
						}
						if pendingLogRetries < 3 {
							pendingLogRetries++
							logPrintf("[NOTIFY] Synthesis item ID not ready at offset 0xB0 yet. Retrying (%d/3) on next poll...\n", pendingLogRetries)
							break
						} else {
							logPrintf("[NOTIFY] Synthesis item ID timeout. Logging payload miss details.\n")
							logInventoryInteractionPayloadMiss(memory, logDataPtr, className)
							pendingLogIndex = -1
							pendingLogRetries = 0
						}
					}
				} else {
					itemID, itemValue, path, ok := readLogItemPayload(memory, logDataPtr)
					if !ok {
						logInventoryInteractionPayloadMiss(memory, logDataPtr, className)
					} else {
						logPrintf("[NOTIFY] %s payload item detected at %s: %s\n", className, path, itemValue)
						itemID = resolveLogItemID(memory, logDataPtr, itemID, className)
						newItems = appendLogManagerItemNotification(newItems, itemID, source)
					}
				}
			}
		}
		lastLogCount = i + 1
	}

	if len(newItems) > 0 {
		for index := range newItems {
			fillMarketableInventoryItemDetails(&newItems[index])
		}
		processNewMarketableInventoryItems(newItems)
	}
}
