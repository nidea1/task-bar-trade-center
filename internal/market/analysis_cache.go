package market

import (
	"strings"
	"time"
)

func isFreshMarketCache(data MarketData, now time.Time) bool {
	if data.Analysis.UpdatedAt.IsZero() {
		return false
	}

	if data.Analysis.HasOrderBook {
		if data.OrderCachedAt.IsZero() || now.Sub(data.OrderCachedAt) >= marketOrderCacheTTL {
			return false
		}
	} else if !data.CachedAt.IsZero() && now.Sub(data.CachedAt) >= marketOrderCacheTTL {
		return false
	}

	if data.Analysis.HasSaleHistory {
		if data.HistoryCachedAt.IsZero() || now.Sub(data.HistoryCachedAt) >= marketHistoryTTL {
			return false
		}
	}

	return true
}

func staleMarketAnalysis(data MarketData, exists bool) (MarketAnalysis, bool) {
	if !exists || data.Analysis.UpdatedAt.IsZero() {
		return MarketAnalysis{}, false
	}
	return data.Analysis, true
}

func normalizeCachedMarketData(data MarketData) (MarketData, bool) {
	analysis := &data.Analysis
	if analysis.HasSaleHistory || (!analysis.HasWeeklyAverage && !analysis.HasDailySales) {
		if !analysis.PriceIsEstimated && analysis.USDFallbackMetrics != 0 {
			analysis.PriceIsEstimated = true
			return data, true
		}
		return data, false
	}

	migrated := false
	if analysis.HasWeeklyAverage {
		if !analysis.HasSteamMedian {
			analysis.SteamMedianPrice = analysis.WeeklyAveragePrice
			analysis.HasSteamMedian = analysis.WeeklyAveragePrice > 0
		}
		analysis.WeeklyAveragePrice = 0
		analysis.HasWeeklyAverage = false
		analysis.USDFallbackMetrics &^= usdFallbackWeeklyAverage
		migrated = true
	}
	if analysis.HasDailySales {
		if !analysis.HasSteamVolume {
			analysis.SteamVolume = analysis.DailySalesVolume
			analysis.HasSteamVolume = true
		}
		analysis.DailySalesVolume = 0
		analysis.HasDailySales = false
		analysis.DailyAveragePrice = 0
		analysis.WeeklyDailyAvgVolume = 0
		analysis.HasWeeklyDailyAvgVolume = false
		analysis.VolumeActivity = ""
		analysis.TrendPercent = 0
		analysis.HasTrend = false
		migrated = true
	}
	if !migrated {
		return data, false
	}

	analysis.DealTag = calculateDealTag(*analysis)
	analysis.HasDealTag = analysis.DealTag != ""
	analysis.SuggestedPrice = 0
	analysis.HasSuggested = false
	if suggested, ok := calculateSuggestedPrice(*analysis); ok {
		analysis.SuggestedPrice = suggested
		analysis.HasSuggested = true
	}
	analysis.Confidence = calculateConfidence(*analysis)
	analysis.HasConfidence = analysis.Confidence != ""
	analysis.PriceIsEstimated = analysis.USDFallbackMetrics != 0
	data.OverlayText = ""
	return data, true
}

func isLegacyMarketOverlayText(text string) bool {
	return strings.Contains(text, "Weekly Sale Avg:") ||
		strings.Contains(text, "Last sold:") ||
		strings.Contains(text, "[Min ") ||
		strings.Contains(text, "(N/A/") ||
		strings.Contains(text, "Range:")
}
