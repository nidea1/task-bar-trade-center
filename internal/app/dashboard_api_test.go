package app

import (
	"testing"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/catalog"
	"github.com/nidea1/task-bar-trade-center/internal/inventory"
	"github.com/nidea1/task-bar-trade-center/internal/market"
)

func TestGetInventoryDashboardReturnsShellStateWithoutInventory(t *testing.T) {
	originalApp := activeApp
	originalScope := market.CurrentScope()
	activeApp = &App{
		allItemMap: make(map[int]catalog.ItemConfig),
		itemMap:    make(map[int]catalog.ItemConfig),
		priceCache: make(map[string]market.MarketData),
	}
	t.Cleanup(func() {
		activeApp = originalApp
		market.SetScope(originalScope.Currency.Code, originalScope.Region.CountryCode)
	})

	state, err := GetInventoryDashboard()
	if err != nil {
		t.Fatalf("GetInventoryDashboard returned error: %v", err)
	}
	if state.UpdatedAt != "" {
		t.Fatalf("shell state UpdatedAt = %q, want empty", state.UpdatedAt)
	}
	if state.Translations == nil {
		t.Fatal("shell state translations are nil")
	}
	if state.CurrencyCode == "" || state.MarketScope == "" {
		t.Fatalf("shell state missing market scope: %+v", state)
	}
}

func TestGetInventoryDashboardReturnsStaleCachedStateWithoutBlocking(t *testing.T) {
	originalApp := activeApp
	originalScope := market.CurrentScope()
	activeApp = &App{
		allItemMap: make(map[int]catalog.ItemConfig),
		itemMap:    make(map[int]catalog.ItemConfig),
		priceCache: make(map[string]market.MarketData),
		inventoryDashboardState: inventory.DashboardState{
			UpdatedAt: time.Now().Add(-time.Hour).Format(time.RFC3339),
			Gold:      42,
		},
	}
	t.Cleanup(func() {
		activeApp = originalApp
		market.SetScope(originalScope.Currency.Code, originalScope.Region.CountryCode)
	})

	state, err := GetInventoryDashboard()
	if err != nil {
		t.Fatalf("GetInventoryDashboard returned error: %v", err)
	}
	if state.Gold != 42 {
		t.Fatalf("cached state Gold = %d, want 42", state.Gold)
	}
	if state.Translations == nil {
		t.Fatal("cached state translations are nil")
	}
}
