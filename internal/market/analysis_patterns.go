package market

import (
	"regexp"
)

var (
	jsonParseCallPattern     = regexp.MustCompile(`JSON\.parse\("((?:\\.|[^"\\])*)"\)`)
	ssrLoaderDataPattern     = regexp.MustCompile(`window\.SSR\.loaderData\s*=`)
	ssrOrderBookPattern      = regexp.MustCompile(`(?s)"amtMaxBuyOrder"\s*:\s*(\d+|null).*?"amtMinSellOrder"\s*:\s*(\d+|null).*?"cBuyOrders"\s*:\s*(\d+).*?"cSellOrders"\s*:\s*(\d+)`)
	ssrBuyOrdersPattern      = regexp.MustCompile(`"rgCompactBuyOrders"\s*:\s*\[\s*(\d+)\s*,\s*(\d+)`)
	ssrSellOrdersPattern     = regexp.MustCompile(`"rgCompactSellOrders"\s*:\s*\[\s*(\d+)\s*,\s*(\d+)`)
	ssrCurrencyPattern       = regexp.MustCompile(`(?i)"eCurrency"\s*:\s*(\d+)`)
	ssrPriceHistoryPattern   = regexp.MustCompile(`"time"\s*:\s*(\d+)\s*,\s*"price_median"\s*:\s*([0-9]+(?:\.[0-9]+)?)\s*,\s*"purchases"\s*:\s*(\d+)`)
	searchHTMLResultPattern  = regexp.MustCompile(`(?s)"strHash"\s*:\s*"((?:\\.|[^"\\])*)"\s*,\s*"cSellOrders"\s*:\s*(\d+).*?"eCurrency"\s*:\s*(\d+)\s*,\s*"strMinSellSubtotal"\s*:\s*"((?:\\.|[^"\\])*)"\s*,\s*"asset_description"\s*:\s*\{\s*"appid"\s*:\s*(\d+)`)
	itemNameIDPattern        = regexp.MustCompile(`Market_LoadOrderSpread\(\s*(\d+)\s*\)`)
	legacySaleHistoryPattern = regexp.MustCompile(`(?s)var\s+line1\s*=\s*(\[.*?\]);`)
	iconURLPattern           = regexp.MustCompile(`\\*"icon_url\\*"\s*:\s*\\*"([^\\"]+)\\*"`)
	digitsPattern            = regexp.MustCompile(`\d+`)
	numberPattern            = regexp.MustCompile(`[0-9][0-9\.,]*`)
	pureDigitsPattern        = regexp.MustCompile(`^\d+$`)
)
