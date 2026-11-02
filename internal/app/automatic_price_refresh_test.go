package app

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/catalog"
	"github.com/nidea1/task-bar-trade-center/internal/inventory"
	"github.com/nidea1/task-bar-trade-center/internal/market"
	"github.com/nidea1/task-bar-trade-center/internal/playerdata"
)

func TestAutomaticRefreshDoesNotRefillRunningBatch(t *testing.T) {
	originalApp := activeApp
	fetched := make(chan int, 4)
	release := make(chan struct{})
	queue := inventory.NewRefreshQueue(func(_ context.Context, id int) error {
		fetched <- id
		if id == 2 {
			<-release
		}
		return nil
	}, nil)
	queue.SetBaseDelay(0)
	activeApp = &App{inventoryPriceQueue: queue}
	t.Cleanup(func() { activeApp = originalApp })
	state := inventory.DashboardState{Items: []inventory.DashboardItem{{ItemID: 1}, {ItemID: 2}}}
	if added := queueAutomaticInventoryPriceRefresh(state); added != 2 {
		t.Fatalf("initial automatic batch added %d items, want 2", added)
	}
	for range 2 {
		select {
		case <-fetched:
		case <-time.After(time.Second):
			close(release)
			t.Fatal("batch did not start")
		}
	}
	for range 5 {
		if added := queueAutomaticInventoryPriceRefresh(state); added != 0 {
			close(release)
			t.Fatalf("periodic poll re-added %d completed items", added)
		}
	}
	status := queue.Status()
	if status.Completed != 1 || status.Queued != 1 {
		close(release)
		t.Fatalf("batch counters grew during periodic polls: %+v", status)
	}
	manual := inventory.DashboardState{Items: []inventory.DashboardItem{{ItemID: 3}}}
	if added := queueInventoryPriceRefresh(manual); added != 1 {
		close(release)
		t.Fatalf("explicit refresh could not add a new item: %d", added)
	}
	close(release)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		status = queue.Status()
		if !status.Refreshing && status.Queued == 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if status.Refreshing || status.Queued != 0 || status.Completed != 3 {
		t.Fatalf("batch failed to drain: %+v", status)
	}
	select {
	case id := <-fetched:
		if id != 3 {
			t.Fatalf("completed item fetched twice: %d", id)
		}
	default:
		t.Fatal("explicitly added item was not fetched")
	}
}

func TestRUBSearchFallbackReachesInventoryCards(t *testing.T) {
	originalApp, originalScope, originalFetcher := activeApp, market.CurrentScope(), fetchMarketDataFromSteam
	config := resolvedTestItem(catalog.ItemConfig{ID: 142001, Name: map[string]string{"en-US": "Silver Ingot"}, Grade: "RARE", Type: "MATERIAL", Marketable: true})
	market.SetScope("RUB", "RU")
	snapshot := playerdata.InventorySnapshot{ReadAt: time.Now(), Items: []playerdata.OwnedItem{{ItemID: config.ID, UniqueID: 1, Quantity: 5, Location: playerdata.LocationInventory, Marketable: true}}}
	activeApp = &App{itemMap: map[int]catalog.ItemConfig{config.ID: config}, allItemMap: map[int]catalog.ItemConfig{config.ID: config}, priceCache: make(map[string]market.MarketData), lastSnapshot: &snapshot}
	t.Cleanup(func() {
		flushInventoryDashboardRebuildNow()
		activeApp = originalApp
		market.SetScope(originalScope.Currency.Code, originalScope.Region.CountryCode)
		fetchMarketDataFromSteam = originalFetcher
	})
	fetchMarketDataFromSteam = func(item catalog.ItemConfig, name string, now time.Time, scope market.MarketScope, _ market.RequestPriority) (market.MarketData, error) {
		if scope.Currency.Code != "RUB" || scope.Region.CountryCode != "RU" {
			t.Fatal("fetch lost the selected RUB scope")
		}
		return market.MarketData{CachedAt: now, Analysis: market.MarketAnalysis{SteamDefinitionID: item.SteamDefinitionID, MarketHashName: name, UpdatedAt: now, ExchangeRateDate: now, HasSuggested: true, SuggestedPrice: 2.4, PriceSuffix: " ₽", PriceIsEstimated: true, PriceSource: "market-search-html-usd"}}, nil
	}
	if err := fetchInventoryMarketPrice(context.Background(), config.ID); err != nil {
		t.Fatal(err)
	}
	state := buildInventoryDashboardState(snapshot)
	if len(state.Items) != 1 || !state.Items[0].HasPrice || !state.Items[0].PriceIsEstimated || math.Abs(state.Items[0].Suggested-2.4) > 0.0001 || math.Abs(state.Items[0].TotalSuggested-12) > 0.0001 || state.Totals.PricedItemCount != 5 {
		t.Fatalf("fetched RUB quote was not displayed in inventory cards: %+v", state)
	}
}
