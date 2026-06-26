package market

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/catalog"
)

const (
	SteamAppID          = 3678970
	steamRequestTimeout = 6 * time.Second
)

func FetchData(config catalog.ItemConfig, marketHashName string, now time.Time, scope MarketScope) (MarketData, error) {
	data, err := fetchDataForScope(config, marketHashName, now, scope)
	if err != nil || scope == defaultMarketScope() || hasCompleteMarketAnalysis(data.Analysis) {
		return data, err
	}

	data.Analysis.USDDataFallbackAttempted = true
	usdData, usdErr := fetchDataForScope(config, marketHashName, now, defaultMarketScope())
	if usdErr != nil {
		return data, nil
	}
	return mergeMarketDataWithUSDFallback(data, usdData, scope), nil
}

func BuildHashName(config catalog.ItemConfig) string {
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

func ListingURL(config catalog.ItemConfig) string {
	return fmt.Sprintf("https://steamcommunity.com/market/listings/%d/%s", SteamAppID, url.PathEscape(BuildHashName(config)))
}

func ListingURLForScope(config catalog.ItemConfig, scope MarketScope) string {
	return fmt.Sprintf(
		"%s?country=%s&currency=%d",
		ListingURL(config),
		url.QueryEscape(scope.Region.CountryCode),
		scope.Currency.SteamCurrencyID,
	)
}

func PriceHistoryURL(marketHashName string, scope MarketScope) string {
	return fmt.Sprintf(
		"https://steamcommunity.com/market/pricehistory/?country=%s&currency=%d&appid=%d&market_hash_name=%s",
		url.QueryEscape(scope.Region.CountryCode),
		scope.Currency.SteamCurrencyID,
		SteamAppID,
		url.QueryEscape(marketHashName),
	)
}

func ItemOrdersHistogramURL(itemNameID string, scope MarketScope) string {
	return fmt.Sprintf(
		"https://steamcommunity.com/market/itemordershistogram?country=%s&language=english&currency=%d&item_nameid=%s&two_factor=0",
		url.QueryEscape(scope.Region.CountryCode),
		scope.Currency.SteamCurrencyID,
		url.QueryEscape(itemNameID),
	)
}

func PriceOverviewURL(marketHashName string, scope MarketScope) string {
	return fmt.Sprintf(
		"https://steamcommunity.com/market/priceoverview/?country=%s&currency=%d&appid=%d&market_hash_name=%s",
		url.QueryEscape(scope.Region.CountryCode),
		scope.Currency.SteamCurrencyID,
		SteamAppID,
		url.QueryEscape(marketHashName),
	)
}

func fetchDataForScope(config catalog.ItemConfig, marketHashName string, now time.Time, scope MarketScope) (MarketData, error) {
	client := &http.Client{Timeout: steamRequestTimeout}
	referer := ListingURLForScope(config, scope)
	var requestErrors []string

	var orderBook MarketOrderBook
	hasOrderBook := false
	var history []MarketSalePoint

	listingBody, _, err := steamGet(client, referer, "")
	if err != nil {
		requestErrors = append(requestErrors, "listing: "+err.Error())
	} else {
		if isSSRListingForScope(listingBody, scope) {
			orderBook, hasOrderBook = parseSSRItemOrderBook(listingBody, scope.Currency)
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

	if len(history) == 0 {
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
		return marketDataFromSources(marketHashName, orderBook, hasOrderBook, history, now, scope.Currency), nil
	}

	body, _, err := fetchPriceOverview(client, marketHashName, scope)
	if err != nil {
		requestErrors = append(requestErrors, "priceoverview: "+err.Error())
	} else if data, ok := marketDataFromPriceOverview(marketHashName, body, now, scope.Currency); ok {
		return data, nil
	} else {
		requestErrors = append(requestErrors, "priceoverview: response did not contain price data")
	}

	if len(requestErrors) == 0 {
		return MarketData{}, fmt.Errorf("no market data available")
	}
	return MarketData{}, fmt.Errorf("%s", strings.Join(requestErrors, "; "))
}

func fetchItemOrdersHistogram(client *http.Client, itemNameID string, referer string, scope MarketScope) ([]byte, int, error) {
	return steamGet(client, ItemOrdersHistogramURL(itemNameID, scope), referer)
}

func fetchSaleHistory(client *http.Client, marketHashName string, referer string, scope MarketScope) ([]byte, int, error) {
	return steamGet(client, PriceHistoryURL(marketHashName, scope), referer)
}

func fetchPriceOverview(client *http.Client, marketHashName string, scope MarketScope) ([]byte, int, error) {
	return steamGet(client, PriceOverviewURL(marketHashName, scope), "")
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
