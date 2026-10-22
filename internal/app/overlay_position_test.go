package app

import (
	"github.com/nidea1/task-bar-trade-center/internal/overlay"
	"github.com/nidea1/task-bar-trade-center/internal/win32"
)

import "testing"

func TestMarketOverlayRectFallsBackToCursorWhenTooltipMemoryIsUnavailable(t *testing.T) {
	originalProcessHandle := activeApp.gameProcessHandle
	originalGameAssemblyBase := activeApp.gameAssemblyBase
	originalShowOverlay := activeApp.showOverlay.Load()
	originalLastOverlayRect := activeApp.lastOverlayRect
	originalHasLastOverlayRect := activeApp.hasLastOverlayRect
	originalCursorScreenPosition := cursorScreenPosition
	t.Cleanup(func() {
		activeApp.gameProcessHandle = originalProcessHandle
		activeApp.gameAssemblyBase = originalGameAssemblyBase
		activeApp.showOverlay.Store(originalShowOverlay)
		activeApp.lastOverlayRect = originalLastOverlayRect
		activeApp.hasLastOverlayRect = originalHasLastOverlayRect
		cursorScreenPosition = originalCursorScreenPosition
	})

	activeApp.gameProcessHandle = 0
	activeApp.gameAssemblyBase = 0
	activeApp.showOverlay.Store(true)
	activeApp.hasLastOverlayRect = false
	cursorScreenPosition = func() (win32.POINT, bool) {
		return win32.POINT{X: 100, Y: 100}, true
	}

	rect, ok := marketOverlayRect()
	if !ok {
		t.Fatal("marketOverlayRect did not fall back to the cursor")
	}

	screen := virtualScreenRect()
	if rect.Left < screen.Left || rect.Top < screen.Top || rect.Right > screen.Right || rect.Bottom > screen.Bottom {
		t.Fatalf("fallback rect (%d,%d,%d,%d) is outside screen (%d,%d,%d,%d)", rect.Left, rect.Top, rect.Right, rect.Bottom, screen.Left, screen.Top, screen.Right, screen.Bottom)
	}
	if rect.Right <= rect.Left || rect.Bottom <= rect.Top {
		t.Fatalf("fallback rect has invalid dimensions: (%d,%d,%d,%d)", rect.Left, rect.Top, rect.Right, rect.Bottom)
	}
}

func TestTooltipScreenBoundsBypassLegacyCalibration(t *testing.T) {
	previous := activeApp.activeGameLayout
	previousScale := selectedGameScale.Load()
	t.Cleanup(func() {
		activeApp.activeGameLayout = previous
		selectedGameScale.Store(previousScale)
	})
	screen := virtualScreenRect()
	snapshot := tooltipMemorySnapshot{ScreenBounds: true, Rect: win32.RECT{
		Left: screen.Left + 100, Top: screen.Top + 20, Right: screen.Left + 328, Bottom: screen.Top + 220,
	}}
	for _, scale := range []int32{GameScale100, GameScale125, GameScale150} {
		selectedGameScale.Store(scale)
		// Saved manual scales and incompatible legacy offsets must not affect actual bounds.
		activeApp.activeGameLayout.ScaleCalibrations = []overlay.ScaleCalibrationProfile{{
			ScalePercent: scale, YOffset: 1000,
			XAnchors: []overlay.XCalibrationAnchor{{X: -310, Offset: 1000}, {X: 230, Offset: 1000}},
		}}
		hud := placeOverlayByTooltipSnapshot(snapshot)
		if hud.Top != snapshot.Rect.Bottom || hud.Left+hud.Right != snapshot.Rect.Left+snapshot.Rect.Right {
			t.Fatalf("manual scale %d affected actual bounds: tooltip=%+v HUD=%+v", scale, snapshot.Rect, hud)
		}
	}
}
