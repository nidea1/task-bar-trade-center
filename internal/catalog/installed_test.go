package catalog

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const itemCSVHeader = "ItemKey,NameKey,ITEMTYPE,GRADE,GEARTYPE,IsCanExchangeMarketable,IsDeletedInServer\n"

func TestMergeGameCatalogPreservesLegacyAndAddsLocalizedItems(t *testing.T) {
	base := []ItemConfig{{ID: 100, Name: map[string]string{"en-US": "Legacy"}, Grade: "RARE", Type: "MATERIAL", Marketable: true}}
	names := map[string]map[string]string{"ItemName_200": {"en-US": "New Gem", "tr-TR": "Yeni Taş"}}
	got, err := mergeGameCSV([]byte(itemCSVHeader+"100,Legacy,MATERIAL,RARE,,,\n200,ItemName_200,MATERIAL,LEGENDARY,,True,\n"), names, base)
	if err != nil || len(got) != 2 || !got[0].Marketable || got[1].Name["tr-TR"] != "Yeni Taş" || !got[1].Marketable {
		t.Fatalf("items=%+v err=%v", got, err)
	}
	if base[0].Name["en-US"] != "Legacy" {
		t.Fatal("mutated source catalog")
	}
	for _, body := range []string{"200,ItemName_missing,MATERIAL,RARE,,True,\n", "200,X,MATERIAL,, ,True,\n", "200,X,MATERIAL,RARE,,yes,\n", "200,X,MATERIAL,RARE,,,\n200,X,MATERIAL,RARE,,,\n"} {
		if _, err := mergeGameCSV([]byte(itemCSVHeader+body), names, base); err == nil {
			t.Fatalf("accepted invalid catalog: %s", body)
		}
	}
}

func TestInstalledCatalogCacheAndFailureRetention(t *testing.T) {
	root := os.Getenv("TBTC_TEST_GAME_DATA")
	if root == "" {
		t.Skip("set TBTC_TEST_GAME_DATA for installed-game integration")
	}
	base, err := LoadItems()
	if err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(t.TempDir(), "catalog.json")
	items, status, err := RefreshInstalled(root, cache, base)
	if err != nil || status.State != "updated" || len(items) < len(base) {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	second, status, err := RefreshInstalled(root, cache, base)
	if err != nil || status.State != "cached" || !reflect.DeepEqual(items, second) {
		t.Fatalf("cache status=%+v err=%v", status, err)
	}
	before, err := os.ReadFile(cache)
	if err != nil {
		t.Fatal(err)
	}
	retained, _, err := RefreshInstalled(filepath.Join(t.TempDir(), "missing"), cache, items)
	if err == nil || !reflect.DeepEqual(retained, items) {
		t.Fatal("failed update did not retain previous catalog")
	}
	after, _ := os.ReadFile(cache)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("failed update overwrote cache")
	}
	loaded, _, err := LoadCached(cache, base)
	if err != nil || !reflect.DeepEqual(loaded, items) {
		t.Fatal("last good catalog could not be loaded", err)
	}
	if err := os.WriteFile(cache, []byte(`{"version":1,"fingerprint":"x","items":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadCached(cache, base); err == nil {
		t.Fatal("accepted incomplete cache")
	}
}
