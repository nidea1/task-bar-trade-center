package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/catalog"
	"github.com/nidea1/task-bar-trade-center/internal/inventory"
	"github.com/nidea1/task-bar-trade-center/internal/market"
)

func TestInventoryCatalogUsesGlobalIconMetadataAcrossScopes(t *testing.T) {
	originalApp := activeApp
	originalScope := market.CurrentScope()
	item := catalog.ItemConfig{
		ID:         100,
		Name:       map[string]string{"en-US": "Test Sword"},
		Grade:      "LEGENDARY",
		Type:       "GEAR",
		Marketable: true,
	}
	marketHashName := buildMarketHashName(item)
	activeApp = &App{
		allItemMap: map[int]catalog.ItemConfig{item.ID: item},
		itemMap:    map[int]catalog.ItemConfig{item.ID: item},
		priceCache: make(map[string]market.MarketData),
		iconMetadata: map[string]iconMetadataEntry{
			marketHashName: {IconPath: "global-icon", LastFetchedAt: time.Now()},
		},
	}
	market.SetScope("EUR", "DE")
	t.Cleanup(func() {
		activeApp = originalApp
		market.SetScope(originalScope.Currency.Code, originalScope.Region.CountryCode)
	})

	catalog := inventoryItemCatalog(market.CurrentScope())
	if got := catalog[item.ID].IconURL; got != steamIconImageURL("global-icon") {
		t.Fatalf("dashboard icon URL = %q, want global icon", got)
	}
}

func TestRecordMarketIconUpdatesGlobalMetadata(t *testing.T) {
	originalApp := activeApp
	now := time.Now()
	activeApp = &App{
		iconMetadata:              make(map[string]iconMetadataEntry),
		notificationIconPreparing: map[string]struct{}{"fresh-icon": {}},
	}
	t.Cleanup(func() {
		activeApp = originalApp
	})

	recordMarketIcon("Example Hash", "fresh-icon", now)

	entry := activeApp.iconMetadata["Example Hash"]
	if entry.IconPath != "fresh-icon" || !entry.LastFetchedAt.Equal(now) {
		t.Fatalf("icon metadata = %+v, want fresh icon at %s", entry, now)
	}
}

func TestMissingOrStaleDashboardItemIDsUsesIconMetadataFreshness(t *testing.T) {
	originalApp := activeApp
	now := time.Now()
	activeApp = &App{
		iconMetadata: map[string]iconMetadataEntry{
			"Fresh": {IconPath: "fresh-icon", LastFetchedAt: now},
			"Stale": {IconPath: "stale-icon", LastFetchedAt: now.Add(-8 * 24 * time.Hour)},
		},
	}
	t.Cleanup(func() {
		activeApp = originalApp
	})

	state := inventory.DashboardState{
		Items: []inventory.DashboardItem{
			{ItemID: 100, MarketHashName: "Fresh", HasPrice: true, UpdatedAt: now.Format(time.RFC3339), IconURL: steamIconImageURL("fresh-icon")},
			{ItemID: 200, MarketHashName: "Stale", HasPrice: true, UpdatedAt: now.Format(time.RFC3339), IconURL: steamIconImageURL("stale-icon")},
		},
	}

	ids := missingOrStaleDashboardItemIDs(state, time.Hour)
	if len(ids) != 1 || ids[0] != 200 {
		t.Fatalf("refresh ids = %+v, want [200]", ids)
	}
}

func TestQueueForceInventoryPriceRefreshQueuesAllMarketableDashboardItems(t *testing.T) {
	originalApp := activeApp
	fetched := make(chan int, 2)
	queue := inventory.NewRefreshQueue(func(_ context.Context, id int) error {
		fetched <- id
		return nil
	}, nil)
	queue.SetBaseDelay(0)
	activeApp = &App{inventoryPriceQueue: queue}
	t.Cleanup(func() {
		activeApp = originalApp
	})

	state := inventory.DashboardState{
		Items: []inventory.DashboardItem{
			{ItemID: 100},
			{ItemID: 200},
			{ItemID: 100},
		},
	}
	if added := queueForceInventoryPriceRefresh(state); added != 2 {
		t.Fatalf("force refresh added = %d, want 2", added)
	}

	got := []int{<-fetched, <-fetched}
	if got[0] != 100 || got[1] != 200 {
		t.Fatalf("fetched = %+v, want [100 200]", got)
	}
}

func TestFlushPriceCacheWriteNowPersistsPendingWrite(t *testing.T) {
	originalApp := activeApp
	cachePath := filepath.Join(t.TempDir(), "price-cache.json")
	activeApp = &App{
		priceCacheFilePath: cachePath,
		priceCache: map[string]market.MarketData{
			"USD|Example": {CachedAt: time.Now()},
		},
	}
	t.Cleanup(func() {
		activeApp = originalApp
	})

	activeApp.priceCacheMu.Lock()
	schedulePriceCacheWriteLocked()
	activeApp.priceCacheMu.Unlock()
	flushPriceCacheWriteNow()

	raw, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("price cache was not written: %v", err)
	}
	if !strings.Contains(string(raw), "USD|Example") {
		t.Fatalf("price cache file = %s, want cache key", raw)
	}
}
