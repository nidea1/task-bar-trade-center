package overlay

import "github.com/nidea1/task-bar-trade-center/internal/win32"

func PlaceByTooltipRect(tooltipRect win32.RECT, screen win32.RECT, clientOrigin win32.POINT, hasClientOrigin bool, activeHeight int32, calibrations []PlacementCalibration, xCalibrations []XCalibration, cfg PlacementConfig) win32.RECT {
	tooltipHeight := tooltipRect.Bottom - tooltipRect.Top
	if tooltipHeight <= 0 {
		tooltipHeight = cfg.ReferenceHeight
	}

	localX := tooltipRect.Left
	localY := tooltipRect.Top
	if hasClientOrigin {
		localX = tooltipRect.Left - clientOrigin.X
		localY = tooltipRect.Top - clientOrigin.Y
	}
	placement := PlacementForTooltip(localY, tooltipHeight, calibrations, cfg)
	width := ClampInt32(placement.PanelWidth, cfg.MinWidth, cfg.MaxWidth)
	anchorOffsetX := FindClosestXOffset(-localX, xCalibrations)
	anchorOffsetY := placement.OffsetY

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

	bottom := top + activeHeight
	if bottom > screen.Bottom {
		bottom = tooltipRect.Top - anchorOffsetY
		top = bottom - activeHeight
	}

	return win32.RECT{
		Left:   ClampInt32(left, screen.Left, screen.Right),
		Top:    ClampInt32(top, screen.Top, screen.Bottom),
		Right:  ClampInt32(right, screen.Left, screen.Right),
		Bottom: ClampInt32(bottom, screen.Top, screen.Bottom),
	}
}

func PlacementForTooltip(localY int32, height int32, calibrations []PlacementCalibration, cfg PlacementConfig) PlacementCalibration {
	bestIndex := -1
	bestHeightScore := int32(0)
	bestYScore := int32(0)
	for index, calibration := range calibrations {
		heightScore := AbsInt32(height - calibration.TooltipHeight)
		yScore := AbsInt32(localY - calibration.TooltipY)
		if bestIndex < 0 || heightScore < bestHeightScore || (heightScore == bestHeightScore && yScore < bestYScore) {
			bestIndex = index
			bestHeightScore = heightScore
			bestYScore = yScore
		}
	}
	if bestIndex >= 0 && bestHeightScore <= 12 {
		return calibrations[bestIndex]
	}

	return PlacementCalibration{
		TooltipY:      localY,
		TooltipHeight: height,
		PanelWidth:    cfg.ReferencePanelWidth,
		OffsetX:       cfg.AnchorOffsetX,
		OffsetY:       ScaleByReference(height, cfg.AnchorOffsetY, cfg.ReferenceHeight),
	}
}

func FindClosestXOffset(localX int32, calibrations []XCalibration) int32 {
	if len(calibrations) == 0 {
		return 0
	}

	bestIndex := 0
	bestDiff := AbsInt32(localX - RoundFloat32ToInt32(calibrations[0].X))
	for index := 1; index < len(calibrations); index++ {
		diff := AbsInt32(localX - RoundFloat32ToInt32(calibrations[index].X))
		if diff < bestDiff {
			bestDiff = diff
			bestIndex = index
		}
	}
	return calibrations[bestIndex].Offset
}

func FallbackRect(cursor win32.POINT, screen win32.RECT, activeHeight int32, cfg PlacementConfig) win32.RECT {
	left := cursor.X + cfg.OffsetX
	top := cursor.Y + cfg.OffsetY
	right := left + cfg.Width
	bottom := top + activeHeight

	if right > screen.Right {
		left = cursor.X - cfg.Width - cfg.OffsetX
		right = left + cfg.Width
	}
	if left < screen.Left {
		left = screen.Left
		right = left + cfg.Width
	}
	if top < screen.Top {
		top = cursor.Y + 24
		bottom = top + activeHeight
	}
	if bottom > screen.Bottom {
		bottom = cursor.Y - 12
		top = bottom - activeHeight
	}
	if top < screen.Top {
		top = screen.Top
		bottom = top + activeHeight
	}

	return win32.RECT{
		Left:   ClampInt32(left, screen.Left, screen.Right),
		Top:    ClampInt32(top, screen.Top, screen.Bottom),
		Right:  ClampInt32(right, screen.Left, screen.Right),
		Bottom: ClampInt32(bottom, screen.Top, screen.Bottom),
	}
}

func ScaleByReference(value int32, referenceValue int32, referenceBase int32) int32 {
	if referenceBase <= 0 {
		return referenceValue
	}
	return int32((int64(value)*int64(referenceValue) + int64(referenceBase)/2) / int64(referenceBase))
}

func RoundFloat32ToInt32(val float32) int32 {
	if val >= 0 {
		return int32(val + 0.5)
	}
	return int32(val - 0.5)
}

func AbsInt32(value int32) int32 {
	if value < 0 {
		return -value
	}
	return value
}

func IsTooltipPanelPixel(color uint32) bool {
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

func DistancePointToRect(point win32.POINT, rect win32.RECT) int {
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

func ClampInt32(value int32, min int32, max int32) int32 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func AbsInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
