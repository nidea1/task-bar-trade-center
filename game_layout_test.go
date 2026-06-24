package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolveGameLayoutUsesRemoteAndCachesIt(t *testing.T) {
	remoteLayout := []byte(strings.Replace(string(embeddedGameLayoutJSON), "0x05DF8198", "0x00000020", 1))
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
	if layout.HoveredItemPointerBaseOffset != 0x05DF8198 {
		t.Fatalf("hovered pointer base = 0x%X, want 0x05DF8198", layout.HoveredItemPointerBaseOffset)
	}
}

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
	oldEnv := os.Getenv(gameLayoutPathEnvironment)
	t.Cleanup(func() {
		GameLayoutCacheFilePath = oldCachePath
		gameLayoutURL = oldURL
		ActiveGameLayout = oldLayout
		GameLayoutSource = oldSource
		_ = os.Setenv(gameLayoutPathEnvironment, oldEnv)
	})
	GameLayoutCacheFilePath = cachePath
	gameLayoutURL = server.URL
	_ = os.Unsetenv(gameLayoutPathEnvironment)

	if err := loadLocalGameLayout(); err != nil {
		t.Fatalf("loadLocalGameLayout returned error: %v", err)
	}
	if GameLayoutSource != gameLayoutSourceCache {
		t.Fatalf("source = %q, want cache", GameLayoutSource)
	}
	if serverCalls != 0 {
		t.Fatalf("local load made %d remote requests", serverCalls)
	}
}

func TestApplyEmbeddedAOBFallback(t *testing.T) {
	layout, err := applyEmbeddedAOBFallback(GameLayout{}, embeddedGameLayoutJSON)
	if err != nil {
		t.Fatalf("applyEmbeddedAOBFallback returned error: %v", err)
	}
	if !layout.HoveredItemPointerBaseAOB.configured() || !layout.TooltipXPointerBaseAOB.configured() || !layout.TooltipYPointerBaseAOB.configured() || !layout.TooltipHeightPointerBaseAOB.configured() {
		t.Fatal("embedded AOB fallback was not applied to every pointer")
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
	localLayout := []byte(strings.Replace(string(embeddedGameLayoutJSON), "0x05DF8198", "0x00000020", 1))
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
	cachedLayout := []byte(strings.Replace(string(embeddedGameLayoutJSON), "0x05DF8198", "0x00000030", 1))
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
	if layout.HoveredItemPointerBaseOffset != 0x05DF8198 {
		t.Fatalf("hovered pointer base = 0x%X, want 0x05DF8198", layout.HoveredItemPointerBaseOffset)
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
	if layout.HoveredItemItemPtrOffset != 0x10 {
		t.Fatalf("hovered item item_ptr_offset = 0x%X, want 0x10", layout.HoveredItemItemPtrOffset)
	}
	if layout.HoveredItemKeyOffset != 0x30 {
		t.Fatalf("hovered item key offset = 0x%X, want 0x30", layout.HoveredItemKeyOffset)
	}
	if len(layout.TooltipXPointerOffsets) != 7 || len(layout.TooltipYPointerOffsets) != 7 {
		t.Fatalf("tooltip pointer offset lengths = %d and %d, want 7", len(layout.TooltipXPointerOffsets), len(layout.TooltipYPointerOffsets))
	}
	if !layout.HoveredItemPointerBaseAOB.configured() {
		t.Fatal("hovered item AOB pattern was not parsed")
	}
	if !layout.TooltipXPointerBaseAOB.configured() || !layout.TooltipYPointerBaseAOB.configured() || !layout.TooltipHeightPointerBaseAOB.configured() {
		t.Fatal("tooltip AOB pattern was not parsed")
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
			raw:  []byte(strings.Replace(string(embeddedGameLayoutJSON), `"schema_version": 3`, `"schema_version": 4`, 1)),
		},
		{
			name: "missing pointer value",
			raw:  []byte(strings.Replace(string(embeddedGameLayoutJSON), `"key_offset": "0x30"`, `"key_offset": ""`, 1)),
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

func TestPointerReadHealthIgnoresTooltipFailures(t *testing.T) {
	var health pointerReadHealth
	start := time.Unix(1_700_000_000, 0)

	for _, elapsed := range []time.Duration{0, 3 * time.Second, 10 * time.Second} {
		if incompatible, notified, recovered := health.record(start.Add(elapsed), pointerReadTooltip, false); incompatible || notified || recovered {
			t.Fatalf("tooltip failure after %s = incompatible:%v notified:%v recovered:%v", elapsed, incompatible, notified, recovered)
		}
	}

	if health.incompatible {
		t.Fatal("tooltip failures marked the game layout incompatible")
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
		if title != tr("dialog.layout_incompatible.title") {
			t.Errorf("title = %q", title)
		}
		if !strings.Contains(message, "Diagnostic log:") && !strings.Contains(message, "Tanılama günlüğü:") {
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
	remoteLayout := []byte(strings.Replace(string(embeddedGameLayoutJSON), "0x05DF8198", "0x00000030", 1))
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

	t.Cleanup(func() {
		gameLayoutURL = oldURL
		ActiveGameLayout = previousLayout
		GameLayoutSource = previousSource
	})

	// Put layout health into incompatible state to verify it resets
	GameLayoutReadHealth.mu.Lock()
	GameLayoutReadHealth.incompatible = true
	GameLayoutReadHealth.mu.Unlock()

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

	GameLayoutReadHealth.mu.Lock()
	incomp := GameLayoutReadHealth.incompatible
	GameLayoutReadHealth.mu.Unlock()

	if incomp {
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

	pattern := layout.HoveredItemPointerBaseAOB
	candidates, err := findAOBPointerBaseCandidates(pHandle, gameAssemblyBase, pattern)
	if err != nil {
		t.Fatalf("AOB scan error: %v", err)
	}

	t.Logf("Found %d base candidates", len(candidates))
	t.Log("Starting 30-second deep scan. Please hover over a marketable item in the game...")

	var scanObjectDeep func(uintptr, int, string, map[uintptr]bool)
	scanObjectDeep = func(addr uintptr, depth int, path string, visited map[uintptr]bool) {
		if depth > 4 || addr == 0 || visited[addr] || addr < 0x10000 {
			return
		}
		visited[addr] = true

		// 1. Scan direct fields for itemID
		for offset := uintptr(0); offset < 0x300; offset += 4 {
			val, ok := readInt32(pHandle, addr+offset)
			if !ok {
				continue
			}
			if item, exists := AllItemMap[int(val)]; exists && item.Marketable {
				t.Logf("  [MATCH] Path: %s + Offset 0x%X = %d (%s)", path, offset, val, item.Name["en-US"])
			}
		}

		// 2. Dereference sub-pointers and recurse
		for offset := uintptr(0); offset < 0x200; offset += 8 {
			subPtr, ok := readUintptr(pHandle, addr+offset)
			if ok && subPtr != 0 && subPtr != addr {
				scanObjectDeep(subPtr, depth+1, fmt.Sprintf("%s -> [+0x%X]", path, offset), visited)
			}
		}
	}

	for i := 0; i < 60; i++ {
		for idx, candidate := range candidates {
			// Resolve full pointer chain
			itemObjectPointerAddress, ok := resolvePointerChainAddress(pHandle, candidate, layout.HoveredItemPointerOffsets)
			if !ok {
				continue
			}
			itemObject, ok := readUintptr(pHandle, itemObjectPointerAddress)
			if !ok || itemObject == 0 {
				continue
			}

			t.Logf("[Active Hover] Candidate [%d] resolved itemObject: 0x%X. Scanning...", idx, itemObject)
			visited := make(map[uintptr]bool)
			scanObjectDeep(itemObject, 1, fmt.Sprintf("itemObject(0x%X)", itemObject), visited)
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Log("Scan finished.")
}
