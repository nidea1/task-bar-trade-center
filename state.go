package main

import (
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

var (
	kernel32                     = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess              = kernel32.NewProc("OpenProcess")
	procReadProcessMemory        = kernel32.NewProc("ReadProcessMemory")
	procEnumProcessModules       = kernel32.NewProc("K32EnumProcessModules")
	procGetModuleBaseNameW       = kernel32.NewProc("K32GetModuleBaseNameW")
	procGetModuleHandleW         = kernel32.NewProc("GetModuleHandleW")
	procCreateToolhelp32Snapshot = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW          = kernel32.NewProc("Process32FirstW")
	procProcess32NextW           = kernel32.NewProc("Process32NextW")
	procCloseHandle              = kernel32.NewProc("CloseHandle")
	procWaitForSingleObject      = kernel32.NewProc("WaitForSingleObject")
	procGetConsoleWindow         = kernel32.NewProc("GetConsoleWindow")
	procGetConsoleProcessList    = kernel32.NewProc("GetConsoleProcessList")
	procCreateMutexW             = kernel32.NewProc("CreateMutexW")

	user32                         = syscall.NewLazyDLL("user32.dll")
	procRegisterClassExW           = user32.NewProc("RegisterClassExW")
	procMessageBoxW                = user32.NewProc("MessageBoxW")
	procCreateWindowExW            = user32.NewProc("CreateWindowExW")
	procShowWindow                 = user32.NewProc("ShowWindow")
	procSetWindowPos               = user32.NewProc("SetWindowPos")
	procUpdateWindow               = user32.NewProc("UpdateWindow")
	procSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	procGetMessageW                = user32.NewProc("GetMessageW")
	procTranslateMessage           = user32.NewProc("TranslateMessage")
	procDispatchMessageW           = user32.NewProc("DispatchMessageW")
	procBeginPaint                 = user32.NewProc("BeginPaint")
	procEndPaint                   = user32.NewProc("EndPaint")
	procPostMessageW               = user32.NewProc("PostMessageW")
	procPostQuitMessage            = user32.NewProc("PostQuitMessage")
	procDestroyWindow              = user32.NewProc("DestroyWindow")
	procDefWindowProcW             = user32.NewProc("DefWindowProcW")
	procGetCursorPos               = user32.NewProc("GetCursorPos")
	procGetDC                      = user32.NewProc("GetDC")
	procEnumWindows                = user32.NewProc("EnumWindows")
	procGetWindowThreadProcessId   = user32.NewProc("GetWindowThreadProcessId")
	procIsWindowVisible            = user32.NewProc("IsWindowVisible")
	procGetClientRect              = user32.NewProc("GetClientRect")
	procClientToScreen             = user32.NewProc("ClientToScreen")
	procInvalidateRect             = user32.NewProc("InvalidateRect")
	procReleaseDC                  = user32.NewProc("ReleaseDC")
	procFillRect                   = user32.NewProc("FillRect")
	procDrawTextW                  = user32.NewProc("DrawTextW")
	procGetSystemMetrics           = user32.NewProc("GetSystemMetrics")
	procLoadIconW                  = user32.NewProc("LoadIconW")
	procLoadImageW                 = user32.NewProc("LoadImageW")
	procCreatePopupMenu            = user32.NewProc("CreatePopupMenu")
	procAppendMenuW                = user32.NewProc("AppendMenuW")
	procTrackPopupMenu             = user32.NewProc("TrackPopupMenu")
	procDestroyMenu                = user32.NewProc("DestroyMenu")
	procSetForegroundWindow        = user32.NewProc("SetForegroundWindow")
	procSetWindowsHookExW          = user32.NewProc("SetWindowsHookExW")
	procCallNextHookEx             = user32.NewProc("CallNextHookEx")
	procUnhookWindowsHookEx        = user32.NewProc("UnhookWindowsHookEx")

	shell32             = syscall.NewLazyDLL("shell32.dll")
	procShellNotifyIcon = shell32.NewProc("Shell_NotifyIconW")
	procShellExecuteW   = shell32.NewProc("ShellExecuteW")

	gdi32                = syscall.NewLazyDLL("gdi32.dll")
	procCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
	procCreatePen        = gdi32.NewProc("CreatePen")
	procCreateFontW      = gdi32.NewProc("CreateFontW")
	procSelectObject     = gdi32.NewProc("SelectObject")
	procDeleteObject     = gdi32.NewProc("DeleteObject")
	procSetBkMode        = gdi32.NewProc("SetBkMode")
	procSetTextColor     = gdi32.NewProc("SetTextColor")
	procMoveToEx         = gdi32.NewProc("MoveToEx")
	procLineTo           = gdi32.NewProc("LineTo")
	procGetPixel         = gdi32.NewProc("GetPixel")

	AllItemMap   = make(map[int]ItemConfig)
	ItemMap      = make(map[int]ItemConfig)
	PriceCache   = make(map[string]MarketData)
	PriceCacheMu sync.RWMutex

	GameLayoutMu         sync.RWMutex
	ActiveGameLayout     GameLayout
	GameLayoutSource     string
	GameLayoutReadHealth pointerReadHealth

	CurrentPriceText      = "Loading market..."
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
}

func getCurrentItemName() string {
	CurrentPriceTextMutex.RLock()
	defer CurrentPriceTextMutex.RUnlock()
	return CurrentItemName
}

func setCurrentItemName(val string) {
	CurrentPriceTextMutex.Lock()
	defer CurrentPriceTextMutex.Unlock()
	CurrentItemName = val
}
