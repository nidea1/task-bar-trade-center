package app

import "github.com/nidea1/task-bar-trade-center/internal/inventory"

type App struct{}

func New(callbacks Callbacks) *App {
	SetCallbacks(callbacks)
	return &App{}
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
