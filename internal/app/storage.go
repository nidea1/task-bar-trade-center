package app

import (
	"github.com/nidea1/task-bar-trade-center/internal/market"

	"encoding/json"
	"fmt"
	"os"
	"time"

	filestore "github.com/nidea1/task-bar-trade-center/internal/storage"
)

func initAppStorage() {
	saveOriginalStdout()
	paths, file, err := filestore.Init(AppName, AppProcessName, AppVersion, AppCreatorName)
	if err != nil {
		fmt.Printf("Application storage could not be initialized: %v\n", err)
		return
	}

	activeApp.appDataDir = paths.AppDataDir
	activeApp.logFilePath = paths.LogFilePath
	activeApp.priceCacheFilePath = paths.PriceCacheFilePath
	activeApp.iconMetadataFilePath = paths.IconMetadataFilePath
	activeApp.inventoryStateFilePath = paths.InventoryStateFilePath
	activeApp.inventoryResolverCacheFilePath = paths.InventoryResolverCacheFilePath
	activeApp.settingsFilePath = paths.SettingsFilePath
	activeApp.gameLayoutCacheFilePath = paths.GameLayoutCacheFilePath
	activeApp.appLogFile = file

	initLogger(file)
}

func closeAppStorage() {
	flushInventoryDashboardRebuildNow()
	flushCacheWritesNow()
	filestore.Close(activeApp.appLogFile, AppName)
	activeApp.appLogFile = nil
}

func loadPriceCacheFromDisk() int {
	if activeApp.priceCacheFilePath == "" {
		return 0
	}

	bytes, err := os.ReadFile(activeApp.priceCacheFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			activeApp.priceCacheMu.Lock()
			writePriceCacheFileLocked()
			activeApp.priceCacheMu.Unlock()
			fmt.Println("Price cache file created.")
		} else {
			fmt.Printf("Price cache file could not be read: %v\n", err)
		}
		return 0
	}

	var diskCache map[string]market.MarketData
	if err := json.Unmarshal(bytes, &diskCache); err != nil {
		fmt.Printf("Price cache file could not be parsed: %v\n", err)
		return 0
	}

	normalizedCache, migrated := normalizePriceCache(diskCache)
	activeApp.priceCacheMu.Lock()
	for cacheKey, data := range normalizedCache {
		activeApp.priceCache[cacheKey] = data
	}
	if migrated {
		writePriceCacheFileLocked()
	}
	count := len(activeApp.priceCache)
	activeApp.priceCacheMu.Unlock()

	fmt.Printf("Price cache loaded from disk: %d item(s).\n", count)
	return count
}

func normalizePriceCache(diskCache map[string]market.MarketData) (map[string]market.MarketData, bool) {
	normalized := make(map[string]market.MarketData, len(diskCache))
	legacy := make(map[string]market.MarketData)
	for cacheKey, data := range diskCache {
		if cacheKey == "" || data.CachedAt.IsZero() {
			continue
		}
		if _, _, ok := market.ParseCacheKey(cacheKey); ok {
			normalized[cacheKey] = data
			continue
		}
		legacy[cacheKey] = data
	}

	for marketHashName, data := range legacy {
		cacheKey := market.CacheKey(market.DefaultScope(), marketHashName)
		if _, exists := normalized[cacheKey]; !exists {
			normalized[cacheKey] = data
		}
	}
	return normalized, len(legacy) > 0
}

func writePriceCacheFileLocked() {
	if activeApp.priceCacheFilePath == "" {
		return
	}

	if err := filestore.WriteJSON(activeApp.priceCacheFilePath, activeApp.priceCache); err != nil {
		fmt.Printf("Price cache file could not be written: %v\n", err)
	}
}

type AppSettings struct {
	OverlayModeSetting int32             `json:"overlay_mode"`
	GameScalePercent   int32             `json:"game_scale_percent"`
	MarketCurrencyCode string            `json:"market_currency"`
	MarketCountry      string            `json:"market_country"`
	DisplayLanguage    string            `json:"display_language"`
	MinRarityNotify    string            `json:"min_rarity_notify"`
	Dashboard          DashboardSettings `json:"dashboard"`
}

func markSettingsReady() {
	activeApp.settingsReadyOnce.Do(func() {
		if activeApp.settingsReadyCh != nil {
			close(activeApp.settingsReadyCh)
		}
	})
}

func waitForSettingsReady(timeout time.Duration) bool {
	if activeApp.settingsReadyCh == nil {
		return true
	}
	if timeout <= 0 {
		select {
		case <-activeApp.settingsReadyCh:
			return true
		default:
			return false
		}
	}
	select {
	case <-activeApp.settingsReadyCh:
		return true
	case <-time.After(timeout):
		return false
	}
}

func rarityLevel(grade string) int {
	switch grade {
	case "COMMON":
		return 0
	case "UNCOMMON":
		return 1
	case "RARE":
		return 2
	case "LEGENDARY":
		return 3
	case "IMMORTAL":
		return 4
	case "ARCANA":
		return 5
	case "BEYOND":
		return 6
	case "CELESTIAL":
		return 7
	case "DIVINE":
		return 8
	case "COSMIC":
		return 9
	default:
		return 0
	}
}

func rarityGrade(level int) string {
	switch level {
	case 0:
		return "COMMON"
	case 1:
		return "UNCOMMON"
	case 2:
		return "RARE"
	case 3:
		return "LEGENDARY"
	case 4:
		return "IMMORTAL"
	case 5:
		return "ARCANA"
	case 6:
		return "BEYOND"
	case 7:
		return "CELESTIAL"
	case 8:
		return "DIVINE"
	case 9:
		return "COSMIC"
	default:
		return "COMMON"
	}
}

func loadSettingsFromDisk() {
	defer markSettingsReady()

	if activeApp.settingsFilePath == "" {
		return
	}

	var settings AppSettings
	if err := filestore.ReadJSON(activeApp.settingsFilePath, &settings); err != nil {
		if os.IsNotExist(err) {
			applyDisplayLanguagePreference(displayLanguageSystem)
			selectedGameScale.Store(GameScale100)
			saveSettingsToDisk()
		} else {
			fmt.Printf("Settings file could not be read: %v\n", err)
		}
		return
	}

	activeApp.overlayMode.Store(settings.OverlayModeSetting)
	selectedGameScale.Store(normalizeGameScale(settings.GameScalePercent))

	scope := market.ScopeFromSettings(settings.MarketCurrencyCode, settings.MarketCountry)
	market.SetScope(scope.Currency.Code, scope.Region.CountryCode)
	applyDisplayLanguagePreference(settings.DisplayLanguage)
	shouldMigrateLanguage := settings.DisplayLanguage == "" || settings.DisplayLanguage == displayLanguageSystem || !supportedDisplayLanguage(settings.DisplayLanguage)

	minRarity := settings.MinRarityNotify
	if minRarity == "" {
		minRarity = "COMMON"
	}
	activeApp.minRarityNotifyLevel.Store(int32(rarityLevel(minRarity)))
	activeApp.dashboardSettingsMu.Lock()
	settings.Dashboard.GameScale = normalizeGameScale(settings.GameScalePercent)
	activeApp.dashboardSettings = normalizeDashboardSettings(settings.Dashboard)
	activeApp.dashboardSettingsMu.Unlock()

	fmt.Printf(
		"Settings loaded from disk: overlayMode=%d gameScale=%s market=%s language=%s minRarity=%s dashboard=%+v\n",
		settings.OverlayModeSetting,
		gameScaleLabel(currentGameScale()),
		market.FormatScope(scope),
		currentDisplayLanguage(),
		minRarity,
		currentDashboardSettings(),
	)
	if shouldMigrateLanguage {
		saveSettingsToDisk()
	}
}

func saveSettingsToDisk() {
	if activeApp.settingsFilePath == "" {
		return
	}

	scope := market.CurrentScope()
	dashSettings := currentDashboardSettings()
	dashSettings.GameScale = currentGameScale()

	settings := AppSettings{
		OverlayModeSetting: activeApp.overlayMode.Load(),
		GameScalePercent:   currentGameScale(),
		MarketCurrencyCode: scope.Currency.Code,
		MarketCountry:      scope.Region.CountryCode,
		DisplayLanguage:    persistedDisplayLanguagePreference(),
		MinRarityNotify:    rarityGrade(int(activeApp.minRarityNotifyLevel.Load())),
		Dashboard:          dashSettings,
	}

	if err := filestore.WriteJSONAtomic(activeApp.settingsFilePath, settings); err != nil {
		fmt.Printf("Settings file could not be written: %v\n", err)
	} else {
		fmt.Println("Settings saved to disk.")
	}
}

func persistedDisplayLanguagePreference() string {
	preference := currentDisplayLanguagePreference()
	if preference == "" || preference == displayLanguageSystem || !supportedDisplayLanguage(preference) {
		return currentDisplayLanguage()
	}
	return preference
}
