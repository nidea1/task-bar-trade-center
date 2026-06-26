package app

import (
	"github.com/nidea1/task-bar-trade-center/internal/win32"

	"fmt"
	"github.com/nidea1/task-bar-trade-center/internal/market"
	"syscall"

	"github.com/nidea1/task-bar-trade-center/internal/winapp"
)

func registerDashboardHotkey() {
	if !winapp.RegisterHotkey(activeApp.appHWND, DashboardHotkeyID, MOD_CONTROL|MOD_ALT, VK_I) {
		fmt.Println("Inventory dashboard hotkey could not be registered.")
		return
	}
	fmt.Println("Inventory dashboard hotkey registered: Ctrl+Alt+I.")
}

func unregisterDashboardHotkey() {
	winapp.UnregisterHotkey(activeApp.appHWND, DashboardHotkeyID)
}

func installMarketClickHook() {
	if activeApp.mouseHook != 0 {
		return
	}

	activeApp.mouseHookCallback = syscall.NewCallback(mouseHookProc)
	hook, _, _ := win32.ProcSetWindowsHookExW.Call(WH_MOUSE_LL, activeApp.mouseHookCallback, 0, 0)
	if hook == 0 {
		fmt.Println("Middle-click market hook could not be installed.")
		return
	}
	activeApp.mouseHook = hook
	fmt.Println("Middle-click market hook installed.")
}

func uninstallMarketClickHook() {
	if activeApp.mouseHook == 0 {
		return
	}
	win32.ProcUnhookWindowsHookEx.Call(activeApp.mouseHook)
	activeApp.mouseHook = 0
}

func mouseHookProc(code uintptr, wParam uintptr, lParam uintptr) uintptr {
	if int32(code) >= 0 && uint32(wParam) == WM_MBUTTONDOWN {
		openActiveItemMarketLink()
	}
	ret, _, _ := win32.ProcCallNextHookEx.Call(activeApp.mouseHook, code, wParam, lParam)
	return ret
}

func openActiveItemMarketLink() {
	if !activeApp.showOverlay.Load() {
		return
	}

	itemID := int(activeApp.activeItemID.Load())
	if itemID == 0 {
		return
	}

	config, exists := activeApp.itemMap[itemID]
	if !exists {
		return
	}

	marketURL := steamMarketListingURLForScope(config, market.CurrentScope())
	fmt.Printf("Opening Steam market listing for %s: %s\n", config.Name["en-US"], marketURL)
	openURLInBrowser(marketURL)
}

func openURLInBrowser(targetURL string) {
	winapp.OpenURL(targetURL)
}
