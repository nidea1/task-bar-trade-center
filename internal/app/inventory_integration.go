package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/inventory"
	"github.com/nidea1/task-bar-trade-center/internal/playerdata"
	"github.com/nidea1/task-bar-trade-center/internal/tbhmem"
)

var (
	inventoryMu             sync.Mutex
	inventoryResolver       *playerdata.Resolver
	inventoryDashboardState inventory.DashboardState
	inventoryPriceQueue     *inventory.RefreshQueue
)

type cacheQuoteProvider struct {
	scope MarketScope
}

func refreshInventoryDashboardState(reason string) {
	state, err := readInventoryDashboardState()
	if err != nil {
		fmt.Printf("[INVENTORY] dashboard refresh failed (%s): %v\n", reason, err)
		return
	}
	inventoryMu.Lock()
	inventoryDashboardState = state
	inventoryMu.Unlock()
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
	inventoryMu.Lock()
	inventoryDashboardState = state
	inventoryMu.Unlock()
}

func currentInventoryDashboardState() inventory.DashboardState {
	inventoryMu.Lock()
	defer inventoryMu.Unlock()
	return inventoryDashboardState
}

func readInventoryDashboardState() (inventory.DashboardState, error) {
	if GameProcessHandle == 0 || GameProcessID == 0 {
		return inventory.DashboardState{}, fmt.Errorf("game process is not attached")
	}
	memory := tbhmem.FromHandle(GameProcessID, GameProcessHandle)
	if memory == nil {
		return inventory.DashboardState{}, fmt.Errorf("game process handle is unavailable")
	}

	resolver := currentInventoryResolver()
	snapshot, ok := resolver.ReadSnapshot(memory, time.Now())
	if !ok {
		return inventory.DashboardState{}, fmt.Errorf("PlayerSaveData could not be resolved")
	}

	scope := currentMarketScope()
	return inventory.BuildDashboard(snapshot, inventoryItemCatalog(scope), cacheQuoteProvider{scope: scope}, inventory.DashboardOptions{
		MarketScope:  formatMarketScope(scope),
		CurrencyCode: scope.Currency.Code,
		PricePrefix:  scope.Currency.PricePrefix,
		PriceSuffix:  scope.Currency.PriceSuffix,
		Refresh:      currentInventoryRefreshStatus(),
		Now:          time.Now(),
	}), nil
}

func currentInventoryResolver() *playerdata.Resolver {
	inventoryMu.Lock()
	defer inventoryMu.Unlock()
	if inventoryResolver == nil {
		inventoryResolver = playerdata.NewResolver(playerItemMetadata())
	}
	return inventoryResolver
}

func playerItemMetadata() map[int]playerdata.ItemMetadata {
	metadata := make(map[int]playerdata.ItemMetadata, len(AllItemMap))
	for id := range AllItemMap {
		_, marketable := ItemMap[id]
		metadata[id] = playerdata.ItemMetadata{Marketable: marketable}
	}
	return metadata
}

func inventoryItemCatalog(scope MarketScope) map[int]inventory.ItemDescriptor {
	catalog := make(map[int]inventory.ItemDescriptor, len(AllItemMap))
	for id, config := range AllItemMap {
		name := config.Name[currentDisplayLanguage()]
		if name == "" {
			name = config.Name["en-US"]
		}
		marketableConfig, marketable := ItemMap[id]
		descriptor := inventory.ItemDescriptor{Name: name, Marketable: marketable}
		if marketable {
			descriptor.MarketHashName = buildMarketHashName(marketableConfig)
			descriptor.MarketURL = steamMarketListingURLForScope(marketableConfig, scope)
		}
		catalog[id] = descriptor
	}
	return catalog
}

func (provider cacheQuoteProvider) Quote(itemID int) (inventory.PriceQuote, bool) {
	config, exists := ItemMap[itemID]
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
		UpdatedAt:    analysis.UpdatedAt,
	}
	return quote, quote.HasSuggested || quote.HasInstant
}

func writeInventoryDashboardState(state inventory.DashboardState) error {
	if InventoryStateFilePath == "" {
		return nil
	}
	return writeJSONAtomic(InventoryStateFilePath, state)
}

func writeJSONAtomic(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
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
		if !item.HasPrice || item.UpdatedAt.IsZero() || now.Sub(item.UpdatedAt) > maxAge {
			ids = append(ids, item.ItemID)
		}
	}
	return ids
}

func currentInventoryPriceQueue() *inventory.RefreshQueue {
	inventoryMu.Lock()
	defer inventoryMu.Unlock()
	if inventoryPriceQueue == nil {
		inventoryPriceQueue = inventory.NewRefreshQueue(fetchInventoryMarketPrice, isSteamRateLimitError)
	}
	return inventoryPriceQueue
}

func currentInventoryRefreshStatus() inventory.RefreshStatus {
	queue := currentInventoryPriceQueue()
	if queue == nil {
		return inventory.RefreshStatus{}
	}
	return queue.Status()
}

func fetchInventoryMarketPrice(_ context.Context, itemID int) error {
	config, exists := ItemMap[itemID]
	if !exists {
		return nil
	}
	scope := currentMarketScope()
	marketHashName := buildMarketHashName(config)
	data, err := fetchMarketData(config, marketHashName, time.Now(), scope)
	if err != nil {
		return err
	}
	PriceCacheMu.Lock()
	PriceCache[marketCacheKey(scope, marketHashName)] = data
	writePriceCacheFileLocked()
	PriceCacheMu.Unlock()
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
