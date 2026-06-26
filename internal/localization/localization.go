package localization

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

const SystemPreference = "sys"

type Locale struct {
	Code string
	Name string
}

var SupportedLocales = []Locale{
	{Code: "en-US", Name: "English"},
	{Code: "de-DE", Name: "Deutsch"},
	{Code: "fr-FR", Name: "Français"},
	{Code: "it-IT", Name: "Italiano"},
	{Code: "es-ES", Name: "Español"},
	{Code: "nl-NL", Name: "Nederlands"},
	{Code: "pt-PT", Name: "Português (Portugal)"},
	{Code: "fi-FI", Name: "Suomi"},
	{Code: "ja-JP", Name: "日本語"},
	{Code: "ko-KR", Name: "한국어"},
	{Code: "zh-Hans", Name: "简体中文"},
	{Code: "hi-IN", Name: "हिन्दी"},
	{Code: "id-ID", Name: "Bahasa Indonesia"},
	{Code: "th-TH", Name: "ไทย"},
	{Code: "vi-VN", Name: "Tiếng Việt"},
	{Code: "pt-BR", Name: "Português (Brasil)"},
	{Code: "pl-PL", Name: "Polski"},
	{Code: "tr-TR", Name: "Türkçe"},
}

//go:embed locales/*.json
var localesFS embed.FS

type Service struct {
	mu             sync.RWMutex
	display        string
	preference     string
	systemLocale   func() string
	english        map[string]string
	localized      map[string]map[string]string
	supportedCodes map[string]struct{}
}

func New(systemLocale func() string) *Service {
	service := &Service{
		display:        "en-US",
		preference:     SystemPreference,
		systemLocale:   systemLocale,
		english:        make(map[string]string),
		localized:      make(map[string]map[string]string),
		supportedCodes: make(map[string]struct{}, len(SupportedLocales)),
	}
	service.loadCatalogs()
	return service
}

func (service *Service) T(key string, args ...any) string {
	service.mu.RLock()
	locale := service.display
	service.mu.RUnlock()
	text := service.localized[locale][key]
	if text == "" {
		text = service.english[key]
	}
	if len(args) == 0 {
		return text
	}
	return fmt.Sprintf(text, args...)
}

func (service *Service) Current() string {
	service.mu.RLock()
	defer service.mu.RUnlock()
	return service.display
}

func (service *Service) Preference() string {
	service.mu.RLock()
	defer service.mu.RUnlock()
	return service.preference
}

func (service *Service) DisplayName(code string) string {
	for _, locale := range SupportedLocales {
		if locale.Code == code {
			return locale.Name
		}
	}
	return "English"
}

func (service *Service) Select(preference string) bool {
	if preference == "" {
		preference = SystemPreference
	}
	resolved := service.Resolve(preference)
	service.mu.Lock()
	changed := service.display != resolved || service.preference != preference
	service.display = resolved
	service.preference = preference
	service.mu.Unlock()
	return changed
}

func (service *Service) Apply(preference string) {
	if preference == "" {
		preference = SystemPreference
	}
	service.mu.Lock()
	service.preference = preference
	service.display = service.Resolve(preference)
	service.mu.Unlock()
}

func (service *Service) Resolve(preference string) string {
	if preference != "" && preference != SystemPreference {
		if service.Supported(preference) {
			return preference
		}
	}
	return MapSystemLocale(service.systemLocale())
}

func (service *Service) Supported(code string) bool {
	_, ok := service.supportedCodes[code]
	return ok
}

func (service *Service) Catalog(locale string) (map[string]string, bool) {
	catalog, ok := service.localized[locale]
	return catalog, ok
}

func (service *Service) Catalogs() map[string]map[string]string {
	return service.localized
}

func (service *Service) EnglishCatalog() map[string]string {
	return service.english
}

func MapSystemLocale(locale string) string {
	normalized := strings.ToLower(strings.ReplaceAll(locale, "_", "-"))
	for _, supported := range SupportedLocales {
		if strings.EqualFold(normalized, supported.Code) {
			return supported.Code
		}
	}
	if strings.HasPrefix(normalized, "pt-br") {
		return "pt-BR"
	}
	if strings.HasPrefix(normalized, "zh") {
		return "zh-Hans"
	}
	for _, supported := range SupportedLocales {
		if strings.HasPrefix(normalized, strings.ToLower(supported.Code[:2])) {
			return supported.Code
		}
	}
	return "en-US"
}

func LocalizedSemanticValue(service *Service, value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "undervalued", "overvalued", "verified", "estimated", "speculative", "active", "normal", "slow":
		return service.T("value." + normalized)
	default:
		return value
	}
}

func (service *Service) loadCatalogs() {
	data, err := localesFS.ReadFile("locales/en-US.json")
	if err != nil {
		panic(fmt.Sprintf("failed to read locales/en-US.json: %v", err))
	}
	if err := json.Unmarshal(data, &service.english); err != nil {
		panic(fmt.Sprintf("failed to parse locales/en-US.json: %v", err))
	}

	for _, locale := range SupportedLocales {
		service.supportedCodes[locale.Code] = struct{}{}
		filePath := fmt.Sprintf("locales/%s.json", locale.Code)
		data, err := localesFS.ReadFile(filePath)
		if err != nil {
			panic(fmt.Sprintf("failed to read %s: %v", filePath, err))
		}

		var overrides map[string]string
		if err := json.Unmarshal(data, &overrides); err != nil {
			panic(fmt.Sprintf("failed to parse %s: %v", filePath, err))
		}

		catalog := make(map[string]string, len(service.english))
		for key, value := range service.english {
			catalog[key] = value
		}
		for key, value := range overrides {
			catalog[key] = value
		}
		service.localized[locale.Code] = catalog
	}
}
