package app

import (
	"github.com/nidea1/task-bar-trade-center/internal/catalog"

	"fmt"
	"sort"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/market"
)

func fetchPriceAndUpdate(config catalog.ItemConfig) {
	fetchPriceAndUpdateWithScope(config, true, market.CurrentScope())
}

func refreshPriceAndUpdate(config catalog.ItemConfig) {
	fetchPriceAndUpdateWithScope(config, false, market.CurrentScope())
}

func fetchPriceAndUpdateWithCache(config catalog.ItemConfig, useCache bool) {
	fetchPriceAndUpdateWithScope(config, useCache, market.CurrentScope())
}

func fetchPriceAndUpdateWithScope(config catalog.ItemConfig, useCache bool, scope market.MarketScope) {
	marketHashName := buildMarketHashName(config)
	cacheKey := market.CacheKey(scope, marketHashName)
	now := time.Now()

	existingCache, hasExistingCache := marketCacheEntry(scope, marketHashName)
	if useCache && hasExistingCache && market.IsFreshCache(existingCache, now) && !market.RequiresUSDFallbackRefresh(scope, existingCache.Analysis) {
		logMarketPrice(config, scope, marketHashName, existingCache.Analysis, "cache")
		updatePriceOverlay(config.ID, scope, existingCache.Analysis)
		return
	}

	data, err := fetchMarketData(config, marketHashName, now, scope)
	if err != nil {
		if analysis, ok := market.StaleAnalysis(existingCache, hasExistingCache); ok {
			logMarketPrice(config, scope, marketHashName, analysis, "stale-cache")
			fmt.Printf("[MARKET:error] Steam market analysis failed, using stale cache: %v\n", err)
			updatePriceOverlay(config.ID, scope, analysis)
			return
		}

		analysis := market.UnavailableAnalysis(marketHashName, now, scope.Currency)
		logMarketPrice(config, scope, marketHashName, analysis, "error")
		fmt.Printf("[MARKET:error] Steam market analysis failed: %v\n", err)
		updatePriceOverlay(config.ID, scope, analysis)
		return
	}
	data = retainCachedIconURL(data, existingCache, hasExistingCache)

	activeApp.priceCacheMu.Lock()
	activeApp.priceCache[cacheKey] = data
	writePriceCacheFileLocked()
	activeApp.priceCacheMu.Unlock()

	source := "steam"
	if !useCache {
		source = "refresh"
	}
	logMarketPrice(config, scope, marketHashName, data.Analysis, source)
	updatePriceOverlay(config.ID, scope, data.Analysis)
}

func fetchMarketData(config catalog.ItemConfig, marketHashName string, now time.Time, scope market.MarketScope) (market.MarketData, error) {
	return market.FetchData(config, marketHashName, now, scope)
}

func marketCacheEntry(scope market.MarketScope, marketHashName string) (market.MarketData, bool) {
	activeApp.priceCacheMu.RLock()
	defer activeApp.priceCacheMu.RUnlock()
	data, exists := activeApp.priceCache[market.CacheKey(scope, marketHashName)]
	return data, exists
}

func retainCachedIconURL(data market.MarketData, existing market.MarketData, exists bool) market.MarketData {
	if data.Analysis.IconURL == "" && exists && existing.Analysis.IconURL != "" {
		data.Analysis.IconURL = existing.Analysis.IconURL
	}
	return data
}

func buildMarketHashName(config catalog.ItemConfig) string {
	return market.BuildHashName(config)
}

func steamMarketListingURL(config catalog.ItemConfig) string {
	return market.ListingURL(config)
}

func steamMarketListingURLForScope(config catalog.ItemConfig, scope market.MarketScope) string {
	return market.ListingURLForScope(config, scope)
}

func priceHistoryURL(marketHashName string, scope market.MarketScope) string {
	return market.PriceHistoryURL(marketHashName, scope)
}

func itemOrdersHistogramURL(itemNameID string, scope market.MarketScope) string {
	return market.ItemOrdersHistogramURL(itemNameID, scope)
}

func priceOverviewURL(marketHashName string, scope market.MarketScope) string {
	return market.PriceOverviewURL(marketHashName, scope)
}

func logMarketPrice(config catalog.ItemConfig, scope market.MarketScope, marketHashName string, analysis market.MarketAnalysis, source string) {
	fmt.Printf("[MARKET:%s] [%s] %s (ID: %d, grade: %s, type: %s) | %s => suggested=%s\n", source, market.FormatScope(scope), config.Name["en-US"], config.ID, config.Grade, config.Type, marketHashName, market.FormatAnalysisPrice(analysis.SuggestedPrice, analysis.HasSuggested, analysis))
}

func updatePriceOverlay(itemID int, scope market.MarketScope, analysis market.MarketAnalysis) {
	if activeApp.activeItemID.Load() != int32(itemID) || market.CurrentScope() != scope {
		return
	}
	setCurrentMarketAnalysis(analysis)
	redrawOverlay()
}

func refreshActiveMarketPrice() {
	if !activeApp.showOverlay.Load() {
		return
	}

	itemID := int(activeApp.activeItemID.Load())
	config, exists := activeApp.itemMap[itemID]
	if !exists {
		return
	}

	setCurrentPriceText(tr("hud.loading"))
	redrawOverlay()
	go fetchPriceAndUpdate(config)
}

func clearPriceCache() int {
	activeApp.priceCacheMu.Lock()
	defer activeApp.priceCacheMu.Unlock()

	count := len(activeApp.priceCache)
	for key := range activeApp.priceCache {
		delete(activeApp.priceCache, key)
	}
	writePriceCacheFileLocked()
	return count
}

func priceCacheSize() int {
	activeApp.priceCacheMu.RLock()
	defer activeApp.priceCacheMu.RUnlock()
	return len(activeApp.priceCache)
}

func refreshCachedPricesInBackground() int {
	if !activeApp.priceCacheRefreshing.CompareAndSwap(false, true) {
		return -1
	}
	requestTrayTooltipUpdate()

	scope := market.CurrentScope()
	configs := cachedPriceConfigs(scope)
	if len(configs) == 0 {
		activeApp.priceCacheRefreshing.Store(false)
		requestTrayTooltipUpdate()
		return 0
	}

	go func() {
		defer func() {
			activeApp.priceCacheRefreshing.Store(false)
			requestTrayTooltipUpdate()
		}()

		fmt.Printf("Refreshing cached prices: %d item(s).\n", len(configs))
		for index, config := range configs {
			fmt.Printf("Refreshing cached price %d/%d: %s\n", index+1, len(configs), config.Name["en-US"])
			fetchPriceAndUpdateWithScope(config, false, scope)
		}
		fmt.Printf("Cached price refresh completed: %d item(s).\n", len(configs))
	}()

	return len(configs)
}

func cachedPriceConfigs(scope market.MarketScope) []catalog.ItemConfig {
	activeApp.priceCacheMu.RLock()
	cachedNames := make(map[string]struct{}, len(activeApp.priceCache))
	for cacheKey := range activeApp.priceCache {
		cachedScope, marketHashName, ok := market.ParseCacheKey(cacheKey)
		if ok && cachedScope == scope {
			cachedNames[marketHashName] = struct{}{}
		}
	}
	activeApp.priceCacheMu.RUnlock()

	configs := make([]catalog.ItemConfig, 0, len(cachedNames))
	for _, config := range activeApp.itemMap {
		if _, exists := cachedNames[buildMarketHashName(config)]; exists {
			configs = append(configs, config)
		}
	}

	sort.Slice(configs, func(i int, j int) bool {
		return configs[i].ID < configs[j].ID
	})
	return configs
}
