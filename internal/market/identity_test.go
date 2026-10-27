package market

import (
	"encoding/json"
	"errors"
	"github.com/nidea1/task-bar-trade-center/internal/catalog"
	"strconv"
	"strings"
	"testing"
)

func resolvedTestItem(item catalog.ItemConfig) catalog.ItemConfig {
	if item.Type == "" {
		item.Type = "MATERIAL"
	}
	if item.Grade == "" {
		item.Grade = "COMMON"
	}
	name := item.Name["en-US"]
	tags := map[string]string{"type": strings.ToLower(item.Type), "rarity": strings.ToLower(item.Grade)}
	if item.Type == "GEAR" {
		name += " (" + strings.ToUpper(item.Grade[:1]) + strings.ToLower(item.Grade[1:]) + ") A"
		tags["level"] = strconv.Itoa(item.Level)
	}
	id, _ := catalog.MarketDefinitionID(item.ID)
	item.MarketHashName, item.SteamDefinitionID, item.SteamTags = name, id, tags
	return item
}

func testDescriptionQuery(name string) string {
	raw, _ := json.Marshal(map[string]any{"queryKey": []any{"market", "description", SteamAppID, name}, "state": map[string]any{"data": map[string]any{"appid": SteamAppID, "market_hash_name": name, "marketable": true, "tags": []map[string]string{{"category": "type", "internal_name": "material"}, {"category": "rarity", "internal_name": "common"}}}}})
	return string(raw)
}

func TestListingIdentityRejectsWrongDescription(t *testing.T) {
	config := resolvedTestItem(catalog.ItemConfig{ID: 116002, Type: "MATERIAL", Name: map[string]string{"en-US": "Astral Diamond"}})
	valid := `<script>` + testDescriptionQuery("Astral Diamond") + `</script>`
	for _, tc := range []struct{ old, new string }{{`"appid":3678970`, `"appid":1`}, {`"market_hash_name":"Astral Diamond"`, `"market_hash_name":"Another item"`}, {`"marketable":true`, `"marketable":false`}, {`"internal_name":"common"`, `"internal_name":"rare"`}} {
		if _, err := verifiedListingSources(config, []byte(strings.ReplaceAll(valid, tc.old, tc.new))); !errors.Is(err, ErrIdentityMismatch) {
			t.Errorf("accepted %s: %v", tc.new, err)
		}
	}
	if _, err := verifiedListingSources(config, []byte(valid)); err != nil {
		t.Fatal(err)
	}
}

func TestListingIdentityExcludesOtherItemsPrices(t *testing.T) {
	config := resolvedTestItem(catalog.ItemConfig{ID: 116002, Name: map[string]string{"en-US": "Astral Diamond"}})
	body := loaderDataListingFixture(1, 20, 25, 1700000000, true) + `<script>{"queryKey":["market","orderbook",3678970,"Other item"],"state":{"data":{"amtMinSellOrder":99999,"eCurrency":1}}}</script>`
	verified, err := verifiedListingSources(config, []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(verified), "99999") {
		t.Fatal("other item's prices leaked into parser")
	}
}
