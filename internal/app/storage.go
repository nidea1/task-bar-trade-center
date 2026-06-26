package app

import (
	"encoding/json"
	"fmt"
	"os"

	filestore "github.com/nidea1/task-bar-trade-center/internal/storage"
)

var (
	AppDataDir              string
	LogFilePath             string
	PriceCacheFilePath      string
	InventoryStateFilePath  string
	SettingsFilePath        string
	GameLayoutCacheFilePath string
	AppLogFile              *os.File
)

func initAppStorage() {
	paths, file, err := filestore.Init(AppName, AppProcessName, AppVersion, AppCreatorName)
	if err != nil {
		fmt.Printf("Application storage could not be initialized: %v\n", err)
		return
	}

	AppDataDir = paths.AppDataDir
	LogFilePath = paths.LogFilePath
	PriceCacheFilePath = paths.PriceCacheFilePath
	InventoryStateFilePath = paths.InventoryStateFilePath
	SettingsFilePath = paths.SettingsFilePath
	GameLayoutCacheFilePath = paths.GameLayoutCacheFilePath
	AppLogFile = file
}

func closeAppStorage() {
	filestore.Close(AppLogFile, AppName)
	AppLogFile = nil
}

func loadPriceCacheFromDisk() int {
	if PriceCacheFilePath == "" {
		return 0
	}

	bytes, err := os.ReadFile(PriceCacheFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			PriceCacheMu.Lock()
			writePriceCacheFileLocked()
			PriceCacheMu.Unlock()
			fmt.Println("Price cache file created.")
		} else {
			fmt.Printf("Price cache file could not be read: %v\n", err)
		}
		return 0
	}

	var diskCache map[string]MarketData
	if err := json.Unmarshal(bytes, &diskCache); err != nil {
		fmt.Printf("Price cache file could not be parsed: %v\n", err)
		return 0
	}

	normalizedCache, migrated := normalizePriceCache(diskCache)
	PriceCacheMu.Lock()
	for cacheKey, data := range normalizedCache {
		PriceCache[cacheKey] = data
	}
	if migrated {
		writePriceCacheFileLocked()
	}
	count := len(PriceCache)
	PriceCacheMu.Unlock()

	fmt.Printf("Price cache loaded from disk: %d item(s).\n", count)
	return count
}

func normalizePriceCache(diskCache map[string]MarketData) (map[string]MarketData, bool) {
	normalized := make(map[string]MarketData, len(diskCache))
	legacy := make(map[string]MarketData)
	for cacheKey, data := range diskCache {
		if cacheKey == "" || data.CachedAt.IsZero() {
			continue
		}
		if _, _, ok := parseMarketCacheKey(cacheKey); ok {
			normalized[cacheKey] = data
			continue
		}
		legacy[cacheKey] = data
	}

	for marketHashName, data := range legacy {
		cacheKey := marketCacheKey(defaultMarketScope(), marketHashName)
		if _, exists := normalized[cacheKey]; !exists {
			normalized[cacheKey] = data
		}
	}
	return normalized, len(legacy) > 0
}

func writePriceCacheFileLocked() {
	if PriceCacheFilePath == "" {
		return
	}

	if err := filestore.WriteJSON(PriceCacheFilePath, PriceCache); err != nil {
		fmt.Printf("Price cache file could not be written: %v\n", err)
	}
}

type AppSettings struct {
	OverlayMode     int32  `json:"overlay_mode"`
	MarketCurrency  string `json:"market_currency"`
	MarketCountry   string `json:"market_country"`
	DisplayLanguage string `json:"display_language"`
}

func loadSettingsFromDisk() {
	if SettingsFilePath == "" {
		return
	}

	var settings AppSettings
	if err := filestore.ReadJSON(SettingsFilePath, &settings); err != nil {
		if os.IsNotExist(err) {
			applyDisplayLanguagePreference(displayLanguageSystem)
			saveSettingsToDisk()
		} else {
			fmt.Printf("Settings file could not be read: %v\n", err)
		}
		return
	}

	OverlayMode.Store(settings.OverlayMode)
	scope := marketScopeFromSettings(settings.MarketCurrency, settings.MarketCountry)
	setMarketScope(scope.Currency.Code, scope.Region.CountryCode)
	applyDisplayLanguagePreference(settings.DisplayLanguage)
	fmt.Printf("Settings loaded from disk: overlayMode=%d market=%s language=%s\n", settings.OverlayMode, formatMarketScope(scope), currentDisplayLanguage())
}

func saveSettingsToDisk() {
	if SettingsFilePath == "" {
		return
	}

	scope := currentMarketScope()
	settings := AppSettings{
		OverlayMode:     OverlayMode.Load(),
		MarketCurrency:  scope.Currency.Code,
		MarketCountry:   scope.Region.CountryCode,
		DisplayLanguage: currentDisplayLanguagePreference(),
	}

	if err := filestore.WriteJSON(SettingsFilePath, settings); err != nil {
		fmt.Printf("Settings file could not be written: %v\n", err)
	} else {
		fmt.Println("Settings saved to disk.")
	}
}
