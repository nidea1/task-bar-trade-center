package app

import "github.com/nidea1/task-bar-trade-center/internal/market"

var (
	supportedMarketCurrencies = market.SupportedCurrencies()
	supportedMarketRegions    = market.SupportedRegions()
)

func defaultMarketScope() MarketScope {
	return market.DefaultScope()
}

func marketScopeFor(currencyCode string, countryCode string) (MarketScope, bool) {
	return market.ScopeFor(currencyCode, countryCode)
}

func marketCurrencyForCode(code string) (MarketCurrency, bool) {
	return market.CurrencyForCode(code)
}

func marketRegionForCurrency(currencyCode string, countryCode string) (MarketRegion, bool) {
	return market.RegionForCurrency(currencyCode, countryCode)
}

func marketRegionsForCurrency(currencyCode string) []MarketRegion {
	return market.RegionsForCurrency(currencyCode)
}

func currentMarketScope() MarketScope {
	return market.CurrentScope()
}

func setMarketScope(currencyCode string, countryCode string) (MarketScope, bool) {
	return market.SetScope(currencyCode, countryCode)
}

func selectMarketCurrency(currencyCode string) (MarketScope, bool, bool) {
	return market.SelectCurrency(currencyCode)
}

func selectMarketRegion(currencyCode string, countryCode string) (MarketScope, bool, bool) {
	return market.SelectRegion(currencyCode, countryCode)
}

func marketScopeFromSettings(currencyCode string, countryCode string) MarketScope {
	return market.ScopeFromSettings(currencyCode, countryCode)
}

func marketScopeKey(scope MarketScope) string {
	return market.ScopeKey(scope)
}

func marketCacheKey(scope MarketScope, marketHashName string) string {
	return market.CacheKey(scope, marketHashName)
}

func parseMarketCacheKey(key string) (MarketScope, string, bool) {
	return market.ParseCacheKey(key)
}

func formatMarketScope(scope MarketScope) string {
	return market.FormatScope(scope)
}

func marketCurrencyMenuLabel(currency MarketCurrency, selectedScope MarketScope) string {
	return market.CurrencyMenuLabel(currency, selectedScope)
}

func hasAdditionalRegionSelection(currency MarketCurrency) bool {
	return market.HasAdditionalRegionSelection(currency)
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

func getExchangeRate(currencyCode string) float64 {
	return market.GetExchangeRate(currencyCode)
}

func setExchangeRate(currencyCode string, rate float64) {
	market.SetExchangeRate(currencyCode, rate)
}

func fetchExchangeRatesFromAPI() {
	market.FetchExchangeRatesFromAPI()
}
