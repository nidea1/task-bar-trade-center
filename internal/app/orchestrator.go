package app

import (
	"github.com/nidea1/task-bar-trade-center/internal/catalog"
	"github.com/nidea1/task-bar-trade-center/internal/inventory"
	"github.com/nidea1/task-bar-trade-center/internal/market"
)

func New(callbacks Callbacks) *App {
	activeApp = &App{
		callbacks:  callbacks,
		allItemMap: make(map[int]catalog.ItemConfig),
		itemMap:    make(map[int]catalog.ItemConfig),
		priceCache: make(map[string]market.MarketData),
	}
	return activeApp
}

func (app *App) Run() {
	Run()
}

func (app *App) Stop() {
	Stop()
}

func (app *App) GetInventoryDashboard() (inventory.DashboardState, error) {
	return GetInventoryDashboard()
}

func (app *App) RefreshInventoryPrices() (inventory.RefreshStatus, error) {
	return RefreshInventoryPrices()
}

func (app *App) OpenMarketListing(itemID int) error {
	return OpenMarketListing(itemID)
}
