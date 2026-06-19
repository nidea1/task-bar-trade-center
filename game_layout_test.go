package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolveGameLayoutUsesRemoteAndCachesIt(t *testing.T) {
	remoteLayout := []byte(strings.Replace(string(embeddedGameLayoutJSON), "0x05DCFD70", "0x00000020", 1))
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("User-Agent"); got != userAgent {
			t.Errorf("User-Agent = %q, want %q", got, userAgent)
		}
		_, _ = writer.Write(remoteLayout)
	}))
	defer server.Close()

	cacheFilePath := filepath.Join(t.TempDir(), "game-layout-cache.json")
	if err := os.WriteFile(cacheFilePath, []byte(`stale layout`), 0600); err != nil {
		t.Fatalf("write stale cache: %v", err)
	}
	layout, source, err := resolveGameLayout(server.URL, cacheFilePath, server.Client(), embeddedGameLayoutJSON)
	if err != nil {
		t.Fatalf("resolveGameLayout returned error: %v", err)
	}
	if source != gameLayoutSourceRemote {
		t.Fatalf("source = %q, want %q", source, gameLayoutSourceRemote)
	}
	if layout.HoveredItemPointerBaseOffset != 0x20 {
		t.Fatalf("hovered pointer base = 0x%X, want 0x20", layout.HoveredItemPointerBaseOffset)
	}

	cached, err := os.ReadFile(cacheFilePath)
	if err != nil {
		t.Fatalf("read cache: %v", err)
	}
	if !bytes.Equal(cached, remoteLayout) {
		t.Fatal("cache did not contain the validated remote layout")
	}
}

func TestLoadGameLayoutFromFile(t *testing.T) {
	layoutPath := filepath.Join(t.TempDir(), "game-layout.json")
	if err := os.WriteFile(layoutPath, embeddedGameLayoutJSON, 0600); err != nil {
		t.Fatalf("write layout: %v", err)
	}

	layout, err := loadGameLayoutFromFile(layoutPath)
	if err != nil {
		t.Fatalf("loadGameLayoutFromFile returned error: %v", err)
	}
	if layout.HoveredItemPointerBaseOffset != 0x05DCFD70 {
		t.Fatalf("hovered pointer base = 0x%X, want 0x05DCFD70", layout.HoveredItemPointerBaseOffset)
	}
}

func TestLoadGameLayoutFromFileRejectsInvalidLayout(t *testing.T) {
	layoutPath := filepath.Join(t.TempDir(), "game-layout.json")
	if err := os.WriteFile(layoutPath, []byte(`not json`), 0600); err != nil {
		t.Fatalf("write layout: %v", err)
	}

	if _, err := loadGameLayoutFromFile(layoutPath); err == nil {
		t.Fatal("loadGameLayoutFromFile succeeded for invalid JSON")
	}
}

func TestLoadGameLayoutPrefersLocalDevelopmentFile(t *testing.T) {
	layoutPath := filepath.Join(t.TempDir(), "game-layout.json")
	localLayout := []byte(strings.Replace(string(embeddedGameLayoutJSON), "0x05DCFD70", "0x00000020", 1))
	if err := os.WriteFile(layoutPath, localLayout, 0600); err != nil {
		t.Fatalf("write layout: %v", err)
	}
	t.Setenv(gameLayoutPathEnvironment, layoutPath)

	previousLayout := ActiveGameLayout
	previousSource := GameLayoutSource
	t.Cleanup(func() {
		ActiveGameLayout = previousLayout
		GameLayoutSource = previousSource
	})

	if err := loadGameLayout(); err != nil {
		t.Fatalf("loadGameLayout returned error: %v", err)
	}
	if GameLayoutSource != gameLayoutSourceLocalDevelopment {
		t.Fatalf("source = %q, want %q", GameLayoutSource, gameLayoutSourceLocalDevelopment)
	}
	if ActiveGameLayout.HoveredItemPointerBaseOffset != 0x20 {
		t.Fatalf("hovered pointer base = 0x%X, want 0x20", ActiveGameLayout.HoveredItemPointerBaseOffset)
	}
}

func TestResolveGameLayoutUsesCacheWhenRemoteIsUnavailable(t *testing.T) {
	cacheFilePath := filepath.Join(t.TempDir(), "game-layout-cache.json")
	cachedLayout := []byte(strings.Replace(string(embeddedGameLayoutJSON), "0x05DCFD70", "0x00000030", 1))
	if err := os.WriteFile(cacheFilePath, cachedLayout, 0600); err != nil {
		t.Fatalf("write cache: %v", err)
	}

	server := httptest.NewServer(http.NotFoundHandler())
	server.Close()
	layout, source, err := resolveGameLayout(server.URL, cacheFilePath, &http.Client{Timeout: time.Second}, embeddedGameLayoutJSON)
	if err != nil {
		t.Fatalf("resolveGameLayout returned error: %v", err)
	}
	if source != gameLayoutSourceCache {
		t.Fatalf("source = %q, want %q", source, gameLayoutSourceCache)
	}
	if layout.HoveredItemPointerBaseOffset != 0x30 {
		t.Fatalf("hovered pointer base = 0x%X, want 0x30", layout.HoveredItemPointerBaseOffset)
	}
}

func TestResolveGameLayoutUsesEmbeddedDefaultWhenRemoteAndCacheAreInvalid(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{"schema_version": 99}`))
	}))
	defer server.Close()

	cacheFilePath := filepath.Join(t.TempDir(), "game-layout-cache.json")
	if err := os.WriteFile(cacheFilePath, []byte(`not json`), 0600); err != nil {
		t.Fatalf("write invalid cache: %v", err)
	}

	layout, source, err := resolveGameLayout(server.URL, cacheFilePath, server.Client(), embeddedGameLayoutJSON)
	if err != nil {
		t.Fatalf("resolveGameLayout returned error: %v", err)
	}
	if source != gameLayoutSourceEmbeddedDefault {
		t.Fatalf("source = %q, want %q", source, gameLayoutSourceEmbeddedDefault)
	}
	if layout.HoveredItemPointerBaseOffset != 0x05DCFD70 {
		t.Fatalf("hovered pointer base = 0x%X, want 0x05DCFD70", layout.HoveredItemPointerBaseOffset)
	}

	cached, err := os.ReadFile(cacheFilePath)
	if err != nil {
		t.Fatalf("read cache: %v", err)
	}
	if string(cached) != "not json" {
		t.Fatal("invalid remote layout overwrote the existing cache")
	}
}

func TestParseGameLayoutValidatesOffsetsAndPlacementCalibrations(t *testing.T) {
	layout, err := parseGameLayout(embeddedGameLayoutJSON)
	if err != nil {
		t.Fatalf("parseGameLayout returned error: %v", err)
	}
	if layout.HoveredItemKeyOffset != 0x1A4 {
		t.Fatalf("hovered item key offset = 0x%X, want 0x1A4", layout.HoveredItemKeyOffset)
	}
	if len(layout.TooltipXPointerOffsets) != 7 || len(layout.TooltipYPointerOffsets) != 7 {
		t.Fatalf("tooltip pointer offset lengths = %d and %d, want 7", len(layout.TooltipXPointerOffsets), len(layout.TooltipYPointerOffsets))
	}
	if len(layout.PlacementCalibrations) != 8 {
		t.Fatalf("placement calibrations = %d, want 8", len(layout.PlacementCalibrations))
	}

	previousLayout := ActiveGameLayout
	ActiveGameLayout = layout
	t.Cleanup(func() { ActiveGameLayout = previousLayout })
	want := layout.PlacementCalibrations[0]
	if got := overlayPlacementForTooltip(want.TooltipY, want.TooltipHeight); got != want {
		t.Fatalf("placement = %+v, want %+v", got, want)
	}
}

func TestOverlayPlacementMatchesCalibrationWhenTooltipYChanges(t *testing.T) {
	layout, err := parseGameLayout(embeddedGameLayoutJSON)
	if err != nil {
		t.Fatalf("parseGameLayout returned error: %v", err)
	}

	previousLayout := ActiveGameLayout
	ActiveGameLayout = layout
	t.Cleanup(func() { ActiveGameLayout = previousLayout })
	want := layout.PlacementCalibrations[0]
	if got := overlayPlacementForTooltip(681, 348); got != want {
		t.Fatalf("placement = %+v, want %+v", got, want)
	}
}

func TestOverlayPlacementUsesFixedTooltipWidth(t *testing.T) {
	layout, err := parseGameLayout(embeddedGameLayoutJSON)
	if err != nil {
		t.Fatalf("parseGameLayout returned error: %v", err)
	}

	previousLayout := ActiveGameLayout
	ActiveGameLayout = layout
	t.Cleanup(func() { ActiveGameLayout = previousLayout })

	want := layout.PlacementCalibrations[1]
	if got := overlayPlacementForTooltip(want.TooltipY, want.TooltipHeight); got != want {
		t.Fatalf("placement = %+v, want %+v", got, want)
	}
}

func TestParseGameLayoutAcceptsLegacyWidthConfiguration(t *testing.T) {
	legacyLayout := strings.Replace(
		string(embeddedGameLayoutJSON),
		`"height_pointer_base_offset":`,
		`"width_pointer_base_offset": "0x05DCAEA0", "width_pointer_offsets": ["0x40"], "height_pointer_base_offset":`,
		1,
	)
	legacyLayout = strings.Replace(legacyLayout, `"tooltip_height": 348`, `"tooltip_width": 308, "tooltip_height": 348`, 1)

	if _, err := parseGameLayout([]byte(legacyLayout)); err != nil {
		t.Fatalf("parseGameLayout rejected a legacy width configuration: %v", err)
	}
}

func TestParseGameLayoutRejectsUnsupportedAndIncompleteDocuments(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
	}{
		{
			name: "unsupported schema",
			raw:  []byte(strings.Replace(string(embeddedGameLayoutJSON), `"schema_version": 2`, `"schema_version": 3`, 1)),
		},
		{
			name: "missing pointer value",
			raw:  []byte(strings.Replace(string(embeddedGameLayoutJSON), `"key_offset": "0x1A4"`, `"key_offset": ""`, 1)),
		},
		{
			name: "empty pointer chain",
			raw: []byte(strings.Replace(
				string(embeddedGameLayoutJSON),
				`"pointer_offsets": ["0x40", "0x88", "0x10", "0xB8", "0x8", "0x20", "0x338"]`,
				`"pointer_offsets": []`,
				1,
			)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseGameLayout(tt.raw); err == nil {
				t.Fatal("parseGameLayout succeeded for an invalid document")
			}
		})
	}
}

func TestPointerReadHealthRequiresContinuousFailureAndRecovers(t *testing.T) {
	var health pointerReadHealth
	start := time.Unix(1_700_000_000, 0)

	if incompatible, notified, recovered := health.record(start, pointerReadHoveredItem, false); incompatible || notified || recovered {
		t.Fatalf("first failure = incompatible:%v notified:%v recovered:%v", incompatible, notified, recovered)
	}
	if incompatible, notified, recovered := health.record(start.Add(2*time.Second), pointerReadHoveredItem, true); incompatible || notified || recovered {
		t.Fatalf("short failure recovery = incompatible:%v notified:%v recovered:%v", incompatible, notified, recovered)
	}
	if incompatible, notified, recovered := health.record(start.Add(3*time.Second), pointerReadHoveredItem, false); incompatible || notified || recovered {
		t.Fatalf("new failure = incompatible:%v notified:%v recovered:%v", incompatible, notified, recovered)
	}
	if incompatible, notified, recovered := health.record(start.Add(6*time.Second), pointerReadHoveredItem, false); !incompatible || !notified || recovered {
		t.Fatalf("continuous failure = incompatible:%v notified:%v recovered:%v", incompatible, notified, recovered)
	}
	if incompatible, notified, recovered := health.record(start.Add(7*time.Second), pointerReadHoveredItem, true); incompatible || notified || !recovered {
		t.Fatalf("successful read = incompatible:%v notified:%v recovered:%v", incompatible, notified, recovered)
	}
}

func TestPointerReadWarningIsShownOnlyOncePerSession(t *testing.T) {
	GameLayoutReadHealth.reset()
	originalStatus := AppStatus.Load()
	originalShowOverlay := ShowOverlay.Load()
	originalErrorMessageBoxMock := showErrorMessageBoxMock
	t.Cleanup(func() {
		GameLayoutReadHealth.reset()
		AppStatus.Store(originalStatus)
		ShowOverlay.Store(originalShowOverlay)
		showErrorMessageBoxMock = originalErrorMessageBoxMock
	})

	messageCount := 0
	showErrorMessageBoxMock = func(title, message string) {
		messageCount++
		if title != "Game Memory Layout Update Required" {
			t.Errorf("title = %q", title)
		}
		if !strings.Contains(message, "restart Task Bar Trade Center") {
			t.Errorf("message did not explain how to recover: %q", message)
		}
	}

	ShowOverlay.Store(true)
	AppStatus.Store(AppStatusReady)
	start := time.Unix(1_700_000_000, 0)
	recordPointerReadResultAt(start, pointerReadHoveredItem, false)
	recordPointerReadResultAt(start.Add(3*time.Second), pointerReadHoveredItem, false)
	if messageCount != 1 {
		t.Fatalf("message count = %d, want 1", messageCount)
	}
	if ShowOverlay.Load() {
		t.Fatal("overlay remained visible after sustained pointer failure")
	}
	if AppStatus.Load() != AppStatusGameLayoutIncompatible {
		t.Fatalf("status = %d, want layout incompatible", AppStatus.Load())
	}

	recordPointerReadResultAt(start.Add(4*time.Second), pointerReadHoveredItem, true)
	if AppStatus.Load() != AppStatusReady {
		t.Fatalf("status = %d, want ready after successful read", AppStatus.Load())
	}

	ShowOverlay.Store(true)
	recordPointerReadResultAt(start.Add(5*time.Second), pointerReadHoveredItem, false)
	recordPointerReadResultAt(start.Add(8*time.Second), pointerReadHoveredItem, false)
	if messageCount != 1 {
		t.Fatalf("message count after second failure = %d, want 1", messageCount)
	}
	if AppStatus.Load() != AppStatusGameLayoutIncompatible {
		t.Fatalf("status = %d, want layout incompatible after second failure", AppStatus.Load())
	}
}

func TestUpdateGameLayoutConfigs(t *testing.T) {
	// Setup test server
	remoteLayout := []byte(strings.Replace(string(embeddedGameLayoutJSON), "0x05DCFD70", "0x00000030", 1))
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !strings.Contains(request.URL.RawQuery, "nocache=") {
			t.Errorf("expected query parameter 'nocache', got query %q", request.URL.RawQuery)
		}
		_, _ = writer.Write(remoteLayout)
	}))
	defer server.Close()

	// Backup and restore globals
	oldURL := gameLayoutURL
	gameLayoutURL = server.URL
	previousLayout := ActiveGameLayout
	previousSource := GameLayoutSource
	oldShowInfo := showInfoMessageBoxMock
	oldShowError := showErrorMessageBoxMock

	t.Cleanup(func() {
		gameLayoutURL = oldURL
		ActiveGameLayout = previousLayout
		GameLayoutSource = previousSource
		showInfoMessageBoxMock = oldShowInfo
		showErrorMessageBoxMock = oldShowError
	})

	var infoCalled bool
	showInfoMessageBoxMock = func(title, message string) {
		infoCalled = true
		if title != "Update Configs" {
			t.Errorf("expected title 'Update Configs', got %q", title)
		}
	}

	showErrorMessageBoxMock = func(title, message string) {
		t.Errorf("error dialog shown: %s - %s", title, message)
	}

	// Put layout health into incompatible state to verify it resets
	GameLayoutReadHealth.mu.Lock()
	GameLayoutReadHealth.incompatible = true
	GameLayoutReadHealth.mu.Unlock()

	updateGameLayoutConfigs()

	if !infoCalled {
		t.Error("expected info box to be called on successful update")
	}

	GameLayoutMu.RLock()
	currentOffset := ActiveGameLayout.HoveredItemPointerBaseOffset
	GameLayoutMu.RUnlock()

	if currentOffset != 0x30 {
		t.Errorf("ActiveGameLayout offset = 0x%X, want 0x30", currentOffset)
	}

	GameLayoutReadHealth.mu.Lock()
	incomp := GameLayoutReadHealth.incompatible
	GameLayoutReadHealth.mu.Unlock()

	if incomp {
		t.Error("expected incompatibility state to be reset")
	}
}
