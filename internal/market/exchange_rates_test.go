package market

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestExchangeRatesUseOneV2RequestForAllCurrencies(t *testing.T) {
	withExchangeRateCache(t, map[string]float64{})
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	rows := make([]map[string]any, 0)
	for _, currency := range supportedMarketCurrencies {
		if currency.Code != "USD" {
			rows = append(rows, map[string]any{"date": time.Now().Add(24 * time.Hour).Format("2006-01-02"), "base": "USD", "quote": currency.Code, "rate": fallbackExchangeRates[currency.Code] * 1.1})
		}
	}
	body, _ := json.Marshal(rows)
	requests := 0
	http.DefaultTransport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		if request.URL.String() != "https://api.frankfurter.dev/v2/rates?base=USD" {
			t.Fatalf("unexpected exchange rate URL: %s", request.URL)
		}
		return testHTTPResponse(200, string(body)), nil
	})
	fetchExchangeRatesFromAPI()
	if requests != 1 {
		t.Fatalf("made %d requests, want one", requests)
	}
	for _, currency := range supportedMarketCurrencies {
		want := fallbackExchangeRates[currency.Code]
		if currency.Code != "USD" {
			want *= 1.1
		}
		assertFloatEqual(t, getExchangeRate(currency.Code), want)
	}
}

func TestExchangeRateFailuresKeepCacheAndOfflineFallbacks(t *testing.T) {
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	for _, response := range []struct {
		status int
		body   string
	}{
		{503, `{}`}, {200, `{`},
		{200, `[{"base":"EUR","quote":"RUB","rate":100},{"base":"USD","quote":"RUB","rate":0},{"base":"USD","quote":"GBP","rate":-1}]`},
	} {
		withExchangeRateCache(t, map[string]float64{"RUB": 85})
		http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
			return testHTTPResponse(response.status, response.body), nil
		})
		fetchExchangeRatesFromAPI()
		assertFloatEqual(t, getExchangeRate("RUB"), 85)
		assertFloatEqual(t, getExchangeRate("GBP"), fallbackExchangeRates["GBP"])
	}
	withExchangeRateCache(t, map[string]float64{})
	http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, fmt.Errorf("offline") })
	fetchExchangeRatesFromAPI()
	for _, currency := range supportedMarketCurrencies {
		assertFloatEqual(t, getExchangeRate(currency.Code), fallbackExchangeRates[currency.Code])
	}
}

func TestNonUSDHistoryUsesItsDeclaredCurrency(t *testing.T) {
	original := currentMarketScope()
	t.Cleanup(func() { setMarketScope(original.Currency.Code, original.Region.CountryCode) })
	withExchangeRateCache(t, map[string]float64{"RUB": 80, "EUR": 0.9, "GBP": 0.8})
	now := time.Now()
	for _, code := range []string{"RUB", "EUR", "GBP"} {
		scope, _, _ := selectMarketCurrency(code)
		for _, historyID := range []int{scope.Currency.SteamCurrencyID, 1, 999, 12} {
			t.Run(fmt.Sprintf("%s/history-%d", code, historyID), func(t *testing.T) {
				body := []byte(fmt.Sprintf(`{"queries":[{"queryKey":["market","orderbook"],"state":{"data":{"amtMaxBuyOrder":100,"amtMinSellOrder":10000,"eCurrency":%d,"cBuyOrders":12,"cSellOrders":3}}},{"queryKey":["market","pricehistory"],"state":{"data":{"ecurrency":%d,"prices":[{"time":%d,"price_median":10.0,"purchases":3}]}}}]}`, scope.Currency.SteamCurrencyID, historyID, now.Unix()))
				data, err := marketDataFromListingSSR("Astral Diamond", body, "", now, scope)
				if err != nil {
					t.Fatal(err)
				}
				switch historyID {
				case scope.Currency.SteamCurrencyID:
					assertFloatEqual(t, data.Analysis.WeeklyAveragePrice, 10)
					if data.Analysis.SuggestedPrice >= data.Analysis.LowestSellPrice {
						t.Fatal("listing price overwrote historical suggested-price cap")
					}
				case 1:
					assertFloatEqual(t, data.Analysis.WeeklyAveragePrice, 10*getExchangeRate(code))
				default:
					if data.Analysis.HasSaleHistory {
						t.Fatal("unsupported or mismatched history was used")
					}
				}
				if _, ok := ssrHistoryCurrency(body); ok == (historyID == 999) {
					t.Fatal("incorrect currency recognition")
				}
			})
		}
	}
}
