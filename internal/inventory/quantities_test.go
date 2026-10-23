package inventory

import (
	"github.com/nidea1/task-bar-trade-center/internal/playerdata"
	"testing"
)

func TestMaterialStackQuantityChangesValueButNotOccupiedSlots(t *testing.T) {
	snapshot := playerdata.InventorySnapshot{Items: []playerdata.OwnedItem{
		{ItemID: 100, UniqueID: 1, Quantity: 25, Location: playerdata.LocationMaterialStash, Marketable: true},
		{ItemID: 100, UniqueID: 2, Quantity: 3, Location: playerdata.LocationInventory, Marketable: true},
	}}
	state := BuildDashboard(snapshot, map[int]ItemDescriptor{100: {Name: "Ruby", Marketable: true}}, quoteMap{100: {Suggested: 2, Instant: 1, HasSuggested: true, HasInstant: true}}, DashboardOptions{})
	if state.Items[0].Count != 28 || state.Totals.SuggestedListingValue != 56 || state.Totals.InventoryValue != 6 || state.Totals.StashValue != 50 || state.Totals.MaterialStashValue != 50 {
		t.Fatalf("stack valuation=%+v", state.Totals)
	}
	if got := countInventoryItems(snapshot.Items); got != 1 {
		t.Fatalf("inventory occupied slots=%d want 1", got)
	}
}

func TestStashPageBreakdownAndMaterialStashMatchTotal(t *testing.T) {
	snapshot := playerdata.InventorySnapshot{StashPageCount: 4, Items: []playerdata.OwnedItem{
		{ItemID: 100, UniqueID: 1, Quantity: 4, Location: playerdata.LocationStash, SlotIndex: 10, Marketable: true},
		{ItemID: 200, UniqueID: 2, Location: playerdata.LocationStash, SlotIndex: 50, Marketable: true},
		{ItemID: 300, UniqueID: 3, Location: playerdata.LocationStash, SlotIndex: 100, Marketable: true},
		{ItemID: 100, UniqueID: 4, Quantity: 25, Location: playerdata.LocationMaterialStash, Marketable: true},
		{ItemID: 100, UniqueID: 5, Location: playerdata.LocationInventory, Marketable: true},
	}}
	catalog := map[int]ItemDescriptor{100: {Marketable: true}, 200: {Marketable: true}, 300: {Marketable: true}}
	state := BuildDashboard(snapshot, catalog, quoteMap{100: {Suggested: 2, HasSuggested: true}, 200: {Suggested: 5, HasSuggested: true}}, DashboardOptions{})
	totals := state.Totals
	if totals.StashPageValues[1] != 8 || totals.StashPageValues[2] != 5 || totals.StashPageValues[3] != 0 || totals.StashPageValues[4] != 0 {
		t.Fatalf("page values = %+v", totals.StashPageValues)
	}
	if totals.StashPageCounts[1] != 1 || totals.StashPageCounts[3] != 1 || totals.StashPageCounts[4] != 0 {
		t.Fatalf("page counts = %+v", totals.StashPageCounts)
	}
	breakdown := totals.MaterialStashValue
	for _, value := range totals.StashPageValues {
		breakdown += value
	}
	if totals.MaterialStashValue != 50 || totals.StashValue != 63 || breakdown != totals.StashValue {
		t.Fatalf("stash breakdown = %+v", totals)
	}
}
