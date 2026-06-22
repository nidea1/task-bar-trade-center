package main

import (
	"strings"
	"testing"
	"time"
)

func TestLocalizedMessageCatalogsAreComplete(t *testing.T) {
	if len(supportedAppLocales) != 18 {
		t.Fatalf("supported locale count = %d, want 18", len(supportedAppLocales))
	}
	for _, locale := range supportedAppLocales {
		catalog, ok := localizedMessages[locale.Code]
		if !ok {
			t.Fatalf("missing catalog for %s", locale.Code)
		}
		for key := range englishMessages {
			if strings.TrimSpace(catalog[key]) == "" {
				t.Errorf("%s has no value for %q", locale.Code, key)
			}
		}
	}
}

func TestStatusWindowCatalogEntriesAreRemoved(t *testing.T) {
	for locale, catalog := range localizedMessages {
		for _, key := range []string{"menu.show_status", "window.status_title", "button.got_it"} {
			if _, exists := catalog[key]; exists {
				t.Errorf("%s retained removed status-window key %q", locale, key)
			}
		}
	}
}

func TestSystemLocaleMappingAndExplicitLanguageSelection(t *testing.T) {
	originalResolver := windowsLocaleName
	originalPreference := currentDisplayLanguagePreference()
	originalSettingsPath := SettingsFilePath
	SettingsFilePath = ""
	t.Cleanup(func() {
		windowsLocaleName = originalResolver
		SettingsFilePath = originalSettingsPath
		applyDisplayLanguagePreference(originalPreference)
	})

	windowsLocaleName = func() string { return "tr_TR" }
	if got := resolveDisplayLanguage(displayLanguageSystem); got != "tr-TR" {
		t.Fatalf("system locale = %q, want tr-TR", got)
	}
	if got := mapSystemLocale("pt-BR"); got != "pt-BR" {
		t.Fatalf("Brazilian Portuguese = %q", got)
	}
	if got := mapSystemLocale("zh-TW"); got != "zh-Hans" {
		t.Fatalf("Chinese fallback = %q", got)
	}
	if !selectDisplayLanguage("ja-JP") || currentDisplayLanguage() != "ja-JP" {
		t.Fatal("explicit language selection did not apply Japanese")
	}
	if got := tr("hud.suggested"); got != "推奨" {
		t.Fatalf("Japanese HUD translation = %q", got)
	}
}

func TestSemanticOverlayIsRenderedInCurrentLanguage(t *testing.T) {
	originalPreference := currentDisplayLanguagePreference()
	originalText := getCurrentPriceText()
	t.Cleanup(func() {
		applyDisplayLanguagePreference(originalPreference)
		setCurrentPriceText(originalText)
	})

	applyDisplayLanguagePreference("tr-TR")
	setCurrentMarketAnalysis(MarketAnalysis{
		UpdatedAt:        time.Now(),
		SuggestedPrice:   12.5,
		HasSuggested:     true,
		DailySalesVolume: 9,
		HasDailySales:    true,
		Confidence:       "verified",
		DealTag:          "undervalued",
		VolumeActivity:   "active",
	})
	view := currentPriceOverlayView()
	if view.Confidence != "verified" || view.DealTag != "undervalued" {
		t.Fatalf("semantic values changed in the cache model: %+v", view)
	}
	if got := localizedSemanticValue(view.Confidence); got != "Doğrulanmış" {
		t.Fatalf("localized confidence = %q", got)
	}
	if !strings.Contains(view.DailySales, "adet") {
		t.Fatalf("localized daily sales = %q", view.DailySales)
	}
}
