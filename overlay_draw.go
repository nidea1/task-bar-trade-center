package main

import (
	"strings"
	"syscall"
	"time"
	"unsafe"
)

func drawGameMarketOverlay(hdc uintptr, rect RECT) {
	if OverlayMode.Load() == OverlayModeCompact {
		drawCompactOverlay(hdc, rect)
		return
	}
	drawDetailOverlay(hdc, rect)
}

func calculateRequiredHeight(data PriceOverlayView, mode int32) int32 {
	y := int32(94)

	if mode == OverlayModeCompact {
		y += 5
		if data.LastSold != "" {
			y += 17
		}
		if data.LowestSell != "" {
			y += 17
		}
		if data.HighestBuy != "" {
			y += 17
		}
		if data.WeeklyAvg != "" {
			y += 17
		}
		if data.DailySales != "" {
			y += 17
		}
	} else {
		y += 3
		if data.DealTag != "" {
			y += 18
		}
		y += 2

		if data.LastSold != "" {
			y += 17
		}
		if data.LowestSell != "" {
			y += 17
		}
		if data.HighestBuy != "" {
			y += 17
		}
		if data.WeeklyAvg != "" {
			y += 17
		}
		if data.SaleP75 != "" {
			y += 17
		}
		if data.Spread != "" {
			y += 17
		}
		if data.Trend != "" {
			y += 17
		}
		if data.DailySales != "" {
			y += 17
		}
		if data.Orders != "" {
			y += 17
		}
	}

	if data.Updated != "" {
		y += 12
	}

	y += 34
	return y
}

func drawDetailOverlay(hdc uintptr, rect RECT) {
	procSetBkMode.Call(hdc, TRANSPARENT)

	fillSolidRect(hdc, rect, colorRGB(3, 3, 4))
	strokeRect(hdc, rect, colorRGB(0, 0, 0), 2)

	frame := insetRect(rect, 3, 3)
	fillSolidRect(hdc, frame, colorRGB(24, 20, 19))
	fillSolidRect(hdc, RECT{Left: frame.Left + 2, Top: frame.Top + 2, Right: frame.Right - 2, Bottom: frame.Top + 4}, colorRGB(126, 83, 38))
	fillSolidRect(hdc, RECT{Left: frame.Left + 2, Top: frame.Bottom - 4, Right: frame.Right - 2, Bottom: frame.Bottom - 2}, colorRGB(48, 32, 25))

	inner := insetRect(frame, 4, 4)
	fillSolidRect(hdc, inner, colorRGB(10, 11, 12))
	strokeRect(hdc, inner, colorRGB(70, 61, 48), 1)

	header := RECT{Left: inner.Left + 5, Top: inner.Top + 5, Right: inner.Right - 5, Bottom: inner.Top + 27}
	fillSolidRect(hdc, header, colorRGB(96, 27, 24))
	fillSolidRect(hdc, RECT{Left: header.Left, Top: header.Top, Right: header.Right, Bottom: header.Top + 2}, colorRGB(174, 67, 42))
	fillSolidRect(hdc, RECT{Left: header.Left, Top: header.Bottom - 2, Right: header.Right, Bottom: header.Bottom}, colorRGB(41, 13, 11))
	strokeRect(hdc, header, colorRGB(227, 169, 67), 1)

	withOverlayFont(hdc, 16, FW_BOLD, func() {
		drawOverlayText(hdc, "TRADE CENTER", header, colorRGB(255, 190, 45), DT_CENTER|DT_SINGLELINE|DT_VCENTER)
	})

	itemName := getCurrentItemName()
	if itemName == "" {
		itemName = "Market Price"
	}
	itemRect := RECT{Left: inner.Left + 8, Top: header.Bottom + 4, Right: inner.Right - 8, Bottom: header.Bottom + 26}
	fillSolidRect(hdc, itemRect, colorRGB(25, 21, 18))
	strokeRect(hdc, itemRect, colorRGB(130, 99, 61), 1)
	withOverlayFont(hdc, 13, FW_BOLD, func() {
		drawOverlayText(hdc, itemName, insetRect(itemRect, 8, 0), colorRGB(235, 205, 156), DT_CENTER|DT_SINGLELINE|DT_VCENTER|DT_END_ELLIPSIS)
	})

	data := parsePriceOverlayView(getCurrentPriceText())
	bodyLeft := inner.Left + 10
	bodyRight := inner.Right - 10
	y := itemRect.Bottom + 6

	suggestedRect := RECT{Left: bodyLeft, Top: y, Right: bodyRight, Bottom: y + 28}
	fillSolidRect(hdc, suggestedRect, colorRGB(42, 25, 14))
	fillSolidRect(hdc, RECT{Left: suggestedRect.Left, Top: suggestedRect.Top, Right: suggestedRect.Right, Bottom: suggestedRect.Top + 2}, colorRGB(131, 75, 30))
	fillSolidRect(hdc, RECT{Left: suggestedRect.Left, Top: suggestedRect.Bottom - 2, Right: suggestedRect.Right, Bottom: suggestedRect.Bottom}, colorRGB(21, 13, 9))
	strokeRect(hdc, suggestedRect, colorRGB(159, 101, 42), 1)
	withOverlayFont(hdc, 12, FW_BOLD, func() {
		labelRect := RECT{Left: suggestedRect.Left + 8, Top: suggestedRect.Top, Right: suggestedRect.Left + 128, Bottom: suggestedRect.Bottom}
		valueRect := RECT{Left: labelRect.Right, Top: suggestedRect.Top, Right: suggestedRect.Right - 8, Bottom: suggestedRect.Bottom}
		suggestedLabel := "Suggested"
		if data.Confidence != "" {
			suggestedLabel += " [" + data.Confidence + "]"
		}
		drawOverlayText(hdc, suggestedLabel, labelRect, colorRGB(202, 179, 139), DT_LEFT|DT_SINGLELINE|DT_VCENTER)
		drawOverlayText(hdc, data.Suggested, valueRect, colorRGB(84, 220, 94), DT_RIGHT|DT_SINGLELINE|DT_VCENTER|DT_END_ELLIPSIS)
	})

	y = suggestedRect.Bottom + 3
	if data.DealTag != "" {
		dealTagRect := RECT{Left: bodyLeft, Top: y, Right: bodyRight, Bottom: y + 16}
		dealColor := colorRGB(84, 220, 94)
		dealBgColor := colorRGB(20, 42, 20)
		if data.DealTag == "Overvalued" {
			dealColor = colorRGB(240, 80, 70)
			dealBgColor = colorRGB(42, 16, 14)
		}
		fillSolidRect(hdc, dealTagRect, dealBgColor)
		withOverlayFont(hdc, 11, FW_BOLD, func() {
			drawOverlayText(hdc, data.DealTag, dealTagRect, dealColor, DT_CENTER|DT_SINGLELINE|DT_VCENTER)
		})
		y += 18
	}

	y += 2
	withOverlayFont(hdc, 12, FW_NORMAL, func() {
		if data.LastSold != "" {
			drawMarketStat(hdc, "Last Sold", data.LastSold, RECT{Left: bodyLeft, Top: y, Right: bodyRight, Bottom: y + 16})
			y += 17
		}
		if data.LowestSell != "" {
			drawMarketStat(hdc, "Lowest Sell", data.LowestSell, RECT{Left: bodyLeft, Top: y, Right: bodyRight, Bottom: y + 16})
			y += 17
		}
		if data.HighestBuy != "" {
			drawMarketStat(hdc, "Highest Buy", data.HighestBuy, RECT{Left: bodyLeft, Top: y, Right: bodyRight, Bottom: y + 16})
			y += 17
		}
		if data.WeeklyAvg != "" {
			drawMarketStat(hdc, "Weekly Avg", data.WeeklyAvg, RECT{Left: bodyLeft, Top: y, Right: bodyRight, Bottom: y + 16})
			y += 17
		}
		if data.SaleP75 != "" {
			drawMarketStat(hdc, "Sale P75", data.SaleP75, RECT{Left: bodyLeft, Top: y, Right: bodyRight, Bottom: y + 16})
			y += 17
		}
		if data.Spread != "" {
			drawMarketStat(hdc, "Spread", data.Spread, RECT{Left: bodyLeft, Top: y, Right: bodyRight, Bottom: y + 16})
			y += 17
		}
		if data.Trend != "" {
			drawMarketStat(hdc, "Trend", data.Trend, RECT{Left: bodyLeft, Top: y, Right: bodyRight, Bottom: y + 16})
			y += 17
		}
		if data.DailySales != "" {
			drawMarketStat(hdc, "Daily Sales", data.DailySales, RECT{Left: bodyLeft, Top: y, Right: bodyRight, Bottom: y + 16})
			y += 17
		}
		if data.Orders != "" {
			drawMarketStat(hdc, "Orders", data.Orders, RECT{Left: bodyLeft, Top: y, Right: bodyRight, Bottom: y + 16})
			y += 17
		}
	})

	updatedText := data.Updated
	if parsed, err := time.Parse(time.RFC3339, data.Updated); err == nil {
		updatedText = formatRelativeTime(parsed, time.Now())
	}
	if updatedText != "" {
		updatedRect := RECT{Left: bodyLeft, Top: y + 2, Right: bodyRight, Bottom: y + 12}
		withOverlayFont(hdc, 10, FW_NORMAL, func() {
			drawOverlayText(hdc, updatedText, updatedRect, colorRGB(142, 130, 111), DT_RIGHT|DT_SINGLELINE|DT_VCENTER|DT_END_ELLIPSIS)
		})
		y = updatedRect.Bottom
	}

	footerRect := RECT{Left: bodyLeft, Top: y + 5, Right: bodyRight, Bottom: y + 20}
	fillSolidRect(hdc, footerRect, colorRGB(18, 17, 16))
	strokeRect(hdc, footerRect, colorRGB(82, 69, 51), 1)
	withOverlayFont(hdc, 10, FW_BOLD, func() {
		drawOverlayText(hdc, "Middle Click: Open Steam Market", insetRect(footerRect, 9, 1), colorRGB(202, 179, 139), DT_CENTER|DT_SINGLELINE|DT_VCENTER|DT_END_ELLIPSIS)
	})
}

func drawCompactOverlay(hdc uintptr, rect RECT) {
	procSetBkMode.Call(hdc, TRANSPARENT)

	fillSolidRect(hdc, rect, colorRGB(3, 3, 4))
	strokeRect(hdc, rect, colorRGB(0, 0, 0), 2)

	frame := insetRect(rect, 3, 3)
	fillSolidRect(hdc, frame, colorRGB(24, 20, 19))
	fillSolidRect(hdc, RECT{Left: frame.Left + 2, Top: frame.Top + 2, Right: frame.Right - 2, Bottom: frame.Top + 4}, colorRGB(126, 83, 38))
	fillSolidRect(hdc, RECT{Left: frame.Left + 2, Top: frame.Bottom - 4, Right: frame.Right - 2, Bottom: frame.Bottom - 2}, colorRGB(48, 32, 25))

	inner := insetRect(frame, 4, 4)
	fillSolidRect(hdc, inner, colorRGB(10, 11, 12))
	strokeRect(hdc, inner, colorRGB(70, 61, 48), 1)

	header := RECT{Left: inner.Left + 5, Top: inner.Top + 5, Right: inner.Right - 5, Bottom: inner.Top + 27}
	fillSolidRect(hdc, header, colorRGB(96, 27, 24))
	fillSolidRect(hdc, RECT{Left: header.Left, Top: header.Top, Right: header.Right, Bottom: header.Top + 2}, colorRGB(174, 67, 42))
	fillSolidRect(hdc, RECT{Left: header.Left, Top: header.Bottom - 2, Right: header.Right, Bottom: header.Bottom}, colorRGB(41, 13, 11))
	strokeRect(hdc, header, colorRGB(227, 169, 67), 1)

	withOverlayFont(hdc, 16, FW_BOLD, func() {
		drawOverlayText(hdc, "TRADE CENTER", header, colorRGB(255, 190, 45), DT_CENTER|DT_SINGLELINE|DT_VCENTER)
	})

	itemName := getCurrentItemName()
	if itemName == "" {
		itemName = "Market Price"
	}
	itemRect := RECT{Left: inner.Left + 8, Top: header.Bottom + 4, Right: inner.Right - 8, Bottom: header.Bottom + 26}
	fillSolidRect(hdc, itemRect, colorRGB(25, 21, 18))
	strokeRect(hdc, itemRect, colorRGB(130, 99, 61), 1)
	withOverlayFont(hdc, 13, FW_BOLD, func() {
		drawOverlayText(hdc, itemName, insetRect(itemRect, 8, 0), colorRGB(235, 205, 156), DT_CENTER|DT_SINGLELINE|DT_VCENTER|DT_END_ELLIPSIS)
	})

	data := parsePriceOverlayView(getCurrentPriceText())
	bodyLeft := inner.Left + 10
	bodyRight := inner.Right - 10
	y := itemRect.Bottom + 6

	suggestedRect := RECT{Left: bodyLeft, Top: y, Right: bodyRight, Bottom: y + 28}
	fillSolidRect(hdc, suggestedRect, colorRGB(42, 25, 14))
	fillSolidRect(hdc, RECT{Left: suggestedRect.Left, Top: suggestedRect.Top, Right: suggestedRect.Right, Bottom: suggestedRect.Top + 2}, colorRGB(131, 75, 30))
	fillSolidRect(hdc, RECT{Left: suggestedRect.Left, Top: suggestedRect.Bottom - 2, Right: suggestedRect.Right, Bottom: suggestedRect.Bottom}, colorRGB(21, 13, 9))
	strokeRect(hdc, suggestedRect, colorRGB(159, 101, 42), 1)
	withOverlayFont(hdc, 12, FW_BOLD, func() {
		labelRect := RECT{Left: suggestedRect.Left + 8, Top: suggestedRect.Top, Right: suggestedRect.Left + 128, Bottom: suggestedRect.Bottom}
		valueRect := RECT{Left: labelRect.Right, Top: suggestedRect.Top, Right: suggestedRect.Right - 8, Bottom: suggestedRect.Bottom}
		drawOverlayText(hdc, "Suggested", labelRect, colorRGB(202, 179, 139), DT_LEFT|DT_SINGLELINE|DT_VCENTER)
		drawOverlayText(hdc, data.Suggested, valueRect, colorRGB(84, 220, 94), DT_RIGHT|DT_SINGLELINE|DT_VCENTER|DT_END_ELLIPSIS)
	})

	y = suggestedRect.Bottom + 5
	withOverlayFont(hdc, 12, FW_NORMAL, func() {
		if data.LastSold != "" {
			drawMarketStat(hdc, "Last Sold", data.LastSold, RECT{Left: bodyLeft, Top: y, Right: bodyRight, Bottom: y + 16})
			y += 17
		}
		if data.LowestSell != "" {
			drawMarketStat(hdc, "Lowest Sell", data.LowestSell, RECT{Left: bodyLeft, Top: y, Right: bodyRight, Bottom: y + 16})
			y += 17
		}
		if data.HighestBuy != "" {
			drawMarketStat(hdc, "Highest Buy", data.HighestBuy, RECT{Left: bodyLeft, Top: y, Right: bodyRight, Bottom: y + 16})
			y += 17
		}
		if data.WeeklyAvg != "" {
			drawMarketStat(hdc, "Weekly Avg", data.WeeklyAvg, RECT{Left: bodyLeft, Top: y, Right: bodyRight, Bottom: y + 16})
			y += 17
		}
		if data.DailySales != "" {
			drawMarketStat(hdc, "Daily Sales", data.DailySales, RECT{Left: bodyLeft, Top: y, Right: bodyRight, Bottom: y + 16})
			y += 17
		}
	})

	updatedText := data.Updated
	if parsed, err := time.Parse(time.RFC3339, data.Updated); err == nil {
		updatedText = formatRelativeTime(parsed, time.Now())
	}
	if updatedText != "" {
		updatedRect := RECT{Left: bodyLeft, Top: y + 2, Right: bodyRight, Bottom: y + 12}
		withOverlayFont(hdc, 10, FW_NORMAL, func() {
			drawOverlayText(hdc, updatedText, updatedRect, colorRGB(142, 130, 111), DT_RIGHT|DT_SINGLELINE|DT_VCENTER|DT_END_ELLIPSIS)
		})
		y = updatedRect.Bottom
	}

	footerRect := RECT{Left: bodyLeft, Top: y + 5, Right: bodyRight, Bottom: y + 20}
	fillSolidRect(hdc, footerRect, colorRGB(18, 17, 16))
	strokeRect(hdc, footerRect, colorRGB(82, 69, 51), 1)
	withOverlayFont(hdc, 10, FW_BOLD, func() {
		drawOverlayText(hdc, "Middle Click: Open Steam Market", insetRect(footerRect, 9, 1), colorRGB(202, 179, 139), DT_CENTER|DT_SINGLELINE|DT_VCENTER|DT_END_ELLIPSIS)
	})
}

func drawMarketStat(hdc uintptr, label string, value string, rect RECT) {
	if value == "" || value == "N/A" || value == "N/A units" {
		value = "N/A"
	}
	fillSolidRect(hdc, rect, colorRGB(14, 14, 13))
	fillSolidRect(hdc, RECT{Left: rect.Left, Top: rect.Bottom - 1, Right: rect.Right, Bottom: rect.Bottom}, colorRGB(50, 45, 38))

	labelRect := RECT{Left: rect.Left + 7, Top: rect.Top, Right: rect.Left + 102, Bottom: rect.Bottom}
	valueRect := RECT{Left: labelRect.Right + 4, Top: rect.Top, Right: rect.Right - 7, Bottom: rect.Bottom}
	drawOverlayText(hdc, label, labelRect, colorRGB(154, 137, 111), DT_LEFT|DT_SINGLELINE|DT_VCENTER|DT_END_ELLIPSIS)
	drawOverlayText(hdc, value, valueRect, colorRGB(225, 213, 191), DT_RIGHT|DT_SINGLELINE|DT_VCENTER|DT_END_ELLIPSIS)
}

func parsePriceOverlayView(text string) PriceOverlayView {
	lines := splitOverlayLines(text)
	view := PriceOverlayView{Suggested: strings.TrimSpace(text)}
	if len(lines) == 0 {
		return view
	}

	lineMap := make(map[string]string, len(lines))
	for _, line := range lines {
		if idx := strings.Index(line, ":"); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			lineMap[key] = val
		}
	}

	if suggested, ok := lineMap[AppShortName+" Suggested"]; ok {
		parts := strings.SplitN(suggested, " [", 2)
		view.Suggested = strings.TrimSpace(parts[0])
		if len(parts) > 1 {
			view.Confidence = strings.TrimRight(parts[1], "]")
		}
	}
	if v, ok := lineMap["Last Sold"]; ok {
		view.LastSold = v
	}
	if v, ok := lineMap["Lowest Sell"]; ok {
		view.LowestSell = v
	}
	if v, ok := lineMap["Highest Buy"]; ok {
		view.HighestBuy = v
	}
	if v, ok := lineMap["Weekly Avg"]; ok {
		view.WeeklyAvg = v
	}
	if v, ok := lineMap["Daily Sales"]; ok {
		view.DailySales = v
	}
	if v, ok := lineMap["Updated"]; ok {
		view.Updated = v
	}
	if v, ok := lineMap["DealTag"]; ok {
		view.DealTag = v
	}
	if v, ok := lineMap["Sale P75"]; ok {
		view.SaleP75 = v
	}
	if v, ok := lineMap["Spread"]; ok {
		view.Spread = v
	}
	if v, ok := lineMap["Trend"]; ok {
		view.Trend = v
	}
	if v, ok := lineMap["Orders"]; ok {
		view.Orders = v
	}
	return view
}

func splitOverlayLines(text string) []string {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	rawLines := strings.Split(normalized, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func trimMarketPrefix(line string, prefix string) string {
	if strings.HasPrefix(line, prefix) {
		return strings.TrimSpace(strings.TrimPrefix(line, prefix))
	}
	return strings.TrimSpace(line)
}

func drawOverlayText(hdc uintptr, text string, rect RECT, color uintptr, format uintptr) {
	if text == "" || rect.Right <= rect.Left || rect.Bottom <= rect.Top {
		return
	}
	textUTF16, _ := syscall.UTF16FromString(text)
	procSetTextColor.Call(hdc, color)
	procDrawTextW.Call(hdc, uintptr(unsafe.Pointer(&textUTF16[0])), ^uintptr(0), uintptr(unsafe.Pointer(&rect)), format)
}

func withOverlayFont(hdc uintptr, height int32, weight uintptr, draw func()) {
	font := createOverlayFont(height, weight)
	if font == 0 {
		draw()
		return
	}

	oldFont, _, _ := procSelectObject.Call(hdc, font)
	draw()
	if oldFont != 0 {
		procSelectObject.Call(hdc, oldFont)
	}
	procDeleteObject.Call(font)
}

func createOverlayFont(height int32, weight uintptr) uintptr {
	faceName, _ := syscall.UTF16PtrFromString("Tahoma")
	font, _, _ := procCreateFontW.Call(
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

func fillSolidRect(hdc uintptr, rect RECT, color uintptr) {
	if rect.Right <= rect.Left || rect.Bottom <= rect.Top {
		return
	}
	brush, _, _ := procCreateSolidBrush.Call(color)
	if brush == 0 {
		return
	}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&rect)), brush)
	procDeleteObject.Call(brush)
}

func strokeRect(hdc uintptr, rect RECT, color uintptr, width int32) {
	if rect.Right <= rect.Left || rect.Bottom <= rect.Top || width <= 0 {
		return
	}
	pen, _, _ := procCreatePen.Call(PS_SOLID, uintptr(uint32(width)), color)
	if pen == 0 {
		return
	}
	oldPen, _, _ := procSelectObject.Call(hdc, pen)

	left := uintptr(int(rect.Left))
	top := uintptr(int(rect.Top))
	right := uintptr(int(rect.Right - 1))
	bottom := uintptr(int(rect.Bottom - 1))
	procMoveToEx.Call(hdc, left, top, 0)
	procLineTo.Call(hdc, right, top)
	procLineTo.Call(hdc, right, bottom)
	procLineTo.Call(hdc, left, bottom)
	procLineTo.Call(hdc, left, top)

	if oldPen != 0 {
		procSelectObject.Call(hdc, oldPen)
	}
	procDeleteObject.Call(pen)
}

func insetRect(rect RECT, dx int32, dy int32) RECT {
	return RECT{
		Left:   rect.Left + dx,
		Top:    rect.Top + dy,
		Right:  rect.Right - dx,
		Bottom: rect.Bottom - dy,
	}
}

func colorRGB(r byte, g byte, b byte) uintptr {
	return uintptr(uint32(r) | uint32(g)<<8 | uint32(b)<<16)
}
