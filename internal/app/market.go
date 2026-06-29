package app

import (
	"github.com/nidea1/task-bar-trade-center/internal/catalog"

	"sort"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/market"
)

type marketFetchCall struct {
	done chan struct{}
	data market.MarketData
	err  error
}

type cachedPriceOverlayState int

const (
	cachedPriceMissing cachedPriceOverlayState = iota
	cachedPriceFresh
	cachedPriceNeedsRefresh
)

var fetchMarketDataFromSteam = market.FetchDataWithPriority

func fetchPriceAndUpdate(config catalog.ItemConfig) {
	fetchPriceAndUpdateWithOptions(config, true, market.CurrentScope(), market.RequestPriorityHigh, true)
}

func refreshPriceAndUpdate(config catalog.ItemConfig) {
	fetchPriceAndUpdateWithOptions(config, false, market.CurrentScope(), market.RequestPriorityHigh, true)
}

func fetchPriceAndUpdateWithCache(config catalog.ItemConfig, useCache bool) {
	fetchPriceAndUpdateWithOptions(config, useCache, market.CurrentScope(), market.RequestPriorityHigh, true)
}

func fetchPriceAndUpdateWithScope(config catalog.ItemConfig, useCache bool, scope market.MarketScope) {
	fetchPriceAndUpdateWithOptions(config, useCache, scope, market.RequestPriorityNormal, true)
}

func fetchPriceAndUpdateWithOptions(config catalog.ItemConfig, useCache bool, scope market.MarketScope, priority market.RequestPriority, showExistingCache bool) {
	marketHashName := buildMarketHashName(config)
	cacheKey := market.CacheKey(scope, marketHashName)
	now := time.Now()

	existingCache, hasExistingCache := marketCacheEntry(scope, marketHashName)
	if useCache && hasExistingCache {
		needsRefresh := !market.IsFreshCache(existingCache, now) || market.RequiresUSDFallbackRefresh(scope, existingCache.Analysis)
		if showExistingCache {
			source := "cache"
			cacheState := "fresh"
			if needsRefresh {
				source = "stale-cache"
				cacheState = "stale"
			}
			logTooltipCacheMetric(config, scope, marketHashName, cacheState, needsRefresh, cachedAnalysisAge(existingCache, now))
			if analysis, ok := market.StaleAnalysis(existingCache, true); ok {
				logMarketPrice(config, scope, marketHashName, analysis, source)
				updatePriceOverlay(config.ID, scope, analysis)
			}
		}
		if !needsRefresh {
			return
		}
	} else if useCache && showExistingCache {
		logTooltipCacheMetric(config, scope, marketHashName, "miss", true, -1)
	}

	data, err := fetchMarketDataWithPriority(config, marketHashName, now, scope, priority)
	if err != nil {
		if analysis, ok := market.StaleAnalysis(existingCache, hasExistingCache); ok {
			logMarketPrice(config, scope, marketHashName, analysis, "stale-cache")
			logPrintf("[MARKET:error] Steam market analysis failed, using stale cache: %v\n", err)
			updatePriceOverlay(config.ID, scope, analysis)
			return
		}

		analysis := market.UnavailableAnalysis(marketHashName, now, scope.Currency)
		logMarketPrice(config, scope, marketHashName, analysis, "error")
		logPrintf("[MARKET:error] Steam market analysis failed: %v\n", err)
		updatePriceOverlay(config.ID, scope, analysis)
		return
	}
	fetchedIconURL := data.Analysis.IconURL
	if fetchedIconURL != "" {
		recordMarketIcon(marketHashName, fetchedIconURL, now)
	}
	data = retainCachedIconURL(data, existingCache, hasExistingCache)
	data = retainIconMetadataURL(data, marketHashName)

	activeApp.priceCacheMu.Lock()
	activeApp.priceCache[cacheKey] = data
	schedulePriceCacheWriteLocked()
	activeApp.priceCacheMu.Unlock()

	source := "steam"
	if !useCache {
		source = "refresh"
	}
	logMarketPrice(config, scope, marketHashName, data.Analysis, source)
	updatePriceOverlay(config.ID, scope, data.Analysis)
}

func fetchMarketData(config catalog.ItemConfig, marketHashName string, now time.Time, scope market.MarketScope) (market.MarketData, error) {
	return fetchMarketDataWithPriority(config, marketHashName, now, scope, market.RequestPriorityNormal)
}

func fetchMarketDataWithPriority(config catalog.ItemConfig, marketHashName string, now time.Time, scope market.MarketScope, priority market.RequestPriority) (market.MarketData, error) {
	cacheKey := market.CacheKey(scope, marketHashName)

	activeApp.marketFetchMu.Lock()
	if activeApp.marketFetchInFlight == nil {
		activeApp.marketFetchInFlight = make(map[string]*marketFetchCall)
	}
	if call := activeApp.marketFetchInFlight[cacheKey]; call != nil {
		activeApp.marketFetchMu.Unlock()
		waitStartedAt := time.Now()
		<-call.done
		logMarketFetchMetric(config, scope, marketHashName, priority, true, time.Since(waitStartedAt), call.err)
		return call.data, call.err
	}

	call := &marketFetchCall{done: make(chan struct{})}
	activeApp.marketFetchInFlight[cacheKey] = call
	activeApp.marketFetchMu.Unlock()

	fetchStartedAt := time.Now()
	call.data, call.err = fetchMarketDataFromSteam(config, marketHashName, now, scope, priority)
	logMarketFetchMetric(config, scope, marketHashName, priority, false, time.Since(fetchStartedAt), call.err)

	activeApp.marketFetchMu.Lock()
	delete(activeApp.marketFetchInFlight, cacheKey)
	close(call.done)
	activeApp.marketFetchMu.Unlock()

	return call.data, call.err
}

func marketCacheEntry(scope market.MarketScope, marketHashName string) (market.MarketData, bool) {
	activeApp.priceCacheMu.RLock()
	defer activeApp.priceCacheMu.RUnlock()
	data, exists := activeApp.priceCache[market.CacheKey(scope, marketHashName)]
	return data, exists
}

func showCachedPriceOverlay(config catalog.ItemConfig, scope market.MarketScope) cachedPriceOverlayState {
	marketHashName := buildMarketHashName(config)
	existingCache, hasExistingCache := marketCacheEntry(scope, marketHashName)
	analysis, ok := market.StaleAnalysis(existingCache, hasExistingCache)
	if !ok {
		logTooltipCacheMetric(config, scope, marketHashName, "miss", true, -1)
		return cachedPriceMissing
	}

	now := time.Now()
	needsRefresh := !market.IsFreshCache(existingCache, now) || market.RequiresUSDFallbackRefresh(scope, existingCache.Analysis)
	source := "cache"
	cacheState := "fresh"
	if needsRefresh {
		source = "stale-cache"
		cacheState = "stale"
	}
	logTooltipCacheMetric(config, scope, marketHashName, cacheState, needsRefresh, cachedAnalysisAge(existingCache, now))
	logMarketPrice(config, scope, marketHashName, analysis, source)
	updatePriceOverlay(config.ID, scope, analysis)
	if needsRefresh {
		return cachedPriceNeedsRefresh
	}
	return cachedPriceFresh
}

func cachedAnalysisAge(data market.MarketData, now time.Time) time.Duration {
	if data.Analysis.UpdatedAt.IsZero() {
		return -1
	}
	return now.Sub(data.Analysis.UpdatedAt)
}

func retainCachedIconURL(data market.MarketData, existing market.MarketData, exists bool) market.MarketData {
	if data.Analysis.IconURL == "" && exists && existing.Analysis.IconURL != "" {
		data.Analysis.IconURL = existing.Analysis.IconURL
	}
	return data
}

func retainIconMetadataURL(data market.MarketData, marketHashName string) market.MarketData {
	if data.Analysis.IconURL != "" {
		return data
	}
	if iconPath, ok := marketIconPath(marketHashName); ok {
		data.Analysis.IconURL = iconPath
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
	logPrintf("[MARKET:%s] [%s] %s (ID: %d, grade: %s, type: %s) | %s => suggested=%s\n", source, market.FormatScope(scope), config.Name["en-US"], config.ID, config.Grade, config.Type, marketHashName, market.FormatAnalysisPrice(analysis.SuggestedPrice, analysis.HasSuggested, analysis))
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
			flushCacheWritesNow()
			activeApp.priceCacheRefreshing.Store(false)
			requestTrayTooltipUpdate()
		}()

		logPrintf("Refreshing cached prices: %d item(s).\n", len(configs))
		for index, config := range configs {
			logPrintf("Refreshing cached price %d/%d: %s\n", index+1, len(configs), config.Name["en-US"])
			fetchPriceAndUpdateWithScope(config, false, scope)
		}
		logPrintf("Cached price refresh completed: %d item(s).\n", len(configs))
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
