package app

import "github.com/nidea1/task-bar-trade-center/internal/game"

func findProcessID(processName string) uint32 {
	return game.FindProcessID(processName)
}

func waitForProcessExit(pid uint32) {
	game.WaitForProcessExit(pid)
}

func hasProcessExited(processHandle uintptr) bool {
	return game.HasProcessExited(processHandle)
}

func gameClientScreenOrigin() (POINT, bool) {
	if GameProcessID == 0 {
		return POINT{}, false
	}
	if GameWindowHWND == 0 || !game.IsWindowVisible(GameWindowHWND) {
		GameWindowHWND = game.FindMainWindowByPID(GameProcessID)
	}
	if GameWindowHWND == 0 {
		return POINT{}, false
	}

	origin, ok := game.ClientScreenOrigin(GameWindowHWND)
	if !ok {
		GameWindowHWND = 0
		return POINT{}, false
	}
	return origin, true
}

func findMainWindowByPID(pid uint32) uintptr {
	return game.FindMainWindowByPID(pid)
}

func isWindowVisible(hwnd uintptr) bool {
	return game.IsWindowVisible(hwnd)
}

func getModuleBaseAddress(processHandle uintptr, moduleName string) uintptr {
	return game.ModuleBaseAddress(processHandle, moduleName)
}
