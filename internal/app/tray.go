package app

import (
	"github.com/nidea1/task-bar-trade-center/internal/market"
	"github.com/nidea1/task-bar-trade-center/internal/win32"

	"fmt"
	"syscall"
	"unsafe"

	"github.com/nidea1/task-bar-trade-center/internal/winapp"
)

func createAppWindow() {
	className, _ := syscall.UTF16PtrFromString(appWindowClassName())
	windowTitle, _ := syscall.UTF16PtrFromString(AppName)
	hInstance, _, _ := procGetModuleHandleW.Call(0)
	cxIcon := getSystemMetric(SM_CXICON)
	cxSmIcon := getSystemMetric(SM_CXSMICON)
	AppIconLarge = loadAppIcon(hInstance, cxIcon)
	AppIconSmall = loadAppIcon(hInstance, cxSmIcon)

	wcex := win32.WNDCLASSEX{
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

	if !winapp.AddNotifyIcon(&nid) {
		fmt.Println("Tray icon could not be added.")
		return
	}

	winapp.SetNotifyIconVersion(&nid)
	TrayIconAdded = true
	updateTrayIconTooltip()
}

func loadAppIcon(hInstance uintptr, size int32) uintptr {
	icon, embedded := winapp.LoadIconResource(hInstance, AppIconResourceID, size, IDI_APPLICATION)
	if embedded {
		fmt.Printf("Application icon (%dx%d) loaded from embedded resource.\n", size, size)
		return icon
	}
	fmt.Printf("Embedded application icon (%dx%d) could not be loaded; using default icon.\n", size, size)
	return icon
}

func removeTrayIcon() {
	if AppHWND == 0 || !TrayIconAdded {
		return
	}

	nid := newNotifyIconData()
	winapp.DeleteNotifyIcon(&nid)
	TrayIconAdded = false
}

func newNotifyIconData() win32.NOTIFYICONDATAW {
	nid := win32.NOTIFYICONDATAW{
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
	winapp.ModifyNotifyIcon(&nid)
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
	winapp.ModifyNotifyIcon(&nid)
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
		}
		return
	}
	if region, ok := marketRegionForMenuCommand(commandID); ok {
		scope, changed, selected := market.SelectRegion(region.CurrencyCode, region.CountryCode)
		if selected && changed {
			fmt.Printf("Market region changed to %s.\n", market.FormatScope(scope))
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
	case MenuOpenInventory:
		openInventoryDashboard()
	case MenuRefreshInventory:
		refreshInventoryPricesFromDashboard()
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

// requestAppShutdown posts WM_CLOSE to the app window and asks the Wails host
// to quit. It is safe to call from any goroutine.
func requestAppShutdown() {
	if AppHWND != 0 {
		procPostMessageW.Call(AppHWND, WM_CLOSE, 0, 0)
	}
	callQuit()
}

func copyUTF16(destination []uint16, value string) {
	winapp.CopyUTF16(destination, value)
}
