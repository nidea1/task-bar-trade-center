package app

import "github.com/nidea1/task-bar-trade-center/internal/winapp"

func showTrayMenu() {
	menu := winapp.NewPopupMenu()
	if menu == 0 {
		return
	}
	defer winapp.DestroyMenu(menu)

	cacheSize := priceCacheSize()
	refreshing := PriceCacheRefreshing.Load()
	ready := GameReady.Load()
	scope := currentMarketScope()

	appendTrayMenuItem(menu, MF_STRING|MF_GRAYED, 0, tr("menu.status", appStatusText()))
	appendTrayMenuItem(menu, MF_STRING|MF_GRAYED, 0, tr("menu.currency_region", formatMarketScope(scope)))
	appendMarketScopeMenus(menu, scope)
	appendLanguageMenu(menu)
	appendTraySeparator(menu)

	refreshFlags := uint32(MF_STRING)
	if !ready || cacheSize == 0 || refreshing {
		refreshFlags |= MF_GRAYED
	}
	clearFlags := uint32(MF_STRING)
	if !ready || cacheSize == 0 || refreshing {
		clearFlags |= MF_GRAYED
	}
	appendTrayMenuItem(menu, refreshFlags, MenuRefreshPriceCache, tr("menu.refresh_cache"))
	appendTrayMenuItem(menu, clearFlags, MenuClearPriceCache, tr("menu.clear_cache"))
	inventoryFlags := uint32(MF_STRING)
	if !ready {
		inventoryFlags |= MF_GRAYED
	}
	appendTrayMenuItem(menu, inventoryFlags, MenuOpenInventory, tr("menu.open_inventory"))
	appendTrayMenuItem(menu, inventoryFlags, MenuRefreshInventory, tr("menu.refresh_inventory"))
	overlayModeText := tr("menu.compact")
	if OverlayMode.Load() == OverlayModeCompact {
		overlayModeText = tr("menu.detail")
	}
	appendTrayMenuItem(menu, MF_STRING, MenuToggleOverlayMode, overlayModeText)
	appendTrayMenuItem(menu, MF_STRING, MenuUpdateConfigs, tr("menu.update_configs"))
	appendTrayMenuItem(menu, MF_STRING, MenuCheckForUpdates, tr("menu.check_updates"))
	if AppStatus.Load() == AppStatusAttachFailed {
		appendTrayMenuItem(menu, MF_STRING, MenuRestartAdministrator, tr("menu.restart_admin"))
	}
	if UpdateStatus.Load() == UpdateStatusAvailable {
		appendTrayMenuItem(menu, MF_STRING, MenuInstallUpdate, tr("menu.install_update"))
	}
	if UpdateStatus.Load() == UpdateStatusFailed {
		_, releaseURL := updateActionURLs()
		if releaseURL != "" {
			appendTrayMenuItem(menu, MF_STRING, MenuOpenRelease, tr("menu.open_release"))
		}
	}
	appendTraySeparator(menu)
	appendTrayMenuItem(menu, MF_STRING|MF_GRAYED, 0, tr("menu.created_by", AppVersion, AppCreatorName))
	appendTrayMenuItem(menu, MF_STRING, MenuExit, tr("menu.exit"))

	winapp.TrackPopupAtCursor(menu, AppHWND, TPM_RIGHTBUTTON)
}

func appendTrayMenuItem(menu uintptr, flags uint32, id uint32, text string) {
	winapp.AppendMenuItem(menu, flags, id, text)
}

func appendMarketScopeMenus(menu uintptr, scope MarketScope) {
	currencyMenu := winapp.NewPopupMenu()
	if currencyMenu != 0 {
		for index, currency := range supportedMarketCurrencies {
			if currency.Code == "USD" {
				for regionIndex, region := range supportedMarketRegions {
					if region.CurrencyCode != "USD" {
						continue
					}
					flags := uint32(MF_STRING)
					if scope.Currency.Code == "USD" && scope.Region.CountryCode == region.CountryCode {
						flags |= MF_CHECKED
					}
					label := "USD â€” " + region.Name
					appendTrayMenuItem(currencyMenu, flags, MenuRegionBase+uint32(regionIndex), label)
				}
				continue
			}

			if hasAdditionalRegionSelection(currency) {
				eurRegionMenu := winapp.NewPopupMenu()
				if eurRegionMenu == 0 {
					appendTrayMenuItem(currencyMenu, MF_STRING|MF_GRAYED, 0, marketCurrencyMenuLabel(currency, scope))
					continue
				}
				for regionIndex, region := range supportedMarketRegions {
					if region.CurrencyCode != currency.Code {
						continue
					}
					flags := uint32(MF_STRING)
					if region.CurrencyCode == scope.Currency.Code && region.CountryCode == scope.Region.CountryCode {
						flags |= MF_CHECKED
					}
					appendTrayMenuItem(eurRegionMenu, flags, MenuRegionBase+uint32(regionIndex), region.Name)
				}
				appendTrayPopupMenu(currencyMenu, eurRegionMenu, marketCurrencyMenuLabel(currency, scope))
				continue
			}

			flags := uint32(MF_STRING)
			if currency.Code == scope.Currency.Code {
				flags |= MF_CHECKED
			}
			appendTrayMenuItem(currencyMenu, flags, MenuCurrencyBase+uint32(index), marketCurrencyMenuLabel(currency, scope))
		}
		appendTrayPopupMenu(menu, currencyMenu, tr("menu.currency"))
	}
}

func appendLanguageMenu(menu uintptr) {
	languageMenu := winapp.NewPopupMenu()
	if languageMenu == 0 {
		return
	}
	current := currentDisplayLanguage()
	for index, locale := range supportedAppLocales {
		flags := uint32(MF_STRING)
		if locale.Code == current {
			flags |= MF_CHECKED
		}
		appendTrayMenuItem(languageMenu, flags, MenuLanguageBase+uint32(index), locale.Name)
	}
	appendTrayPopupMenu(menu, languageMenu, tr("menu.language"))
}

func appendTrayPopupMenu(menu uintptr, popupMenu uintptr, text string) {
	winapp.AppendPopupMenu(menu, popupMenu, text, MF_POPUP)
}

func appendTraySeparator(menu uintptr) {
	winapp.AppendSeparator(menu, MF_SEPARATOR)
}
