package app

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/catalog"
	"github.com/nidea1/task-bar-trade-center/internal/market"
)

// Test fixtures model a successful SDK identity without loading a native SDK.
func resolvedTestItem(item catalog.ItemConfig) catalog.ItemConfig {
	if item.ID == 0 {
		item.ID = 110001
	}
	if item.Type == "" {
		item.Type = "MATERIAL"
	}
	if item.Grade == "" {
		item.Grade = "COMMON"
	}
	name := item.Name["en-US"]
	tags := map[string]string{"type": strings.ToLower(item.Type), "rarity": strings.ToLower(item.Grade)}
	if item.Type == "GEAR" {
		if item.Level == 0 {
			item.Level = 40
		}
		name += " (" + strings.ToUpper(item.Grade[:1]) + strings.ToLower(item.Grade[1:]) + ") A"
		if item.ID == 516193 {
			name = "Eternal Armor (Beyond) C"
		}
		tags["level"] = strconv.Itoa(item.Level)
	}
	id, _ := catalog.MarketDefinitionID(item.ID)
	item.MarketHashName, item.SteamDefinitionID, item.SteamTags = name, id, tags
	return item
}

func TestCatalogIdentityChangeClearsOverlayAndRejectsOldResult(t *testing.T) {
	original := activeApp
	t.Cleanup(func() { activeApp = original })
	old := resolvedTestItem(catalog.ItemConfig{ID: 516193, Name: map[string]string{"en-US": "Eternal Armor"}, Type: "GEAR", Grade: "BEYOND", Level: 90, Marketable: true})
	activeApp = &App{itemMap: map[int]catalog.ItemConfig{old.ID: old}}
	activeApp.activeItemID.Store(int32(old.ID))
	analysis := market.MarketAnalysis{SteamDefinitionID: old.SteamDefinitionID, MarketHashName: old.MarketHashName, HasSuggested: true, SuggestedPrice: 7}
	setCurrentMarketAnalysis(analysis)
	current := old
	current.MarketHashName = "Eternal Armor (Beyond) D"
	installCatalog([]catalog.ItemConfig{current}, catalog.Status{State: "updated"})
	if _, shown := getCurrentMarketAnalysis(); shown {
		t.Fatal("catalog update retained old overlay")
	}
	updatePriceOverlay(old.ID, market.CurrentScope(), analysis)
	if _, shown := getCurrentMarketAnalysis(); shown {
		t.Fatal("late result replaced new identity")
	}
}

func TestIdentityMismatchInvalidatesCacheAndOverlay(t *testing.T) {
	original, fetcher := activeApp, fetchMarketDataFromSteam
	scope := market.CurrentScope()
	config := resolvedTestItem(catalog.ItemConfig{ID: 100, Name: map[string]string{"en-US": "Ruby"}})
	activeApp = &App{itemMap: map[int]catalog.ItemConfig{100: config}, priceCache: map[string]market.MarketData{market.CacheKey(scope, config.MarketHashName): {Analysis: market.MarketAnalysis{SteamDefinitionID: config.SteamDefinitionID, MarketHashName: config.MarketHashName, HasSuggested: true, SuggestedPrice: 7, UpdatedAt: time.Now()}}}}
	activeApp.activeItemID.Store(100)
	t.Cleanup(func() { flushCacheWritesNow(); activeApp = original; fetchMarketDataFromSteam = fetcher })
	fetchMarketDataFromSteam = func(catalog.ItemConfig, string, time.Time, market.MarketScope, market.RequestPriority) (market.MarketData, error) {
		return market.MarketData{}, market.ErrIdentityMismatch
	}
	fetchPriceAndUpdateWithOptions(config, false, scope, market.RequestPriorityNormal, true)
	if _, ok := marketCacheEntry(scope, config); ok {
		t.Fatal("identity mismatch retained cached price")
	}
	if analysis, _ := getCurrentMarketAnalysis(); analysis.HasSuggested {
		t.Fatal("identity mismatch displayed old price")
	}
	setCurrentMarketAnalysis(market.MarketAnalysis{SteamDefinitionID: config.SteamDefinitionID, MarketHashName: config.MarketHashName, HasSuggested: true, SuggestedPrice: 7})
	if err := fetchInventoryMarketPrice(context.Background(), config.ID); err != nil {
		t.Fatal(err)
	}
	if analysis, _ := getCurrentMarketAnalysis(); analysis.HasSuggested {
		t.Fatal("inventory rejection retained the active overlay price")
	}
}

func TestLegacyPriceWithoutDefinitionIsRejected(t *testing.T) {
	original := activeApp
	t.Cleanup(func() { activeApp = original })
	config := resolvedTestItem(catalog.ItemConfig{ID: 100, Name: map[string]string{"en-US": "Ruby"}})
	scope := market.DefaultScope()
	activeApp = &App{priceCache: map[string]market.MarketData{market.CacheKey(scope, config.MarketHashName): {Analysis: market.MarketAnalysis{HasSuggested: true, SuggestedPrice: 7}}}}
	if _, ok := marketCacheEntry(scope, config); ok {
		t.Fatal("accepted unverified legacy price")
	}
}
