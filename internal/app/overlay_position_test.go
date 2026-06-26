package app

import (
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
