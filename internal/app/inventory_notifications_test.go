package app

import (
	"strings"
	"testing"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/catalog"
	"github.com/nidea1/task-bar-trade-center/internal/market"
	"github.com/nidea1/task-bar-trade-center/internal/playerdata"
)

func TestRecordMarketableInventoryItemsSeedsThenDetectsNewItems(t *testing.T) {
	originalApp := activeApp
	originalPreference := currentDisplayLanguagePreference()
	originalScope := market.CurrentScope()
	applyDisplayLanguagePreference("en-US")
	market.SetScope(market.DefaultScope().Currency.Code, market.DefaultScope().Region.CountryCode)

	emerald := catalog.ItemConfig{ID: 200, Name: map[string]string{"en-US": "Emerald"}, Grade: "RARE", Marketable: true}
	scope := market.CurrentScope()
	activeApp = &App{
		allItemMap: map[int]catalog.ItemConfig{
			100: {ID: 100, Name: map[string]string{"en-US": "Ruby"}, Grade: "COMMON", Marketable: true},
			200: emerald,
		},
		itemMap: map[int]catalog.ItemConfig{
			200: emerald,
		},
		priceCache: map[string]market.MarketData{
			market.CacheKey(scope, buildMarketHashName(emerald)): {
				Analysis: market.MarketAnalysis{
					UpdatedAt:      time.Now(),
					PricePrefix:    "$",
					SuggestedPrice: 12.5,
					HasSuggested:   true,
				},
			},
		},
	}
	t.Cleanup(func() {
		activeApp = originalApp
		applyDisplayLanguagePreference(originalPreference)
		market.SetScope(originalScope.Currency.Code, originalScope.Region.CountryCode)
	})

	first := playerdata.InventorySnapshot{
		Items: []playerdata.OwnedItem{
			{ItemID: 100, UniqueID: 1000, Location: playerdata.LocationInventory, Marketable: true},
		},
	}
	if got := recordMarketableInventoryItems(first); len(got) != 0 {
		t.Fatalf("first snapshot notifications = %+v, want none", got)
	}

	second := playerdata.InventorySnapshot{
		Items: []playerdata.OwnedItem{
			{ItemID: 100, UniqueID: 1000, Location: playerdata.LocationInventory, Marketable: true},
			{ItemID: 200, UniqueID: 2000, Location: playerdata.LocationStash, Marketable: true},
			{ItemID: 300, UniqueID: 3000, Location: playerdata.LocationStash, Marketable: false},
		},
	}
	got := recordMarketableInventoryItems(second)
	if len(got) != 1 {
		t.Fatalf("second snapshot notifications = %+v, want 1", got)
	}
	if got[0].itemID != 200 || got[0].name != "Emerald" {
		t.Fatalf("new item = %+v, want Emerald item 200", got[0])
	}
	if got[0].rarity != "Rare" || got[0].price != "$12.50" {
		t.Fatalf("new item details = %+v, want Rare and $12.50", got[0])
	}
}

func TestNotifyMarketableInventoryItemsQueuesTrayNotification(t *testing.T) {
	originalApp := activeApp
	originalPublisher := publishTrayNotification
	originalPreference := currentDisplayLanguagePreference()

	activeApp = &App{appHWND: 1, trayIconAdded: true}
	applyDisplayLanguagePreference("en-US")
	var receivedTitle string
	var receivedBody string
	publishTrayNotification = func(title string, message string, _ uintptr) {
		receivedTitle = title
		receivedBody = message
	}
	clearPendingTrayNotifications()
	t.Cleanup(func() {
		activeApp = originalApp
		publishTrayNotification = originalPublisher
		applyDisplayLanguagePreference(originalPreference)
		clearPendingTrayNotifications()
	})

	notifyMarketableInventoryItems([]marketableInventoryItem{{itemID: 200, name: "Emerald", rarity: "Rare", price: "$12.50", hasPrice: true}})
	flushTrayNotifications()

	if receivedTitle != "Emerald Acquired" {
		t.Fatalf("notification title = %q", receivedTitle)
	}
	if !strings.Contains(receivedBody, "Emerald") {
		t.Fatalf("notification body = %q, want item name", receivedBody)
	}
	if !strings.Contains(receivedBody, "Rare") || !strings.Contains(receivedBody, "$12.50") {
		t.Fatalf("notification body = %q, want rarity and price", receivedBody)
	}
}
