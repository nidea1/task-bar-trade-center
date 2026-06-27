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

func TestMissingOrStaleDashboardItemIDsIncludesMissingIcons(t *testing.T) {
	now := time.Now()
	state := inventory.DashboardState{
		Items: []inventory.DashboardItem{
			{ItemID: 100, HasPrice: true, UpdatedAt: now.Format(time.RFC3339), IconURL: "https://example.invalid/icon.png"},
			{ItemID: 200, HasPrice: true, UpdatedAt: now.Format(time.RFC3339)},
			{ItemID: 300, IconURL: "https://example.invalid/icon.png"},
		},
	}

	ids := missingOrStaleDashboardItemIDs(state, time.Hour)
	if len(ids) != 3 || ids[0] != 100 || ids[1] != 200 || ids[2] != 300 {
		t.Fatalf("refresh ids = %+v, want [100 200 300]", ids)
	}
}

func TestRetainCachedIconURL(t *testing.T) {
	data := retainCachedIconURL(
		market.MarketData{Analysis: market.MarketAnalysis{}},
		market.MarketData{Analysis: market.MarketAnalysis{IconURL: "cached-icon"}},
		true,
	)
	if data.Analysis.IconURL != "cached-icon" {
		t.Fatalf("icon = %q, want cached-icon", data.Analysis.IconURL)
	}

	data = retainCachedIconURL(
		market.MarketData{Analysis: market.MarketAnalysis{IconURL: "fresh-icon"}},
		market.MarketData{Analysis: market.MarketAnalysis{IconURL: "cached-icon"}},
		true,
	)
	if data.Analysis.IconURL != "fresh-icon" {
		t.Fatalf("icon = %q, want fresh-icon", data.Analysis.IconURL)
	}
}
