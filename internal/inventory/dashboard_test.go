package inventory

import (
	"testing"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/playerdata"
)

type quoteMap map[int]PriceQuote

func (quotes quoteMap) Quote(itemID int) (PriceQuote, bool) {
	quote, ok := quotes[itemID]
	return quote, ok
}

func TestBuildDashboardGroupsAndValuesInventory(t *testing.T) {
	now := time.Unix(1700000000, 0)
	snapshot := playerdata.InventorySnapshot{
		ReadAt: now,
		Gold:   123,
		Items: []playerdata.OwnedItem{
			{ItemID: 100, UniqueID: 1, Location: playerdata.LocationStash, Marketable: true},
			{ItemID: 100, UniqueID: 2, Location: playerdata.LocationStash, Marketable: true},
			{ItemID: 200, UniqueID: 3, Location: playerdata.LocationEquipped, Marketable: true},
			{ItemID: 300, UniqueID: 4, Location: playerdata.LocationInventory, Marketable: false},
			{ItemID: 400, UniqueID: 5, Location: playerdata.LocationInventory, Marketable: true},
		},
	}
	catalog := map[int]ItemDescriptor{
		100: {Name: "Ruby", Marketable: true},
		200: {Name: "Sword", Marketable: true},
		300: {Name: "Hidden", Marketable: false},
		400: {Name: "Box", Marketable: true},
	}
	quotes := quoteMap{
		100: {Suggested: 2, Instant: 1.5, HasSuggested: true, HasInstant: true, PricePrefix: "$", UpdatedAt: now},
		200: {Suggested: 10, Instant: 8, HasSuggested: true, HasInstant: true, PricePrefix: "$", UpdatedAt: now},
	}

	state := BuildDashboard(snapshot, catalog, quotes, DashboardOptions{Now: now, PricePrefix: "$"})

	if state.Gold != 123 {
		t.Fatalf("gold = %d, want 123", state.Gold)
	}
	if state.Totals.TotalItemCount != 5 || state.Totals.MarketableItemCount != 4 {
		t.Fatalf("counts = %+v", state.Totals)
	}
	if state.Totals.PricedItemCount != 3 || state.Totals.UnknownItemCount != 1 {
		t.Fatalf("price coverage = %+v", state.Totals)
	}
	if state.Totals.SuggestedListingValue != 14 || state.Totals.InstantSellValue != 11 {
		t.Fatalf("totals = %+v", state.Totals)
	}
	if len(state.Duplicates) != 1 || state.Duplicates[0].ItemID != 100 || state.Duplicates[0].Count != 2 {
		t.Fatalf("duplicates = %+v", state.Duplicates)
	}
	if len(state.Equipped) != 1 || state.Equipped[0].ItemID != 200 {
		t.Fatalf("equipped = %+v", state.Equipped)
	}
	if len(state.MissingPrices) != 1 || state.MissingPrices[0].ItemID != 400 {
		t.Fatalf("missing = %+v", state.MissingPrices)
	}
}
