package app

import (
	"github.com/nidea1/task-bar-trade-center/internal/playerdata"
	"math"
	"time"
)

func forecastSnapshot(snapshot playerdata.InventorySnapshot, now time.Time) playerdata.InventorySnapshot {
	age := now.Sub(snapshot.ReadAt).Seconds()
	if age < 0 || age > 15 {
		snapshot.ChestTimers = playerdata.ChestTimers{}
		return snapshot
	}
	if snapshot.ChestTimers.Common.Enabled {
		snapshot.ChestTimers.Common.RemainingSeconds = math.Max(0, snapshot.ChestTimers.Common.RemainingSeconds-age)
	}
	if snapshot.ChestTimers.Stage.Enabled {
		snapshot.ChestTimers.Stage.RemainingSeconds = math.Max(0, snapshot.ChestTimers.Stage.RemainingSeconds-age)
	}
	return snapshot
}
