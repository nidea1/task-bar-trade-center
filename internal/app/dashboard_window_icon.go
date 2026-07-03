package app

import (
	"syscall"
	"unsafe"

	"github.com/nidea1/task-bar-trade-center/internal/win32"
)

func ApplyDashboardWindowIcon() {
	className, err := syscall.UTF16PtrFromString(DashboardWindowClassName)
	if err != nil {
		return
	}
	hwnd, _, _ := win32.ProcFindWindowW.Call(uintptr(unsafe.Pointer(className)), 0)
	if hwnd == 0 {
		return
	}

	hInstance, _, _ := win32.ProcGetModuleHandleW.Call(0)
	largeIcon := cachedAppIcon(hInstance, true)
	if largeIcon != 0 && windowIcon(hwnd, ICON_BIG) != largeIcon {
		win32.ProcSendMessageW.Call(hwnd, WM_SETICON, ICON_BIG, largeIcon)
	}
	smallIcon := cachedAppIcon(hInstance, false)
	if smallIcon != 0 && windowIcon(hwnd, ICON_SMALL) != smallIcon {
		win32.ProcSendMessageW.Call(hwnd, WM_SETICON, ICON_SMALL, smallIcon)
	}
}

func cachedAppIcon(hInstance uintptr, large bool) uintptr {
	if large {
		if activeApp.appIconLarge == 0 {
			activeApp.appIconLarge = loadAppIcon(hInstance, getSystemMetric(SM_CXICON))
		}
		return activeApp.appIconLarge
	}
	if activeApp.appIconSmall == 0 {
		activeApp.appIconSmall = loadAppIcon(hInstance, getSystemMetric(SM_CXSMICON))
	}
	return activeApp.appIconSmall
}

func windowIcon(hwnd uintptr, iconType uintptr) uintptr {
	icon, _, _ := win32.ProcSendMessageW.Call(hwnd, WM_GETICON, iconType, 0)
	return icon
}
