package app

import (
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/catalog"
	"github.com/nidea1/task-bar-trade-center/internal/game"
	"github.com/nidea1/task-bar-trade-center/internal/inventory"
	"github.com/nidea1/task-bar-trade-center/internal/market"
	"github.com/nidea1/task-bar-trade-center/internal/playerdata"
	"github.com/nidea1/task-bar-trade-center/internal/win32"
)

type inventorySnapshotCall struct {
	done          chan struct{}
	processID     uint32
	processHandle uintptr
	snapshot      playerdata.InventorySnapshot
	err           error
}

type App struct {
	// Callbacks
	callbacks Callbacks

	// Storage paths
	appDataDir                     string
	logFilePath                    string
	priceCacheFilePath             string
	iconMetadataFilePath           string
	inventoryStateFilePath         string
	inventoryResolverCacheFilePath string
	settingsFilePath               string
	gameLayoutCacheFilePath        string
	appLogFile                     *os.File

	// Windows HWNDs and hooks
	appHWND              uintptr
	appIconLarge         uintptr
	appIconSmall         uintptr
	trayIconAdded        bool
	mouseHook            uintptr
	mouseHookCallback    uintptr
	overlayHWND          uintptr
	overlayOriginX       int32
	overlayOriginY       int32
	overlayWidth         atomic.Int32
	overlayHeight        atomic.Int32
	showOverlay          atomic.Bool
	overlayMode          atomic.Int32
	lastOverlayRect      win32.RECT
	lastOverlayRectAt    time.Time
	hasLastOverlayRect   bool
	overlayWindowRect    win32.RECT
	hasOverlayWindowRect bool
	dashboardSettingsMu  sync.RWMutex
	dashboardSettings    DashboardSettings
	settingsReadyOnce    sync.Once
	settingsReadyCh      chan struct{}

	appStatus            atomic.Int32
	configurationStatus  atomic.Int32
	updateStatus         atomic.Int32
	runtimeStateMu       sync.RWMutex
	runtimeState         RuntimeStateInfo
	appInitialized       atomic.Bool
	shutdownRequested    atomic.Bool
	minRarityNotifyLevel atomic.Int32

	// Game process info
	gameProcessID           uint32
	gameWindowHWND          uintptr
	gameProcessHandle       uintptr
	gameAssemblyBase        uintptr
	gameAssemblyFingerprint string
	gameReady               atomic.Bool

	// Caches and databases
	allItemMap   map[int]catalog.ItemConfig
	itemMap      map[int]catalog.ItemConfig
	priceCache   map[string]market.MarketData
	priceCacheMu sync.RWMutex

	iconMetadata   map[string]iconMetadataEntry
	iconMetadataMu sync.RWMutex

	priceCacheWriteMu        sync.Mutex
	priceCacheWriteTimer     *time.Timer
	priceCacheWritePending   bool
	iconMetadataWriteMu      sync.Mutex
	iconMetadataWriteTimer   *time.Timer
	iconMetadataWritePending bool

	// Game layout resolution
	gameLayoutMu             sync.RWMutex
	activeGameLayout         game.GameLayout
	gameLayoutSource         string
	gameLayoutReadHealth     game.PointerReadHealth
	tooltipXAOBResolver      game.TooltipAOBResolver
	tooltipYAOBResolver      game.TooltipAOBResolver
	tooltipHeightAOBResolver game.TooltipAOBResolver

	// Overlay window and draw state
	overlayUpdatePending         atomic.Bool
	overlayPaintLogged           bool
	overlayRedrawMu              sync.Mutex
	lastOverlayRedrawAt          time.Time
	overlayWindowVisible         bool
	overlayContentVersion        atomic.Uint64
	overlayPaintedContentVersion uint64
	lastTooltipDebugLog          time.Time
	tooltipFetchVersion          atomic.Uint64
	currentPriceText             string
	currentMarketAnalysis        market.MarketAnalysis
	currentOverlayHasData        bool
	currentItemName              string
	activeItemID                 atomic.Int32
	currentPriceTextMutex        sync.RWMutex
	marketFetchMu                sync.Mutex
	marketFetchInFlight          map[string]*marketFetchCall

	// Inventory integration
	inventoryMu                    sync.Mutex
	inventoryDashboardBuildMu      sync.Mutex
	inventoryDashboardRebuildMu    sync.Mutex
	inventoryDashboardRebuildTimer *time.Timer
	pendingInventoryRebuildReason  string
	inventorySnapshotMu            sync.Mutex
	inventorySnapshotInFlight      *inventorySnapshotCall
	inventorySnapshotBackoffUntil  time.Time
	inventorySnapshotLastErr       string
	inventoryResolver              *playerdata.Resolver
	lastSnapshot                   *playerdata.InventorySnapshot
	inventoryDashboardState        inventory.DashboardState
	inventoryPriceQueue            *inventory.RefreshQueue
	marketableInventorySeen        map[uint64]struct{}
	marketableInventorySeeded      bool
	notificationIconCache          map[string]uintptr
	notificationIconPreparing      map[string]struct{}
	priceCacheRefreshing           atomic.Bool
}

var activeApp = &App{
	allItemMap:                make(map[int]catalog.ItemConfig),
	itemMap:                   make(map[int]catalog.ItemConfig),
	priceCache:                make(map[string]market.MarketData),
	iconMetadata:              make(map[string]iconMetadataEntry),
	notificationIconCache:     make(map[string]uintptr),
	notificationIconPreparing: make(map[string]struct{}),
	dashboardSettings:         defaultDashboardSettings(),
	settingsReadyCh:           make(chan struct{}),
}

func getCurrentPriceText() string {
	activeApp.currentPriceTextMutex.RLock()
	defer activeApp.currentPriceTextMutex.RUnlock()
	return activeApp.currentPriceText
}

func setCurrentPriceText(val string) {
	activeApp.currentPriceTextMutex.Lock()
	defer activeApp.currentPriceTextMutex.Unlock()
	activeApp.currentPriceText = val
	activeApp.currentOverlayHasData = false
	activeApp.overlayContentVersion.Add(1)
}

func setCurrentMarketAnalysis(analysis market.MarketAnalysis) {
	activeApp.currentPriceTextMutex.Lock()
	defer activeApp.currentPriceTextMutex.Unlock()
	activeApp.currentMarketAnalysis = analysis
	activeApp.currentOverlayHasData = true
	activeApp.overlayContentVersion.Add(1)
}

func getCurrentMarketAnalysis() (market.MarketAnalysis, bool) {
	activeApp.currentPriceTextMutex.RLock()
	defer activeApp.currentPriceTextMutex.RUnlock()
	return activeApp.currentMarketAnalysis, activeApp.currentOverlayHasData
}

func getCurrentItemName() string {
	itemID := activeApp.activeItemID.Load()
	if itemID > 0 {
		if config, exists := activeApp.allItemMap[int(itemID)]; exists {
			lang := currentDisplayLanguage()
			if name, ok := config.Name[lang]; ok && name != "" {
				return name
			}
			if name, ok := config.Name["en-US"]; ok && name != "" {
				return name
			}
		}
	}
	activeApp.currentPriceTextMutex.RLock()
	defer activeApp.currentPriceTextMutex.RUnlock()
	return activeApp.currentItemName
}

func setCurrentItemName(val string) {
	activeApp.currentPriceTextMutex.Lock()
	defer activeApp.currentPriceTextMutex.Unlock()
	activeApp.currentItemName = val
	activeApp.overlayContentVersion.Add(1)
}
