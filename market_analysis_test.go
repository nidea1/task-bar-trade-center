package main

import (
	"encoding/json"
	"fmt"
	"math"
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
	assertFloatEqual(t, analysis.WeeklyAveragePrice, 16.0)
	assertFloatEqual(t, analysis.RecentSaleP75Price, 20.0)
	assertFloatEqual(t, analysis.LastSoldPrice, 10.0)
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
			name: "order missing uses weekly average",
			analysis: MarketAnalysis{
				WeeklyAveragePrice: 10.0,
				LastSoldPrice:      9.0,
				HasWeeklyAverage:   true,
				HasLastSold:        true,
			},
			want: 10.0,
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

func TestMergeMarketDataWithUSDFallback(t *testing.T) {
	originalLanguage := currentDisplayLanguagePreference()
	applyDisplayLanguagePreference("en-US")
	t.Cleanup(func() { applyDisplayLanguagePreference(originalLanguage) })

	now := time.Now().UTC()
	local := MarketData{
		CachedAt: now,
		Analysis: MarketAnalysis{
			PriceSuffix:        "€",
			SuggestedPrice:     0.05,
			LowestSellPrice:    0.06,
			WeeklyAveragePrice: 0.07,
			DailySalesVolume:   3528,
			HasSuggested:       true,
			HasLowestSell:      true,
			HasWeeklyAverage:   true,
			HasDailySales:      true,
			UpdatedAt:          now,
		},
	}
	usd := MarketData{
		CachedAt:        now,
		OrderCachedAt:   now,
		HistoryCachedAt: now,
		OrderBook: MarketOrderBook{
			HighestBuyPrice: 0.03,
			LowestSellPrice: 0.08,
			BuyOrderCount:   62,
			SellOrderCount:  12162,
			PricePrefix:     "$",
		},
		History: []MarketSalePoint{{Time: now.Add(-time.Hour).Unix(), Price: 0.08, Volume: 100}},
		Analysis: MarketAnalysis{
			PricePrefix:             "$",
			HighestBuyPrice:         0.03,
			LowestSellPrice:         0.08,
			WeeklyAveragePrice:      0.08,
			RecentSaleP75Price:      0.08,
			LastSoldPrice:           0.08,
			DailySalesVolume:        3200,
			BuyOrderCount:           62,
			SellOrderCount:          12162,
			TrendPercent:            10,
			SpreadPercent:           62.5,
			HasHighestBuy:           true,
			HasLowestSell:           true,
			HasWeeklyAverage:        true,
			HasRecentSaleP75:        true,
			HasLastSold:             true,
			HasDailySales:           true,
			HasOrderBook:            true,
			HasSaleHistory:          true,
			HasTrend:                true,
			HasSpread:               true,
			HasWeeklyDailyAvgVolume: true,
			WeeklyDailyAvgVolume:    450,
			UpdatedAt:               now,
		},
	}

	eurDE, _ := marketScopeFor("EUR", "DE")
	merged := mergeMarketDataWithUSDFallback(local, usd, eurDE)
	analysis := merged.Analysis
	assertFloatEqual(t, analysis.LowestSellPrice, 0.06)
	assertFloatEqual(t, analysis.WeeklyAveragePrice, 0.07)
	if !analysis.HasOrderBook || !analysis.HasSaleHistory || analysis.Confidence != "estimated" {
		t.Fatalf("merged analysis did not contain USD fallback data: %+v", analysis)
	}
	if analysis.USDFallbackMetrics&(usdFallbackHighestBuy|usdFallbackSaleP75|usdFallbackLastSold) == 0 {
		t.Fatalf("USD fallback fields were not marked: %b", analysis.USDFallbackMetrics)
	}

	view := priceOverlayViewFromAnalysis(analysis)
	if view.LowestSell != "0.06€" || view.WeeklyAvg != "0.07€" || view.Suggested != "0.05€" {
		t.Fatalf("local currency values = %+v", view)
	}
	if view.HighestBuy != "0.02€" || view.LastSold != "0.06€" || view.SaleP75 != "0.06€" {
		t.Fatalf("USD fallback values = %+v", view)
	}
	if view.FallbackNotice != "" {
		t.Fatalf("fallback notice = %q, want empty", view.FallbackNotice)
	}

	if !requiresUSDFallbackRefresh(eurDE, local.Analysis) || requiresUSDFallbackRefresh(eurDE, analysis) {
		t.Fatal("USD fallback cache refresh state was not tracked correctly")
	}
}

func TestLegacyCacheCompatibilityAndStaleFallback(t *testing.T) {
	now := time.Now().UTC()
	var diskCache map[string]MarketData
	raw := []byte(fmt.Sprintf(`{
		"Minor Ruby": {
			"OverlayText": "legacy overlay",
			"CachedAt": %q
		}
	}`, now.Add(-5*time.Minute).Format(time.RFC3339Nano)))
	if err := json.Unmarshal(raw, &diskCache); err != nil {
		t.Fatalf("legacy cache did not unmarshal: %v", err)
	}

	legacy := diskCache["Minor Ruby"]
	if isFreshMarketCache(legacy, now) {
		t.Fatal("expected text-only legacy cache to be refreshed")
	}

	legacyFormat := MarketData{
		OverlayText: "TBTC Suggested: $1.00\nWeekly Sale Avg: $1.00\nLast sold: $1.00",
		CachedAt:    now,
	}
	if isFreshMarketCache(legacyFormat, now) {
		t.Fatal("expected legacy overlay text to be treated as stale")
	}

	expired := MarketData{OverlayText: "stale overlay", CachedAt: now.Add(-24 * time.Hour)}
	if isFreshMarketCache(expired, now) {
		t.Fatal("expected old cache not to be fresh")
	}
	if _, ok := staleMarketAnalysis(expired, true); ok {
		t.Fatal("expected text-only cache not to be used as stale analysis")
	}
	analysis := MarketAnalysis{UpdatedAt: now, HasSuggested: true, SuggestedPrice: 1}
	if stale, ok := staleMarketAnalysis(MarketData{Analysis: analysis}, true); !ok || stale.SuggestedPrice != 1 {
		t.Fatalf("stale analysis = %+v, %v; want suggested analysis, true", stale, ok)
	}
}

func assertFloatEqual(t *testing.T, got float64, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.0001 {
		t.Fatalf("got %.6f, want %.6f", got, want)
	}
}

func TestNewAnalysisFeatures(t *testing.T) {
	originalLanguage := currentDisplayLanguagePreference()
	applyDisplayLanguagePreference("en-US")
	t.Cleanup(func() { applyDisplayLanguagePreference(originalLanguage) })

	// 1. calculateDealTag
	t.Run("calculateDealTag", func(t *testing.T) {
		tests := []struct {
			name     string
			analysis MarketAnalysis
			want     string
		}{
			{
				name: "Good Buy below weekly average by 15%+",
				analysis: MarketAnalysis{
					LowestSellPrice:    8.0,
					WeeklyAveragePrice: 10.0,
					HasLowestSell:      true,
					HasWeeklyAverage:   true,
				},
				want: "undervalued", // 8.0 < 8.5
			},
			{
				name: "Overpriced above weekly average by 20%+",
				analysis: MarketAnalysis{
					LowestSellPrice:    12.1,
					WeeklyAveragePrice: 10.0,
					HasLowestSell:      true,
					HasWeeklyAverage:   true,
				},
				want: "overvalued", // 12.1 > 12.0
			},
			{
				name: "Normal price within range",
				analysis: MarketAnalysis{
					LowestSellPrice:    10.0,
					WeeklyAveragePrice: 10.0,
					HasLowestSell:      true,
					HasWeeklyAverage:   true,
				},
				want: "",
			},
		}

		for _, tt := range tests {
			if got := calculateDealTag(tt.analysis); got != tt.want {
				t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
			}
		}
	})

	// 2. calculateConfidence
	t.Run("calculateConfidence", func(t *testing.T) {
		tests := []struct {
			name     string
			analysis MarketAnalysis
			want     string
		}{
			{
				name: "High confidence with all data",
				analysis: MarketAnalysis{
					HasOrderBook:     true,
					HasSaleHistory:   true,
					HasWeeklyAverage: true,
					HasRecentSaleP75: true,
					HasDailySales:    true,
					DailySalesVolume: 5,
				},
				want: "verified", // score = 2+2+1+1+1 = 7 >= 5
			},
			{
				name: "Medium confidence with partial data",
				analysis: MarketAnalysis{
					HasOrderBook:     true,
					HasWeeklyAverage: true,
				},
				want: "estimated", // score = 2+1 = 3 >= 3
			},
			{
				name: "Low confidence with minimal data",
				analysis: MarketAnalysis{
					HasWeeklyAverage: true,
				},
				want: "speculative", // score = 1
			},
		}

		for _, tt := range tests {
			if got := calculateConfidence(tt.analysis); got != tt.want {
				t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
			}
		}
	})

	// 3. calculateVolumeActivity
	t.Run("calculateVolumeActivity", func(t *testing.T) {
		if got := calculateVolumeActivity(15.0, 10.0); got != "active" {
			t.Errorf("expected active, got %q", got)
		}
		if got := calculateVolumeActivity(4.0, 10.0); got != "slow" {
			t.Errorf("expected slow, got %q", got)
		}
		if got := calculateVolumeActivity(8.0, 10.0); got != "normal" {
			t.Errorf("expected normal, got %q", got)
		}
	})

	// 4. formatRelativeTime
	t.Run("formatRelativeTime", func(t *testing.T) {
		now := time.Now()
		if got := formatRelativeTime(now.Add(-30*time.Second), now); got != "just now" {
			t.Errorf("expected just now, got %q", got)
		}
		if got := formatRelativeTime(now.Add(-5*time.Minute), now); got != "5m ago" {
			t.Errorf("expected 5m ago, got %q", got)
		}
		if got := formatRelativeTime(now.Add(-3*time.Hour), now); got != "3h ago" {
			t.Errorf("expected 3h ago, got %q", got)
		}
		if got := formatRelativeTime(now.Add(-2*24*time.Hour), now); got != "2d ago" {
			t.Errorf("expected 2d ago, got %q", got)
		}
	})

	// 5. formatTrendPercent
	t.Run("formatTrendPercent", func(t *testing.T) {
		if got := formatTrendPercent(12.3, true); got != "+12%" {
			t.Errorf("expected +12%%, got %q", got)
		}
		if got := formatTrendPercent(-8.1, true); got != "-8%" {
			t.Errorf("expected -8%%, got %q", got)
		}
		if got := formatTrendPercent(0.0, false); got != "N/A" {
			t.Errorf("expected N/A, got %q", got)
		}
	})

	// 6. formatSpread
	t.Run("formatSpread", func(t *testing.T) {
		ma := MarketAnalysis{SpreadPercent: 12.0, HasSpread: true, IsWideSpread: false}
		if got := formatSpread(ma); got != "12%" {
			t.Errorf("expected 12%%, got %q", got)
		}
		maWide := MarketAnalysis{SpreadPercent: 28.0, HasSpread: true, IsWideSpread: true}
		if got := formatSpread(maWide); got != "28% Wide" {
			t.Errorf("expected 28%% Wide, got %q", got)
		}
	})
}
