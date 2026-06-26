package market

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

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
