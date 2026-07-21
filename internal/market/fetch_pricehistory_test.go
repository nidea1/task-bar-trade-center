package market

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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

func TestLoaderDataListingSkipsLegacyMarketJSONEndpoints(t *testing.T) {
	originalTransport := steamHTTPTransport
	originalLimiter := steamRequestLimiter
	t.Cleanup(func() {
		steamHTTPTransport = originalTransport
		steamRequestLimiter = originalLimiter
	})

	steamRequestLimiter = newSteamRateLimiter()
	listingRequests := 0
	steamHTTPTransport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		resetSteamRequestLimiterForTest()
		if encoding := request.Header.Get("Accept-Encoding"); encoding != "" {
			t.Fatalf("Accept-Encoding = %q, want omitted header", encoding)
		}
		if !strings.Contains(request.URL.Path, "/market/listings/") {
			t.Fatalf("unexpected request URL: %s", request.URL.String())
			return testHTTPResponse(http.StatusNotFound, ""), nil
		}
		listingRequests++
		return testHTTPResponse(http.StatusOK, loaderDataListingFixture(1, 25, 27, 1_700_000_000, true)), nil
	})

	scope, ok := marketScopeFor("USD", "US")
	if !ok {
		t.Fatal("expected USD/US scope")
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
	if !data.Analysis.HasOrderBook || !data.Analysis.HasSaleHistory || !data.Analysis.HasSteamMedian || !data.Analysis.HasSteamVolume {
		t.Fatalf("loaderData metrics were not populated: %+v", data.Analysis)
	}
	if data.Analysis.PriceSource != "listing-ssr" {
		t.Fatalf("price source = %q, want listing-ssr", data.Analysis.PriceSource)
	}
	if listingRequests != 1 {
		t.Fatalf("listing requests = %d, want 1", listingRequests)
	}
}

func TestScopedListingOrderBookSkipsPricehistory(t *testing.T) {
	originalTransport := steamHTTPTransport
	originalLimiter := steamRequestLimiter
	t.Cleanup(func() {
		steamHTTPTransport = originalTransport
		steamRequestLimiter = originalLimiter
		SetSteamRequestMetricLogger(nil)
	})

	scope, ok := marketScopeFor("GBP", "GB")
	if !ok {
		t.Fatal("expected GBP/GB scope")
	}

	steamRequestLimiter = newSteamRateLimiter()
	steamHTTPTransport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
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
	assertFloatEqual(t, data.Analysis.SuggestedPrice, 0.03)
}

func TestScopedSuggestedPriceMergesUSDFallbackDetails(t *testing.T) {
	originalTransport := steamHTTPTransport
	originalLimiter := steamRequestLimiter
	t.Cleanup(func() {
		steamHTTPTransport = originalTransport
		steamRequestLimiter = originalLimiter
	})

	scope, ok := marketScopeFor("GBP", "GB")
	if !ok {
		t.Fatal("expected GBP/GB scope")
	}
	usdScope := defaultMarketScope()

	steamRequestLimiter = newSteamRateLimiter()
	listingRequests := 0
	steamHTTPTransport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		resetSteamRequestLimiterForTest()
		if !strings.Contains(request.URL.Path, "/market/listings/") {
			t.Fatalf("unexpected request URL: %s", request.URL.String())
			return testHTTPResponse(http.StatusNotFound, ""), nil
		}
		listingRequests++
		if request.URL.Query().Get("currency") == fmt.Sprint(scope.Currency.SteamCurrencyID) {
			return testHTTPResponse(http.StatusOK, loaderDataListingFixture(scope.Currency.SteamCurrencyID, 40, 51, 0, false)), nil
		}
		if request.URL.Query().Get("currency") == fmt.Sprint(usdScope.Currency.SteamCurrencyID) {
			return testHTTPResponse(http.StatusOK, loaderDataListingFixture(usdScope.Currency.SteamCurrencyID, 80, 101, 1_700_000_000, true)), nil
		}
		t.Fatalf("unexpected listing scope: %s", request.URL.String())
		return testHTTPResponse(http.StatusNotFound, ""), nil
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
	if listingRequests != 2 {
		t.Fatalf("listing requests = %d, want 2 scoped listing requests", listingRequests)
	}
}

func TestListingWithoutMarketDataUsesOnlyHTMLSearchFallback(t *testing.T) {
	originalTransport := steamHTTPTransport
	originalLimiter := steamRequestLimiter
	t.Cleanup(func() {
		steamHTTPTransport = originalTransport
		steamRequestLimiter = originalLimiter
	})

	steamRequestLimiter = newSteamRateLimiter()
	listingRequests := 0
	searchPageRequests := 0
	steamHTTPTransport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		resetSteamRequestLimiterForTest()
		switch {
		case strings.Contains(request.URL.Path, "/market/listings/"):
			listingRequests++
			return testHTTPResponse(http.StatusOK, `<html></html>`), nil
		case request.URL.Path == "/market/search":
			searchPageRequests++
			return testHTTPResponse(http.StatusOK, `<html></html>`), nil
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
	if listingRequests != 1 || searchPageRequests != 1 {
		t.Fatalf("listing/search page requests = %d/%d, want 1/1", listingRequests, searchPageRequests)
	}
	if strings.Contains(err.Error(), "priceoverview") || strings.Contains(err.Error(), "search/render") {
		t.Fatalf("legacy endpoint leaked into error: %v", err)
	}
}

func TestListingShellFallsBackToSearchHTMLPrice(t *testing.T) {
	originalTransport := steamHTTPTransport
	originalLimiter := steamRequestLimiter
	t.Cleanup(func() {
		steamHTTPTransport = originalTransport
		steamRequestLimiter = originalLimiter
	})

	steamRequestLimiter = newSteamRateLimiter()
	steamHTTPTransport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		resetSteamRequestLimiterForTest()
		switch {
		case strings.Contains(request.URL.Path, "/market/listings/"):
			return testHTTPResponse(http.StatusOK, `<html></html>`), nil
		case request.URL.Path == "/market/search":
			if query := request.URL.Query(); query.Get("q") != "Astral Diamond" || query.Get("appid") != "3678970" || query.Get("country") != "US" || query.Get("currency") != "1" {
				t.Fatalf("search page query is not deterministic: %s", request.URL.RawQuery)
			}
			return testHTTPResponse(http.StatusOK, loaderDataSearchFixture("Astral Diamond", SteamAppID, 1, "$0.12", 42)), nil
		default:
			t.Fatalf("unexpected request URL: %s", request.URL.String())
			return testHTTPResponse(http.StatusNotFound, ""), nil
		}
	})

	config := catalog.ItemConfig{ID: 116002, Type: "MATERIAL", Name: map[string]string{"en-US": "Astral Diamond"}}
	data, err := fetchDataForScope(config, "Astral Diamond", time.Unix(1_700_000_000, 0), defaultMarketScope(), RequestPriorityHigh)
	if err != nil {
		t.Fatalf("fetchDataForScope returned error: %v", err)
	}
	analysis := data.Analysis
	if !analysis.HasLowestSell || !analysis.HasSuggested || analysis.PriceSource != "market-search-html" {
		t.Fatalf("HTML search fallback analysis = %+v", analysis)
	}
	if analysis.HasOrderBook || analysis.HasSaleHistory || analysis.HasSteamMedian || analysis.HasSteamVolume {
		t.Fatalf("HTML search fallback invented unavailable metrics: %+v", analysis)
	}
	assertFloatEqual(t, analysis.LowestSellPrice, 0.12)
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

func loaderDataListingFixture(currencyID int, highestBuyCents int, lowestSellCents int, historyTime int64, includeHistory bool) string {
	queries := fmt.Sprintf(
		`{"state":{"data":{"amtMaxBuyOrder":%d,"amtMinSellOrder":%d,"eCurrency":%d,"cBuyOrders":12,"cSellOrders":3,"rgCompactBuyOrders":[%d,10],"rgCompactSellOrders":[%d,8]}},"queryKey":["market","orderbook",%d,"Astral Diamond"]}`,
		highestBuyCents,
		lowestSellCents,
		currencyID,
		highestBuyCents,
		lowestSellCents,
		SteamAppID,
	)
	if includeHistory {
		queries += fmt.Sprintf(
			`,{"state":{"data":{"ecurrency":%d,"prices":[{"time":%d,"price_median":1.0,"purchases":3}]}},"queryKey":["market","pricehistory",%d,"Astral Diamond"]}`,
			currencyID,
			historyTime,
			SteamAppID,
		)
	}
	dehydratedState := `{"queries":[` + queries + `]}`
	loaderEntry, _ := json.Marshal(map[string]any{"dehydratedState": dehydratedState})
	loaderData, _ := json.Marshal([]string{string(loaderEntry)})
	return `<script>window.SSR={};window.SSR.loaderData = ` + string(loaderData) + `;</script>`
}

func loaderDataSearchFixture(marketHashName string, appID int, currencyID int, formattedPrice string, sellOrders int) string {
	dehydratedState := fmt.Sprintf(
		`{"queries":[{"state":{"data":{"pages":[{"start":0,"total_count":1,"results":[{"strHash":%q,"cSellOrders":%d,"cBuyOrders":0,"eCurrency":%d,"strMinSellSubtotal":%q,"asset_description":{"appid":%d}}]}]}}}]}`,
		marketHashName,
		sellOrders,
		currencyID,
		formattedPrice,
		appID,
	)
	loaderEntry, _ := json.Marshal(map[string]any{"searchState": dehydratedState})
	loaderData, _ := json.Marshal([]string{string(loaderEntry)})
	return `<script>window.SSR={};window.SSR.loaderData = ` + string(loaderData) + `;</script>`
}
