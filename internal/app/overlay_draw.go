package app

import (
	"github.com/nidea1/task-bar-trade-center/internal/market"
	"github.com/nidea1/task-bar-trade-center/internal/win32"

	"fmt"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/nidea1/task-bar-trade-center/internal/overlay"
)

func drawGameMarketOverlay(hdc uintptr, rect win32.RECT) {
	if activeApp.overlayMode.Load() == OverlayModeCompact {
		drawCompactOverlay(hdc, rect)
		return
	}
	drawDetailOverlay(hdc, rect)
}

func calculateRequiredHeight(data overlay.PriceView, mode int32) int32 {
	return overlay.CalculateRequiredHeight(data, mode, OverlayModeCompact)
}

func drawDetailOverlay(hdc uintptr, rect win32.RECT) {
	win32.ProcSetBkMode.Call(hdc, TRANSPARENT)

	fillSolidRect(hdc, rect, colorRGB(3, 3, 4))
	strokeRect(hdc, rect, colorRGB(0, 0, 0), 2)

	frame := insetRect(rect, 3, 3)
	fillSolidRect(hdc, frame, colorRGB(24, 20, 19))
	fillSolidRect(hdc, win32.RECT{Left: frame.Left + 2, Top: frame.Top + 2, Right: frame.Right - 2, Bottom: frame.Top + 4}, colorRGB(126, 83, 38))
	fillSolidRect(hdc, win32.RECT{Left: frame.Left + 2, Top: frame.Bottom - 4, Right: frame.Right - 2, Bottom: frame.Bottom - 2}, colorRGB(48, 32, 25))

	inner := insetRect(frame, 4, 4)
	fillSolidRect(hdc, inner, colorRGB(10, 11, 12))
	strokeRect(hdc, inner, colorRGB(70, 61, 48), 1)

	header := win32.RECT{Left: inner.Left + 5, Top: inner.Top + 5, Right: inner.Right - 5, Bottom: inner.Top + 27}
	fillSolidRect(hdc, header, colorRGB(96, 27, 24))
	fillSolidRect(hdc, win32.RECT{Left: header.Left, Top: header.Top, Right: header.Right, Bottom: header.Top + 2}, colorRGB(174, 67, 42))
	fillSolidRect(hdc, win32.RECT{Left: header.Left, Top: header.Bottom - 2, Right: header.Right, Bottom: header.Bottom}, colorRGB(41, 13, 11))
	strokeRect(hdc, header, colorRGB(227, 169, 67), 1)

	withOverlayFont(hdc, 16, FW_BOLD, func() {
		drawOverlayText(hdc, tr("hud.title"), header, colorRGB(255, 190, 45), DT_CENTER|DT_SINGLELINE|DT_VCENTER)
	})

	itemName := getCurrentItemName()
	if itemName == "" {
		itemName = tr("hud.market_price")
	}
	itemRect := win32.RECT{Left: inner.Left + 8, Top: header.Bottom + 4, Right: inner.Right - 8, Bottom: header.Bottom + 26}
	fillSolidRect(hdc, itemRect, colorRGB(25, 21, 18))
	strokeRect(hdc, itemRect, colorRGB(130, 99, 61), 1)
	withOverlayFont(hdc, 13, FW_BOLD, func() {
		drawOverlayText(hdc, itemName, insetRect(itemRect, 8, 0), colorRGB(235, 205, 156), DT_CENTER|DT_SINGLELINE|DT_VCENTER|DT_END_ELLIPSIS)
	})

	data := currentPriceOverlayView()
	bodyLeft := inner.Left + 10
	bodyRight := inner.Right - 10
	y := itemRect.Bottom + 6

	suggestedRect := win32.RECT{Left: bodyLeft, Top: y, Right: bodyRight, Bottom: y + 28}
	fillSolidRect(hdc, suggestedRect, colorRGB(42, 25, 14))
	fillSolidRect(hdc, win32.RECT{Left: suggestedRect.Left, Top: suggestedRect.Top, Right: suggestedRect.Right, Bottom: suggestedRect.Top + 2}, colorRGB(131, 75, 30))
	fillSolidRect(hdc, win32.RECT{Left: suggestedRect.Left, Top: suggestedRect.Bottom - 2, Right: suggestedRect.Right, Bottom: suggestedRect.Bottom}, colorRGB(21, 13, 9))
	strokeRect(hdc, suggestedRect, colorRGB(159, 101, 42), 1)
	withOverlayFont(hdc, 12, FW_BOLD, func() {
		labelRect := win32.RECT{Left: suggestedRect.Left + 8, Top: suggestedRect.Top, Right: suggestedRect.Left + 128, Bottom: suggestedRect.Bottom}
		valueRect := win32.RECT{Left: labelRect.Right, Top: suggestedRect.Top, Right: suggestedRect.Right - 8, Bottom: suggestedRect.Bottom}
		suggestedLabel := tr("hud.suggested")
		if data.Confidence != "" {
			suggestedLabel += " [" + localizedSemanticValue(data.Confidence) + "]"
		}
		drawOverlayText(hdc, suggestedLabel, labelRect, colorRGB(202, 179, 139), DT_LEFT|DT_SINGLELINE|DT_VCENTER)
		drawOverlayText(hdc, data.Suggested, valueRect, colorRGB(84, 220, 94), DT_RIGHT|DT_SINGLELINE|DT_VCENTER|DT_END_ELLIPSIS)
	})

	y = suggestedRect.Bottom + 3
	if data.DealTag != "" {
		dealTagRect := win32.RECT{Left: bodyLeft, Top: y, Right: bodyRight, Bottom: y + 16}
		dealColor := colorRGB(84, 220, 94)
		dealBgColor := colorRGB(20, 42, 20)
		if strings.EqualFold(data.DealTag, "overvalued") {
			dealColor = colorRGB(240, 80, 70)
			dealBgColor = colorRGB(42, 16, 14)
		}
		fillSolidRect(hdc, dealTagRect, dealBgColor)
		withOverlayFont(hdc, 11, FW_BOLD, func() {
			drawOverlayText(hdc, localizedSemanticValue(data.DealTag), dealTagRect, dealColor, DT_CENTER|DT_SINGLELINE|DT_VCENTER)
		})
		y += 18
	}

	y += 2
	withOverlayFont(hdc, 12, FW_NORMAL, func() {
		if data.LastSold != "" {
			drawMarketStat(hdc, tr("hud.last_sold"), data.LastSold, win32.RECT{Left: bodyLeft, Top: y, Right: bodyRight, Bottom: y + 16})
			y += 17
		}
		if data.LowestSell != "" {
			drawMarketStat(hdc, tr("hud.lowest_sell"), data.LowestSell, win32.RECT{Left: bodyLeft, Top: y, Right: bodyRight, Bottom: y + 16})
			y += 17
		}
		if data.HighestBuy != "" {
			drawMarketStat(hdc, tr("hud.highest_buy"), data.HighestBuy, win32.RECT{Left: bodyLeft, Top: y, Right: bodyRight, Bottom: y + 16})
			y += 17
		}
		if data.WeeklyAvg != "" {
			drawMarketStat(hdc, tr("hud.weekly_average"), data.WeeklyAvg, win32.RECT{Left: bodyLeft, Top: y, Right: bodyRight, Bottom: y + 16})
			y += 17
		}
		if data.SaleP75 != "" {
			drawMarketStat(hdc, tr("hud.sale_p75"), data.SaleP75, win32.RECT{Left: bodyLeft, Top: y, Right: bodyRight, Bottom: y + 16})
			y += 17
		}
		if data.Spread != "" {
			drawMarketStat(hdc, tr("hud.spread"), data.Spread, win32.RECT{Left: bodyLeft, Top: y, Right: bodyRight, Bottom: y + 16})
			y += 17
		}
		if data.Trend != "" {
			drawMarketStat(hdc, tr("hud.trend"), data.Trend, win32.RECT{Left: bodyLeft, Top: y, Right: bodyRight, Bottom: y + 16})
			y += 17
		}
		if data.DailySales != "" {
			drawMarketStat(hdc, tr("hud.daily_sales"), data.DailySales, win32.RECT{Left: bodyLeft, Top: y, Right: bodyRight, Bottom: y + 16})
			y += 17
		}
		if data.Orders != "" {
			drawMarketStat(hdc, tr("hud.orders"), data.Orders, win32.RECT{Left: bodyLeft, Top: y, Right: bodyRight, Bottom: y + 16})
			y += 17
		}
	})

	if data.FallbackNotice != "" {
		fallbackRect := win32.RECT{Left: bodyLeft, Top: y + 2, Right: bodyRight, Bottom: y + 12}
		withOverlayFont(hdc, 10, FW_NORMAL, func() {
			drawOverlayText(hdc, data.FallbackNotice, fallbackRect, colorRGB(178, 150, 92), DT_CENTER|DT_SINGLELINE|DT_VCENTER|DT_END_ELLIPSIS)
		})
		y = fallbackRect.Bottom
	}

	updatedText := data.Updated
	if parsed, err := time.Parse(time.RFC3339, data.Updated); err == nil {
		updatedText = market.FormatRelativeTime(parsed, time.Now())
	}
	if updatedText != "" {
		updatedRect := win32.RECT{Left: bodyLeft, Top: y + 2, Right: bodyRight, Bottom: y + 12}
		withOverlayFont(hdc, 10, FW_NORMAL, func() {
			drawOverlayText(hdc, updatedText, updatedRect, colorRGB(142, 130, 111), DT_RIGHT|DT_SINGLELINE|DT_VCENTER|DT_END_ELLIPSIS)
		})
		y = updatedRect.Bottom
	}

	footerRect := win32.RECT{Left: bodyLeft, Top: y + 5, Right: bodyRight, Bottom: y + 20}
	fillSolidRect(hdc, footerRect, colorRGB(18, 17, 16))
	strokeRect(hdc, footerRect, colorRGB(82, 69, 51), 1)
	withOverlayFont(hdc, 10, FW_BOLD, func() {
		drawOverlayText(hdc, tr("hud.open_market"), insetRect(footerRect, 9, 1), colorRGB(202, 179, 139), DT_CENTER|DT_SINGLELINE|DT_VCENTER|DT_END_ELLIPSIS)
	})
}

func drawCompactOverlay(hdc uintptr, rect win32.RECT) {
	win32.ProcSetBkMode.Call(hdc, TRANSPARENT)

	fillSolidRect(hdc, rect, colorRGB(3, 3, 4))
	strokeRect(hdc, rect, colorRGB(0, 0, 0), 2)

	frame := insetRect(rect, 3, 3)
	fillSolidRect(hdc, frame, colorRGB(24, 20, 19))
	fillSolidRect(hdc, win32.RECT{Left: frame.Left + 2, Top: frame.Top + 2, Right: frame.Right - 2, Bottom: frame.Top + 4}, colorRGB(126, 83, 38))
	fillSolidRect(hdc, win32.RECT{Left: frame.Left + 2, Top: frame.Bottom - 4, Right: frame.Right - 2, Bottom: frame.Bottom - 2}, colorRGB(48, 32, 25))

	inner := insetRect(frame, 4, 4)
	fillSolidRect(hdc, inner, colorRGB(10, 11, 12))
	strokeRect(hdc, inner, colorRGB(70, 61, 48), 1)

	header := win32.RECT{Left: inner.Left + 5, Top: inner.Top + 5, Right: inner.Right - 5, Bottom: inner.Top + 27}
	fillSolidRect(hdc, header, colorRGB(96, 27, 24))
	fillSolidRect(hdc, win32.RECT{Left: header.Left, Top: header.Top, Right: header.Right, Bottom: header.Top + 2}, colorRGB(174, 67, 42))
	fillSolidRect(hdc, win32.RECT{Left: header.Left, Top: header.Bottom - 2, Right: header.Right, Bottom: header.Bottom}, colorRGB(41, 13, 11))
	strokeRect(hdc, header, colorRGB(227, 169, 67), 1)

	withOverlayFont(hdc, 16, FW_BOLD, func() {
		drawOverlayText(hdc, tr("hud.title"), header, colorRGB(255, 190, 45), DT_CENTER|DT_SINGLELINE|DT_VCENTER)
	})

	itemName := getCurrentItemName()
	if itemName == "" {
		itemName = tr("hud.market_price")
	}
	itemRect := win32.RECT{Left: inner.Left + 8, Top: header.Bottom + 4, Right: inner.Right - 8, Bottom: header.Bottom + 26}
	fillSolidRect(hdc, itemRect, colorRGB(25, 21, 18))
	strokeRect(hdc, itemRect, colorRGB(130, 99, 61), 1)
	withOverlayFont(hdc, 13, FW_BOLD, func() {
		drawOverlayText(hdc, itemName, insetRect(itemRect, 8, 0), colorRGB(235, 205, 156), DT_CENTER|DT_SINGLELINE|DT_VCENTER|DT_END_ELLIPSIS)
	})

	data := currentPriceOverlayView()
	bodyLeft := inner.Left + 10
	bodyRight := inner.Right - 10
	y := itemRect.Bottom + 6

	suggestedRect := win32.RECT{Left: bodyLeft, Top: y, Right: bodyRight, Bottom: y + 28}
	fillSolidRect(hdc, suggestedRect, colorRGB(42, 25, 14))
	fillSolidRect(hdc, win32.RECT{Left: suggestedRect.Left, Top: suggestedRect.Top, Right: suggestedRect.Right, Bottom: suggestedRect.Top + 2}, colorRGB(131, 75, 30))
	fillSolidRect(hdc, win32.RECT{Left: suggestedRect.Left, Top: suggestedRect.Bottom - 2, Right: suggestedRect.Right, Bottom: suggestedRect.Bottom}, colorRGB(21, 13, 9))
	strokeRect(hdc, suggestedRect, colorRGB(159, 101, 42), 1)
	withOverlayFont(hdc, 12, FW_BOLD, func() {
		labelRect := win32.RECT{Left: suggestedRect.Left + 8, Top: suggestedRect.Top, Right: suggestedRect.Left + 128, Bottom: suggestedRect.Bottom}
		valueRect := win32.RECT{Left: labelRect.Right, Top: suggestedRect.Top, Right: suggestedRect.Right - 8, Bottom: suggestedRect.Bottom}
		drawOverlayText(hdc, tr("hud.suggested"), labelRect, colorRGB(202, 179, 139), DT_LEFT|DT_SINGLELINE|DT_VCENTER)
		drawOverlayText(hdc, data.Suggested, valueRect, colorRGB(84, 220, 94), DT_RIGHT|DT_SINGLELINE|DT_VCENTER|DT_END_ELLIPSIS)
	})

	y = suggestedRect.Bottom + 5
	withOverlayFont(hdc, 12, FW_NORMAL, func() {
		if data.LastSold != "" {
			drawMarketStat(hdc, tr("hud.last_sold"), data.LastSold, win32.RECT{Left: bodyLeft, Top: y, Right: bodyRight, Bottom: y + 16})
			y += 17
		}
		if data.LowestSell != "" {
			drawMarketStat(hdc, tr("hud.lowest_sell"), data.LowestSell, win32.RECT{Left: bodyLeft, Top: y, Right: bodyRight, Bottom: y + 16})
			y += 17
		}
		if data.HighestBuy != "" {
			drawMarketStat(hdc, tr("hud.highest_buy"), data.HighestBuy, win32.RECT{Left: bodyLeft, Top: y, Right: bodyRight, Bottom: y + 16})
			y += 17
		}
		if data.WeeklyAvg != "" {
			drawMarketStat(hdc, tr("hud.weekly_average"), data.WeeklyAvg, win32.RECT{Left: bodyLeft, Top: y, Right: bodyRight, Bottom: y + 16})
			y += 17
		}
		if data.DailySales != "" {
			drawMarketStat(hdc, tr("hud.daily_sales"), data.DailySales, win32.RECT{Left: bodyLeft, Top: y, Right: bodyRight, Bottom: y + 16})
			y += 17
		}
	})

	if data.FallbackNotice != "" {
		fallbackRect := win32.RECT{Left: bodyLeft, Top: y + 2, Right: bodyRight, Bottom: y + 12}
		withOverlayFont(hdc, 10, FW_NORMAL, func() {
			drawOverlayText(hdc, data.FallbackNotice, fallbackRect, colorRGB(178, 150, 92), DT_CENTER|DT_SINGLELINE|DT_VCENTER|DT_END_ELLIPSIS)
		})
		y = fallbackRect.Bottom
	}

	updatedText := data.Updated
	if parsed, err := time.Parse(time.RFC3339, data.Updated); err == nil {
		updatedText = market.FormatRelativeTime(parsed, time.Now())
	}
	if updatedText != "" {
		updatedRect := win32.RECT{Left: bodyLeft, Top: y + 2, Right: bodyRight, Bottom: y + 12}
		withOverlayFont(hdc, 10, FW_NORMAL, func() {
			drawOverlayText(hdc, updatedText, updatedRect, colorRGB(142, 130, 111), DT_RIGHT|DT_SINGLELINE|DT_VCENTER|DT_END_ELLIPSIS)
		})
		y = updatedRect.Bottom
	}

	footerRect := win32.RECT{Left: bodyLeft, Top: y + 5, Right: bodyRight, Bottom: y + 20}
	fillSolidRect(hdc, footerRect, colorRGB(18, 17, 16))
	strokeRect(hdc, footerRect, colorRGB(82, 69, 51), 1)
	withOverlayFont(hdc, 10, FW_BOLD, func() {
		drawOverlayText(hdc, tr("hud.open_market"), insetRect(footerRect, 9, 1), colorRGB(202, 179, 139), DT_CENTER|DT_SINGLELINE|DT_VCENTER|DT_END_ELLIPSIS)
	})
}

func drawMarketStat(hdc uintptr, label string, value string, rect win32.RECT) {
	if value == "" || value == "N/A" || value == "N/A units" || value == tr("value.na") {
		value = tr("value.na")
	}
	fillSolidRect(hdc, rect, colorRGB(14, 14, 13))
	fillSolidRect(hdc, win32.RECT{Left: rect.Left, Top: rect.Bottom - 1, Right: rect.Right, Bottom: rect.Bottom}, colorRGB(50, 45, 38))

	labelRect := win32.RECT{Left: rect.Left + 7, Top: rect.Top, Right: rect.Left + 102, Bottom: rect.Bottom}
	valueRect := win32.RECT{Left: labelRect.Right + 4, Top: rect.Top, Right: rect.Right - 7, Bottom: rect.Bottom}
	drawOverlayText(hdc, label, labelRect, colorRGB(154, 137, 111), DT_LEFT|DT_SINGLELINE|DT_VCENTER|DT_END_ELLIPSIS)
	drawOverlayText(hdc, value, valueRect, colorRGB(225, 213, 191), DT_RIGHT|DT_SINGLELINE|DT_VCENTER|DT_END_ELLIPSIS)
}

func currentPriceOverlayView() overlay.PriceView {
	if analysis, ok := getCurrentMarketAnalysis(); ok {
		return priceOverlayViewFromAnalysis(analysis)
	}
	return parsePriceOverlayView(getCurrentPriceText())
}

func priceOverlayViewFromAnalysis(analysis market.MarketAnalysis) overlay.PriceView {
	lowestSellStr := formatOverlayAnalysisPrice(analysis.LowestSellPrice, analysis.HasLowestSell, analysis, market.USDFallbackLowestSell)
	if analysis.HasLowestSell && analysis.LowestSellQuantity > 1 {
		lowestSellStr = fmt.Sprintf("%s (%d)", lowestSellStr, analysis.LowestSellQuantity)
	}

	highestBuyStr := formatOverlayAnalysisPrice(analysis.HighestBuyPrice, analysis.HasHighestBuy, analysis, market.USDFallbackHighestBuy)
	if analysis.HasHighestBuy && analysis.HighestBuyQuantity > 1 {
		highestBuyStr = fmt.Sprintf("%s (%d)", highestBuyStr, analysis.HighestBuyQuantity)
	}

	view := overlay.PriceView{
		Suggested:  formatOverlayAnalysisPrice(analysis.SuggestedPrice, analysis.HasSuggested, analysis, market.USDFallbackSuggested),
		LowestSell: lowestSellStr,
		HighestBuy: highestBuyStr,
		DailySales: market.FormatAnalysisVolume(analysis.DailySalesVolume, analysis.HasDailySales, analysis.VolumeActivity),
		WeeklyAvg:  formatOverlayAnalysisPrice(analysis.WeeklyAveragePrice, analysis.HasWeeklyAverage, analysis, market.USDFallbackWeeklyAverage),
		LastSold:   formatOverlayAnalysisPrice(analysis.LastSoldPrice, analysis.HasLastSold, analysis, market.USDFallbackLastSold),
		Trend:      market.FormatTrendPercent(analysis.TrendPercent, analysis.HasTrend),
		Spread:     market.FormatSpread(analysis),
		Orders:     market.FormatOrderCounts(analysis.BuyOrderCount, analysis.SellOrderCount, analysis.HasOrderBook),
		DealTag:    analysis.DealTag,
		Confidence: analysis.Confidence,
	}
	if analysis.HasRecentSaleP75 {
		view.SaleP75 = formatOverlayAnalysisPrice(analysis.RecentSaleP75Price, true, analysis, market.USDFallbackSaleP75)
	}
	if !analysis.UpdatedAt.IsZero() {
		view.Updated = analysis.UpdatedAt.Format(time.RFC3339)
	}
	return view
}

func formatOverlayAnalysisPrice(price float64, ok bool, analysis market.MarketAnalysis, usdFallbackMetric uint16) string {
	return market.FormatAnalysisPrice(price, ok, analysis)
}

func parsePriceOverlayView(text string) overlay.PriceView {
	return overlay.ParsePriceView(text, AppShortName+" Suggested")
}

func drawOverlayText(hdc uintptr, text string, rect win32.RECT, color uintptr, format uintptr) {
	if text == "" || rect.Right <= rect.Left || rect.Bottom <= rect.Top {
		return
	}
	textUTF16, _ := syscall.UTF16FromString(text)
	win32.ProcSetTextColor.Call(hdc, color)
	win32.ProcDrawTextW.Call(hdc, uintptr(unsafe.Pointer(&textUTF16[0])), ^uintptr(0), uintptr(unsafe.Pointer(&rect)), format)
}

func withOverlayFont(hdc uintptr, height int32, weight uintptr, draw func()) {
	font := createOverlayFont(height, weight)
	if font == 0 {
		draw()
		return
	}

	oldFont, _, _ := win32.ProcSelectObject.Call(hdc, font)
	draw()
	if oldFont != 0 {
		win32.ProcSelectObject.Call(hdc, oldFont)
	}
	win32.ProcDeleteObject.Call(font)
}

func createOverlayFont(height int32, weight uintptr) uintptr {
	faceName, _ := syscall.UTF16PtrFromString("Tahoma")
	font, _, _ := win32.ProcCreateFontW.Call(
		uintptr(uint32(height)),
		0,
		0,
		0,
		weight,
		0,
		0,
		0,
		DEFAULT_CHARSET,
		0,
		0,
		DEFAULT_QUALITY,
		0,
		uintptr(unsafe.Pointer(faceName)),
	)
	return font
}

func fillSolidRect(hdc uintptr, rect win32.RECT, color uintptr) {
	if rect.Right <= rect.Left || rect.Bottom <= rect.Top {
		return
	}
	brush, _, _ := win32.ProcCreateSolidBrush.Call(color)
	if brush == 0 {
		return
	}
	win32.ProcFillRect.Call(hdc, uintptr(unsafe.Pointer(&rect)), brush)
	win32.ProcDeleteObject.Call(brush)
}

func strokeRect(hdc uintptr, rect win32.RECT, color uintptr, width int32) {
	if rect.Right <= rect.Left || rect.Bottom <= rect.Top || width <= 0 {
		return
	}
	pen, _, _ := win32.ProcCreatePen.Call(PS_SOLID, uintptr(uint32(width)), color)
	if pen == 0 {
		return
	}
	oldPen, _, _ := win32.ProcSelectObject.Call(hdc, pen)

	left := uintptr(int(rect.Left))
	top := uintptr(int(rect.Top))
	right := uintptr(int(rect.Right - 1))
	bottom := uintptr(int(rect.Bottom - 1))
	win32.ProcMoveToEx.Call(hdc, left, top, 0)
	win32.ProcLineTo.Call(hdc, right, top)
	win32.ProcLineTo.Call(hdc, right, bottom)
	win32.ProcLineTo.Call(hdc, left, bottom)
	win32.ProcLineTo.Call(hdc, left, top)

	if oldPen != 0 {
		win32.ProcSelectObject.Call(hdc, oldPen)
	}
	win32.ProcDeleteObject.Call(pen)
}

func insetRect(rect win32.RECT, dx int32, dy int32) win32.RECT {
	return overlay.InsetRect(rect, dx, dy)
}

func colorRGB(r byte, g byte, b byte) uintptr {
	return uintptr(uint32(r) | uint32(g)<<8 | uint32(b)<<16)
}
