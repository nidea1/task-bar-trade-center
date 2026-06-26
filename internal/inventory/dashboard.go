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
	totals := DashboardTotals{TotalItemCount: len(snapshot.Items)}

	for _, owned := range snapshot.Items {
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
			acc.item.PricePrefix = quote.PricePrefix
			acc.item.PriceSuffix = quote.PriceSuffix
			acc.item.UpdatedAt = quote.UpdatedAt
			if quote.HasSuggested {
				acc.item.TotalSuggested += quote.Suggested
				totals.SuggestedListingValue += quote.Suggested
				addLocationValue(&totals, owned.Location, quote.Suggested)
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
		items = append(items, acc.item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].TotalSuggested == items[j].TotalSuggested {
			return items[i].Name < items[j].Name
		}
		return items[i].TotalSuggested > items[j].TotalSuggested
	})

	state := DashboardState{
		UpdatedAt:      now,
		SnapshotReadAt: snapshot.ReadAt,
		MarketScope:    options.MarketScope,
		CurrencyCode:   options.CurrencyCode,
		PricePrefix:    options.PricePrefix,
		PriceSuffix:    options.PriceSuffix,
		Gold:           snapshot.Gold,
		Totals:         totals,
		Items:          items,
		MostValuable:   limitItems(items, 25),
		Duplicates:     duplicateItems(items, 25),
		Equipped:       equippedItems(items, 25),
		MissingPrices:  missingPriceItems(items, 25),
		Refresh:        options.Refresh,
	}
	return state
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
