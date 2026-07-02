package app

import (
	"encoding/json"
	"github.com/nidea1/task-bar-trade-center/internal/market"
	filestore "github.com/nidea1/task-bar-trade-center/internal/storage"
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestSettingsDoesNotPersistSystemLanguagePreference(t *testing.T) {
	originalPath := activeApp.settingsFilePath
	originalPreference := currentDisplayLanguagePreference()
	originalWindowsLocaleName := windowsLocaleName
	t.Cleanup(func() {
		activeApp.settingsFilePath = originalPath
		applyDisplayLanguagePreference(originalPreference)
		windowsLocaleName = originalWindowsLocaleName
	})

	windowsLocaleName = func() string { return "tr-TR" }
	activeApp.settingsFilePath = filepath.Join(t.TempDir(), "settings.json")

	loadSettingsFromDisk()

	var disk AppSettings
	raw, err := os.ReadFile(activeApp.settingsFilePath)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	if err := json.Unmarshal(raw, &disk); err != nil {
		t.Fatalf("unmarshal settings: %v", err)
	}
	if disk.DisplayLanguage != "tr-TR" {
		t.Fatalf("display language = %q, want tr-TR", disk.DisplayLanguage)
	}
}

func TestSettingsMigratesSystemLanguagePreference(t *testing.T) {
	originalPath := activeApp.settingsFilePath
	originalPreference := currentDisplayLanguagePreference()
	originalWindowsLocaleName := windowsLocaleName
	t.Cleanup(func() {
		activeApp.settingsFilePath = originalPath
		applyDisplayLanguagePreference(originalPreference)
		windowsLocaleName = originalWindowsLocaleName
	})

	windowsLocaleName = func() string { return "tr-TR" }
	activeApp.settingsFilePath = filepath.Join(t.TempDir(), "settings.json")
	if err := filestore.WriteJSON(activeApp.settingsFilePath, AppSettings{DisplayLanguage: displayLanguageSystem}); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	loadSettingsFromDisk()

	var disk AppSettings
	raw, err := os.ReadFile(activeApp.settingsFilePath)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	if err := json.Unmarshal(raw, &disk); err != nil {
		t.Fatalf("unmarshal settings: %v", err)
	}
	if disk.DisplayLanguage != "tr-TR" {
		t.Fatalf("display language = %q, want tr-TR", disk.DisplayLanguage)
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
		ThemeMode:           "light",
		PriceMode:           "instant",
		RarityFilter:        "RARE",
		EquipmentFilter:     "weapon",
		SortMode:            "rarity_desc",
		BestRarityFilter:    "LEGENDARY",
		BestEquipmentFilter: "SWORD",
		BestOwnershipFilter: "unequipped",
		BestSortMode:        "score_asc",
		MarketableItemsTab:  "best",
		NotifySources:       "box,offering",
		HotkeyModifiers:     0,
		HotkeyVK:            VK_F2,
		GameScale:           GameScale100,
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

func TestGetDashboardSettingsWaitsForSettingsLoad(t *testing.T) {
	originalApp := activeApp
	callbacks.RLock()
	originalCallbacks := callbacks.value
	callbacks.RUnlock()
	t.Cleanup(func() {
		activeApp = originalApp
		SetCallbacks(originalCallbacks)
	})

	app := New(Callbacks{})
	app.settingsFilePath = filepath.Join(t.TempDir(), "settings.json")
	want := DashboardSettings{
		ThemeMode:           "light",
		PriceMode:           "instant",
		RarityFilter:        "RARE",
		EquipmentFilter:     "weapon",
		SortMode:            "rarity_desc",
		BestRarityFilter:    "LEGENDARY",
		BestEquipmentFilter: "SWORD",
		BestOwnershipFilter: "unequipped",
		BestSortMode:        "score_asc",
		MarketableItemsTab:  "best",
		NotifySources:       "box,offering",
		HotkeyModifiers:     0,
		HotkeyVK:            VK_F2,
		GameScale:           GameScale125,
	}
	if err := filestore.WriteJSON(app.settingsFilePath, AppSettings{
		GameScalePercent: GameScale125,
		DisplayLanguage:  "en-US",
		Dashboard:        want,
	}); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	gotCh := make(chan DashboardSettings, 1)
	go func() {
		gotCh <- app.GetDashboardSettings()
	}()

	select {
	case got := <-gotCh:
		t.Fatalf("GetDashboardSettings returned before settings loaded: %+v", got)
	case <-time.After(25 * time.Millisecond):
	}

	loadSettingsFromDisk()

	select {
	case got := <-gotCh:
		if got != want {
			t.Fatalf("dashboard settings = %+v, want %+v", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("GetDashboardSettings did not return after settings loaded")
	}
}
