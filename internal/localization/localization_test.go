package localization

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestRussianCatalogAndLocaleResolution(t *testing.T) {
	service := New(func() string { return "ru_BY" })
	if !service.Supported("ru-RU") || service.DisplayName("ru-RU") != "Русский" {
		t.Fatal("Russian is not registered with its native name")
	}
	for _, locale := range []string{"ru", "ru-RU", "ru_BY", "RU-KZ", "ru-UA"} {
		if got := MapSystemLocale(locale); got != "ru-RU" {
			t.Errorf("MapSystemLocale(%q) = %q", locale, got)
		}
	}
	service.Apply(SystemPreference)
	if service.Current() != "ru-RU" || service.T("location.inventory") != "Инвентарь" {
		t.Fatal("Russian system locale did not load the embedded catalog")
	}
	if !service.Select("en-US") || service.T("location.inventory") != "Inventory" {
		t.Fatal("switching back to English did not update translations")
	}
	service.Select("ru-RU")
	if service.Preference() != "ru-RU" || service.T("dashboard.updating_pricing_data", 2, 3) != "Обновление цен: готово 2, осталось 3" {
		t.Fatal("Russian selection or placeholder formatting failed")
	}
	for input, want := range map[string]string{"tr_TR": "tr-TR", "pt-BR": "pt-BR", "pt-AO": "pt-PT", "zh-TW": "zh-Hans", "en-GB": "en-US", "unknown": "en-US"} {
		if got := MapSystemLocale(input); got != want {
			t.Errorf("MapSystemLocale(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestRussianRawCatalogCoverageAndPlaceholders(t *testing.T) {
	read := func(locale string) map[string]string {
		t.Helper()
		raw, err := localesFS.ReadFile("locales/" + locale + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var catalog map[string]string
		if err := json.Unmarshal(raw, &catalog); err != nil {
			t.Fatal(err)
		}
		return catalog
	}
	english, russian := read("en-US"), read("ru-RU")
	if len(english) != len(russian) {
		t.Fatalf("Russian key count = %d, want %d", len(russian), len(english))
	}
	placeholder := regexp.MustCompile(`%[-+# 0-9.]*[a-zA-Z%]`)
	for key, value := range english {
		translated := russian[key]
		if strings.TrimSpace(translated) == "" {
			t.Errorf("missing or empty Russian key %q", key)
		}
		if !reflect.DeepEqual(placeholder.FindAllString(value, -1), placeholder.FindAllString(translated, -1)) {
			t.Errorf("incompatible placeholders for %q: %q / %q", key, value, translated)
		}
		if strings.Count(value, "\n") != strings.Count(translated, "\n") {
			t.Errorf("incompatible line breaks for %q", key)
		}
	}
}

func TestBestItemsDashboardTranslationsExistInRawLocaleFiles(t *testing.T) {
	keys := []string{
		"dashboard.best_items_to_sell_now",
		"dashboard.weekly_avg",
		"dashboard.score",
		"dashboard.daily_sales",
		"dashboard.steam_median",
		"dashboard.steam_volume",
		"dashboard.spread",
		"dashboard.buy_orders",
		"dashboard.sell_reason.high_daily_sales",
		"dashboard.sell_reason.narrow_spread",
		"dashboard.sell_reason.high_buy_orders",
		"dashboard.sell_reason.high_confidence",
		"dashboard.sell_reason.above_weekly_average",
		"hud.estimated_local_price",
		"hud.steam_median",
		"hud.steam_volume",
	}

	for _, locale := range SupportedLocales {
		t.Run(locale.Code, func(t *testing.T) {
			raw, err := localesFS.ReadFile(fmt.Sprintf("locales/%s.json", locale.Code))
			if err != nil {
				t.Fatalf("read locale file: %v", err)
			}

			var catalog map[string]string
			if err := json.Unmarshal(raw, &catalog); err != nil {
				t.Fatalf("parse locale file: %v", err)
			}

			for _, key := range keys {
				value, ok := catalog[key]
				if !ok {
					t.Errorf("missing key %q", key)
					continue
				}
				if strings.TrimSpace(value) == "" {
					t.Errorf("empty value for key %q", key)
				}
			}
		})
	}
}
