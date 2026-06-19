package main

import (
	"fmt"
	"net/url"
	"syscall"
	"unsafe"
)

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

	marketURL := steamMarketListingURL(config)
	fmt.Printf("Opening Steam market listing for %s: %s\n", config.Name["en-US"], marketURL)
	openURLInBrowser(marketURL)
}

func steamMarketListingURL(config ItemConfig) string {
	return fmt.Sprintf("https://steamcommunity.com/market/listings/%d/%s", steamMarketAppID, url.PathEscape(buildMarketHashName(config)))
}

func openURLInBrowser(targetURL string) {
	operation, _ := syscall.UTF16PtrFromString("open")
	target, _ := syscall.UTF16PtrFromString(targetURL)
	ret, _, _ := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(operation)),
		uintptr(unsafe.Pointer(target)),
		0,
		0,
		SW_SHOWDEFAULT,
	)
	if ret <= 32 {
		fmt.Printf("Could not open browser for Steam market listing. ShellExecute result=%d\n", ret)
	}
}
