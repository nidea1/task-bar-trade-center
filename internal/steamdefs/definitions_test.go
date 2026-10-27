package steamdefs

import (
	"context"
	"github.com/nidea1/task-bar-trade-center/internal/catalog"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestReadDefinitionValidatesProperties(t *testing.T) {
	valid := map[string]string{"appid": "3678970", "itemdefid": "516173900", "name": "Dimensional Armor (Beyond) C", "marketable": "1", "tags": "type:gear;rarity:beyond;level:80"}
	read := func(values map[string]string) (catalog.MarketDefinition, bool) {
		return readDefinition(516173900, func(_ int, key string) (string, bool) { v, ok := values[key]; return v, ok })
	}
	if d, ok := read(valid); !ok || d.Tags["level"] != "80" {
		t.Fatalf("definition=%+v ok=%v", d, ok)
	}
	for _, key := range []string{"appid", "itemdefid", "name", "marketable", "tags"} {
		values := map[string]string{}
		for k, v := range valid {
			values[k] = v
		}
		values[key] = "invalid"
		if _, ok := read(values); ok && key != "name" {
			t.Fatalf("accepted invalid %s", key)
		}
	}
}

func TestReadLiveSteamDefinitions(t *testing.T) {
	root := os.Getenv("TBTC_TEST_GAME_DATA")
	if root == "" || os.Getenv("TBTC_TEST_STEAM") != "1" {
		t.Skip("set TBTC_TEST_GAME_DATA and TBTC_TEST_STEAM=1 for live SDK integration")
	}
	items, err := catalog.LoadItems()
	if err != nil {
		t.Fatal(err)
	}
	ids := []int{}
	for _, item := range items {
		if item.Marketable {
			id, _ := catalog.MarketDefinitionID(item.ID)
			ids = append(ids, id)
		}
	}
	definitions, err := Read(context.Background(), filepath.Join(root, "Plugins", "x86_64", "steam_api64.dll"), ids)
	if err != nil {
		t.Fatal(err)
	}
	for id, want := range map[int]string{516171900: "Dimensional Armor (Beyond) A", 516173900: "Dimensional Armor (Beyond) C", 516193900: "Eternal Armor (Beyond) C"} {
		if got := definitions[id]; got.Name != want || !got.Marketable || got.AppID != catalog.SteamAppID {
			t.Errorf("%d: %+v, want %s", id, got, want)
		}
	}
	resolved := catalog.ApplyMarketDefinitions(items, definitions)
	count := 0
	for _, item := range resolved {
		if item.HasMarketIdentity() {
			count++
		}
	}
	t.Log("resolved market identities:", strconv.Itoa(count), "/", len(ids))
	if count < 1100 {
		t.Fatalf("unexpected identity coverage: %d", count)
	}
}
