package market

import (
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
		303193: "Eternal Sword (Legendary) A",
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
		if got := BuildHashName(item); got != want {
			t.Errorf("item %d market name = %q, want %q", id, got, want)
		}
	}
}
