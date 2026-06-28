package app

import (
	"context"
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
	if strings.Contains(receivedBody, "Emerald") {
		t.Fatalf("notification body = %q, should not contain item name (already in title)", receivedBody)
	}
	if !strings.Contains(receivedBody, "Rare") || !strings.Contains(receivedBody, "$12.50") {
		t.Fatalf("notification body = %q, want rarity and price", receivedBody)
	}
}

func TestProcessNewMarketableInventoryItems(t *testing.T) {
	originalApp := activeApp
	originalPublisher := publishTrayNotification
	originalFetcher := fetchUncachedItemPrice
	originalPreference := currentDisplayLanguagePreference()
	originalScope := market.CurrentScope()

	scope := market.CurrentScope()
	ruby := catalog.ItemConfig{ID: 100, Name: map[string]string{"en-US": "Ruby"}, Grade: "COMMON", Marketable: true}
	emerald := catalog.ItemConfig{ID: 200, Name: map[string]string{"en-US": "Emerald"}, Grade: "RARE", Marketable: true}

	activeApp = &App{
		appHWND:       1,
		trayIconAdded: true,
		allItemMap: map[int]catalog.ItemConfig{
			100: ruby,
			200: emerald,
		},
		itemMap: map[int]catalog.ItemConfig{
			100: ruby,
			200: emerald,
		},
		priceCache: map[string]market.MarketData{
			market.CacheKey(scope, buildMarketHashName(ruby)): {
				Analysis: market.MarketAnalysis{
					UpdatedAt:      time.Now(),
					PricePrefix:    "$",
					SuggestedPrice: 5.0,
					HasSuggested:   true,
				},
			},
		},
	}
	applyDisplayLanguagePreference("en-US")

	var notifications []string
	publishTrayNotification = func(title string, message string, _ uintptr) {
		notifications = append(notifications, title+": "+message)
	}

	// Mock fetchUncachedItemPrice to simulate a successful API fetch that caches the price.
	fetchUncachedItemPrice = func(_ context.Context, itemID int) error {
		if itemID == 200 {
			activeApp.priceCacheMu.Lock()
			activeApp.priceCache[market.CacheKey(scope, buildMarketHashName(emerald))] = market.MarketData{
				Analysis: market.MarketAnalysis{
					UpdatedAt:      time.Now(),
					PricePrefix:    "$",
					SuggestedPrice: 12.50,
					HasSuggested:   true,
				},
			}
			activeApp.priceCacheMu.Unlock()
		}
		return nil
	}
	clearPendingTrayNotifications()

	t.Cleanup(func() {
		activeApp = originalApp
		publishTrayNotification = originalPublisher
		fetchUncachedItemPrice = originalFetcher
		applyDisplayLanguagePreference(originalPreference)
		market.SetScope(originalScope.Currency.Code, originalScope.Region.CountryCode)
		clearPendingTrayNotifications()
	})

	items := []marketableInventoryItem{
		{itemID: 100, name: "Ruby", rarity: "Common", price: "$5.00", hasPrice: true},
		{itemID: 200, name: "Emerald", rarity: "Rare", price: "Updating...", hasPrice: false},
	}

	processNewMarketableInventoryItems(items)
	flushTrayNotifications()

	// Ruby (cached) should notify immediately.
	if len(notifications) != 1 {
		t.Fatalf("notifications count immediately = %d, want 1 (only cached Ruby)", len(notifications))
	}
	if !strings.Contains(notifications[0], "Ruby") {
		t.Fatalf("notification 0 = %q, want Ruby", notifications[0])
	}

	// Wait for the background goroutine to fetch and notify Emerald.
	start := time.Now()
	for time.Since(start) < 5*time.Second {
		flushTrayNotifications()
		if len(notifications) >= 2 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if len(notifications) != 2 {
		t.Fatalf("notifications count after price resolution = %d, want 2", len(notifications))
	}
	if !strings.Contains(notifications[1], "Emerald") || !strings.Contains(notifications[1], "$12.50") {
		t.Fatalf("notification 1 = %q, want Emerald notification with price $12.50", notifications[1])
	}
}

func TestNotificationRarityFilter(t *testing.T) {
	originalApp := activeApp
	originalPreference := currentDisplayLanguagePreference()
	originalScope := market.CurrentScope()
	applyDisplayLanguagePreference("en-US")
	market.SetScope(market.DefaultScope().Currency.Code, market.DefaultScope().Region.CountryCode)

	ruby := catalog.ItemConfig{ID: 100, Name: map[string]string{"en-US": "Ruby"}, Grade: "COMMON", Marketable: true}
	emerald := catalog.ItemConfig{ID: 200, Name: map[string]string{"en-US": "Emerald"}, Grade: "RARE", Marketable: true}
	legendaryItem := catalog.ItemConfig{ID: 300, Name: map[string]string{"en-US": "Crown"}, Grade: "LEGENDARY", Marketable: true}

	activeApp = &App{
		allItemMap: map[int]catalog.ItemConfig{
			100: ruby,
			200: emerald,
			300: legendaryItem,
		},
		itemMap: map[int]catalog.ItemConfig{
			100: ruby,
			200: emerald,
			300: legendaryItem,
		},
	}
	t.Cleanup(func() {
		activeApp = originalApp
		applyDisplayLanguagePreference(originalPreference)
		market.SetScope(originalScope.Currency.Code, originalScope.Region.CountryCode)
	})

	// 1. Filter set to COMMON (level 0) - should notify everything
	activeApp.minRarityNotifyLevel.Store(int32(rarityLevel("COMMON")))
	if !shouldNotifyItem(100) {
		t.Error("COMMON item should notify when filter is COMMON")
	}
	if !shouldNotifyItem(200) {
		t.Error("RARE item should notify when filter is COMMON")
	}

	// 2. Filter set to RARE (level 2) - should NOT notify COMMON, but should notify RARE & LEGENDARY
	activeApp.minRarityNotifyLevel.Store(int32(rarityLevel("RARE")))
	if shouldNotifyItem(100) {
		t.Error("COMMON item should not notify when filter is RARE")
	}
	if !shouldNotifyItem(200) {
		t.Error("RARE item should notify when filter is RARE")
	}
	if !shouldNotifyItem(300) {
		t.Error("LEGENDARY item should notify when filter is RARE")
	}

	// 3. Filter set to LEGENDARY (level 3) - should NOT notify COMMON/RARE, but should notify LEGENDARY
	activeApp.minRarityNotifyLevel.Store(int32(rarityLevel("LEGENDARY")))
	if shouldNotifyItem(100) {
		t.Error("COMMON item should not notify when filter is LEGENDARY")
	}
	if shouldNotifyItem(200) {
		t.Error("RARE item should not notify when filter is LEGENDARY")
	}
	if !shouldNotifyItem(300) {
		t.Error("LEGENDARY item should notify when filter is LEGENDARY")
	}
}

func TestProcessNewMarketableInventoryItems_StalePrice(t *testing.T) {
	originalApp := activeApp
	originalPublisher := publishTrayNotification
	originalFetcher := fetchUncachedItemPrice
	originalPreference := currentDisplayLanguagePreference()
	originalScope := market.CurrentScope()

	scope := market.CurrentScope()
	ruby := catalog.ItemConfig{ID: 100, Name: map[string]string{"en-US": "Ruby"}, Grade: "COMMON", Marketable: true}

	activeApp = &App{
		appHWND:       1,
		trayIconAdded: true,
		allItemMap: map[int]catalog.ItemConfig{
			100: ruby,
		},
		itemMap: map[int]catalog.ItemConfig{
			100: ruby,
		},
		priceCache: map[string]market.MarketData{
			market.CacheKey(scope, buildMarketHashName(ruby)): {
				Analysis: market.MarketAnalysis{
					// Update time is 6 minutes ago (stale!)
					UpdatedAt:      time.Now().Add(-6 * time.Minute),
					PricePrefix:    "$",
					SuggestedPrice: 5.0,
					HasSuggested:   true,
				},
			},
		},
	}
	applyDisplayLanguagePreference("en-US")

	var notifications []string
	publishTrayNotification = func(title string, message string, _ uintptr) {
		notifications = append(notifications, title+": "+message)
	}

	// Mock fetchUncachedItemPrice to simulate a successful API fetch that refreshes the price.
	fetchUncachedItemPriceCalled := false
	fetchUncachedItemPrice = func(_ context.Context, itemID int) error {
		if itemID == 100 {
			fetchUncachedItemPriceCalled = true
			activeApp.priceCacheMu.Lock()
			activeApp.priceCache[market.CacheKey(scope, buildMarketHashName(ruby))] = market.MarketData{
				Analysis: market.MarketAnalysis{
					UpdatedAt:      time.Now(),
					PricePrefix:    "$",
					SuggestedPrice: 6.50, // Updated price
					HasSuggested:   true,
				},
			}
			activeApp.priceCacheMu.Unlock()
		}
		return nil
	}
	clearPendingTrayNotifications()

	t.Cleanup(func() {
		activeApp = originalApp
		publishTrayNotification = originalPublisher
		fetchUncachedItemPrice = originalFetcher
		applyDisplayLanguagePreference(originalPreference)
		market.SetScope(originalScope.Currency.Code, originalScope.Region.CountryCode)
		clearPendingTrayNotifications()
	})

	items := []marketableInventoryItem{
		{itemID: 100, name: "Ruby", rarity: "Common", price: "$5.00", hasPrice: true},
	}

	processNewMarketableInventoryItems(items)
	flushTrayNotifications()

	// Ruby has stale price, so it should NOT notify immediately.
	if len(notifications) != 0 {
		t.Fatalf("notifications count immediately = %d, want 0 (since Ruby is stale and fetching)", len(notifications))
	}

	// Wait for the background goroutine to fetch and notify Ruby.
	start := time.Now()
	for time.Since(start) < 5*time.Second {
		flushTrayNotifications()
		if len(notifications) >= 1 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !fetchUncachedItemPriceCalled {
		t.Fatal("expected fetchUncachedItemPrice to be called for stale item")
	}
	if len(notifications) != 1 {
		t.Fatalf("notifications count after price resolution = %d, want 1", len(notifications))
	}
	if !strings.Contains(notifications[0], "Ruby") || !strings.Contains(notifications[0], "$6.50") {
		t.Fatalf("notification 0 = %q, want Ruby notification with updated price $6.50", notifications[0])
	}
}

func TestProcessNewMarketableInventoryItems_MissingIcon(t *testing.T) {
	originalApp := activeApp
	originalPublisher := publishTrayNotification
	originalFetcher := fetchUncachedItemPrice
	originalPreference := currentDisplayLanguagePreference()
	originalScope := market.CurrentScope()

	scope := market.CurrentScope()
	ruby := catalog.ItemConfig{ID: 100, Name: map[string]string{"en-US": "Ruby"}, Grade: "COMMON", Marketable: true}

	activeApp = &App{
		appHWND:       1,
		trayIconAdded: true,
		allItemMap: map[int]catalog.ItemConfig{
			100: ruby,
		},
		itemMap: map[int]catalog.ItemConfig{
			100: ruby,
		},
		priceCache: map[string]market.MarketData{
			market.CacheKey(scope, buildMarketHashName(ruby)): {
				Analysis: market.MarketAnalysis{
					UpdatedAt:      time.Now(), // Fresh price!
					PricePrefix:    "$",
					SuggestedPrice: 5.0,
					HasSuggested:   true,
					IconURL:        "ruby-icon-url", // We have an IconURL but it's not downloaded/cached
				},
			},
		},
		notificationIconCache: make(map[string]uintptr),
	}
	applyDisplayLanguagePreference("en-US")

	var notifications []string
	var receivedIcon uintptr
	publishTrayNotification = func(title string, message string, icon uintptr) {
		notifications = append(notifications, title+": "+message)
		receivedIcon = icon
	}

	// Mock fetchUncachedItemPrice to make sure it is NOT called (since price is fresh)
	fetchUncachedItemPriceCalled := false
	fetchUncachedItemPrice = func(_ context.Context, itemID int) error {
		fetchUncachedItemPriceCalled = true
		return nil
	}
	clearPendingTrayNotifications()

	activeApp.appIconSmall = 42

	t.Cleanup(func() {
		activeApp = originalApp
		publishTrayNotification = originalPublisher
		fetchUncachedItemPrice = originalFetcher
		applyDisplayLanguagePreference(originalPreference)
		market.SetScope(originalScope.Currency.Code, originalScope.Region.CountryCode)
		clearPendingTrayNotifications()
	})

	items := []marketableInventoryItem{
		{itemID: 100, name: "Ruby", rarity: "Common", price: "$5.00", hasPrice: true},
	}

	processNewMarketableInventoryItems(items)
	flushTrayNotifications()

	// Since icon is missing/not cached, it should NOT notify immediately!
	if len(notifications) != 0 {
		t.Fatalf("notifications count immediately = %d, want 0 (since Ruby icon is missing and downloading)", len(notifications))
	}

	// Wait for the background goroutine to complete
	start := time.Now()
	for time.Since(start) < 5*time.Second {
		flushTrayNotifications()
		if len(notifications) >= 1 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if fetchUncachedItemPriceCalled {
		t.Fatal("expected fetchUncachedItemPrice NOT to be called since price is fresh")
	}
	if len(notifications) != 1 {
		t.Fatalf("notifications count after completion = %d, want 1", len(notifications))
	}
	if receivedIcon != 42 {
		t.Fatalf("receivedIcon = %v, want 42 (fallback to appIconSmall)", receivedIcon)
	}
}

