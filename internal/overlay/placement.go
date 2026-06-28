package overlay

import (
	"math"
	"sort"

	"github.com/nidea1/task-bar-trade-center/internal/win32"
)

func PlaceByTooltipRect(
	tooltipRect win32.RECT,
	screen win32.RECT,
	clientOrigin win32.POINT,
	hasClientOrigin bool,
	activeHeight int32,
	calibrations []PlacementCalibration,
	xCalibrations []XCalibration,
	cfg PlacementConfig,
) win32.RECT {
	return placeByTooltipRect(
		tooltipRect,
		screen,
		clientOrigin,
		hasClientOrigin,
		activeHeight,
		calibrations,
		xCalibrations,
		nil,
		nil,
		0,
		0,
		0,
		cfg,
	)
}

func PlaceByTooltipRectWithPosition(
	tooltipRect win32.RECT,
	screen win32.RECT,
	clientOrigin win32.POINT,
	hasClientOrigin bool,
	activeHeight int32,
	calibrations []PlacementCalibration,
	xCalibrations []XCalibration,
	scaleCalibrations []ScaleCalibrationProfile,
	legacyPositionCalibrations []PositionCalibration,
	memoryX float32,
	memoryY float32,
	scalePercent int32,
	cfg PlacementConfig,
) win32.RECT {
	return placeByTooltipRect(
		tooltipRect,
		screen,
		clientOrigin,
		hasClientOrigin,
		activeHeight,
		calibrations,
		xCalibrations,
		scaleCalibrations,
		legacyPositionCalibrations,
		memoryX,
		memoryY,
		scalePercent,
		cfg,
	)
}

func placeByTooltipRect(
	tooltipRect win32.RECT,
	screen win32.RECT,
	clientOrigin win32.POINT,
	hasClientOrigin bool,
	activeHeight int32,
	calibrations []PlacementCalibration,
	xCalibrations []XCalibration,
	scaleCalibrations []ScaleCalibrationProfile,
	legacyPositionCalibrations []PositionCalibration,
	memoryX float32,
	memoryY float32,
	scalePercent int32,
	cfg PlacementConfig,
) win32.RECT {
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

	// New compact profile format has priority. It uses the selected scale,
	// a profile-level Y offset and interpolated X anchors.
	if matchedOffset, ok := ResolveScalePositionOffset(
		memoryX,
		scalePercent,
		scaleCalibrations,
	); ok {
		anchorOffsetX = matchedOffset.XOffset
		anchorOffsetY = matchedOffset.YOffset
	} else if matchedOffset, ok := FindClosestPositionOffsetForScale(
		memoryX,
		memoryY,
		scalePercent,
		legacyPositionCalibrations,
	); ok {
		// Transitional fallback for old cached/remote position_calibrations.
		anchorOffsetX = matchedOffset.XOffset
		anchorOffsetY = matchedOffset.YOffset
	}

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

// ResolveScalePositionOffset finds the profile for the user-selected scale and
// resolves X with piecewise-linear interpolation. Tooltip Y is intentionally
// not an input because YOffset is constant within a profile.
func ResolveScalePositionOffset(
	x float32,
	scalePercent int32,
	profiles []ScaleCalibrationProfile,
) (PositionOffset, bool) {
	profile, ok := FindScaleCalibrationProfile(scalePercent, profiles)
	if !ok {
		return PositionOffset{}, false
	}

	xOffset, ok := InterpolateXOffset(x, profile.XAnchors)
	if !ok {
		return PositionOffset{}, false
	}

	return PositionOffset{
		XOffset:      xOffset,
		YOffset:      profile.YOffset,
		ScalePercent: profile.ScalePercent,
	}, true
}

func FindScaleCalibrationProfile(
	scalePercent int32,
	profiles []ScaleCalibrationProfile,
) (ScaleCalibrationProfile, bool) {
	for _, profile := range profiles {
		if profile.ScalePercent == scalePercent {
			return profile, true
		}
	}
	return ScaleCalibrationProfile{}, false
}

// InterpolateXOffset resolves an exact anchor or linearly interpolates between
// the nearest left/right anchors. Values outside the calibrated range are
// clamped to the closest endpoint instead of being extrapolated.
func InterpolateXOffset(x float32, anchors []XCalibrationAnchor) (int32, bool) {
	if len(anchors) == 0 {
		return 0, false
	}
	if len(anchors) == 1 {
		return anchors[0].Offset, true
	}

	index := sort.Search(len(anchors), func(index int) bool {
		return anchors[index].X >= x
	})

	if index == 0 {
		return anchors[0].Offset, true
	}
	if index >= len(anchors) {
		return anchors[len(anchors)-1].Offset, true
	}

	left := anchors[index-1]
	right := anchors[index]
	if right.X == x {
		return right.Offset, true
	}

	distance := right.X - left.X
	if math.Abs(float64(distance)) < 0.000001 {
		return left.Offset, true
	}

	ratio := (x - left.X) / distance
	offset := float64(left.Offset) +
		float64(ratio)*float64(right.Offset-left.Offset)
	return int32(math.Round(offset)), true
}

// FindClosestPositionOffset chooses the calibration point with the smallest
// two-dimensional squared distance. This is retained for legacy layouts.
func FindClosestPositionOffset(x float32, y float32, calibrations []PositionCalibration) (PositionOffset, bool) {
	if len(calibrations) == 0 {
		return PositionOffset{}, false
	}

	bestIndex := 0
	bestScore := positionDistanceSquared(x, y, calibrations[0])
	for index := 1; index < len(calibrations); index++ {
		score := positionDistanceSquared(x, y, calibrations[index])
		if score < bestScore {
			bestIndex = index
			bestScore = score
		}
	}

	best := calibrations[bestIndex]
	return PositionOffset{
		XOffset:      best.XOffset,
		YOffset:      best.YOffset,
		ScalePercent: calibrationScalePercent(best.Scale),
	}, true
}

func positionDistanceSquared(x float32, y float32, calibration PositionCalibration) float64 {
	dx := float64(x - calibration.X)
	dy := float64(y - calibration.Y)
	return dx*dx + dy*dy
}

// FindClosestPositionOffsetForScale only considers legacy entries for the
// selected game scale. It never falls through to a different scale.
func FindClosestPositionOffsetForScale(
	x float32,
	y float32,
	scalePercent int32,
	calibrations []PositionCalibration,
) (PositionOffset, bool) {
	bestIndex := -1
	bestScore := float64(0)

	for index, calibration := range calibrations {
		if calibrationScalePercent(calibration.Scale) != scalePercent {
			continue
		}

		score := positionDistanceSquared(x, y, calibration)
		if bestIndex < 0 || score < bestScore {
			bestIndex = index
			bestScore = score
		}
	}

	if bestIndex < 0 {
		return PositionOffset{}, false
	}

	best := calibrations[bestIndex]
	return PositionOffset{
		XOffset:      best.XOffset,
		YOffset:      best.YOffset,
		ScalePercent: calibrationScalePercent(best.Scale),
	}, true
}

func calibrationScalePercent(scale float32) int32 {
	if scale <= 0 {
		return 0
	}
	return RoundFloat32ToInt32(scale * 100)
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
