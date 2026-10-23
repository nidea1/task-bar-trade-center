package app

import (
	"github.com/nidea1/task-bar-trade-center/internal/catalog"
	"github.com/nidea1/task-bar-trade-center/internal/playerdata"
	"testing"
	"time"
)

func TestPriceRebuildRetainsCatalogAndTargetProgress(t *testing.T) {
	previous := activeApp
	t.Cleanup(func() { activeApp = previous })
	settings := defaultDashboardSettings()
	settings.NotificationTargets = []NotificationTarget{{100, 3}}
	activeApp = &App{dashboardSettings: settings, lastSnapshot: &playerdata.InventorySnapshot{ReadAt: time.Now(), Items: []playerdata.OwnedItem{{ItemID: 100, UniqueID: 1, Quantity: 2}}}}
	installCatalog([]catalog.ItemConfig{{ID: 100, Name: map[string]string{"en-US": "Ruby"}, Grade: "RARE", Type: "MATERIAL"}}, catalog.Status{State: "updated", ItemCount: 1})
	state, ok := rebuildDashboardState("test-price-refresh")
	if !ok || state.Catalog.ItemCount != 1 || len(state.NotificationTargets) != 1 || state.NotificationTargets[0].Count != 2 {
		t.Fatalf("rebuilt runtime fields lost: catalog=%+v targets=%+v", state.Catalog, state.NotificationTargets)
	}
}
