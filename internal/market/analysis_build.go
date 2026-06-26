package market

import (
	"math"
	"sort"
	"time"
)

func buildMarketAnalysis(marketHashName string, orderBook MarketOrderBook, hasOrderBook bool, history []MarketSalePoint, now time.Time, currency MarketCurrency) MarketAnalysis {
	analysis := unavailableMarketAnalysis(marketHashName, now, currency)
	if hasOrderBook {
		analysis.HasOrderBook = true
		analysis.LowestSellPrice = orderBook.LowestSellPrice
		analysis.HighestBuyPrice = orderBook.HighestBuyPrice
		analysis.LowestSellQuantity = orderBook.LowestSellQuantity
		analysis.HighestBuyQuantity = orderBook.HighestBuyQuantity
		analysis.BuyOrderCount = orderBook.BuyOrderCount
		analysis.SellOrderCount = orderBook.SellOrderCount
		analysis.HasLowestSell = orderBook.LowestSellPrice > 0
		analysis.HasHighestBuy = orderBook.HighestBuyPrice > 0
		if orderBook.PricePrefix != "" || orderBook.PriceSuffix != "" {
			analysis.PricePrefix = orderBook.PricePrefix
			analysis.PriceSuffix = orderBook.PriceSuffix
		}
	}

	if len(history) > 0 {
		analysis.HasSaleHistory = true
		sort.Slice(history, func(i int, j int) bool {
			return history[i].Time < history[j].Time
		})

		last := history[len(history)-1]
		if last.Price > 0 {
			analysis.LastSoldPrice = last.Price
			analysis.HasLastSold = true
		}

		dayStart := now.Add(-24 * time.Hour).Unix()
		weekStart := now.Add(-7 * 24 * time.Hour).Unix()
		nowUnix := now.Unix()
		weeklyVolume := 0
		weeklyTotal := 0.0
		dailyPriceTotal := 0.0
		dailyPriceVolume := 0
		recentSales := make([]MarketSalePoint, 0, len(history))

		for _, point := range history {
			if point.Time > nowUnix {
				continue
			}
			if point.Time >= dayStart {
				analysis.DailySalesVolume += point.Volume
				if point.Price > 0 && point.Volume > 0 {
					dailyPriceTotal += point.Price * float64(point.Volume)
					dailyPriceVolume += point.Volume
				}
			}
			if point.Time >= weekStart && point.Volume > 0 && point.Price > 0 {
				weeklyVolume += point.Volume
				weeklyTotal += point.Price * float64(point.Volume)
				recentSales = append(recentSales, point)
			}
		}
		analysis.HasDailySales = true
		if weeklyVolume > 0 {
			analysis.WeeklyAveragePrice = weeklyTotal / float64(weeklyVolume)
			analysis.HasWeeklyAverage = true
			analysis.WeeklyDailyAvgVolume = float64(weeklyVolume) / 7.0
			analysis.HasWeeklyDailyAvgVolume = true
		}
		if dailyPriceVolume > 0 {
			analysis.DailyAveragePrice = dailyPriceTotal / float64(dailyPriceVolume)
		}
		if dailyPriceVolume > 0 && analysis.HasWeeklyAverage && analysis.WeeklyAveragePrice > 0 {
			analysis.TrendPercent = ((analysis.DailyAveragePrice - analysis.WeeklyAveragePrice) / analysis.WeeklyAveragePrice) * 100
			analysis.HasTrend = true
		}
		if p75, ok := weightedSalePercentile(recentSales, 0.75); ok {
			analysis.RecentSaleP75Price = p75
			analysis.HasRecentSaleP75 = true
		}
		if analysis.HasWeeklyDailyAvgVolume && analysis.WeeklyDailyAvgVolume > 0 {
			analysis.VolumeActivity = calculateVolumeActivity(float64(analysis.DailySalesVolume), analysis.WeeklyDailyAvgVolume)
		}
	}

	if analysis.HasLowestSell && analysis.HasHighestBuy && analysis.LowestSellPrice > 0 {
		analysis.SpreadPercent = ((analysis.LowestSellPrice - analysis.HighestBuyPrice) / analysis.LowestSellPrice) * 100
		analysis.HasSpread = true
		analysis.IsWideSpread = analysis.SpreadPercent > 25
	}

	if suggested, ok := calculateSuggestedPrice(analysis); ok {
		analysis.SuggestedPrice = suggested
		analysis.HasSuggested = true
	}

	analysis.DealTag = calculateDealTag(analysis)
	analysis.HasDealTag = analysis.DealTag != ""
	analysis.Confidence = calculateConfidence(analysis)
	analysis.HasConfidence = analysis.Confidence != ""
	normalizeAnalysisCurrencyFormat(&analysis, currency)

	return analysis
}

func unavailableMarketAnalysis(marketHashName string, now time.Time, currency MarketCurrency) MarketAnalysis {
	prefix := currency.PricePrefix
	suffix := currency.PriceSuffix
	if prefix == "" && suffix == "" {
		prefix = "$"
	}
	return MarketAnalysis{
		MarketHashName: marketHashName,
		PricePrefix:    prefix,
		PriceSuffix:    suffix,
		UpdatedAt:      now,
	}
}

func calculateSuggestedPrice(analysis MarketAnalysis) (float64, bool) {
	historicalTarget, hasHistoricalTarget := historicalSuggestedTarget(analysis)

	if analysis.HasLowestSell && analysis.LowestSellPrice > 0 {
		target := undercutPrice(analysis.LowestSellPrice)
		if hasHistoricalTarget && analysis.LowestSellPrice > historicalTarget*1.20 {
			target = math.Min(target, historicalTarget)
		}
		floor := suggestedFloor(analysis)
		if floor > 0 {
			target = math.Max(target, floor)
		}
		ceiling := undercutPrice(analysis.LowestSellPrice)
		if target > ceiling {
			target = ceiling
		}
		if target <= 0 {
			target = analysis.LowestSellPrice
		}
		return roundPrice(target), true
	}

	if hasHistoricalTarget {
		target := math.Max(historicalTarget, suggestedFloor(analysis))
		return roundPrice(target), true
	}
	if analysis.HasHighestBuy && analysis.HighestBuyPrice > 0 {
		return roundPrice(analysis.HighestBuyPrice * 1.03), true
	}
	if analysis.HasLastSold && analysis.LastSoldPrice > 0 {
		return roundPrice(analysis.LastSoldPrice), true
	}
	return 0, false
}

func historicalSuggestedTarget(analysis MarketAnalysis) (float64, bool) {
	target := 0.0
	if analysis.HasRecentSaleP75 && analysis.RecentSaleP75Price > 0 {
		target = analysis.RecentSaleP75Price
	}
	if analysis.HasWeeklyAverage && analysis.WeeklyAveragePrice > 0 {
		target = math.Max(target, analysis.WeeklyAveragePrice)
	}
	if target > 0 {
		return target, true
	}
	if analysis.HasLastSold && analysis.LastSoldPrice > 0 {
		return analysis.LastSoldPrice, true
	}
	return 0, false
}

func suggestedFloor(analysis MarketAnalysis) float64 {
	if analysis.HasHighestBuy && analysis.HighestBuyPrice > 0 {
		return analysis.HighestBuyPrice * 1.03
	}
	return 0
}

func weightedSalePercentile(points []MarketSalePoint, percentile float64) (float64, bool) {
	if len(points) == 0 || percentile <= 0 {
		return 0, false
	}
	points = append([]MarketSalePoint(nil), points...)
	sort.Slice(points, func(i int, j int) bool {
		return points[i].Price < points[j].Price
	})

	totalVolume := 0
	for _, point := range points {
		if point.Price > 0 && point.Volume > 0 {
			totalVolume += point.Volume
		}
	}
	if totalVolume == 0 {
		return 0, false
	}

	threshold := int(math.Ceil(float64(totalVolume) * percentile))
	if threshold < 1 {
		threshold = 1
	}
	runningVolume := 0
	for _, point := range points {
		if point.Price <= 0 || point.Volume <= 0 {
			continue
		}
		runningVolume += point.Volume
		if runningVolume >= threshold {
			return point.Price, true
		}
	}
	return 0, false
}

func undercutPrice(price float64) float64 {
	if price <= 0 {
		return 0
	}
	if price < 1 {
		return math.Max(0.01, price-0.01)
	}
	if price <= 10 {
		return math.Max(0.01, price-0.01)
	}
	return price * 0.99
}

func roundPrice(price float64) float64 {
	return math.Round(price*100) / 100
}

func calculateDealTag(analysis MarketAnalysis) string {
	if !analysis.HasLowestSell || analysis.LowestSellPrice <= 0 {
		return ""
	}
	hasRef := false
	isGoodBuy := false
	isOverpriced := false

	if analysis.HasWeeklyAverage && analysis.WeeklyAveragePrice > 0 {
		hasRef = true
		if analysis.LowestSellPrice < analysis.WeeklyAveragePrice*0.85 {
			isGoodBuy = true
		}
		if analysis.LowestSellPrice > analysis.WeeklyAveragePrice*1.20 {
			isOverpriced = true
		}
	}
	if analysis.HasRecentSaleP75 && analysis.RecentSaleP75Price > 0 {
		hasRef = true
		if analysis.LowestSellPrice < analysis.RecentSaleP75Price*0.85 {
			isGoodBuy = true
		}
		if analysis.LowestSellPrice > analysis.RecentSaleP75Price*1.20 {
			if isOverpriced || (!analysis.HasWeeklyAverage) {
				isOverpriced = true
			}
		}
	}

	if !hasRef {
		return ""
	}
	if isGoodBuy {
		return "undervalued"
	}
	if isOverpriced {
		return "overvalued"
	}
	return ""
}

func calculateConfidence(analysis MarketAnalysis) string {
	score := 0
	if analysis.HasOrderBook {
		score += 2
	}
	if analysis.HasSaleHistory {
		score += 2
	}
	if analysis.HasWeeklyAverage {
		score++
	}
	if analysis.HasRecentSaleP75 {
		score++
	}
	if analysis.HasDailySales && analysis.DailySalesVolume > 0 {
		score++
	}

	switch {
	case score >= 5:
		return "verified"
	case score >= 3:
		return "estimated"
	default:
		return "speculative"
	}
}

func calculateVolumeActivity(dailyVolume float64, weeklyDailyAvg float64) string {
	if weeklyDailyAvg <= 0 {
		return ""
	}
	ratio := dailyVolume / weeklyDailyAvg
	switch {
	case ratio >= 1.5:
		return "active"
	case ratio <= 0.5:
		return "slow"
	default:
		return "normal"
	}
}
