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

type RequestPriority int

const (
	RequestPriorityNormal RequestPriority = iota
	RequestPriorityHigh
)

func (priority RequestPriority) String() string {
	if priority == RequestPriorityHigh {
		return "high"
	}
	return "normal"
}

type SteamRequestMetric struct {
	Endpoint        string
	Priority        RequestPriority
	LimiterWait     time.Duration
	RequestDuration time.Duration
	StatusCode      int
	RetryAfter      string
	Error           string
	URL             string
	AppID           string
	MarketHashName  string
	ResponseBody    string
}

type steamRateLimiter struct {
	mu            sync.Mutex
	cond          *sync.Cond
	lastStartedAt time.Time
	waitingHigh   int
}

func newSteamRateLimiter() *steamRateLimiter {
	limiter := &steamRateLimiter{}
	limiter.cond = sync.NewCond(&limiter.mu)
	return limiter
}

var steamRequestLimiter = newSteamRateLimiter()

var steamRequestMetricLogger = struct {
	sync.RWMutex
	log func(SteamRequestMetric)
}{}

func SetSteamRequestMetricLogger(logger func(SteamRequestMetric)) {
	steamRequestMetricLogger.Lock()
	steamRequestMetricLogger.log = logger
	steamRequestMetricLogger.Unlock()
}

func FetchData(config catalog.ItemConfig, marketHashName string, now time.Time, scope MarketScope) (MarketData, error) {
	return FetchDataWithPriority(config, marketHashName, now, scope, RequestPriorityNormal)
}

func FetchDataWithPriority(config catalog.ItemConfig, marketHashName string, now time.Time, scope MarketScope, priority RequestPriority) (MarketData, error) {
	data, err := fetchDataForScope(config, marketHashName, now, scope, priority)
	if err != nil {
		return data, err
	}
	if scope == defaultMarketScope() || hasCompleteMarketAnalysis(data.Analysis) {
		return data, nil
	}

	data.Analysis.USDDataFallbackAttempted = true
	usdData, usdErr := fetchDataForScope(config, marketHashName, now, defaultMarketScope(), priority)
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

func PriceHistoryURL(marketHashName string) string {
	return fmt.Sprintf(
		"https://steamcommunity.com/market/pricehistory/?appid=%d&market_hash_name=%s",
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

func fetchDataForScope(config catalog.ItemConfig, marketHashName string, now time.Time, scope MarketScope, priority RequestPriority) (MarketData, error) {
	client := &http.Client{Timeout: steamRequestTimeout}
	referer := ListingURLForScope(config, scope)
	var requestErrors []string

	var orderBook MarketOrderBook
	hasOrderBook := false
	var history []MarketSalePoint

	var iconURL string
	listingBody, _, err := steamGetWithPriority(client, referer, "", priority)
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
				body, _, err := fetchItemOrdersHistogram(client, itemNameID, referer, scope, priority)
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

	if len(history) > 0 || hasOrderBook {
		data := marketDataFromBaseHistory(marketHashName, orderBook, hasOrderBook, history, now, scope.Currency)
		data.Analysis.IconURL = iconURL
		if len(history) > 0 || data.Analysis.HasSuggested {
			return data, nil
		}
	}

	body, _, err := fetchPriceOverview(client, marketHashName, scope, priority)
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

	if len(history) == 0 {
		body, _, err := fetchSaleHistory(client, marketHashName, referer, priority)
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
		data := marketDataFromBaseHistory(marketHashName, orderBook, hasOrderBook, history, now, scope.Currency)
		data.Analysis.IconURL = iconURL
		return data, nil
	}

	if len(requestErrors) == 0 {
		return MarketData{}, fmt.Errorf("no market data available")
	}
	return MarketData{}, fmt.Errorf("%s", strings.Join(requestErrors, "; "))
}

func fetchItemOrdersHistogram(client *http.Client, itemNameID string, referer string, scope MarketScope, priority RequestPriority) ([]byte, int, error) {
	return steamGetWithPriority(client, ItemOrdersHistogramURL(itemNameID, scope), referer, priority)
}

func fetchSaleHistory(client *http.Client, marketHashName string, referer string, priority RequestPriority) ([]byte, int, error) {
	return steamGetWithPriority(client, PriceHistoryURL(marketHashName), referer, priority)
}

func fetchPriceOverview(client *http.Client, marketHashName string, scope MarketScope, priority RequestPriority) ([]byte, int, error) {
	return steamGetWithPriority(client, PriceOverviewURL(marketHashName, scope), "", priority)
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
	return steamGetWithPriority(client, targetURL, referer, RequestPriorityNormal)
}

func steamGetWithPriority(client *http.Client, targetURL string, referer string, priority RequestPriority) ([]byte, int, error) {
	endpoint := getEndpointFromURL(targetURL)
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

	waitStartedAt := time.Now()
	waitForSteamRequestTurn(priority)
	waitDuration := time.Since(waitStartedAt)

	requestStartedAt := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		emitSteamRequestMetric(SteamRequestMetric{
			Endpoint:        endpoint,
			Priority:        priority,
			LimiterWait:     waitDuration,
			RequestDuration: time.Since(requestStartedAt),
			Error:           err.Error(),
		})
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		emitSteamRequestMetric(SteamRequestMetric{
			Endpoint:        endpoint,
			Priority:        priority,
			LimiterWait:     waitDuration,
			RequestDuration: time.Since(requestStartedAt),
			StatusCode:      resp.StatusCode,
			RetryAfter:      resp.Header.Get("Retry-After"),
			Error:           readErr.Error(),
		})
		return body, resp.StatusCode, readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		urlText, appID, marketHashName, responseBody := steamErrorMetricDetails(endpoint, targetURL, body)
		steamErr := &SteamError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Endpoint:   endpoint,
			RetryAfter: resp.Header.Get("Retry-After"),
		}
		emitSteamRequestMetric(SteamRequestMetric{
			Endpoint:        steamErr.Endpoint,
			Priority:        priority,
			LimiterWait:     waitDuration,
			RequestDuration: time.Since(requestStartedAt),
			StatusCode:      resp.StatusCode,
			RetryAfter:      steamErr.RetryAfter,
			Error:           steamErr.Error(),
			URL:             urlText,
			AppID:           appID,
			MarketHashName:  marketHashName,
			ResponseBody:    responseBody,
		})
		return body, resp.StatusCode, steamErr
	}
	emitSteamRequestMetric(SteamRequestMetric{
		Endpoint:        endpoint,
		Priority:        priority,
		LimiterWait:     waitDuration,
		RequestDuration: time.Since(requestStartedAt),
		StatusCode:      resp.StatusCode,
		RetryAfter:      resp.Header.Get("Retry-After"),
	})
	return body, resp.StatusCode, nil
}

func steamErrorMetricDetails(endpoint string, targetURL string, body []byte) (string, string, string, string) {
	if endpoint != "pricehistory" {
		return "", "", "", ""
	}
	parsed, err := url.Parse(targetURL)
	if err != nil {
		return targetURL, "", "", limitMetricBody(body, 1024)
	}
	query := parsed.Query()
	return targetURL, query.Get("appid"), query.Get("market_hash_name"), limitMetricBody(body, 1024)
}

func limitMetricBody(body []byte, maxBytes int) string {
	if maxBytes <= 0 || len(body) == 0 {
		return ""
	}
	if len(body) > maxBytes {
		body = body[:maxBytes]
	}
	return strings.TrimSpace(string(body))
}

func emitSteamRequestMetric(metric SteamRequestMetric) {
	steamRequestMetricLogger.RLock()
	logger := steamRequestMetricLogger.log
	steamRequestMetricLogger.RUnlock()
	if logger != nil {
		logger(metric)
	}
}

func waitForSteamRequestTurn(priority RequestPriority) {
	highPriority := priority == RequestPriorityHigh
	steamRequestLimiter.mu.Lock()
	if highPriority {
		steamRequestLimiter.waitingHigh++
	}

	for {
		if !highPriority && steamRequestLimiter.waitingHigh > 0 {
			steamRequestLimiter.cond.Wait()
			continue
		}

		wait := time.Duration(0)
		if !steamRequestLimiter.lastStartedAt.IsZero() {
			delay := jitteredSteamRequestSpacing()
			wait = time.Until(steamRequestLimiter.lastStartedAt.Add(delay))
		}
		if wait <= 0 {
			steamRequestLimiter.lastStartedAt = time.Now()
			if highPriority {
				steamRequestLimiter.waitingHigh--
			}
			steamRequestLimiter.cond.Broadcast()
			steamRequestLimiter.mu.Unlock()
			return
		}

		timer := time.NewTimer(wait)
		steamRequestLimiter.mu.Unlock()
		<-timer.C
		steamRequestLimiter.mu.Lock()
	}
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
