package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

const displayLanguageSystem = "system"

type appLocale struct {
	Code string
	Name string
}

var supportedAppLocales = []appLocale{
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

var (
	displayLanguageMu         sync.RWMutex
	displayLanguage           = "en-US"
	displayLanguagePreference = displayLanguageSystem
	windowsLocaleName         = readWindowsLocaleName
)

var englishMessages map[string]string
var localizedMessages map[string]map[string]string

func init() {
	englishMessages = make(map[string]string)
	localizedMessages = make(map[string]map[string]string)

	// Load en-US.json as the baseline
	enData, err := localesFS.ReadFile("locales/en-US.json")
	if err != nil {
		panic(fmt.Sprintf("failed to read locales/en-US.json: %v", err))
	}
	if err := json.Unmarshal(enData, &englishMessages); err != nil {
		panic(fmt.Sprintf("failed to parse locales/en-US.json: %v", err))
	}

	// Load all supported locales
	for _, locale := range supportedAppLocales {
		filePath := fmt.Sprintf("locales/%s.json", locale.Code)
		data, err := localesFS.ReadFile(filePath)
		if err != nil {
			panic(fmt.Sprintf("failed to read %s: %v", filePath, err))
		}

		var overrides map[string]string
		if err := json.Unmarshal(data, &overrides); err != nil {
			panic(fmt.Sprintf("failed to parse %s: %v", filePath, err))
		}

		// Merge overrides with english baseline fallback
		catalog := make(map[string]string, len(englishMessages))
		for k, v := range englishMessages {
			catalog[k] = v
		}
		for k, v := range overrides {
			catalog[k] = v
		}
		localizedMessages[locale.Code] = catalog
	}
}

func tr(key string, args ...any) string {
	displayLanguageMu.RLock()
	locale := displayLanguage
	displayLanguageMu.RUnlock()
	text := localizedMessages[locale][key]
	if text == "" {
		text = englishMessages[key]
	}
	if len(args) == 0 {
		return text
	}
	return fmt.Sprintf(text, args...)
}

func currentDisplayLanguage() string {
	displayLanguageMu.RLock()
	defer displayLanguageMu.RUnlock()
	return displayLanguage
}

func currentDisplayLanguagePreference() string {
	displayLanguageMu.RLock()
	defer displayLanguageMu.RUnlock()
	return displayLanguagePreference
}

func displayLanguageName(code string) string {
	for _, locale := range supportedAppLocales {
		if locale.Code == code {
			return locale.Name
		}
	}
	return "English"
}

func selectDisplayLanguage(preference string) bool {
	if preference == "" {
		preference = displayLanguageSystem
	}
	resolved := resolveDisplayLanguage(preference)
	displayLanguageMu.Lock()
	changed := displayLanguage != resolved || displayLanguagePreference != preference
	displayLanguage = resolved
	displayLanguagePreference = preference
	displayLanguageMu.Unlock()
	if !changed {
		return false
	}
	saveSettingsToDisk()
	requestStatusRefresh()
	if ShowOverlay.Load() {
		redrawOverlay()
	}
	return true
}

func applyDisplayLanguagePreference(preference string) {
	if preference == "" {
		preference = displayLanguageSystem
	}
	displayLanguageMu.Lock()
	displayLanguagePreference = preference
	displayLanguage = resolveDisplayLanguage(preference)
	displayLanguageMu.Unlock()
}

func resolveDisplayLanguage(preference string) string {
	if preference != "" && preference != displayLanguageSystem {
		if supportedDisplayLanguage(preference) {
			return preference
		}
	}
	return mapSystemLocale(windowsLocaleName())
}

func supportedDisplayLanguage(code string) bool {
	for _, locale := range supportedAppLocales {
		if locale.Code == code {
			return true
		}
	}
	return false
}

func mapSystemLocale(locale string) string {
	normalized := strings.ToLower(strings.ReplaceAll(locale, "_", "-"))
	for _, supported := range supportedAppLocales {
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
	for _, supported := range supportedAppLocales {
		if strings.HasPrefix(normalized, strings.ToLower(supported.Code[:2])) {
			return supported.Code
		}
	}
	return "en-US"
}

func readWindowsLocaleName() string {
	var locale [85]uint16
	if result, _, _ := procGetUserDefaultLocaleName.Call(uintptr(unsafePointer(&locale[0])), uintptr(len(locale))); result == 0 {
		return "en-US"
	}
	return syscall.UTF16ToString(locale[:])
}

func localizedSemanticValue(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "undervalued", "overvalued", "verified", "estimated", "speculative", "active", "normal", "slow":
		return tr("value." + normalized)
	default:
		return value
	}
}

// unsafePointer keeps the Windows call isolated from the rest of the locale code.
func unsafePointer(value *uint16) uintptr { return uintptr(unsafe.Pointer(value)) }
