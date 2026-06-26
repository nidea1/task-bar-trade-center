package inventory

import "time"

type ItemDescriptor struct {
	Name           string `json:"name"`
	MarketHashName string `json:"market_hash_name"`
	MarketURL      string `json:"market_url"`
	Marketable     bool   `json:"marketable"`
}

type PriceQuote struct {
	Suggested    float64   `json:"suggested"`
	Instant      float64   `json:"instant"`
	HasSuggested bool      `json:"has_suggested"`
	HasInstant   bool      `json:"has_instant"`
	PricePrefix  string    `json:"price_prefix"`
	PriceSuffix  string    `json:"price_suffix"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type QuoteProvider interface {
	Quote(itemID int) (PriceQuote, bool)
}

type DashboardTotals struct {
	SuggestedListingValue float64 `json:"suggested_listing_value"`
	InstantSellValue      float64 `json:"instant_sell_value"`
	InventoryValue        float64 `json:"inventory_value"`
	StashValue            float64 `json:"stash_value"`
	EquippedValue         float64 `json:"equipped_value"`
	PricedItemCount       int     `json:"priced_item_count"`
	UnknownItemCount      int     `json:"unknown_item_count"`
	MarketableItemCount   int     `json:"marketable_item_count"`
	TotalItemCount        int     `json:"total_item_count"`
}

type DashboardItem struct {
	ItemID         int       `json:"item_id"`
	Name           string    `json:"name"`
	MarketHashName string    `json:"market_hash_name"`
	MarketURL      string    `json:"market_url"`
	Count          int       `json:"count"`
	Location       string    `json:"location"`
	Equipped       bool      `json:"equipped"`
	Suggested      float64   `json:"suggested"`
	Instant        float64   `json:"instant"`
	TotalSuggested float64   `json:"total_suggested"`
	TotalInstant   float64   `json:"total_instant"`
	HasPrice       bool      `json:"has_price"`
	PricePrefix    string    `json:"price_prefix"`
	PriceSuffix    string    `json:"price_suffix"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type RefreshStatus struct {
	Refreshing     bool      `json:"refreshing"`
	Queued         int       `json:"queued"`
	Completed      int       `json:"completed"`
	BackoffUntil   time.Time `json:"backoff_until,omitempty"`
	LastError      string    `json:"last_error,omitempty"`
	LastStartedAt  time.Time `json:"last_started_at,omitempty"`
	LastFinishedAt time.Time `json:"last_finished_at,omitempty"`
}

type DashboardState struct {
	UpdatedAt      time.Time       `json:"updated_at"`
	SnapshotReadAt time.Time       `json:"snapshot_read_at"`
	MarketScope    string          `json:"market_scope"`
	CurrencyCode   string          `json:"currency_code"`
	PricePrefix    string          `json:"price_prefix"`
	PriceSuffix    string          `json:"price_suffix"`
	Gold           uint64          `json:"gold"`
	Totals         DashboardTotals `json:"totals"`
	Items          []DashboardItem `json:"items"`
	MostValuable   []DashboardItem `json:"most_valuable"`
	Duplicates     []DashboardItem `json:"duplicates"`
	Equipped       []DashboardItem `json:"equipped"`
	MissingPrices  []DashboardItem `json:"missing_prices"`
	Refresh        RefreshStatus   `json:"refresh"`
}
