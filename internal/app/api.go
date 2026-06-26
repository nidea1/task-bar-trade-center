package app

import (
	"fmt"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/inventory"
	"github.com/nidea1/task-bar-trade-center/internal/market"
)

const inventoryDashboardPollCacheMaxAge = 15 * time.Second

func RunRestartAfterUpdateHelper() bool {
	return runRestartAfterUpdateHelper()
}

func RunRestartAfterElevationHelper() bool {
	return runRestartAfterElevationHelper()
}

func GetInventoryDashboard() (inventory.DashboardState, error) {
	if cached, ok := freshInventoryDashboardCache(inventoryDashboardPollCacheMaxAge); ok {
		return cached, nil
	}

	cached := currentInventoryDashboardState()
	if canReadInventorySnapshot() {
		go refreshInventoryDashboardState("dashboard-cache-miss")
	}
	if cached.UpdatedAt != "" {
		return withCurrentDashboardRuntimeFields(cached), nil
	}
	return currentInventoryDashboardShellState(), nil
}

func RefreshInventoryPrices() (inventory.RefreshStatus, error) {
	state, err := readInventoryDashboardStateLocked()
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

func freshInventoryDashboardCache(maxAge time.Duration) (inventory.DashboardState, bool) {
	cached := currentInventoryDashboardState()
	if cached.UpdatedAt == "" {
		return inventory.DashboardState{}, false
	}
	updatedAt, err := time.Parse(time.RFC3339, cached.UpdatedAt)
	if err != nil || time.Since(updatedAt) > maxAge {
		return inventory.DashboardState{}, false
	}
	return withCurrentDashboardRuntimeFields(cached), true
}

func OpenMarketListing(itemID int) error {
	config, exists := activeApp.itemMap[itemID]
	if !exists {
		return fmt.Errorf("market listing is not available")
	}
	openURLInBrowser(steamMarketListingURLForScope(config, market.CurrentScope()))
	return nil
}

type LanguageInfo struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type CurrencyInfo struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type RegionInfo struct {
	CountryCode  string `json:"country_code"`
	Name         string `json:"name"`
	CurrencyCode string `json:"currency_code"`
}

type CurrentMarketScopeInfo struct {
	CurrencyCode string `json:"currency_code"`
	CountryCode  string `json:"country_code"`
}

func GetDisplayLanguages() []LanguageInfo {
	locales := supportedAppLocales
	list := make([]LanguageInfo, len(locales))
	for i, l := range locales {
		list[i] = LanguageInfo{
			Code: l.Code,
			Name: l.Name,
		}
	}
	return list
}

func GetMarketCurrencies() []CurrencyInfo {
	currencies := supportedMarketCurrencies
	list := make([]CurrencyInfo, len(currencies))
	for i, c := range currencies {
		list[i] = CurrencyInfo{
			Code: c.Code,
			Name: c.Code,
		}
	}
	return list
}

func GetMarketRegions() []RegionInfo {
	regions := supportedMarketRegions
	list := make([]RegionInfo, len(regions))
	for i, r := range regions {
		list[i] = RegionInfo{
			CountryCode:  r.CountryCode,
			Name:         r.Name,
			CurrencyCode: r.CurrencyCode,
		}
	}
	return list
}

func GetCurrentLanguage() string {
	return currentDisplayLanguagePreference()
}

func GetCurrentMarketScope() CurrentMarketScopeInfo {
	scope := market.CurrentScope()
	return CurrentMarketScopeInfo{
		CurrencyCode: scope.Currency.Code,
		CountryCode:  scope.Region.CountryCode,
	}
}

func SetDisplayLanguage(preference string) bool {
	return selectDisplayLanguage(preference)
}

func SetMarketScope(currencyCode string, countryCode string) bool {
	scope, changed, selected := market.SelectRegion(currencyCode, countryCode)
	if selected && changed {
		fmt.Printf("Market region changed via dashboard to %s.\n", market.FormatScope(scope))
		saveSettingsToDisk()
		refreshActiveMarketPrice()
		if state, ok := rebuildDashboardState("region-changed"); ok {
			queued := queueInventoryPriceRefresh(state)
			if queued > 0 {
				refreshInventoryDashboardState("region-price-refresh-queued")
			}
		}
		return true
	}
	return false
}

func GetTranslations() map[string]string {
	return currentTranslations()
}
