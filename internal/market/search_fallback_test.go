package market

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/catalog"
)

func TestRUBFallbackUsesVerifiedListingClassForSparseSteamSearch(t *testing.T) {
	withExchangeRateCache(t, map[string]float64{"RUB": 80})
	listing, err := os.ReadFile("testdata/silver-ingot-eur-listing.html")
	if err != nil {
		t.Fatal(err)
	}
	search, err := os.ReadFile("testdata/silver-ingot-usd-search.html")
	if err != nil {
		t.Fatal(err)
	}
	originalTransport, originalLimiter := steamHTTPTransport, steamRequestLimiter
	t.Cleanup(func() { steamHTTPTransport, steamRequestLimiter = originalTransport, originalLimiter })
	steamRequestLimiter = newSteamRateLimiter()
	requests := 0
	steamHTTPTransport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		resetSteamRequestLimiterForTest()
		requests++
		if strings.Contains(request.URL.Path, "/market/listings/") {
			if request.URL.Query().Get("currency") != "5" || request.URL.Query().Get("country") != "RU" {
				t.Fatalf("incorrect RUB listing request: %s", request.URL)
			}
			return testHTTPResponse(200, string(listing)), nil
		}
		if request.URL.Path != "/market/search" || request.URL.Query().Get("currency") != "1" {
			t.Fatalf("unexpected fallback request: %s", request.URL)
		}
		return testHTTPResponse(200, string(search)), nil
	})
	config := resolvedTestItem(catalog.ItemConfig{ID: 142001, Name: map[string]string{"en-US": "Silver Ingot"}, Grade: "RARE", Type: "MATERIAL"})
	scope, _ := marketScopeFor("RUB", "RU")
	data, err := FetchData(config, config.MarketHashName, time.Now(), scope)
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 || !data.Analysis.HasSuggested || !data.Analysis.PriceIsEstimated || data.Analysis.SteamDefinitionID != config.SteamDefinitionID || data.Analysis.PriceSuffix != " ₽" {
		t.Fatalf("verified search fallback did not produce a RUB quote: requests=%d data=%+v", requests, data)
	}
	assertFloatEqual(t, data.Analysis.SuggestedPrice, 2.4)
	if data.Analysis.IconURL == "" {
		t.Fatal("fallback lost the verified listing icon")
	}
}

func TestSparseSearchCannotBorrowAnotherListingIdentity(t *testing.T) {
	config := resolvedTestItem(catalog.ItemConfig{ID: 142001, Name: map[string]string{"en-US": "Silver Ingot"}, Grade: "RARE", Type: "MATERIAL"})
	listing, err := os.ReadFile("testdata/silver-ingot-eur-listing.html")
	if err != nil {
		t.Fatal(err)
	}
	verified, err := verifiedListingSources(config, listing)
	if err != nil {
		t.Fatal(err)
	}
	valid := `{"appid":3678970,"classid":"8507174885","market_hash_name":"Silver Ingot","tags":[]}`
	if err := validateSearchDescription(config, []byte(valid), verified); err != nil {
		t.Fatalf("matching class was rejected: %v", err)
	}
	for _, listingInstance := range []string{`"instanceid":"0"`, `"instanceid":""`} {
		anchor := []byte(strings.ReplaceAll(string(verified), `"instanceid":"0"`, listingInstance))
		if err := validateSearchDescription(config, []byte(valid), anchor); err != nil {
			t.Fatalf("default listing instance was rejected: %v", err)
		}
	}
	withoutInstance := []byte(strings.ReplaceAll(string(verified), `"instanceid":"0",`, ""))
	if err := validateSearchDescription(config, []byte(valid), withoutInstance); err != nil {
		t.Fatalf("omitted default listing instance was rejected: %v", err)
	}
	for _, test := range []struct {
		name, search string
		listing      []byte
	}{
		{"no-anchor", valid, nil},
		{"wrong-class", strings.Replace(valid, "8507174885", "8507174886", 1), verified},
		{"missing-class", strings.Replace(valid, `"classid":"8507174885",`, "", 1), verified},
		{"wrong-name", strings.Replace(valid, "Silver Ingot", "Gold Ingot", 1), verified},
		{"wrong-app", strings.Replace(valid, "3678970", "1", 1), verified},
		{"wrong-instance", strings.Replace(valid, `"tags":[]`, `"instanceid":"7","tags":[]`, 1), verified},
		{"not-marketable", strings.Replace(valid, `"tags":[]`, `"marketable":false,"tags":[]`, 1), verified},
		{"conflicting-tags", strings.Replace(valid, `"tags":[]`, `"tags":[{"category":"type","internal_name":"material"},{"category":"rarity","internal_name":"common"}]`, 1), verified},
		{"invalid-anchor-tags", valid, []byte(strings.ReplaceAll(string(verified), `"internal_name":"rare"`, `"internal_name":"common"`))},
		{"invalid-anchor-name", valid, []byte(strings.ReplaceAll(string(verified), "Silver Ingot", "Gold Ingot"))},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := validateSearchDescription(config, []byte(test.search), test.listing); !errors.Is(err, ErrIdentityMismatch) {
				t.Fatalf("unverified sparse search was accepted: %v", err)
			}
		})
	}
}
