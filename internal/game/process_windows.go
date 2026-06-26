//go:build windows

package game

import (
	"strings"
	"syscall"
	"unsafe"

	"github.com/nidea1/task-bar-trade-center/internal/win32"
)

const (
	Synchronize        = 0x00100000
	Infinite           = 0xFFFFFFFF
	WaitObject0        = 0x00000000
	TH32CS_SNAPPROCESS = 0x00000002
)

type WindowSearchState struct {
	PID  uint32
	HWND uintptr
}

func FindProcessID(processName string) uint32 {
	snapshot, _, _ := win32.ProcCreateToolhelp32Snapshot.Call(TH32CS_SNAPPROCESS, 0)
	if snapshot == ^uintptr(0) {
		return 0
	}
	defer win32.ProcCloseHandle.Call(snapshot)

	var entry win32.PROCESSENTRY32W
	entry.DwSize = uint32(unsafe.Sizeof(entry))

	ret, _, _ := win32.ProcProcess32FirstW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	for ret != 0 {
		exeName := syscall.UTF16ToString(entry.SzExeFile[:])
		if strings.EqualFold(exeName, processName) {
			return entry.Th32ProcessID
		}
		ret, _, _ = win32.ProcProcess32NextW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	}
	return 0
}

func WaitForProcessExit(pid uint32) {
	if pid == 0 {
		return
	}

	processHandle, _, _ := win32.ProcOpenProcess.Call(Synchronize, 0, uintptr(pid))
	if processHandle == 0 {
		return
	}
	defer win32.ProcCloseHandle.Call(processHandle)
	win32.ProcWaitForSingleObject.Call(processHandle, Infinite)
}

func HasProcessExited(processHandle uintptr) bool {
	if processHandle == 0 {
		return true
	}
	result, _, _ := win32.ProcWaitForSingleObject.Call(processHandle, 0)
	return result == WaitObject0
}

func ClientScreenOrigin(hwnd uintptr) (win32.POINT, bool) {
	origin := win32.POINT{}
	ret, _, _ := win32.ProcClientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&origin)))
	if ret == 0 {
		return win32.POINT{}, false
	}
	return origin, true
}

func FindMainWindowByPID(pid uint32) uintptr {
	state := WindowSearchState{PID: pid}
	win32.ProcEnumWindows.Call(syscall.NewCallback(enumWindowsForPID), uintptr(unsafe.Pointer(&state)))
	return state.HWND
}

func enumWindowsForPID(hwnd uintptr, lParam uintptr) uintptr {
	state := (*WindowSearchState)(unsafe.Pointer(lParam))
	if !IsWindowVisible(hwnd) {
		return 1
	}

	var windowPID uint32
	win32.ProcGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&windowPID)))
	if windowPID != state.PID {
		return 1
	}

	var client win32.RECT
	ret, _, _ := win32.ProcGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&client)))
	if ret == 0 || client.Right <= client.Left || client.Bottom <= client.Top {
		return 1
	}

	state.HWND = hwnd
	return 0
}

func IsWindowVisible(hwnd uintptr) bool {
	if hwnd == 0 {
		return false
	}
	ret, _, _ := win32.ProcIsWindowVisible.Call(hwnd)
	return ret != 0
}

func ModuleBaseAddress(processHandle uintptr, moduleName string) uintptr {
	var modules [1024]uintptr
	var cbNeeded uint32

	res, _, _ := win32.ProcEnumProcessModules.Call(processHandle, uintptr(unsafe.Pointer(&modules[0])), unsafe.Sizeof(modules), uintptr(unsafe.Pointer(&cbNeeded)))
	if res == 0 {
		return 0
	}
	count := cbNeeded / uint32(unsafe.Sizeof(modules[0]))
	for i := uint32(0); i < count; i++ {
		var name [266]uint16
		win32.ProcGetModuleBaseNameW.Call(processHandle, modules[i], uintptr(unsafe.Pointer(&name[0])), unsafe.Sizeof(name))
		modName := syscall.UTF16ToString(name[:])
		if strings.EqualFold(modName, moduleName) {
			return modules[i]
		}
	}
	return 0
}
