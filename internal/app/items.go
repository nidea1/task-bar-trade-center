package app

import (
	"fmt"
	"path/filepath"

	"github.com/nidea1/task-bar-trade-center/internal/catalog"
	"github.com/nidea1/task-bar-trade-center/internal/game"
)

func loadItemsJSON() error {
	db, err := catalog.LoadItems()
	if err != nil {
		fmt.Printf("%v\n", err)
		return err
	}
	status := catalog.Status{State: "embedded", ItemCount: len(db)}
	if activeApp.appDataDir != "" {
		if cached, cachedStatus, err := catalog.LoadCached(filepath.Join(activeApp.appDataDir, "cache", "item-catalog.json"), db); err == nil {
			db, status = cached, cachedStatus
		}
	}
	installCatalog(db, status)
	fmt.Printf("Database loaded: %d marketable items active.\n", len(marketCatalogItems()))
	return nil
}

// Published maps are immutable. Readers retain a safe snapshot while a new
// catalog is installed when the game reconnects after an update.
func installCatalog(items []catalog.ItemConfig, status catalog.Status) {
	all, marketable := make(map[int]catalog.ItemConfig, len(items)), make(map[int]catalog.ItemConfig)
	for _, item := range items {
		all[item.ID] = item
		if item.Marketable {
			marketable[item.ID] = item
		}
	}
	activeApp.catalogMu.Lock()
	activeApp.allItemMap, activeApp.itemMap, activeApp.catalogStatus = all, marketable, status
	activeApp.catalogMu.Unlock()
}
func allCatalogItems() map[int]catalog.ItemConfig {
	activeApp.catalogMu.RLock()
	defer activeApp.catalogMu.RUnlock()
	return activeApp.allItemMap
}
func marketCatalogItems() map[int]catalog.ItemConfig {
	activeApp.catalogMu.RLock()
	defer activeApp.catalogMu.RUnlock()
	return activeApp.itemMap
}
func currentCatalogStatus() catalog.Status {
	activeApp.catalogMu.RLock()
	defer activeApp.catalogMu.RUnlock()
	return activeApp.catalogStatus
}

func refreshInstalledCatalog(processHandle uintptr) {
	path := game.ModuleFilePath(processHandle, GameProcessName)
	if path == "" {
		return
	}
	base := make([]catalog.ItemConfig, 0, len(allCatalogItems()))
	for _, item := range allCatalogItems() {
		base = append(base, item)
	}
	items, status, err := catalog.RefreshInstalled(filepath.Join(filepath.Dir(path), "TaskBarHero_Data"), filepath.Join(activeApp.appDataDir, "cache", "item-catalog.json"), base)
	if err != nil {
		logPrintf("[CATALOG] Refresh failed; retaining current catalog: %v\n", err)
		activeApp.catalogMu.Lock()
		activeApp.catalogStatus.State = "failed"
		activeApp.catalogStatus.Message = err.Error()
		activeApp.catalogMu.Unlock()
		return
	}
	installCatalog(items, status)
	logPrintf("[CATALOG] state=%s items=%d added=%d\n", status.State, status.ItemCount, status.Added)
}
