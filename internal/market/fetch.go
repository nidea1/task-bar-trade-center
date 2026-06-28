package market

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/catalog"
)

const (
	SteamAppID          = 3678970
	steamRequestTimeout = 6 * time.Second
	steamRequestSpacing = 2500 * time.Millisecond
	steamRequestJitter  = 0.15
)

var steamRequestLimiter = struct {
	sync.Mutex
	lastStartedAt time.Time
}{}

func FetchData(config catalog.ItemConfig, marketHashName string, now time.Time, scope MarketScope) (MarketData, error) {
	data, err := fetchDataForScope(config, marketHashName, now, scope)
	if err != nil {
		return data, err
	}
	if scope == defaultMarketScope() || hasCompleteMarketAnalysis(data.Analysis) {
		return data, nil
	}

	data.Analysis.USDDataFallbackAttempted = true
	usdData, usdErr := fetchDataForScope(config, marketHashName, now, defaultMarketScope())
	if usdErr != nil {
		if se, ok := usdErr.(*SteamError); ok && se.StatusCode == 429 {
			return data, usdErr
		}
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

	var iconURL string
	listingBody, _, err := steamGet(client, referer, "")
	if err != nil {
		if se, ok := err.(*SteamError); ok && se.StatusCode == 429 {
			return MarketData{}, err
		}
		requestErrors = append(requestErrors, "listing: "+err.Error())
	} else {
		iconURL = ParseIconURL(listingBody)
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
					if se, ok := err.(*SteamError); ok && se.StatusCode == 429 {
						return MarketData{}, err
					}
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
			if se, ok := err.(*SteamError); ok && se.StatusCode == 429 {
				return MarketData{}, err
			}
			requestErrors = append(requestErrors, "pricehistory: "+err.Error())
		} else {
			history = parseSaleHistoryResponse(body)
			if len(history) == 0 {
				requestErrors = append(requestErrors, "pricehistory: response did not contain sale data")
			}
		}
	}

	if hasOrderBook || len(history) > 0 {
		data := marketDataFromSources(marketHashName, orderBook, hasOrderBook, history, now, scope.Currency)
		data.Analysis.IconURL = iconURL
		return data, nil
	}

	body, _, err := fetchPriceOverview(client, marketHashName, scope)
	if err != nil {
		if se, ok := err.(*SteamError); ok && se.StatusCode == 429 {
			return MarketData{}, err
		}
		requestErrors = append(requestErrors, "priceoverview: "+err.Error())
	} else if data, ok := marketDataFromPriceOverview(marketHashName, body, now, scope.Currency); ok {
		data.Analysis.IconURL = iconURL
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

type SteamError struct {
	StatusCode int
	Status     string
	Endpoint   string
	RetryAfter string
}

func (e *SteamError) Error() string {
	return fmt.Sprintf("status %s", e.Status)
}

func getEndpointFromURL(targetURL string) string {
	if strings.Contains(targetURL, "/pricehistory") {
		return "pricehistory"
	}
	if strings.Contains(targetURL, "/priceoverview") {
		return "priceoverview"
	}
	if strings.Contains(targetURL, "/itemordershistogram") {
		return "itemordershistogram"
	}
	if strings.Contains(targetURL, "/listings/") {
		return "listings"
	}
	return "unknown"
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

	waitForSteamRequestTurn()
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
		return body, resp.StatusCode, &SteamError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Endpoint:   getEndpointFromURL(targetURL),
			RetryAfter: resp.Header.Get("Retry-After"),
		}
	}
	return body, resp.StatusCode, nil
}

func waitForSteamRequestTurn() {
	steamRequestLimiter.Lock()
	defer steamRequestLimiter.Unlock()

	if !steamRequestLimiter.lastStartedAt.IsZero() {
		delay := jitteredSteamRequestSpacing()
		if wait := time.Until(steamRequestLimiter.lastStartedAt.Add(delay)); wait > 0 {
			time.Sleep(wait)
		}
	}
	steamRequestLimiter.lastStartedAt = time.Now()
}

func jitteredSteamRequestSpacing() time.Duration {
	if steamRequestJitter <= 0 {
		return steamRequestSpacing
	}
	min := 1 - steamRequestJitter
	max := 1 + steamRequestJitter
	factor := min + rand.Float64()*(max-min)
	return time.Duration(float64(steamRequestSpacing) * factor)
}
