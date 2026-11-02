package market

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/catalog"
)

func withRUBRate(t *testing.T, rate float64, date time.Time) {
	t.Helper()
	withExchangeRateCache(t, map[string]float64{"RUB": rate})
	originalDate, originalAttempt := rubExchangeRateDate, rubExchangeRateLastAttempt
	originalFailed := rubExchangeRateRefreshFailed
	rubExchangeRateDate = date
	rubExchangeRateLastAttempt = time.Now()
	t.Cleanup(func() {
		rubExchangeRateDate, rubExchangeRateLastAttempt = originalDate, originalAttempt
		rubExchangeRateRefreshFailed = originalFailed
	})
}

func TestRUBRegistrationSelectionAndCacheIsolation(t *testing.T) {
	original := currentMarketScope()
	t.Cleanup(func() { setMarketScope(original.Currency.Code, original.Region.CountryCode) })
	scope, changed, ok := selectMarketCurrency("RUB")
	if !ok || scope.Currency.SteamCurrencyID != 5 || scope.Region.CountryCode != "RU" || scope.Region.Name != "Russia" || !changed {
		t.Fatalf("RUB selection = %+v, changed=%t, ok=%t", scope, changed, ok)
	}
	if got := marketCurrencyMenuLabel(scope.Currency, scope); got != "RUB — Russia" {
		t.Fatalf("RUB menu = %q", got)
	}
	name := "Astral Diamond"
	key := marketCacheKey(scope, name)
	parsed, hash, ok := parseMarketCacheKey(key)
	if !ok || parsed != scope || hash != name || key == marketCacheKey(defaultMarketScope(), name) {
		t.Fatalf("RUB cache key did not round-trip independently: %q", key)
	}
	config := resolvedTestItem(catalog.ItemConfig{ID: 116002, Type: "MATERIAL", Name: map[string]string{"en-US": name}})
	for _, raw := range []string{ListingURLForScope(config, scope), ItemOrdersHistogramURL("123", scope)} {
		parsedURL, _ := url.Parse(raw)
		if parsedURL.Query().Get("currency") != "5" || parsedURL.Query().Get("country") != "RU" {
			t.Fatalf("incorrect RUB request URL: %s", raw)
		}
	}
	if got := marketScopeFromSettings("RUB", "RU"); got != scope {
		t.Fatalf("RUB settings scope = %+v", got)
	}
}

func TestRUBRateResponseValidation(t *testing.T) {
	now := time.Now().UTC()
	today := now.Format("2006-01-02")
	for _, test := range []struct {
		name, body string
		status     int
		valid      bool
	}{
		{"current", fmt.Sprintf(`[{"date":%q,"base":"USD","quote":"RUB","rate":80.68}]`, today), 200, true},
		{"weekend", fmt.Sprintf(`[{"date":%q,"base":"USD","quote":"RUB","rate":80}]`, now.Add(-3*24*time.Hour).Format("2006-01-02")), 200, true},
		{"stale", `[{"date":"2020-01-01","base":"USD","quote":"RUB","rate":80}]`, 200, false},
		{"future", `[{"date":"2099-01-01","base":"USD","quote":"RUB","rate":80}]`, 200, false},
		{"zero", fmt.Sprintf(`[{"date":%q,"base":"USD","quote":"RUB","rate":0}]`, today), 200, false},
		{"negative", fmt.Sprintf(`[{"date":%q,"base":"USD","quote":"RUB","rate":-80}]`, today), 200, false},
		{"unreasonable", fmt.Sprintf(`[{"date":%q,"base":"USD","quote":"RUB","rate":1000000}]`, today), 200, false},
		{"parity", fmt.Sprintf(`[{"date":%q,"base":"USD","quote":"RUB","rate":1}]`, today), 200, false},
		{"wrong-pair", fmt.Sprintf(`[{"date":%q,"base":"EUR","quote":"RUB","rate":80}]`, today), 200, false},
		{"missing-date", `[{"base":"USD","quote":"RUB","rate":80}]`, 200, false},
		{"missing", `[]`, 200, false},
		{"malformed", `{`, 200, false},
		{"invalid-number", fmt.Sprintf(`[{"date":%q,"base":"USD","quote":"RUB","rate":1e999}]`, today), 200, false},
		{"unavailable", `{}`, 503, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			withRUBRate(t, 0, time.Time{})
			client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if request.URL.String() != "https://api.frankfurter.dev/v2/rates?base=USD&quotes=RUB" {
					t.Fatalf("unexpected rate URL: %s", request.URL)
				}
				return testHTTPResponse(test.status, test.body), nil
			})}
			err := fetchRUBExchangeRate(client, now)
			if (err == nil) != test.valid {
				t.Fatalf("fetchRUBExchangeRate error = %v, valid=%t", err, test.valid)
			}
			rate, ok := configuredExchangeRate("RUB")
			if ok != test.valid || (!test.valid && getExchangeRate("RUB") != 0) || (test.valid && rate <= 1) {
				t.Fatalf("cached RUB rate = %v, available=%t", rate, ok)
			}
		})
	}
}

func TestRUBRateRefreshRetainsFreshCacheAndThrottlesFailures(t *testing.T) {
	withRUBRate(t, 80, time.Now().Add(-24*time.Hour))
	originalClient := exchangeRateHTTPClient
	t.Cleanup(func() { exchangeRateHTTPClient = originalClient })
	requests := 0
	exchangeRateHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		return nil, fmt.Errorf("offline")
	})}
	rubExchangeRateLastAttempt = time.Time{}
	refreshRUBExchangeRate()
	refreshRUBExchangeRate()
	if requests != 1 || getExchangeRate("RUB") != 80 {
		t.Fatalf("failed refresh lost fresh cache or retried: requests=%d, rate=%v", requests, getExchangeRate("RUB"))
	}
	rubExchangeRateLastAttempt = time.Now().Add(-6 * time.Minute)
	refreshRUBExchangeRate()
	if requests != 2 {
		t.Fatal("failed refresh did not retry after five minutes")
	}
	rubExchangeRateDate = time.Now().Add(-8 * 24 * time.Hour)
	if rate, ok := configuredExchangeRate("RUB"); ok || rate != 0 || getExchangeRate("RUB") != 0 {
		t.Fatal("expired RUB rate remained available")
	}
	for _, rate := range []float64{math.NaN(), math.Inf(1), -1, 0} {
		setExchangeRate("EUR", rate)
		if value, _ := configuredExchangeRate("EUR"); math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
			t.Fatal("invalid rate replaced an existing currency fallback")
		}
	}
}

func TestRUBHistoryUsesItsDeclaredCurrency(t *testing.T) {
	now := time.Now()
	withRUBRate(t, 80, now)
	scope, _ := marketScopeFor("RUB", "RU")
	for _, test := range []struct {
		currency  int
		want      float64
		estimated bool
	}{
		{5, 1.5, false},
		{1, 120, true},
		{3, 0, false},
		{999, 0, false},
	} {
		t.Run(fmt.Sprint(test.currency), func(t *testing.T) {
			body := fmt.Sprintf(`{"queries":[{"queryKey":["market","orderbook",3678970,"Astral Diamond"],"state":{"data":{"amtMaxBuyOrder":10000,"amtMinSellOrder":20000,"eCurrency":5,"cBuyOrders":1,"cSellOrders":1}}},{"queryKey":["market","pricehistory",3678970,"Astral Diamond"],"state":{"data":{"ecurrency":%d,"prices":[{"time":%d,"price_median":1.5,"purchases":3}]}}}]}`, test.currency, now.Unix())
			data, err := marketDataFromListingSSR("Astral Diamond", []byte(body), "", now, scope)
			if err != nil {
				t.Fatal(err)
			}
			assertFloatEqual(t, data.Analysis.WeeklyAveragePrice, test.want)
			assertFloatEqual(t, data.Analysis.LowestSellPrice, 200)
			if data.Analysis.HasWeeklyAverage != (test.want > 0) || data.Analysis.PriceIsEstimated != test.estimated {
				t.Fatalf("history currency %d was misinterpreted: %+v", test.currency, data.Analysis)
			}
		})
	}
	foreignBody := loaderDataListingFixture(3, 100, 200, now.Unix(), true)
	if _, err := marketDataFromListingSSR("Astral Diamond", []byte(foreignBody), "", now, scope); err == nil {
		t.Fatal("EUR listing was accepted as RUB")
	}
}

func TestRUBListingAndUSDFallback(t *testing.T) {
	now := time.Now()
	scope, _ := marketScopeFor("RUB", "RU")
	for _, test := range []struct {
		name                 string
		currency             int
		rate                 float64
		date                 time.Time
		history              bool
		wantPrice, estimated bool
	}{
		{"direct-no-rate", 5, 0, time.Time{}, true, true, false},
		{"direct-with-rate", 5, 80, now, true, true, false},
		{"direct-orderbook-only", 5, 0, time.Time{}, false, true, false},
		{"converted-usd", 1, 80, now, true, true, true},
		{"usd-without-rate", 1, 0, time.Time{}, true, false, false},
		{"usd-with-stale-rate", 1, 80, now.Add(-8 * 24 * time.Hour), true, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			withRUBRate(t, test.rate, test.date)
			originalTransport, originalLimiter := steamHTTPTransport, steamRequestLimiter
			t.Cleanup(func() { steamHTTPTransport, steamRequestLimiter = originalTransport, originalLimiter })
			steamRequestLimiter = newSteamRateLimiter()
			steamHTTPTransport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
				resetSteamRequestLimiterForTest()
				if request.URL.Query().Get("currency") != "5" || request.URL.Query().Get("country") != "RU" {
					t.Fatalf("unexpected RUB request: %s", request.URL)
				}
				return testHTTPResponse(200, loaderDataListingFixture(test.currency, 100, 200, now.Unix(), test.history)), nil
			})
			config := resolvedTestItem(catalog.ItemConfig{ID: 116002, Type: "MATERIAL", Name: map[string]string{"en-US": "Astral Diamond"}})
			var data MarketData
			var err error
			if test.history {
				data, err = FetchDataWithPriority(config, config.MarketHashName, now, scope, RequestPriorityHigh)
			} else {
				data, err = fetchDataForScope(config, config.MarketHashName, now, scope, RequestPriorityHigh)
			}
			if err != nil {
				t.Fatal(err)
			}
			analysis := data.Analysis
			if analysis.HasSuggested != test.wantPrice || analysis.PriceIsEstimated != test.estimated || analysis.PriceSuffix != " ₽" {
				t.Fatalf("RUB analysis = %+v", analysis)
			}
			if test.wantPrice {
				wantBuy, wantHistory := 1.0, 1.0
				if test.estimated {
					wantBuy, wantHistory = 80, 80
				}
				assertFloatEqual(t, analysis.HighestBuyPrice, wantBuy)
				if test.history {
					assertFloatEqual(t, analysis.WeeklyAveragePrice, wantHistory)
				}
			} else if analysis.HasHighestBuy || analysis.HasLowestSell || analysis.HasSteamMedian || analysis.HasWeeklyAverage || len(data.History) != 0 {
				t.Fatalf("USD values leaked into unavailable RUB prices: %+v", data)
			}
		})
	}
}

func TestRUBPartialFallbackPreservesDirectPrices(t *testing.T) {
	withRUBRate(t, 80, time.Now())
	now := time.Now()
	scope, _ := marketScopeFor("RUB", "RU")
	local := marketDataFromSources("Astral Diamond", MarketOrderBook{LowestSellPrice: 200, HighestBuyPrice: 100}, true, nil, now, scope.Currency)
	usd := marketDataFromSources("Astral Diamond", MarketOrderBook{LowestSellPrice: 2, HighestBuyPrice: 1}, true, []MarketSalePoint{{Time: now.Unix(), Price: 1.5, Volume: 3}}, now, defaultMarketScope().Currency)
	merged := mergeMarketDataWithUSDFallback(local, usd, scope)
	assertFloatEqual(t, merged.Analysis.LowestSellPrice, 200)
	assertFloatEqual(t, merged.Analysis.HighestBuyPrice, 100)
	assertFloatEqual(t, merged.Analysis.WeeklyAveragePrice, 120)
	if !merged.Analysis.PriceIsEstimated || getExchangeRate("RUB") != 80 {
		t.Fatal("partial fallback overwrote the official rate with a listing ratio")
	}
	merged.Analysis.ExchangeRateDate = now.Add(-8 * 24 * time.Hour)
	if _, ok := staleMarketAnalysis(merged, true); ok || isFreshMarketCache(merged, now) {
		t.Fatal("converted RUB cache outlived its exchange rate")
	}
	local.Analysis.UpdatedAt = now.Add(-8 * 24 * time.Hour)
	if _, ok := staleMarketAnalysis(local, true); !ok {
		t.Fatal("direct Steam RUB cache unexpectedly depends on an exchange rate")
	}
}

func TestRUBFormattingAndPriceIncrements(t *testing.T) {
	withRUBRate(t, 80.68, time.Now())
	originalLanguage := displayLanguage
	t.Cleanup(func() { displayLanguage = originalLanguage })
	displayLanguage = func() string { return "ru-RU" }
	for price, want := range map[float64]string{125.5: "125,50 ₽", 1250: "1 250,00 ₽", 12345.67: "12 345,67 ₽"} {
		if got := formatAnalysisPrice(price, true, MarketAnalysis{PriceSuffix: " ₽"}); got != want {
			t.Errorf("RUB %v = %q, want %q", price, got, want)
		}
	}
	for _, language := range []string{"en-US", "tr-TR"} {
		displayLanguage = func() string { return language }
		if got := formatAnalysisPrice(125.5, true, MarketAnalysis{PriceSuffix: " ₽"}); got != "125.50 ₽" {
			t.Errorf("%s RUB = %q", language, got)
		}
	}
	displayLanguage = func() string { return "ru-RU" }
	if got := formatAnalysisPrice(125.5, true, MarketAnalysis{PricePrefix: "$"}); got != "$125.50" {
		t.Errorf("Russian USD formatting changed: %q", got)
	}
	rub, _ := marketCurrencyForCode("RUB")
	assertFloatEqual(t, rub.PriceIncrement, .01)
	assertFloatEqual(t, marketMinListingPrice(rub), 2.43)
	assertFloatEqual(t, roundToMarketIncrement(125.5051, rub.PriceIncrement), 125.51)
	if strings.Contains(formatAnalysisPrice(0, false, MarketAnalysis{PriceSuffix: " ₽"}), "₽") {
		t.Fatal("unavailable price displayed as a numeric RUB amount")
	}
	for _, invalid := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if got := formatAnalysisPrice(invalid, true, MarketAnalysis{PriceSuffix: " ₽"}); strings.Contains(got, "₽") {
			t.Fatalf("invalid RUB price was displayed: %q", got)
		}
	}
}
