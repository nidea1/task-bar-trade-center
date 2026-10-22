package app

import (
	"fmt"
	"sync/atomic"
)

const (
	GameScale100 int32 = 100
	GameScale125 int32 = 125
	GameScale150 int32 = 150
)

// Retained for legacy layout calibration and settings compatibility. Native
// RectTransform placement obtains scale directly from the game's hierarchy.
var selectedGameScale atomic.Int32

func init() {
	selectedGameScale.Store(GameScale100)
}

func normalizeGameScale(scale int32) int32 {
	switch scale {
	case GameScale100, GameScale125, GameScale150:
		return scale
	default:
		return GameScale100
	}
}

func currentGameScale() int32 {
	return normalizeGameScale(selectedGameScale.Load())
}

func currentGameScaleFactor() float32 {
	return float32(currentGameScale()) / 100
}

func gameScaleLabel(scale int32) string {
	return fmt.Sprintf("%gx", float32(normalizeGameScale(scale))/100)
}

func selectGameScale(scale int32) {
	scale = normalizeGameScale(scale)
	previous := currentGameScale()
	if previous == scale {
		return
	}

	selectedGameScale.Store(scale)
	activeApp.hasLastOverlayRect = false
	saveSettingsToDisk()

	fmt.Printf("Game scale changed to %s.\n", gameScaleLabel(scale))

	if activeApp.showOverlay.Load() {
		redrawOverlay()
	}
}
