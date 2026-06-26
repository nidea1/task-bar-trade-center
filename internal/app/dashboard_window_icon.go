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
	if largeIcon := loadAppIcon(hInstance, getSystemMetric(SM_CXICON)); largeIcon != 0 {
		win32.ProcSendMessageW.Call(hwnd, WM_SETICON, ICON_BIG, largeIcon)
	}
	if smallIcon := loadAppIcon(hInstance, getSystemMetric(SM_CXSMICON)); smallIcon != 0 {
		win32.ProcSendMessageW.Call(hwnd, WM_SETICON, ICON_SMALL, smallIcon)
	}
}
