package storage

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

type Paths struct {
	AppDataDir              string
	LogFilePath             string
	PriceCacheFilePath      string
	IconMetadataFilePath    string
	InventoryStateFilePath  string
	SettingsFilePath        string
	GameLayoutCacheFilePath string
}

func Init(appName, processName, version, creator string) (Paths, *os.File, error) {
	baseDir, err := os.UserCacheDir()
	if err != nil {
		baseDir = "."
	}

	paths := Paths{}
	paths.AppDataDir = filepath.Join(baseDir, appName)
	logDir := filepath.Join(paths.AppDataDir, "logs")
	cacheDir := filepath.Join(paths.AppDataDir, "cache")
	configDir := filepath.Join(paths.AppDataDir, "config")

	for _, dir := range []string{logDir, cacheDir, configDir} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return paths, nil, err
		}
	}

	paths.LogFilePath = filepath.Join(logDir, processName+".log")
	paths.PriceCacheFilePath = filepath.Join(cacheDir, "price-cache.json")
	paths.IconMetadataFilePath = filepath.Join(cacheDir, "icon-metadata-cache.json")
	paths.InventoryStateFilePath = filepath.Join(cacheDir, "inventory-dashboard-state.json")
	paths.SettingsFilePath = filepath.Join(configDir, "settings.json")
	paths.GameLayoutCacheFilePath = filepath.Join(configDir, "game-layout-cache.json")

	if info, err := os.Stat(paths.LogFilePath); err == nil && info.Size() > 5*1024*1024 {
		_ = os.Remove(paths.LogFilePath)
	}

	file, err := os.OpenFile(paths.LogFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return paths, nil, err
	}

	os.Stdout = file
	os.Stderr = file
	log.SetOutput(file)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)

	fmt.Printf("\n[%s] %s started\n", time.Now().Format(time.RFC3339), appName)
	fmt.Printf("Version: %s\n", version)
	fmt.Printf("Created by: %s\n", creator)
	fmt.Printf("Log file: %s\n", paths.LogFilePath)
	fmt.Printf("Price cache file: %s\n", paths.PriceCacheFilePath)
	fmt.Printf("Icon metadata cache file: %s\n", paths.IconMetadataFilePath)
	fmt.Printf("Inventory dashboard state file: %s\n", paths.InventoryStateFilePath)
	fmt.Printf("Game layout cache file: %s\n", paths.GameLayoutCacheFilePath)
	fmt.Printf("Runtime: go=%s os=%s arch=%s pid=%d\n", runtime.Version(), runtime.GOOS, runtime.GOARCH, os.Getpid())
	if workingDir, err := os.Getwd(); err == nil {
		fmt.Printf("Working directory: %s\n", workingDir)
	}

	return paths, file, nil
}

func Close(file *os.File, appName string) {
	if file == nil {
		return
	}
	fmt.Printf("[%s] %s stopped\n", time.Now().Format(time.RFC3339), appName)
	_ = file.Close()
}

func ReadJSON(path string, target any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}

func WriteJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0600)
}

func WriteJSONAtomic(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
