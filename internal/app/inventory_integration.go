package app

import (
	"github.com/nidea1/task-bar-trade-center/internal/market"

	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/inventory"
	"github.com/nidea1/task-bar-trade-center/internal/playerdata"
	filestore "github.com/nidea1/task-bar-trade-center/internal/storage"
	"github.com/nidea1/task-bar-trade-center/internal/tbhmem"
)

type cacheQuoteProvider struct {
	scope market.MarketScope
}

func refreshInventoryDashboardState(reason string) {
	state, err := readInventoryDashboardState()
	if err != nil {
		fmt.Printf("[INVENTORY] dashboard refresh failed (%s): %v\n", reason, err)
		return
	}
	activeApp.inventoryMu.Lock()
	activeApp.inventoryDashboardState = state
	activeApp.inventoryMu.Unlock()
	if err := writeInventoryDashboardState(state); err != nil {
		fmt.Printf("[INVENTORY] dashboard state write failed: %v\n", err)
		return
	}
	callDashboardUpdated(state)
	fmt.Printf("[INVENTORY] dashboard state refreshed (%s): items=%d marketable=%d priced=%d unknown=%d\n",
		reason,
		state.Totals.TotalItemCount,
		state.Totals.MarketableItemCount,
		state.Totals.PricedItemCount,
		state.Totals.UnknownItemCount,
	)
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

func readInventoryDashboardState() (inventory.DashboardState, error) {
	if activeApp.gameProcessHandle == 0 || activeApp.gameProcessID == 0 {
		return inventory.DashboardState{}, fmt.Errorf("game process is not attached")
	}
	memory := tbhmem.FromHandle(activeApp.gameProcessID, activeApp.gameProcessHandle)
	if memory == nil {
		return inventory.DashboardState{}, fmt.Errorf("game process handle is unavailable")
	}

	resolver := currentInventoryResolver()
	snapshot, ok := resolver.ReadSnapshot(memory, time.Now())
	if !ok {
		return inventory.DashboardState{}, fmt.Errorf("PlayerSaveData could not be resolved")
	}

	activeApp.inventoryMu.Lock()
	activeApp.lastSnapshot = &snapshot
	activeApp.inventoryMu.Unlock()

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
			if cacheData, exists := marketCacheEntry(scope, hashName); exists && cacheData.Analysis.IconURL != "" {
				descriptor.IconURL = "https://community.cloudflare.steamstatic.com/economy/image/" + cacheData.Analysis.IconURL
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
		Suggested:    analysis.SuggestedPrice,
		Instant:      analysis.HighestBuyPrice,
		HasSuggested: analysis.HasSuggested,
		HasInstant:   analysis.HasHighestBuy,
		PricePrefix:  analysis.PricePrefix,
		PriceSuffix:  analysis.PriceSuffix,
		UpdatedAt:    analysis.UpdatedAt.Format(time.RFC3339),
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

func refreshInventoryPricesFromDashboard() {
	state, err := readInventoryDashboardState()
	if err != nil {
		fmt.Printf("[INVENTORY] cannot queue price refresh: %v\n", err)
		return
	}
	added := queueInventoryPriceRefresh(state)
	fmt.Printf("[INVENTORY] queued inventory price refresh: added=%d\n", added)
	refreshInventoryDashboardState("price-refresh-queued")
}

func queueInventoryPriceRefresh(state inventory.DashboardState) int {
	ids := missingOrStaleDashboardItemIDs(state, 24*time.Hour)
	if len(ids) == 0 {
		fmt.Println("[INVENTORY] no inventory prices need refresh.")
		return 0
	}
	added := currentInventoryPriceQueue().Enqueue(ids)
	fmt.Printf("[INVENTORY] queued inventory price refresh: added=%d total_candidates=%d\n", added, len(ids))
	return added
}

func missingOrStaleDashboardItemIDs(state inventory.DashboardState, maxAge time.Duration) []int {
	now := time.Now()
	ids := make([]int, 0)
	for _, item := range state.Items {
		if !item.HasPrice || item.UpdatedAt == "" {
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
	data, err := fetchMarketData(config, marketHashName, time.Now(), scope)
	if err != nil {
		return err
	}
	activeApp.priceCacheMu.Lock()
	activeApp.priceCache[market.CacheKey(scope, marketHashName)] = data
	writePriceCacheFileLocked()
	activeApp.priceCacheMu.Unlock()
	refreshInventoryDashboardState("price-refreshed")
	return nil
}

func isSteamRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "429") || strings.Contains(message, "too many")
}

func rebuildDashboardState(reason string) {
	activeApp.inventoryMu.Lock()
	snapshot := activeApp.lastSnapshot
	activeApp.inventoryMu.Unlock()

	if snapshot == nil {
		// If we don't have a cached snapshot in memory, and the game is running, we can read memory.
		if activeApp.gameProcessHandle != 0 && activeApp.gameProcessID != 0 {
			go refreshInventoryDashboardState(reason)
		} else {
			// If not running, let's update the translations and scope on the cached dashboard state
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
		}
		return
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
	fmt.Printf("[INVENTORY] dashboard state rebuilt (%s): items=%d marketable=%d priced=%d\n",
		reason, state.Totals.TotalItemCount, state.Totals.MarketableItemCount, state.Totals.PricedItemCount)
}
