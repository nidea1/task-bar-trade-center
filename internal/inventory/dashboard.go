package inventory

import (
	"sort"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/playerdata"
)

type DashboardOptions struct {
	MarketScope  string
	CurrencyCode string
	PricePrefix  string
	PriceSuffix  string
	Refresh      RefreshStatus
	Now          time.Time
}

const stashSlotsPerPage = 100

type groupAccumulator struct {
	item     DashboardItem
	seen     map[string]struct{}
	equipped bool
}

func BuildDashboard(snapshot playerdata.InventorySnapshot, catalog map[int]ItemDescriptor, quotes QuoteProvider, options DashboardOptions) DashboardState {
	now := options.Now
	if now.IsZero() {
		now = time.Now()
	}
	groups := make(map[int]*groupAccumulator)
	stashPageCount := dashboardStashPageCount(snapshot)
	totals := DashboardTotals{
		TotalItemCount: len(snapshot.Items),
		StashPageCount: stashPageCount,
		HeroEquippedValues: map[int]float64{
			1: 0.0,
			2: 0.0,
			3: 0.0,
			4: 0.0,
			5: 0.0,
			6: 0.0,
		},
		StashPageValues: make(map[int]float64),
		StashPageCounts: make(map[int]int),
	}
	for page := 1; page <= stashPageCount; page++ {
		totals.StashPageValues[page] = 0
		totals.StashPageCounts[page] = 0
	}

	for _, owned := range snapshot.Items {
		if owned.Location == playerdata.LocationStash {
			page := stashPageForSlot(owned.SlotIndex)
			totals.StashPageCounts[page]++
			if _, exists := totals.StashPageValues[page]; !exists {
				totals.StashPageValues[page] = 0
			}
		}
		desc := catalog[owned.ItemID]
		if !owned.Marketable || !desc.Marketable {
			continue
		}
		totals.MarketableItemCount++
		acc := groups[owned.ItemID]
		if acc == nil {
			acc = &groupAccumulator{
				item: DashboardItem{
					ItemID:         owned.ItemID,
					Name:           desc.Name,
					MarketHashName: desc.MarketHashName,
					MarketURL:      desc.MarketURL,
					IconURL:        desc.IconURL,
					Grade:          desc.Grade,
					Type:           desc.Type,
					Gear:           desc.Gear,
				},
				seen: make(map[string]struct{}),
			}
			groups[owned.ItemID] = acc
		}
		acc.item.Count++
		if owned.Location == playerdata.LocationEquipped {
			acc.equipped = true
			acc.item.Equipped = true
		}
		acc.seen[string(owned.Location)] = struct{}{}
		quote, ok := quotes.Quote(owned.ItemID)
		if ok && (quote.HasSuggested || quote.HasInstant) {
			acc.item.HasPrice = true
			acc.item.Suggested = quote.Suggested
			acc.item.Instant = quote.Instant
			acc.item.WeeklyAveragePrice = quote.WeeklyAveragePrice
			acc.item.SpreadPercent = quote.SpreadPercent
			acc.item.DailySalesVolume = quote.DailySalesVolume
			acc.item.BuyOrderCount = quote.BuyOrderCount
			acc.item.SellOrderCount = quote.SellOrderCount
			acc.item.HasWeeklyAverage = quote.HasWeeklyAverage
			acc.item.HasSpread = quote.HasSpread
			acc.item.HasDailySales = quote.HasDailySales
			acc.item.HasOrderBook = quote.HasOrderBook
			acc.item.Confidence = quote.Confidence
			acc.item.HasConfidence = quote.HasConfidence
			acc.item.PricePrefix = quote.PricePrefix
			acc.item.PriceSuffix = quote.PriceSuffix
			acc.item.UpdatedAt = quote.UpdatedAt
			if quote.HasSuggested {
				acc.item.TotalSuggested += quote.Suggested
				totals.SuggestedListingValue += quote.Suggested
				addLocationValue(&totals, owned.Location, quote.Suggested)
				if owned.Location == playerdata.LocationStash {
					page := stashPageForSlot(owned.SlotIndex)
					totals.StashPageValues[page] += quote.Suggested
				}
				if owned.Location == playerdata.LocationEquipped && owned.EquippedHeroKey > 0 {
					classID := mapHeroKeyToClassID(owned.EquippedHeroKey)
					if _, exists := totals.HeroEquippedValues[classID]; exists {
						totals.HeroEquippedValues[classID] += quote.Suggested
					}
				}
			}
			if quote.HasInstant {
				acc.item.TotalInstant += quote.Instant
				totals.InstantSellValue += quote.Instant
			}
			totals.PricedItemCount++
		} else {
			totals.UnknownItemCount++
		}
	}
	items := make([]DashboardItem, 0, len(groups))
	for _, acc := range groups {
		acc.item.Location = joinLocations(acc.seen)
		acc.item.SellScore, acc.item.SellReasons = sellNowScore(acc.item)
		items = append(items, acc.item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].TotalSuggested == items[j].TotalSuggested {
			return items[i].Name < items[j].Name
		}
		return items[i].TotalSuggested > items[j].TotalSuggested
	})

	state := DashboardState{
		UpdatedAt:      now.Format(time.RFC3339),
		SnapshotReadAt: snapshot.ReadAt.Format(time.RFC3339),
		MarketScope:    options.MarketScope,
		CurrencyCode:   options.CurrencyCode,
		PricePrefix:    options.PricePrefix,
		PriceSuffix:    options.PriceSuffix,
		Gold:           snapshot.Gold,
		Totals:         totals,
		Items:          items,
		MostValuable:   limitItems(items, 25),
		BestToSellNow:  bestItemsToSellNow(items, 12),
		Duplicates:     duplicateItems(items, 25),
		Equipped:       equippedItems(items, 25),
		MissingPrices:  missingPriceItems(items, 25),
		Refresh:        options.Refresh,
		TradeSlots:     snapshot.TradeSlots,
	}
	return state
}

func dashboardStashPageCount(snapshot playerdata.InventorySnapshot) int {
	pageCount := snapshot.StashPageCount
	for _, owned := range snapshot.Items {
		if owned.Location != playerdata.LocationStash {
			continue
		}
		page := stashPageForSlot(owned.SlotIndex)
		if page > pageCount {
			pageCount = page
		}
	}
	return pageCount
}

func stashPageForSlot(slotIndex int) int {
	if slotIndex < 0 {
		return 1
	}
	return (slotIndex / stashSlotsPerPage) + 1
}

func addLocationValue(totals *DashboardTotals, location playerdata.Location, value float64) {
	switch location {
	case playerdata.LocationInventory:
		totals.InventoryValue += value
	case playerdata.LocationStash:
		totals.StashValue += value
	case playerdata.LocationEquipped:
		totals.EquippedValue += value
	}
}

func joinLocations(locations map[string]struct{}) string {
	order := []string{string(playerdata.LocationEquipped), string(playerdata.LocationStash), string(playerdata.LocationInventory)}
	result := ""
	for _, location := range order {
		if _, exists := locations[location]; !exists {
			continue
		}
		if result != "" {
			result += ", "
		}
		result += location
	}
	return result
}

func limitItems(items []DashboardItem, limit int) []DashboardItem {
	if len(items) <= limit {
		return append([]DashboardItem(nil), items...)
	}
	return append([]DashboardItem(nil), items[:limit]...)
}

func duplicateItems(items []DashboardItem, limit int) []DashboardItem {
	filtered := make([]DashboardItem, 0)
	for _, item := range items {
		if item.Count > 1 {
			filtered = append(filtered, item)
		}
	}
	return limitItems(filtered, limit)
}

func equippedItems(items []DashboardItem, limit int) []DashboardItem {
	filtered := make([]DashboardItem, 0)
	for _, item := range items {
		if item.Equipped {
			filtered = append(filtered, item)
		}
	}
	return limitItems(filtered, limit)
}

func missingPriceItems(items []DashboardItem, limit int) []DashboardItem {
	filtered := make([]DashboardItem, 0)
	for _, item := range items {
		if !item.HasPrice {
			filtered = append(filtered, item)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Count == filtered[j].Count {
			return filtered[i].Name < filtered[j].Name
		}
		return filtered[i].Count > filtered[j].Count
	})
	return limitItems(filtered, limit)
}

func bestItemsToSellNow(items []DashboardItem, limit int) []DashboardItem {
	filtered := make([]DashboardItem, 0)
	for _, item := range items {
		if item.SellScore >= 45 {
			filtered = append(filtered, item)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].SellScore == filtered[j].SellScore {
			return filtered[i].TotalSuggested > filtered[j].TotalSuggested
		}
		return filtered[i].SellScore > filtered[j].SellScore
	})
	return limitItems(filtered, limit)
}

func sellNowScore(item DashboardItem) (float64, []string) {
	if !item.HasPrice {
		return 0, nil
	}
	score := 0.0
	reasons := make([]string, 0, 5)

	if item.HasDailySales && item.DailySalesVolume > 0 {
		switch {
		case item.DailySalesVolume >= 100:
			score += 25
		case item.DailySalesVolume >= 30:
			score += 18
		case item.DailySalesVolume >= 10:
			score += 10
		}
		if item.DailySalesVolume >= 10 {
			reasons = append(reasons, "high_daily_sales")
		}
	}

	if item.HasSpread && item.SpreadPercent >= 0 {
		switch {
		case item.SpreadPercent <= 8:
			score += 25
		case item.SpreadPercent <= 15:
			score += 18
		case item.SpreadPercent <= 25:
			score += 10
		}
		if item.SpreadPercent <= 25 {
			reasons = append(reasons, "narrow_spread")
		}
	}

	if item.HasOrderBook && item.BuyOrderCount > 0 {
		switch {
		case item.BuyOrderCount >= 100:
			score += 20
		case item.BuyOrderCount >= 30:
			score += 14
		case item.BuyOrderCount >= 10:
			score += 8
		}
		if item.BuyOrderCount >= 10 {
			reasons = append(reasons, "high_buy_orders")
		}
	}

	if item.HasConfidence {
		switch item.Confidence {
		case "verified":
			score += 20
			reasons = append(reasons, "high_confidence")
		case "estimated":
			score += 10
		}
	}

	if item.HasWeeklyAverage && item.WeeklyAveragePrice > 0 && item.Suggested > item.WeeklyAveragePrice {
		score += 15
		reasons = append(reasons, "above_weekly_average")
	}

	if len(reasons) == 0 {
		return 0, nil
	}
	return score, reasons
}

func mapHeroKeyToClassID(heroKey int) int {
	if heroKey >= 101 && heroKey <= 601 && heroKey%100 == 1 {
		return heroKey / 100
	}
	if heroKey >= 1 && heroKey <= 6 {
		return heroKey
	}
	return 0
}
