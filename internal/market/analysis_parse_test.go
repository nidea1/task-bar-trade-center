package market

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestParseItemOrdersHistogramResponse(t *testing.T) {
	body := []byte(`{
		"response": {
			"highest_buy_order": 2.55,
			"lowest_sell_order": 2.57,
			"buy_order_summary": 51893,
			"sell_order_summary": 2763,
			"buy_order_graph": [{"price": 2.55, "volume": 5}],
			"sell_order_graph": [{"price": 2.57, "volume": 1}],
			"price_prefix": "$",
			"price_suffix": ""
		}
	}`)

	orderBook, ok := parseItemOrdersHistogramResponse(body)
	if !ok {
		t.Fatal("expected histogram response to parse")
	}
	assertFloatEqual(t, orderBook.HighestBuyPrice, 2.55)
	assertFloatEqual(t, orderBook.LowestSellPrice, 2.57)
	if orderBook.HighestBuyQuantity != 5 {
		t.Fatalf("highest buy quantity = %d, want 5", orderBook.HighestBuyQuantity)
	}
	if orderBook.LowestSellQuantity != 1 {
		t.Fatalf("lowest sell quantity = %d, want 1", orderBook.LowestSellQuantity)
	}
	if orderBook.BuyOrderCount != 51893 {
		t.Fatalf("buy orders = %d, want 51893", orderBook.BuyOrderCount)
	}
	if orderBook.SellOrderCount != 2763 {
		t.Fatalf("sell orders = %d, want 2763", orderBook.SellOrderCount)
	}
}

func TestParseSSRMarketData(t *testing.T) {
	body := []byte(`<script>JSON.parse("{\"state\":{\"data\":{\"amtMaxBuyOrder\":255,\"amtMinSellOrder\":257,\"eCurrency\":1,\"cBuyOrders\":12,\"cSellOrders\":3,\"rgCompactBuyOrders\":[255,10],\"rgCompactSellOrders\":[257,8]},\"queryKey\":[\"market\",\"orderbook\",3678970,\"Example\"]}}")</script>
<script>JSON.parse("{\"state\":{\"data\":{\"history\":[{\"time\":1700000000,\"price_median\":2.51,\"purchases\":4}]},\"queryKey\":[\"market\",\"pricehistory\",3678970,\"Example\"]}}")</script>`)

	usd, ok := marketScopeFor("USD", "TR")
	if !ok {
		t.Fatal("expected USD/TR scope to exist")
	}
	orderBook, ok := parseSSRItemOrderBook(body, usd.Currency)
	if !ok {
		t.Fatal("expected SSR order book to parse")
	}
	assertFloatEqual(t, orderBook.HighestBuyPrice, 2.55)
	assertFloatEqual(t, orderBook.LowestSellPrice, 2.57)
	if orderBook.HighestBuyQuantity != 10 {
		t.Fatalf("highest buy quantity = %d, want 10", orderBook.HighestBuyQuantity)
	}
	if orderBook.LowestSellQuantity != 8 {
		t.Fatalf("lowest sell quantity = %d, want 8", orderBook.LowestSellQuantity)
	}

	history := parseSSRPriceHistory(body)
	if len(history) != 1 {
		t.Fatalf("history length = %d, want 1", len(history))
	}
	assertFloatEqual(t, history[0].Price, 2.51)
	if history[0].Volume != 4 {
		t.Fatalf("history volume = %d, want 4", history[0].Volume)
	}

	if !isSSRListingForScope(body, usd) {
		t.Fatal("expected USD SSR data to apply to the USD/TR scope")
	}
	eur, ok := marketScopeFor("EUR", "DE")
	if !ok || isSSRListingForScope(body, eur) {
		t.Fatal("USD SSR data must not apply to the EUR/DE scope")
	}
}

func TestParseSSRMarketDataFromLoaderData(t *testing.T) {
	dehydratedState := map[string]any{
		"queries": []any{
			map[string]any{
				"state": map[string]any{"data": map[string]any{
					"amtMaxBuyOrder":      255,
					"amtMinSellOrder":     257,
					"eCurrency":           1,
					"cBuyOrders":          12,
					"cSellOrders":         3,
					"rgCompactBuyOrders":  []int{255, 10},
					"rgCompactSellOrders": []int{257, 8},
				}},
				"queryKey": []any{"market", "pricehistory", SteamAppID, "Example"},
			},
			map[string]any{
				"state": map[string]any{"data": map[string]any{
					"ecurrency": 1,
					"prices": []any{
						struct {
							Time        int64   `json:"time"`
							PriceMedian float64 `json:"price_median"`
							Purchases   int     `json:"purchases"`
						}{Time: 1700000000, PriceMedian: 2.51, Purchases: 4},
					},
				}},
				"queryKey": []any{"market", "pricehistory", SteamAppID, "Example"},
			},
		},
	}
	dehydratedJSON, err := json.Marshal(dehydratedState)
	if err != nil {
		t.Fatalf("marshal dehydrated state: %v", err)
	}
	loaderEntry, err := json.Marshal(map[string]any{"dehydratedState": string(dehydratedJSON)})
	if err != nil {
		t.Fatalf("marshal loader entry: %v", err)
	}
	loaderData, err := json.Marshal([]string{string(loaderEntry)})
	if err != nil {
		t.Fatalf("marshal loader data: %v", err)
	}
	body := []byte(`<script>window.SSR={};window.SSR.loaderData = ` + string(loaderData) + `;</script>`)

	usd, ok := marketScopeFor("USD", "US")
	if !ok {
		t.Fatal("expected USD/US scope to exist")
	}
	if !isSSRListingForScope(body, usd) {
		t.Fatal("loaderData currency was not detected")
	}
	orderBook, ok := parseSSRItemOrderBook(body, usd.Currency)
	if !ok {
		t.Fatal("loaderData order book was not parsed")
	}
	assertFloatEqual(t, orderBook.HighestBuyPrice, 2.55)
	assertFloatEqual(t, orderBook.LowestSellPrice, 2.57)
	if orderBook.HighestBuyQuantity != 10 || orderBook.LowestSellQuantity != 8 {
		t.Fatalf("loaderData top quantities = %d/%d, want 10/8", orderBook.HighestBuyQuantity, orderBook.LowestSellQuantity)
	}
	history := parseSSRPriceHistory(body)
	if len(history) != 1 || history[0].Volume != 4 {
		t.Fatalf("loaderData history = %+v, want one point with volume 4", history)
	}
	assertFloatEqual(t, history[0].Price, 2.51)
}

func TestParseSSRMarketDataWithNullPrices(t *testing.T) {
	body := []byte(`<script>JSON.parse("{\"state\":{\"data\":{\"amtMaxBuyOrder\":230,\"amtMinSellOrder\":null,\"eCurrency\":1,\"cBuyOrders\":2311,\"cSellOrders\":0,\"rgCompactBuyOrders\":[230,20]},\"queryKey\":[\"market\",\"orderbook\",3678970,\"Example\"]}}")</script>`)

	usd, ok := marketScopeFor("USD", "TR")
	if !ok {
		t.Fatal("expected USD/TR scope to exist")
	}
	orderBook, ok := parseSSRItemOrderBook(body, usd.Currency)
	if !ok {
		t.Fatal("expected SSR order book to parse even with null min sell order price")
	}
	assertFloatEqual(t, orderBook.HighestBuyPrice, 2.30)
	assertFloatEqual(t, orderBook.LowestSellPrice, 0.0)
	if orderBook.BuyOrderCount != 2311 {
		t.Fatalf("buy orders = %d, want 2311", orderBook.BuyOrderCount)
	}
	if orderBook.SellOrderCount != 0 {
		t.Fatalf("sell orders = %d, want 0", orderBook.SellOrderCount)
	}
	if orderBook.HighestBuyQuantity != 20 {
		t.Fatalf("highest buy quantity = %d, want 20", orderBook.HighestBuyQuantity)
	}
	if orderBook.LowestSellQuantity != 0 {
		t.Fatalf("lowest sell quantity = %d, want 0", orderBook.LowestSellQuantity)
	}
}

func TestSSRMarketDataUsesSelectedCurrencyFormat(t *testing.T) {
	now := time.Unix(1700000000, 0)
	tests := []struct {
		currency string
		country  string
	}{
		{currency: "PHP", country: "PH"},
		{currency: "EUR", country: "DE"},
		{currency: "CAD", country: "CA"},
	}

	for _, tt := range tests {
		t.Run(tt.currency+"/"+tt.country, func(t *testing.T) {
			scope, ok := marketScopeFor(tt.currency, tt.country)
			if !ok {
				t.Fatalf("expected %s/%s scope to exist", tt.currency, tt.country)
			}
			body := []byte(fmt.Sprintf(`<script>JSON.parse("{\"state\":{\"data\":{\"amtMaxBuyOrder\":392,\"amtMinSellOrder\":445,\"eCurrency\":%d,\"cBuyOrders\":16,\"cSellOrders\":121,\"rgCompactBuyOrders\":[392,3],\"rgCompactSellOrders\":[445,1]},\"queryKey\":[\"market\",\"orderbook\",3678970,\"Fate Helmet (Legendary) A\"]}}")</script>`, scope.Currency.SteamCurrencyID))
			if !isSSRListingForScope(body, scope) {
				t.Fatalf("expected SSR data to apply to the %s/%s scope", tt.currency, tt.country)
			}

			orderBook, ok := parseSSRItemOrderBook(body, scope.Currency)
			if !ok {
				t.Fatal("expected SSR order book to parse")
			}
			data := marketDataFromSources("Fate Helmet (Legendary) A", orderBook, true, nil, now, scope.Currency)

			if data.OrderBook.PricePrefix != scope.Currency.PricePrefix || data.OrderBook.PriceSuffix != scope.Currency.PriceSuffix {
				t.Fatalf("order book format = %q/%q, want %q/%q", data.OrderBook.PricePrefix, data.OrderBook.PriceSuffix, scope.Currency.PricePrefix, scope.Currency.PriceSuffix)
			}
			if data.Analysis.PricePrefix != scope.Currency.PricePrefix || data.Analysis.PriceSuffix != scope.Currency.PriceSuffix {
				t.Fatalf("analysis format = %q/%q, want %q/%q", data.Analysis.PricePrefix, data.Analysis.PriceSuffix, scope.Currency.PricePrefix, scope.Currency.PriceSuffix)
			}
		})
	}
}

func TestUSDFallbackNormalizesTargetCurrencyFormat(t *testing.T) {
	now := time.Unix(1700000000, 0)
	eur, ok := marketScopeFor("EUR", "DE")
	if !ok {
		t.Fatal("expected EUR/DE scope to exist")
	}
	local := MarketData{
		Analysis: MarketAnalysis{
			MarketHashName: "Example",
			PricePrefix:    "$",
			PriceSuffix:    eur.Currency.PriceSuffix,
			UpdatedAt:      now,
		},
	}
	usd := MarketData{
		Analysis: MarketAnalysis{
			MarketHashName:     "Example",
			PricePrefix:        "$",
			UpdatedAt:          now,
			HighestBuyPrice:    2,
			LowestSellPrice:    3,
			SuggestedPrice:     2.99,
			HasHighestBuy:      true,
			HasLowestSell:      true,
			HasSuggested:       true,
			HasOrderBook:       true,
			HighestBuyQuantity: 4,
			LowestSellQuantity: 5,
		},
		OrderBook: MarketOrderBook{
			HighestBuyPrice: 2,
			LowestSellPrice: 3,
			PricePrefix:     "$",
		},
		OrderCachedAt: now,
	}

	merged := mergeMarketDataWithUSDFallback(local, usd, eur)

	if merged.Analysis.PricePrefix != eur.Currency.PricePrefix || merged.Analysis.PriceSuffix != eur.Currency.PriceSuffix {
		t.Fatalf("analysis format = %q/%q, want %q/%q", merged.Analysis.PricePrefix, merged.Analysis.PriceSuffix, eur.Currency.PricePrefix, eur.Currency.PriceSuffix)
	}
	if merged.OrderBook.PricePrefix != eur.Currency.PricePrefix || merged.OrderBook.PriceSuffix != eur.Currency.PriceSuffix {
		t.Fatalf("order book format = %q/%q, want %q/%q", merged.OrderBook.PricePrefix, merged.OrderBook.PriceSuffix, eur.Currency.PricePrefix, eur.Currency.PriceSuffix)
	}
	if merged.Analysis.PricePrefix == "$" && merged.Analysis.PriceSuffix != "" {
		t.Fatalf("merged format kept mixed currency symbols: %q/%q", merged.Analysis.PricePrefix, merged.Analysis.PriceSuffix)
	}
}

func TestSaleHistoryAnalysis(t *testing.T) {
	now := time.Unix(1700000000, 0)
	body := []byte(fmt.Sprintf(`{
		"response": [
			{"time": %d, "price": 30.0, "volume": 4},
			{"time": %d, "price": 20.0, "volume": 3},
			{"time": %d, "price": 10.0, "volume": 2}
		]
	}`, now.Add(-8*24*time.Hour).Unix(), now.Add(-48*time.Hour).Unix(), now.Add(-1*time.Hour).Unix()))

	history := parseSaleHistoryResponse(body)
	if len(history) != 3 {
		t.Fatalf("history length = %d, want 3", len(history))
	}

	analysis := buildMarketAnalysis("Example", MarketOrderBook{}, false, history, now, MarketCurrency{Code: "USD", PricePrefix: "$"})
	if analysis.DailySalesVolume != 2 {
		t.Fatalf("daily sales = %d, want 2", analysis.DailySalesVolume)
	}
	if !analysis.HasSteamVolume || analysis.SteamVolume != 2 {
		t.Fatalf("Steam 24h volume = %d (available=%t), want 2", analysis.SteamVolume, analysis.HasSteamVolume)
	}
	if !analysis.HasSteamMedian {
		t.Fatal("Steam 24h median was not derived from sale history")
	}
	assertFloatEqual(t, analysis.SteamMedianPrice, 10.0)
	assertFloatEqual(t, analysis.WeeklyAveragePrice, 16.0)
	assertFloatEqual(t, analysis.RecentSaleP75Price, 20.0)
	assertFloatEqual(t, analysis.LastSoldPrice, 10.0)
}

func TestMarketSearchHTMLRequiresExactNameAndAppID(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	body := []byte(loaderDataSearchFixture("Example (Legendary) A", SteamAppID, 1, "$0.12", 42))
	data, ok := marketDataFromSearchHTML("Example (Legendary) A", body, now, defaultMarketScope().Currency)
	if !ok {
		t.Fatal("HTML search result did not parse")
	}
	analysis := data.Analysis
	assertFloatEqual(t, analysis.LowestSellPrice, 0.12)
	if !analysis.HasLowestSell || !analysis.HasSuggested || analysis.SellOrderCount != 42 || analysis.PriceSource != "market-search-html" {
		t.Fatalf("HTML search analysis = %+v", analysis)
	}
	if analysis.HasOrderBook || analysis.HasSaleHistory || analysis.HasSteamMedian || analysis.HasSteamVolume {
		t.Fatalf("HTML search result invented unavailable metrics: %+v", analysis)
	}
	if _, ok := marketDataFromSearchHTML("example (Legendary) A", body, now, defaultMarketScope().Currency); ok {
		t.Fatal("case-mismatched HTML search result was accepted")
	}
	wrongAppBody := []byte(loaderDataSearchFixture("Example (Legendary) A", SteamAppID+1, 1, "$0.12", 42))
	if _, ok := marketDataFromSearchHTML("Example (Legendary) A", wrongAppBody, now, defaultMarketScope().Currency); ok {
		t.Fatal("wrong-AppID HTML search result was accepted")
	}
}

func TestCalculateSuggestedPrice(t *testing.T) {
	tests := []struct {
		name     string
		analysis MarketAnalysis
		want     float64
	}{
		{
			name: "normal tight spread",
			analysis: MarketAnalysis{
				LowestSellPrice:    2.57,
				HighestBuyPrice:    2.55,
				WeeklyAveragePrice: 2.56,
				HasLowestSell:      true,
				HasHighestBuy:      true,
				HasWeeklyAverage:   true,
			},
			want: 2.56,
		},
		{
			name: "wide spread uses historical cap",
			analysis: MarketAnalysis{
				LowestSellPrice:    10.0,
				HighestBuyPrice:    2.0,
				WeeklyAveragePrice: 8.0,
				HasLowestSell:      true,
				HasHighestBuy:      true,
				HasWeeklyAverage:   true,
			},
			want: 8.0,
		},
		{
			name: "recent percentile beats weekly average",
			analysis: MarketAnalysis{
				LowestSellPrice:    6.14,
				HighestBuyPrice:    1.29,
				WeeklyAveragePrice: 2.09,
				RecentSaleP75Price: 2.45,
				HasLowestSell:      true,
				HasHighestBuy:      true,
				HasWeeklyAverage:   true,
				HasRecentSaleP75:   true,
				HasLastSold:        true,
				LastSoldPrice:      0.08,
			},
			want: 2.45,
		},
		{
			name: "history missing uses current book",
			analysis: MarketAnalysis{
				LowestSellPrice: 5.0,
				HighestBuyPrice: 4.0,
				HasLowestSell:   true,
				HasHighestBuy:   true,
			},
			want: 4.99,
		},
		{
			name: "current book at market floor",
			analysis: MarketAnalysis{
				LowestSellPrice: 0.03,
				HighestBuyPrice: 0.02,
				MinListingPrice: 0.03,
				HasLowestSell:   true,
				HasHighestBuy:   true,
			},
			want: 0.03,
		},
		{
			name: "order missing uses weekly average",
			analysis: MarketAnalysis{
				WeeklyAveragePrice: 10.0,
				LastSoldPrice:      9.0,
				HasWeeklyAverage:   true,
				HasLastSold:        true,
			},
			want: 10.0,
		},
		{
			name: "historical target below market floor",
			analysis: MarketAnalysis{
				WeeklyAveragePrice: 0.02,
				MinListingPrice:    0.03,
				HasWeeklyAverage:   true,
			},
			want: 0.03,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := calculateSuggestedPrice(tt.analysis)
			if !ok {
				t.Fatal("expected suggested price")
			}
			assertFloatEqual(t, got, tt.want)
		})
	}
}

func TestMarketMinimumListingPriceByCurrency(t *testing.T) {
	withExchangeRateCache(t, map[string]float64{
		"USD": 1.0,
		"GBP": 0.79,
		"AUD": 1.50,
		"CNY": 7.25,
		"JPY": 155.0,
		"KRW": 1380.0,
		"VND": 25400.0,
		"CLP": 940.0,
		"THB": 36.7,
	})

	tests := []struct {
		code          string
		wantMinimum   float64
		wantIncrement float64
	}{
		{code: "USD", wantMinimum: 0.03, wantIncrement: 0.01},
		{code: "GBP", wantMinimum: 0.03, wantIncrement: 0.01},
		{code: "AUD", wantMinimum: 0.06, wantIncrement: 0.01},
		{code: "CNY", wantMinimum: 0.21, wantIncrement: 0.01},
		{code: "JPY", wantMinimum: 6, wantIncrement: 1},
		{code: "KRW", wantMinimum: 30, wantIncrement: 10},
		{code: "VND", wantMinimum: 1500, wantIncrement: 500},
		{code: "CLP", wantMinimum: 27, wantIncrement: 1},
		{code: "THB", wantMinimum: 1.11, wantIncrement: 0.01},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			currency, ok := marketCurrencyForCode(tt.code)
			if !ok {
				t.Fatalf("currency %s not found", tt.code)
			}
			assertFloatEqual(t, currency.MinListingPrice, tt.wantMinimum)
			assertFloatEqual(t, currency.PriceIncrement, tt.wantIncrement)

			analysis := unavailableMarketAnalysis("Example", time.Unix(1700000000, 0), currency)
			assertFloatEqual(t, analysis.MinListingPrice, tt.wantMinimum)
			assertFloatEqual(t, analysis.PriceIncrement, tt.wantIncrement)
		})
	}
}

func TestCalculateSuggestedPriceUsesCurrencyIncrement(t *testing.T) {
	tests := []struct {
		name     string
		analysis MarketAnalysis
		want     float64
	}{
		{
			name: "KRW undercuts by ten won",
			analysis: MarketAnalysis{
				LowestSellPrice: 40,
				MinListingPrice: 30,
				PriceIncrement:  10,
				HasLowestSell:   true,
			},
			want: 30,
		},
		{
			name: "VND undercuts by five hundred dong",
			analysis: MarketAnalysis{
				LowestSellPrice: 2000,
				MinListingPrice: 1500,
				PriceIncrement:  500,
				HasLowestSell:   true,
			},
			want: 1500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := calculateSuggestedPrice(tt.analysis)
			if !ok {
				t.Fatal("expected suggested price")
			}
			assertFloatEqual(t, got, tt.want)
		})
	}
}

func TestParseIconURL(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "unescaped JSON",
			body: `"icon_url":"abcdef123"`,
			want: "abcdef123",
		},
		{
			name: "escaped JSON inside JS string",
			body: `\"icon_url\":\"abcdef123\"`,
			want: "abcdef123",
		},
		{
			name: "triple escaped JSON inside JS string",
			body: `\\\"icon_url\\\":\\\"abcdef123\\\"`,
			want: "abcdef123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseIconURL([]byte(tt.body))
			if got != tt.want {
				t.Errorf("ParseIconURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
