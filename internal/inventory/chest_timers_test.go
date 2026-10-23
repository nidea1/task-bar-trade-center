package inventory

import (
	"github.com/nidea1/task-bar-trade-center/internal/playerdata"
	"testing"
)

func TestForecastUsesRemainingOpenTimeAndDisabledAutomation(t *testing.T) {
	snapshot := playerdata.InventorySnapshot{
		Region:          playerdata.RegionNormal,
		InventorySlots:  playerdata.InventorySlotSummary{Total: 10, Used: 9, Free: 1},
		ChestAutomation: playerdata.ChestAutomation{HasCommonAutoOpenRune: true, RuneResolved: true},
		ChestCounts:     playerdata.ChestCounts{NormalChests: 1, TotalChests: 1},
		ChestTimers:     playerdata.ChestTimers{Common: playerdata.ChestTimer{Available: true, Enabled: true, DurationSeconds: 300, RemainingSeconds: 37.2}},
	}
	got := BuildInventoryFillEstimate(snapshot)
	if !got.CommonOpenObserved || got.NextCommonOpenSeconds != 38 || got.TimeToFullSeconds != 38 || got.CommonAutoOpenSeconds != 300 {
		t.Fatalf("live forecast=%+v", got)
	}
	snapshot.ChestTimers.Common.Enabled = false
	got = BuildInventoryFillEstimate(snapshot)
	if got.Fills || got.CommonAutoOpenSeconds != 0 || got.CommonOpenObserved {
		t.Fatalf("disabled auto-open forecast=%+v", got)
	}
	snapshot.ChestTimers.Common.Available = false
	got = BuildInventoryFillEstimate(snapshot)
	if !got.Fills || got.TimeToFullSeconds != 300 || got.CommonOpenObserved {
		t.Fatalf("unavailable timer should retain estimated interval: %+v", got)
	}
	snapshot.ChestTimers.Common = playerdata.ChestTimer{Available: true, Enabled: true, DurationSeconds: 300, RemainingSeconds: 37}
	snapshot.ChestCounts = playerdata.ChestCounts{}
	got = BuildInventoryFillEstimate(snapshot)
	if got.CommonOpenObserved || got.TimeToFullSeconds != 480 {
		t.Fatalf("empty queue must wait for a drop and full opening interval: %+v", got)
	}
}
