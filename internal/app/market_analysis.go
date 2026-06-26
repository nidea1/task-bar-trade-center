package app

import (
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/market"
)

const (
	usdFallbackSuggested     = market.USDFallbackSuggested
	usdFallbackLowestSell    = market.USDFallbackLowestSell
	usdFallbackHighestBuy    = market.USDFallbackHighestBuy
	usdFallbackWeeklyAverage = market.USDFallbackWeeklyAverage
	usdFallbackSaleP75       = market.USDFallbackSaleP75
	usdFallbackLastSold      = market.USDFallbackLastSold
)

func isFreshMarketCache(data MarketData, now time.Time) bool {
	return market.IsFreshCache(data, now)
}

func staleMarketAnalysis(data MarketData, exists bool) (MarketAnalysis, bool) {
	return market.StaleAnalysis(data, exists)
}

func isSSRListingForScope(body []byte, scope MarketScope) bool {
	return market.IsSSRListingForScope(body, scope)
}

func parseSSRItemOrderBook(body []byte, currency MarketCurrency) (MarketOrderBook, bool) {
	return market.ParseSSRItemOrderBook(body, currency)
}

func parseSSRPriceHistory(body []byte) []MarketSalePoint {
	return market.ParseSSRPriceHistory(body)
}

func parseLegacySaleHistoryFromListing(body []byte) []MarketSalePoint {
	return market.ParseLegacySaleHistoryFromListing(body)
}

func parseItemNameID(body []byte) string {
	return market.ParseItemNameID(body)
}

func parseItemOrdersHistogramResponse(body []byte) (MarketOrderBook, bool) {
	return market.ParseItemOrdersHistogramResponse(body)
}

func parseSaleHistoryResponse(body []byte) []MarketSalePoint {
	return market.ParseSaleHistoryResponse(body)
}

func marketDataFromPriceOverview(marketHashName string, body []byte, now time.Time, currency MarketCurrency) (MarketData, bool) {
	return market.DataFromPriceOverview(marketHashName, body, now, currency)
}

func buildMarketAnalysis(marketHashName string, orderBook MarketOrderBook, hasOrderBook bool, history []MarketSalePoint, now time.Time, currency MarketCurrency) MarketAnalysis {
	return market.BuildAnalysis(marketHashName, orderBook, hasOrderBook, history, now, currency)
}

func unavailableMarketAnalysis(marketHashName string, now time.Time, currency MarketCurrency) MarketAnalysis {
	return market.UnavailableAnalysis(marketHashName, now, currency)
}

func calculateSuggestedPrice(analysis MarketAnalysis) (float64, bool) {
	return market.SuggestedPrice(analysis)
}

func buildMarketOverlayText(analysis MarketAnalysis) string {
	return market.BuildOverlayText(analysis)
}

func formatAnalysisPrice(price float64, ok bool, analysis MarketAnalysis) string {
	return market.FormatAnalysisPrice(price, ok, analysis)
}

func formatAnalysisVolume(volume int, ok bool, activity string) string {
	return market.FormatAnalysisVolume(volume, ok, activity)
}

func formatTrendPercent(percent float64, hasTrend bool) string {
	return market.FormatTrendPercent(percent, hasTrend)
}

func formatSpread(analysis MarketAnalysis) string {
	return market.FormatSpread(analysis)
}

func formatOrderCounts(buyCount int, sellCount int, hasOrderBook bool) string {
	return market.FormatOrderCounts(buyCount, sellCount, hasOrderBook)
}

func formatRelativeTime(updatedAt time.Time, now time.Time) string {
	return market.FormatRelativeTime(updatedAt, now)
}

func parseSteamFormattedPrice(value string) (float64, string, string, bool) {
	return market.ParseSteamFormattedPrice(value)
}
