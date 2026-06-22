package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	steamMarketAppID    = 3678970
	marketOrderCacheTTL = 30 * time.Minute
	marketHistoryTTL    = 6 * time.Hour
	steamRequestTimeout = 6 * time.Second
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
	if useCache && hasExistingCache && isFreshMarketCache(existingCache, now) {
		logMarketPrice(config, scope, marketHashName, existingCache.OverlayText, "cache")
		updatePriceOverlay(config.ID, scope, existingCache.OverlayText)
		return
	}

	data, err := fetchMarketData(config, marketHashName, now, scope)
	if err != nil {
		if overlayText, ok := staleMarketOverlayText(existingCache, hasExistingCache); ok {
			logMarketPrice(config, scope, marketHashName, overlayText, "stale-cache")
			fmt.Printf("[MARKET:error] Steam market analysis failed, using stale cache: %v\n", err)
			updatePriceOverlay(config.ID, scope, overlayText)
			return
		}

		overlayText := buildMarketOverlayText(unavailableMarketAnalysis(marketHashName, now))
		logMarketPrice(config, scope, marketHashName, overlayText, "error")
		fmt.Printf("[MARKET:error] Steam market analysis failed: %v\n", err)
		updatePriceOverlay(config.ID, scope, overlayText)
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
	logMarketPrice(config, scope, marketHashName, data.OverlayText, source)
	updatePriceOverlay(config.ID, scope, data.OverlayText)
}

func fetchMarketData(config ItemConfig, marketHashName string, now time.Time, scope MarketScope) (MarketData, error) {
	client := &http.Client{Timeout: steamRequestTimeout}
	referer := steamMarketListingURL(config)
	var requestErrors []string

	var orderBook MarketOrderBook
	hasOrderBook := false
	var history []MarketSalePoint

	listingBody, _, err := steamGet(client, referer, "")
	if err != nil {
		requestErrors = append(requestErrors, "listing: "+err.Error())
	} else {
		if isUSMarketScope(scope) {
			orderBook, hasOrderBook = parseSSRItemOrderBook(listingBody)
			history = parseSSRPriceHistory(listingBody)
			if len(history) == 0 {
				history = parseLegacySaleHistoryFromListing(listingBody)
			}
		}

		if !hasOrderBook {
			itemNameID := parseItemNameID(listingBody)
			if itemNameID != "" {
				body, _, err := fetchItemOrdersHistogram(client, itemNameID, referer, scope)
				if err != nil {
					requestErrors = append(requestErrors, "histogram: "+err.Error())
				} else {
					orderBook, hasOrderBook = parseItemOrdersHistogramResponse(body)
					if !hasOrderBook {
						requestErrors = append(requestErrors, "histogram: response did not contain order data")
					}
				}
			}
		}
	}

	if isUSMarketScope(scope) && len(history) == 0 {
		body, _, err := fetchSaleHistory(client, marketHashName, referer, scope)
		if err != nil {
			requestErrors = append(requestErrors, "pricehistory: "+err.Error())
		} else {
			history = parseSaleHistoryResponse(body)
			if len(history) == 0 {
				requestErrors = append(requestErrors, "pricehistory: response did not contain sale data")
			}
		}
	}

	if hasOrderBook || len(history) > 0 {
		return marketDataFromSources(marketHashName, orderBook, hasOrderBook, history, now), nil
	}

	body, _, err := fetchPriceOverview(client, marketHashName, scope)
	if err != nil {
		requestErrors = append(requestErrors, "priceoverview: "+err.Error())
	} else if data, ok := marketDataFromPriceOverview(marketHashName, body, now); ok {
		return data, nil
	} else {
		requestErrors = append(requestErrors, "priceoverview: response did not contain price data")
	}

	if len(requestErrors) == 0 {
		return MarketData{}, fmt.Errorf("no market data available")
	}
	return MarketData{}, fmt.Errorf("%s", strings.Join(requestErrors, "; "))
}

func marketDataFromSources(marketHashName string, orderBook MarketOrderBook, hasOrderBook bool, history []MarketSalePoint, now time.Time) MarketData {
	analysis := buildMarketAnalysis(marketHashName, orderBook, hasOrderBook, history, now)
	data := MarketData{
		OverlayText: buildMarketOverlayText(analysis),
		CachedAt:    now,
		Analysis:    analysis,
	}
	if hasOrderBook {
		data.OrderBook = orderBook
		data.OrderCachedAt = now
	}
	if len(history) > 0 {
		data.History = history
		data.HistoryCachedAt = now
	}
	return data
}

func fetchItemOrdersHistogram(client *http.Client, itemNameID string, referer string, scope MarketScope) ([]byte, int, error) {
	return steamGet(client, itemOrdersHistogramURL(itemNameID, scope), referer)
}

func fetchSaleHistory(client *http.Client, marketHashName string, referer string, scope MarketScope) ([]byte, int, error) {
	return steamGet(client, priceHistoryURL(marketHashName, scope), referer)
}

func priceHistoryURL(marketHashName string, scope MarketScope) string {
	apiURL := fmt.Sprintf(
		"https://steamcommunity.com/market/pricehistory/?country=%s&currency=%d&appid=%d&market_hash_name=%s",
		url.QueryEscape(scope.Region.CountryCode),
		scope.Currency.SteamCurrencyID,
		steamMarketAppID,
		url.QueryEscape(marketHashName),
	)
	return apiURL
}

func fetchPriceOverview(client *http.Client, marketHashName string, scope MarketScope) ([]byte, int, error) {
	return steamGet(client, priceOverviewURL(marketHashName, scope), "")
}

func itemOrdersHistogramURL(itemNameID string, scope MarketScope) string {
	return fmt.Sprintf(
		"https://steamcommunity.com/market/itemordershistogram?country=%s&language=english&currency=%d&item_nameid=%s&two_factor=0",
		url.QueryEscape(scope.Region.CountryCode),
		scope.Currency.SteamCurrencyID,
		url.QueryEscape(itemNameID),
	)
}

func priceOverviewURL(marketHashName string, scope MarketScope) string {
	return fmt.Sprintf(
		"https://steamcommunity.com/market/priceoverview/?country=%s&currency=%d&appid=%d&market_hash_name=%s",
		url.QueryEscape(scope.Region.CountryCode),
		scope.Currency.SteamCurrencyID,
		steamMarketAppID,
		url.QueryEscape(marketHashName),
	)
}

func steamGet(client *http.Client, targetURL string, referer string) ([]byte, int, error) {
	req, err := http.NewRequest(http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) TaskBarTradeCenter/0.1")
	req.Header.Set("Accept", "application/json,text/html;q=0.9,*/*;q=0.8")
	req.Header.Set("Origin", "https://steamcommunity.com")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return body, resp.StatusCode, readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return body, resp.StatusCode, fmt.Errorf("status %s", resp.Status)
	}
	return body, resp.StatusCode, nil
}

func marketCacheEntry(scope MarketScope, marketHashName string) (MarketData, bool) {
	PriceCacheMu.RLock()
	defer PriceCacheMu.RUnlock()
	data, exists := PriceCache[marketCacheKey(scope, marketHashName)]
	return data, exists
}

func buildMarketHashName(config ItemConfig) string {
	gradeUpper := strings.ToUpper(config.Grade)
	if strings.ToUpper(config.Type) == "MATERIAL" {
		return config.Name["en-US"]
	}

	var gradeFormatted string
	switch gradeUpper {
	case "LEGENDARY":
		gradeFormatted = "Legendary"
	case "IMMORTAL":
		gradeFormatted = "Immortal"
	case "ARCANA":
		gradeFormatted = "Arcana"
	case "BEYOND":
		gradeFormatted = "Beyond"
	case "CELESTIAL":
		gradeFormatted = "Celestial"
	case "DIVINE":
		gradeFormatted = "Divine"
	case "COSMIC":
		gradeFormatted = "Cosmic"
	default:
		gradeFormatted = "Immortal"
	}
	return fmt.Sprintf("%s (%s) A", config.Name["en-US"], gradeFormatted)
}

func logMarketPrice(config ItemConfig, scope MarketScope, marketHashName string, priceText string, source string) {
	fmt.Printf("[MARKET:%s] [%s] %s (ID: %d, grade: %s, type: %s) | %s => %s\n", source, formatMarketScope(scope), config.Name["en-US"], config.ID, config.Grade, config.Type, marketHashName, priceText)
}

func updatePriceOverlay(itemID int, scope MarketScope, priceText string) {
	if ActiveItemID.Load() != int32(itemID) || currentMarketScope() != scope {
		return
	}
	setCurrentPriceText(priceText)
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

	setCurrentPriceText("Loading price...")
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
