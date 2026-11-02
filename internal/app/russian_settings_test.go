package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/nidea1/task-bar-trade-center/internal/catalog"
	"github.com/nidea1/task-bar-trade-center/internal/market"
	"github.com/nidea1/task-bar-trade-center/internal/win32"
)

func TestRussianLanguageAndMarketSettingsRemainIndependent(t *testing.T) {
	originalApp, originalLanguage, originalScope := activeApp, currentDisplayLanguagePreference(), market.CurrentScope()
	activeApp = &App{settingsFilePath: filepath.Join(t.TempDir(), "settings.json")}
	t.Cleanup(func() {
		activeApp = originalApp
		applyDisplayLanguagePreference(originalLanguage)
		market.SetScope(originalScope.Currency.Code, originalScope.Region.CountryCode)
	})
	for _, test := range []struct{ language, currency, country string }{
		{"ru-RU", "RUB", "RU"},
		{"ru-RU", "USD", "US"},
		{"ru-RU", "EUR", "DE"},
		{"en-US", "RUB", "RU"},
		{"tr-TR", "RUB", "RU"},
	} {
		t.Run(test.language+"/"+test.currency, func(t *testing.T) {
			before := market.CurrentScope()
			SetDisplayLanguage(test.language)
			if market.CurrentScope() != before {
				t.Fatal("language selection changed the currency")
			}
			SetMarketScope(test.currency, test.country)
			if currentDisplayLanguage() != test.language {
				t.Fatal("currency selection changed the language")
			}
			state, err := GetInventoryDashboard()
			if err != nil || state.DisplayLanguage != test.language || state.CurrencyCode != test.currency || state.Translations["location.inventory"] != tr("location.inventory") {
				t.Fatalf("dashboard did not update language/scope: %+v, %v", state, err)
			}
			saveSettingsToDisk()
			raw, err := os.ReadFile(activeApp.settingsFilePath)
			if err != nil {
				t.Fatal(err)
			}
			var settings AppSettings
			if err := json.Unmarshal(raw, &settings); err != nil {
				t.Fatal(err)
			}
			if settings.DisplayLanguage != test.language || settings.MarketCurrencyCode != test.currency || settings.MarketCountry != test.country {
				t.Fatalf("persisted settings = %+v", settings)
			}
			applyDisplayLanguagePreference("ja-JP")
			market.SetScope("USD", "US")
			loadSettingsFromDisk()
			if currentDisplayLanguage() != test.language || market.CurrentScope().Currency.Code != test.currency || market.CurrentScope().Region.CountryCode != test.country {
				t.Fatal("language and currency did not survive settings reload")
			}
		})
	}
	foundLanguage, foundRegion := false, false
	for _, language := range GetDisplayLanguages() {
		foundLanguage = foundLanguage || language.Code == "ru-RU" && language.Name == "Русский"
	}
	for _, region := range GetMarketRegions() {
		foundRegion = foundRegion || region.CurrencyCode == "RUB" && region.CountryCode == "RU" && region.Name == "Russia"
	}
	if !foundLanguage || !foundRegion {
		t.Fatal("Russian language or RUB region is missing from dashboard choices")
	}
}

func TestRussianHUDLabelsFitExistingRows(t *testing.T) {
	originalLanguage := currentDisplayLanguagePreference()
	applyDisplayLanguagePreference("ru-RU")
	t.Cleanup(func() { applyDisplayLanguagePreference(originalLanguage) })
	hdc, _, _ := win32.ProcGetDC.Call(0)
	if hdc == 0 {
		t.Fatal("could not acquire a device context")
	}
	defer win32.ProcReleaseDC.Call(0, hdc)
	const dtCalcRect = 0x400
	for _, key := range []string{"hud.last_sold", "hud.lowest_sell", "hud.highest_buy", "hud.weekly_average", "hud.steam_median", "hud.steam_volume", "hud.sale_p75", "hud.spread", "hud.trend", "hud.daily_sales", "hud.orders"} {
		text, _ := syscall.UTF16PtrFromString(tr(key))
		var bounds win32.RECT
		withOverlayFont(hdc, 12, FW_NORMAL, func() {
			win32.ProcDrawTextW.Call(hdc, uintptr(unsafe.Pointer(text)), ^uintptr(0), uintptr(unsafe.Pointer(&bounds)), DT_SINGLELINE|dtCalcRect)
		})
		if width := bounds.Right - bounds.Left; width <= 0 || width > 95 {
			t.Errorf("Russian HUD label %s uses %d pixels, available 95", key, width)
		}
	}
}

func TestRussianRUBOverlayAndNotifications(t *testing.T) {
	originalApp, originalLanguage, originalScope := activeApp, currentDisplayLanguagePreference(), market.CurrentScope()
	item := resolvedTestItem(catalog.ItemConfig{ID: 116002, Type: "MATERIAL", Name: map[string]string{"en-US": "Astral Diamond"}, Grade: "COMMON", Marketable: true})
	scope, _ := market.ScopeFor("RUB", "RU")
	analysis := market.MarketAnalysis{SteamDefinitionID: item.SteamDefinitionID, UpdatedAt: time.Now(), SuggestedPrice: 12345.67, HasSuggested: true, PriceSuffix: " ₽"}
	activeApp = &App{
		allItemMap: map[int]catalog.ItemConfig{item.ID: item},
		itemMap:    map[int]catalog.ItemConfig{item.ID: item},
		priceCache: map[string]market.MarketData{market.CacheKey(scope, item.MarketHashName): {Analysis: analysis}},
	}
	applyDisplayLanguagePreference("ru-RU")
	market.SetScope("RUB", "RU")
	t.Cleanup(func() {
		activeApp = originalApp
		applyDisplayLanguagePreference(originalLanguage)
		market.SetScope(originalScope.Currency.Code, originalScope.Region.CountryCode)
	})
	if got := priceOverlayViewFromAnalysis(analysis).Suggested; got != "12 345,67 ₽" {
		t.Fatalf("Russian overlay price = %q", got)
	}
	if got, ok := inventoryNotificationItemPrice(item.ID); !ok || got != "12 345,67 ₽" {
		t.Fatalf("Russian notification price = %q, available=%t", got, ok)
	}
	if got := inventoryNotificationItemName(item.ID); got != "Astral Diamond" {
		t.Fatalf("canonical item name changed: %q", got)
	}
	analysis.PriceIsEstimated = true
	analysis.ExchangeRateDate = time.Now()
	activeApp.priceCache[market.CacheKey(scope, item.MarketHashName)] = market.MarketData{Analysis: analysis}
	if got, ok := inventoryNotificationItemPrice(item.ID); !ok || got != "12 345,67 ₽ (Оценка)" {
		t.Fatalf("converted RUB notification is not marked estimated: %q", got)
	}
	analysis.ExchangeRateDate = time.Now().Add(-8 * 24 * time.Hour)
	activeApp.priceCache[market.CacheKey(scope, item.MarketHashName)] = market.MarketData{Analysis: analysis}
	if _, ok := (cacheQuoteProvider{scope: scope}).Quote(item.ID); ok {
		t.Fatal("expired RUB conversion leaked into inventory valuation")
	}
	if _, ok := inventoryNotificationItemPrice(item.ID); ok {
		t.Fatal("expired RUB conversion leaked into notifications")
	}
}
