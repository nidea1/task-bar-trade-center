package main

const (
	AppName         = "Task Bar Trade Center"
	AppShortName    = "TBTC"
	AppProcessName  = "tbtc"
	AppCreatorName  = "nidea1"
	AppVersion      = "0.6.1"
	GameProcessName = "TaskBarHero.exe"

	EnablePriceHUD           = true
	EnableDynamicTooltipScan = false

	TooltipOverlayOffsetX             = 18
	TooltipOverlayOffsetY             = -170
	TooltipOverlayWidth               = 360
	TooltipOverlayHeight              = 340
	TooltipOverlayHeightCompact       = 105
	TooltipOverlayAnchorOffsetX       = 248
	TooltipOverlayAnchorOffsetY       = 136
	TooltipOverlayReferenceWidth      = 308
	TooltipOverlayReferenceHeight     = 348
	TooltipOverlayReferencePanelWidth = 244
	TooltipOverlayMinWidth            = 244
	TooltipOverlayMaxWidth            = 420
	TooltipScanStep                   = 8

	PROCESS_VM_READ           = 0x0010
	PROCESS_QUERY_INFORMATION = 0x0400
	SYNCHRONIZE               = 0x00100000
	INFINITE                  = 0xFFFFFFFF
	WAIT_OBJECT_0             = 0x00000000

	WS_POPUP          = 0x80000000
	WS_EX_TOPMOST     = 0x00000008
	WS_EX_TRANSPARENT = 0x00000020
	WS_EX_LAYERED     = 0x00080000
	WS_EX_TOOLWINDOW  = 0x00000080
	LWA_COLORKEY      = 0x00000001

	CS_HREDRAW                = 0x0002
	CS_VREDRAW                = 0x0001
	WM_PAINT                  = 0x000F
	WM_CLOSE                  = 0x0010
	WM_DESTROY                = 0x0002
	WM_COMMAND                = 0x0111
	WM_OVERLAY_UPDATE         = 0x8001
	WM_TRAY_ICON              = 0x8002
	WM_TRAY_TIP_UPDATE        = 0x8003
	WM_APP_LOCAL_READY        = 0x8004
	WM_APP_TRAY_NOTIFICATION  = 0x8005
	WM_APP_STATUS_REFRESH     = 0x8006
	WM_APP_GAME_CLOSED_PROMPT = 0x8007
	WM_APP_OPEN_TRAY_MENU     = 0x8008
	SW_HIDE                   = 0
	SW_SHOWDEFAULT            = 10
	SW_SHOWNA                 = 8

	SWP_NOACTIVATE = 0x0010
	SWP_SHOWWINDOW = 0x0040

	TRANSPARENT     = 1
	PS_SOLID        = 0
	DT_LEFT         = 0x00000000
	DT_CENTER       = 0x00000001
	DT_RIGHT        = 0x00000002
	DT_VCENTER      = 0x00000004
	DT_WORDBREAK    = 0x00000010
	DT_SINGLELINE   = 0x00000020
	DT_END_ELLIPSIS = 0x00008000

	FW_NORMAL       = 400
	FW_BOLD         = 700
	DEFAULT_CHARSET = 1
	DEFAULT_QUALITY = 0

	SM_CXSCREEN        = 0
	SM_CYSCREEN        = 1
	SM_CXICON          = 11
	SM_CYICON          = 12
	SM_CXSMICON        = 49
	SM_CYSMICON        = 50
	SM_XVIRTUALSCREEN  = 76
	SM_YVIRTUALSCREEN  = 77
	SM_CXVIRTUALSCREEN = 78
	SM_CYVIRTUALSCREEN = 79

	TH32CS_SNAPPROCESS = 0x00000002

	NIM_ADD        = 0x00000000
	NIM_MODIFY     = 0x00000001
	NIM_DELETE     = 0x00000002
	NIM_SETVERSION = 0x00000004

	NIF_MESSAGE = 0x00000001
	NIF_ICON    = 0x00000002
	NIF_TIP     = 0x00000004
	NIF_INFO    = 0x00000010
	NIF_SHOWTIP = 0x00000080

	NIIF_INFO = 0x00000001

	NOTIFYICON_VERSION_4 = 4
	IDI_APPLICATION      = 32512
	AppIconResourceID    = 1

	WM_CONTEXTMENU = 0x007B
	WM_LBUTTONDOWN = 0x0201
	WM_LBUTTONUP   = 0x0202
	WM_MBUTTONDOWN = 0x0207
	WM_RBUTTONUP   = 0x0205

	MF_STRING    = 0x00000000
	MF_GRAYED    = 0x00000001
	MF_CHECKED   = 0x00000008
	MF_POPUP     = 0x00000010
	MF_SEPARATOR = 0x00000800

	TPM_RIGHTBUTTON = 0x00000002

	TrayIconID               = 1
	MenuRefreshPriceCache    = 1001
	MenuClearPriceCache      = 1002
	MenuExit                 = 1003
	MenuToggleOverlayMode    = 1004
	MenuCheckForUpdates      = 1005
	MenuUpdateConfigs        = 1006
	MenuRestartAdministrator = 1007
	MenuInstallUpdate        = 1008
	MenuOpenRelease          = 1009
	MenuCurrencyBase         = 1100
	MenuRegionBase           = 1200
	MenuLanguageBase         = 1300

	OverlayModeDetail  = 0
	OverlayModeCompact = 1

	WH_MOUSE_LL = 14
)

const (
	AppStatusStarting = iota
	AppStatusWaitingForGame
	AppStatusWaitingForGameAssembly
	AppStatusReady
	AppStatusAttachFailed
	AppStatusGameLayoutIncompatible
	AppStatusInitializationFailed
)

const (
	ConfigStatusUnknown = iota
	ConfigStatusLocalCache
	ConfigStatusEmbedded
	ConfigStatusDevelopment
	ConfigStatusRefreshing
	ConfigStatusCurrent
	ConfigStatusRefreshFailed
)

const (
	UpdateStatusUnknown = iota
	UpdateStatusChecking
	UpdateStatusUpToDate
	UpdateStatusAvailable
	UpdateStatusFailed
	UpdateStatusDownloading
)
