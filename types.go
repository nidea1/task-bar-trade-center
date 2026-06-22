package main

import (
	"time"
)

type RECT struct {
	Left, Top, Right, Bottom int32
}
type POINT struct {
	X, Y int32
}
type PAINTSTRUCT struct {
	Hdc         uintptr
	FErase      int32
	RcPaint     RECT
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}
type WNDCLASSEX struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}
type MSG struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
}
type MSLLHOOKSTRUCT struct {
	Pt          POINT
	MouseData   uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}
type PROCESSENTRY32W struct {
	DwSize              uint32
	CntUsage            uint32
	Th32ProcessID       uint32
	Th32DefaultHeapID   uintptr
	Th32ModuleID        uint32
	CntThreads          uint32
	Th32ParentProcessID uint32
	PcPriClassBase      int32
	DwFlags             uint32
	SzExeFile           [260]uint16
}
type GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}
type NOTIFYICONDATAW struct {
	CbSize           uint32
	HWnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UVersion         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         GUID
	HBalloonIcon     uintptr
}

type MarketData struct {
	// OverlayText is read only for legacy cache compatibility. New cache entries
	// render directly from Analysis so changing the UI language is immediate.
	OverlayText     string `json:"OverlayText,omitempty"`
	CachedAt        time.Time
	Analysis        MarketAnalysis
	OrderBook       MarketOrderBook
	OrderCachedAt   time.Time
	History         []MarketSalePoint
	HistoryCachedAt time.Time
}

type MarketOrderBook struct {
	HighestBuyPrice float64
	LowestSellPrice float64
	BuyOrderCount   int
	SellOrderCount  int
	PricePrefix     string
	PriceSuffix     string
}

type MarketSalePoint struct {
	Time   int64
	Price  float64
	Volume int
}

type MarketAnalysis struct {
	MarketHashName string
	PricePrefix    string
	PriceSuffix    string
	UpdatedAt      time.Time

	SuggestedPrice     float64
	LowestSellPrice    float64
	HighestBuyPrice    float64
	WeeklyAveragePrice float64
	RecentSaleP75Price float64
	LastSoldPrice      float64
	DailySalesVolume   int
	BuyOrderCount      int
	SellOrderCount     int

	DailyAveragePrice    float64
	TrendPercent         float64
	SpreadPercent        float64
	WeeklyDailyAvgVolume float64
	Confidence           string
	DealTag              string
	VolumeActivity       string

	HasSuggested            bool
	HasLowestSell           bool
	HasHighestBuy           bool
	HasWeeklyAverage        bool
	HasRecentSaleP75        bool
	HasLastSold             bool
	HasDailySales           bool
	HasOrderBook            bool
	HasSaleHistory          bool
	HasTrend                bool
	HasSpread               bool
	HasDealTag              bool
	HasConfidence           bool
	HasWeeklyDailyAvgVolume bool
	IsWideSpread            bool
}

type ItemConfig struct {
	ID         int               `json:"id"`
	Name       map[string]string `json:"name"`
	Grade      string            `json:"grade"`
	Type       string            `json:"type"`
	Marketable bool              `json:"marketable"`
}

type PriceOverlayView struct {
	Suggested  string
	LowestSell string
	HighestBuy string
	DailySales string
	WeeklyAvg  string
	LastSold   string
	Updated    string
	Trend      string
	Spread     string
	DealTag    string
	Confidence string
	Orders     string
	SaleP75    string
}

type WindowSearchState struct {
	PID  uint32
	HWND uintptr
}

type OverlayPlacementCalibration struct {
	TooltipY      int32 `json:"tooltip_y"`
	TooltipHeight int32 `json:"tooltip_height"`
	PanelWidth    int32 `json:"panel_width"`
	OffsetX       int32 `json:"offset_x"`
	OffsetY       int32 `json:"offset_y"`
}

type OverlayXCalibration struct {
	X      float32 `json:"x"`
	Offset int32   `json:"offset"`
}
