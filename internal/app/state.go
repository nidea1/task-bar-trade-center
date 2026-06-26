package app

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/game"
	"github.com/nidea1/task-bar-trade-center/internal/win32"
)

var (
	procOpenProcess                = win32.ProcOpenProcess
	procReadProcessMemory          = win32.ProcReadProcessMemory
	procEnumProcessModules         = win32.ProcEnumProcessModules
	procGetModuleBaseNameW         = win32.ProcGetModuleBaseNameW
	procGetModuleHandleW           = win32.ProcGetModuleHandleW
	procCreateToolhelp32Snapshot   = win32.ProcCreateToolhelp32Snapshot
	procProcess32FirstW            = win32.ProcProcess32FirstW
	procProcess32NextW             = win32.ProcProcess32NextW
	procCloseHandle                = win32.ProcCloseHandle
	procWaitForSingleObject        = win32.ProcWaitForSingleObject
	procGetConsoleWindow           = win32.ProcGetConsoleWindow
	procGetConsoleProcessList      = win32.ProcGetConsoleProcessList
	procCreateMutexW               = win32.ProcCreateMutexW
	procGetUserDefaultLocaleName   = win32.ProcGetUserDefaultLocaleName
	procRegisterClassExW           = win32.ProcRegisterClassExW
	procMessageBoxW                = win32.ProcMessageBoxW
	procCreateWindowExW            = win32.ProcCreateWindowExW
	procShowWindow                 = win32.ProcShowWindow
	procSetWindowPos               = win32.ProcSetWindowPos
	procUpdateWindow               = win32.ProcUpdateWindow
	procSetLayeredWindowAttributes = win32.ProcSetLayeredWindowAttributes
	procGetMessageW                = win32.ProcGetMessageW
	procTranslateMessage           = win32.ProcTranslateMessage
	procDispatchMessageW           = win32.ProcDispatchMessageW
	procBeginPaint                 = win32.ProcBeginPaint
	procEndPaint                   = win32.ProcEndPaint
	procPostMessageW               = win32.ProcPostMessageW
	procSendMessageW               = win32.ProcSendMessageW
	procPostQuitMessage            = win32.ProcPostQuitMessage
	procDestroyWindow              = win32.ProcDestroyWindow
	procDefWindowProcW             = win32.ProcDefWindowProcW
	procGetCursorPos               = win32.ProcGetCursorPos
	procGetDC                      = win32.ProcGetDC
	procEnumWindows                = win32.ProcEnumWindows
	procGetWindowThreadProcessId   = win32.ProcGetWindowThreadProcessId
	procIsWindowVisible            = win32.ProcIsWindowVisible
	procGetClientRect              = win32.ProcGetClientRect
	procClientToScreen             = win32.ProcClientToScreen
	procInvalidateRect             = win32.ProcInvalidateRect
	procReleaseDC                  = win32.ProcReleaseDC
	procFillRect                   = win32.ProcFillRect
	procDrawTextW                  = win32.ProcDrawTextW
	procGetSystemMetrics           = win32.ProcGetSystemMetrics
	procLoadIconW                  = win32.ProcLoadIconW
	procLoadImageW                 = win32.ProcLoadImageW
	procCreatePopupMenu            = win32.ProcCreatePopupMenu
	procAppendMenuW                = win32.ProcAppendMenuW
	procTrackPopupMenu             = win32.ProcTrackPopupMenu
	procDestroyMenu                = win32.ProcDestroyMenu
	procSetForegroundWindow        = win32.ProcSetForegroundWindow
	procFindWindowW                = win32.ProcFindWindowW
	procSetWindowsHookExW          = win32.ProcSetWindowsHookExW
	procCallNextHookEx             = win32.ProcCallNextHookEx
	procUnhookWindowsHookEx        = win32.ProcUnhookWindowsHookEx
	procRegisterHotKey             = win32.ProcRegisterHotKey
	procUnregisterHotKey           = win32.ProcUnregisterHotKey
	procShellNotifyIcon            = win32.ProcShellNotifyIcon
	procShellExecuteW              = win32.ProcShellExecuteW
	procCreateSolidBrush           = win32.ProcCreateSolidBrush
	procCreatePen                  = win32.ProcCreatePen
	procCreateFontW                = win32.ProcCreateFontW
	procSelectObject               = win32.ProcSelectObject
	procDeleteObject               = win32.ProcDeleteObject
	procSetBkMode                  = win32.ProcSetBkMode
	procSetTextColor               = win32.ProcSetTextColor
	procMoveToEx                   = win32.ProcMoveToEx
	procLineTo                     = win32.ProcLineTo
	procGetPixel                   = win32.ProcGetPixel

	AllItemMap   = make(map[int]ItemConfig)
	ItemMap      = make(map[int]ItemConfig)
	PriceCache   = make(map[string]MarketData)
	PriceCacheMu sync.RWMutex

	GameLayoutMu             sync.RWMutex
	ActiveGameLayout         GameLayout
	GameLayoutSource         string
	GameLayoutReadHealth     game.PointerReadHealth
	TooltipXAOBResolver      game.TooltipAOBResolver
	TooltipYAOBResolver      game.TooltipAOBResolver
	TooltipHeightAOBResolver game.TooltipAOBResolver

	CurrentPriceText      = "Loading market..."
	CurrentMarketAnalysis MarketAnalysis
	CurrentOverlayHasData bool
	CurrentItemName       = ""
	ActiveItemID          atomic.Int32
	OverlayHWND           uintptr
	OverlayOriginX        int32
	OverlayOriginY        int32
	OverlayWidth          atomic.Int32
	OverlayHeight         atomic.Int32
	OverlayUpdatePending  atomic.Bool
	OverlayPaintLogged    bool
	ShowOverlay           atomic.Bool
	PriceCacheRefreshing  atomic.Bool
	GameReady             atomic.Bool
	AppStatus             atomic.Int32
	LastOverlayRect       RECT
	HasLastOverlayRect    bool
	LastTooltipDebugLog   time.Time
	GameProcessID         uint32
	GameWindowHWND        uintptr
	GameProcessHandle     uintptr
	GameAssemblyBase      uintptr
	AppHWND               uintptr
	AppIconLarge          uintptr
	AppIconSmall          uintptr
	TrayIconAdded         bool
	MouseHook             uintptr
	MouseHookCallback     uintptr
	OverlayMode           atomic.Int32
	ConfigurationStatus   atomic.Int32
	UpdateStatus          atomic.Int32
	AppInitialized        atomic.Bool
	CurrentPriceTextMutex sync.RWMutex
)

func getCurrentPriceText() string {
	CurrentPriceTextMutex.RLock()
	defer CurrentPriceTextMutex.RUnlock()
	return CurrentPriceText
}

func setCurrentPriceText(val string) {
	CurrentPriceTextMutex.Lock()
	defer CurrentPriceTextMutex.Unlock()
	CurrentPriceText = val
	CurrentOverlayHasData = false
}

func setCurrentMarketAnalysis(analysis MarketAnalysis) {
	CurrentPriceTextMutex.Lock()
	defer CurrentPriceTextMutex.Unlock()
	CurrentMarketAnalysis = analysis
	CurrentOverlayHasData = true
}

func getCurrentMarketAnalysis() (MarketAnalysis, bool) {
	CurrentPriceTextMutex.RLock()
	defer CurrentPriceTextMutex.RUnlock()
	return CurrentMarketAnalysis, CurrentOverlayHasData
}

func getCurrentItemName() string {
	itemID := ActiveItemID.Load()
	if itemID > 0 {
		if config, exists := AllItemMap[int(itemID)]; exists {
			lang := currentDisplayLanguage()
			if name, ok := config.Name[lang]; ok && name != "" {
				return name
			}
			if name, ok := config.Name["en-US"]; ok && name != "" {
				return name
			}
		}
	}
	CurrentPriceTextMutex.RLock()
	defer CurrentPriceTextMutex.RUnlock()
	return CurrentItemName
}

func setCurrentItemName(val string) {
	CurrentPriceTextMutex.Lock()
	defer CurrentPriceTextMutex.Unlock()
	CurrentItemName = val
}
