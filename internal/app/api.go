package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/inventory"
	"github.com/nidea1/task-bar-trade-center/internal/market"
	"github.com/nidea1/task-bar-trade-center/internal/win32"
)

const inventoryDashboardPollCacheMaxAge = 2 * time.Second

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
	if cached.UpdatedAt != "" {
		return withCurrentDashboardRuntimeFields(cached), nil
	}
	if runtimeReady() && canReadInventorySnapshot() {
		go refreshInventoryDashboardState("dashboard-cache-miss")
	}
	return currentInventoryDashboardShellState(), nil
}

func RefreshInventoryPrices() (inventory.RefreshStatus, error) {
	if !runtimeReady() {
		return currentInventoryRefreshStatus(), fmt.Errorf("runtime is still preparing")
	}
	state, err := readInventoryDashboardStateLocked()
	if err != nil {
		return currentInventoryRefreshStatus(), err
	}
	storeInventoryDashboardState(state)
	queued := queueInventoryPriceRefresh(state)
	if queued == 0 {
		publishInventoryDashboardState(state, "price-refresh-noop")
		return currentInventoryRefreshStatus(), nil
	}
	publishInventoryDashboardState(state, "price-refresh-queued")
	return currentInventoryRefreshStatus(), nil
}

func ForceRefreshInventoryPrices() (inventory.RefreshStatus, error) {
	if !runtimeReady() {
		return currentInventoryRefreshStatus(), fmt.Errorf("runtime is still preparing")
	}
	state, err := readInventoryDashboardStateLocked()
	if err != nil {
		return currentInventoryRefreshStatus(), err
	}
	storeInventoryDashboardState(state)
	queued := queueForceInventoryPriceRefresh(state)
	if queued == 0 {
		publishInventoryDashboardState(state, "force-price-refresh-noop")
		return currentInventoryRefreshStatus(), nil
	}
	publishInventoryDashboardState(state, "force-price-refresh-queued")
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

type DashboardFooterInfo struct {
	AppName         string `json:"app_name"`
	AppShortName    string `json:"app_short_name"`
	Version         string `json:"version"`
	CreatorName     string `json:"creator_name"`
	UpdateStatus    int32  `json:"update_status"`
	UpdateText      string `json:"update_text"`
	UpdateAvailable bool   `json:"update_available"`
	ReleaseURL      string `json:"release_url"`
}

type DashboardSettings struct {
	ThemeMode           string `json:"theme_mode"`
	PriceMode           string `json:"price_mode"`
	RarityFilter        string `json:"rarity_filter"`
	EquipmentFilter     string `json:"equipment_filter"`
	SortMode            string `json:"sort_mode"`
	BestRarityFilter    string `json:"best_rarity_filter"`
	BestEquipmentFilter string `json:"best_equipment_filter"`
	BestOwnershipFilter string `json:"best_ownership_filter"`
	BestSortMode        string `json:"best_sort_mode"`
	MarketableItemsTab  string `json:"marketable_items_tab"`
	NotifySources       string `json:"notify_sources"`
	HotkeyModifiers     int    `json:"hotkey_modifiers"`
	HotkeyVK            int    `json:"hotkey_vk"`
	GameScale           int32  `json:"game_scale"`
}

const (
	notificationSourceBox       = "box"
	notificationSourceCraft     = "craft"
	notificationSourceSynthesis = "synthesis"
	notificationSourceOffering  = "offering"
	notificationSourcesNone     = "none"
	notificationSourcesAll      = "box,craft,synthesis,offering"
)

var notificationSourceOrder = []string{
	notificationSourceBox,
	notificationSourceCraft,
	notificationSourceSynthesis,
	notificationSourceOffering,
}

func defaultDashboardSettings() DashboardSettings {
	return DashboardSettings{
		ThemeMode:           "dark",
		PriceMode:           "suggested",
		RarityFilter:        "all",
		EquipmentFilter:     "all",
		SortMode:            "price_desc",
		BestRarityFilter:    "all",
		BestEquipmentFilter: "all",
		BestOwnershipFilter: "all",
		BestSortMode:        "score_desc",
		MarketableItemsTab:  "all",
		NotifySources:       notificationSourcesAll,
		HotkeyModifiers:     0,
		HotkeyVK:            VK_F2,
		GameScale:           GameScale100,
	}
}

func normalizeDashboardSettings(settings DashboardSettings) DashboardSettings {
	normalized := defaultDashboardSettings()
	switch settings.ThemeMode {
	case "dark", "light":
		normalized.ThemeMode = settings.ThemeMode
	}
	switch settings.PriceMode {
	case "suggested", "instant":
		normalized.PriceMode = settings.PriceMode
	}
	if settings.RarityFilter != "" {
		normalized.RarityFilter = settings.RarityFilter
	}
	if settings.EquipmentFilter != "" {
		normalized.EquipmentFilter = settings.EquipmentFilter
	}
	switch settings.SortMode {
	case "price_desc", "price_asc", "name_asc", "count_desc", "rarity_desc":
		normalized.SortMode = settings.SortMode
	}
	if settings.BestRarityFilter != "" {
		normalized.BestRarityFilter = settings.BestRarityFilter
	}
	if settings.BestEquipmentFilter != "" {
		normalized.BestEquipmentFilter = settings.BestEquipmentFilter
	}
	switch settings.BestOwnershipFilter {
	case "all", "equipped", "unequipped":
		normalized.BestOwnershipFilter = settings.BestOwnershipFilter
	}
	switch settings.BestSortMode {
	case "score_desc", "score_asc", "price_desc", "price_asc", "name_asc", "rarity_desc":
		normalized.BestSortMode = settings.BestSortMode
	}
	switch settings.MarketableItemsTab {
	case "best", "all":
		normalized.MarketableItemsTab = settings.MarketableItemsTab
	}
	normalized.NotifySources = normalizeNotificationSources(settings.NotifySources)
	if settings.HotkeyVK != 0 {
		normalized.HotkeyModifiers = settings.HotkeyModifiers
		normalized.HotkeyVK = settings.HotkeyVK
	}
	normalized.GameScale = normalizeGameScale(settings.GameScale)
	return normalized
}

func normalizeNotificationSources(sources string) string {
	if strings.TrimSpace(sources) == "" {
		return notificationSourcesAll
	}
	enabled := make(map[string]struct{})
	for _, token := range strings.Split(sources, ",") {
		token = strings.TrimSpace(strings.ToLower(token))
		if token == notificationSourcesNone {
			return notificationSourcesNone
		}
		for _, known := range notificationSourceOrder {
			if token == known {
				enabled[known] = struct{}{}
				break
			}
		}
	}
	if len(enabled) == 0 {
		return notificationSourcesNone
	}
	ordered := make([]string, 0, len(enabled))
	for _, source := range notificationSourceOrder {
		if _, ok := enabled[source]; ok {
			ordered = append(ordered, source)
		}
	}
	return strings.Join(ordered, ",")
}

func notificationSourceEnabled(source string) bool {
	settings := currentDashboardSettings()
	if settings.NotifySources == notificationSourcesNone {
		return false
	}
	for _, token := range strings.Split(settings.NotifySources, ",") {
		if strings.TrimSpace(token) == source {
			return true
		}
	}
	return false
}

func currentDashboardSettings() DashboardSettings {
	activeApp.dashboardSettingsMu.RLock()
	defer activeApp.dashboardSettingsMu.RUnlock()
	return normalizeDashboardSettings(activeApp.dashboardSettings)
}

func setDashboardSettings(settings DashboardSettings) DashboardSettings {
	normalized := normalizeDashboardSettings(settings)

	activeApp.dashboardSettingsMu.Lock()
	oldModifiers := activeApp.dashboardSettings.HotkeyModifiers
	oldVK := activeApp.dashboardSettings.HotkeyVK
	activeApp.dashboardSettings = normalized
	activeApp.dashboardSettingsMu.Unlock()

	if normalized.HotkeyModifiers != oldModifiers || normalized.HotkeyVK != oldVK {
		if activeApp.appHWND != 0 {
			win32.ProcPostMessageW.Call(activeApp.appHWND, WM_APP_HOTKEY_UPDATE, 0, 0)
		}
	}

	selectGameScale(normalized.GameScale)

	saveSettingsToDisk()
	return normalized
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

func GetDashboardFooterInfo() DashboardFooterInfo {
	_, releaseURL := updateActionURLs()
	status := activeApp.updateStatus.Load()
	return DashboardFooterInfo{
		AppName:         AppName,
		AppShortName:    AppShortName,
		Version:         AppVersion,
		CreatorName:     AppCreatorName,
		UpdateStatus:    status,
		UpdateText:      updateStatusText(),
		UpdateAvailable: status == UpdateStatusAvailable,
		ReleaseURL:      releaseURL,
	}
}

func InstallAvailableUpdate() bool {
	return installAvailableUpdate()
}

func GetDashboardSettings() DashboardSettings {
	return currentDashboardSettings()
}

func SetDashboardSettings(settings DashboardSettings) DashboardSettings {
	return setDashboardSettings(settings)
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

func GetMinRarityNotify() string {
	return rarityGrade(int(activeApp.minRarityNotifyLevel.Load()))
}

func SetMinRarityNotify(grade string) bool {
	level := rarityLevel(grade)
	activeApp.minRarityNotifyLevel.Store(int32(level))
	saveSettingsToDisk()
	fmt.Printf("Minimum notification rarity changed via dashboard to %s.\n", grade)
	return true
}
