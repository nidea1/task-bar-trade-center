package app

import (
	"fmt"
	"sync/atomic"
)

const (
	GameScale100 int32 = 100
	GameScale125 int32 = 125
	GameScale150 int32 = 150

	MenuGameScaleBase uint32 = 1400
)

type gameScaleOption struct {
	Percent int32
	Label   string
}

var (
	supportedGameScales = []gameScaleOption{
		{Percent: GameScale100, Label: "1x"},
		{Percent: GameScale125, Label: "1.25x"},
		{Percent: GameScale150, Label: "1.5x"},
	}

	selectedGameScale atomic.Int32
)

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
	scale = normalizeGameScale(scale)
	for _, option := range supportedGameScales {
		if option.Percent == scale {
			return option.Label
		}
	}
	return "1x"
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

func gameScaleForMenuCommand(commandID uint32) (int32, bool) {
	if commandID < MenuGameScaleBase {
		return 0, false
	}

	index := int(commandID - MenuGameScaleBase)
	if index < 0 || index >= len(supportedGameScales) {
		return 0, false
	}

	return supportedGameScales[index].Percent, true
}
