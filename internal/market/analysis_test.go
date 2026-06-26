package market

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"
	"time"
)

func TestMergeMarketDataWithUSDFallback(t *testing.T) {
	now := time.Now().UTC()
	local := MarketData{
		CachedAt: now,
		Analysis: MarketAnalysis{
			PriceSuffix:        "â‚¬",
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

	if got := formatAnalysisPrice(analysis.LowestSellPrice, analysis.HasLowestSell, analysis); got != "0.06â‚¬" {
		t.Fatalf("lowest sell = %q, want 0.06â‚¬", got)
	}
	if got := formatAnalysisPrice(analysis.WeeklyAveragePrice, analysis.HasWeeklyAverage, analysis); got != "0.07â‚¬" {
		t.Fatalf("weekly avg = %q, want 0.07â‚¬", got)
	}
	if got := formatAnalysisPrice(analysis.HighestBuyPrice, analysis.HasHighestBuy, analysis); got != "0.02â‚¬" {
		t.Fatalf("highest buy = %q, want 0.02â‚¬", got)
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
