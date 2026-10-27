package app

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/nidea1/task-bar-trade-center/internal/catalog"
	"github.com/nidea1/task-bar-trade-center/internal/game"
	"github.com/nidea1/task-bar-trade-center/internal/steamdefs"
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
	activeID := int(activeApp.activeItemID.Load())
	previous := activeApp.itemMap[activeID]
	activeApp.allItemMap, activeApp.itemMap, activeApp.catalogStatus = all, marketable, status
	activeApp.catalogMu.Unlock()
	current := marketable[activeID]
	if activeID > 0 && (previous.SteamDefinitionID != current.SteamDefinitionID || previous.MarketHashName != current.MarketHashName) {
		setCurrentPriceText(tr("hud.loading"))
		redrawOverlay()
	}
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
	activeApp.catalogRefreshMu.Lock()
	defer activeApp.catalogRefreshMu.Unlock()
	base := make([]catalog.ItemConfig, 0, len(allCatalogItems()))
	for _, item := range allCatalogItems() {
		base = append(base, item)
	}
	path := game.ModuleFilePath(processHandle, GameProcessName)
	if path == "" {
		status := currentCatalogStatus()
		status.State, status.Message = "failed", "Game installation path unavailable; Steam identities could not be refreshed"
		installCatalog(catalog.ApplyMarketDefinitions(base, nil), status)
		return
	}
	dataDir := filepath.Join(filepath.Dir(path), "TaskBarHero_Data")
	cachePath := filepath.Join(activeApp.appDataDir, "cache", "item-catalog.json")
	items, status, err := catalog.RefreshInstalled(dataDir, cachePath, base)
	if err != nil {
		logPrintf("[CATALOG] Refresh failed; retaining current catalog: %v\n", err)
		items, status = base, currentCatalogStatus()
		status.State, status.Message = "failed", err.Error()
	}
	ids := make([]int, 0, len(items))
	for _, item := range items {
		if id, ok := catalog.MarketDefinitionID(item.ID); item.Marketable && ok {
			ids = append(ids, id)
		}
	}
	definitions, definitionErr := steamdefs.Read(context.Background(), filepath.Join(dataDir, "Plugins", "x86_64", "steam_api64.dll"), ids)
	items = catalog.ApplyMarketDefinitions(items, definitions)
	if definitionErr != nil {
		status.State = "failed"
		status.Message = fmt.Sprintf("Steam market definitions unavailable: %v", definitionErr)
		logPrintf("[CATALOG] %s\n", status.Message)
	}
	if activeApp.gameProcessHandle != processHandle || hasGameProcessExited(processHandle) {
		return
	}
	if err == nil {
		if saveErr := catalog.SaveMarketIdentities(cachePath, items); saveErr != nil {
			logPrintf("[CATALOG] Save identities: %v\n", saveErr)
		}
	}
	installCatalog(items, status)
	logPrintf("[CATALOG] state=%s items=%d added=%d\n", status.State, status.ItemCount, status.Added)
}

func refreshMarketDefinitionsForCurrentGame() {
	if canReadInventorySnapshot() {
		refreshInstalledCatalog(activeApp.gameProcessHandle)
	}
}
