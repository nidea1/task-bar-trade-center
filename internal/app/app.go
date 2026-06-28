package app

import (
	"fmt"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"github.com/nidea1/task-bar-trade-center/internal/game"
	"github.com/nidea1/task-bar-trade-center/internal/market"
	"github.com/nidea1/task-bar-trade-center/internal/win32"
)

func Run() {
	runtime.LockOSThread()
	hideConsoleWindowIfNeeded()

	configureNotificationIdentity()
	createAppWindow()
	setAppStatus(AppStatusStarting)
	addTrayIcon()
	startedAt := time.Now()
	go initializeApplication(startedAt)
	runOverlayMessageLoop()
	shutdownApplication()
	closeAppStorage()
	if activeApp.appHWND != 0 {
		removeTrayIcon()
	}
	if activeApp.gameProcessHandle != 0 {
		win32.ProcCloseHandle.Call(activeApp.gameProcessHandle)
		activeApp.gameProcessHandle = 0
	}
}

func Stop() {
	if activeApp.appHWND != 0 {
		win32.ProcPostMessageW.Call(activeApp.appHWND, WM_CLOSE, 0, 0)
	}
}

func initializeApplication(startedAt time.Time) {
	initAppStorage()
	logPrintf("%s is starting...\n", AppName)
	logPrintf("startup tray_ready=%s\n", time.Since(startedAt))
	cleanOldVersion()
	activeApp.gameReady.Store(false)

	if err := loadItemsJSON(); err != nil {
		failApplicationInitialization(fmt.Errorf("items database: %w", err))
		return
	}
	loadPriceCacheFromDisk()
	loadIconMetadataFromDisk()
	loadSettingsFromDisk()
	notifyApplicationStarted()
	if err := loadLocalGameLayout(); err != nil {
		failApplicationInitialization(fmt.Errorf("game layout: %w", err))
		return
	}

	logPrintf("startup local_ready=%s\n", time.Since(startedAt))
	win32.ProcPostMessageW.Call(activeApp.appHWND, WM_APP_LOCAL_READY, 0, 0)
}

func failApplicationInitialization(err error) {
	logPrintf("Application initialization failed: %v\n", err)
	notifyApplicationStarted()
	setConfigurationStatus(ConfigStatusRefreshFailed, err.Error())
	setAppStatus(AppStatusInitializationFailed)
}

func startMonitoringAfterLocalInitialization() {
	if !activeApp.appInitialized.CompareAndSwap(false, true) {
		return
	}
	if EnablePriceHUD {
		createOverlayWindow()
	}
	registerDashboardHotkey()
	installMarketClickHook()
	go watchLocalGameLayoutChanges()
	go attachGameAndWatchHoveredItems()
	go checkUpdatesOnStartup()
	go refreshGameLayoutInBackground()
	go market.FetchExchangeRatesFromAPI()
	logPrintln("startup monitor_ready")
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
			win32.ProcCloseHandle.Call(pHandle)
			if handleGameClosed() {
				return
			}
			continue
		}

		configureGameProcess(pid, pHandle, gameAssemblyBase)
		activeApp.gameReady.Store(true)
		setAppStatus(AppStatusReady)
		go refreshInventoryDashboardState("game-attached")
		go monitorInventoryNotifications(pHandle)
		go preScanTooltipAOB()
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
			logPrintf("Waiting for %s... Launch the game.\n", GameProcessName)
			time.Sleep(2 * time.Second)
		}
	}
	logPrintf("Game process ID (PID) found: %d\n", pid)
	return pid
}

func openGameProcess(pid uint32) (uintptr, bool) {
	pHandle, _, _ := win32.ProcOpenProcess.Call(PROCESS_VM_READ|PROCESS_QUERY_INFORMATION|SYNCHRONIZE, 0, uintptr(pid))
	if pHandle == 0 {
		logPrintln("Could not attach to game memory. Please run the command prompt as administrator.")
		return 0, false
	}
	logPrintln("Successfully attached to game memory.")
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
			logPrintln("Waiting for GameAssembly.dll to load into memory...")
			time.Sleep(1 * time.Second)
		}
	}
	logPrintf("GameAssembly.dll address verified: 0x%X\n", gameAssemblyBase)
	return gameAssemblyBase, true
}

func configureGameProcess(pid uint32, processHandle uintptr, gameAssemblyBase uintptr) {
	activeApp.gameProcessHandle = processHandle
	activeApp.gameProcessID = pid
	activeApp.gameAssemblyBase = gameAssemblyBase
	activeApp.gameLayoutReadHealth.Reset()
	activeApp.tooltipXAOBResolver.Reset()
	activeApp.tooltipYAOBResolver.Reset()
	activeApp.tooltipHeightAOBResolver.Reset()
	resetMarketableInventoryNotifications()
	triggerSavePollNow = false
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

		activeApp.gameLayoutMu.RLock()
		layout := activeApp.activeGameLayout
		activeApp.gameLayoutMu.RUnlock()

		currentItemID, readMode, rawValue, ok := aobResolver.Read(pHandle, gameAssemblyBase, layout, marketableItemExists)
		if !ok {
			recordPointerReadResult(game.PointerReadHoveredItem, false)
			if !lastReadFailed {
				logPrintf("Memory read failed. The AOB pattern or pointer/offset chain may be outdated.\n")
				lastReadFailed = true
			}
			activeApp.activeItemID.Store(0)
			if EnablePriceHUD && activeApp.showOverlay.Load() {
				activeApp.showOverlay.Store(false)
				redrawOverlay()
			}
			time.Sleep(200 * time.Millisecond)
			continue
		}
		recordPointerReadResult(game.PointerReadHoveredItem, true)
		lastReadFailed = false
		if currentItemID == 0 && rawValue > 0 && rawValue != lastUnknownRaw {
			lastUnknownRaw = rawValue
			logPrintf("Read raw tooltip value is not an item ID: %d\n", rawValue)
		}

		if currentItemID != lastID {
			lastID = currentItemID
			activeApp.activeItemID.Store(currentItemID)
			if currentItemID > 0 {
				if config, exists := activeApp.itemMap[int(currentItemID)]; exists {
					lang := currentDisplayLanguage()
					name := config.Name[lang]
					if name == "" {
						name = config.Name["en-US"]
					}
					setCurrentItemName(name)
					logCurrentTooltipCalibration()
					logPrintf("Mouse is over item: %s (ID: %d)\n", getCurrentItemName(), currentItemID)
					logPrintf("Item ID read mode: %s\n", readMode)
					if EnablePriceHUD {
						activeApp.showOverlay.Store(true)
						setCurrentPriceText(tr("hud.loading"))
						redrawOverlay()
					}
					go fetchPriceAndUpdate(config)
				} else {
					activeApp.activeItemID.Store(0)
					if EnablePriceHUD {
						activeApp.showOverlay.Store(false)
						redrawOverlay()
					}
					logPrintf("Read item ID is not in the marketable list: %d\n", currentItemID)
				}
			} else {
				activeApp.activeItemID.Store(0)
				if EnablePriceHUD {
					activeApp.showOverlay.Store(false)
					redrawOverlay()
				}
			}
		}
		if EnablePriceHUD && activeApp.showOverlay.Load() {
			redrawOverlay()
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func handleGameClosed() bool {
	shouldClose := askGameClosedOnUIThread()
	if shouldClose {
		activeApp.shutdownRequested.Store(true)
	}
	resetGameProcess()
	if !shouldClose {
		return false
	}
	requestAppShutdown()
	return true
}

func askGameClosedOnUIThread() bool {
	if activeApp.appHWND == 0 {
		return showYesNoMessageBox(tr("dialog.game_closed.title"), tr("dialog.game_closed.body"))
	}
	result, _, _ := win32.ProcSendMessageW.Call(activeApp.appHWND, WM_APP_GAME_CLOSED_PROMPT, 0, 0)
	return result != 0
}

func resetGameProcess() {
	activeApp.gameReady.Store(false)
	activeApp.activeItemID.Store(0)
	setCurrentItemName("")
	activeApp.showOverlay.Store(false)
	activeApp.hasLastOverlayRect = false
	redrawOverlay()
	if activeApp.gameProcessHandle != 0 {
		win32.ProcCloseHandle.Call(activeApp.gameProcessHandle)
		activeApp.gameProcessHandle = 0
	}
	activeApp.gameProcessID = 0
	activeApp.gameAssemblyBase = 0
	activeApp.gameWindowHWND = 0
	activeApp.gameLayoutReadHealth.Reset()
	activeApp.tooltipXAOBResolver.Reset()
	activeApp.tooltipYAOBResolver.Reset()
	activeApp.tooltipHeightAOBResolver.Reset()
	setAppStatus(AppStatusWaitingForGame)
}

func marketableItemExists(itemID int32) bool {
	item, exists := activeApp.allItemMap[int(itemID)]
	return exists && item.Marketable
}

func runOverlayMessageLoop() {
	var msg win32.MSG
	for {
		ret, _, _ := win32.ProcGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if ret == 0 || ret == ^uintptr(0) {
			break
		}
		win32.ProcTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		win32.ProcDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func hideConsoleWindowIfNeeded() {
	hwnd, _, _ := win32.ProcGetConsoleWindow.Call()
	if hwnd == 0 {
		return
	}

	var processList [2]uint32
	count, _, _ := win32.ProcGetConsoleProcessList.Call(uintptr(unsafe.Pointer(&processList[0])), uintptr(len(processList)))
	if count == 1 {
		// Only this process is attached to the console (e.g. launched from Explorer).
		// Hide the console window.
		// SW_HIDE = 0
		win32.ProcShowWindow.Call(hwnd, 0)
	}
}

func checkSingleInstance() (uintptr, bool) {
	name, _ := syscall.UTF16PtrFromString("Local\\TaskBarTradeCenterUniqueMutex")
	// CreateMutexW(nil, FALSE, name)
	mutex, _, err := win32.ProcCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if mutex == 0 {
		return 0, false
	}
	if err != nil && err.(syscall.Errno) == syscall.ERROR_ALREADY_EXISTS {
		win32.ProcCloseHandle.Call(mutex)
		return 0, false
	}
	return mutex, true
}

func activateExistingInstance() bool {
	className, _ := syscall.UTF16PtrFromString(appWindowClassName())
	for i := 0; i < 10; i++ {
		hwnd, _, _ := win32.ProcFindWindowW.Call(uintptr(unsafe.Pointer(className)), 0)
		if hwnd != 0 {
			win32.ProcPostMessageW.Call(hwnd, WM_APP_OPEN_TRAY_MENU, 0, 0)
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
	win32.ProcMessageBoxW.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(caption)), 0x00000000|0x00000030)
}

func preScanTooltipAOB() {
	if !EnablePriceHUD {
		return
	}
	pHandle := activeApp.gameProcessHandle
	base := activeApp.gameAssemblyBase
	if pHandle == 0 || base == 0 {
		return
	}

	activeApp.gameLayoutMu.RLock()
	layout := activeApp.activeGameLayout
	activeApp.gameLayoutMu.RUnlock()

	logPrintln("Pre-scanning tooltip AOB signatures in the background...")

	// Pre-resolve X
	activeApp.tooltipXAOBResolver.Resolve("x", pHandle, base, layout.TooltipXPointerBaseAOB, layout.TooltipXPointerOffsets)
	// Pre-resolve Y
	activeApp.tooltipYAOBResolver.Resolve("y", pHandle, base, layout.TooltipYPointerBaseAOB, layout.TooltipYPointerOffsets)
	// Pre-resolve Height
	activeApp.tooltipHeightAOBResolver.Resolve("height", pHandle, base, layout.TooltipHeightPointerBaseAOB, layout.TooltipHeightPointerOffsets)

	logPrintln("Tooltip AOB background pre-scan completed.")
}
