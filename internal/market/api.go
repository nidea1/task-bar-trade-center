package market

import "time"

func IsFreshCache(data MarketData, now time.Time) bool {
	return isFreshMarketCache(data, now)
}

func StaleAnalysis(data MarketData, exists bool) (MarketAnalysis, bool) {
	return staleMarketAnalysis(data, exists)
}

func UnavailableAnalysis(marketHashName string, now time.Time, currency MarketCurrency) MarketAnalysis {
	return unavailableMarketAnalysis(marketHashName, now, currency)
}

func FormatAnalysisPrice(price float64, ok bool, analysis MarketAnalysis) string {
	return formatAnalysisPrice(price, ok, analysis)
}

func FormatAnalysisVolume(volume int, ok bool, activity string) string {
	return formatAnalysisVolume(volume, ok, activity)
}

func FormatTrendPercent(percent float64, hasTrend bool) string {
	return formatTrendPercent(percent, hasTrend)
}

func FormatSpread(analysis MarketAnalysis) string {
	return formatSpread(analysis)
}

func FormatOrderCounts(buyCount int, sellCount int, hasOrderBook bool) string {
	return formatOrderCounts(buyCount, sellCount, hasOrderBook)
}

func FormatRelativeTime(updatedAt time.Time, now time.Time) string {
	return formatRelativeTime(updatedAt, now)
}

func ParseSteamFormattedPrice(value string) (float64, string, string, bool) {
	return parseSteamFormattedPrice(value)
}

func IsSSRListingForScope(body []byte, scope MarketScope) bool {
	return isSSRListingForScope(body, scope)
}

func ParseSSRItemOrderBook(body []byte, currency MarketCurrency) (MarketOrderBook, bool) {
	return parseSSRItemOrderBook(body, currency)
}

func ParseSSRPriceHistory(body []byte) []MarketSalePoint {
	return parseSSRPriceHistory(body)
}

func ParseLegacySaleHistoryFromListing(body []byte) []MarketSalePoint {
	return parseLegacySaleHistoryFromListing(body)
}

func ParseItemNameID(body []byte) string {
	return parseItemNameID(body)
}

func ParseItemOrdersHistogramResponse(body []byte) (MarketOrderBook, bool) {
	return parseItemOrdersHistogramResponse(body)
}

func ParseSaleHistoryResponse(body []byte) []MarketSalePoint {
	return parseSaleHistoryResponse(body)
}

func DataFromPriceOverview(marketHashName string, body []byte, now time.Time, currency MarketCurrency) (MarketData, bool) {
	return marketDataFromPriceOverview(marketHashName, body, now, currency)
}

func DataFromSources(marketHashName string, orderBook MarketOrderBook, hasOrderBook bool, history []MarketSalePoint, now time.Time, currency MarketCurrency) MarketData {
	return marketDataFromSources(marketHashName, orderBook, hasOrderBook, history, now, currency)
}

func HasCompleteAnalysis(analysis MarketAnalysis) bool {
	return hasCompleteMarketAnalysis(analysis)
}

func RequiresUSDFallbackRefresh(scope MarketScope, analysis MarketAnalysis) bool {
	return requiresUSDFallbackRefresh(scope, analysis)
}

func MergeWithUSDFallback(local MarketData, usd MarketData, targetScope MarketScope) MarketData {
	return mergeMarketDataWithUSDFallback(local, usd, targetScope)
}

func BuildAnalysis(marketHashName string, orderBook MarketOrderBook, hasOrderBook bool, history []MarketSalePoint, now time.Time, currency MarketCurrency) MarketAnalysis {
	return buildMarketAnalysis(marketHashName, orderBook, hasOrderBook, history, now, currency)
}

func SuggestedPrice(analysis MarketAnalysis) (float64, bool) {
	return calculateSuggestedPrice(analysis)
}

func BuildOverlayText(analysis MarketAnalysis) string {
	return buildMarketOverlayText(analysis)
}
