package market

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

var supportedMarketCurrencies = []MarketCurrency{
	{Code: "USD", SteamCurrencyID: 1, DefaultCountry: "US", PricePrefix: "$"},
	{Code: "EUR", SteamCurrencyID: 3, DefaultCountry: "DE", PriceSuffix: "€"},
	{Code: "GBP", SteamCurrencyID: 2, DefaultCountry: "GB", PricePrefix: "£"},
	{Code: "PHP", SteamCurrencyID: 12, DefaultCountry: "PH", PricePrefix: "₱"},
	{Code: "JPY", SteamCurrencyID: 8, DefaultCountry: "JP", PricePrefix: "¥"},
	{Code: "KRW", SteamCurrencyID: 16, DefaultCountry: "KR", PricePrefix: "₩"},
	{Code: "CNY", SteamCurrencyID: 23, DefaultCountry: "CN", PricePrefix: "¥"},
	{Code: "INR", SteamCurrencyID: 24, DefaultCountry: "IN", PricePrefix: "₹"},
	{Code: "IDR", SteamCurrencyID: 10, DefaultCountry: "ID", PricePrefix: "Rp"},
	{Code: "THB", SteamCurrencyID: 14, DefaultCountry: "TH", PricePrefix: "฿"},
	{Code: "VND", SteamCurrencyID: 15, DefaultCountry: "VN", PriceSuffix: "₫"},
	{Code: "BRL", SteamCurrencyID: 7, DefaultCountry: "BR", PricePrefix: "R$"},
	{Code: "PLN", SteamCurrencyID: 6, DefaultCountry: "PL", PriceSuffix: "zł"},
	{Code: "CAD", SteamCurrencyID: 20, DefaultCountry: "CA", PricePrefix: "CDN$ "},
	{Code: "AUD", SteamCurrencyID: 21, DefaultCountry: "AU", PricePrefix: "A$ "},
}

var supportedMarketRegions = []MarketRegion{
	{CountryCode: "TR", Name: "Türkiye/MENA", CurrencyCode: "USD"},
	{CountryCode: "US", Name: "United States", CurrencyCode: "USD"},
	{CountryCode: "DE", Name: "Germany", CurrencyCode: "EUR"},
	{CountryCode: "FR", Name: "France", CurrencyCode: "EUR"},
	{CountryCode: "IT", Name: "Italy", CurrencyCode: "EUR"},
	{CountryCode: "ES", Name: "Spain", CurrencyCode: "EUR"},
	{CountryCode: "NL", Name: "Netherlands", CurrencyCode: "EUR"},
	{CountryCode: "AT", Name: "Austria", CurrencyCode: "EUR"},
	{CountryCode: "BE", Name: "Belgium", CurrencyCode: "EUR"},
	{CountryCode: "PT", Name: "Portugal", CurrencyCode: "EUR"},
	{CountryCode: "FI", Name: "Finland", CurrencyCode: "EUR"},
	{CountryCode: "IE", Name: "Ireland", CurrencyCode: "EUR"},
	{CountryCode: "GB", Name: "United Kingdom", CurrencyCode: "GBP"},
	{CountryCode: "PH", Name: "Philippines", CurrencyCode: "PHP"},
	{CountryCode: "JP", Name: "Japan", CurrencyCode: "JPY"},
	{CountryCode: "KR", Name: "South Korea", CurrencyCode: "KRW"},
	{CountryCode: "CN", Name: "China", CurrencyCode: "CNY"},
	{CountryCode: "IN", Name: "India", CurrencyCode: "INR"},
	{CountryCode: "ID", Name: "Indonesia", CurrencyCode: "IDR"},
	{CountryCode: "TH", Name: "Thailand", CurrencyCode: "THB"},
	{CountryCode: "VN", Name: "Vietnam", CurrencyCode: "VND"},
	{CountryCode: "BR", Name: "Brazil", CurrencyCode: "BRL"},
	{CountryCode: "PL", Name: "Poland", CurrencyCode: "PLN"},
	{CountryCode: "CA", Name: "Canada", CurrencyCode: "CAD"},
	{CountryCode: "AU", Name: "Australia", CurrencyCode: "AUD"},
}

var marketSelection = struct {
	sync.RWMutex
	scope MarketScope
}{scope: defaultMarketScope()}

func defaultMarketScope() MarketScope {
	scope, ok := marketScopeFor("USD", "US")
	if !ok {
		panic("USD/US market scope is not configured")
	}
	return scope
}

func marketScopeFor(currencyCode string, countryCode string) (MarketScope, bool) {
	currency, ok := marketCurrencyForCode(currencyCode)
	if !ok {
		return MarketScope{}, false
	}
	region, ok := marketRegionForCurrency(currency.Code, countryCode)
	if !ok {
		return MarketScope{}, false
	}
	return MarketScope{Currency: currency, Region: region}, true
}

func marketCurrencyForCode(code string) (MarketCurrency, bool) {
	code = strings.ToUpper(strings.TrimSpace(code))
	for _, currency := range supportedMarketCurrencies {
		if currency.Code == code {
			return currency, true
		}
	}
	return MarketCurrency{}, false
}

func marketRegionForCurrency(currencyCode string, countryCode string) (MarketRegion, bool) {
	currencyCode = strings.ToUpper(strings.TrimSpace(currencyCode))
	countryCode = strings.ToUpper(strings.TrimSpace(countryCode))
	for _, region := range supportedMarketRegions {
		if region.CurrencyCode == currencyCode && region.CountryCode == countryCode {
			return region, true
		}
	}
	return MarketRegion{}, false
}

func marketRegionsForCurrency(currencyCode string) []MarketRegion {
	currencyCode = strings.ToUpper(strings.TrimSpace(currencyCode))
	regions := make([]MarketRegion, 0, 10)
	for _, region := range supportedMarketRegions {
		if region.CurrencyCode == currencyCode {
			regions = append(regions, region)
		}
	}
	return regions
}

func currentMarketScope() MarketScope {
	marketSelection.RLock()
	defer marketSelection.RUnlock()
	return marketSelection.scope
}

func setMarketScope(currencyCode string, countryCode string) (MarketScope, bool) {
	scope, ok := marketScopeFor(currencyCode, countryCode)
	if !ok {
		return MarketScope{}, false
	}
	marketSelection.Lock()
	marketSelection.scope = scope
	marketSelection.Unlock()
	return scope, true
}

func selectMarketCurrency(currencyCode string) (MarketScope, bool, bool) {
	currency, ok := marketCurrencyForCode(currencyCode)
	if !ok {
		return MarketScope{}, false, false
	}

	current := currentMarketScope()
	countryCode := current.Region.CountryCode
	if _, ok := marketRegionForCurrency(currency.Code, countryCode); !ok {
		countryCode = currency.DefaultCountry
	}
	scope, ok := setMarketScope(currency.Code, countryCode)
	if !ok {
		return MarketScope{}, false, false
	}
	return scope, scope != current, true
}

func selectMarketRegion(currencyCode string, countryCode string) (MarketScope, bool, bool) {
	current := currentMarketScope()
	scope, ok := setMarketScope(currencyCode, countryCode)
	if !ok {
		return MarketScope{}, false, false
	}
	return scope, scope != current, true
}

func marketScopeFromSettings(currencyCode string, countryCode string) MarketScope {
	if scope, ok := marketScopeFor(currencyCode, countryCode); ok {
		return scope
	}
	return defaultMarketScope()
}

func marketScopeKey(scope MarketScope) string {
	return scope.Currency.Code + ":" + scope.Region.CountryCode
}

func marketCacheKey(scope MarketScope, marketHashName string) string {
	return marketScopeKey(scope) + ":" + marketHashName
}

func parseMarketCacheKey(key string) (MarketScope, string, bool) {
	parts := strings.SplitN(key, ":", 3)
	if len(parts) != 3 || parts[2] == "" {
		return MarketScope{}, "", false
	}
	scope, ok := marketScopeFor(parts[0], parts[1])
	if !ok {
		return MarketScope{}, "", false
	}
	return scope, parts[2], true
}

func formatMarketScope(scope MarketScope) string {
	return fmt.Sprintf("%s — %s", scope.Currency.Code, scope.Region.Name)
}

func marketCurrencyMenuLabel(currency MarketCurrency, selectedScope MarketScope) string {
	region, ok := marketRegionForCurrency(currency.Code, currency.DefaultCountry)
	if !ok {
		return currency.Code
	}
	if currency.Code == selectedScope.Currency.Code {
		region = selectedScope.Region
	}
	return fmt.Sprintf("%s — %s", currency.Code, region.Name)
}

func hasAdditionalRegionSelection(currency MarketCurrency) bool {
	return currency.Code == "EUR"
}

var fallbackExchangeRates = map[string]float64{
	"USD": 1.0,
	"EUR": 0.92,
	"GBP": 0.79,
	"PHP": 58.5,
	"JPY": 155.0,
	"KRW": 1380.0,
	"CNY": 7.25,
	"INR": 83.5,
	"IDR": 16400.0,
	"THB": 36.7,
	"VND": 25400.0,
	"BRL": 5.4,
	"PLN": 4.0,
	"CAD": 1.37,
	"AUD": 1.50,
}

var (
	exchangeRateCache = make(map[string]float64)
	exchangeRateMu    sync.RWMutex
)

func getExchangeRate(currencyCode string) float64 {
	exchangeRateMu.RLock()
	rate, exists := exchangeRateCache[currencyCode]
	exchangeRateMu.RUnlock()
	if exists {
		return rate
	}
	if fallbackRate, ok := fallbackExchangeRates[currencyCode]; ok {
		return fallbackRate
	}
	return 1.0
}

func setExchangeRate(currencyCode string, rate float64) {
	if rate <= 0 {
		return
	}
	exchangeRateMu.Lock()
	exchangeRateCache[currencyCode] = rate
	exchangeRateMu.Unlock()
}

func fetchExchangeRatesFromAPI() {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://api.frankfurter.dev/v1/latest?base=USD")
	if err != nil {
		fmt.Printf("[CURRENCY] Failed to fetch exchange rates from Frankfurter: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("[CURRENCY] Frankfurter API returned status %s\n", resp.Status)
		return
	}

	var data struct {
		Rates map[string]float64 `json:"rates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		fmt.Printf("[CURRENCY] Failed to decode exchange rates: %v\n", err)
		return
	}

	exchangeRateMu.Lock()
	for currency, rate := range data.Rates {
		exchangeRateCache[currency] = rate
	}
	exchangeRateMu.Unlock()
	fmt.Printf("[CURRENCY] Successfully updated %d exchange rates from Frankfurter\n", len(data.Rates))
}
