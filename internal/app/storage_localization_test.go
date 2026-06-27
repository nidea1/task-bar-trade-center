package app

import (
	"encoding/json"
	"github.com/nidea1/task-bar-trade-center/internal/market"
	"os"
	"path/filepath"
	"testing"
)

func TestSettingsPersistLanguage(t *testing.T) {
	originalPath := activeApp.settingsFilePath
	originalPreference := currentDisplayLanguagePreference()
	originalScope := market.CurrentScope()
	t.Cleanup(func() {
		activeApp.settingsFilePath = originalPath
		applyDisplayLanguagePreference(originalPreference)
		market.SetScope(originalScope.Currency.Code, originalScope.Region.CountryCode)
	})

	activeApp.settingsFilePath = filepath.Join(t.TempDir(), "settings.json")
	applyDisplayLanguagePreference("tr-TR")
	if _, ok := market.SetScope("EUR", "DE"); !ok {
		t.Fatal("could not select EUR/DE")
	}
	saveSettingsToDisk()

	var disk AppSettings
	raw, err := os.ReadFile(activeApp.settingsFilePath)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	if err := json.Unmarshal(raw, &disk); err != nil {
		t.Fatalf("unmarshal settings: %v", err)
	}
	if disk.DisplayLanguage != "tr-TR" {
		t.Fatalf("saved settings = %+v", disk)
	}

	applyDisplayLanguagePreference("en-US")
	loadSettingsFromDisk()
	if currentDisplayLanguage() != "tr-TR" {
		t.Fatalf("loaded language = %q", currentDisplayLanguage())
	}
}

func TestSettingsPersistDashboardSettings(t *testing.T) {
	originalPath := activeApp.settingsFilePath
	originalDashboardSettings := currentDashboardSettings()
	t.Cleanup(func() {
		activeApp.settingsFilePath = originalPath
		activeApp.dashboardSettingsMu.Lock()
		activeApp.dashboardSettings = originalDashboardSettings
		activeApp.dashboardSettingsMu.Unlock()
	})

	activeApp.settingsFilePath = filepath.Join(t.TempDir(), "settings.json")
	want := DashboardSettings{
		ThemeMode:          "light",
		PriceMode:          "instant",
		RarityFilter:       "RARE",
		EquipmentFilter:    "weapon",
		SortMode:           "rarity_desc",
		MarketableItemsTab: "best",
	}
	setDashboardSettings(want)

	var disk AppSettings
	raw, err := os.ReadFile(activeApp.settingsFilePath)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	if err := json.Unmarshal(raw, &disk); err != nil {
		t.Fatalf("unmarshal settings: %v", err)
	}
	if disk.Dashboard != want {
		t.Fatalf("saved dashboard settings = %+v, want %+v", disk.Dashboard, want)
	}

	activeApp.dashboardSettingsMu.Lock()
	activeApp.dashboardSettings = defaultDashboardSettings()
	activeApp.dashboardSettingsMu.Unlock()

	loadSettingsFromDisk()
	if got := currentDashboardSettings(); got != want {
		t.Fatalf("loaded dashboard settings = %+v, want %+v", got, want)
	}
}
