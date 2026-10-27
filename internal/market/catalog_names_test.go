package market

import (
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/nidea1/task-bar-trade-center/internal/catalog"
)

func TestCatalogMarketHashNames(t *testing.T) {
	items, err := catalog.LoadItems()
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[int]catalog.ItemConfig, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}
	for id, want := range map[int]string{
		110001: "Minor Ruby",
		638111: "Emerald Bracer (Divine) A",
		303193: "Eternal Sword (Legendary) C",
		516171: "Dimensional Armor (Beyond) A",
		516173: "Dimensional Armor (Beyond) C",
		516193: "Eternal Armor (Beyond) C",
		150103: "Plague Fruit (Rare)",
		150107: "Plague Fruit (Beyond)",
		161001: "Contaminated Reliquary (Common)",
		161006: "Contaminated Reliquary (Arcana)",
		161010: "Contaminated Reliquary (Cosmic)",
		162002: "Contaminated Relic (Uncommon)",
		162006: "Contaminated Relic (Arcana)",
		190102: "Contaminated Soulstone - Nightmare",
	} {
		item, ok := byID[id]
		if !ok {
			t.Errorf("missing item %d", id)
			continue
		}
		if BuildHashName(item) != "" {
			t.Fatalf("unresolved item %d guessed a name", id)
		}
		definitionID, _ := catalog.MarketDefinitionID(id)
		tags := map[string]string{"type": strings.ToLower(item.Type), "rarity": strings.ToLower(item.Grade)}
		if item.Type == "GEAR" {
			tags["level"] = strconv.Itoa(item.Level)
		}
		item = catalog.ApplyMarketDefinitions([]catalog.ItemConfig{item}, map[int]catalog.MarketDefinition{definitionID: {AppID: SteamAppID, ID: definitionID, Name: want, Marketable: true, Tags: tags}})[0]
		if got := BuildHashName(item); got != want {
			t.Errorf("item %d market name = %q, want %q", id, got, want)
		}
		if got, err := url.PathUnescape(ListingURL(item)); err != nil || got != "https://steamcommunity.com/market/listings/3678970/"+want {
			t.Errorf("item %d listing URL = %q, err = %v", id, got, err)
		}
	}
}
