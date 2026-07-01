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
		DashboardFooterUpdated: func(info core.DashboardFooterInfo) {
			runtime.EventsEmit(ctx, "dashboard-footer-updated", info)
		},
		RuntimeStateUpdated: func(info core.RuntimeStateInfo) {
			runtime.EventsEmit(ctx, "runtime-state-updated", info)
		},
	})
	a.mu.Lock()
	a.core = appCore
	a.mu.Unlock()
	core.ApplyDashboardWindowIcon()
	go appCore.Run()
}

func (a *App) domReady(ctx context.Context) {
	a.mu.Lock()
	a.ctx = ctx
	a.mu.Unlock()
	a.showDashboard()
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
	core.ApplyDashboardWindowIcon()
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

func (a *App) ForceRefreshInventoryPrices() (inventory.RefreshStatus, error) {
	if appCore := a.coreApp(); appCore != nil {
		return appCore.ForceRefreshInventoryPrices()
	}
	return inventory.RefreshStatus{}, nil
}

func (a *App) OpenMarketListing(itemID int) error {
	if appCore := a.coreApp(); appCore != nil {
		return appCore.OpenMarketListing(itemID)
	}
	return nil
}

func (a *App) GetDisplayLanguages() ([]core.LanguageInfo, error) {
	if appCore := a.coreApp(); appCore != nil {
		return appCore.GetDisplayLanguages(), nil
	}
	return nil, nil
}

func (a *App) GetMarketCurrencies() ([]core.CurrencyInfo, error) {
	if appCore := a.coreApp(); appCore != nil {
		return appCore.GetMarketCurrencies(), nil
	}
	return nil, nil
}

func (a *App) GetMarketRegions() ([]core.RegionInfo, error) {
	if appCore := a.coreApp(); appCore != nil {
		return appCore.GetMarketRegions(), nil
	}
	return nil, nil
}

func (a *App) GetCurrentLanguage() (string, error) {
	if appCore := a.coreApp(); appCore != nil {
		return appCore.GetCurrentLanguage(), nil
	}
	return "", nil
}

func (a *App) GetCurrentMarketScope() (core.CurrentMarketScopeInfo, error) {
	if appCore := a.coreApp(); appCore != nil {
		return appCore.GetCurrentMarketScope(), nil
	}
	return core.CurrentMarketScopeInfo{}, nil
}

func (a *App) GetDashboardFooterInfo() (core.DashboardFooterInfo, error) {
	if appCore := a.coreApp(); appCore != nil {
		return appCore.GetDashboardFooterInfo(), nil
	}
	return core.DashboardFooterInfo{}, nil
}

func (a *App) GetRuntimeState() (core.RuntimeStateInfo, error) {
	if appCore := a.coreApp(); appCore != nil {
		return appCore.GetRuntimeState(), nil
	}
	return core.RuntimeStateInfo{}, nil
}

func (a *App) InstallAvailableUpdate() (bool, error) {
	if appCore := a.coreApp(); appCore != nil {
		return appCore.InstallAvailableUpdate(), nil
	}
	return false, nil
}

func (a *App) GetDashboardSettings() (core.DashboardSettings, error) {
	if appCore := a.coreApp(); appCore != nil {
		return appCore.GetDashboardSettings(), nil
	}
	return core.DashboardSettings{}, nil
}

func (a *App) GetMinRarityNotify() (string, error) {
	if appCore := a.coreApp(); appCore != nil {
		return appCore.GetMinRarityNotify(), nil
	}
	return "COMMON", nil
}

func (a *App) SetDisplayLanguage(preference string) (bool, error) {
	if appCore := a.coreApp(); appCore != nil {
		return appCore.SetDisplayLanguage(preference), nil
	}
	return false, nil
}

func (a *App) SetMinRarityNotify(grade string) (bool, error) {
	if appCore := a.coreApp(); appCore != nil {
		return appCore.SetMinRarityNotify(grade), nil
	}
	return false, nil
}

func (a *App) SetDashboardSettings(settings core.DashboardSettings) (core.DashboardSettings, error) {
	if appCore := a.coreApp(); appCore != nil {
		return appCore.SetDashboardSettings(settings), nil
	}
	return core.DashboardSettings{}, nil
}

func (a *App) SetMarketScope(currencyCode string, countryCode string) (bool, error) {
	if appCore := a.coreApp(); appCore != nil {
		return appCore.SetMarketScope(currencyCode, countryCode), nil
	}
	return false, nil
}

func (a *App) GetTranslations() (map[string]string, error) {
	if appCore := a.coreApp(); appCore != nil {
		return appCore.GetTranslations(), nil
	}
	return nil, nil
}

func (a *App) DisableDashboardHotkey() error {
	if appCore := a.coreApp(); appCore != nil {
		appCore.DisableDashboardHotkey()
	}
	return nil
}

func (a *App) EnableDashboardHotkey() error {
	if appCore := a.coreApp(); appCore != nil {
		appCore.EnableDashboardHotkey()
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
