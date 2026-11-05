package market

import (
	"math"
	"net/url"
	"strings"
	"testing"

	"github.com/nidea1/task-bar-trade-center/internal/catalog"
)

func TestRUBRegistrationSelectionAndCacheIsolation(t *testing.T) {
	original := currentMarketScope()
	t.Cleanup(func() { setMarketScope(original.Currency.Code, original.Region.CountryCode) })
	scope, changed, ok := selectMarketCurrency("RUB")
	if !ok || scope.Currency.SteamCurrencyID != 5 || scope.Region.CountryCode != "RU" || scope.Region.Name != "Russia" || !changed {
		t.Fatalf("RUB selection = %+v, changed=%t, ok=%t", scope, changed, ok)
	}
	if got := marketCurrencyMenuLabel(scope.Currency, scope); got != "RUB — Russia" {
		t.Fatalf("RUB menu = %q", got)
	}
	name := "Astral Diamond"
	key := marketCacheKey(scope, name)
	parsed, hash, ok := parseMarketCacheKey(key)
	if !ok || parsed != scope || hash != name || key == marketCacheKey(defaultMarketScope(), name) {
		t.Fatalf("RUB cache key did not round-trip independently: %q", key)
	}
	config := resolvedTestItem(catalog.ItemConfig{ID: 116002, Type: "MATERIAL", Name: map[string]string{"en-US": name}})
	for _, raw := range []string{ListingURLForScope(config, scope), ItemOrdersHistogramURL("123", scope)} {
		parsedURL, _ := url.Parse(raw)
		if parsedURL.Query().Get("currency") != "5" || parsedURL.Query().Get("country") != "RU" {
			t.Fatalf("incorrect RUB request URL: %s", raw)
		}
	}
	if got := marketScopeFromSettings("RUB", "RU"); got != scope {
		t.Fatalf("RUB settings scope = %+v", got)
	}
}

func TestRUBFormattingAndPriceIncrements(t *testing.T) {
	withExchangeRateCache(t, map[string]float64{"RUB": 80.68})
	originalLanguage := displayLanguage
	t.Cleanup(func() { displayLanguage = originalLanguage })
	displayLanguage = func() string { return "ru-RU" }
	for price, want := range map[float64]string{125.5: "125,50 ₽", 1250: "1 250,00 ₽", 12345.67: "12 345,67 ₽"} {
		if got := formatAnalysisPrice(price, true, MarketAnalysis{PriceSuffix: " ₽"}); got != want {
			t.Errorf("RUB %v = %q, want %q", price, got, want)
		}
	}
	for _, language := range []string{"en-US", "tr-TR"} {
		displayLanguage = func() string { return language }
		if got := formatAnalysisPrice(125.5, true, MarketAnalysis{PriceSuffix: " ₽"}); got != "125.50 ₽" {
			t.Errorf("%s RUB = %q", language, got)
		}
	}
	displayLanguage = func() string { return "ru-RU" }
	if got := formatAnalysisPrice(125.5, true, MarketAnalysis{PricePrefix: "$"}); got != "$125,50" {
		t.Errorf("Russian USD formatting is incorrect: %q", got)
	}
	rub, _ := marketCurrencyForCode("RUB")
	assertFloatEqual(t, rub.PriceIncrement, .01)
	assertFloatEqual(t, marketMinListingPrice(rub), 2.43)
	assertFloatEqual(t, roundToMarketIncrement(125.5051, rub.PriceIncrement), 125.51)
	if strings.Contains(formatAnalysisPrice(0, false, MarketAnalysis{PriceSuffix: " ₽"}), "₽") {
		t.Fatal("unavailable price displayed as a numeric RUB amount")
	}
	for _, invalid := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if got := formatAnalysisPrice(invalid, true, MarketAnalysis{PriceSuffix: " ₽"}); strings.Contains(got, "₽") {
			t.Fatalf("invalid RUB price was displayed: %q", got)
		}
	}
}
