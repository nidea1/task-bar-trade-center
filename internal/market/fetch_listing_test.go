package market

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/catalog"
)

func TestSteamHTTPTransportDisablesCompression(t *testing.T) {
	transport, ok := newSteamHTTPTransport().(*http.Transport)
	if !ok {
		t.Fatalf("Steam transport type = %T, want *http.Transport", newSteamHTTPTransport())
	}
	if !transport.DisableCompression {
		t.Fatal("Steam transport must omit Accept-Encoding to receive complete SSR data")
	}
}

func TestListing429DoesNotCascadeToLegacyEndpoints(t *testing.T) {
	originalTransport := steamHTTPTransport
	originalLimiter := steamRequestLimiter
	t.Cleanup(func() {
		steamHTTPTransport = originalTransport
		steamRequestLimiter = originalLimiter
	})

	steamRequestLimiter = newSteamRateLimiter()
	listingRequests := 0
	steamHTTPTransport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		resetSteamRequestLimiterForTest()
		if !strings.Contains(request.URL.Path, "/market/listings/") {
			t.Fatalf("unexpected request URL: %s", request.URL.String())
			return testHTTPResponse(http.StatusNotFound, ``), nil
		}
		listingRequests++
		return testHTTPResponse(http.StatusTooManyRequests, ``), nil
	})

	name := "Issue Eight Item"
	config := resolvedTestItem(catalog.ItemConfig{ID: 1, Type: "MATERIAL", Name: map[string]string{"en-US": name}})
	_, err := fetchDataForScope(config, name, time.Unix(1_700_000_000, 0), defaultMarketScope(), RequestPriorityHigh)
	if err == nil || !strings.Contains(err.Error(), "429") {
		t.Fatalf("listing rate limit error = %v, want 429", err)
	}
	if listingRequests != 1 {
		t.Fatalf("listing requests = %d, want 1", listingRequests)
	}
}

func TestScopedListingAcceptsUSDSSRAndConvertsLocally(t *testing.T) {
	originalTransport := steamHTTPTransport
	originalLimiter := steamRequestLimiter
	t.Cleanup(func() {
		steamHTTPTransport = originalTransport
		steamRequestLimiter = originalLimiter
	})
	withExchangeRateCache(t, map[string]float64{"GBP": 0.5})

	steamRequestLimiter = newSteamRateLimiter()
	now := time.Unix(1_700_000_100, 0)
	steamHTTPTransport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		resetSteamRequestLimiterForTest()
		if !strings.Contains(request.URL.Path, "/market/listings/") {
			t.Fatalf("USD SSR should avoid fallback endpoints: %s", request.URL.String())
		}
		body := `<script>JSON.parse("{\"state\":{\"data\":{\"amtMaxBuyOrder\":80,\"amtMinSellOrder\":101,\"eCurrency\":1,\"cBuyOrders\":4,\"cSellOrders\":5,\"rgCompactBuyOrders\":[80,6],\"rgCompactSellOrders\":[101,7]}},\"queryKey\":[\"market\",\"orderbook\",3678970,\"Astral Diamond\"]}")</script>
<script>JSON.parse("{\"state\":{\"data\":{\"history\":[{\"time\":1700000000,\"price_median\":1.00,\"purchases\":3}]}},\"queryKey\":[\"market\",\"pricehistory\",3678970,\"Astral Diamond\"]}")</script>`
		body += `<script>` + testDescriptionQuery("Astral Diamond") + `</script>`
		return testHTTPResponse(http.StatusOK, body), nil
	})

	scope, _ := marketScopeFor("GBP", "GB")
	config := resolvedTestItem(catalog.ItemConfig{ID: 116002, Type: "MATERIAL", Name: map[string]string{"en-US": "Astral Diamond"}})
	data, err := FetchDataWithPriority(config, "Astral Diamond", now, scope, RequestPriorityHigh)
	if err != nil {
		t.Fatalf("FetchDataWithPriority failed: %v", err)
	}
	analysis := data.Analysis
	if !analysis.HasOrderBook || !analysis.HasSaleHistory || !analysis.PriceIsEstimated || analysis.PriceSource != "listing-ssr-usd" {
		t.Fatalf("converted SSR analysis = %+v", analysis)
	}
	assertFloatEqual(t, analysis.SuggestedPrice, 0.50)
	assertFloatEqual(t, analysis.WeeklyAveragePrice, 0.50)
	if analysis.PricePrefix != scope.Currency.PricePrefix || analysis.PriceSuffix != scope.Currency.PriceSuffix {
		t.Fatalf("converted format = %q/%q", analysis.PricePrefix, analysis.PriceSuffix)
	}
}
