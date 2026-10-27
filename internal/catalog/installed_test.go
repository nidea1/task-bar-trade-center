package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const itemCSVHeader = "ItemKey,NameKey,ITEMTYPE,GRADE,GEARTYPE,IsCanExchangeMarketable,IsDeletedInServer,Level\n"

func TestInstalledCatalogCacheRequiresEquipmentLevel(t *testing.T) {
	base := []ItemConfig{{ID: 516193, Name: map[string]string{"en-US": "Eternal Armor"}, Grade: "BEYOND", Type: "GEAR", Level: 90, Marketable: true}}
	for _, tc := range []struct {
		name    string
		version int
		level   int
		valid   bool
	}{
		{"old-format", 1, 0, false},
		{"guessed-variant-format", 2, 90, false},
		{"missing-level", installedCacheVersion, 0, false},
		{"current-format", installedCacheVersion, 90, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item := base[0]
			item.Level = tc.level
			body, err := json.Marshal(installedCache{Version: tc.version, Fingerprint: "test", Items: []ItemConfig{item}})
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "catalog.json")
			if err := os.WriteFile(path, body, 0600); err != nil {
				t.Fatal(err)
			}
			items, _, err := LoadCached(path, base)
			if (err == nil) != tc.valid {
				t.Fatalf("LoadCached error = %v, want valid = %v", err, tc.valid)
			}
			if tc.valid && items[0].Level != tc.level {
				t.Fatalf("cached level = %d, want %d", items[0].Level, tc.level)
			}
		})
	}
}

func TestRenamedEquipmentDoesNotGuessMarketName(t *testing.T) {
	for _, tc := range []struct {
		id    int
		level int
		want  string
	}{
		{516193, 80, ""},
		{516191, 90, ""},
	} {
		base := []ItemConfig{{ID: tc.id, Name: map[string]string{"en-US": "Eternal Armor"}, Grade: "BEYOND", Type: "GEAR", Level: tc.level, Marketable: true}}
		row := fmt.Sprintf("%d,Renamed Armor,GEAR,BEYOND,ARMOR,True,,%d\n", tc.id, tc.level)
		items, err := mergeGameCSV([]byte(itemCSVHeader+row), nil, base)
		if err != nil {
			t.Fatal(err)
		}
		if got := items[0].MarketHashName; got != tc.want {
			t.Errorf("item %d retained market name = %q, want %q", tc.id, got, tc.want)
		}
		if items[0].Level != tc.level {
			t.Errorf("item %d level = %d, want %d", tc.id, items[0].Level, tc.level)
		}
	}
}

func TestMergeGameCatalogPreservesLegacyAndAddsLocalizedItems(t *testing.T) {
	base := []ItemConfig{{ID: 100, Name: map[string]string{"en-US": "Legacy"}, Grade: "RARE", Type: "MATERIAL", Marketable: true}}
	names := map[string]map[string]string{"ItemName_200": {"en-US": "New Gem", "tr-TR": "Yeni Taş"}}
	got, err := mergeGameCSV([]byte(itemCSVHeader+"100,Legacy,MATERIAL,RARE,,,,\n200,ItemName_200,MATERIAL,LEGENDARY,,True,,\n"), names, base)
	if err != nil || len(got) != 2 || !got[0].Marketable || got[1].Name["tr-TR"] != "Yeni Taş" || !got[1].Marketable {
		t.Fatalf("items=%+v err=%v", got, err)
	}
	if base[0].Name["en-US"] != "Legacy" {
		t.Fatal("mutated source catalog")
	}
	for _, body := range []string{"200,ItemName_missing,MATERIAL,RARE,,True,,\n", "200,X,MATERIAL,, ,True,,\n", "200,X,MATERIAL,RARE,,yes,,\n", "200,X,MATERIAL,RARE,,,,\n200,X,MATERIAL,RARE,,,,\n", "200,X,GEAR,RARE,ARMOR,True,,invalid\n", "200,X,GEAR,RARE,ARMOR,True,,-1\n", "200,X,GEAR,RARE,ARMOR,True,,\n"} {
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
	var legacy installedCache
	if err := json.Unmarshal(before, &legacy); err != nil {
		t.Fatal(err)
	}
	legacy.Version = 1
	for index := range legacy.Items {
		legacy.Items[index].Level = 0
	}
	legacyBody, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cache, legacyBody, 0600); err != nil {
		t.Fatal(err)
	}
	refreshed, status, err := RefreshInstalled(root, cache, base)
	if err != nil || status.State != "updated" || !reflect.DeepEqual(refreshed, items) {
		t.Fatalf("legacy cache refresh status=%+v err=%v", status, err)
	}
	before, err = os.ReadFile(cache)
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
