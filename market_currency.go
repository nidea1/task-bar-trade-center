package main

import (
	"fmt"
	"strings"
	"sync"
)

type MarketCurrency struct {
	Code            string
	SteamCurrencyID int
	DefaultCountry  string
}

type MarketRegion struct {
	CountryCode  string
	Name         string
	CurrencyCode string
}

type MarketScope struct {
	Currency MarketCurrency
	Region   MarketRegion
}

var supportedMarketCurrencies = []MarketCurrency{
	{Code: "USD", SteamCurrencyID: 1, DefaultCountry: "US"},
	{Code: "EUR", SteamCurrencyID: 3, DefaultCountry: "DE"},
	{Code: "GBP", SteamCurrencyID: 2, DefaultCountry: "GB"},
	{Code: "PHP", SteamCurrencyID: 12, DefaultCountry: "PH"},
	{Code: "JPY", SteamCurrencyID: 8, DefaultCountry: "JP"},
	{Code: "KRW", SteamCurrencyID: 16, DefaultCountry: "KR"},
	{Code: "CNY", SteamCurrencyID: 23, DefaultCountry: "CN"},
	{Code: "INR", SteamCurrencyID: 24, DefaultCountry: "IN"},
	{Code: "IDR", SteamCurrencyID: 10, DefaultCountry: "ID"},
	{Code: "THB", SteamCurrencyID: 14, DefaultCountry: "TH"},
	{Code: "VND", SteamCurrencyID: 15, DefaultCountry: "VN"},
	{Code: "BRL", SteamCurrencyID: 7, DefaultCountry: "BR"},
	{Code: "PLN", SteamCurrencyID: 6, DefaultCountry: "PL"},
	{Code: "CAD", SteamCurrencyID: 20, DefaultCountry: "CA"},
	{Code: "AUD", SteamCurrencyID: 21, DefaultCountry: "AU"},
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

func isUSMarketScope(scope MarketScope) bool {
	return scope.Currency.Code == "USD" && scope.Region.CountryCode == "US"
}

func marketCurrencyForMenuCommand(commandID uint32) (MarketCurrency, bool) {
	if commandID < MenuCurrencyBase {
		return MarketCurrency{}, false
	}
	index := int(commandID - MenuCurrencyBase)
	if index < 0 || index >= len(supportedMarketCurrencies) {
		return MarketCurrency{}, false
	}
	return supportedMarketCurrencies[index], true
}

func marketRegionForMenuCommand(commandID uint32) (MarketRegion, bool) {
	if commandID < MenuRegionBase {
		return MarketRegion{}, false
	}
	index := int(commandID - MenuRegionBase)
	if index < 0 || index >= len(supportedMarketRegions) {
		return MarketRegion{}, false
	}
	return supportedMarketRegions[index], true
}
