package app

import (
	"github.com/nidea1/task-bar-trade-center/internal/inventory"
	"github.com/nidea1/task-bar-trade-center/internal/playerdata"
	"testing"
	"time"
)

func TestInventoryWarningCrossingDoesNotSpam(t *testing.T) {
	tracker := advancedAlertTracker{}
	settings := defaultDashboardSettings()
	snapshot := playerdata.InventorySnapshot{Region: playerdata.RegionNormal}
	estimate := inventory.InventoryFillEstimate{Available: true, Fills: true, RuneResolved: true}
	for _, tt := range []struct{ eta, want int }{{200, 0}, {190, 0}, {600, 0}, {301, 0}, {300, 1}, {299, 0}, {305, 0}, {300, 0}, {600, 0}, {200, 1}} {
		estimate.TimeToFullSeconds = tt.eta
		if alerts := tracker.evaluate(snapshot, settings, estimate); len(alerts) != tt.want {
			t.Fatalf("eta=%d alerts=%+v want=%d", tt.eta, alerts, tt.want)
		}
	}
	settings.InventoryWarningMinutes = -1
	estimate.TimeToFullSeconds = 100
	if alerts := tracker.evaluate(snapshot, settings, estimate); len(alerts) != 0 {
		t.Fatal("disabled warning fired")
	}
}

func TestTargetAlertsIgnoreStartupAndItemMoves(t *testing.T) {
	tracker := advancedAlertTracker{}
	settings := defaultDashboardSettings()
	settings.NotificationTargets = []NotificationTarget{{ItemID: 100, Count: 2}}
	snapshot := playerdata.InventorySnapshot{Items: []playerdata.OwnedItem{{ItemID: 100, UniqueID: 1, Location: playerdata.LocationInventory}}}
	if alerts := tracker.evaluate(snapshot, settings, inventory.InventoryFillEstimate{}); len(alerts) != 0 {
		t.Fatal("startup alert")
	}
	// The same item visible in inventory and stash must not satisfy a target.
	snapshot.Items = append(snapshot.Items, playerdata.OwnedItem{ItemID: 100, UniqueID: 1, Location: playerdata.LocationStash})
	if alerts := tracker.evaluate(snapshot, settings, inventory.InventoryFillEstimate{}); len(alerts) != 0 {
		t.Fatal("duplicate/moved item satisfied target")
	}
	snapshot.Items = append(snapshot.Items, playerdata.OwnedItem{ItemID: 100, UniqueID: 2, Location: playerdata.LocationStash})
	if alerts := tracker.evaluate(snapshot, settings, inventory.InventoryFillEstimate{}); len(alerts) != 1 || alerts[0].itemID != 100 {
		t.Fatalf("target alerts=%+v", alerts)
	}
	if alerts := tracker.evaluate(snapshot, settings, inventory.InventoryFillEstimate{}); len(alerts) != 0 {
		t.Fatal("repeated target alert")
	}
	settings.NotificationTargets[0].Count = 1
	if alerts := tracker.evaluate(snapshot, settings, inventory.InventoryFillEstimate{}); len(alerts) != 0 {
		t.Fatal("editing an already satisfied target caused an acquisition alert")
	}
}

func TestForecastTimersExpireAndDoNotMutateSnapshot(t *testing.T) {
	now := time.Now()
	snapshot := playerdata.InventorySnapshot{ReadAt: now, ChestTimers: playerdata.ChestTimers{Common: playerdata.ChestTimer{Available: true, Enabled: true, RemainingSeconds: 20}}}
	got := forecastSnapshot(snapshot, now.Add(5*time.Second))
	if got.ChestTimers.Common.RemainingSeconds != 15 || snapshot.ChestTimers.Common.RemainingSeconds != 20 {
		t.Fatal("timer age or snapshot immutability failed")
	}
	if got := forecastSnapshot(snapshot, now.Add(20*time.Second)); got.ChestTimers.Common.Available {
		t.Fatal("stale timer remained live")
	}
}

func TestNotificationPreferencesNormalize(t *testing.T) {
	got := normalizeDashboardSettings(DashboardSettings{NotificationTargets: []NotificationTarget{{100, 2}, {100, 3}, {0, 1}, {200, -1}}})
	if got.InventoryWarningMinutes != 5 || len(got.NotificationTargets) != 1 || got.NotificationTargets[0].Count != 2 {
		t.Fatalf("settings=%+v", got)
	}
}

func TestTargetCountsUseStackQuantity(t *testing.T) {
	snapshot := playerdata.InventorySnapshot{Items: []playerdata.OwnedItem{
		{ItemID: 100, UniqueID: 1, Quantity: 25, Location: playerdata.LocationMaterialStash},
		{ItemID: 100, UniqueID: 1, Location: playerdata.LocationInventory},
		{ItemID: 100, UniqueID: 2, Quantity: 3, Location: playerdata.LocationInventory},
	}}
	if got := ownedItemCounts(snapshot)[100]; got != 28 {
		t.Fatalf("quantity=%d want 28", got)
	}
}
