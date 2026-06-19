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
	fetchPriceAndUpdateWithCache(config, true)
}

func refreshPriceAndUpdate(config ItemConfig) {
	fetchPriceAndUpdateWithCache(config, false)
}

func fetchPriceAndUpdateWithCache(config ItemConfig, useCache bool) {
	marketHashName := buildMarketHashName(config)
	now := time.Now()

	existingCache, hasExistingCache := marketCacheEntry(marketHashName)
	if useCache && hasExistingCache && isFreshMarketCache(existingCache, now) {
		logMarketPrice(config, marketHashName, existingCache.OverlayText, "cache")
		updatePriceOverlay(config.ID, existingCache.OverlayText)
		return
	}

	data, err := fetchMarketData(config, marketHashName, now)
	if err != nil {
		if overlayText, ok := staleMarketOverlayText(existingCache, hasExistingCache); ok {
			logMarketPrice(config, marketHashName, overlayText, "stale-cache")
			fmt.Printf("[MARKET:error] Steam market analysis failed, using stale cache: %v\n", err)
			updatePriceOverlay(config.ID, overlayText)
			return
		}

		overlayText := buildMarketOverlayText(unavailableMarketAnalysis(marketHashName, now))
		logMarketPrice(config, marketHashName, overlayText, "error")
		fmt.Printf("[MARKET:error] Steam market analysis failed: %v\n", err)
		updatePriceOverlay(config.ID, overlayText)
		return
	}

	PriceCacheMu.Lock()
	PriceCache[marketHashName] = data
	writePriceCacheFileLocked()
	PriceCacheMu.Unlock()

	source := "steam"
	if !useCache {
		source = "refresh"
	}
	logMarketPrice(config, marketHashName, data.OverlayText, source)
	updatePriceOverlay(config.ID, data.OverlayText)
}

func fetchMarketData(config ItemConfig, marketHashName string, now time.Time) (MarketData, error) {
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
		orderBook, hasOrderBook = parseSSRItemOrderBook(listingBody)
		history = parseSSRPriceHistory(listingBody)
		if len(history) == 0 {
			history = parseLegacySaleHistoryFromListing(listingBody)
		}

		if !hasOrderBook {
			itemNameID := parseItemNameID(listingBody)
			if itemNameID != "" {
				body, _, err := fetchItemOrdersHistogram(client, itemNameID, referer)
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

	if len(history) == 0 {
		body, _, err := fetchSaleHistory(client, marketHashName, referer)
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

	body, _, err := fetchPriceOverview(client, marketHashName)
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

func fetchItemOrdersHistogram(client *http.Client, itemNameID string, referer string) ([]byte, int, error) {
	apiURL := fmt.Sprintf(
		"https://steamcommunity.com/market/itemordershistogram?country=US&language=english&currency=1&item_nameid=%s&two_factor=0",
		url.QueryEscape(itemNameID),
	)
	return steamGet(client, apiURL, referer)
}

func fetchSaleHistory(client *http.Client, marketHashName string, referer string) ([]byte, int, error) {
	apiURL := fmt.Sprintf(
		"https://steamcommunity.com/market/pricehistory/?appid=%d&market_hash_name=%s",
		steamMarketAppID,
		url.QueryEscape(marketHashName),
	)
	return steamGet(client, apiURL, referer)
}

func fetchPriceOverview(client *http.Client, marketHashName string) ([]byte, int, error) {
	apiURL := fmt.Sprintf(
		"https://steamcommunity.com/market/priceoverview/?country=US&currency=1&appid=%d&market_hash_name=%s",
		steamMarketAppID,
		url.QueryEscape(marketHashName),
	)
	return steamGet(client, apiURL, "")
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

func marketCacheEntry(marketHashName string) (MarketData, bool) {
	PriceCacheMu.RLock()
	defer PriceCacheMu.RUnlock()
	data, exists := PriceCache[marketHashName]
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

func logMarketPrice(config ItemConfig, marketHashName string, priceText string, source string) {
	fmt.Printf("[MARKET:%s] %s (ID: %d, grade: %s, type: %s) | %s => %s\n", source, config.Name["en-US"], config.ID, config.Grade, config.Type, marketHashName, priceText)
}

func updatePriceOverlay(itemID int, priceText string) {
	if ActiveItemID.Load() != int32(itemID) {
		return
	}
	setCurrentPriceText(priceText)
	redrawOverlay()
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

	configs := cachedPriceConfigs()
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
			refreshPriceAndUpdate(config)
		}
		fmt.Printf("Cached price refresh completed: %d item(s).\n", len(configs))
	}()

	return len(configs)
}

func cachedPriceConfigs() []ItemConfig {
	PriceCacheMu.RLock()
	cachedNames := make(map[string]struct{}, len(PriceCache))
	for marketHashName := range PriceCache {
		cachedNames[marketHashName] = struct{}{}
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
