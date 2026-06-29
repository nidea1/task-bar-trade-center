package app

import (
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/catalog"
	"github.com/nidea1/task-bar-trade-center/internal/market"
)

var hoverPriceFetchDebounce = 500 * time.Millisecond

func scheduleHoveredPriceFetch(config catalog.ItemConfig, itemID int32, scope market.MarketScope) {
	version := activeApp.tooltipFetchVersion.Add(1)

	go func() {
		time.Sleep(hoverPriceFetchDebounce)
		if activeApp.tooltipFetchVersion.Load() != version {
			return
		}
		if activeApp.activeItemID.Load() != itemID || !activeApp.showOverlay.Load() || market.CurrentScope() != scope {
			return
		}
		prioritizeActiveInventoryPriceRefresh(itemID)
		fetchPriceAndUpdateWithOptions(config, true, scope, market.RequestPriorityHigh, false)
	}()
}

func cancelHoveredPriceFetch() {
	activeApp.tooltipFetchVersion.Add(1)
}

func prioritizeActiveInventoryPriceRefresh(itemID int32) {
	if itemID <= 0 {
		return
	}

	activeApp.inventoryMu.Lock()
	queue := activeApp.inventoryPriceQueue
	activeApp.inventoryMu.Unlock()
	if queue == nil {
		return
	}

	status := queue.Status()
	if !status.Refreshing && status.Queued == 0 {
		return
	}
	queue.EnqueuePriority([]int{int(itemID)})
}
