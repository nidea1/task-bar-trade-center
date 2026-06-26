package app

import (
	"unsafe"

	"github.com/nidea1/task-bar-trade-center/internal/overlay"
)

func marketOverlayRect() (RECT, bool) {
	if tooltipRect, ok := readTooltipRectFromMemory(); ok {
		LastOverlayRect = placeOverlayByTooltipRect(tooltipRect)
		HasLastOverlayRect = true
		return LastOverlayRect, true
	}
	if !ShowOverlay.Load() {
		return RECT{}, false
	}
	if cursor, ok := cursorScreenPosition(); ok {
		LastOverlayRect = fallbackOverlayRect(cursor)
		HasLastOverlayRect = true
		return LastOverlayRect, true
	}
	if HasLastOverlayRect {
		return LastOverlayRect, true
	}
	return RECT{}, false
}

var cursorScreenPosition = func() (POINT, bool) {
	var cursor POINT
	result, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&cursor)))
	return cursor, result != 0
}

func findTooltipRect(cursor POINT) (RECT, bool) {
	screen := virtualScreenRect()
	if screen.Right <= screen.Left || screen.Bottom <= screen.Top {
		return RECT{}, false
	}

	left := overlay.ClampInt32(cursor.X-360, screen.Left, screen.Right-1)
	right := overlay.ClampInt32(cursor.X+360, screen.Left, screen.Right-1)
	top := overlay.ClampInt32(cursor.Y-520, screen.Top, screen.Bottom-1)
	bottom := overlay.ClampInt32(cursor.Y+260, screen.Top, screen.Bottom-1)
	if right <= left || bottom <= top {
		return RECT{}, false
	}

	hdc, _, _ := procGetDC.Call(0)
	if hdc == 0 {
		return RECT{}, false
	}
	defer procReleaseDC.Call(0, hdc)

	step := int32(TooltipScanStep)
	cols := int((right-left)/step) + 1
	rows := int((bottom-top)/step) + 1
	panelPixels := make([]bool, cols*rows)

	for row := 0; row < rows; row++ {
		y := top + int32(row)*step
		for col := 0; col < cols; col++ {
			x := left + int32(col)*step
			color, _, _ := procGetPixel.Call(hdc, uintptr(x), uintptr(y))
			if color != 0xFFFFFFFF && overlay.IsTooltipPanelPixel(uint32(color)) {
				panelPixels[row*cols+col] = true
			}
		}
	}

	visited := make([]bool, len(panelPixels))
	bestScore := -1
	var best RECT

	for i, isPanel := range panelPixels {
		if !isPanel || visited[i] {
			continue
		}

		count, minCol, maxCol, minRow, maxRow := collectComponent(panelPixels, visited, cols, rows, i)
		width := int32(maxCol-minCol+1) * step
		height := int32(maxRow-minRow+1) * step
		if width < 150 || width > 650 || height < 80 || height > 560 || count < 120 {
			continue
		}

		rect := RECT{
			Left:   overlay.ClampInt32(left+int32(minCol)*step-step, screen.Left, screen.Right-1),
			Top:    overlay.ClampInt32(top+int32(minRow)*step-step, screen.Top, screen.Bottom-1),
			Right:  overlay.ClampInt32(left+int32(maxCol+1)*step+step, screen.Left, screen.Right),
			Bottom: overlay.ClampInt32(top+int32(maxRow+1)*step+step, screen.Top, screen.Bottom),
		}
		if overlay.DistancePointToRect(cursor, rect) > 140 {
			continue
		}

		centerY := (rect.Top + rect.Bottom) / 2
		verticalPenalty := overlay.AbsInt(int(centerY - cursor.Y))
		sizePenalty := overlay.AbsInt(int(width-250)) + overlay.AbsInt(int(height-260))
		score := 2000 - overlay.DistancePointToRect(cursor, rect)*4 - verticalPenalty - sizePenalty + count/8
		if score > bestScore {
			bestScore = score
			best = rect
		}
	}

	if bestScore < 0 {
		return RECT{}, false
	}
	return best, true
}

func collectComponent(panelPixels []bool, visited []bool, cols int, rows int, start int) (int, int, int, int, int) {
	queue := []int{start}
	visited[start] = true
	minCol, maxCol := start%cols, start%cols
	minRow, maxRow := start/cols, start/cols
	count := 0

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		count++

		col := current % cols
		row := current / cols
		if col < minCol {
			minCol = col
		}
		if col > maxCol {
			maxCol = col
		}
		if row < minRow {
			minRow = row
		}
		if row > maxRow {
			maxRow = row
		}

		neighbors := [4]int{-1, 1, -cols, cols}
		for _, delta := range neighbors {
			next := current + delta
			if next < 0 || next >= len(panelPixels) || visited[next] || !panelPixels[next] {
				continue
			}
			nextCol := next % cols
			nextRow := next / cols
			if overlay.AbsInt(nextCol-col)+overlay.AbsInt(nextRow-row) != 1 {
				continue
			}
			visited[next] = true
			queue = append(queue, next)
		}
	}

	return count, minCol, maxCol, minRow, maxRow
}

func activeOverlayHeight() int32 {
	data := currentPriceOverlayView()
	return calculateRequiredHeight(data, OverlayMode.Load())
}

func placeOverlayByTooltipRect(tooltipRect RECT) RECT {
	screen := virtualScreenRect()
	var clientOrigin POINT
	hasClientOrigin := false
	if origin, ok := gameClientScreenOrigin(); ok {
		clientOrigin = origin
		hasClientOrigin = true
	}
	GameLayoutMu.RLock()
	placementCalibrations := append([]OverlayPlacementCalibration(nil), ActiveGameLayout.PlacementCalibrations...)
	xCalibrations := append([]OverlayXCalibration(nil), ActiveGameLayout.XCalibrations...)
	GameLayoutMu.RUnlock()
	return overlay.PlaceByTooltipRect(tooltipRect, screen, clientOrigin, hasClientOrigin, activeOverlayHeight(), placementCalibrations, xCalibrations, overlayPlacementConfig())
}

func scaleByReference(value int32, referenceValue int32, referenceBase int32) int32 {
	return overlay.ScaleByReference(value, referenceValue, referenceBase)
}

func overlayPlacementForTooltip(localY int32, height int32) OverlayPlacementCalibration {
	GameLayoutMu.RLock()
	calibrations := append([]OverlayPlacementCalibration(nil), ActiveGameLayout.PlacementCalibrations...)
	GameLayoutMu.RUnlock()
	return overlay.PlacementForTooltip(localY, height, calibrations, overlayPlacementConfig())
}

func findClosestXOffset(localX int32) int32 {
	GameLayoutMu.RLock()
	calibrations := append([]OverlayXCalibration(nil), ActiveGameLayout.XCalibrations...)
	GameLayoutMu.RUnlock()
	return overlay.FindClosestXOffset(localX, calibrations)
}

func roundFloat32ToInt32(val float32) int32 {
	return overlay.RoundFloat32ToInt32(val)
}

func absInt32(value int32) int32 {
	return overlay.AbsInt32(value)
}

func fallbackOverlayRect(cursor POINT) RECT {
	return overlay.FallbackRect(cursor, virtualScreenRect(), activeOverlayHeight(), overlayPlacementConfig())
}

func isTooltipPanelPixel(color uint32) bool {
	return overlay.IsTooltipPanelPixel(color)
}

func distancePointToRect(point POINT, rect RECT) int {
	return overlay.DistancePointToRect(point, rect)
}

func overlayClientRect(screenRect RECT) RECT {
	return RECT{
		Left:   screenRect.Left - OverlayOriginX,
		Top:    screenRect.Top - OverlayOriginY,
		Right:  screenRect.Right - OverlayOriginX,
		Bottom: screenRect.Bottom - OverlayOriginY,
	}
}

func virtualScreenRect() RECT {
	left := getSystemMetric(SM_XVIRTUALSCREEN)
	top := getSystemMetric(SM_YVIRTUALSCREEN)
	width := getSystemMetric(SM_CXVIRTUALSCREEN)
	height := getSystemMetric(SM_CYVIRTUALSCREEN)
	if width <= 0 || height <= 0 {
		left = 0
		top = 0
		width = getSystemMetric(SM_CXSCREEN)
		height = getSystemMetric(SM_CYSCREEN)
	}
	return RECT{
		Left:   left,
		Top:    top,
		Right:  left + width,
		Bottom: top + height,
	}
}

func getSystemMetric(index int32) int32 {
	value, _, _ := procGetSystemMetrics.Call(uintptr(index))
	return int32(value)
}

func clampInt32(value int32, min int32, max int32) int32 {
	return overlay.ClampInt32(value, min, max)
}

func absInt(value int) int {
	return overlay.AbsInt(value)
}

func overlayPlacementConfig() overlay.PlacementConfig {
	return overlay.PlacementConfig{
		OffsetX:             TooltipOverlayOffsetX,
		OffsetY:             TooltipOverlayOffsetY,
		Width:               TooltipOverlayWidth,
		ReferenceHeight:     TooltipOverlayReferenceHeight,
		ReferencePanelWidth: TooltipOverlayReferencePanelWidth,
		MinWidth:            TooltipOverlayMinWidth,
		MaxWidth:            TooltipOverlayMaxWidth,
		AnchorOffsetX:       TooltipOverlayAnchorOffsetX,
		AnchorOffsetY:       TooltipOverlayAnchorOffsetY,
	}
}
