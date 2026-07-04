package inventory

import (
	"math"
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
		100: {Suggested: 2, Instant: 1.5, HasSuggested: true, HasInstant: true, PricePrefix: "$", UpdatedAt: now.Format(time.RFC3339)},
		200: {Suggested: 10, Instant: 8, HasSuggested: true, HasInstant: true, PricePrefix: "$", UpdatedAt: now.Format(time.RFC3339)},
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

func TestBuildDashboardHeroEquippedValues(t *testing.T) {
	now := time.Unix(1700000000, 0)
	snapshot := playerdata.InventorySnapshot{
		ReadAt: now,
		Gold:   0,
		Items: []playerdata.OwnedItem{
			{ItemID: 100, UniqueID: 1, Location: playerdata.LocationEquipped, EquippedHeroKey: 101, Marketable: true},  // Knight (1)
			{ItemID: 200, UniqueID: 2, Location: playerdata.LocationEquipped, EquippedHeroKey: 201, Marketable: true},  // Ranger (2)
			{ItemID: 300, UniqueID: 3, Location: playerdata.LocationEquipped, EquippedHeroKey: 301, Marketable: true},  // Sorcerer (3)
			{ItemID: 400, UniqueID: 4, Location: playerdata.LocationInventory, EquippedHeroKey: 401, Marketable: true}, // Not equipped
		},
	}
	catalog := map[int]ItemDescriptor{
		100: {Name: "Sword", Marketable: true},
		200: {Name: "Bow", Marketable: true},
		300: {Name: "Staff", Marketable: true},
		400: {Name: "Scepter", Marketable: true},
	}
	quotes := quoteMap{
		100: {Suggested: 10, HasSuggested: true},
		200: {Suggested: 20, HasSuggested: true},
		300: {Suggested: 30, HasSuggested: true},
		400: {Suggested: 40, HasSuggested: true},
	}

	state := BuildDashboard(snapshot, catalog, quotes, DashboardOptions{Now: now})

	if state.Totals.HeroEquippedValues[1] != 10 {
		t.Errorf("Knight equipped value = %f, want 10", state.Totals.HeroEquippedValues[1])
	}
	if state.Totals.HeroEquippedValues[2] != 20 {
		t.Errorf("Ranger equipped value = %f, want 20", state.Totals.HeroEquippedValues[2])
	}
	if state.Totals.HeroEquippedValues[3] != 30 {
		t.Errorf("Sorcerer equipped value = %f, want 30", state.Totals.HeroEquippedValues[3])
	}
	if state.Totals.HeroEquippedValues[4] != 0 {
		t.Errorf("Priest equipped value = %f, want 0 (item in inventory)", state.Totals.HeroEquippedValues[4])
	}
}

func TestBuildDashboardBestItemsToSellNowUsesMarketSignals(t *testing.T) {
	now := time.Unix(1700000000, 0)
	snapshot := playerdata.InventorySnapshot{
		ReadAt: now,
		Items: []playerdata.OwnedItem{
			{ItemID: 100, UniqueID: 1, Location: playerdata.LocationInventory, Marketable: true},
			{ItemID: 200, UniqueID: 2, Location: playerdata.LocationInventory, Marketable: true},
		},
	}
	catalog := map[int]ItemDescriptor{
		100: {Name: "Liquid Ruby", Marketable: true},
		200: {Name: "Expensive Relic", Marketable: true},
	}
	quotes := quoteMap{
		100: {
			Suggested:          12,
			WeeklyAveragePrice: 10,
			SpreadPercent:      6,
			DailySalesVolume:   120,
			BuyOrderCount:      80,
			HasSuggested:       true,
			HasWeeklyAverage:   true,
			HasSpread:          true,
			HasDailySales:      true,
			HasOrderBook:       true,
			Confidence:         "verified",
			HasConfidence:      true,
		},
		200: {
			Suggested:        200,
			HasSuggested:     true,
			Confidence:       "speculative",
			HasConfidence:    true,
			DailySalesVolume: 1,
			HasDailySales:    true,
		},
	}

	state := BuildDashboard(snapshot, catalog, quotes, DashboardOptions{Now: now})

	if len(state.BestToSellNow) != 1 {
		t.Fatalf("best sell items = %+v, want 1", state.BestToSellNow)
	}
	best := state.BestToSellNow[0]
	if best.ItemID != 100 {
		t.Fatalf("best item = %d, want 100", best.ItemID)
	}
	if best.SellScore <= 0 || len(best.SellReasons) == 0 {
		t.Fatalf("best item missing sell score/reasons: %+v", best)
	}
}

func TestBuildDashboardBestItemsToSellNowIsNotLimitedToTwelve(t *testing.T) {
	now := time.Unix(1700000000, 0)
	snapshot := playerdata.InventorySnapshot{ReadAt: now}
	catalog := make(map[int]ItemDescriptor)
	quotes := make(quoteMap)
	for id := 1; id <= 13; id++ {
		snapshot.Items = append(snapshot.Items, playerdata.OwnedItem{
			ItemID:     id,
			UniqueID:   uint64(id),
			Location:   playerdata.LocationInventory,
			Marketable: true,
		})
		catalog[id] = ItemDescriptor{Name: "Liquid Item", Marketable: true}
		quotes[id] = PriceQuote{
			Suggested:        float64(id),
			SpreadPercent:    6,
			DailySalesVolume: 120,
			BuyOrderCount:    80,
			HasSuggested:     true,
			HasSpread:        true,
			HasDailySales:    true,
			HasOrderBook:     true,
			Confidence:       "verified",
			HasConfidence:    true,
		}
	}

	state := BuildDashboard(snapshot, catalog, quotes, DashboardOptions{Now: now})

	if len(state.BestToSellNow) != 13 {
		t.Fatalf("best sell item count = %d, want 13", len(state.BestToSellNow))
	}
}

func TestBuildDashboardIncludesEmptyStashPages(t *testing.T) {
	now := time.Unix(1700000000, 0)
	snapshot := playerdata.InventorySnapshot{
		ReadAt:         now,
		StashPageCount: 7,
		Items: []playerdata.OwnedItem{
			{ItemID: 100, UniqueID: 1, Location: playerdata.LocationStash, SlotIndex: 0, Marketable: true},
			{ItemID: 200, UniqueID: 2, Location: playerdata.LocationStash, SlotIndex: 401, Marketable: true},
			{ItemID: 300, UniqueID: 3, Location: playerdata.LocationStash, SlotIndex: 520, Marketable: true},
			{ItemID: 400, UniqueID: 4, Location: playerdata.LocationStash, SlotIndex: 650, Marketable: true},
		},
	}
	catalog := map[int]ItemDescriptor{
		100: {Name: "Ruby", Marketable: true},
		200: {Name: "Emerald", Marketable: true},
		300: {Name: "Topaz", Marketable: true},
		400: {Name: "Sapphire", Marketable: true},
	}
	quotes := quoteMap{
		100: {Suggested: 2, HasSuggested: true},
		200: {Suggested: 5, HasSuggested: true},
		300: {Suggested: 6, HasSuggested: true},
		400: {Suggested: 7, HasSuggested: true},
	}

	state := BuildDashboard(snapshot, catalog, quotes, DashboardOptions{Now: now})

	if state.Totals.StashPageCount != 7 {
		t.Fatalf("stash page count = %d, want 7", state.Totals.StashPageCount)
	}
	if len(state.Totals.StashPageValues) != 7 {
		t.Fatalf("stash page values = %+v, want 7 pages", state.Totals.StashPageValues)
	}
	if len(state.Totals.StashPageCounts) != 7 {
		t.Fatalf("stash page counts = %+v, want 7 pages", state.Totals.StashPageCounts)
	}
	for page := 1; page <= 7; page++ {
		if _, exists := state.Totals.StashPageValues[page]; !exists {
			t.Fatalf("stash page %d missing from %+v", page, state.Totals.StashPageValues)
		}
		if _, exists := state.Totals.StashPageCounts[page]; !exists {
			t.Fatalf("stash page %d missing from counts %+v", page, state.Totals.StashPageCounts)
		}
	}
	if state.Totals.StashPageValues[1] != 2 {
		t.Errorf("stash page 1 value = %f, want 2", state.Totals.StashPageValues[1])
	}
	if state.Totals.StashPageCounts[1] != 1 {
		t.Errorf("stash page 1 count = %d, want 1", state.Totals.StashPageCounts[1])
	}
	if state.Totals.StashPageCounts[2] != 0 {
		t.Errorf("stash page 2 count = %d, want 0", state.Totals.StashPageCounts[2])
	}
	if state.Totals.StashPageValues[5] != 5 {
		t.Errorf("stash page 5 value = %f, want 5", state.Totals.StashPageValues[5])
	}
	if state.Totals.StashPageCounts[5] != 1 {
		t.Errorf("stash page 5 count = %d, want 1", state.Totals.StashPageCounts[5])
	}
	if state.Totals.StashPageValues[6] != 6 {
		t.Errorf("stash page 6 value = %f, want 6", state.Totals.StashPageValues[6])
	}
	if state.Totals.StashPageValues[7] != 7 {
		t.Errorf("stash page 7 value = %f, want 7", state.Totals.StashPageValues[7])
	}
}

func TestBuildInventoryFillEstimateUsesRuneReductions(t *testing.T) {
	snapshot := playerdata.InventorySnapshot{
		InventorySlots: playerdata.InventorySlotSummary{Total: 20, Used: 10, Free: 10},
		ChestAutomation: playerdata.ChestAutomation{
			CommonRuneReductionSeconds: 39,
			StageRuneReductionSeconds:  75,
			RuneResolved:               true,
			HasCommonAutoOpenRune:      true,
			HasStageAutoOpenRune:       true,
		},
		ChestCounts: playerdata.ChestCounts{NormalChests: 1, StageChests: 1, TotalChests: 2},
	}

	estimate := BuildInventoryFillEstimate(snapshot)

	if !estimate.Available || !estimate.Fills {
		t.Fatalf("estimate availability = available %t fills %t, want true/true", estimate.Available, estimate.Fills)
	}
	if estimate.CommonAutoOpenSeconds != 261 {
		t.Fatalf("common auto-open = %d, want 261", estimate.CommonAutoOpenSeconds)
	}
	if estimate.StageAutoOpenSeconds != 525 {
		t.Fatalf("stage auto-open = %d, want 525", estimate.StageAutoOpenSeconds)
	}
	if math.Abs(estimate.NetFillPerHour-20.6502) > 0.001 {
		t.Fatalf("net fill per hour = %.4f, want about 20.6502", estimate.NetFillPerHour)
	}
	if estimate.NextCommonDropSeconds != 240 || estimate.NextStageDropSeconds != 360 {
		t.Fatalf("next drops = common %d stage %d, want 240/360", estimate.NextCommonDropSeconds, estimate.NextStageDropSeconds)
	}
	if estimate.CommonChestCount != 1 || estimate.StageChestCount != 1 {
		t.Fatalf("chest counts = common %d stage %d, want 1/1", estimate.CommonChestCount, estimate.StageChestCount)
	}
	if estimate.TimeToFullSeconds != 1827 {
		t.Fatalf("time to full = %d, want 1827 seconds", estimate.TimeToFullSeconds)
	}
}

func TestBuildInventoryFillEstimateUsesCustomChestTiming(t *testing.T) {
	snapshot := playerdata.InventorySnapshot{
		InventorySlots: playerdata.InventorySlotSummary{Total: 20, Used: 10, Free: 10},
		ChestAutomation: playerdata.ChestAutomation{
			CommonRuneReductionSeconds: 39,
			StageRuneReductionSeconds:  75,
			RuneResolved:               true,
			HasCommonAutoOpenRune:      true,
			HasStageAutoOpenRune:       true,
		},
		ChestCounts: playerdata.ChestCounts{NormalChests: 1, StageChests: 1, TotalChests: 2},
	}
	timing := ChestTimingConfig{
		CommonDropSeconds:         300,
		StageDropSeconds:          420,
		CommonBaseAutoOpenSeconds: 360,
		StageBaseAutoOpenSeconds:  700,
	}

	estimate := BuildInventoryFillEstimateWithTiming(snapshot, timing)

	if estimate.CommonDropSeconds != 300 || estimate.StageDropSeconds != 420 {
		t.Fatalf("drop seconds = common %d stage %d, want 300/420", estimate.CommonDropSeconds, estimate.StageDropSeconds)
	}
	if estimate.CommonAutoOpenSeconds != 321 || estimate.StageAutoOpenSeconds != 625 {
		t.Fatalf("auto-open seconds = common %d stage %d, want 321/625", estimate.CommonAutoOpenSeconds, estimate.StageAutoOpenSeconds)
	}
	if estimate.NextCommonDropSeconds != 300 || estimate.NextStageDropSeconds != 420 {
		t.Fatalf("next drops = common %d stage %d, want 300/420", estimate.NextCommonDropSeconds, estimate.NextStageDropSeconds)
	}
}

func TestChestItemFillRateUsesAutoOpenThroughputWhenDropsBacklog(t *testing.T) {
	stageRate := chestItemFillRate(stageChestDropSeconds, 525) * 3600
	if math.Abs(stageRate-6.8571) > 0.001 {
		t.Fatalf("stage rate = %.4f, want about 6.8571", stageRate)
	}

	commonRate := chestItemFillRate(commonChestDropSeconds, 261) * 3600
	if math.Abs(commonRate-13.7931) > 0.001 {
		t.Fatalf("common rate = %.4f, want about 13.7931", commonRate)
	}
}

func TestBuildInventoryFillEstimateUsesFreeSlotSummary(t *testing.T) {
	snapshot := playerdata.InventorySnapshot{
		InventorySlots: playerdata.InventorySlotSummary{Total: 300, Used: 196, Free: 104},
		ChestAutomation: playerdata.ChestAutomation{
			CommonRuneReductionSeconds: 39,
			StageRuneReductionSeconds:  75,
			RuneResolved:               true,
			HasCommonAutoOpenRune:      true,
			HasStageAutoOpenRune:       true,
		},
	}

	estimate := BuildInventoryFillEstimate(snapshot)

	if estimate.FreeSlots != 104 {
		t.Fatalf("free slots = %d, want 104", estimate.FreeSlots)
	}
	if estimate.TimeToFullSeconds != 18510 {
		t.Fatalf("time to full = %d, want 18510 seconds", estimate.TimeToFullSeconds)
	}
}

func TestBuildInventoryFillEstimateUsesObservedDropSchedule(t *testing.T) {
	snapshot := playerdata.InventorySnapshot{
		InventorySlots: playerdata.InventorySlotSummary{Total: 20, Used: 10, Free: 10},
		ChestAutomation: playerdata.ChestAutomation{
			CommonRuneReductionSeconds: 39,
			StageRuneReductionSeconds:  75,
			RuneResolved:               true,
			HasCommonAutoOpenRune:      true,
			HasStageAutoOpenRune:       true,
		},
	}

	estimate := BuildInventoryFillEstimate(snapshot, ChestDropSchedule{
		NextCommonDropSeconds: 10,
		NextStageDropSeconds:  20,
		CommonDropObserved:    true,
		StageDropObserved:     true,
	})

	if !estimate.CommonDropObserved || !estimate.StageDropObserved {
		t.Fatalf("drop observed flags = common %t stage %t, want true/true", estimate.CommonDropObserved, estimate.StageDropObserved)
	}
	if estimate.TimeToFullSeconds != 1837 {
		t.Fatalf("time to full = %d, want 1837 seconds", estimate.TimeToFullSeconds)
	}
}

func TestBuildInventoryFillEstimatePreservesReadyDropSchedule(t *testing.T) {
	snapshot := playerdata.InventorySnapshot{
		InventorySlots: playerdata.InventorySlotSummary{Total: 20, Used: 19, Free: 1},
		ChestAutomation: playerdata.ChestAutomation{
			CommonRuneReductionSeconds: 39,
			StageRuneReductionSeconds:  75,
			RuneResolved:               true,
			HasCommonAutoOpenRune:      true,
			HasStageAutoOpenRune:       true,
		},
	}

	estimate := BuildInventoryFillEstimate(snapshot, ChestDropSchedule{
		NextCommonDropSeconds: 0,
		NextStageDropSeconds:  360,
		CommonDropObserved:    true,
		CommonDropReady:       true,
	})

	if !estimate.CommonDropReady || estimate.NextCommonDropSeconds != 0 {
		t.Fatalf("common ready state = ready %t next %d, want true/0", estimate.CommonDropReady, estimate.NextCommonDropSeconds)
	}
	if estimate.TimeToFullSeconds != 261 {
		t.Fatalf("time to full = %d, want ready common drop to open after 261 seconds", estimate.TimeToFullSeconds)
	}
}

func TestBuildInventoryFillEstimateUsesDropPaceWhenAutoOpenOutpacesDrops(t *testing.T) {
	snapshot := playerdata.InventorySnapshot{
		InventorySlots: playerdata.InventorySlotSummary{Total: 20, Used: 10, Free: 10},
		ChestAutomation: playerdata.ChestAutomation{
			CommonRuneReductionSeconds: 120,
			StageRuneReductionSeconds:  300,
			RuneResolved:               true,
			HasCommonAutoOpenRune:      true,
			HasStageAutoOpenRune:       true,
		},
	}

	estimate := BuildInventoryFillEstimate(snapshot)

	if !estimate.Available {
		t.Fatal("estimate should be available when slot count is known")
	}
	if !estimate.Fills {
		t.Fatalf("estimate fills = false, want true: %+v", estimate)
	}
	if math.Abs(estimate.NetFillPerHour-25) > 0.001 {
		t.Fatalf("net fill per hour = %.4f, want 25", estimate.NetFillPerHour)
	}
	if estimate.TimeToFullSeconds != 1740 {
		t.Fatalf("time to full = %d, want 1740", estimate.TimeToFullSeconds)
	}
}

func TestBuildInventoryFillEstimateCanFillTransientlyBeforeAutoOpen(t *testing.T) {
	snapshot := playerdata.InventorySnapshot{
		InventorySlots: playerdata.InventorySlotSummary{Total: 20, Used: 19, Free: 1},
		ChestAutomation: playerdata.ChestAutomation{
			CommonRuneReductionSeconds: 120,
			StageRuneReductionSeconds:  300,
			RuneResolved:               true,
			HasCommonAutoOpenRune:      true,
			HasStageAutoOpenRune:       true,
		},
	}

	estimate := BuildInventoryFillEstimate(snapshot)

	if !estimate.Fills {
		t.Fatalf("estimate fills = false, want true")
	}
	if estimate.TimeToFullSeconds != 420 {
		t.Fatalf("time to full = %d, want first common auto-open at 420 seconds", estimate.TimeToFullSeconds)
	}
}

func TestBuildInventoryFillEstimateRequiresRunes(t *testing.T) {
	snapshot := playerdata.InventorySnapshot{
		InventorySlots: playerdata.InventorySlotSummary{Total: 20, Used: 10, Free: 10},
		ChestAutomation: playerdata.ChestAutomation{
			CommonRuneReductionSeconds: 39,
			StageRuneReductionSeconds:  75,
			RuneResolved:               true,
			HasCommonAutoOpenRune:      false,
			HasStageAutoOpenRune:       false,
		},
		ChestCounts: playerdata.ChestCounts{NormalChests: 1, StageChests: 1, TotalChests: 2},
	}

	estimate := BuildInventoryFillEstimate(snapshot)

	if estimate.Fills {
		t.Fatal("estimate fills = true, want false since player lacks auto-open runes")
	}
	if estimate.CommonAutoOpenSeconds != 0 || estimate.StageAutoOpenSeconds != 0 {
		t.Fatalf("expected auto-open seconds to be 0 when runes are missing, got common=%d stage=%d",
			estimate.CommonAutoOpenSeconds, estimate.StageAutoOpenSeconds)
	}
}
