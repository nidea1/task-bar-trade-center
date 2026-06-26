package app

import (
	"github.com/nidea1/task-bar-trade-center/internal/market"
	"github.com/nidea1/task-bar-trade-center/internal/win32"

	"fmt"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"github.com/nidea1/task-bar-trade-center/internal/game"
)

func Run() {
	runtime.LockOSThread()
	hideConsoleWindowIfNeeded()

	createAppWindow()
	setAppStatus(AppStatusStarting)
	addTrayIcon()
	startedAt := time.Now()
	go initializeApplication(startedAt)
	runOverlayMessageLoop()
	shutdownApplication()
	closeAppStorage()
	if AppHWND != 0 {
		removeTrayIcon()
	}
	if GameProcessHandle != 0 {
		procCloseHandle.Call(GameProcessHandle)
		GameProcessHandle = 0
	}
}

func Stop() {
	if AppHWND != 0 {
		procPostMessageW.Call(AppHWND, WM_CLOSE, 0, 0)
	}
}

func initializeApplication(startedAt time.Time) {
	initAppStorage()
	fmt.Printf("%s is starting...\n", AppName)
	fmt.Printf("startup tray_ready=%s\n", time.Since(startedAt))
	cleanOldVersion()
	GameReady.Store(false)

	if err := loadItemsJSON(); err != nil {
		failApplicationInitialization(fmt.Errorf("items database: %w", err))
		return
	}
	loadPriceCacheFromDisk()
	loadSettingsFromDisk()
	notifyApplicationStarted()
	if err := loadLocalGameLayout(); err != nil {
		failApplicationInitialization(fmt.Errorf("game layout: %w", err))
		return
	}

	fmt.Printf("startup local_ready=%s\n", time.Since(startedAt))
	procPostMessageW.Call(AppHWND, WM_APP_LOCAL_READY, 0, 0)
}

func failApplicationInitialization(err error) {
	fmt.Printf("Application initialization failed: %v\n", err)
	notifyApplicationStarted()
	setConfigurationStatus(ConfigStatusRefreshFailed, err.Error())
	setAppStatus(AppStatusInitializationFailed)
}

func startMonitoringAfterLocalInitialization() {
	if !AppInitialized.CompareAndSwap(false, true) {
		return
	}
	if EnablePriceHUD {
		createOverlayWindow()
	}
	registerDashboardHotkey()
	installMarketClickHook()
	go attachGameAndWatchHoveredItems()
	go checkUpdatesOnStartup()
	go refreshGameLayoutInBackground()
	go market.FetchExchangeRatesFromAPI()
	fmt.Println("startup monitor_ready")
}

func shutdownApplication() {
	unregisterDashboardHotkey()
	uninstallMarketClickHook()
}

func attachGameAndWatchHoveredItems() {
	for {
		setAppStatus(AppStatusWaitingForGame)
		pid := waitForGameProcess()
		pHandle, ok := openGameProcess(pid)
		if !ok {
			setAppStatus(AppStatusAttachFailed)
			return
		}

		setAppStatus(AppStatusWaitingForGameAssembly)
		gameAssemblyBase, gameStillRunning := waitForGameAssembly(pHandle)
		if !gameStillRunning {
			procCloseHandle.Call(pHandle)
			if handleGameClosed() {
				return
			}
			continue
		}

		configureGameProcess(pid, pHandle, gameAssemblyBase)
		GameReady.Store(true)
		setAppStatus(AppStatusReady)
		go refreshInventoryDashboardState("game-attached")
		watchHoveredItems(pHandle, gameAssemblyBase)
		if handleGameClosed() {
			return
		}
	}
}

func waitForGameProcess() uint32 {
	var pid uint32
	for pid == 0 {
		pid = game.FindProcessID(GameProcessName)
		if pid == 0 {
			fmt.Printf("Waiting for %s... Launch the game.\n", GameProcessName)
			time.Sleep(2 * time.Second)
		}
	}
	fmt.Printf("Game process ID (PID) found: %d\n", pid)
	return pid
}

func openGameProcess(pid uint32) (uintptr, bool) {
	pHandle, _, _ := procOpenProcess.Call(PROCESS_VM_READ|PROCESS_QUERY_INFORMATION|SYNCHRONIZE, 0, uintptr(pid))
	if pHandle == 0 {
		fmt.Println("Could not attach to game memory. Please run the command prompt as administrator.")
		return 0, false
	}
	fmt.Println("Successfully attached to game memory.")
	return pHandle, true
}

func waitForGameAssembly(processHandle uintptr) (uintptr, bool) {
	var gameAssemblyBase uintptr
	for gameAssemblyBase == 0 {
		if game.HasProcessExited(processHandle) {
			return 0, false
		}
		gameAssemblyBase = game.ModuleBaseAddress(processHandle, "GameAssembly.dll")
		if gameAssemblyBase == 0 {
			fmt.Println("Waiting for GameAssembly.dll to load into memory...")
			time.Sleep(1 * time.Second)
		}
	}
	fmt.Printf("GameAssembly.dll address verified: 0x%X\n", gameAssemblyBase)
	return gameAssemblyBase, true
}

func configureGameProcess(pid uint32, processHandle uintptr, gameAssemblyBase uintptr) {
	GameProcessHandle = processHandle
	GameProcessID = pid
	GameAssemblyBase = gameAssemblyBase
	GameLayoutReadHealth.Reset()
	TooltipXAOBResolver.Reset()
	TooltipYAOBResolver.Reset()
	TooltipHeightAOBResolver.Reset()
}

func watchHoveredItems(pHandle uintptr, gameAssemblyBase uintptr) {
	var lastID int32 = 0
	lastReadFailed := false
	var lastUnknownRaw int32 = 0
	aobResolver := game.HoveredItemAOBResolver{}

	for {
		if game.HasProcessExited(pHandle) {
			return
		}

		GameLayoutMu.RLock()
		layout := ActiveGameLayout
		GameLayoutMu.RUnlock()

		currentItemID, readMode, rawValue, ok := aobResolver.Read(pHandle, gameAssemblyBase, layout, marketableItemExists)
		if !ok {
			recordPointerReadResult(game.PointerReadHoveredItem, false)
			if !lastReadFailed {
				fmt.Printf("Memory read failed. The AOB pattern or pointer/offset chain may be outdated.\n")
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
		recordPointerReadResult(game.PointerReadHoveredItem, true)
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
					lang := currentDisplayLanguage()
					name := config.Name[lang]
					if name == "" {
						name = config.Name["en-US"]
					}
					setCurrentItemName(name)
					fmt.Printf("Mouse is over item: %s (ID: %d)\n", getCurrentItemName(), currentItemID)
					fmt.Printf("Item ID read mode: %s\n", readMode)
					if EnablePriceHUD {
						ShowOverlay.Store(true)
						setCurrentPriceText(tr("hud.loading"))
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

func handleGameClosed() bool {
	shouldClose := askGameClosedOnUIThread()
	resetGameProcess()
	if !shouldClose {
		return false
	}
	requestAppShutdown()
	return true
}

func askGameClosedOnUIThread() bool {
	if AppHWND == 0 {
		return showYesNoMessageBox(tr("dialog.game_closed.title"), tr("dialog.game_closed.body"))
	}
	result, _, _ := procSendMessageW.Call(AppHWND, WM_APP_GAME_CLOSED_PROMPT, 0, 0)
	return result != 0
}

func resetGameProcess() {
	GameReady.Store(false)
	ActiveItemID.Store(0)
	setCurrentItemName("")
	ShowOverlay.Store(false)
	HasLastOverlayRect = false
	redrawOverlay()
	if GameProcessHandle != 0 {
		procCloseHandle.Call(GameProcessHandle)
		GameProcessHandle = 0
	}
	GameProcessID = 0
	GameAssemblyBase = 0
	GameWindowHWND = 0
	GameLayoutReadHealth.Reset()
	TooltipXAOBResolver.Reset()
	TooltipYAOBResolver.Reset()
	TooltipHeightAOBResolver.Reset()
	setAppStatus(AppStatusWaitingForGame)
}

func marketableItemExists(itemID int32) bool {
	item, exists := AllItemMap[int(itemID)]
	return exists && item.Marketable
}

func runOverlayMessageLoop() {
	var msg win32.MSG
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

func activateExistingInstance() bool {
	className, _ := syscall.UTF16PtrFromString(appWindowClassName())
	for i := 0; i < 10; i++ {
		hwnd, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(className)), 0)
		if hwnd != 0 {
			procPostMessageW.Call(hwnd, WM_APP_OPEN_TRAY_MENU, 0, 0)
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

func showAlreadyRunningMessage() {
	text, _ := syscall.UTF16PtrFromString(tr("dialog.already_running"))
	caption, _ := syscall.UTF16PtrFromString("Task Bar Trade Center")
	// MB_OK = 0x00000000 | MB_ICONWARNING = 0x00000030
	procMessageBoxW.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(caption)), 0x00000000|0x00000030)
}
