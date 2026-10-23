package catalog

import "testing"

func TestCatalogIDsAreUnique(t *testing.T) {
	items, err := LoadItems()
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[int]bool, len(items))
	for _, item := range items {
		if item.ID <= 0 || seen[item.ID] {
			t.Errorf("invalid or duplicate item ID: %d", item.ID)
		}
		seen[item.ID] = true
	}
}

func TestPlagueCatalogItems(t *testing.T) {
	items, err := LoadItems()
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[int]ItemConfig, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}
	for _, want := range []struct {
		id         int
		name       string
		itemType   string
		gear       string
		marketable bool
		languages  int
	}{
		{915001, "Contaminated Normal Box (Nightmare) - 1~5", "STAGEBOX", "", false, 1},
		{150103, "Plague Fruit", "MATERIAL", "", true, 16},
		{161006, "Contaminated Reliquary", "MATERIAL", "", true, 16},
		{162006, "Contaminated Relic", "MATERIAL", "", true, 16},
		{303193, "Eternal Sword", "GEAR", "SWORD", true, 16},
		{790001, "Relentless Unfinished Reliquary", "GEAR", "RELIQUARY", false, 16},
	} {
		got, ok := byID[want.id]
		if !ok {
			t.Errorf("missing item %d", want.id)
			continue
		}
		gear := ""
		if got.Gear != nil {
			gear = *got.Gear
		}
		if got.Name["en-US"] != want.name || got.Type != want.itemType || gear != want.gear || got.Marketable != want.marketable || len(got.Name) != want.languages {
			t.Errorf("unexpected item %d: %+v", want.id, got)
		}
		for locale, name := range got.Name {
			if name == "" {
				t.Errorf("item %d has an empty %s name", want.id, locale)
			}
		}
	}
}
