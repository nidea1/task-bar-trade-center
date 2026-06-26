package app

import (
	"fmt"
	"sort"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/market"
)

func fetchPriceAndUpdate(config ItemConfig) {
	fetchPriceAndUpdateWithScope(config, true, currentMarketScope())
}

func refreshPriceAndUpdate(config ItemConfig) {
	fetchPriceAndUpdateWithScope(config, false, currentMarketScope())
}

func fetchPriceAndUpdateWithCache(config ItemConfig, useCache bool) {
	fetchPriceAndUpdateWithScope(config, useCache, currentMarketScope())
}

func fetchPriceAndUpdateWithScope(config ItemConfig, useCache bool, scope MarketScope) {
	marketHashName := buildMarketHashName(config)
	cacheKey := marketCacheKey(scope, marketHashName)
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

	PriceCacheMu.Lock()
	PriceCache[cacheKey] = data
	writePriceCacheFileLocked()
	PriceCacheMu.Unlock()

	source := "steam"
	if !useCache {
		source = "refresh"
	}
	logMarketPrice(config, scope, marketHashName, data.Analysis, source)
	updatePriceOverlay(config.ID, scope, data.Analysis)
}

func fetchMarketData(config ItemConfig, marketHashName string, now time.Time, scope MarketScope) (MarketData, error) {
	return market.FetchData(config, marketHashName, now, scope)
}

func marketCacheEntry(scope MarketScope, marketHashName string) (MarketData, bool) {
	PriceCacheMu.RLock()
	defer PriceCacheMu.RUnlock()
	data, exists := PriceCache[marketCacheKey(scope, marketHashName)]
	return data, exists
}

func buildMarketHashName(config ItemConfig) string {
	return market.BuildHashName(config)
}

func steamMarketListingURL(config ItemConfig) string {
	return market.ListingURL(config)
}

func steamMarketListingURLForScope(config ItemConfig, scope MarketScope) string {
	return market.ListingURLForScope(config, scope)
}

func priceHistoryURL(marketHashName string, scope MarketScope) string {
	return market.PriceHistoryURL(marketHashName, scope)
}

func itemOrdersHistogramURL(itemNameID string, scope MarketScope) string {
	return market.ItemOrdersHistogramURL(itemNameID, scope)
}

func priceOverviewURL(marketHashName string, scope MarketScope) string {
	return market.PriceOverviewURL(marketHashName, scope)
}

func logMarketPrice(config ItemConfig, scope MarketScope, marketHashName string, analysis MarketAnalysis, source string) {
	fmt.Printf("[MARKET:%s] [%s] %s (ID: %d, grade: %s, type: %s) | %s => suggested=%s\n", source, formatMarketScope(scope), config.Name["en-US"], config.ID, config.Grade, config.Type, marketHashName, formatAnalysisPrice(analysis.SuggestedPrice, analysis.HasSuggested, analysis))
}

func updatePriceOverlay(itemID int, scope MarketScope, analysis MarketAnalysis) {
	if ActiveItemID.Load() != int32(itemID) || currentMarketScope() != scope {
		return
	}
	setCurrentMarketAnalysis(analysis)
	redrawOverlay()
}

func refreshActiveMarketPrice() {
	if !ShowOverlay.Load() {
		return
	}

	itemID := int(ActiveItemID.Load())
	config, exists := ItemMap[itemID]
	if !exists {
		return
	}

	setCurrentPriceText(tr("hud.loading"))
	redrawOverlay()
	go fetchPriceAndUpdate(config)
}

func clearPriceCache() int {
	PriceCacheMu.Lock()
	defer PriceCacheMu.Unlock()

	count := len(PriceCache)
	for key := range PriceCache {
		delete(PriceCache, key)
	}
	writePriceCacheFileLocked()
	return count
}

func priceCacheSize() int {
	PriceCacheMu.RLock()
	defer PriceCacheMu.RUnlock()
	return len(PriceCache)
}

func refreshCachedPricesInBackground() int {
	if !PriceCacheRefreshing.CompareAndSwap(false, true) {
		return -1
	}
	requestTrayTooltipUpdate()

	scope := currentMarketScope()
	configs := cachedPriceConfigs(scope)
	if len(configs) == 0 {
		PriceCacheRefreshing.Store(false)
		requestTrayTooltipUpdate()
		return 0
	}

	go func() {
		defer func() {
			PriceCacheRefreshing.Store(false)
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

func cachedPriceConfigs(scope MarketScope) []ItemConfig {
	PriceCacheMu.RLock()
	cachedNames := make(map[string]struct{}, len(PriceCache))
	for cacheKey := range PriceCache {
		cachedScope, marketHashName, ok := parseMarketCacheKey(cacheKey)
		if ok && cachedScope == scope {
			cachedNames[marketHashName] = struct{}{}
		}
	}
	PriceCacheMu.RUnlock()

	configs := make([]ItemConfig, 0, len(cachedNames))
	for _, config := range ItemMap {
		if _, exists := cachedNames[buildMarketHashName(config)]; exists {
			configs = append(configs, config)
		}
	}

	sort.Slice(configs, func(i int, j int) bool {
		return configs[i].ID < configs[j].ID
	})
	return configs
}
