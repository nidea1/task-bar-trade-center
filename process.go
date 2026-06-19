package main

import (
	"strings"
	"syscall"
	"unsafe"
)

func findProcessID(processName string) uint32 {
	snapshot, _, _ := procCreateToolhelp32Snapshot.Call(TH32CS_SNAPPROCESS, 0)
	if snapshot == ^uintptr(0) {
		return 0
	}
	defer procCloseHandle.Call(snapshot)

	var entry PROCESSENTRY32W
	entry.DwSize = uint32(unsafe.Sizeof(entry))

	ret, _, _ := procProcess32FirstW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	for ret != 0 {
		exeName := syscall.UTF16ToString(entry.SzExeFile[:])
		if strings.EqualFold(exeName, processName) {
			return entry.Th32ProcessID
		}
		ret, _, _ = procProcess32NextW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	}
	return 0
}

func waitForProcessExit(pid uint32) {
	if pid == 0 {
		return
	}

	processHandle, _, _ := procOpenProcess.Call(SYNCHRONIZE, 0, uintptr(pid))
	if processHandle == 0 {
		return
	}
	defer procCloseHandle.Call(processHandle)
	procWaitForSingleObject.Call(processHandle, INFINITE)
}

func hasProcessExited(processHandle uintptr) bool {
	if processHandle == 0 {
		return true
	}
	result, _, _ := procWaitForSingleObject.Call(processHandle, 0)
	return result == WAIT_OBJECT_0
}

func gameClientScreenOrigin() (POINT, bool) {
	if GameProcessID == 0 {
		return POINT{}, false
	}
	if GameWindowHWND == 0 || !isWindowVisible(GameWindowHWND) {
		GameWindowHWND = findMainWindowByPID(GameProcessID)
	}
	if GameWindowHWND == 0 {
		return POINT{}, false
	}

	origin := POINT{}
	ret, _, _ := procClientToScreen.Call(GameWindowHWND, uintptr(unsafe.Pointer(&origin)))
	if ret == 0 {
		GameWindowHWND = 0
		return POINT{}, false
	}
	return origin, true
}

func findMainWindowByPID(pid uint32) uintptr {
	state := WindowSearchState{PID: pid}
	procEnumWindows.Call(syscall.NewCallback(enumWindowsForPID), uintptr(unsafe.Pointer(&state)))
	return state.HWND
}

func enumWindowsForPID(hwnd uintptr, lParam uintptr) uintptr {
	state := (*WindowSearchState)(unsafe.Pointer(lParam))
	if !isWindowVisible(hwnd) {
		return 1
	}

	var windowPID uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&windowPID)))
	if windowPID != state.PID {
		return 1
	}

	var client RECT
	ret, _, _ := procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&client)))
	if ret == 0 || client.Right <= client.Left || client.Bottom <= client.Top {
		return 1
	}

	state.HWND = hwnd
	return 0
}

func isWindowVisible(hwnd uintptr) bool {
	if hwnd == 0 {
		return false
	}
	ret, _, _ := procIsWindowVisible.Call(hwnd)
	return ret != 0
}

func getModuleBaseAddress(processHandle uintptr, moduleName string) uintptr {
	var modules [1024]uintptr
	var cbNeeded uint32

	res, _, _ := procEnumProcessModules.Call(processHandle, uintptr(unsafe.Pointer(&modules[0])), unsafe.Sizeof(modules), uintptr(unsafe.Pointer(&cbNeeded)))
	if res != 0 {
		count := cbNeeded / uint32(unsafe.Sizeof(modules[0]))
		for i := uint32(0); i < count; i++ {
			var name [266]uint16
			procGetModuleBaseNameW.Call(processHandle, modules[i], uintptr(unsafe.Pointer(&name[0])), unsafe.Sizeof(name))
			modName := syscall.UTF16ToString(name[:])
			if strings.EqualFold(modName, moduleName) {
				return modules[i]
			}
		}
	}
	return 0
}
