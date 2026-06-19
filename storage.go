package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

var (
	AppDataDir              string
	LogFilePath             string
	PriceCacheFilePath      string
	SettingsFilePath        string
	GameLayoutCacheFilePath string
	AppLogFile              *os.File
)

func initAppStorage() {
	baseDir, err := os.UserCacheDir()
	if err != nil {
		baseDir = "."
	}

	AppDataDir = filepath.Join(baseDir, AppName)
	logDir := filepath.Join(AppDataDir, "logs")
	cacheDir := filepath.Join(AppDataDir, "cache")
	configDir := filepath.Join(AppDataDir, "config")

	if err := os.MkdirAll(logDir, 0700); err != nil {
		fmt.Printf("Log directory could not be created: %v\n", err)
		return
	}
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		fmt.Printf("Cache directory could not be created: %v\n", err)
		return
	}
	if err := os.MkdirAll(configDir, 0700); err != nil {
		fmt.Printf("Config directory could not be created: %v\n", err)
		return
	}

	LogFilePath = filepath.Join(logDir, AppProcessName+".log")
	PriceCacheFilePath = filepath.Join(cacheDir, "price-cache.json")
	SettingsFilePath = filepath.Join(configDir, "settings.json")
	GameLayoutCacheFilePath = filepath.Join(configDir, "game-layout-cache.json")

	// Limit log file size to 5MB (5 * 1024 * 1024 bytes)
	if info, err := os.Stat(LogFilePath); err == nil && info.Size() > 5*1024*1024 {
		_ = os.Remove(LogFilePath)
	}

	file, err := os.OpenFile(LogFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		fmt.Printf("Log file could not be opened: %v\n", err)
		return
	}

	AppLogFile = file
	os.Stdout = file
	os.Stderr = file
	log.SetOutput(file)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)

	fmt.Printf("\n[%s] %s started\n", time.Now().Format(time.RFC3339), AppName)
	fmt.Printf("Version: %s\n", AppVersion)
	fmt.Printf("Created by: %s\n", AppCreatorName)
	fmt.Printf("Log file: %s\n", LogFilePath)
	fmt.Printf("Price cache file: %s\n", PriceCacheFilePath)
	fmt.Printf("Game layout cache file: %s\n", GameLayoutCacheFilePath)
	fmt.Printf("Runtime: go=%s os=%s arch=%s pid=%d\n", runtime.Version(), runtime.GOOS, runtime.GOARCH, os.Getpid())
	if workingDir, err := os.Getwd(); err == nil {
		fmt.Printf("Working directory: %s\n", workingDir)
	}
}

func closeAppStorage() {
	if AppLogFile == nil {
		return
	}
	fmt.Printf("[%s] %s stopped\n", time.Now().Format(time.RFC3339), AppName)
	AppLogFile.Close()
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

	PriceCacheMu.Lock()
	for marketHashName, data := range diskCache {
		if marketHashName != "" && !data.CachedAt.IsZero() {
			PriceCache[marketHashName] = data
		}
	}
	count := len(PriceCache)
	PriceCacheMu.Unlock()

	fmt.Printf("Price cache loaded from disk: %d item(s).\n", count)
	return count
}

func writePriceCacheFileLocked() {
	if PriceCacheFilePath == "" {
		return
	}

	bytes, err := json.MarshalIndent(PriceCache, "", "  ")
	if err != nil {
		fmt.Printf("Price cache could not be serialized: %v\n", err)
		return
	}

	if err := os.WriteFile(PriceCacheFilePath, bytes, 0600); err != nil {
		fmt.Printf("Price cache file could not be written: %v\n", err)
	}
}

type AppSettings struct {
	OverlayMode int32 `json:"overlay_mode"`
}

func loadSettingsFromDisk() {
	if SettingsFilePath == "" {
		return
	}

	bytes, err := os.ReadFile(SettingsFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			saveSettingsToDisk()
		} else {
			fmt.Printf("Settings file could not be read: %v\n", err)
		}
		return
	}

	var settings AppSettings
	if err := json.Unmarshal(bytes, &settings); err != nil {
		fmt.Printf("Settings file could not be parsed: %v\n", err)
		return
	}

	OverlayMode.Store(settings.OverlayMode)
	fmt.Printf("Settings loaded from disk: overlayMode=%d\n", settings.OverlayMode)
}

func saveSettingsToDisk() {
	if SettingsFilePath == "" {
		return
	}

	settings := AppSettings{
		OverlayMode: OverlayMode.Load(),
	}

	bytes, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		fmt.Printf("Settings could not be serialized: %v\n", err)
		return
	}

	if err := os.WriteFile(SettingsFilePath, bytes, 0600); err != nil {
		fmt.Printf("Settings file could not be written: %v\n", err)
	} else {
		fmt.Println("Settings saved to disk.")
	}
}
