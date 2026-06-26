package market

import (
	"regexp"
)

var (
	jsonParseCallPattern     = regexp.MustCompile(`JSON\.parse\("((?:\\.|[^"\\])*)"\)`)
	ssrOrderBookPattern      = regexp.MustCompile(`(?s)"amtMaxBuyOrder"\s*:\s*(\d+|null).*?"amtMinSellOrder"\s*:\s*(\d+|null).*?"cBuyOrders"\s*:\s*(\d+).*?"cSellOrders"\s*:\s*(\d+)`)
	ssrBuyOrdersPattern      = regexp.MustCompile(`"rgCompactBuyOrders"\s*:\s*\[\s*(\d+)\s*,\s*(\d+)`)
	ssrSellOrdersPattern     = regexp.MustCompile(`"rgCompactSellOrders"\s*:\s*\[\s*(\d+)\s*,\s*(\d+)`)
	ssrCurrencyPattern       = regexp.MustCompile(`"eCurrency"\s*:\s*(\d+)`)
	ssrPriceHistoryPattern   = regexp.MustCompile(`"time"\s*:\s*(\d+)\s*,\s*"price_median"\s*:\s*([0-9]+(?:\.[0-9]+)?)\s*,\s*"purchases"\s*:\s*(\d+)`)
	itemNameIDPattern        = regexp.MustCompile(`Market_LoadOrderSpread\(\s*(\d+)\s*\)`)
	legacySaleHistoryPattern = regexp.MustCompile(`(?s)var\s+line1\s*=\s*(\[.*?\]);`)
	digitsPattern            = regexp.MustCompile(`\d+`)
	numberPattern            = regexp.MustCompile(`[0-9][0-9\.,]*`)
	pureDigitsPattern        = regexp.MustCompile(`^\d+$`)
)
