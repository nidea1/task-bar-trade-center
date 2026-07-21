package inventory

import "github.com/nidea1/task-bar-trade-center/internal/playerdata"

type ItemDescriptor struct {
	Name           string `json:"name"`
	MarketHashName string `json:"market_hash_name"`
	MarketURL      string `json:"market_url"`
	IconURL        string `json:"icon_url"`
	Grade          string `json:"grade"`
	Type           string `json:"type"`
	Gear           string `json:"gear"`
	Marketable     bool   `json:"marketable"`
}

type PriceQuote struct {
	Suggested          float64 `json:"suggested"`
	Instant            float64 `json:"instant"`
	WeeklyAveragePrice float64 `json:"weekly_average_price"`
	SteamMedianPrice   float64 `json:"steam_median_price"`
	SpreadPercent      float64 `json:"spread_percent"`
	DailySalesVolume   int     `json:"daily_sales_volume"`
	SteamVolume        int     `json:"steam_volume"`
	LowestSellQuantity int     `json:"lowest_sell_quantity"`
	BuyOrderCount      int     `json:"buy_order_count"`
	SellOrderCount     int     `json:"sell_order_count"`
	HasSuggested       bool    `json:"has_suggested"`
	HasInstant         bool    `json:"has_instant"`
	HasWeeklyAverage   bool    `json:"has_weekly_average"`
	HasSteamMedian     bool    `json:"has_steam_median"`
	HasSpread          bool    `json:"has_spread"`
	HasDailySales      bool    `json:"has_daily_sales"`
	HasSteamVolume     bool    `json:"has_steam_volume"`
	HasOrderBook       bool    `json:"has_order_book"`
	Confidence         string  `json:"confidence"`
	HasConfidence      bool    `json:"has_confidence"`
	PriceIsEstimated   bool    `json:"price_is_estimated"`
	PricePrefix        string  `json:"price_prefix"`
	PriceSuffix        string  `json:"price_suffix"`
	UpdatedAt          string  `json:"updated_at"`
}

type QuoteProvider interface {
	Quote(itemID int) (PriceQuote, bool)
}

type DashboardTotals struct {
	SuggestedListingValue float64         `json:"suggested_listing_value"`
	InstantSellValue      float64         `json:"instant_sell_value"`
	InventoryValue        float64         `json:"inventory_value"`
	StashValue            float64         `json:"stash_value"`
	EquippedValue         float64         `json:"equipped_value"`
	HeroEquippedValues    map[int]float64 `json:"hero_equipped_values"`
	StashPageValues       map[int]float64 `json:"stash_page_values"`
	StashPageCounts       map[int]int     `json:"stash_page_counts"`
	StashPageCount        int             `json:"stash_page_count"`
	PricedItemCount       int             `json:"priced_item_count"`
	UnknownItemCount      int             `json:"unknown_item_count"`
	MarketableItemCount   int             `json:"marketable_item_count"`
	TotalItemCount        int             `json:"total_item_count"`
}

type DashboardItem struct {
	ItemID             int      `json:"item_id"`
	Name               string   `json:"name"`
	MarketHashName     string   `json:"market_hash_name"`
	MarketURL          string   `json:"market_url"`
	IconURL            string   `json:"icon_url"`
	Grade              string   `json:"grade"`
	Type               string   `json:"type"`
	Gear               string   `json:"gear"`
	Count              int      `json:"count"`
	Location           string   `json:"location"`
	Equipped           bool     `json:"equipped"`
	Suggested          float64  `json:"suggested"`
	Instant            float64  `json:"instant"`
	TotalSuggested     float64  `json:"total_suggested"`
	TotalInstant       float64  `json:"total_instant"`
	HasPrice           bool     `json:"has_price"`
	WeeklyAveragePrice float64  `json:"weekly_average_price"`
	SteamMedianPrice   float64  `json:"steam_median_price"`
	SpreadPercent      float64  `json:"spread_percent"`
	DailySalesVolume   int      `json:"daily_sales_volume"`
	SteamVolume        int      `json:"steam_volume"`
	LowestSellQuantity int      `json:"lowest_sell_quantity"`
	BuyOrderCount      int      `json:"buy_order_count"`
	SellOrderCount     int      `json:"sell_order_count"`
	HasWeeklyAverage   bool     `json:"has_weekly_average"`
	HasSteamMedian     bool     `json:"has_steam_median"`
	HasSpread          bool     `json:"has_spread"`
	HasDailySales      bool     `json:"has_daily_sales"`
	HasSteamVolume     bool     `json:"has_steam_volume"`
	HasOrderBook       bool     `json:"has_order_book"`
	HasSellThrough     bool     `json:"has_sell_through"`
	SellThroughDays    float64  `json:"sell_through_days"`
	Confidence         string   `json:"confidence"`
	HasConfidence      bool     `json:"has_confidence"`
	PriceIsEstimated   bool     `json:"price_is_estimated"`
	SellScore          float64  `json:"sell_score"`
	SellReasons        []string `json:"sell_reasons"`
	PricePrefix        string   `json:"price_prefix"`
	PriceSuffix        string   `json:"price_suffix"`
	UpdatedAt          string   `json:"updated_at"`
}

type RefreshStatus struct {
	Refreshing                bool   `json:"refreshing"`
	Queued                    int    `json:"queued"`
	Completed                 int    `json:"completed"`
	EstimatedRemainingSeconds int    `json:"estimated_remaining_seconds,omitempty"`
	BackoffUntil              string `json:"backoff_until,omitempty"`
	LastError                 string `json:"last_error,omitempty"`
	LastStartedAt             string `json:"last_started_at,omitempty"`
	LastFinishedAt            string `json:"last_finished_at,omitempty"`
}

type ChestDropSchedule struct {
	NextCommonDropSeconds int  `json:"next_common_drop_seconds"`
	NextStageDropSeconds  int  `json:"next_stage_drop_seconds"`
	CommonDropObserved    bool `json:"common_drop_observed"`
	StageDropObserved     bool `json:"stage_drop_observed"`
	CommonDropReady       bool `json:"common_drop_ready"`
	StageDropReady        bool `json:"stage_drop_ready"`
}

type ChestTimingConfig struct {
	CommonDropSeconds         int `json:"common_drop_seconds"`
	StageDropSeconds          int `json:"stage_drop_seconds"`
	CommonBaseAutoOpenSeconds int `json:"common_base_auto_open_seconds"`
	StageBaseAutoOpenSeconds  int `json:"stage_base_auto_open_seconds"`
}

type InventoryFillEstimate struct {
	Available                  bool    `json:"available"`
	Fills                      bool    `json:"fills"`
	FillModel                  string  `json:"fill_model"`
	SlotCount                  int     `json:"slot_count"`
	UsedSlots                  int     `json:"used_slots"`
	FreeSlots                  int     `json:"free_slots"`
	CommonDropSeconds          int     `json:"common_drop_seconds"`
	StageDropSeconds           int     `json:"stage_drop_seconds"`
	NextCommonDropSeconds      int     `json:"next_common_drop_seconds"`
	NextStageDropSeconds       int     `json:"next_stage_drop_seconds"`
	CommonAutoOpenSeconds      int     `json:"common_auto_open_seconds"`
	StageAutoOpenSeconds       int     `json:"stage_auto_open_seconds"`
	CommonRuneReductionSeconds int     `json:"common_rune_reduction_seconds"`
	StageRuneReductionSeconds  int     `json:"stage_rune_reduction_seconds"`
	NetFillPerHour             float64 `json:"net_fill_per_hour"`
	TimeToFullSeconds          int     `json:"time_to_full_seconds"`
	RuneResolved               bool    `json:"rune_resolved"`
	HasCommonAutoOpenRune      bool    `json:"has_common_auto_open_rune"`
	HasStageAutoOpenRune       bool    `json:"has_stage_auto_open_rune"`
	CommonDropObserved         bool    `json:"common_drop_observed"`
	StageDropObserved          bool    `json:"stage_drop_observed"`
	CommonDropReady            bool    `json:"common_drop_ready"`
	StageDropReady             bool    `json:"stage_drop_ready"`
	CommonChestCount           int     `json:"common_chest_count"`
	StageChestCount            int     `json:"stage_chest_count"`
}

type DashboardState struct {
	UpdatedAt      string                     `json:"updated_at"`
	SnapshotReadAt string                     `json:"snapshot_read_at"`
	MarketScope    string                     `json:"market_scope"`
	CurrencyCode   string                     `json:"currency_code"`
	PricePrefix    string                     `json:"price_prefix"`
	PriceSuffix    string                     `json:"price_suffix"`
	Gold           uint64                     `json:"gold"`
	Totals         DashboardTotals            `json:"totals"`
	Items          []DashboardItem            `json:"items"`
	MostValuable   []DashboardItem            `json:"most_valuable"`
	BestToSellNow  []DashboardItem            `json:"best_to_sell_now"`
	Duplicates     []DashboardItem            `json:"duplicates"`
	Equipped       []DashboardItem            `json:"equipped"`
	MissingPrices  []DashboardItem            `json:"missing_prices"`
	Refresh        RefreshStatus              `json:"refresh"`
	Translations   map[string]string          `json:"translations"`
	TradeSlots     []playerdata.TradeShipSlot `json:"trade_slots"`
	InventoryFill  InventoryFillEstimate      `json:"inventory_fill"`
	ChestCounts    playerdata.ChestCounts     `json:"chest_counts"`
}
