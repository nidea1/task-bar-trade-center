package market

import (
	"fmt"
	"strings"
	"time"
)

const (
	marketOrderCacheTTL = 5 * time.Minute
	marketHistoryTTL    = 6 * time.Hour
)

var (
	translate = func(key string, args ...any) string {
		if fallback, ok := englishFallbacks[key]; ok {
			if len(args) > 0 {
				return fmt.Sprintf(fallback, args...)
			}
			return fallback
		}
		return key
	}
	localizeSemantic = func(value string) string { return value }
)

var englishFallbacks = map[string]string{
	"value.na":          "N/A",
	"value.units":       "units",
	"value.wide":        "Wide",
	"value.active":      "Active",
	"value.slow":        "Slow",
	"value.normal":      "Normal",
	"value.verified":    "Verified",
	"value.estimated":   "Estimated",
	"value.speculative": "Speculative",
	"time.just_now":     "just now",
	"time.minutes_ago":  "%dm ago",
	"time.hours_ago":    "%dh ago",
	"time.days_ago":     "%dd ago",
}

func SetLocalizer(t func(string, ...any) string, semantic func(string) string) {
	if t != nil {
		translate = t
	}
	if semantic != nil {
		localizeSemantic = semantic
	}
}

func tr(key string, args ...any) string {
	return translate(key, args...)
}

func localizedSemanticValue(value string) string {
	return localizeSemantic(strings.TrimSpace(value))
}
