package market

import "time"

const AppShortName = "TBTC"

type MarketData struct {
	OverlayText     string `json:"OverlayText,omitempty"`
	CachedAt        time.Time
	Analysis        MarketAnalysis
	OrderBook       MarketOrderBook
	OrderCachedAt   time.Time
	History         []MarketSalePoint
	HistoryCachedAt time.Time
}

type MarketOrderBook struct {
	HighestBuyPrice    float64
	LowestSellPrice    float64
	HighestBuyQuantity int
	LowestSellQuantity int
	BuyOrderCount      int
	SellOrderCount     int
	PricePrefix        string
	PriceSuffix        string
}

type MarketSalePoint struct {
	Time   int64
	Price  float64
	Volume int
}

type MarketAnalysis struct {
	MarketHashName string
	IconURL        string
	PricePrefix    string
	PriceSuffix    string
	UpdatedAt      time.Time

	SuggestedPrice     float64
	LowestSellPrice    float64
	HighestBuyPrice    float64
	LowestSellQuantity int
	HighestBuyQuantity int
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

	HasSuggested             bool
	HasLowestSell            bool
	HasHighestBuy            bool
	HasWeeklyAverage         bool
	HasRecentSaleP75         bool
	HasLastSold              bool
	HasDailySales            bool
	HasOrderBook             bool
	HasSaleHistory           bool
	HasTrend                 bool
	HasSpread                bool
	HasDealTag               bool
	HasConfidence            bool
	HasWeeklyDailyAvgVolume  bool
	IsWideSpread             bool
	USDFallbackMetrics       uint16
	USDDataFallbackAttempted bool
}

type MarketCurrency struct {
	Code            string
	SteamCurrencyID int
	DefaultCountry  string
	PricePrefix     string
	PriceSuffix     string
}

type MarketRegion struct {
	CountryCode  string
	Name         string
	CurrencyCode string
}

type MarketScope struct {
	Currency MarketCurrency
	Region   MarketRegion
}
