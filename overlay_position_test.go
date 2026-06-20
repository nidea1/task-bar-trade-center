package main

import "testing"

func TestMarketOverlayRectFallsBackToCursorWhenTooltipMemoryIsUnavailable(t *testing.T) {
	originalProcessHandle := GameProcessHandle
	originalGameAssemblyBase := GameAssemblyBase
	originalShowOverlay := ShowOverlay.Load()
	originalLastOverlayRect := LastOverlayRect
	originalHasLastOverlayRect := HasLastOverlayRect
	t.Cleanup(func() {
		GameProcessHandle = originalProcessHandle
		GameAssemblyBase = originalGameAssemblyBase
		ShowOverlay.Store(originalShowOverlay)
		LastOverlayRect = originalLastOverlayRect
		HasLastOverlayRect = originalHasLastOverlayRect
	})

	GameProcessHandle = 0
	GameAssemblyBase = 0
	ShowOverlay.Store(true)
	HasLastOverlayRect = false

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
