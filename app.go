package main

import (
	"context"
	"sync"

	core "github.com/nidea1/task-bar-trade-center/internal/app"
	"github.com/nidea1/task-bar-trade-center/internal/inventory"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	mu   sync.RWMutex
	ctx  context.Context
	core *core.App
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.mu.Lock()
	a.ctx = ctx
	a.mu.Unlock()

	appCore := core.New(core.Callbacks{
		OpenDashboard: a.showDashboard,
		Quit:          a.quit,
		DashboardUpdated: func(state inventory.DashboardState) {
			runtime.EventsEmit(ctx, "inventory-dashboard-updated", state)
		},
	})
	a.mu.Lock()
	a.core = appCore
	a.mu.Unlock()
	go appCore.Run()
}

func (a *App) shutdown(context.Context) {
	if appCore := a.coreApp(); appCore != nil {
		appCore.Stop()
	}
}

func (a *App) showDashboard() {
	ctx, ok := a.context()
	if !ok {
		return
	}
	runtime.WindowShow(ctx)
	runtime.WindowUnminimise(ctx)
}

func (a *App) quit() {
	ctx, ok := a.context()
	if !ok {
		return
	}
	runtime.Quit(ctx)
}

func (a *App) GetInventoryDashboard() (inventory.DashboardState, error) {
	if appCore := a.coreApp(); appCore != nil {
		return appCore.GetInventoryDashboard()
	}
	return inventory.DashboardState{}, nil
}

func (a *App) RefreshInventoryPrices() (inventory.RefreshStatus, error) {
	if appCore := a.coreApp(); appCore != nil {
		return appCore.RefreshInventoryPrices()
	}
	return inventory.RefreshStatus{}, nil
}

func (a *App) OpenMarketListing(itemID int) error {
	if appCore := a.coreApp(); appCore != nil {
		return appCore.OpenMarketListing(itemID)
	}
	return nil
}

func (a *App) context() (context.Context, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.ctx == nil {
		return nil, false
	}
	return a.ctx, true
}

func (a *App) coreApp() *core.App {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.core
}
