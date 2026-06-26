package market

func SupportedCurrencies() []MarketCurrency {
	return supportedMarketCurrencies
}

func SupportedRegions() []MarketRegion {
	return supportedMarketRegions
}

func DefaultScope() MarketScope {
	return defaultMarketScope()
}

func ScopeFor(currencyCode string, countryCode string) (MarketScope, bool) {
	return marketScopeFor(currencyCode, countryCode)
}

func CurrencyForCode(code string) (MarketCurrency, bool) {
	return marketCurrencyForCode(code)
}

func RegionForCurrency(currencyCode string, countryCode string) (MarketRegion, bool) {
	return marketRegionForCurrency(currencyCode, countryCode)
}

func RegionsForCurrency(currencyCode string) []MarketRegion {
	return marketRegionsForCurrency(currencyCode)
}

func CurrentScope() MarketScope {
	return currentMarketScope()
}

func SetScope(currencyCode string, countryCode string) (MarketScope, bool) {
	return setMarketScope(currencyCode, countryCode)
}

func SelectCurrency(currencyCode string) (MarketScope, bool, bool) {
	return selectMarketCurrency(currencyCode)
}

func SelectRegion(currencyCode string, countryCode string) (MarketScope, bool, bool) {
	return selectMarketRegion(currencyCode, countryCode)
}

func ScopeFromSettings(currencyCode string, countryCode string) MarketScope {
	return marketScopeFromSettings(currencyCode, countryCode)
}

func ScopeKey(scope MarketScope) string {
	return marketScopeKey(scope)
}

func CacheKey(scope MarketScope, marketHashName string) string {
	return marketCacheKey(scope, marketHashName)
}

func ParseCacheKey(key string) (MarketScope, string, bool) {
	return parseMarketCacheKey(key)
}

func FormatScope(scope MarketScope) string {
	return formatMarketScope(scope)
}

func CurrencyMenuLabel(currency MarketCurrency, selectedScope MarketScope) string {
	return marketCurrencyMenuLabel(currency, selectedScope)
}

func HasAdditionalRegionSelection(currency MarketCurrency) bool {
	return hasAdditionalRegionSelection(currency)
}

func GetExchangeRate(currencyCode string) float64 {
	return getExchangeRate(currencyCode)
}

func SetExchangeRate(currencyCode string, rate float64) {
	setExchangeRate(currencyCode, rate)
}

func FetchExchangeRatesFromAPI() {
	fetchExchangeRatesFromAPI()
}
