package main

func marketOverlayRect() (RECT, bool) {
	if tooltipRect, ok := readTooltipRectFromMemory(); ok {
		LastOverlayRect = placeOverlayByTooltipRect(tooltipRect)
		HasLastOverlayRect = true
		return LastOverlayRect, true
	}
	if !ShowOverlay.Load() {
		return RECT{}, false
	}
	if HasLastOverlayRect {
		return LastOverlayRect, true
	}
	return RECT{}, false
}

func findTooltipRect(cursor POINT) (RECT, bool) {
	screen := virtualScreenRect()
	if screen.Right <= screen.Left || screen.Bottom <= screen.Top {
		return RECT{}, false
	}

	left := clampInt32(cursor.X-360, screen.Left, screen.Right-1)
	right := clampInt32(cursor.X+360, screen.Left, screen.Right-1)
	top := clampInt32(cursor.Y-520, screen.Top, screen.Bottom-1)
	bottom := clampInt32(cursor.Y+260, screen.Top, screen.Bottom-1)
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
			if color != 0xFFFFFFFF && isTooltipPanelPixel(uint32(color)) {
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
			Left:   clampInt32(left+int32(minCol)*step-step, screen.Left, screen.Right-1),
			Top:    clampInt32(top+int32(minRow)*step-step, screen.Top, screen.Bottom-1),
			Right:  clampInt32(left+int32(maxCol+1)*step+step, screen.Left, screen.Right),
			Bottom: clampInt32(top+int32(maxRow+1)*step+step, screen.Top, screen.Bottom),
		}
		if distancePointToRect(cursor, rect) > 140 {
			continue
		}

		centerY := (rect.Top + rect.Bottom) / 2
		verticalPenalty := absInt(int(centerY - cursor.Y))
		sizePenalty := absInt(int(width-250)) + absInt(int(height-260))
		score := 2000 - distancePointToRect(cursor, rect)*4 - verticalPenalty - sizePenalty + count/8
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
			if absInt(nextCol-col)+absInt(nextRow-row) != 1 {
				continue
			}
			visited[next] = true
			queue = append(queue, next)
		}
	}

	return count, minCol, maxCol, minRow, maxRow
}

func activeOverlayHeight() int32 {
	data := parsePriceOverlayView(getCurrentPriceText())
	return calculateRequiredHeight(data, OverlayMode.Load())
}

func placeOverlayByTooltipRect(tooltipRect RECT) RECT {
	screen := virtualScreenRect()
	tooltipHeight := tooltipRect.Bottom - tooltipRect.Top
	if tooltipHeight <= 0 {
		tooltipHeight = TooltipOverlayReferenceHeight
	}

	localX := tooltipRect.Left
	localY := tooltipRect.Top
	if clientOrigin, ok := gameClientScreenOrigin(); ok {
		localX = tooltipRect.Left - clientOrigin.X
		localY = tooltipRect.Top - clientOrigin.Y
	}
	placement := overlayPlacementForTooltip(localY, tooltipHeight)
	width := clampInt32(placement.PanelWidth, TooltipOverlayMinWidth, TooltipOverlayMaxWidth)
	anchorOffsetX := findClosestXOffset(-localX)
	anchorOffsetY := placement.OffsetY

	overlayHeight := activeOverlayHeight()

	left := tooltipRect.Left + anchorOffsetX
	top := tooltipRect.Bottom + anchorOffsetY

	right := left + width
	if right > screen.Right {
		right = screen.Right
		left = right - width
	}
	if left < screen.Left {
		left = screen.Left
		right = left + width
	}

	bottom := top + overlayHeight
	if bottom > screen.Bottom {
		bottom = tooltipRect.Top - anchorOffsetY
		top = bottom - overlayHeight
	}

	return RECT{
		Left:   clampInt32(left, screen.Left, screen.Right),
		Top:    clampInt32(top, screen.Top, screen.Bottom),
		Right:  clampInt32(right, screen.Left, screen.Right),
		Bottom: clampInt32(bottom, screen.Top, screen.Bottom),
	}
}

func scaleByReference(value int32, referenceValue int32, referenceBase int32) int32 {
	if referenceBase <= 0 {
		return referenceValue
	}
	return int32((int64(value)*int64(referenceValue) + int64(referenceBase)/2) / int64(referenceBase))
}

func overlayPlacementForTooltip(localY int32, height int32) OverlayPlacementCalibration {
	GameLayoutMu.RLock()
	calibrations := ActiveGameLayout.PlacementCalibrations
	GameLayoutMu.RUnlock()
	bestIndex := -1
	bestHeightScore := int32(0)
	bestYScore := int32(0)
	for index, calibration := range calibrations {
		heightScore := absInt32(height - calibration.TooltipHeight)
		yScore := absInt32(localY - calibration.TooltipY)
		if bestIndex < 0 || heightScore < bestHeightScore || (heightScore == bestHeightScore && yScore < bestYScore) {
			bestIndex = index
			bestHeightScore = heightScore
			bestYScore = yScore
		}
	}
	if bestIndex >= 0 && bestHeightScore <= 12 {
		return calibrations[bestIndex]
	}

	return OverlayPlacementCalibration{
		TooltipY:      localY,
		TooltipHeight: height,
		PanelWidth:    TooltipOverlayReferencePanelWidth,
		OffsetX:       TooltipOverlayAnchorOffsetX,
		OffsetY:       scaleByReference(height, TooltipOverlayAnchorOffsetY, TooltipOverlayReferenceHeight),
	}
}

func findClosestXOffset(localX int32) int32 {
	GameLayoutMu.RLock()
	calibrations := ActiveGameLayout.XCalibrations
	GameLayoutMu.RUnlock()

	if len(calibrations) == 0 {
		return 0
	}

	bestIndex := 0
	bestDiff := absInt32(localX - roundFloat32ToInt32(calibrations[0].X))
	for index := 1; index < len(calibrations); index++ {
		diff := absInt32(localX - roundFloat32ToInt32(calibrations[index].X))
		if diff < bestDiff {
			bestDiff = diff
			bestIndex = index
		}
	}
	return calibrations[bestIndex].Offset
}

func roundFloat32ToInt32(val float32) int32 {
	if val >= 0 {
		return int32(val + 0.5)
	}
	return int32(val - 0.5)
}

func absInt32(value int32) int32 {
	if value < 0 {
		return -value
	}
	return value
}

func fallbackOverlayRect(cursor POINT) RECT {
	screen := virtualScreenRect()
	overlayHeight := activeOverlayHeight()
	left := cursor.X + TooltipOverlayOffsetX
	top := cursor.Y + TooltipOverlayOffsetY
	right := left + TooltipOverlayWidth
	bottom := top + overlayHeight

	if right > screen.Right {
		left = cursor.X - TooltipOverlayWidth - TooltipOverlayOffsetX
		right = left + TooltipOverlayWidth
	}
	if left < screen.Left {
		left = screen.Left
		right = left + TooltipOverlayWidth
	}
	if top < screen.Top {
		top = cursor.Y + 24
		bottom = top + overlayHeight
	}
	if bottom > screen.Bottom {
		bottom = cursor.Y - 12
		top = bottom - overlayHeight
	}
	if top < screen.Top {
		top = screen.Top
		bottom = top + overlayHeight
	}

	return RECT{
		Left:   clampInt32(left, screen.Left, screen.Right),
		Top:    clampInt32(top, screen.Top, screen.Bottom),
		Right:  clampInt32(right, screen.Left, screen.Right),
		Bottom: clampInt32(bottom, screen.Top, screen.Bottom),
	}
}

func isTooltipPanelPixel(color uint32) bool {
	r := int(color & 0xFF)
	g := int((color >> 8) & 0xFF)
	b := int((color >> 16) & 0xFF)

	if r <= 42 && g <= 42 && b <= 42 {
		return true
	}
	if r >= 80 && g >= 55 && b <= 125 && r >= g && r-g <= 95 {
		return true
	}
	return false
}

func distancePointToRect(point POINT, rect RECT) int {
	dx := 0
	if point.X < rect.Left {
		dx = int(rect.Left - point.X)
	} else if point.X > rect.Right {
		dx = int(point.X - rect.Right)
	}

	dy := 0
	if point.Y < rect.Top {
		dy = int(rect.Top - point.Y)
	} else if point.Y > rect.Bottom {
		dy = int(point.Y - rect.Bottom)
	}

	return dx + dy
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
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
