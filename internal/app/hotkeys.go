package app

import (
	"fmt"
	"github.com/nidea1/task-bar-trade-center/internal/market"
	"syscall"

	"github.com/nidea1/task-bar-trade-center/internal/winapp"
)

func registerDashboardHotkey() {
	if !winapp.RegisterHotkey(AppHWND, DashboardHotkeyID, MOD_CONTROL|MOD_ALT, VK_I) {
		fmt.Println("Inventory dashboard hotkey could not be registered.")
		return
	}
	fmt.Println("Inventory dashboard hotkey registered: Ctrl+Alt+I.")
}

func unregisterDashboardHotkey() {
	winapp.UnregisterHotkey(AppHWND, DashboardHotkeyID)
}

func installMarketClickHook() {
	if MouseHook != 0 {
		return
	}

	MouseHookCallback = syscall.NewCallback(mouseHookProc)
	hook, _, _ := procSetWindowsHookExW.Call(WH_MOUSE_LL, MouseHookCallback, 0, 0)
	if hook == 0 {
		fmt.Println("Middle-click market hook could not be installed.")
		return
	}
	MouseHook = hook
	fmt.Println("Middle-click market hook installed.")
}

func uninstallMarketClickHook() {
	if MouseHook == 0 {
		return
	}
	procUnhookWindowsHookEx.Call(MouseHook)
	MouseHook = 0
}

func mouseHookProc(code uintptr, wParam uintptr, lParam uintptr) uintptr {
	if int32(code) >= 0 && uint32(wParam) == WM_MBUTTONDOWN {
		openActiveItemMarketLink()
	}
	ret, _, _ := procCallNextHookEx.Call(MouseHook, code, wParam, lParam)
	return ret
}

func openActiveItemMarketLink() {
	if !ShowOverlay.Load() {
		return
	}

	itemID := int(ActiveItemID.Load())
	if itemID == 0 {
		return
	}

	config, exists := ItemMap[itemID]
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
