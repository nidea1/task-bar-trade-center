package app

import (
	"context"
	"testing"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/catalog"
	"github.com/nidea1/task-bar-trade-center/internal/inventory"
	"github.com/nidea1/task-bar-trade-center/internal/market"
	"github.com/nidea1/task-bar-trade-center/internal/playerdata"
)

func TestSetMarketScopeQueuesInventoryRefresh(t *testing.T) {
	originalApp := activeApp
	originalScope := market.CurrentScope()
	defer func() {
		activeApp = originalApp
		market.SetScope(originalScope.Currency.Code, originalScope.Region.CountryCode)
	}()

	fetched := make(chan int, 1)
	queue := inventory.NewRefreshQueue(func(_ context.Context, id int) error {
		fetched <- id
		return nil
	}, nil)
	queue.SetBaseDelay(0)

	item := catalog.ItemConfig{
		ID:         100,
		Name:       map[string]string{"en-US": "Test Sword"},
		Grade:      "LEGENDARY",
		Type:       "GEAR",
		Marketable: true,
	}
	now := time.Unix(1700000000, 0)
	snapshot := &playerdata.InventorySnapshot{
		ReadAt: now,
		Items: []playerdata.OwnedItem{
			{ItemID: item.ID, UniqueID: 1, Location: playerdata.LocationInventory, Marketable: true},
		},
	}

	activeApp = &App{
		allItemMap:          map[int]catalog.ItemConfig{item.ID: item},
		itemMap:             map[int]catalog.ItemConfig{item.ID: item},
		priceCache:          make(map[string]market.MarketData),
		lastSnapshot:        snapshot,
		inventoryPriceQueue: queue,
	}
	market.SetScope("USD", "US")

	if !SetMarketScope("EUR", "DE") {
		t.Fatal("SetMarketScope returned false")
	}

	select {
	case got := <-fetched:
		if got != item.ID {
			t.Fatalf("queued item = %d, want %d", got, item.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("inventory refresh was not queued: %+v", queue.Status())
	}
	if state := currentInventoryDashboardState(); state.CurrencyCode != "EUR" {
		t.Fatalf("dashboard currency = %q, want EUR", state.CurrencyCode)
	}
}
