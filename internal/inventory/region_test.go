package inventory

import (
	"github.com/nidea1/task-bar-trade-center/internal/playerdata"
	"math"
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

func TestPlagueRuneReductionsChangeInventoryFillTime(t *testing.T) {
	for _, tt := range []struct {
		name                  string
		common, stage         int
		commonOpen, stageOpen bool
		wantCommon, wantStage int
		wantFillSeconds       int
		wantRate              float64
	}{
		{"unlock only", 0, 0, true, true, 600, 1200, 1800, 9},
		{"max new runes", 60, 120, true, true, 540, 1080, 1620, 10},
		{"only normal chest unlocked", 60, 120, true, false, 540, 0, 2160, 3600.0 / 540},
		{"locked", 60, 120, false, false, 0, 0, 0, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := playerdata.InventorySnapshot{
				Region:         playerdata.RegionPlaguelands,
				InventorySlots: playerdata.InventorySlotSummary{Total: 4, Free: 4},
				ChestCounts:    playerdata.ChestCounts{NormalChests: 1, StageChests: 1, TotalChests: 2},
				ChestAutomation: playerdata.ChestAutomation{
					CommonBaseAutoOpenSeconds: 600, StageBaseAutoOpenSeconds: 1200,
					CommonRuneReductionSeconds: tt.common, StageRuneReductionSeconds: tt.stage,
					HasCommonAutoOpenRune: tt.commonOpen, HasStageAutoOpenRune: tt.stageOpen, RuneResolved: true,
				},
			}
			got := BuildInventoryFillEstimate(snapshot)
			if got.CommonDropSeconds != 240 || got.StageDropSeconds != 360 || got.CommonAutoOpenSeconds != tt.wantCommon || got.StageAutoOpenSeconds != tt.wantStage {
				t.Fatalf("incorrect Plague timing: %+v", got)
			}
			if got.TimeToFullSeconds != tt.wantFillSeconds || got.Fills != (tt.wantFillSeconds > 0) || math.Abs(got.NetFillPerHour-tt.wantRate) > 0.0001 {
				t.Fatalf("fill=%d rate=%f, want %d/%f", got.TimeToFullSeconds, got.NetFillPerHour, tt.wantFillSeconds, tt.wantRate)
			}
		})
	}
}
