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

func isLegacyMarketOverlayText(text string) bool {
	return strings.Contains(text, "Weekly Sale Avg:") ||
		strings.Contains(text, "Last sold:") ||
		strings.Contains(text, "[Min ") ||
		strings.Contains(text, "(N/A/") ||
		strings.Contains(text, "Range:")
}
