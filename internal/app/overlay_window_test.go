package app

import "testing"

func TestOverlayDestroyOnlyClearsOverlayState(t *testing.T) {
	original := activeApp
	activeApp = &App{}
	defer func() {
		activeApp = original
	}()

	const hwnd uintptr = 12345
	activeApp.overlayHWND = hwnd
	activeApp.overlayUpdatePending.Store(true)
	activeApp.hasOverlayWindowRect = true
	activeApp.overlayWindowVisible = true

	if got := wndProc(hwnd, WM_DESTROY, 0, 0); got != 0 {
		t.Fatalf("wndProc WM_DESTROY returned %d, want 0", got)
	}
	if activeApp.overlayHWND != 0 {
		t.Fatalf("overlayHWND = %d, want 0", activeApp.overlayHWND)
	}
	if activeApp.overlayUpdatePending.Load() {
		t.Fatal("overlayUpdatePending remained set")
	}
	if activeApp.hasOverlayWindowRect {
		t.Fatal("hasOverlayWindowRect remained set")
	}
	if activeApp.overlayWindowVisible {
		t.Fatal("overlayWindowVisible remained set")
	}
}
