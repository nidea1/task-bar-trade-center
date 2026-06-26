package app

import (
	"github.com/nidea1/task-bar-trade-center/internal/market"

	"encoding/json"
	"fmt"
	"os"

	filestore "github.com/nidea1/task-bar-trade-center/internal/storage"
)

func initAppStorage() {
	paths, file, err := filestore.Init(AppName, AppProcessName, AppVersion, AppCreatorName)
	if err != nil {
		fmt.Printf("Application storage could not be initialized: %v\n", err)
		return
	}

	activeApp.appDataDir = paths.AppDataDir
	activeApp.logFilePath = paths.LogFilePath
	activeApp.priceCacheFilePath = paths.PriceCacheFilePath
	activeApp.inventoryStateFilePath = paths.InventoryStateFilePath
	activeApp.settingsFilePath = paths.SettingsFilePath
	activeApp.gameLayoutCacheFilePath = paths.GameLayoutCacheFilePath
	activeApp.appLogFile = file
}

func closeAppStorage() {
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
	OverlayModeSetting int32  `json:"overlay_mode"`
	MarketCurrencyCode string `json:"market_currency"`
	MarketCountry      string `json:"market_country"`
	DisplayLanguage    string `json:"display_language"`
}

func loadSettingsFromDisk() {
	if activeApp.settingsFilePath == "" {
		return
	}

	var settings AppSettings
	if err := filestore.ReadJSON(activeApp.settingsFilePath, &settings); err != nil {
		if os.IsNotExist(err) {
			applyDisplayLanguagePreference(displayLanguageSystem)
			saveSettingsToDisk()
		} else {
			fmt.Printf("Settings file could not be read: %v\n", err)
		}
		return
	}

	activeApp.overlayMode.Store(settings.OverlayModeSetting)
	scope := market.ScopeFromSettings(settings.MarketCurrencyCode, settings.MarketCountry)
	market.SetScope(scope.Currency.Code, scope.Region.CountryCode)
	applyDisplayLanguagePreference(settings.DisplayLanguage)
	fmt.Printf("Settings loaded from disk: overlayMode=%d market=%s language=%s\n", settings.OverlayModeSetting, market.FormatScope(scope), currentDisplayLanguage())
}

func saveSettingsToDisk() {
	if activeApp.settingsFilePath == "" {
		return
	}

	scope := market.CurrentScope()
	settings := AppSettings{
		OverlayModeSetting: activeApp.overlayMode.Load(),
		MarketCurrencyCode: scope.Currency.Code,
		MarketCountry:      scope.Region.CountryCode,
		DisplayLanguage:    currentDisplayLanguagePreference(),
	}

	if err := filestore.WriteJSON(activeApp.settingsFilePath, settings); err != nil {
		fmt.Printf("Settings file could not be written: %v\n", err)
	} else {
		fmt.Println("Settings saved to disk.")
	}
}
