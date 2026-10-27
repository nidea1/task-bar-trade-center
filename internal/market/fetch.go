package market

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/catalog"
)

const (
	SteamAppID                    = catalog.SteamAppID
	steamRequestTimeout           = 6 * time.Second
	steamSearchPageRequestTimeout = 15 * time.Second
	steamRequestSpacing           = 2500 * time.Millisecond
	steamRequestJitter            = 0.15
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
	backoffUntil  time.Time
	backoffStep   int
}

func newSteamRateLimiter() *steamRateLimiter {
	limiter := &steamRateLimiter{}
	limiter.cond = sync.NewCond(&limiter.mu)
	return limiter
}

var steamRequestLimiter = newSteamRateLimiter()

var steamHTTPTransport = newSteamHTTPTransport()

var steamRequestMetricLogger = struct {
	sync.RWMutex
	log func(SteamRequestMetric)
}{}

func SetSteamRequestMetricLogger(logger func(SteamRequestMetric)) {
	steamRequestMetricLogger.Lock()
	steamRequestMetricLogger.log = logger
	steamRequestMetricLogger.Unlock()
}

func newSteamHTTPTransport() http.RoundTripper {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return http.DefaultTransport
	}
	transport := base.Clone()
	// Steam's compressed cache variant can return a successful React shell
	// without the listing's SSR market data. Omitting Accept-Encoding selects
	// the complete response; DisableCompression prevents Go from adding gzip.
	transport.DisableCompression = true
	return transport
}

func newSteamHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: steamHTTPTransport}
}

func FetchData(config catalog.ItemConfig, marketHashName string, now time.Time, scope MarketScope) (MarketData, error) {
	return FetchDataWithPriority(config, marketHashName, now, scope, RequestPriorityNormal)
}

func FetchDataWithPriority(config catalog.ItemConfig, marketHashName string, now time.Time, scope MarketScope, priority RequestPriority) (MarketData, error) {
	data, err := fetchDataForScope(config, marketHashName, now, scope, priority)
	if err != nil {
		return data, err
	}
	if scope == defaultMarketScope() || hasCompleteMarketAnalysis(data.Analysis) || data.Analysis.USDDataFallbackAttempted {
		return data, nil
	}

	data.Analysis.USDDataFallbackAttempted = true
	usdData, usdErr := fetchDataForScope(config, marketHashName, now, defaultMarketScope(), priority)
	if usdErr != nil {
		return data, nil
	}
	return mergeMarketDataWithUSDFallback(data, usdData, scope), nil
}

func BuildHashName(config catalog.ItemConfig) string {
	if config.HasMarketIdentity() {
		return config.MarketHashName
	}
	return ""
}

func ListingURL(config catalog.ItemConfig) string {
	name := BuildHashName(config)
	if name == "" {
		return ""
	}
	return fmt.Sprintf("https://steamcommunity.com/market/listings/%d/%s", SteamAppID, url.PathEscape(name))
}

func ListingURLForScope(config catalog.ItemConfig, scope MarketScope) string {
	listing := ListingURL(config)
	if listing == "" {
		return ""
	}
	return fmt.Sprintf(
		"%s?country=%s&currency=%d",
		listing,
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

func MarketSearchPageURL(marketHashName string) string {
	query := url.Values{}
	query.Set("q", marketHashName)
	query.Set("appid", fmt.Sprint(SteamAppID))
	query.Set("country", "US")
	query.Set("currency", "1")
	query.Set("l", "english")
	return "https://steamcommunity.com/market/search?" + query.Encode()
}

func fetchDataForScope(config catalog.ItemConfig, marketHashName string, now time.Time, scope MarketScope, priority RequestPriority) (MarketData, error) {
	if marketHashName == "" || marketHashName != BuildHashName(config) {
		return MarketData{}, fmt.Errorf("%w: item %d", ErrIdentityUnavailable, config.ID)
	}
	client := newSteamHTTPClient(steamRequestTimeout)
	listingURL := ListingURLForScope(config, scope)
	listingBody, _, err := steamGetWithPriority(client, listingURL, "", priority)
	if err != nil {
		return MarketData{}, fmt.Errorf("listing: %w", err)
	}

	sources, listingErr := verifiedListingSources(config, listingBody)
	iconURL := ""
	if listingErr == nil {
		iconURL = ParseIconURL(sources)
		data, err := marketDataFromListingSSR(marketHashName, sources, iconURL, now, scope)
		if err == nil {
			data.Analysis.SteamDefinitionID = config.SteamDefinitionID
			return data, nil
		}
		listingErr = err
	}

	searchClient := newSteamHTTPClient(steamSearchPageRequestTimeout)
	searchBody, _, searchErr := steamGetWithPriority(searchClient, MarketSearchPageURL(marketHashName), listingURL, priority)
	if searchErr != nil {
		return MarketData{}, fmt.Errorf("%w; market search page: %w", listingErr, searchErr)
	}
	searchSources, identityErr := verifiedSearchSources(config, searchBody)
	if identityErr != nil {
		return MarketData{}, fmt.Errorf("%v; search: %w", listingErr, identityErr)
	}
	usdData, ok := marketDataFromSearchHTML(marketHashName, searchSources, now, defaultMarketScope().Currency)
	if !ok {
		return MarketData{}, fmt.Errorf("%v; market search page did not contain an exact price match", listingErr)
	}
	usdData.Analysis.IconURL = iconURL
	usdData.Analysis.SteamDefinitionID = config.SteamDefinitionID
	if scope.Currency.Code == "USD" {
		return usdData, nil
	}
	local := MarketData{CachedAt: now, Analysis: unavailableMarketAnalysis(marketHashName, now, scope.Currency)}
	local.Analysis.IconURL = iconURL
	data := mergeMarketDataWithUSDFallback(local, usdData, scope)
	data.Analysis.PriceSource = "market-search-html-usd"
	data.Analysis.SteamDefinitionID = config.SteamDefinitionID
	return data, nil
}

func marketDataFromListingSSR(marketHashName string, listingBody []byte, iconURL string, now time.Time, scope MarketScope) (MarketData, error) {
	listingCurrency, hasListingCurrency := ssrListingCurrency(listingBody)
	if !hasListingCurrency {
		return MarketData{}, fmt.Errorf("listing SSR did not contain a supported currency")
	}
	if listingCurrency.SteamCurrencyID != scope.Currency.SteamCurrencyID && listingCurrency.Code != "USD" {
		return MarketData{}, fmt.Errorf(
			"listing SSR currency mismatch: got %s, want %s or USD",
			listingCurrency.Code,
			scope.Currency.Code,
		)
	}

	orderBook, hasOrderBook := parseSSRItemOrderBook(listingBody, listingCurrency)
	history := parseSSRPriceHistory(listingBody)
	if len(history) == 0 {
		history = parseLegacySaleHistoryFromListing(listingBody)
	}
	if !hasOrderBook && len(history) == 0 {
		return MarketData{}, fmt.Errorf("listing SSR did not contain order book or sale history")
	}

	data := marketDataFromBaseHistory(marketHashName, orderBook, hasOrderBook, history, now, listingCurrency)
	data.Analysis.PriceSource = "listing-ssr"
	data.Analysis.IconURL = iconURL
	if listingCurrency.Code == "USD" && scope.Currency.Code != "USD" {
		local := MarketData{CachedAt: now, Analysis: unavailableMarketAnalysis(marketHashName, now, scope.Currency)}
		local.Analysis.IconURL = iconURL
		data = mergeMarketDataWithUSDFallback(local, data, scope)
		data.Analysis.PriceSource = "listing-ssr-usd"
	}
	return data, nil
}

func fetchItemOrdersHistogram(client *http.Client, itemNameID string, referer string, scope MarketScope, priority RequestPriority) ([]byte, int, error) {
	return steamGetWithPriority(client, ItemOrdersHistogramURL(itemNameID, scope), referer, priority)
}

func fetchSaleHistory(client *http.Client, marketHashName string, referer string, priority RequestPriority) ([]byte, int, error) {
	return steamGetWithPriority(client, PriceHistoryURL(marketHashName), referer, priority)
}

type SteamError struct {
	StatusCode int
	Status     string
	Endpoint   string
	RetryAfter string
	RetryUntil time.Time
}

func (e *SteamError) RetryAt() time.Time { return e.RetryUntil }

func (e *SteamError) Error() string {
	return fmt.Sprintf("status %s", e.Status)
}

func getEndpointFromURL(targetURL string) string {
	if strings.Contains(targetURL, "/pricehistory") {
		return "pricehistory"
	}
	if strings.Contains(targetURL, "/itemordershistogram") {
		return "itemordershistogram"
	}
	if strings.Contains(targetURL, "/market/search") {
		return "search-page"
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
	if referer != "" {
		req.Header.Set("Referer", referer)
	}

	waitStartedAt := time.Now()
	if err := waitForSteamRequestTurn(priority); err != nil {
		err.Endpoint = endpoint
		return nil, err.StatusCode, err
	}
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
	var retryUntil time.Time
	if resp.StatusCode == http.StatusTooManyRequests {
		retryUntil = steamRequestLimiter.enterBackoff(resp.Header.Get("Retry-After"), time.Now())
	} else if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		steamRequestLimiter.mu.Lock()
		steamRequestLimiter.backoffStep = 0
		steamRequestLimiter.mu.Unlock()
	}

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil && resp.StatusCode != http.StatusTooManyRequests {
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
			RetryUntil: retryUntil,
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

func (limiter *steamRateLimiter) enterBackoff(retryAfter string, now time.Time) time.Time {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	backoffs := [...]time.Duration{15 * time.Minute, 30 * time.Minute, time.Hour}
	delay := backoffs[limiter.backoffStep]
	if limiter.backoffStep < len(backoffs)-1 {
		limiter.backoffStep++
	}
	if seconds, err := strconv.ParseInt(retryAfter, 10, 64); err == nil && seconds > 0 && seconds <= int64((1<<63-1)/time.Second) {
		delay = time.Duration(seconds) * time.Second
	} else if deadline, err := http.ParseTime(retryAfter); err == nil && deadline.After(now) {
		delay = deadline.Sub(now)
	}
	if deadline := now.Add(delay); deadline.After(limiter.backoffUntil) {
		limiter.backoffUntil = deadline
	}
	limiter.cond.Broadcast()
	return limiter.backoffUntil
}

func waitForSteamRequestTurn(priority RequestPriority) *SteamError {
	highPriority := priority == RequestPriorityHigh
	steamRequestLimiter.mu.Lock()
	if highPriority {
		steamRequestLimiter.waitingHigh++
	}

	for {
		if now := time.Now(); now.Before(steamRequestLimiter.backoffUntil) {
			if highPriority {
				steamRequestLimiter.waitingHigh--
			}
			deadline := steamRequestLimiter.backoffUntil
			steamRequestLimiter.cond.Broadcast()
			steamRequestLimiter.mu.Unlock()
			return &SteamError{
				StatusCode: http.StatusTooManyRequests,
				Status:     "429 Too Many Requests (Steam cooldown until " + deadline.Format(time.RFC3339) + ")",
				RetryUntil: deadline,
			}
		}
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
			return nil
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
