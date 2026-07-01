package app

import (
	"fmt"

	"github.com/nidea1/task-bar-trade-center/internal/win32"

	"syscall"
	"unsafe"

	"github.com/nidea1/task-bar-trade-center/internal/localization"
	"github.com/nidea1/task-bar-trade-center/internal/market"
)

const displayLanguageSystem = localization.SystemPreference

type appLocale = localization.Locale

var (
	supportedAppLocales = localization.SupportedLocales
	windowsLocaleName   = readWindowsLocaleName
	localeService       = localization.New(func() string { return windowsLocaleName() })
	englishMessages     = localeService.EnglishCatalog()
	localizedMessages   = localeService.Catalogs()
)

func init() {
	market.SetLocalizer(tr, localizedSemanticValue)
}

func tr(key string, args ...any) string {
	return localeService.T(key, args...)
}

func trFallback(key string, fallback string, args ...any) string {
	value := tr(key, args...)
	if value != "" && value != key {
		return value
	}
	if len(args) > 0 {
		return fmt.Sprintf(fallback, args...)
	}
	return fallback
}

func currentDisplayLanguage() string {
	return localeService.Current()
}

func currentDisplayLanguagePreference() string {
	return localeService.Preference()
}

func currentTranslations() map[string]string {
	lang := currentDisplayLanguage()
	if catalog, ok := localizedMessages[lang]; ok {
		return catalog
	}
	return englishMessages
}

func displayLanguageName(code string) string {
	return localeService.DisplayName(code)
}

func selectDisplayLanguage(preference string) bool {
	changed := localeService.Select(preference)
	if !changed {
		return false
	}
	saveSettingsToDisk()
	requestStatusRefresh()
	if activeApp.showOverlay.Load() {
		redrawOverlay()
	}
	rebuildDashboardState("language-changed")
	return true
}

func applyDisplayLanguagePreference(preference string) {
	localeService.Apply(preference)
}

func resolveDisplayLanguage(preference string) string {
	return localeService.Resolve(preference)
}

func supportedDisplayLanguage(code string) bool {
	return localeService.Supported(code)
}

func mapSystemLocale(locale string) string {
	return localization.MapSystemLocale(locale)
}

func readWindowsLocaleName() string {
	var locale [85]uint16
	if result, _, _ := win32.ProcGetUserDefaultLocaleName.Call(uintptr(unsafePointer(&locale[0])), uintptr(len(locale))); result == 0 {
		return "en-US"
	}
	return syscall.UTF16ToString(locale[:])
}

func localizedSemanticValue(value string) string {
	return localization.LocalizedSemanticValue(localeService, value)
}

func unsafePointer(value *uint16) uintptr { return uintptr(unsafe.Pointer(value)) }
