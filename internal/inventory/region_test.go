package inventory

import (
	"github.com/nidea1/task-bar-trade-center/internal/playerdata"
	"testing"
)

func TestInventoryForecastUsesRegionChestTiming(t *testing.T) {
	for _, tt := range []struct {
		region        playerdata.Region
		common, stage int
	}{{playerdata.RegionNormal, 180, 300}, {playerdata.RegionPlaguelands, 240, 360}} {
		snapshot := playerdata.InventorySnapshot{Region: tt.region, InventorySlots: playerdata.InventorySlotSummary{Total: 20, Free: 20}, ChestAutomation: playerdata.ChestAutomation{CommonBaseAutoOpenSeconds: 420, StageBaseAutoOpenSeconds: 720, CommonRuneReductionSeconds: 20, StageRuneReductionSeconds: 30, HasCommonAutoOpenRune: true, HasStageAutoOpenRune: true, RuneResolved: true}}
		estimate := BuildInventoryFillEstimate(snapshot)
		if estimate.Region != tt.region || estimate.CommonDropSeconds != tt.common || estimate.StageDropSeconds != tt.stage || estimate.NextCommonDropSeconds != tt.common || estimate.NextStageDropSeconds != tt.stage {
			t.Fatalf("region %s estimate=%+v", tt.region, estimate)
		}
		if estimate.CommonAutoOpenSeconds != 400 || estimate.StageAutoOpenSeconds != 690 || !estimate.Fills {
			t.Fatalf("region %s did not use its own automation: %+v", tt.region, estimate)
		}
	}
}
