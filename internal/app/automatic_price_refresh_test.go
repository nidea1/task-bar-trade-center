package app

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/catalog"
	"github.com/nidea1/task-bar-trade-center/internal/inventory"
	"github.com/nidea1/task-bar-trade-center/internal/market"
	"github.com/nidea1/task-bar-trade-center/internal/playerdata"
)

func TestAutomaticRefreshWaitsForRecentUnpriceableAttempts(t *testing.T) {
	originalApp, originalFetcher, originalScope := activeApp, fetchMarketDataFromSteam, market.CurrentScope()
	config := resolvedTestItem(catalog.ItemConfig{ID: 142001, Name: map[string]string{"en-US": "Silver Ingot"}, Grade: "RARE", Type: "MATERIAL", Marketable: true})
	market.SetScope("USD", "US")
	activeApp = &App{itemMap: map[int]catalog.ItemConfig{config.ID: config}, allItemMap: map[int]catalog.ItemConfig{config.ID: config}, priceCache: make(map[string]market.MarketData)}
	t.Cleanup(func() {
		flushInventoryDashboardRebuildNow()
		activeApp, fetchMarketDataFromSteam = originalApp, originalFetcher
		market.SetScope(originalScope.Currency.Code, originalScope.Region.CountryCode)
	})
	state := inventory.DashboardState{Items: []inventory.DashboardItem{{ItemID: config.ID, MarketHashName: config.MarketHashName}}}
	for _, failure := range []error{nil, market.ErrIdentityUnavailable, market.ErrIdentityMismatch} {
		t.Run(fmt.Sprint(failure), func(t *testing.T) {
			fetchMarketDataFromSteam = func(_ catalog.ItemConfig, name string, now time.Time, scope market.MarketScope, _ market.RequestPriority) (market.MarketData, error) {
				data := market.MarketData{CachedAt: now, Analysis: market.UnavailableAnalysis(name, now, scope.Currency)}
				data.Analysis.SteamDefinitionID = config.SteamDefinitionID
				return data, failure
			}
			if err := fetchInventoryMarketPrice(context.Background(), config.ID); err != nil {
				t.Fatal(err)
			}
			if ids := missingOrStaleDashboardItemIDs(state, 5*time.Minute); len(ids) != 0 {
				t.Fatalf("recent unavailable result was requeued: %v", ids)
			}
			scope := market.CurrentScope()
			key := market.CacheKey(scope, config.MarketHashName)
			data := activeApp.priceCache[key]
			data.CachedAt = time.Now().Add(-6 * time.Minute)
			activeApp.priceCache[key] = data
			if ids := missingOrStaleDashboardItemIDs(state, 5*time.Minute); len(ids) != 1 || ids[0] != config.ID {
				t.Fatalf("expired unavailable result was not retried: %v", ids)
			}
			data.CachedAt = time.Now()
			activeApp.priceCache[key] = data
			market.SetScope("EUR", "DE")
			if ids := missingOrStaleDashboardItemIDs(state, 5*time.Minute); len(ids) != 1 {
				t.Fatalf("another currency suppressed refresh: %v", ids)
			}
			market.SetScope(scope.Currency.Code, scope.Region.CountryCode)
		})
	}
	unresolved := config
	unresolved.MarketHashName = ""
	activeApp.itemMap[config.ID] = unresolved
	if ids := missingOrStaleDashboardItemIDs(state, 5*time.Minute); len(ids) != 0 {
		t.Fatalf("item without a queryable identity was queued: %v", ids)
	}
}

func TestRUBSearchFallbackReachesInventoryCards(t *testing.T) {
	originalApp, originalScope, originalFetcher := activeApp, market.CurrentScope(), fetchMarketDataFromSteam
	config := resolvedTestItem(catalog.ItemConfig{ID: 142001, Name: map[string]string{"en-US": "Silver Ingot"}, Grade: "RARE", Type: "MATERIAL", Marketable: true})
	market.SetScope("RUB", "RU")
	snapshot := playerdata.InventorySnapshot{ReadAt: time.Now(), Items: []playerdata.OwnedItem{{ItemID: config.ID, UniqueID: 1, Quantity: 5, Location: playerdata.LocationInventory, Marketable: true}}}
	activeApp = &App{itemMap: map[int]catalog.ItemConfig{config.ID: config}, allItemMap: map[int]catalog.ItemConfig{config.ID: config}, priceCache: make(map[string]market.MarketData), lastSnapshot: &snapshot}
	t.Cleanup(func() {
		flushInventoryDashboardRebuildNow()
		activeApp = originalApp
		market.SetScope(originalScope.Currency.Code, originalScope.Region.CountryCode)
		fetchMarketDataFromSteam = originalFetcher
	})
	fetchMarketDataFromSteam = func(item catalog.ItemConfig, name string, now time.Time, scope market.MarketScope, _ market.RequestPriority) (market.MarketData, error) {
		if scope.Currency.Code != "RUB" || scope.Region.CountryCode != "RU" {
			t.Fatal("fetch lost the selected RUB scope")
		}
		return market.MarketData{CachedAt: now, Analysis: market.MarketAnalysis{SteamDefinitionID: item.SteamDefinitionID, MarketHashName: name, UpdatedAt: now, HasSuggested: true, SuggestedPrice: 2.4, PriceSuffix: " ₽", PriceIsEstimated: true, PriceSource: "market-search-html-usd"}}, nil
	}
	if err := fetchInventoryMarketPrice(context.Background(), config.ID); err != nil {
		t.Fatal(err)
	}
	state := buildInventoryDashboardState(snapshot)
	if len(state.Items) != 1 || !state.Items[0].HasPrice || !state.Items[0].PriceIsEstimated || math.Abs(state.Items[0].Suggested-2.4) > 0.0001 || math.Abs(state.Items[0].TotalSuggested-12) > 0.0001 || state.Totals.PricedItemCount != 5 {
		t.Fatalf("fetched RUB quote was not displayed in inventory cards: %+v", state)
	}
}
