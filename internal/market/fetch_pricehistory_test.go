package market

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/catalog"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestPriceHistoryURLOmitsCountryAndCurrency(t *testing.T) {
	rawURL := PriceHistoryURL("Astral Diamond")
	if strings.Contains(rawURL, "country=") {
		t.Fatalf("pricehistory URL contains country: %s", rawURL)
	}
	if strings.Contains(rawURL, "currency=") {
		t.Fatalf("pricehistory URL contains currency: %s", rawURL)
	}
	if !strings.Contains(rawURL, "appid=3678970") {
		t.Fatalf("pricehistory URL missing appid: %s", rawURL)
	}
	if !strings.Contains(rawURL, "market_hash_name=Astral+Diamond") {
		t.Fatalf("pricehistory URL missing escaped market_hash_name: %s", rawURL)
	}
}

func TestScopedPriceOverviewSkipsPricehistory(t *testing.T) {
	originalTransport := http.DefaultTransport
	originalLimiter := steamRequestLimiter
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
		steamRequestLimiter = originalLimiter
		SetSteamRequestMetricLogger(nil)
	})

	var priceHistoryRequests int
	SetSteamRequestMetricLogger(func(metric SteamRequestMetric) {
		if metric.Endpoint != "pricehistory" {
			return
		}
		priceHistoryRequests++
	})

	steamRequestLimiter = newSteamRateLimiter()
	http.DefaultTransport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		resetSteamRequestLimiterForTest()
		switch {
		case strings.Contains(request.URL.Path, "/market/listings/"):
			return testHTTPResponse(http.StatusOK, `<html></html>`), nil
		case strings.Contains(request.URL.Path, "/market/pricehistory/"):
			t.Fatalf("pricehistory should not be requested when scoped priceoverview has price data")
			return testHTTPResponse(http.StatusBadRequest, `[]`), nil
		case strings.Contains(request.URL.Path, "/market/priceoverview/"):
			return testHTTPResponse(http.StatusOK, `{"success":true,"lowest_price":"\u00a30.26","median_price":"\u00a30.30","volume":"42"}`), nil
		default:
			t.Fatalf("unexpected request URL: %s", request.URL.String())
			return testHTTPResponse(http.StatusNotFound, ""), nil
		}
	})

	scope, ok := marketScopeFor("GBP", "GB")
	if !ok {
		t.Fatal("expected GBP/GB scope")
	}
	config := catalog.ItemConfig{
		ID:   116002,
		Type: "MATERIAL",
		Name: map[string]string{"en-US": "Astral Diamond"},
	}
	data, err := fetchDataForScope(config, "Astral Diamond", time.Unix(1700000000, 0), scope, RequestPriorityHigh)
	if err != nil {
		t.Fatalf("fetchDataForScope returned error: %v", err)
	}
	if !data.Analysis.HasSuggested {
		t.Fatalf("analysis did not contain suggested price: %+v", data.Analysis)
	}
	if data.Analysis.PricePrefix != scope.Currency.PricePrefix || data.Analysis.PriceSuffix != scope.Currency.PriceSuffix {
		t.Fatalf("analysis format = %q/%q, want %q/%q", data.Analysis.PricePrefix, data.Analysis.PriceSuffix, scope.Currency.PricePrefix, scope.Currency.PriceSuffix)
	}
	if priceHistoryRequests != 0 {
		t.Fatalf("pricehistory requests = %d, want 0", priceHistoryRequests)
	}
}

func TestScopedListingOrderBookSkipsPricehistory(t *testing.T) {
	originalTransport := http.DefaultTransport
	originalLimiter := steamRequestLimiter
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
		steamRequestLimiter = originalLimiter
		SetSteamRequestMetricLogger(nil)
	})

	scope, ok := marketScopeFor("GBP", "GB")
	if !ok {
		t.Fatal("expected GBP/GB scope")
	}

	steamRequestLimiter = newSteamRateLimiter()
	http.DefaultTransport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		resetSteamRequestLimiterForTest()
		switch {
		case strings.Contains(request.URL.Path, "/market/listings/"):
			body := fmt.Sprintf(`<script>JSON.parse("{\"state\":{\"data\":{\"amtMaxBuyOrder\":2,\"amtMinSellOrder\":3,\"eCurrency\":%d,\"cBuyOrders\":4,\"cSellOrders\":5,\"rgCompactBuyOrders\":[2,6],\"rgCompactSellOrders\":[3,7]},\"queryKey\":[\"market\",\"orderbook\",3678970,\"Astral Diamond\"]}}")</script>`, scope.Currency.SteamCurrencyID)
			return testHTTPResponse(http.StatusOK, body), nil
		case strings.Contains(request.URL.Path, "/market/pricehistory/"):
			t.Fatalf("pricehistory should not be requested when scoped listing has order book data")
			return testHTTPResponse(http.StatusBadRequest, `[]`), nil
		case strings.Contains(request.URL.Path, "/market/priceoverview/"):
			t.Fatalf("priceoverview should not be requested when scoped listing has order book data")
			return testHTTPResponse(http.StatusOK, `{}`), nil
		default:
			t.Fatalf("unexpected request URL: %s", request.URL.String())
			return testHTTPResponse(http.StatusNotFound, ""), nil
		}
	})

	config := catalog.ItemConfig{
		ID:   116002,
		Type: "MATERIAL",
		Name: map[string]string{"en-US": "Astral Diamond"},
	}
	data, err := fetchDataForScope(config, "Astral Diamond", time.Unix(1700000000, 0), scope, RequestPriorityHigh)
	if err != nil {
		t.Fatalf("fetchDataForScope returned error: %v", err)
	}
	if !data.Analysis.HasSuggested || data.Analysis.PricePrefix != scope.Currency.PricePrefix {
		t.Fatalf("analysis did not use scoped order book: %+v", data.Analysis)
	}
}

func TestScopedSuggestedPriceMergesUSDFallbackDetails(t *testing.T) {
	originalTransport := http.DefaultTransport
	originalLimiter := steamRequestLimiter
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
		steamRequestLimiter = originalLimiter
	})

	scope, ok := marketScopeFor("GBP", "GB")
	if !ok {
		t.Fatal("expected GBP/GB scope")
	}
	usdScope := defaultMarketScope()

	steamRequestLimiter = newSteamRateLimiter()
	http.DefaultTransport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		resetSteamRequestLimiterForTest()
		switch {
		case strings.Contains(request.URL.Path, "/market/listings/"):
			if request.URL.Query().Get("currency") == fmt.Sprint(scope.Currency.SteamCurrencyID) {
				return testHTTPResponse(http.StatusOK, `<html></html>`), nil
			}
			if request.URL.Query().Get("currency") == fmt.Sprint(usdScope.Currency.SteamCurrencyID) {
				body := fmt.Sprintf(`<script>JSON.parse("{\"state\":{\"data\":{\"amtMaxBuyOrder\":80,\"amtMinSellOrder\":101,\"eCurrency\":%d,\"cBuyOrders\":4,\"cSellOrders\":5,\"rgCompactBuyOrders\":[80,6],\"rgCompactSellOrders\":[101,7]},\"queryKey\":[\"market\",\"orderbook\",3678970,\"Astral Diamond\"]}}")</script>
<script>JSON.parse("{\"state\":{\"data\":{\"history\":[{\"time\":1700000000,\"price_median\":1.00,\"purchases\":3}]},\"queryKey\":[\"market\",\"pricehistory\",3678970,\"Astral Diamond\"]}}")</script>`, usdScope.Currency.SteamCurrencyID)
				return testHTTPResponse(http.StatusOK, body), nil
			}
			t.Fatalf("unexpected listing scope: %s", request.URL.String())
			return testHTTPResponse(http.StatusNotFound, ""), nil
		case strings.Contains(request.URL.Path, "/market/priceoverview/"):
			if request.URL.Query().Get("currency") != fmt.Sprint(scope.Currency.SteamCurrencyID) {
				t.Fatalf("unexpected priceoverview scope: %s", request.URL.String())
			}
			return testHTTPResponse(http.StatusOK, `{"success":true,"lowest_price":"\u00a30.51","median_price":"\u00a30.55","volume":"42"}`), nil
		case strings.Contains(request.URL.Path, "/market/pricehistory/"):
			t.Fatalf("pricehistory endpoint should not be requested in this fallback flow")
			return testHTTPResponse(http.StatusBadRequest, `[]`), nil
		default:
			t.Fatalf("unexpected request URL: %s", request.URL.String())
			return testHTTPResponse(http.StatusNotFound, ""), nil
		}
	})

	config := catalog.ItemConfig{
		ID:   116002,
		Type: "MATERIAL",
		Name: map[string]string{"en-US": "Astral Diamond"},
	}
	data, err := FetchDataWithPriority(config, "Astral Diamond", time.Unix(1700000100, 0), scope, RequestPriorityHigh)
	if err != nil {
		t.Fatalf("FetchDataWithPriority returned error: %v", err)
	}
	analysis := data.Analysis
	if !analysis.HasSuggested || !analysis.HasOrderBook || !analysis.HasSaleHistory || !analysis.HasLastSold {
		t.Fatalf("analysis missing merged fallback details: %+v", analysis)
	}
	if analysis.PricePrefix != scope.Currency.PricePrefix || analysis.PriceSuffix != scope.Currency.PriceSuffix {
		t.Fatalf("analysis format = %q/%q, want %q/%q", analysis.PricePrefix, analysis.PriceSuffix, scope.Currency.PricePrefix, scope.Currency.PriceSuffix)
	}
	assertFloatEqual(t, analysis.SuggestedPrice, 0.50)
	if analysis.LastSoldPrice < 0.49 || analysis.LastSoldPrice > 0.51 {
		t.Fatalf("last sold price = %.6f, want converted GBP price around 0.50", analysis.LastSoldPrice)
	}
	if analysis.USDFallbackMetrics == 0 {
		t.Fatalf("USD fallback fields were not marked: %+v", analysis)
	}
}

func TestPricehistoryFailureLogsMinimalDiagnosticDetails(t *testing.T) {
	originalTransport := http.DefaultTransport
	originalLimiter := steamRequestLimiter
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
		steamRequestLimiter = originalLimiter
		SetSteamRequestMetricLogger(nil)
	})

	var priceHistoryQuery string
	var metricsMu sync.Mutex
	var priceHistoryMetric SteamRequestMetric
	SetSteamRequestMetricLogger(func(metric SteamRequestMetric) {
		if metric.Endpoint != "pricehistory" {
			return
		}
		metricsMu.Lock()
		priceHistoryMetric = metric
		metricsMu.Unlock()
	})

	steamRequestLimiter = newSteamRateLimiter()
	http.DefaultTransport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		resetSteamRequestLimiterForTest()
		switch {
		case strings.Contains(request.URL.Path, "/market/listings/"):
			return testHTTPResponse(http.StatusOK, `<html></html>`), nil
		case strings.Contains(request.URL.Path, "/market/priceoverview/"):
			return testHTTPResponse(http.StatusOK, `{"success":false}`), nil
		case strings.Contains(request.URL.Path, "/market/pricehistory/"):
			priceHistoryQuery = request.URL.RawQuery
			return testHTTPResponse(http.StatusBadRequest, `{"success":false,"error":"bad request"}`), nil
		default:
			t.Fatalf("unexpected request URL: %s", request.URL.String())
			return testHTTPResponse(http.StatusNotFound, ""), nil
		}
	})

	scope, ok := marketScopeFor("GBP", "GB")
	if !ok {
		t.Fatal("expected GBP/GB scope")
	}
	config := catalog.ItemConfig{
		ID:   116002,
		Type: "MATERIAL",
		Name: map[string]string{"en-US": "Astral Diamond"},
	}
	_, err := fetchDataForScope(config, "Astral Diamond", time.Unix(1700000000, 0), scope, RequestPriorityHigh)
	if err == nil {
		t.Fatal("fetchDataForScope returned nil error")
	}
	if strings.Contains(priceHistoryQuery, "country=") || strings.Contains(priceHistoryQuery, "currency=") {
		t.Fatalf("pricehistory query contains scoped params: %s", priceHistoryQuery)
	}

	metricsMu.Lock()
	metric := priceHistoryMetric
	metricsMu.Unlock()
	if metric.StatusCode != http.StatusBadRequest || metric.AppID != "3678970" || metric.MarketHashName != "Astral Diamond" {
		t.Fatalf("pricehistory metric = %+v", metric)
	}
	if metric.URL == "" || !strings.Contains(metric.ResponseBody, "bad request") {
		t.Fatalf("pricehistory metric did not include URL/body: %+v", metric)
	}
}

func TestBaseHistoryConvertsToTargetCurrency(t *testing.T) {
	withExchangeRateCache(t, map[string]float64{"GBP": 0.5})

	scope, ok := marketScopeFor("GBP", "GB")
	if !ok {
		t.Fatal("expected GBP/GB scope")
	}
	now := time.Unix(1700000000, 0)
	history := []MarketSalePoint{
		{Time: now.Add(-time.Hour).Unix(), Price: 10, Volume: 2},
	}

	data := marketDataFromBaseHistory("Astral Diamond", MarketOrderBook{}, false, history, now, scope.Currency)
	assertFloatEqual(t, data.Analysis.LastSoldPrice, 5)
	assertFloatEqual(t, data.Analysis.WeeklyAveragePrice, 5)
	if !data.Analysis.HasSuggested || data.Analysis.PricePrefix != scope.Currency.PricePrefix {
		t.Fatalf("converted analysis = %+v", data.Analysis)
	}
}

func TestBaseHistoryWithoutExchangeRateKeepsVolumeSignalsOnly(t *testing.T) {
	withExchangeRateCache(t, map[string]float64{})

	now := time.Unix(1700000000, 0)
	history := []MarketSalePoint{
		{Time: now.Add(-6 * 24 * time.Hour).Unix(), Price: 10, Volume: 1},
		{Time: now.Add(-time.Hour).Unix(), Price: 20, Volume: 2},
	}

	data := marketDataFromBaseHistory("Astral Diamond", MarketOrderBook{}, false, history, now, MarketCurrency{Code: "ZZZ", PricePrefix: "Z$"})
	if data.Analysis.HasSuggested || data.Analysis.HasWeeklyAverage || data.Analysis.HasLastSold || data.Analysis.HasRecentSaleP75 {
		t.Fatalf("history price signals leaked into analysis without exchange rate: %+v", data.Analysis)
	}
	if !data.Analysis.HasSaleHistory || !data.Analysis.HasDailySales || !data.Analysis.HasTrend {
		t.Fatalf("history volume/trend signals were not preserved: %+v", data.Analysis)
	}
}

func testHTTPResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Status:     fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func resetSteamRequestLimiterForTest() {
	steamRequestLimiter.mu.Lock()
	steamRequestLimiter.lastStartedAt = time.Time{}
	steamRequestLimiter.waitingHigh = 0
	steamRequestLimiter.cond.Broadcast()
	steamRequestLimiter.mu.Unlock()
}

func withExchangeRateCache(t *testing.T, rates map[string]float64) {
	t.Helper()
	exchangeRateMu.Lock()
	original := make(map[string]float64, len(exchangeRateCache))
	for code, rate := range exchangeRateCache {
		original[code] = rate
	}
	exchangeRateCache = make(map[string]float64, len(rates))
	for code, rate := range rates {
		exchangeRateCache[code] = rate
	}
	exchangeRateMu.Unlock()

	t.Cleanup(func() {
		exchangeRateMu.Lock()
		exchangeRateCache = original
		exchangeRateMu.Unlock()
	})
}
