package market

import "time"

const (
	usdFallbackSuggested uint16 = 1 << iota
	usdFallbackLowestSell
	usdFallbackHighestBuy
	usdFallbackWeeklyAverage
	usdFallbackSaleP75
	usdFallbackLastSold
)

const (
	USDFallbackSuggested     = usdFallbackSuggested
	USDFallbackLowestSell    = usdFallbackLowestSell
	USDFallbackHighestBuy    = usdFallbackHighestBuy
	USDFallbackWeeklyAverage = usdFallbackWeeklyAverage
	USDFallbackSaleP75       = usdFallbackSaleP75
	USDFallbackLastSold      = usdFallbackLastSold
)

func hasCompleteMarketAnalysis(analysis MarketAnalysis) bool {
	return analysis.HasOrderBook && analysis.HasSaleHistory
}

func requiresUSDFallbackRefresh(scope MarketScope, analysis MarketAnalysis) bool {
	if scope == defaultMarketScope() || hasCompleteMarketAnalysis(analysis) {
		return false
	}
	if !analysis.USDDataFallbackAttempted {
		return true
	}
	if analysis.USDFallbackMetrics != 0 && analysis.PricePrefix == "$" && scope.Currency.PricePrefix != "$" {
		return true
	}
	return false
}

func calculateExchangeRate(local MarketAnalysis, usd MarketAnalysis) (float64, bool) {
	if local.HasLowestSell && local.LowestSellPrice > 0 && usd.HasLowestSell && usd.LowestSellPrice > 0 {
		return local.LowestSellPrice / usd.LowestSellPrice, true
	}
	if local.HasHighestBuy && local.HighestBuyPrice > 0 && usd.HasHighestBuy && usd.HighestBuyPrice > 0 {
		return local.HighestBuyPrice / usd.HighestBuyPrice, true
	}
	if local.HasWeeklyAverage && local.WeeklyAveragePrice > 0 && usd.HasWeeklyAverage && usd.WeeklyAveragePrice > 0 {
		return local.WeeklyAveragePrice / usd.WeeklyAveragePrice, true
	}
	if local.HasSuggested && local.SuggestedPrice > 0 && usd.HasSuggested && usd.SuggestedPrice > 0 {
		return local.SuggestedPrice / usd.SuggestedPrice, true
	}
	if local.HasLastSold && local.LastSoldPrice > 0 && usd.HasLastSold && usd.LastSoldPrice > 0 {
		return local.LastSoldPrice / usd.LastSoldPrice, true
	}
	if local.HasRecentSaleP75 && local.RecentSaleP75Price > 0 && usd.HasRecentSaleP75 && usd.RecentSaleP75Price > 0 {
		return local.RecentSaleP75Price / usd.RecentSaleP75Price, true
	}
	return 0, false
}

func mergeMarketDataWithUSDFallback(local MarketData, usd MarketData, targetScope MarketScope) MarketData {
	analysis := &local.Analysis
	usdAnalysis := usd.Analysis
	analysis.USDDataFallbackAttempted = true
	if analysis.IconURL == "" && usdAnalysis.IconURL != "" {
		analysis.IconURL = usdAnalysis.IconURL
	}

	currencyCode := targetScope.Currency.Code
	rate := 1.0
	if currencyCode != "USD" {
		if r, ok := calculateExchangeRate(*analysis, usdAnalysis); ok {
			rate = r
			setExchangeRate(currencyCode, rate)
		} else {
			rate = getExchangeRate(currencyCode)
		}
	}

	if analysis.PricePrefix == "" && analysis.PriceSuffix == "" {
		analysis.PricePrefix = targetScope.Currency.PricePrefix
		analysis.PriceSuffix = targetScope.Currency.PriceSuffix
		if analysis.PricePrefix == "" && analysis.PriceSuffix == "" {
			analysis.PricePrefix = "$"
		}
	}

	if usdAnalysis.HasOrderBook {
		if !analysis.HasOrderBook {
			analysis.HasOrderBook = true
			analysis.BuyOrderCount = usdAnalysis.BuyOrderCount
			analysis.SellOrderCount = usdAnalysis.SellOrderCount
			analysis.LowestSellQuantity = usdAnalysis.LowestSellQuantity
			analysis.HighestBuyQuantity = usdAnalysis.HighestBuyQuantity
			local.OrderBook = usd.OrderBook
			local.OrderCachedAt = usd.OrderCachedAt
			if rate != 1.0 {
				local.OrderBook.HighestBuyPrice *= rate
				local.OrderBook.LowestSellPrice *= rate
			}
			local.OrderBook.PricePrefix = targetScope.Currency.PricePrefix
			local.OrderBook.PriceSuffix = targetScope.Currency.PriceSuffix
		}
		if !analysis.HasLowestSell && usdAnalysis.HasLowestSell {
			analysis.LowestSellPrice = usdAnalysis.LowestSellPrice * rate
			analysis.HasLowestSell = true
			analysis.LowestSellQuantity = usdAnalysis.LowestSellQuantity
			analysis.USDFallbackMetrics |= usdFallbackLowestSell
		}
		if !analysis.HasHighestBuy && usdAnalysis.HasHighestBuy {
			analysis.HighestBuyPrice = usdAnalysis.HighestBuyPrice * rate
			analysis.HasHighestBuy = true
			analysis.HighestBuyQuantity = usdAnalysis.HighestBuyQuantity
			analysis.USDFallbackMetrics |= usdFallbackHighestBuy
		}
		if !analysis.HasSpread && usdAnalysis.HasSpread {
			analysis.SpreadPercent = usdAnalysis.SpreadPercent
			analysis.HasSpread = true
			analysis.IsWideSpread = usdAnalysis.IsWideSpread
		}
	}

	if usdAnalysis.HasSaleHistory {
		if !analysis.HasSaleHistory {
			analysis.HasSaleHistory = true
			local.History = make([]MarketSalePoint, len(usd.History))
			copy(local.History, usd.History)
			local.HistoryCachedAt = usd.HistoryCachedAt
			if rate != 1.0 {
				for i := range local.History {
					local.History[i].Price *= rate
				}
			}
		}
		if !analysis.HasTrend && usdAnalysis.HasTrend {
			analysis.TrendPercent = usdAnalysis.TrendPercent
			analysis.HasTrend = true
		}
		if !analysis.HasRecentSaleP75 && usdAnalysis.HasRecentSaleP75 {
			analysis.RecentSaleP75Price = usdAnalysis.RecentSaleP75Price * rate
			analysis.HasRecentSaleP75 = true
			analysis.USDFallbackMetrics |= usdFallbackSaleP75
		}
		if !analysis.HasLastSold && usdAnalysis.HasLastSold {
			analysis.LastSoldPrice = usdAnalysis.LastSoldPrice * rate
			analysis.HasLastSold = true
			analysis.USDFallbackMetrics |= usdFallbackLastSold
		}
		if !analysis.HasWeeklyAverage && usdAnalysis.HasWeeklyAverage {
			analysis.WeeklyAveragePrice = usdAnalysis.WeeklyAveragePrice * rate
			analysis.HasWeeklyAverage = true
			analysis.USDFallbackMetrics |= usdFallbackWeeklyAverage
		}
		if !analysis.HasDailySales && usdAnalysis.HasDailySales {
			analysis.DailySalesVolume = usdAnalysis.DailySalesVolume
			analysis.HasDailySales = true
			analysis.VolumeActivity = usdAnalysis.VolumeActivity
		}
		if !analysis.HasWeeklyDailyAvgVolume && usdAnalysis.HasWeeklyDailyAvgVolume {
			analysis.WeeklyDailyAvgVolume = usdAnalysis.WeeklyDailyAvgVolume
			analysis.HasWeeklyDailyAvgVolume = true
		}
	}

	if !analysis.HasSuggested && usdAnalysis.HasSuggested {
		analysis.SuggestedPrice = usdAnalysis.SuggestedPrice * rate
		analysis.HasSuggested = true
		analysis.USDFallbackMetrics |= usdFallbackSuggested
	}
	if analysis.USDFallbackMetrics != 0 {
		analysis.Confidence = "estimated"
		analysis.HasConfidence = true
	}
	normalizeAnalysisCurrencyFormat(analysis, targetScope.Currency)
	return local
}

func normalizeAnalysisCurrencyFormat(analysis *MarketAnalysis, currency MarketCurrency) {
	prefix := currency.PricePrefix
	suffix := currency.PriceSuffix
	if prefix == "" && suffix == "" {
		prefix = "$"
	}
	analysis.PricePrefix = prefix
	analysis.PriceSuffix = suffix
}

func marketDataFromSources(marketHashName string, orderBook MarketOrderBook, hasOrderBook bool, history []MarketSalePoint, now time.Time, currency MarketCurrency) MarketData {
	analysis := buildMarketAnalysis(marketHashName, orderBook, hasOrderBook, history, now, currency)
	data := MarketData{
		CachedAt: now,
		Analysis: analysis,
	}
	if hasOrderBook {
		data.OrderBook = orderBook
		data.OrderCachedAt = now
	}
	if len(history) > 0 {
		data.History = history
		data.HistoryCachedAt = now
	}
	return data
}

func marketDataFromBaseHistory(marketHashName string, orderBook MarketOrderBook, hasOrderBook bool, baseHistory []MarketSalePoint, now time.Time, currency MarketCurrency) MarketData {
	if len(baseHistory) == 0 || currency.Code == "USD" {
		return marketDataFromSources(marketHashName, orderBook, hasOrderBook, baseHistory, now, currency)
	}

	if rate, ok := baseHistoryConversionRate(marketHashName, orderBook, hasOrderBook, baseHistory, now, currency); ok {
		return marketDataFromSources(marketHashName, orderBook, hasOrderBook, scaleSaleHistory(baseHistory, rate), now, currency)
	}

	data := marketDataFromSources(marketHashName, orderBook, hasOrderBook, nil, now, currency)
	baseAnalysis := buildMarketAnalysis(marketHashName, MarketOrderBook{}, false, copySaleHistory(baseHistory), now, MarketCurrency{Code: "USD", PricePrefix: "$"})
	copyHistorySignalsWithoutPrices(&data.Analysis, baseAnalysis)
	data.History = copySaleHistory(baseHistory)
	data.HistoryCachedAt = now
	return data
}

func baseHistoryConversionRate(marketHashName string, orderBook MarketOrderBook, hasOrderBook bool, baseHistory []MarketSalePoint, now time.Time, currency MarketCurrency) (float64, bool) {
	if currency.Code == "USD" {
		return 1.0, true
	}

	configured, hasConfigured := configuredExchangeRate(currency.Code)
	if derived, ok := deriveHistoryRateFromLocalSources(marketHashName, orderBook, hasOrderBook, baseHistory, now, currency); ok {
		if !hasConfigured || exchangeRateClose(derived, configured) {
			setExchangeRate(currency.Code, derived)
			return derived, true
		}
	}
	if hasConfigured {
		return configured, true
	}
	return 0, false
}

func deriveHistoryRateFromLocalSources(marketHashName string, orderBook MarketOrderBook, hasOrderBook bool, baseHistory []MarketSalePoint, now time.Time, currency MarketCurrency) (float64, bool) {
	if !hasOrderBook || len(baseHistory) == 0 {
		return 0, false
	}
	local := buildMarketAnalysis(marketHashName, orderBook, true, nil, now, currency)
	base := buildMarketAnalysis(marketHashName, MarketOrderBook{}, false, copySaleHistory(baseHistory), now, MarketCurrency{Code: "USD", PricePrefix: "$"})
	if local.HasSuggested && local.SuggestedPrice > 0 && base.HasSuggested && base.SuggestedPrice > 0 {
		return local.SuggestedPrice / base.SuggestedPrice, true
	}
	if local.HasLowestSell && local.LowestSellPrice > 0 && base.HasLastSold && base.LastSoldPrice > 0 {
		return local.LowestSellPrice / base.LastSoldPrice, true
	}
	if local.HasHighestBuy && local.HighestBuyPrice > 0 && base.HasLastSold && base.LastSoldPrice > 0 {
		return local.HighestBuyPrice / base.LastSoldPrice, true
	}
	return 0, false
}

func exchangeRateClose(candidate float64, reference float64) bool {
	if candidate <= 0 || reference <= 0 {
		return false
	}
	return candidate >= reference*0.5 && candidate <= reference*1.5
}

func scaleSaleHistory(history []MarketSalePoint, rate float64) []MarketSalePoint {
	scaled := copySaleHistory(history)
	for index := range scaled {
		scaled[index].Price *= rate
	}
	return scaled
}

func copySaleHistory(history []MarketSalePoint) []MarketSalePoint {
	if len(history) == 0 {
		return nil
	}
	copied := make([]MarketSalePoint, len(history))
	copy(copied, history)
	return copied
}

func copyHistorySignalsWithoutPrices(target *MarketAnalysis, source MarketAnalysis) {
	target.HasSaleHistory = source.HasSaleHistory
	target.DailySalesVolume = source.DailySalesVolume
	target.HasDailySales = source.HasDailySales
	target.TrendPercent = source.TrendPercent
	target.HasTrend = source.HasTrend
	target.WeeklyDailyAvgVolume = source.WeeklyDailyAvgVolume
	target.HasWeeklyDailyAvgVolume = source.HasWeeklyDailyAvgVolume
	target.VolumeActivity = source.VolumeActivity
	target.Confidence = calculateConfidence(*target)
	target.HasConfidence = target.Confidence != ""
}
