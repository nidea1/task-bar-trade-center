package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSettingsPersistLanguage(t *testing.T) {
	originalPath := SettingsFilePath
	originalPreference := currentDisplayLanguagePreference()
	originalScope := currentMarketScope()
	t.Cleanup(func() {
		SettingsFilePath = originalPath
		applyDisplayLanguagePreference(originalPreference)
		setMarketScope(originalScope.Currency.Code, originalScope.Region.CountryCode)
	})

	SettingsFilePath = filepath.Join(t.TempDir(), "settings.json")
	applyDisplayLanguagePreference("tr-TR")
	if _, ok := setMarketScope("EUR", "DE"); !ok {
		t.Fatal("could not select EUR/DE")
	}
	saveSettingsToDisk()

	var disk AppSettings
	raw, err := os.ReadFile(SettingsFilePath)
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
