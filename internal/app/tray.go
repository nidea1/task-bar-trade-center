package app

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/nidea1/task-bar-trade-center/internal/market"
	"github.com/nidea1/task-bar-trade-center/internal/win32"
	"github.com/nidea1/task-bar-trade-center/internal/winapp"
)

func createAppWindow() {
	className, _ := syscall.UTF16PtrFromString(appWindowClassName())
	windowTitle, _ := syscall.UTF16PtrFromString(AppName)
	hInstance, _, _ := win32.ProcGetModuleHandleW.Call(0)
	cxIcon := getSystemMetric(SM_CXICON)
	cxSmIcon := getSystemMetric(SM_CXSMICON)
	activeApp.appIconLarge = loadAppIcon(hInstance, cxIcon)
	activeApp.appIconSmall = loadAppIcon(hInstance, cxSmIcon)

	wcex := win32.WNDCLASSEX{
		Style:         CS_HREDRAW | CS_VREDRAW,
		LpfnWndProc:   syscall.NewCallback(appWndProc),
		HInstance:     hInstance,
		LpszClassName: className,
		HIcon:         activeApp.appIconLarge,
		HIconSm:       activeApp.appIconSmall,
	}
	wcex.CbSize = uint32(unsafe.Sizeof(wcex))

	win32.ProcRegisterClassExW.Call(uintptr(unsafe.Pointer(&wcex)))

	activeApp.appHWND, _, _ = win32.ProcCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowTitle)),
		0,
		0, 0, 0, 0,
		0, 0, hInstance, 0,
	)
	if activeApp.appHWND == 0 {
		fmt.Println("Tray message window could not be created.")
		return
	}
}

func appWindowClassName() string {
	return AppProcessName + "AppWindow"
}

func addTrayIcon() {
	if activeApp.appHWND == 0 || activeApp.trayIconAdded {
		return
	}

	if activeApp.appIconSmall == 0 {
		hInstance, _, _ := win32.ProcGetModuleHandleW.Call(0)
		cxSmIcon := getSystemMetric(SM_CXSMICON)
		activeApp.appIconSmall = loadAppIcon(hInstance, cxSmIcon)
	}
	nid := newNotifyIconData()
	nid.UFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP | NIF_SHOWTIP
	nid.UVersion = NOTIFYICON_VERSION_4

	if !winapp.AddNotifyIcon(&nid) {
		fmt.Println("Tray icon could not be added.")
		return
	}

	winapp.SetNotifyIconVersion(&nid)
	activeApp.trayIconAdded = true
	updateTrayIconTooltip()
}

func loadAppIcon(hInstance uintptr, size int32) uintptr {
	icon, embedded := winapp.LoadIconResource(hInstance, AppIconResourceID, size, IDI_APPLICATION)
	if embedded {
		fmt.Printf("Application icon (%dx%d) loaded successfully.\n", size, size)
		return icon
	}
	fmt.Printf("Embedded application icon (%dx%d) could not be loaded; using default icon.\n", size, size)
	return icon
}

func removeTrayIcon() {
	if activeApp.appHWND == 0 || !activeApp.trayIconAdded {
		return
	}

	nid := newNotifyIconData()
	winapp.DeleteNotifyIcon(&nid)
	activeApp.trayIconAdded = false
}

func newNotifyIconData() win32.NOTIFYICONDATAW {
	nid := win32.NOTIFYICONDATAW{
		HWnd:             activeApp.appHWND,
		UID:              TrayIconID,
		UCallbackMessage: WM_TRAY_ICON,
		HIcon:            activeApp.appIconSmall,
	}
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	copyUTF16(nid.SzTip[:], trayTooltipText())
	return nid
}

func setAppStatus(status int32) {
	previous := activeApp.appStatus.Swap(status)
	requestTrayTooltipUpdate()
	requestStatusRefresh()
	notifyRuntimeStateChange(previous, status)
}

func requestTrayTooltipUpdate() {
	if activeApp.appHWND == 0 {
		return
	}
	win32.ProcPostMessageW.Call(activeApp.appHWND, WM_TRAY_TIP_UPDATE, 0, 0)
}

func requestStatusRefresh() {
	if activeApp.appHWND == 0 {
		return
	}
	win32.ProcPostMessageW.Call(activeApp.appHWND, WM_APP_STATUS_REFRESH, 0, 0)
}

func updateTrayIconTooltip() {
	if activeApp.appHWND == 0 || !activeApp.trayIconAdded {
		return
	}
	nid := newNotifyIconData()
	nid.UFlags = NIF_TIP | NIF_SHOWTIP
	winapp.ModifyNotifyIcon(&nid)
}

func showTrayNotification(title, message string, icon uintptr) {
	if activeApp.appHWND == 0 || !activeApp.trayIconAdded {
		logPrintf("[NOTIFY] Cannot show tray notification. appHWND=%d trayIconAdded=%t. title=%q\n", activeApp.appHWND, activeApp.trayIconAdded, title)
		return
	}
	nid := newNotifyIconData()
	nid.UFlags = NIF_INFO
	copyUTF16(nid.SzInfoTitle[:], title)
	copyUTF16(nid.SzInfo[:], message)
	nid.DwInfoFlags = NIIF_NONE
	if icon != 0 {
		nid.HBalloonIcon = icon
		nid.DwInfoFlags = NIIF_USER | NIIF_LARGE_ICON
	}
	if !winapp.ModifyNotifyIcon(&nid) {
		logPrintf("[NOTIFY] Shell_NotifyIconW (NIM_MODIFY) failed to show balloon notification. title=%q\n", title)
	} else {
		logPrintf("[NOTIFY] Shell_NotifyIconW (NIM_MODIFY) succeeded for title=%q\n", title)
	}
}

func trayTooltipText() string {
	if activeApp.appStatus.Load() == AppStatusReady {
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
	case WM_APP_HOTKEY_UPDATE:
		unregisterDashboardHotkey()
		registerDashboardHotkey()
		return 0
	case WM_APP_HOTKEY_DISABLE:
		unregisterDashboardHotkey()
		return 0
	case WM_APP_HOTKEY_ENABLE:
		registerDashboardHotkey()
		return 0
	case WM_APP_GAME_CLOSED_PROMPT:
		if showYesNoMessageBox(tr("dialog.game_closed.title"), tr("dialog.game_closed.body")) {
			return 1
		}
		return 0
	case WM_APP_OPEN_TRAY_MENU:
		showTrayMenu()
		return 0
	case WM_APP_INVENTORY_REFRESH:
		refreshInventoryPricesFromDashboard()
		return 0
	case WM_HOTKEY:
		if wParam == DashboardHotkeyID {
			openInventoryDashboard()
			return 0
		}
	case WM_COMMAND:
		handleTrayCommand(uint32(wParam & 0xffff))
		return 0
	case WM_CLOSE:
		activeApp.showOverlay.Store(false)
		if activeApp.overlayHWND != 0 {
			win32.ProcShowWindow.Call(activeApp.overlayHWND, SW_HIDE)
		}
		if activeApp.gameProcessHandle != 0 {
			win32.ProcCloseHandle.Call(activeApp.gameProcessHandle)
			activeApp.gameProcessHandle = 0
		}
		win32.ProcDestroyWindow.Call(hWnd)
		return 0
	case WM_DESTROY:
		removeTrayIcon()
		win32.ProcPostQuitMessage.Call(0)
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

func handleTrayCommand(commandID uint32) {
	if language, ok := appLanguageForMenuCommand(commandID); ok {
		selectDisplayLanguage(language)
		return
	}
	if currency, ok := marketCurrencyForMenuCommand(commandID); ok {
		scope, changed, selected := market.SelectCurrency(currency.Code)
		if selected && changed {
			fmt.Printf("Market currency changed to %s.\n", market.FormatScope(scope))
			saveSettingsToDisk()
			refreshActiveMarketPrice()
			if state, ok := rebuildDashboardState("currency-changed"); ok {
				queued := queueInventoryPriceRefresh(state)
				if queued > 0 {
					refreshInventoryDashboardState("currency-price-refresh-queued")
				}
			}
		}
		return
	}
	if region, ok := marketRegionForMenuCommand(commandID); ok {
		scope, changed, selected := market.SelectRegion(region.CurrencyCode, region.CountryCode)
		if selected && changed {
			fmt.Printf("Market region changed to %s.\n", market.FormatScope(scope))
			saveSettingsToDisk()
			refreshActiveMarketPrice()
			if state, ok := rebuildDashboardState("region-changed"); ok {
				queued := queueInventoryPriceRefresh(state)
				if queued > 0 {
					refreshInventoryDashboardState("region-price-refresh-queued")
				}
			}
		}
		return
	}

	switch commandID {
	case MenuRefreshPriceCache:
		if !activeApp.gameReady.Load() {
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
		if !activeApp.gameReady.Load() {
			fmt.Printf("Cannot clear cache while waiting for %s.\n", GameProcessName)
			return
		}
		if activeApp.priceCacheRefreshing.Load() {
			fmt.Println("Cannot clear cache while cached price refresh is running.")
			return
		}
		count := clearPriceCache()
		fmt.Printf("Price cache cleared: %d item(s).\n", count)
	case MenuOpenInventory:
		openInventoryDashboard()
	case MenuRefreshInventory:
		refreshInventoryPricesFromDashboard()
	case MenuForceRefreshInventory:
		forceRefreshInventoryPricesFromDashboard()
	case MenuToggleOverlayMode:
		if activeApp.overlayMode.Load() == OverlayModeDetail {
			activeApp.overlayMode.Store(OverlayModeCompact)
			fmt.Println("Overlay mode switched to Compact.")
		} else {
			activeApp.overlayMode.Store(OverlayModeDetail)
			fmt.Println("Overlay mode switched to Detail.")
		}
		saveSettingsToDisk()
		if activeApp.showOverlay.Load() {
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

// requestAppShutdown posts WM_CLOSE to the app window and asks the Wails host
// to quit. It is safe to call from any goroutine.
func requestAppShutdown() {
	if activeApp.appHWND != 0 {
		win32.ProcPostMessageW.Call(activeApp.appHWND, WM_CLOSE, 0, 0)
	}
	callQuit()
}

func copyUTF16(destination []uint16, value string) {
	winapp.CopyUTF16(destination, value)
}
