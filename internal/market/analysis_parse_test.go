package market

import (
	"fmt"
	"testing"
	"time"
)

func TestParseItemOrdersHistogramResponse(t *testing.T) {
	body := []byte(`{
		"response": {
			"highest_buy_order": 2.55,
			"lowest_sell_order": 2.57,
			"buy_order_summary": 51893,
			"sell_order_summary": 2763,
			"buy_order_graph": [{"price": 2.55, "volume": 5}],
			"sell_order_graph": [{"price": 2.57, "volume": 1}],
			"price_prefix": "$",
			"price_suffix": ""
		}
	}`)

	orderBook, ok := parseItemOrdersHistogramResponse(body)
	if !ok {
		t.Fatal("expected histogram response to parse")
	}
	assertFloatEqual(t, orderBook.HighestBuyPrice, 2.55)
	assertFloatEqual(t, orderBook.LowestSellPrice, 2.57)
	if orderBook.HighestBuyQuantity != 5 {
		t.Fatalf("highest buy quantity = %d, want 5", orderBook.HighestBuyQuantity)
	}
	if orderBook.LowestSellQuantity != 1 {
		t.Fatalf("lowest sell quantity = %d, want 1", orderBook.LowestSellQuantity)
	}
	if orderBook.BuyOrderCount != 51893 {
		t.Fatalf("buy orders = %d, want 51893", orderBook.BuyOrderCount)
	}
	if orderBook.SellOrderCount != 2763 {
		t.Fatalf("sell orders = %d, want 2763", orderBook.SellOrderCount)
	}
}

func TestParseSSRMarketData(t *testing.T) {
	body := []byte(`<script>JSON.parse("{\"state\":{\"data\":{\"amtMaxBuyOrder\":255,\"amtMinSellOrder\":257,\"eCurrency\":1,\"cBuyOrders\":12,\"cSellOrders\":3,\"rgCompactBuyOrders\":[255,10],\"rgCompactSellOrders\":[257,8]},\"queryKey\":[\"market\",\"orderbook\",3678970,\"Example\"]}}")</script>
<script>JSON.parse("{\"state\":{\"data\":{\"history\":[{\"time\":1700000000,\"price_median\":2.51,\"purchases\":4}]},\"queryKey\":[\"market\",\"pricehistory\",3678970,\"Example\"]}}")</script>`)

	usd, ok := marketScopeFor("USD", "TR")
	if !ok {
		t.Fatal("expected USD/TR scope to exist")
	}
	orderBook, ok := parseSSRItemOrderBook(body, usd.Currency)
	if !ok {
		t.Fatal("expected SSR order book to parse")
	}
	assertFloatEqual(t, orderBook.HighestBuyPrice, 2.55)
	assertFloatEqual(t, orderBook.LowestSellPrice, 2.57)
	if orderBook.HighestBuyQuantity != 10 {
		t.Fatalf("highest buy quantity = %d, want 10", orderBook.HighestBuyQuantity)
	}
	if orderBook.LowestSellQuantity != 8 {
		t.Fatalf("lowest sell quantity = %d, want 8", orderBook.LowestSellQuantity)
	}

	history := parseSSRPriceHistory(body)
	if len(history) != 1 {
		t.Fatalf("history length = %d, want 1", len(history))
	}
	assertFloatEqual(t, history[0].Price, 2.51)
	if history[0].Volume != 4 {
		t.Fatalf("history volume = %d, want 4", history[0].Volume)
	}

	if !isSSRListingForScope(body, usd) {
		t.Fatal("expected USD SSR data to apply to the USD/TR scope")
	}
	eur, ok := marketScopeFor("EUR", "DE")
	if !ok || isSSRListingForScope(body, eur) {
		t.Fatal("USD SSR data must not apply to the EUR/DE scope")
	}
}

func TestParseSSRMarketDataWithNullPrices(t *testing.T) {
	body := []byte(`<script>JSON.parse("{\"state\":{\"data\":{\"amtMaxBuyOrder\":230,\"amtMinSellOrder\":null,\"eCurrency\":1,\"cBuyOrders\":2311,\"cSellOrders\":0,\"rgCompactBuyOrders\":[230,20]},\"queryKey\":[\"market\",\"orderbook\",3678970,\"Example\"]}}")</script>`)

	usd, ok := marketScopeFor("USD", "TR")
	if !ok {
		t.Fatal("expected USD/TR scope to exist")
	}
	orderBook, ok := parseSSRItemOrderBook(body, usd.Currency)
	if !ok {
		t.Fatal("expected SSR order book to parse even with null min sell order price")
	}
	assertFloatEqual(t, orderBook.HighestBuyPrice, 2.30)
	assertFloatEqual(t, orderBook.LowestSellPrice, 0.0)
	if orderBook.BuyOrderCount != 2311 {
		t.Fatalf("buy orders = %d, want 2311", orderBook.BuyOrderCount)
	}
	if orderBook.SellOrderCount != 0 {
		t.Fatalf("sell orders = %d, want 0", orderBook.SellOrderCount)
	}
	if orderBook.HighestBuyQuantity != 20 {
		t.Fatalf("highest buy quantity = %d, want 20", orderBook.HighestBuyQuantity)
	}
	if orderBook.LowestSellQuantity != 0 {
		t.Fatalf("lowest sell quantity = %d, want 0", orderBook.LowestSellQuantity)
	}
}

func TestSSRMarketDataUsesSelectedCurrencyFormat(t *testing.T) {
	now := time.Unix(1700000000, 0)
	tests := []struct {
		currency string
		country  string
	}{
		{currency: "PHP", country: "PH"},
		{currency: "EUR", country: "DE"},
		{currency: "CAD", country: "CA"},
	}

	for _, tt := range tests {
		t.Run(tt.currency+"/"+tt.country, func(t *testing.T) {
			scope, ok := marketScopeFor(tt.currency, tt.country)
			if !ok {
				t.Fatalf("expected %s/%s scope to exist", tt.currency, tt.country)
			}
			body := []byte(fmt.Sprintf(`<script>JSON.parse("{\"state\":{\"data\":{\"amtMaxBuyOrder\":392,\"amtMinSellOrder\":445,\"eCurrency\":%d,\"cBuyOrders\":16,\"cSellOrders\":121,\"rgCompactBuyOrders\":[392,3],\"rgCompactSellOrders\":[445,1]},\"queryKey\":[\"market\",\"orderbook\",3678970,\"Fate Helmet (Legendary) A\"]}}")</script>`, scope.Currency.SteamCurrencyID))
			if !isSSRListingForScope(body, scope) {
				t.Fatalf("expected SSR data to apply to the %s/%s scope", tt.currency, tt.country)
			}

			orderBook, ok := parseSSRItemOrderBook(body, scope.Currency)
			if !ok {
				t.Fatal("expected SSR order book to parse")
			}
			data := marketDataFromSources("Fate Helmet (Legendary) A", orderBook, true, nil, now, scope.Currency)

			if data.OrderBook.PricePrefix != scope.Currency.PricePrefix || data.OrderBook.PriceSuffix != scope.Currency.PriceSuffix {
				t.Fatalf("order book format = %q/%q, want %q/%q", data.OrderBook.PricePrefix, data.OrderBook.PriceSuffix, scope.Currency.PricePrefix, scope.Currency.PriceSuffix)
			}
			if data.Analysis.PricePrefix != scope.Currency.PricePrefix || data.Analysis.PriceSuffix != scope.Currency.PriceSuffix {
				t.Fatalf("analysis format = %q/%q, want %q/%q", data.Analysis.PricePrefix, data.Analysis.PriceSuffix, scope.Currency.PricePrefix, scope.Currency.PriceSuffix)
			}
		})
	}
}

func TestSaleHistoryAnalysis(t *testing.T) {
	now := time.Unix(1700000000, 0)
	body := []byte(fmt.Sprintf(`{
		"response": [
			{"time": %d, "price": 30.0, "volume": 4},
			{"time": %d, "price": 20.0, "volume": 3},
			{"time": %d, "price": 10.0, "volume": 2}
		]
	}`, now.Add(-8*24*time.Hour).Unix(), now.Add(-48*time.Hour).Unix(), now.Add(-1*time.Hour).Unix()))

	history := parseSaleHistoryResponse(body)
	if len(history) != 3 {
		t.Fatalf("history length = %d, want 3", len(history))
	}

	analysis := buildMarketAnalysis("Example", MarketOrderBook{}, false, history, now, MarketCurrency{Code: "USD", PricePrefix: "$"})
	if analysis.DailySalesVolume != 2 {
		t.Fatalf("daily sales = %d, want 2", analysis.DailySalesVolume)
	}
	assertFloatEqual(t, analysis.WeeklyAveragePrice, 16.0)
	assertFloatEqual(t, analysis.RecentSaleP75Price, 20.0)
	assertFloatEqual(t, analysis.LastSoldPrice, 10.0)
}

func TestCalculateSuggestedPrice(t *testing.T) {
	tests := []struct {
		name     string
		analysis MarketAnalysis
		want     float64
	}{
		{
			name: "normal tight spread",
			analysis: MarketAnalysis{
				LowestSellPrice:    2.57,
				HighestBuyPrice:    2.55,
				WeeklyAveragePrice: 2.56,
				HasLowestSell:      true,
				HasHighestBuy:      true,
				HasWeeklyAverage:   true,
			},
			want: 2.56,
		},
		{
			name: "wide spread uses historical cap",
			analysis: MarketAnalysis{
				LowestSellPrice:    10.0,
				HighestBuyPrice:    2.0,
				WeeklyAveragePrice: 8.0,
				HasLowestSell:      true,
				HasHighestBuy:      true,
				HasWeeklyAverage:   true,
			},
			want: 8.0,
		},
		{
			name: "recent percentile beats weekly average",
			analysis: MarketAnalysis{
				LowestSellPrice:    6.14,
				HighestBuyPrice:    1.29,
				WeeklyAveragePrice: 2.09,
				RecentSaleP75Price: 2.45,
				HasLowestSell:      true,
				HasHighestBuy:      true,
				HasWeeklyAverage:   true,
				HasRecentSaleP75:   true,
				HasLastSold:        true,
				LastSoldPrice:      0.08,
			},
			want: 2.45,
		},
		{
			name: "history missing uses current book",
			analysis: MarketAnalysis{
				LowestSellPrice: 5.0,
				HighestBuyPrice: 4.0,
				HasLowestSell:   true,
				HasHighestBuy:   true,
			},
			want: 4.99,
		},
		{
			name: "order missing uses weekly average",
			analysis: MarketAnalysis{
				WeeklyAveragePrice: 10.0,
				LastSoldPrice:      9.0,
				HasWeeklyAverage:   true,
				HasLastSold:        true,
			},
			want: 10.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := calculateSuggestedPrice(tt.analysis)
			if !ok {
				t.Fatal("expected suggested price")
			}
			assertFloatEqual(t, got, tt.want)
		})
	}
}

func TestParseIconURL(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "unescaped JSON",
			body: `"icon_url":"abcdef123"`,
			want: "abcdef123",
		},
		{
			name: "escaped JSON inside JS string",
			body: `\"icon_url\":\"abcdef123\"`,
			want: "abcdef123",
		},
		{
			name: "triple escaped JSON inside JS string",
			body: `\\\"icon_url\\\":\\\"abcdef123\\\"`,
			want: "abcdef123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseIconURL([]byte(tt.body))
			if got != tt.want {
				t.Errorf("ParseIconURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

