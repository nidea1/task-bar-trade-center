package market

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func parseSSRItemOrderBook(body []byte, currency MarketCurrency) (MarketOrderBook, bool) {
	text := searchableListingText(body)
	match := ssrOrderBookPattern.FindStringSubmatch(text)
	if len(match) != 5 {
		return MarketOrderBook{}, false
	}

	var highestBuyCents int
	if match[1] != "null" {
		var ok bool
		highestBuyCents, ok = parseInt(match[1])
		if !ok {
			return MarketOrderBook{}, false
		}
	}

	var lowestSellCents int
	if match[2] != "null" {
		var ok bool
		lowestSellCents, ok = parseInt(match[2])
		if !ok {
			return MarketOrderBook{}, false
		}
	}

	if highestBuyCents <= 0 && lowestSellCents <= 0 {
		return MarketOrderBook{}, false
	}
	buyOrders, _ := parseInt(match[3])
	sellOrders, _ := parseInt(match[4])

	var highestBuyQty int
	if highestBuyCents > 0 {
		if buyMatch := ssrBuyOrdersPattern.FindStringSubmatch(text); len(buyMatch) == 3 {
			if priceCents, ok := parseInt(buyMatch[1]); ok && priceCents == highestBuyCents {
				highestBuyQty, _ = parseInt(buyMatch[2])
			}
		}
	}

	var lowestSellQty int
	if lowestSellCents > 0 {
		if sellMatch := ssrSellOrdersPattern.FindStringSubmatch(text); len(sellMatch) == 3 {
			if priceCents, ok := parseInt(sellMatch[1]); ok && priceCents == lowestSellCents {
				lowestSellQty, _ = parseInt(sellMatch[2])
			}
		}
	}

	return MarketOrderBook{
		HighestBuyPrice:    centsToPrice(highestBuyCents),
		LowestSellPrice:    centsToPrice(lowestSellCents),
		HighestBuyQuantity: highestBuyQty,
		LowestSellQuantity: lowestSellQty,
		BuyOrderCount:      buyOrders,
		SellOrderCount:     sellOrders,
		PricePrefix:        currency.PricePrefix,
		PriceSuffix:        currency.PriceSuffix,
	}, true
}

func isSSRListingForScope(body []byte, scope MarketScope) bool {
	text := searchableListingText(body)
	matches := ssrCurrencyPattern.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		if len(match) != 2 {
			continue
		}
		currencyID, ok := parseInt(match[1])
		if ok && currencyID == scope.Currency.SteamCurrencyID {
			return true
		}
	}
	return false
}

func parseSSRPriceHistory(body []byte) []MarketSalePoint {
	text := searchableListingText(body)
	matches := ssrPriceHistoryPattern.FindAllStringSubmatch(text, -1)
	points := make([]MarketSalePoint, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		if len(match) != 4 {
			continue
		}
		unixTime, ok := parseInt64(match[1])
		if !ok {
			continue
		}
		price, ok := parseFloat(match[2])
		if !ok {
			continue
		}
		volume, ok := parseInt(match[3])
		if !ok {
			continue
		}
		key := fmt.Sprintf("%d:%0.6f:%d", unixTime, price, volume)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		points = append(points, MarketSalePoint{Time: unixTime, Price: price, Volume: volume})
	}
	return points
}

func parseLegacySaleHistoryFromListing(body []byte) []MarketSalePoint {
	match := legacySaleHistoryPattern.FindSubmatch(body)
	if len(match) != 2 {
		return nil
	}
	return parseSteamSaleArray(match[1])
}

func parseItemNameID(body []byte) string {
	match := itemNameIDPattern.FindSubmatch(body)
	if len(match) != 2 {
		return ""
	}
	return string(match[1])
}

func parseItemOrdersHistogramResponse(body []byte) (MarketOrderBook, bool) {
	rawObject, ok := unwrapJSONResponseObject(body)
	if !ok {
		return MarketOrderBook{}, false
	}

	highestBuy, hasHighestBuy := parseHistogramPrice(rawObject["highest_buy_order"])
	lowestSell, hasLowestSell := parseHistogramPrice(rawObject["lowest_sell_order"])
	if (!hasHighestBuy || highestBuy <= 0) && (!hasLowestSell || lowestSell <= 0) {
		return MarketOrderBook{}, false
	}

	buyOrders, _ := parseJSONInt(rawObject["buy_order_summary"])
	sellOrders, _ := parseJSONInt(rawObject["sell_order_summary"])
	pricePrefix, _ := parseJSONString(rawObject["price_prefix"])
	priceSuffix, _ := parseJSONString(rawObject["price_suffix"])
	if pricePrefix == "" && priceSuffix == "" {
		pricePrefix = "$"
	}
	if pricePrefix == "$" && strings.TrimSpace(priceSuffix) == "USD" {
		priceSuffix = ""
	}

	highestBuyQty := parseGraphFirstQuantity(rawObject["buy_order_graph"])
	lowestSellQty := parseGraphFirstQuantity(rawObject["sell_order_graph"])

	return MarketOrderBook{
		HighestBuyPrice:    highestBuy,
		LowestSellPrice:    lowestSell,
		HighestBuyQuantity: highestBuyQty,
		LowestSellQuantity: lowestSellQty,
		BuyOrderCount:      buyOrders,
		SellOrderCount:     sellOrders,
		PricePrefix:        pricePrefix,
		PriceSuffix:        priceSuffix,
	}, true
}

func parseSaleHistoryResponse(body []byte) []MarketSalePoint {
	var raw interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil
	}

	if object, ok := raw.(map[string]interface{}); ok {
		if response, exists := object["response"]; exists {
			return parseSaleHistoryValue(response)
		}
		if prices, exists := object["prices"]; exists {
			return parseSaleHistoryValue(prices)
		}
	}

	return parseSaleHistoryValue(raw)
}

func marketDataFromPriceOverview(marketHashName string, body []byte, now time.Time, currency MarketCurrency) (MarketData, bool) {
	var payload struct {
		Success     bool   `json:"success"`
		LowestPrice string `json:"lowest_price"`
		MedianPrice string `json:"median_price"`
		Volume      string `json:"volume"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return MarketData{}, false
	}
	if !payload.Success && payload.LowestPrice == "" && payload.MedianPrice == "" {
		return MarketData{}, false
	}

	analysis := unavailableMarketAnalysis(marketHashName, now, currency)
	priceFormatSet := false
	if lowest, pricePrefix, priceSuffix, ok := parseSteamFormattedPrice(payload.LowestPrice); ok {
		analysis.LowestSellPrice = lowest
		analysis.HasLowestSell = true
		analysis.PricePrefix = pricePrefix
		analysis.PriceSuffix = priceSuffix
		priceFormatSet = true
	}
	if median, pricePrefix, priceSuffix, ok := parseSteamFormattedPrice(payload.MedianPrice); ok {
		analysis.WeeklyAveragePrice = median
		analysis.HasWeeklyAverage = true
		if !priceFormatSet {
			analysis.PricePrefix = pricePrefix
			analysis.PriceSuffix = priceSuffix
		}
	}
	if volume, ok := parseFirstInt(payload.Volume); ok {
		analysis.DailySalesVolume = volume
		analysis.HasDailySales = true
	}
	if suggested, ok := calculateSuggestedPrice(analysis); ok {
		analysis.SuggestedPrice = suggested
		analysis.HasSuggested = true
	}

	return MarketData{
		CachedAt: now,
		Analysis: analysis,
	}, true
}
