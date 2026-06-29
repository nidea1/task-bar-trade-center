package app

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/catalog"
	"github.com/nidea1/task-bar-trade-center/internal/market"
)

func TestFetchPriceAndUpdateShowsStaleCacheBeforeRefresh(t *testing.T) {
	originalApp := activeApp
	originalScope := market.CurrentScope()
	originalFetcher := fetchMarketDataFromSteam
	t.Cleanup(func() {
		activeApp = originalApp
		market.SetScope(originalScope.Currency.Code, originalScope.Region.CountryCode)
		fetchMarketDataFromSteam = originalFetcher
	})

	scope := market.DefaultScope()
	market.SetScope(scope.Currency.Code, scope.Region.CountryCode)
	config := catalog.ItemConfig{ID: 100, Name: map[string]string{"en-US": "Ruby"}, Grade: "IMMORTAL", Marketable: true}
	marketHashName := buildMarketHashName(config)
	staleAt := time.Now().Add(-10 * time.Minute)

	activeApp = &App{
		allItemMap: map[int]catalog.ItemConfig{100: config},
		itemMap:    map[int]catalog.ItemConfig{100: config},
		priceCache: map[string]market.MarketData{
			market.CacheKey(scope, marketHashName): {
				CachedAt:      staleAt,
				OrderCachedAt: staleAt,
				Analysis: market.MarketAnalysis{
					MarketHashName: marketHashName,
					UpdatedAt:      staleAt,
					PricePrefix:    "$",
					SuggestedPrice: 1,
					HasSuggested:   true,
					HasOrderBook:   true,
				},
			},
		},
	}
	activeApp.activeItemID.Store(100)

	fetchStarted := make(chan struct{})
	releaseFetch := make(chan struct{})
	fetchMarketDataFromSteam = func(_ catalog.ItemConfig, _ string, now time.Time, _ market.MarketScope, _ market.RequestPriority) (market.MarketData, error) {
		close(fetchStarted)
		<-releaseFetch
		return market.MarketData{
			CachedAt:      now,
			OrderCachedAt: now,
			Analysis: market.MarketAnalysis{
				MarketHashName: marketHashName,
				UpdatedAt:      now,
				PricePrefix:    "$",
				SuggestedPrice: 2,
				HasSuggested:   true,
				HasOrderBook:   true,
			},
		}, nil
	}

	done := make(chan struct{})
	go func() {
		fetchPriceAndUpdateWithOptions(config, true, scope, market.RequestPriorityHigh, true)
		close(done)
	}()

	<-fetchStarted
	analysis, ok := getCurrentMarketAnalysis()
	if !ok || analysis.SuggestedPrice != 1 {
		t.Fatalf("overlay analysis before refresh = %+v, %v; want stale suggested price 1", analysis, ok)
	}

	close(releaseFetch)
	<-done
	analysis, ok = getCurrentMarketAnalysis()
	if !ok || analysis.SuggestedPrice != 2 {
		t.Fatalf("overlay analysis after refresh = %+v, %v; want fresh suggested price 2", analysis, ok)
	}
}

func TestFetchMarketDataDeduplicatesInFlightRequests(t *testing.T) {
	originalApp := activeApp
	originalScope := market.CurrentScope()
	originalFetcher := fetchMarketDataFromSteam
	t.Cleanup(func() {
		activeApp = originalApp
		market.SetScope(originalScope.Currency.Code, originalScope.Region.CountryCode)
		fetchMarketDataFromSteam = originalFetcher
	})

	scope := market.DefaultScope()
	market.SetScope(scope.Currency.Code, scope.Region.CountryCode)
	config := catalog.ItemConfig{ID: 100, Name: map[string]string{"en-US": "Ruby"}, Grade: "IMMORTAL", Marketable: true}
	marketHashName := buildMarketHashName(config)
	activeApp = &App{}

	var calls atomic.Int32
	fetchStarted := make(chan struct{})
	releaseFetch := make(chan struct{})
	fetchMarketDataFromSteam = func(_ catalog.ItemConfig, _ string, now time.Time, _ market.MarketScope, _ market.RequestPriority) (market.MarketData, error) {
		if calls.Add(1) == 1 {
			close(fetchStarted)
		}
		<-releaseFetch
		return market.MarketData{
			CachedAt: now,
			Analysis: market.MarketAnalysis{
				MarketHashName: marketHashName,
				UpdatedAt:      now,
			},
		}, nil
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if _, err := fetchMarketDataWithPriority(config, marketHashName, time.Now(), scope, market.RequestPriorityNormal); err != nil {
			t.Errorf("first fetch returned error: %v", err)
		}
	}()
	<-fetchStarted
	go func() {
		defer wg.Done()
		if _, err := fetchMarketDataWithPriority(config, marketHashName, time.Now(), scope, market.RequestPriorityHigh); err != nil {
			t.Errorf("second fetch returned error: %v", err)
		}
	}()
	time.Sleep(10 * time.Millisecond)
	close(releaseFetch)
	wg.Wait()

	if got := calls.Load(); got != 1 {
		t.Fatalf("fetch calls = %d, want 1", got)
	}
}

func TestScheduleHoveredPriceFetchDebouncesPreviousHover(t *testing.T) {
	originalApp := activeApp
	originalScope := market.CurrentScope()
	originalFetcher := fetchMarketDataFromSteam
	originalDebounce := hoverPriceFetchDebounce
	t.Cleanup(func() {
		activeApp = originalApp
		market.SetScope(originalScope.Currency.Code, originalScope.Region.CountryCode)
		fetchMarketDataFromSteam = originalFetcher
		hoverPriceFetchDebounce = originalDebounce
	})

	scope := market.DefaultScope()
	market.SetScope(scope.Currency.Code, scope.Region.CountryCode)
	hoverPriceFetchDebounce = 20 * time.Millisecond

	ruby := catalog.ItemConfig{ID: 100, Name: map[string]string{"en-US": "Ruby"}, Grade: "IMMORTAL", Marketable: true}
	emerald := catalog.ItemConfig{ID: 200, Name: map[string]string{"en-US": "Emerald"}, Grade: "IMMORTAL", Marketable: true}
	activeApp = &App{
		allItemMap: map[int]catalog.ItemConfig{100: ruby, 200: emerald},
		itemMap:    map[int]catalog.ItemConfig{100: ruby, 200: emerald},
		priceCache: make(map[string]market.MarketData),
	}
	activeApp.showOverlay.Store(true)

	fetched := make(chan int, 2)
	fetchMarketDataFromSteam = func(config catalog.ItemConfig, marketHashName string, now time.Time, _ market.MarketScope, _ market.RequestPriority) (market.MarketData, error) {
		fetched <- config.ID
		return market.MarketData{
			CachedAt: now,
			Analysis: market.MarketAnalysis{
				MarketHashName: marketHashName,
				UpdatedAt:      now,
			},
		}, nil
	}

	activeApp.activeItemID.Store(100)
	scheduleHoveredPriceFetch(ruby, 100, scope)
	activeApp.activeItemID.Store(200)
	scheduleHoveredPriceFetch(emerald, 200, scope)

	select {
	case got := <-fetched:
		if got != 200 {
			t.Fatalf("fetched item ID = %d, want only the latest hover item 200", got)
		}
	case <-time.After(time.Second):
		t.Fatal("debounced hover fetch did not run")
	}

	select {
	case got := <-fetched:
		t.Fatalf("unexpected extra hover fetch for item ID %d", got)
	case <-time.After(50 * time.Millisecond):
	}
}
