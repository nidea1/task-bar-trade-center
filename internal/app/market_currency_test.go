package app

import (
	"github.com/nidea1/task-bar-trade-center/internal/catalog"
	"github.com/nidea1/task-bar-trade-center/internal/market"

	"math"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSupportedMarketCurrenciesAndRegions(t *testing.T) {
	if len(supportedMarketCurrencies) != 15 {
		t.Fatalf("supported currency count = %d, want 15", len(supportedMarketCurrencies))
	}

	wantIDs := map[string]int{
		"USD": 1,
		"EUR": 3,
		"PHP": 12,
	}
	seenCurrencies := make(map[string]struct{}, len(supportedMarketCurrencies))
	for _, currency := range supportedMarketCurrencies {
		if _, exists := seenCurrencies[currency.Code]; exists {
			t.Fatalf("duplicate currency %q", currency.Code)
		}
		seenCurrencies[currency.Code] = struct{}{}
		if wantID, exists := wantIDs[currency.Code]; exists && currency.SteamCurrencyID != wantID {
			t.Fatalf("%s steam currency ID = %d, want %d", currency.Code, currency.SteamCurrencyID, wantID)
		}
	}
	if _, exists := seenCurrencies["TRY"]; exists {
		t.Fatal("TRY must not be a supported market currency")
	}

	eurRegions := market.RegionsForCurrency("EUR")
	if len(eurRegions) != 10 {
		t.Fatalf("EUR region count = %d, want 10", len(eurRegions))
	}
	for _, region := range eurRegions {
		if region.CurrencyCode != "EUR" {
			t.Fatalf("EUR list contained region for %s", region.CurrencyCode)
		}
	}
}

func TestMarketSelectionRestrictsRegions(t *testing.T) {
	original := market.CurrentScope()
	defer market.SetScope(original.Currency.Code, original.Region.CountryCode)

	if _, ok := market.SetScope("USD", "US"); !ok {
		t.Fatal("could not set USD/US")
	}
	scope, changed, ok := market.SelectCurrency("EUR")
	if !ok || !changed {
		t.Fatal("selecting EUR did not change the market scope")
	}
	if scope.Currency.Code != "EUR" || scope.Region.CountryCode != "DE" {
		t.Fatalf("EUR scope = %+v, want EUR/DE", scope)
	}

	scope, changed, ok = market.SelectRegion("EUR", "FR")
	if !ok || !changed || scope.Region.CountryCode != "FR" {
		t.Fatalf("selecting France = %+v, changed=%t, ok=%t", scope, changed, ok)
	}
	if _, _, ok := market.SelectRegion("EUR", "US"); ok {
		t.Fatal("USD region was accepted while EUR was selected")
	}
	if _, ok := market.SetScope("USD", "US"); !ok {
		t.Fatal("could not reset USD/US")
	}
	scope, changed, ok = market.SelectRegion("EUR", "FR")
	if !ok || !changed || scope.Currency.Code != "EUR" || scope.Region.CountryCode != "FR" {
		t.Fatalf("selecting EUR/France from USD = %+v, changed=%t, ok=%t", scope, changed, ok)
	}

	scope, changed, ok = market.SelectCurrency("USD")
	if !ok || !changed || scope.Region.CountryCode != "US" {
		t.Fatalf("switching back to USD = %+v, changed=%t, ok=%t", scope, changed, ok)
	}

	scope, changed, ok = market.SelectRegion("USD", "TR")
	if !ok || !changed || scope.Region.CountryCode != "TR" {
		t.Fatalf("selecting Turkey = %+v, changed=%t, ok=%t", scope, changed, ok)
	}

	usd, _ := market.CurrencyForCode("USD")
	eur, _ := market.CurrencyForCode("EUR")
	if market.HasAdditionalRegionSelection(usd) {
		t.Fatal("USD should not expose a sub-menu region selection (US, Turkey), listed directly instead")
	}
	if !market.HasAdditionalRegionSelection(eur) {
		t.Fatal("EUR must expose a region selection menu")
	}
	eurFR, _ := market.ScopeFor("EUR", "FR")
	if got := market.CurrencyMenuLabel(eur, eurFR); got != "EUR — France" {
		t.Fatalf("EUR menu label = %q, want EUR — France", got)
	}
	if got := market.CurrencyMenuLabel(usd, eurFR); got != "USD — United States" {
		t.Fatalf("USD menu label = %q, want USD — United States", got)
	}
	usdTR, _ := market.ScopeFor("USD", "TR")

	// Test English language behavior
	originalLang := currentDisplayLanguagePreference()
	applyDisplayLanguagePreference("en-US")
	t.Cleanup(func() {
		applyDisplayLanguagePreference(originalLang)
	})

	if got := market.CurrencyMenuLabel(usd, usdTR); got != "USD — Türkiye/MENA" {
		t.Fatalf("USD/TR menu label in English = %q, want USD — Türkiye/MENA", got)
	}
	if got := market.FormatScope(usdTR); got != "USD — Türkiye/MENA" {
		t.Fatalf("USD/TR market.FormatScope in English = %q, want USD — Türkiye/MENA", got)
	}

	// Test Turkish language behavior
	applyDisplayLanguagePreference("tr-TR")
	if got := market.CurrencyMenuLabel(usd, usdTR); got != "USD — Türkiye/MENA" {
		t.Fatalf("USD/TR menu label in Turkish = %q, want USD — Türkiye/MENA", got)
	}
	if got := market.FormatScope(usdTR); got != "USD — Türkiye/MENA" {
		t.Fatalf("USD/TR market.FormatScope in Turkish = %q, want USD — Türkiye/MENA", got)
	}
}

func TestMarketScopeURLBuilders(t *testing.T) {
	tests := []struct {
		currency string
		country  string
		wantID   string
	}{
		{currency: "USD", country: "US", wantID: "1"},
		{currency: "EUR", country: "DE", wantID: "3"},
		{currency: "EUR", country: "FR", wantID: "3"},
		{currency: "PHP", country: "PH", wantID: "12"},
	}

	for _, tt := range tests {
		t.Run(tt.currency+"/"+tt.country, func(t *testing.T) {
			scope, ok := market.ScopeFor(tt.currency, tt.country)
			if !ok {
				t.Fatalf("market.ScopeFor(%q, %q) failed", tt.currency, tt.country)
			}
			assertMarketURLScope(t, priceOverviewURL("Minor Ruby", scope), scope, tt.wantID)
			assertMarketURLScope(t, itemOrdersHistogramURL("12345", scope), scope, tt.wantID)
			assertMarketURLScope(t, priceHistoryURL("Minor Ruby", scope), scope, tt.wantID)
			assertMarketURLScope(t, steamMarketListingURLForScope(catalog.ItemConfig{Name: map[string]string{"en-US": "Minor Ruby"}}, scope), scope, tt.wantID)
		})
	}
}

func assertMarketURLScope(t *testing.T, rawURL string, scope market.MarketScope, wantCurrencyID string) {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse URL %q: %v", rawURL, err)
	}
	query := parsed.Query()
	if got := query.Get("country"); got != scope.Region.CountryCode {
		t.Fatalf("country = %q, want %q", got, scope.Region.CountryCode)
	}
	if got := query.Get("currency"); got != wantCurrencyID {
		t.Fatalf("currency = %q, want %q", got, wantCurrencyID)
	}
}

func TestPriceOverviewFormattingAndCacheMigration(t *testing.T) {
	for _, tt := range []struct {
		input      string
		wantPrice  float64
		wantPrefix string
		wantSuffix string
	}{
		{input: "$46.00", wantPrice: 46, wantPrefix: "$"},
		{input: "$0.21 USD", wantPrice: 0.21, wantPrefix: "$", wantSuffix: ""},
		{input: "40,13€", wantPrice: 40.13, wantSuffix: "€"},
		{input: "P2,788.25", wantPrice: 2788.25, wantPrefix: "P"},
	} {
		price, prefix, suffix, ok := market.ParseSteamFormattedPrice(tt.input)
		if !ok || math.Abs(price-tt.wantPrice) > 0.0001 || prefix != tt.wantPrefix || suffix != tt.wantSuffix {
			t.Fatalf("ParseSteamFormattedPrice(%q) = %f, %q, %q, %t", tt.input, price, prefix, suffix, ok)
		}
	}

	data, ok := market.DataFromPriceOverview("Minor Ruby", []byte(`{"success":true,"median_price":"0,05€","volume":"4"}`), time.Now(), market.MarketCurrency{Code: "EUR", PriceSuffix: "€"})
	if !ok || data.Analysis.PricePrefix != "" || data.Analysis.PriceSuffix != "€" {
		t.Fatalf("median-only EUR format = %+v, ok=%t", data.Analysis, ok)
	}

	now := time.Now().UTC()
	eurDE, _ := market.ScopeFor("EUR", "DE")
	eurFR, _ := market.ScopeFor("EUR", "FR")
	usdUS := market.DefaultScope()
	if market.CacheKey(eurDE, "Minor Ruby") == market.CacheKey(eurFR, "Minor Ruby") {
		t.Fatal("EUR/DE and EUR/FR cache keys must differ")
	}

	normalized, migrated := normalizePriceCache(map[string]market.MarketData{
		"Minor Ruby":                         {CachedAt: now},
		market.CacheKey(eurDE, "Minor Ruby"): {CachedAt: now},
	})
	if !migrated {
		t.Fatal("legacy cache was not marked as migrated")
	}
	if _, exists := normalized[market.CacheKey(usdUS, "Minor Ruby")]; !exists {
		t.Fatal("legacy cache was not migrated to USD/US")
	}
	if _, exists := normalized[market.CacheKey(eurDE, "Minor Ruby")]; !exists {
		t.Fatal("scoped cache entry was not preserved")
	}
}

func TestMarketSettingsPersistScope(t *testing.T) {
	originalPath := SettingsFilePath
	originalScope := market.CurrentScope()
	originalOverlayMode := OverlayMode.Load()
	defer func() {
		SettingsFilePath = originalPath
		market.SetScope(originalScope.Currency.Code, originalScope.Region.CountryCode)
		OverlayMode.Store(originalOverlayMode)
	}()

	SettingsFilePath = filepath.Join(t.TempDir(), "settings.json")
	OverlayMode.Store(OverlayModeCompact)
	if _, ok := market.SetScope("PHP", "PH"); !ok {
		t.Fatal("could not set PHP/PH")
	}
	saveSettingsToDisk()

	bytes, err := os.ReadFile(SettingsFilePath)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	if string(bytes) == "" || !containsAll(string(bytes), `"market_currency": "PHP"`, `"market_country": "PH"`) {
		t.Fatalf("saved settings did not contain PHP/PH: %s", bytes)
	}

	market.SetScope("USD", "US")
	loadSettingsFromDisk()
	if scope := market.CurrentScope(); scope.Currency.Code != "PHP" || scope.Region.CountryCode != "PH" {
		t.Fatalf("loaded scope = %+v, want PHP/PH", scope)
	}
	if got := market.ScopeFromSettings("INVALID", "US"); got != market.DefaultScope() {
		t.Fatalf("invalid settings scope = %+v, want default", got)
	}
}

func TestPriceOverlayIgnoresStaleMarketScope(t *testing.T) {
	originalScope := market.CurrentScope()
	originalItemID := ActiveItemID.Load()
	originalPriceText := getCurrentPriceText()
	defer func() {
		market.SetScope(originalScope.Currency.Code, originalScope.Region.CountryCode)
		ActiveItemID.Store(originalItemID)
		setCurrentPriceText(originalPriceText)
	}()

	usdUS := market.DefaultScope()
	eurDE, _ := market.ScopeFor("EUR", "DE")
	market.SetScope(eurDE.Currency.Code, eurDE.Region.CountryCode)
	ActiveItemID.Store(42)
	setCurrentPriceText("current")

	updatePriceOverlay(42, usdUS, market.MarketAnalysis{UpdatedAt: time.Now(), HasSuggested: true, SuggestedPrice: 1})
	if got := getCurrentPriceText(); got != "current" {
		t.Fatalf("stale request updated overlay to %q", got)
	}
	updatePriceOverlay(42, eurDE, market.MarketAnalysis{UpdatedAt: time.Now(), HasSuggested: true, SuggestedPrice: 2})
	analysis, ok := getCurrentMarketAnalysis()
	if !ok || analysis.SuggestedPrice != 2 {
		t.Fatalf("current request left analysis at %+v, %v", analysis, ok)
	}
}

func containsAll(value string, expected ...string) bool {
	for _, fragment := range expected {
		if !strings.Contains(value, fragment) {
			return false
		}
	}
	return true
}
