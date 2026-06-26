package app

import (
	"fmt"
	"github.com/nidea1/task-bar-trade-center/internal/market"

	"github.com/nidea1/task-bar-trade-center/internal/inventory"
)

func RunRestartAfterUpdateHelper() bool {
	return runRestartAfterUpdateHelper()
}

func RunRestartAfterElevationHelper() bool {
	return runRestartAfterElevationHelper()
}

func GetInventoryDashboard() (inventory.DashboardState, error) {
	state, err := readInventoryDashboardState()
	if err != nil {
		cached := currentInventoryDashboardState()
		if !cached.UpdatedAt.IsZero() {
			cached.Refresh = currentInventoryRefreshStatus()
			return cached, nil
		}
		return inventory.DashboardState{}, err
	}
	storeInventoryDashboardState(state)
	if err := writeInventoryDashboardState(state); err != nil {
		return state, err
	}
	callDashboardUpdated(state)
	return state, nil
}

func RefreshInventoryPrices() (inventory.RefreshStatus, error) {
	state, err := readInventoryDashboardState()
	if err != nil {
		return currentInventoryRefreshStatus(), err
	}
	storeInventoryDashboardState(state)
	queued := queueInventoryPriceRefresh(state)
	if queued == 0 {
		refreshInventoryDashboardState("price-refresh-noop")
		return currentInventoryRefreshStatus(), nil
	}
	refreshInventoryDashboardState("price-refresh-queued")
	return currentInventoryRefreshStatus(), nil
}

func OpenMarketListing(itemID int) error {
	config, exists := activeApp.itemMap[itemID]
	if !exists {
		return fmt.Errorf("market listing is not available")
	}
	openURLInBrowser(steamMarketListingURLForScope(config, market.CurrentScope()))
	return nil
}
