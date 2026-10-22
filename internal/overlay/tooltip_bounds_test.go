package overlay

import (
	"github.com/nidea1/task-bar-trade-center/internal/win32"
	"testing"
)

func TestPlaceBelowTooltipTracksActualBottom(t *testing.T) {
	screen := win32.RECT{Left: -1920, Top: -100, Right: 1920, Bottom: 1200}
	cfg := PlacementConfig{MinWidth: 244, MaxWidth: 420}
	for _, tooltip := range []win32.RECT{
		{Left: 61, Top: 200, Right: 289, Bottom: 400},
		{Left: 61, Top: 200, Right: 289, Bottom: 750},
		{Left: -1400, Top: -50, Right: -1058, Bottom: 500},
	} {
		hud := PlaceBelowTooltip(tooltip, screen, 332, cfg)
		if hud.Top != tooltip.Bottom || hud.Bottom-hud.Top != 332 ||
			hud.Left+hud.Right != tooltip.Left+tooltip.Right {
			t.Errorf("tooltip=%+v: HUD=%+v", tooltip, hud)
		}
	}
}

func TestPlaceBelowTooltipUsesAboveWhenBottomIsFull(t *testing.T) {
	screen := win32.RECT{Right: 1920, Bottom: 1080}
	tooltip := win32.RECT{Left: 100, Top: 600, Right: 328, Bottom: 1000}
	hud := PlaceBelowTooltip(tooltip, screen, 332, PlacementConfig{MinWidth: 244, MaxWidth: 420})
	if hud.Bottom != tooltip.Top || hud.Top < screen.Top {
		t.Fatalf("HUD should be above tooltip: %+v", hud)
	}
}

func TestPlaceBelowTooltipStaysWithinScreenAtEdges(t *testing.T) {
	screen := win32.RECT{Left: -1280, Top: -100, Right: 0, Bottom: 900}
	for _, tooltip := range []win32.RECT{
		{Left: -1280, Top: 0, Right: -1052, Bottom: 200},
		{Left: -228, Top: 0, Right: 0, Bottom: 200},
	} {
		hud := PlaceBelowTooltip(tooltip, screen, 332, PlacementConfig{MinWidth: 244, MaxWidth: 420})
		if hud.Left < screen.Left || hud.Right > screen.Right || hud.Top < screen.Top || hud.Bottom > screen.Bottom {
			t.Errorf("HUD outside screen: %+v", hud)
		}
	}
}
