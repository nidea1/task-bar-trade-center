package app

import (
	"github.com/nidea1/task-bar-trade-center/internal/catalog"
	"github.com/nidea1/task-bar-trade-center/internal/inventory"
	"github.com/nidea1/task-bar-trade-center/internal/market"
)

func New(callbacks Callbacks) *App {
	SetCallbacks(callbacks)
	activeApp = &App{
		callbacks:                 callbacks,
		allItemMap:                make(map[int]catalog.ItemConfig),
		itemMap:                   make(map[int]catalog.ItemConfig),
		priceCache:                make(map[string]market.MarketData),
		iconMetadata:              make(map[string]iconMetadataEntry),
		notificationIconCache:     make(map[string]uintptr),
		notificationIconPreparing: make(map[string]struct{}),
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

func (app *App) ForceRefreshInventoryPrices() (inventory.RefreshStatus, error) {
	return ForceRefreshInventoryPrices()
}

func (app *App) OpenMarketListing(itemID int) error {
	return OpenMarketListing(itemID)
}

func (app *App) GetDisplayLanguages() []LanguageInfo {
	return GetDisplayLanguages()
}

func (app *App) GetMarketCurrencies() []CurrencyInfo {
	return GetMarketCurrencies()
}

func (app *App) GetMarketRegions() []RegionInfo {
	return GetMarketRegions()
}

func (app *App) GetCurrentLanguage() string {
	return GetCurrentLanguage()
}

func (app *App) GetCurrentMarketScope() CurrentMarketScopeInfo {
	return GetCurrentMarketScope()
}

func (app *App) GetDashboardFooterInfo() DashboardFooterInfo {
	return GetDashboardFooterInfo()
}

func (app *App) GetMinRarityNotify() string {
	return GetMinRarityNotify()
}

func (app *App) SetDisplayLanguage(preference string) bool {
	return SetDisplayLanguage(preference)
}

func (app *App) SetMinRarityNotify(grade string) bool {
	return SetMinRarityNotify(grade)
}

func (app *App) SetMarketScope(currencyCode string, countryCode string) bool {
	return SetMarketScope(currencyCode, countryCode)
}

func (app *App) GetTranslations() map[string]string {
	return GetTranslations()
}
