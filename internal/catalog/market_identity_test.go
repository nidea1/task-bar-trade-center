package catalog

import "testing"

func TestMarketDefinitionsResolveVariantsIndependentlyOfLevel(t *testing.T) {
	items := []ItemConfig{{ID: 516171, Type: "GEAR", Grade: "BEYOND", Level: 80, Marketable: true}, {ID: 516173, Type: "GEAR", Grade: "BEYOND", Level: 80, Marketable: true}}
	definitions := map[int]MarketDefinition{}
	for index, name := range []string{"Dimensional Armor (Beyond) A", "Dimensional Armor (Beyond) C"} {
		id, _ := MarketDefinitionID(items[index].ID)
		definitions[id] = MarketDefinition{AppID: SteamAppID, ID: id, Name: name, Marketable: true, Tags: map[string]string{"type": "gear", "rarity": "beyond", "level": "80"}}
	}
	resolved := ApplyMarketDefinitions(items, definitions)
	for i, item := range resolved {
		if !item.HasMarketIdentity() || item.MarketHashName != definitions[item.SteamDefinitionID].Name {
			t.Fatalf("item %d unresolved: %+v", i, item)
		}
	}
	if items[0].SteamDefinitionID != 0 {
		t.Fatal("mutated input")
	}
	updated := definitions[516173900]
	updated.Name = "Dimensional Armor (Beyond) D"
	definitions[updated.ID] = updated
	if got := ApplyMarketDefinitions(resolved, definitions)[1].MarketHashName; got != updated.Name {
		t.Fatalf("variant update retained %q", got)
	}
	if got := ApplyMarketDefinitions(resolved, nil)[1]; got.HasMarketIdentity() || got.MarketHashName != "" || !got.Marketable {
		t.Fatalf("missing definition retained identity or hid item: %+v", got)
	}
}

func TestMarketDefinitionsRejectInvalidProperties(t *testing.T) {
	item := ItemConfig{ID: 516173, Type: "GEAR", Grade: "BEYOND", Level: 80, Marketable: true}
	for _, change := range []func(*MarketDefinition){func(d *MarketDefinition) { d.AppID = 1 }, func(d *MarketDefinition) { d.ID = 1 }, func(d *MarketDefinition) { d.Marketable = false }, func(d *MarketDefinition) { d.Name = "" }, func(d *MarketDefinition) { d.Tags["level"] = "90" }, func(d *MarketDefinition) { d.Tags["rarity"] = "rare" }, func(d *MarketDefinition) { d.Tags["type"] = "material" }} {
		definition := MarketDefinition{AppID: SteamAppID, ID: 516173900, Name: "Dimensional Armor (Beyond) C", Marketable: true, Tags: map[string]string{"type": "gear", "rarity": "beyond", "level": "80"}}
		change(&definition)
		got := ApplyMarketDefinitions([]ItemConfig{item}, map[int]MarketDefinition{516173900: definition})[0]
		if got.HasMarketIdentity() {
			t.Fatalf("accepted %+v", definition)
		}
	}
}
