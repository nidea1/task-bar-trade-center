package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

func createAppWindow() {
	className, _ := syscall.UTF16PtrFromString(appWindowClassName())
	windowTitle, _ := syscall.UTF16PtrFromString(AppName)
	hInstance, _, _ := procGetModuleHandleW.Call(0)
	cxIcon := getSystemMetric(SM_CXICON)
	cxSmIcon := getSystemMetric(SM_CXSMICON)
	AppIconLarge = loadAppIcon(hInstance, cxIcon)
	AppIconSmall = loadAppIcon(hInstance, cxSmIcon)

	wcex := WNDCLASSEX{
		Style:         CS_HREDRAW | CS_VREDRAW,
		LpfnWndProc:   syscall.NewCallback(appWndProc),
		HInstance:     hInstance,
		LpszClassName: className,
		HIcon:         AppIconLarge,
		HIconSm:       AppIconSmall,
	}
	wcex.CbSize = uint32(unsafe.Sizeof(wcex))

	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wcex)))

	AppHWND, _, _ = procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowTitle)),
		0,
		0, 0, 0, 0,
		0, 0, hInstance, 0,
	)
	if AppHWND == 0 {
		fmt.Println("Tray message window could not be created.")
		return
	}
}

func appWindowClassName() string {
	return AppProcessName + "AppWindow"
}

func addTrayIcon() {
	if AppHWND == 0 || TrayIconAdded {
		return
	}

	if AppIconSmall == 0 {
		hInstance, _, _ := procGetModuleHandleW.Call(0)
		cxSmIcon := getSystemMetric(SM_CXSMICON)
		AppIconSmall = loadAppIcon(hInstance, cxSmIcon)
	}
	nid := newNotifyIconData()
	nid.UFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP | NIF_SHOWTIP
	nid.UVersion = NOTIFYICON_VERSION_4

	ret, _, _ := procShellNotifyIcon.Call(NIM_ADD, uintptr(unsafe.Pointer(&nid)))
	if ret == 0 {
		fmt.Println("Tray icon could not be added.")
		return
	}

	procShellNotifyIcon.Call(NIM_SETVERSION, uintptr(unsafe.Pointer(&nid)))
	TrayIconAdded = true
	updateTrayIconTooltip()
}

func loadAppIcon(hInstance uintptr, size int32) uintptr {
	// IMAGE_ICON = 1
	icon, _, _ := procLoadImageW.Call(
		hInstance,
		AppIconResourceID,
		1, // IMAGE_ICON
		uintptr(size),
		uintptr(size),
		0, // fuLoad
	)
	if icon != 0 {
		fmt.Printf("Application icon (%dx%d) loaded from embedded resource.\n", size, size)
		return icon
	}

	fallbackIcon, _, _ := procLoadIconW.Call(0, IDI_APPLICATION)
	fmt.Printf("Embedded application icon (%dx%d) could not be loaded; using default icon.\n", size, size)
	return fallbackIcon
}

func removeTrayIcon() {
	if AppHWND == 0 || !TrayIconAdded {
		return
	}

	nid := newNotifyIconData()
	procShellNotifyIcon.Call(NIM_DELETE, uintptr(unsafe.Pointer(&nid)))
	TrayIconAdded = false
}

func newNotifyIconData() NOTIFYICONDATAW {
	nid := NOTIFYICONDATAW{
		HWnd:             AppHWND,
		UID:              TrayIconID,
		UCallbackMessage: WM_TRAY_ICON,
		HIcon:            AppIconSmall,
	}
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	copyUTF16(nid.SzTip[:], trayTooltipText())
	return nid
}

func setAppStatus(status int32) {
	previous := AppStatus.Swap(status)
	requestTrayTooltipUpdate()
	requestStatusRefresh()
	notifyRuntimeStateChange(previous, status)
}

func requestTrayTooltipUpdate() {
	if AppHWND == 0 {
		return
	}
	procPostMessageW.Call(AppHWND, WM_TRAY_TIP_UPDATE, 0, 0)
}

func requestStatusRefresh() {
	if AppHWND == 0 {
		return
	}
	procPostMessageW.Call(AppHWND, WM_APP_STATUS_REFRESH, 0, 0)
}

func updateTrayIconTooltip() {
	if AppHWND == 0 || !TrayIconAdded {
		return
	}
	nid := newNotifyIconData()
	nid.UFlags = NIF_TIP | NIF_SHOWTIP
	procShellNotifyIcon.Call(NIM_MODIFY, uintptr(unsafe.Pointer(&nid)))
}

func showTrayNotification(title, message string) {
	if AppHWND == 0 || !TrayIconAdded {
		return
	}
	nid := newNotifyIconData()
	nid.UFlags = NIF_INFO
	copyUTF16(nid.SzInfoTitle[:], title)
	copyUTF16(nid.SzInfo[:], message)
	nid.DwInfoFlags = NIIF_INFO
	procShellNotifyIcon.Call(NIM_MODIFY, uintptr(unsafe.Pointer(&nid)))
}

func trayTooltipText() string {
	if AppStatus.Load() == AppStatusReady {
		return AppName
	}
	return AppName + " - " + appStatusText()
}

func appWndProc(hWnd uintptr, msg uint32, wParam uintptr, lParam uintptr) uintptr {
	switch msg {
	case WM_TRAY_ICON:
		switch trayCallbackEvent(lParam) {
		case WM_CONTEXTMENU, WM_RBUTTONUP, WM_LBUTTONUP:
			showTrayMenu()
			return 0
		}
	case WM_TRAY_TIP_UPDATE:
		updateTrayIconTooltip()
		return 0
	case WM_APP_LOCAL_READY:
		startMonitoringAfterLocalInitialization()
		return 0
	case WM_APP_TRAY_NOTIFICATION:
		flushTrayNotifications()
		return 0
	case WM_APP_STATUS_REFRESH:
		updateTrayIconTooltip()
		return 0
	case WM_APP_GAME_CLOSED_PROMPT:
		if showYesNoMessageBox(tr("dialog.game_closed.title"), tr("dialog.game_closed.body")) {
			return 1
		}
		return 0
	case WM_APP_OPEN_TRAY_MENU:
		showTrayMenu()
		return 0
	case WM_COMMAND:
		handleTrayCommand(uint32(wParam & 0xffff))
		return 0
	case WM_CLOSE:
		ShowOverlay.Store(false)
		if OverlayHWND != 0 {
			procShowWindow.Call(OverlayHWND, SW_HIDE)
		}
		if GameProcessHandle != 0 {
			procCloseHandle.Call(GameProcessHandle)
			GameProcessHandle = 0
		}
		procDestroyWindow.Call(hWnd)
		return 0
	case WM_DESTROY:
		removeTrayIcon()
		procPostQuitMessage.Call(0)
		return 0
	}
	return winDefWindowProc(hWnd, msg, wParam, lParam)
}

func trayCallbackEvent(lParam uintptr) uint32 {
	event := uint32(lParam & 0xffff)
	if event != 0 {
		return event
	}
	return uint32(lParam)
}

func showTrayMenu() {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)

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

	var cursor POINT
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&cursor)))
	procSetForegroundWindow.Call(AppHWND)
	procTrackPopupMenu.Call(menu, TPM_RIGHTBUTTON, uintptr(int(cursor.X)), uintptr(int(cursor.Y)), 0, AppHWND, 0)
	procPostMessageW.Call(AppHWND, 0, 0, 0)
}

func appendTrayMenuItem(menu uintptr, flags uint32, id uint32, text string) {
	textUTF16, _ := syscall.UTF16PtrFromString(text)
	procAppendMenuW.Call(menu, uintptr(flags), uintptr(id), uintptr(unsafe.Pointer(textUTF16)))
}

func appendMarketScopeMenus(menu uintptr, scope MarketScope) {
	currencyMenu, _, _ := procCreatePopupMenu.Call()
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
					label := "USD — " + region.Name
					appendTrayMenuItem(currencyMenu, flags, MenuRegionBase+uint32(regionIndex), label)
				}
				continue
			}

			if hasAdditionalRegionSelection(currency) {
				eurRegionMenu, _, _ := procCreatePopupMenu.Call()
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
	languageMenu, _, _ := procCreatePopupMenu.Call()
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
	textUTF16, _ := syscall.UTF16PtrFromString(text)
	procAppendMenuW.Call(menu, MF_POPUP, popupMenu, uintptr(unsafe.Pointer(textUTF16)))
}

func appendTraySeparator(menu uintptr) {
	procAppendMenuW.Call(menu, MF_SEPARATOR, 0, 0)
}

func handleTrayCommand(commandID uint32) {
	if language, ok := appLanguageForMenuCommand(commandID); ok {
		selectDisplayLanguage(language)
		return
	}
	if currency, ok := marketCurrencyForMenuCommand(commandID); ok {
		scope, changed, selected := selectMarketCurrency(currency.Code)
		if selected && changed {
			fmt.Printf("Market currency changed to %s.\n", formatMarketScope(scope))
			saveSettingsToDisk()
			refreshActiveMarketPrice()
		}
		return
	}
	if region, ok := marketRegionForMenuCommand(commandID); ok {
		scope, changed, selected := selectMarketRegion(region.CurrencyCode, region.CountryCode)
		if selected && changed {
			fmt.Printf("Market region changed to %s.\n", formatMarketScope(scope))
			saveSettingsToDisk()
			refreshActiveMarketPrice()
		}
		return
	}

	switch commandID {
	case MenuRefreshPriceCache:
		if !GameReady.Load() {
			fmt.Printf("Cannot refresh cached prices while waiting for %s.\n", GameProcessName)
			return
		}
		count := refreshCachedPricesInBackground()
		switch {
		case count < 0:
			fmt.Println("Cached price refresh is already running.")
		case count == 0:
			fmt.Println("No cached prices to refresh.")
		default:
			fmt.Printf("Queued cached price refresh: %d item(s).\n", count)
		}
	case MenuClearPriceCache:
		if !GameReady.Load() {
			fmt.Printf("Cannot clear cache while waiting for %s.\n", GameProcessName)
			return
		}
		if PriceCacheRefreshing.Load() {
			fmt.Println("Cannot clear cache while cached price refresh is running.")
			return
		}
		count := clearPriceCache()
		fmt.Printf("Price cache cleared: %d item(s).\n", count)
	case MenuToggleOverlayMode:
		if OverlayMode.Load() == OverlayModeDetail {
			OverlayMode.Store(OverlayModeCompact)
			fmt.Println("Overlay mode switched to Compact.")
		} else {
			OverlayMode.Store(OverlayModeDetail)
			fmt.Println("Overlay mode switched to Detail.")
		}
		saveSettingsToDisk()
		if ShowOverlay.Load() {
			redrawOverlay()
		}
	case MenuCheckForUpdates:
		go runManualUpdateCheck()
	case MenuUpdateConfigs:
		go updateGameLayoutConfigs()
	case MenuRestartAdministrator:
		requestElevatedRestart()
	case MenuInstallUpdate:
		installAvailableUpdate()
	case MenuOpenRelease:
		_, releaseURL := updateActionURLs()
		if releaseURL != "" {
			openURLInBrowser(releaseURL)
		}
	case MenuExit:
		requestAppShutdown()
	}
}

func appLanguageForMenuCommand(commandID uint32) (string, bool) {
	if commandID < MenuLanguageBase {
		return "", false
	}
	index := int(commandID - MenuLanguageBase)
	if index < 0 || index >= len(supportedAppLocales) {
		return "", false
	}
	return supportedAppLocales[index].Code, true
}

// requestAppShutdown posts WM_CLOSE to the application window. It is safe to
// call from any goroutine. The message is dispatched on the UI thread where
// appWndProc handles cleanup, DestroyWindow, and PostQuitMessage — ensuring
// the main message loop exits and the single-instance mutex is released.
func requestAppShutdown() {
	if AppHWND != 0 {
		procPostMessageW.Call(AppHWND, WM_CLOSE, 0, 0)
	}
}

func copyUTF16(destination []uint16, value string) {
	if len(destination) == 0 {
		return
	}
	encoded, _ := syscall.UTF16FromString(value)
	if len(encoded) > len(destination) {
		encoded = encoded[:len(destination)]
		encoded[len(encoded)-1] = 0
	}
	copy(destination, encoded)
}
