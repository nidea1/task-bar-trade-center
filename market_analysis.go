package main

import (
	"encoding/json"
	"fmt"
	"html"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	jsonParseCallPattern     = regexp.MustCompile(`JSON\.parse\("((?:\\.|[^"\\])*)"\)`)
	ssrOrderBookPattern      = regexp.MustCompile(`(?s)"amtMaxBuyOrder"\s*:\s*(\d+|null).*?"amtMinSellOrder"\s*:\s*(\d+|null).*?"cBuyOrders"\s*:\s*(\d+).*?"cSellOrders"\s*:\s*(\d+)`)
	ssrCurrencyPattern       = regexp.MustCompile(`"eCurrency"\s*:\s*(\d+)`)
	ssrPriceHistoryPattern   = regexp.MustCompile(`"time"\s*:\s*(\d+)\s*,\s*"price_median"\s*:\s*([0-9]+(?:\.[0-9]+)?)\s*,\s*"purchases"\s*:\s*(\d+)`)
	itemNameIDPattern        = regexp.MustCompile(`Market_LoadOrderSpread\(\s*(\d+)\s*\)`)
	legacySaleHistoryPattern = regexp.MustCompile(`(?s)var\s+line1\s*=\s*(\[.*?\]);`)
	digitsPattern            = regexp.MustCompile(`\d+`)
	numberPattern            = regexp.MustCompile(`[0-9][0-9\.,]*`)
	pureDigitsPattern        = regexp.MustCompile(`^\d+$`)
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

func parseSSRItemOrderBook(body []byte) (MarketOrderBook, bool) {
	text := searchableListingText(body)
	match := ssrOrderBookPattern.FindStringSubmatch(text)
	if len(match) != 5 {
		return MarketOrderBook{}, false
	}

	var highestBuyCents int
	if match[1] != "null" {
		var ok bool
		highestBuyCents, ok = parseInt(match[1])
		if !ok {
			return MarketOrderBook{}, false
		}
	}

	var lowestSellCents int
	if match[2] != "null" {
		var ok bool
		lowestSellCents, ok = parseInt(match[2])
		if !ok {
			return MarketOrderBook{}, false
		}
	}

	if highestBuyCents <= 0 && lowestSellCents <= 0 {
		return MarketOrderBook{}, false
	}
	buyOrders, _ := parseInt(match[3])
	sellOrders, _ := parseInt(match[4])

	return MarketOrderBook{
		HighestBuyPrice: centsToPrice(highestBuyCents),
		LowestSellPrice: centsToPrice(lowestSellCents),
		BuyOrderCount:   buyOrders,
		SellOrderCount:  sellOrders,
		PricePrefix:     "$",
	}, true
}

func isSSRListingForScope(body []byte, scope MarketScope) bool {
	text := searchableListingText(body)
	matches := ssrCurrencyPattern.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		if len(match) != 2 {
			continue
		}
		currencyID, ok := parseInt(match[1])
		if ok && currencyID == scope.Currency.SteamCurrencyID {
			return true
		}
	}
	return false
}

func parseSSRPriceHistory(body []byte) []MarketSalePoint {
	text := searchableListingText(body)
	matches := ssrPriceHistoryPattern.FindAllStringSubmatch(text, -1)
	points := make([]MarketSalePoint, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		if len(match) != 4 {
			continue
		}
		unixTime, ok := parseInt64(match[1])
		if !ok {
			continue
		}
		price, ok := parseFloat(match[2])
		if !ok {
			continue
		}
		volume, ok := parseInt(match[3])
		if !ok {
			continue
		}
		key := fmt.Sprintf("%d:%0.6f:%d", unixTime, price, volume)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		points = append(points, MarketSalePoint{Time: unixTime, Price: price, Volume: volume})
	}
	return points
}

func parseLegacySaleHistoryFromListing(body []byte) []MarketSalePoint {
	match := legacySaleHistoryPattern.FindSubmatch(body)
	if len(match) != 2 {
		return nil
	}
	return parseSteamSaleArray(match[1])
}

func parseItemNameID(body []byte) string {
	match := itemNameIDPattern.FindSubmatch(body)
	if len(match) != 2 {
		return ""
	}
	return string(match[1])
}

func parseItemOrdersHistogramResponse(body []byte) (MarketOrderBook, bool) {
	rawObject, ok := unwrapJSONResponseObject(body)
	if !ok {
		return MarketOrderBook{}, false
	}

	highestBuy, hasHighestBuy := parseHistogramPrice(rawObject["highest_buy_order"])
	lowestSell, hasLowestSell := parseHistogramPrice(rawObject["lowest_sell_order"])
	if (!hasHighestBuy || highestBuy <= 0) && (!hasLowestSell || lowestSell <= 0) {
		return MarketOrderBook{}, false
	}

	buyOrders, _ := parseJSONInt(rawObject["buy_order_summary"])
	sellOrders, _ := parseJSONInt(rawObject["sell_order_summary"])
	pricePrefix, _ := parseJSONString(rawObject["price_prefix"])
	priceSuffix, _ := parseJSONString(rawObject["price_suffix"])
	if pricePrefix == "" && priceSuffix == "" {
		pricePrefix = "$"
	}
	if pricePrefix == "$" && strings.TrimSpace(priceSuffix) == "USD" {
		priceSuffix = ""
	}

	return MarketOrderBook{
		HighestBuyPrice: highestBuy,
		LowestSellPrice: lowestSell,
		BuyOrderCount:   buyOrders,
		SellOrderCount:  sellOrders,
		PricePrefix:     pricePrefix,
		PriceSuffix:     priceSuffix,
	}, true
}

func parseSaleHistoryResponse(body []byte) []MarketSalePoint {
	var raw interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil
	}

	if object, ok := raw.(map[string]interface{}); ok {
		if response, exists := object["response"]; exists {
			return parseSaleHistoryValue(response)
		}
		if prices, exists := object["prices"]; exists {
			return parseSaleHistoryValue(prices)
		}
	}

	return parseSaleHistoryValue(raw)
}

func marketDataFromPriceOverview(marketHashName string, body []byte, now time.Time, currency MarketCurrency) (MarketData, bool) {
	var payload struct {
		Success     bool   `json:"success"`
		LowestPrice string `json:"lowest_price"`
		MedianPrice string `json:"median_price"`
		Volume      string `json:"volume"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return MarketData{}, false
	}
	if !payload.Success && payload.LowestPrice == "" && payload.MedianPrice == "" {
		return MarketData{}, false
	}

	analysis := unavailableMarketAnalysis(marketHashName, now, currency)
	priceFormatSet := false
	if lowest, pricePrefix, priceSuffix, ok := parseSteamFormattedPrice(payload.LowestPrice); ok {
		analysis.LowestSellPrice = lowest
		analysis.HasLowestSell = true
		analysis.PricePrefix = pricePrefix
		analysis.PriceSuffix = priceSuffix
		priceFormatSet = true
	}
	if median, pricePrefix, priceSuffix, ok := parseSteamFormattedPrice(payload.MedianPrice); ok {
		analysis.WeeklyAveragePrice = median
		analysis.HasWeeklyAverage = true
		if !priceFormatSet {
			analysis.PricePrefix = pricePrefix
			analysis.PriceSuffix = priceSuffix
		}
	}
	if volume, ok := parseFirstInt(payload.Volume); ok {
		analysis.DailySalesVolume = volume
		analysis.HasDailySales = true
	}
	if suggested, ok := calculateSuggestedPrice(analysis); ok {
		analysis.SuggestedPrice = suggested
		analysis.HasSuggested = true
	}

	return MarketData{
		CachedAt: now,
		Analysis: analysis,
	}, true
}

func buildMarketAnalysis(marketHashName string, orderBook MarketOrderBook, hasOrderBook bool, history []MarketSalePoint, now time.Time, currency MarketCurrency) MarketAnalysis {
	analysis := unavailableMarketAnalysis(marketHashName, now, currency)
	if hasOrderBook {
		analysis.HasOrderBook = true
		analysis.LowestSellPrice = orderBook.LowestSellPrice
		analysis.HighestBuyPrice = orderBook.HighestBuyPrice
		analysis.BuyOrderCount = orderBook.BuyOrderCount
		analysis.SellOrderCount = orderBook.SellOrderCount
		analysis.HasLowestSell = orderBook.LowestSellPrice > 0
		analysis.HasHighestBuy = orderBook.HighestBuyPrice > 0
		if orderBook.PricePrefix != "" || orderBook.PriceSuffix != "" {
			if !(currency.Code != "USD" && orderBook.PricePrefix == "$") {
				analysis.PricePrefix = orderBook.PricePrefix
				analysis.PriceSuffix = orderBook.PriceSuffix
			}
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

func buildMarketOverlayText(analysis MarketAnalysis) string {
	var builder strings.Builder
	builder.WriteString(AppShortName + " Suggested: " + formatAnalysisPrice(analysis.SuggestedPrice, analysis.HasSuggested, analysis))
	if analysis.HasConfidence {
		builder.WriteString(" [" + analysis.Confidence + "]")
	}
	builder.WriteString("\r\n")
	if analysis.HasDealTag {
		builder.WriteString("DealTag: " + analysis.DealTag + "\r\n")
	}
	builder.WriteString("Last Sold: " + formatAnalysisPrice(analysis.LastSoldPrice, analysis.HasLastSold, analysis) + "\r\n")
	builder.WriteString("Lowest Sell: " + formatAnalysisPrice(analysis.LowestSellPrice, analysis.HasLowestSell, analysis) + "\r\n")
	builder.WriteString("Highest Buy: " + formatAnalysisPrice(analysis.HighestBuyPrice, analysis.HasHighestBuy, analysis) + "\r\n")
	builder.WriteString("Weekly Avg: " + formatAnalysisPrice(analysis.WeeklyAveragePrice, analysis.HasWeeklyAverage, analysis) + "\r\n")
	builder.WriteString("Sale P75: " + formatAnalysisPrice(analysis.RecentSaleP75Price, analysis.HasRecentSaleP75, analysis) + "\r\n")
	builder.WriteString("Spread: " + formatSpread(analysis) + "\r\n")
	builder.WriteString("Trend: " + formatTrendPercent(analysis.TrendPercent, analysis.HasTrend) + "\r\n")
	builder.WriteString("Daily Sales: " + formatAnalysisVolume(analysis.DailySalesVolume, analysis.HasDailySales, analysis.VolumeActivity) + "\r\n")
	builder.WriteString("Orders: " + formatOrderCounts(analysis.BuyOrderCount, analysis.SellOrderCount, analysis.HasOrderBook) + "\r\n")
	builder.WriteString("Updated: " + analysis.UpdatedAt.Format(time.RFC3339))
	return builder.String()
}

func formatAnalysisPrice(price float64, ok bool, analysis MarketAnalysis) string {
	if !ok || price <= 0 {
		return tr("value.na")
	}
	prefix := analysis.PricePrefix
	suffix := analysis.PriceSuffix
	if prefix == "" && suffix == "" {
		prefix = "$"
	}
	return fmt.Sprintf("%s%.2f%s", prefix, price, suffix)
}

func formatUSDAnalysisPrice(price float64, ok bool) string {
	return formatAnalysisPrice(price, ok, MarketAnalysis{PricePrefix: "$"})
}

func formatAnalysisVolume(volume int, ok bool, activity string) string {
	if !ok {
		return tr("value.na")
	}
	result := formatIntWithCommas(volume) + " " + tr("value.units")
	if activity != "" {
		result += " (" + localizedSemanticValue(activity) + ")"
	}
	return result
}

func formatTrendPercent(percent float64, hasTrend bool) string {
	if !hasTrend {
		return tr("value.na")
	}
	if percent > 0 {
		return fmt.Sprintf("+%.0f%%", percent)
	}
	return fmt.Sprintf("%.0f%%", percent)
}

func formatSpread(analysis MarketAnalysis) string {
	if !analysis.HasSpread {
		return tr("value.na")
	}
	result := fmt.Sprintf("%.0f%%", analysis.SpreadPercent)
	if analysis.IsWideSpread {
		result += " " + tr("value.wide")
	}
	return result
}

func formatOrderCounts(buyCount int, sellCount int, hasOrderBook bool) string {
	if !hasOrderBook {
		return tr("value.na")
	}
	return fmt.Sprintf("%sB / %sS", formatIntWithCommas(buyCount), formatIntWithCommas(sellCount))
}

func formatRelativeTime(updatedAt time.Time, now time.Time) string {
	delta := now.Sub(updatedAt)
	if delta < 0 {
		delta = 0
	}
	switch {
	case delta < time.Minute:
		return tr("time.just_now")
	case delta < time.Hour:
		return tr("time.minutes_ago", int(delta.Minutes()))
	case delta < 24*time.Hour:
		return tr("time.hours_ago", int(delta.Hours()))
	default:
		return tr("time.days_ago", int(delta.Hours()/24))
	}
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

func formatIntWithCommas(value int) string {
	if value < 1000 {
		return strconv.Itoa(value)
	}
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}
	raw := strconv.Itoa(value)
	var builder strings.Builder
	firstGroup := len(raw) % 3
	if firstGroup == 0 {
		firstGroup = 3
	}
	builder.WriteString(raw[:firstGroup])
	for i := firstGroup; i < len(raw); i += 3 {
		builder.WriteByte(',')
		builder.WriteString(raw[i : i+3])
	}
	return sign + builder.String()
}

func searchableListingText(body []byte) string {
	raw := html.UnescapeString(string(body))
	var builder strings.Builder
	builder.WriteString(raw)
	appendDecodedJSONParseStrings(&builder, raw, 0)
	return builder.String()
}

func appendDecodedJSONParseStrings(builder *strings.Builder, text string, depth int) {
	if depth > 2 {
		return
	}
	for _, match := range jsonParseCallPattern.FindAllStringSubmatch(text, -1) {
		if len(match) != 2 {
			continue
		}
		decoded, err := strconv.Unquote(`"` + match[1] + `"`)
		if err != nil {
			continue
		}
		normalized := strings.ReplaceAll(decoded, `\"`, `"`)
		builder.WriteByte('\n')
		builder.WriteString(decoded)
		builder.WriteByte('\n')
		builder.WriteString(normalized)
		appendDecodedJSONParseStrings(builder, decoded, depth+1)
		appendDecodedJSONParseStrings(builder, normalized, depth+1)
	}
}

func unwrapJSONResponseObject(body []byte) (map[string]json.RawMessage, bool) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		return nil, false
	}
	if successRaw, exists := top["success"]; exists {
		if success, ok := parseJSONBool(successRaw); ok && !success {
			return nil, false
		}
	}
	if responseRaw, exists := top["response"]; exists {
		var response map[string]json.RawMessage
		if err := json.Unmarshal(responseRaw, &response); err == nil {
			return response, true
		}
	}
	return top, true
}

func parseHistogramPrice(raw json.RawMessage) (float64, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return parseSteamPriceString(text, true)
	}
	var value float64
	if err := json.Unmarshal(raw, &value); err == nil {
		return value, true
	}
	return 0, false
}

func parseJSONBool(raw json.RawMessage) (bool, bool) {
	var value bool
	if err := json.Unmarshal(raw, &value); err == nil {
		return value, true
	}
	var numeric float64
	if err := json.Unmarshal(raw, &numeric); err == nil {
		return numeric != 0, true
	}
	return false, false
}

func parseJSONInt(raw json.RawMessage) (int, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	var value float64
	if err := json.Unmarshal(raw, &value); err == nil {
		return int(value), true
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return parseFirstInt(text)
	}
	return 0, false
}

func parseJSONString(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}
	return value, true
}

func parseSaleHistoryValue(value interface{}) []MarketSalePoint {
	items, ok := value.([]interface{})
	if !ok {
		return nil
	}
	points := make([]MarketSalePoint, 0, len(items))
	for _, item := range items {
		switch typed := item.(type) {
		case map[string]interface{}:
			point, ok := parseSaleHistoryObject(typed)
			if ok {
				points = append(points, point)
			}
		case []interface{}:
			point, ok := parseSaleHistoryArray(typed)
			if ok {
				points = append(points, point)
			}
		}
	}
	return points
}

func parseSteamSaleArray(raw []byte) []MarketSalePoint {
	var items []interface{}
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil
	}
	return parseSaleHistoryValue(items)
}

func parseSaleHistoryObject(item map[string]interface{}) (MarketSalePoint, bool) {
	unixTime, ok := interfaceInt64(item["time"])
	if !ok {
		return MarketSalePoint{}, false
	}
	price, ok := interfaceFloat(item["price"])
	if !ok || price <= 0 {
		return MarketSalePoint{}, false
	}
	volume, ok := interfaceInt(item["volume"])
	if !ok {
		return MarketSalePoint{}, false
	}
	return MarketSalePoint{Time: unixTime, Price: price, Volume: volume}, true
}

func parseSaleHistoryArray(item []interface{}) (MarketSalePoint, bool) {
	if len(item) < 3 {
		return MarketSalePoint{}, false
	}

	var unixTime int64
	switch value := item[0].(type) {
	case string:
		parsedTime, ok := parseSteamHistoryTime(value)
		if !ok {
			return MarketSalePoint{}, false
		}
		unixTime = parsedTime
	case float64:
		unixTime = int64(value)
	default:
		return MarketSalePoint{}, false
	}

	price, ok := interfaceFloat(item[1])
	if !ok || price <= 0 {
		return MarketSalePoint{}, false
	}
	volume, ok := interfaceInt(item[2])
	if !ok {
		return MarketSalePoint{}, false
	}
	return MarketSalePoint{Time: unixTime, Price: price, Volume: volume}, true
}

func parseSteamHistoryTime(value string) (int64, bool) {
	fields := strings.Fields(value)
	if len(fields) < 3 {
		return 0, false
	}
	parsed, err := time.ParseInLocation("Jan 02 2006", fields[0]+" "+fields[1]+" "+fields[2], time.UTC)
	if err != nil {
		return 0, false
	}
	return parsed.Unix(), true
}

func interfaceFloat(value interface{}) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case string:
		return parseSteamPriceString(typed, false)
	default:
		return 0, false
	}
}

func interfaceInt(value interface{}) (int, bool) {
	switch typed := value.(type) {
	case float64:
		return int(typed), true
	case string:
		return parseFirstInt(typed)
	default:
		return 0, false
	}
}

func interfaceInt64(value interface{}) (int64, bool) {
	switch typed := value.(type) {
	case float64:
		return int64(typed), true
	case string:
		return parseInt64(typed)
	default:
		return 0, false
	}
}

func parseSteamPriceString(value string, integerStringIsCents bool) (float64, bool) {
	value = strings.TrimSpace(html.UnescapeString(value))
	if value == "" {
		return 0, false
	}
	if integerStringIsCents && pureDigitsPattern.MatchString(value) {
		cents, ok := parseInt(value)
		if !ok {
			return 0, false
		}
		return centsToPrice(cents), true
	}

	match := numberPattern.FindString(value)
	if match == "" {
		return 0, false
	}
	normalized := normalizeDecimalString(match)
	price, err := strconv.ParseFloat(normalized, 64)
	if err != nil {
		return 0, false
	}
	return price, true
}

func parseSteamFormattedPrice(value string) (float64, string, string, bool) {
	value = strings.TrimSpace(html.UnescapeString(value))
	matchIndex := numberPattern.FindStringIndex(value)
	if matchIndex == nil {
		return 0, "", "", false
	}
	price, ok := parseSteamPriceString(value, false)
	if !ok {
		return 0, "", "", false
	}
	prefix := strings.TrimSpace(value[:matchIndex[0]])
	suffix := strings.TrimSpace(value[matchIndex[1]:])
	if prefix == "$" && strings.TrimSpace(suffix) == "USD" {
		suffix = ""
	}
	return price, prefix, suffix, true
}

func normalizeDecimalString(value string) string {
	if strings.Contains(value, ".") && strings.Contains(value, ",") {
		return strings.ReplaceAll(value, ",", "")
	}
	if strings.Contains(value, ",") && !strings.Contains(value, ".") {
		lastComma := strings.LastIndex(value, ",")
		if len(value)-lastComma-1 == 2 {
			value = strings.ReplaceAll(value, ".", "")
			return strings.Replace(value, ",", ".", 1)
		}
		return strings.ReplaceAll(value, ",", "")
	}
	return value
}

func parseFirstInt(value string) (int, bool) {
	matches := digitsPattern.FindAllString(value, -1)
	if len(matches) == 0 {
		return 0, false
	}
	joined := strings.Join(matches, "")
	return parseInt(joined)
}

func parseInt(value string) (int, bool) {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func parseInt64(value string) (int64, bool) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func parseFloat(value string) (float64, bool) {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func centsToPrice(cents int) float64 {
	return float64(cents) / 100
}
