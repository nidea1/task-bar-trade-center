package main

import (
	"fmt"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

func runApp() {
	runtime.LockOSThread()
	hideConsoleWindowIfNeeded()

	mutex, ok := checkSingleInstance()
	if !ok {
		showAlreadyRunningMessage()
		return
	}
	defer procCloseHandle.Call(mutex)

	initAppStorage()
	defer closeAppStorage()

	cleanOldVersion()

	fmt.Printf("%s is starting...\n", AppName)
	GameReady.Store(false)
	AppStatus.Store(AppStatusWaitingForGame)
	loadItemsJSON()
	loadPriceCacheFromDisk()
	loadSettingsFromDisk()
	createAppWindow()
	addTrayIcon()
	installMarketClickHook()

	if EnablePriceHUD {
		createOverlayWindow()
	}

	go attachGameAndWatchHoveredItems()
	go checkUpdatesOnStartup()
	runOverlayMessageLoop()
	uninstallMarketClickHook()
	removeTrayIcon()
	if GameProcessHandle != 0 {
		procCloseHandle.Call(GameProcessHandle)
		GameProcessHandle = 0
	}
}

func attachGameAndWatchHoveredItems() {
	setAppStatus(AppStatusWaitingForGame)
	pid := waitForGameProcess()
	pHandle, ok := openGameProcess(pid)
	if !ok {
		setAppStatus(AppStatusAttachFailed)
		return
	}

	setAppStatus(AppStatusWaitingForGameAssembly)
	gameAssemblyBase := waitForGameAssembly(pHandle)
	configureGameProcess(pid, pHandle, gameAssemblyBase)
	GameReady.Store(true)
	setAppStatus(AppStatusReady)
	watchHoveredItems(pHandle, gameAssemblyBase)
}

func waitForGameProcess() uint32 {
	var pid uint32
	for pid == 0 {
		pid = findProcessID(GameProcessName)
		if pid == 0 {
			fmt.Printf("Waiting for %s... Launch the game.\n", GameProcessName)
			time.Sleep(2 * time.Second)
		}
	}
	fmt.Printf("Game process ID (PID) found: %d\n", pid)
	return pid
}

func openGameProcess(pid uint32) (uintptr, bool) {
	pHandle, _, _ := procOpenProcess.Call(PROCESS_VM_READ|PROCESS_QUERY_INFORMATION, 0, uintptr(pid))
	if pHandle == 0 {
		fmt.Println("Could not attach to game memory. Please run the command prompt as administrator.")
		return 0, false
	}
	fmt.Println("Successfully attached to game memory.")
	return pHandle, true
}

func waitForGameAssembly(processHandle uintptr) uintptr {
	var gameAssemblyBase uintptr
	for gameAssemblyBase == 0 {
		gameAssemblyBase = getModuleBaseAddress(processHandle, "GameAssembly.dll")
		if gameAssemblyBase == 0 {
			fmt.Println("Waiting for GameAssembly.dll to load into memory...")
			time.Sleep(1 * time.Second)
		}
	}
	fmt.Printf("GameAssembly.dll address verified: 0x%X\n", gameAssemblyBase)
	return gameAssemblyBase
}

func configureGameProcess(pid uint32, processHandle uintptr, gameAssemblyBase uintptr) {
	GameProcessHandle = processHandle
	GameProcessID = pid
	GameAssemblyBase = gameAssemblyBase
}

func watchHoveredItems(pHandle uintptr, gameAssemblyBase uintptr) {
	baseAddress := gameAssemblyBase + 0x05D59190
	offsets := []uintptr{0x20, 0x80, 0x30, 0xB8, 0x8, 0x40, 0x338}

	var lastID int32 = 0
	lastReadFailed := false
	var lastUnknownRaw int32 = 0

	for {
		currentItemID, readMode, rawValue, ok := readHoveredItemID(pHandle, baseAddress, offsets)
		if !ok {
			if !lastReadFailed {
				fmt.Println("Memory read failed. The pointer/offset chain may be outdated.")
				lastReadFailed = true
			}
			ActiveItemID.Store(0)
			if EnablePriceHUD && ShowOverlay.Load() {
				ShowOverlay.Store(false)
				redrawOverlay()
			}
			time.Sleep(200 * time.Millisecond)
			continue
		}
		lastReadFailed = false
		if currentItemID == 0 && rawValue > 0 && rawValue != lastUnknownRaw {
			lastUnknownRaw = rawValue
			fmt.Printf("Read raw tooltip value is not an item ID: %d\n", rawValue)
		}

		if currentItemID != lastID {
			lastID = currentItemID
			ActiveItemID.Store(currentItemID)
			if currentItemID > 0 {
				if config, exists := ItemMap[int(currentItemID)]; exists {
					setCurrentItemName(config.Name["en-US"])
					fmt.Printf("Mouse is over item: %s (ID: %d)\n", getCurrentItemName(), currentItemID)
					fmt.Printf("Item ID read mode: %s\n", readMode)
					if EnablePriceHUD {
						ShowOverlay.Store(true)
						setCurrentPriceText("Loading price...")
						redrawOverlay()
					}
					go fetchPriceAndUpdate(config)
				} else {
					ActiveItemID.Store(0)
					if EnablePriceHUD {
						ShowOverlay.Store(false)
						redrawOverlay()
					}
					fmt.Printf("Read item ID is not in the marketable list: %d\n", currentItemID)
				}
			} else {
				ActiveItemID.Store(0)
				if EnablePriceHUD {
					ShowOverlay.Store(false)
					redrawOverlay()
				}
			}
		}
		if EnablePriceHUD && ShowOverlay.Load() {
			redrawOverlay()
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func runOverlayMessageLoop() {
	var msg MSG
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if ret == 0 || ret == ^uintptr(0) {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func hideConsoleWindowIfNeeded() {
	hwnd, _, _ := procGetConsoleWindow.Call()
	if hwnd == 0 {
		return
	}

	var processList [2]uint32
	count, _, _ := procGetConsoleProcessList.Call(uintptr(unsafe.Pointer(&processList[0])), uintptr(len(processList)))
	if count == 1 {
		// Only this process is attached to the console (e.g. launched from Explorer).
		// Hide the console window.
		// SW_HIDE = 0
		procShowWindow.Call(hwnd, 0)
	}
}

func checkSingleInstance() (uintptr, bool) {
	name, _ := syscall.UTF16PtrFromString("Local\\TaskBarTradeCenterUniqueMutex")
	// CreateMutexW(nil, FALSE, name)
	mutex, _, err := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if mutex == 0 {
		return 0, false
	}
	if err != nil && err.(syscall.Errno) == syscall.ERROR_ALREADY_EXISTS {
		procCloseHandle.Call(mutex)
		return 0, false
	}
	return mutex, true
}

func showAlreadyRunningMessage() {
	text, _ := syscall.UTF16PtrFromString("Task Bar Trade Center is already running.")
	caption, _ := syscall.UTF16PtrFromString("Task Bar Trade Center")
	// MB_OK = 0x00000000 | MB_ICONWARNING = 0x00000030
	procMessageBoxW.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(caption)), 0x00000000|0x00000030)
}
