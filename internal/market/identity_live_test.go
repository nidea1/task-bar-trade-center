package market

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/catalog"
	"github.com/nidea1/task-bar-trade-center/internal/steamdefs"
)

func TestLiveRUBListingAndVerifiedSearchFallback(t *testing.T) {
	if os.Getenv("TBTC_TEST_STEAM_MARKET") != "1" {
		t.Skip("set TBTC_TEST_STEAM_MARKET=1 for live RUB pricing integration")
	}
	config := resolvedTestItem(catalog.ItemConfig{ID: 142001, Name: map[string]string{"en-US": "Silver Ingot"}, Grade: "RARE", Type: "MATERIAL"})
	scope, _ := ScopeFor("RUB", "RU")
	data, err := FetchData(config, config.MarketHashName, time.Now(), scope)
	if err != nil {
		t.Fatal(err)
	}
	if !data.Analysis.HasSuggested || data.Analysis.SuggestedPrice <= 0 || data.Analysis.PriceSuffix != " ₽" || data.Analysis.SteamDefinitionID != config.SteamDefinitionID {
		t.Fatalf("missing or unverified RUB quote: %+v", data.Analysis)
	}
	t.Logf("Silver Ingot: %.2f RUB, estimated=%t, source=%s", data.Analysis.SuggestedPrice, data.Analysis.PriceIsEstimated, data.Analysis.PriceSource)
}

func TestLiveSDKIdentityAndListingPrices(t *testing.T) {
	root := os.Getenv("TBTC_TEST_GAME_DATA")
	if root == "" || os.Getenv("TBTC_TEST_STEAM_MARKET") != "1" {
		t.Skip("set TBTC_TEST_GAME_DATA and TBTC_TEST_STEAM_MARKET=1 for live pricing integration")
	}
	definitions, err := steamdefs.Read(context.Background(), filepath.Join(root, "Plugins", "x86_64", "steam_api64.dll"), []int{516171900, 516173900, 516193900})
	if err != nil {
		t.Fatal(err)
	}
	items, err := catalog.LoadItems()
	if err != nil {
		t.Fatal(err)
	}
	for _, config := range catalog.ApplyMarketDefinitions(items, definitions) {
		if !config.HasMarketIdentity() {
			continue
		}
		data, err := FetchData(config, config.MarketHashName, time.Now(), DefaultScope())
		if err != nil {
			t.Errorf("%d %s: %v", config.ID, config.MarketHashName, err)
			continue
		}
		if data.Analysis.SteamDefinitionID != config.SteamDefinitionID || data.Analysis.MarketHashName != config.MarketHashName || !data.Analysis.HasSuggested {
			t.Errorf("unverified/missing analysis: %+v", data.Analysis)
		}
		t.Logf("%d %s: %.2f (%s)", config.ID, config.MarketHashName, data.Analysis.SuggestedPrice, data.Analysis.PriceSource)
	}
}
