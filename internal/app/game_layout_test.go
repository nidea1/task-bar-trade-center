package app

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/game"
)

func TestLoadLocalGameLayoutUsesCacheWithoutRemoteRequest(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "game-layout-cache.json")
	if err := os.WriteFile(cachePath, embeddedGameLayoutJSON, 0600); err != nil {
		t.Fatalf("write cache: %v", err)
	}
	serverCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		serverCalls++
	}))
	defer server.Close()

	oldCachePath := GameLayoutCacheFilePath
	oldURL := gameLayoutURL
	oldLayout := ActiveGameLayout
	oldSource := GameLayoutSource
	oldEnv := os.Getenv(game.LayoutPathEnvironment)
	t.Cleanup(func() {
		GameLayoutCacheFilePath = oldCachePath
		gameLayoutURL = oldURL
		ActiveGameLayout = oldLayout
		GameLayoutSource = oldSource
		_ = os.Setenv(game.LayoutPathEnvironment, oldEnv)
	})
	GameLayoutCacheFilePath = cachePath
	gameLayoutURL = server.URL
	_ = os.Unsetenv(game.LayoutPathEnvironment)

	if err := loadLocalGameLayout(); err != nil {
		t.Fatalf("loadLocalGameLayout returned error: %v", err)
	}
	if GameLayoutSource != game.LayoutSourceCache {
		t.Fatalf("source = %q, want cache", GameLayoutSource)
	}
	if serverCalls != 0 {
		t.Fatalf("local load made %d remote requests", serverCalls)
	}
}

func TestLoadGameLayoutPrefersLocalDevelopmentFile(t *testing.T) {
	layoutPath := filepath.Join(t.TempDir(), "game-layout.json")
	localLayout := []byte(strings.Replace(string(embeddedGameLayoutJSON), "0x05DF8198", "0x00000020", 1))
	if err := os.WriteFile(layoutPath, localLayout, 0600); err != nil {
		t.Fatalf("write layout: %v", err)
	}
	t.Setenv(game.LayoutPathEnvironment, layoutPath)

	previousLayout := ActiveGameLayout
	previousSource := GameLayoutSource
	t.Cleanup(func() {
		ActiveGameLayout = previousLayout
		GameLayoutSource = previousSource
	})

	if err := loadGameLayout(); err != nil {
		t.Fatalf("loadGameLayout returned error: %v", err)
	}
	if GameLayoutSource != game.LayoutSourceLocalDevelopment {
		t.Fatalf("source = %q, want %q", GameLayoutSource, game.LayoutSourceLocalDevelopment)
	}
	if ActiveGameLayout.HoveredItemPointerBaseOffset != 0x20 {
		t.Fatalf("hovered pointer base = 0x%X, want 0x20", ActiveGameLayout.HoveredItemPointerBaseOffset)
	}
}

func TestOverlayPlacementMatchesCalibrationWhenTooltipYChanges(t *testing.T) {
	want := OverlayPlacementCalibration{TooltipY: 173, TooltipHeight: 348, PanelWidth: 200, OffsetY: 116}
	previousLayout := ActiveGameLayout
	ActiveGameLayout = GameLayout{PlacementCalibrations: []OverlayPlacementCalibration{want}}
	t.Cleanup(func() { ActiveGameLayout = previousLayout })
	if got := overlayPlacementForTooltip(681, 348); got != want {
		t.Fatalf("placement = %+v, want %+v", got, want)
	}
}

func TestOverlayPlacementUsesFixedTooltipWidth(t *testing.T) {
	want := OverlayPlacementCalibration{TooltipY: 199, TooltipHeight: 398, PanelWidth: 200, OffsetY: 66}
	previousLayout := ActiveGameLayout
	ActiveGameLayout = GameLayout{PlacementCalibrations: []OverlayPlacementCalibration{want}}
	t.Cleanup(func() { ActiveGameLayout = previousLayout })

	if got := overlayPlacementForTooltip(want.TooltipY, want.TooltipHeight); got != want {
		t.Fatalf("placement = %+v, want %+v", got, want)
	}
}

func TestPointerReadWarningIsShownOnlyOncePerSession(t *testing.T) {
	GameLayoutReadHealth.Reset()
	originalStatus := AppStatus.Load()
	originalShowOverlay := ShowOverlay.Load()
	originalErrorMessageBoxMock := showErrorMessageBoxMock
	t.Cleanup(func() {
		GameLayoutReadHealth.Reset()
		AppStatus.Store(originalStatus)
		ShowOverlay.Store(originalShowOverlay)
		showErrorMessageBoxMock = originalErrorMessageBoxMock
	})

	messageCount := 0
	showErrorMessageBoxMock = func(title, message string) {
		messageCount++
		if title != tr("dialog.layout_incompatible.title") {
			t.Errorf("title = %q", title)
		}
		if !strings.Contains(message, "Diagnostic log:") {
			t.Errorf("message did not explain how to recover: %q", message)
		}
	}

	ShowOverlay.Store(true)
	AppStatus.Store(AppStatusReady)
	start := time.Unix(1_700_000_000, 0)
	recordPointerReadResultAt(start, game.PointerReadHoveredItem, false)
	recordPointerReadResultAt(start.Add(3*time.Second), game.PointerReadHoveredItem, false)
	if messageCount != 1 {
		t.Fatalf("message count = %d, want 1", messageCount)
	}
	if ShowOverlay.Load() {
		t.Fatal("overlay remained visible after sustained pointer failure")
	}
	if AppStatus.Load() != AppStatusGameLayoutIncompatible {
		t.Fatalf("status = %d, want layout incompatible", AppStatus.Load())
	}

	recordPointerReadResultAt(start.Add(4*time.Second), game.PointerReadHoveredItem, true)
	if AppStatus.Load() != AppStatusReady {
		t.Fatalf("status = %d, want ready after successful read", AppStatus.Load())
	}

	ShowOverlay.Store(true)
	recordPointerReadResultAt(start.Add(5*time.Second), game.PointerReadHoveredItem, false)
	recordPointerReadResultAt(start.Add(8*time.Second), game.PointerReadHoveredItem, false)
	if messageCount != 1 {
		t.Fatalf("message count after second failure = %d, want 1", messageCount)
	}
	if AppStatus.Load() != AppStatusGameLayoutIncompatible {
		t.Fatalf("status = %d, want layout incompatible after second failure", AppStatus.Load())
	}
}

func TestUpdateGameLayoutConfigs(t *testing.T) {
	remoteLayout := []byte(strings.Replace(string(embeddedGameLayoutJSON), "0x05DF8198", "0x00000030", 1))
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !strings.Contains(request.URL.RawQuery, "nocache=") {
			t.Errorf("expected query parameter 'nocache', got query %q", request.URL.RawQuery)
		}
		_, _ = writer.Write(remoteLayout)
	}))
	defer server.Close()

	oldURL := gameLayoutURL
	gameLayoutURL = server.URL
	previousLayout := ActiveGameLayout
	previousSource := GameLayoutSource

	t.Cleanup(func() {
		gameLayoutURL = oldURL
		ActiveGameLayout = previousLayout
		GameLayoutSource = previousSource
	})

	GameLayoutReadHealth.SetIncompatibleForTest(true)
	updateGameLayoutConfigs()

	if ConfigurationStatus.Load() != ConfigStatusCurrent {
		t.Errorf("configuration status = %d, want current", ConfigurationStatus.Load())
	}

	GameLayoutMu.RLock()
	currentOffset := ActiveGameLayout.HoveredItemPointerBaseOffset
	GameLayoutMu.RUnlock()

	if currentOffset != 0x30 {
		t.Errorf("ActiveGameLayout offset = 0x%X, want 0x30", currentOffset)
	}
	if GameLayoutReadHealth.IncompatibleForTest() {
		t.Error("expected incompatibility state to be reset")
	}
}

func TestScanOffsets(t *testing.T) {
	pid := findProcessID(GameProcessName)
	if pid == 0 {
		t.Log("TaskBarHero.exe is not running, skipping inspection")
		return
	}
	pHandle, ok := openGameProcess(pid)
	if !ok {
		t.Fatalf("Could not open game process")
	}
	defer procCloseHandle.Call(pHandle)

	gameAssemblyBase := getModuleBaseAddress(pHandle, "GameAssembly.dll")
	if gameAssemblyBase == 0 {
		t.Fatalf("Could not find GameAssembly.dll base address")
	}

	_ = loadItemsJSON()
	_ = loadGameLayout()

	GameLayoutMu.RLock()
	layout := ActiveGameLayout
	GameLayoutMu.RUnlock()

	var resolver game.HoveredItemAOBResolver
	t.Log("Starting 30-second deep scan. Please hover over a marketable item in the game...")

	for i := 0; i < 60; i++ {
		itemID, readMode, rawValue, ok := resolver.Read(pHandle, gameAssemblyBase, layout, marketableItemExists)
		t.Logf("scan=%d itemID=%d readMode=%s raw=%d ok=%t", i, itemID, readMode, rawValue, ok)
		time.Sleep(500 * time.Millisecond)
	}
	t.Log("Scan finished.")
}
