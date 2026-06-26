package winapp

import "github.com/nidea1/task-bar-trade-center/internal/win32"

func RegisterHotkey(hwnd uintptr, id int, modifiers uintptr, key uintptr) bool {
	if hwnd == 0 {
		return false
	}
	ret, _, _ := win32.ProcRegisterHotKey.Call(hwnd, uintptr(id), modifiers, key)
	return ret != 0
}

func UnregisterHotkey(hwnd uintptr, id int) {
	if hwnd == 0 {
		return
	}
	win32.ProcUnregisterHotKey.Call(hwnd, uintptr(id))
}
