package localization

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestBestItemsDashboardTranslationsExistInRawLocaleFiles(t *testing.T) {
	keys := []string{
		"dashboard.best_items_to_sell_now",
		"dashboard.weekly_avg",
		"dashboard.score",
		"dashboard.daily_sales",
		"dashboard.spread",
		"dashboard.buy_orders",
		"dashboard.sell_reason.high_daily_sales",
		"dashboard.sell_reason.narrow_spread",
		"dashboard.sell_reason.high_buy_orders",
		"dashboard.sell_reason.high_confidence",
		"dashboard.sell_reason.above_weekly_average",
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
