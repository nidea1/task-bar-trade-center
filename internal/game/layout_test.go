package game

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testUserAgent = "TBTC-test"

func TestResolveGameLayoutUsesRemoteAndCachesIt(t *testing.T) {
	embedded := EmbeddedLayoutJSON()
	remoteLayout := []byte(strings.Replace(string(embedded), "0x05DF8198", "0x00000020", 1))
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("User-Agent"); got != testUserAgent {
			t.Errorf("User-Agent = %q, want %q", got, testUserAgent)
		}
		_, _ = writer.Write(remoteLayout)
	}))
	defer server.Close()

	cacheFilePath := filepath.Join(t.TempDir(), "game-layout-cache.json")
	if err := os.WriteFile(cacheFilePath, []byte(`stale layout`), 0600); err != nil {
		t.Fatalf("write stale cache: %v", err)
	}
	layout, source, err := ResolveGameLayout(server.URL, cacheFilePath, server.Client(), embedded, testUserAgent)
	if err != nil {
		t.Fatalf("ResolveGameLayout returned error: %v", err)
	}
	if source != LayoutSourceRemote {
		t.Fatalf("source = %q, want %q", source, LayoutSourceRemote)
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
	if err := os.WriteFile(layoutPath, EmbeddedLayoutJSON(), 0600); err != nil {
		t.Fatalf("write layout: %v", err)
	}

	layout, err := LoadGameLayoutFromFile(layoutPath)
	if err != nil {
		t.Fatalf("LoadGameLayoutFromFile returned error: %v", err)
	}
	if layout.HoveredItemPointerBaseOffset != 0x05DF8198 {
		t.Fatalf("hovered pointer base = 0x%X, want 0x05DF8198", layout.HoveredItemPointerBaseOffset)
	}
}

func TestApplyEmbeddedAOBFallback(t *testing.T) {
	layout, err := ApplyEmbeddedAOBFallback(GameLayout{}, EmbeddedLayoutJSON())
	if err != nil {
		t.Fatalf("ApplyEmbeddedAOBFallback returned error: %v", err)
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

	if _, err := LoadGameLayoutFromFile(layoutPath); err == nil {
		t.Fatal("LoadGameLayoutFromFile succeeded for invalid JSON")
	}
}

func TestResolveGameLayoutUsesCacheWhenRemoteIsUnavailable(t *testing.T) {
	embedded := EmbeddedLayoutJSON()
	cacheFilePath := filepath.Join(t.TempDir(), "game-layout-cache.json")
	cachedLayout := []byte(strings.Replace(string(embedded), "0x05DF8198", "0x00000030", 1))
	if err := os.WriteFile(cacheFilePath, cachedLayout, 0600); err != nil {
		t.Fatalf("write cache: %v", err)
	}

	server := httptest.NewServer(http.NotFoundHandler())
	server.Close()
	layout, source, err := ResolveGameLayout(server.URL, cacheFilePath, &http.Client{Timeout: time.Second}, embedded, testUserAgent)
	if err != nil {
		t.Fatalf("ResolveGameLayout returned error: %v", err)
	}
	if source != LayoutSourceCache {
		t.Fatalf("source = %q, want %q", source, LayoutSourceCache)
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

	layout, source, err := ResolveGameLayout(server.URL, cacheFilePath, server.Client(), EmbeddedLayoutJSON(), testUserAgent)
	if err != nil {
		t.Fatalf("ResolveGameLayout returned error: %v", err)
	}
	if source != LayoutSourceEmbeddedDefault {
		t.Fatalf("source = %q, want %q", source, LayoutSourceEmbeddedDefault)
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
	layout, err := ParseGameLayout(EmbeddedLayoutJSON())
	if err != nil {
		t.Fatalf("ParseGameLayout returned error: %v", err)
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

func TestParseGameLayoutAcceptsLegacyWidthConfiguration(t *testing.T) {
	legacyLayout := strings.Replace(
		string(EmbeddedLayoutJSON()),
		`"height_pointer_base_offset":`,
		`"width_pointer_base_offset": "0x05DCAEA0", "width_pointer_offsets": ["0x40"], "height_pointer_base_offset":`,
		1,
	)
	legacyLayout = strings.Replace(legacyLayout, `"tooltip_height": 348`, `"tooltip_width": 308, "tooltip_height": 348`, 1)

	if _, err := ParseGameLayout([]byte(legacyLayout)); err != nil {
		t.Fatalf("ParseGameLayout rejected a legacy width configuration: %v", err)
	}
}

func TestParseGameLayoutRejectsUnsupportedAndIncompleteDocuments(t *testing.T) {
	modifyLayoutJSON := func(modify func(m map[string]any)) []byte {
		var doc map[string]any
		if err := json.Unmarshal(EmbeddedLayoutJSON(), &doc); err != nil {
			t.Fatalf("failed to unmarshal embedded layout: %v", err)
		}
		modify(doc)
		raw, err := json.Marshal(doc)
		if err != nil {
			t.Fatalf("failed to marshal modified layout: %v", err)
		}
		return raw
	}

	tests := []struct {
		name string
		raw  []byte
	}{
		{
			name: "unsupported schema",
			raw: modifyLayoutJSON(func(m map[string]any) {
				m["schema_version"] = 4
			}),
		},
		{
			name: "missing pointer value",
			raw: modifyLayoutJSON(func(m map[string]any) {
				hovered, ok := m["hovered_item"].(map[string]any)
				if !ok {
					t.Fatal("hovered_item not found or not a map")
				}
				hovered["key_offset"] = ""
			}),
		},
		{
			name: "empty pointer chain",
			raw: modifyLayoutJSON(func(m map[string]any) {
				hovered, ok := m["hovered_item"].(map[string]any)
				if !ok {
					t.Fatal("hovered_item not found or not a map")
				}
				hovered["pointer_offsets"] = []any{}
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseGameLayout(tt.raw); err == nil {
				t.Fatal("ParseGameLayout succeeded for an invalid document")
			}
		})
	}
}

func TestPointerReadHealthRequiresContinuousFailureAndRecovers(t *testing.T) {
	var health PointerReadHealth
	start := time.Unix(1_700_000_000, 0)

	if incompatible, notified, recovered := health.Record(start, PointerReadHoveredItem, false); incompatible || notified || recovered {
		t.Fatalf("first failure = incompatible:%v notified:%v recovered:%v", incompatible, notified, recovered)
	}
	if incompatible, notified, recovered := health.Record(start.Add(2*time.Second), PointerReadHoveredItem, true); incompatible || notified || recovered {
		t.Fatalf("short failure recovery = incompatible:%v notified:%v recovered:%v", incompatible, notified, recovered)
	}
	if incompatible, notified, recovered := health.Record(start.Add(3*time.Second), PointerReadHoveredItem, false); incompatible || notified || recovered {
		t.Fatalf("new failure = incompatible:%v notified:%v recovered:%v", incompatible, notified, recovered)
	}
	if incompatible, notified, recovered := health.Record(start.Add(6*time.Second), PointerReadHoveredItem, false); !incompatible || !notified || recovered {
		t.Fatalf("continuous failure = incompatible:%v notified:%v recovered:%v", incompatible, notified, recovered)
	}
	if incompatible, notified, recovered := health.Record(start.Add(7*time.Second), PointerReadHoveredItem, true); incompatible || notified || !recovered {
		t.Fatalf("successful read = incompatible:%v notified:%v recovered:%v", incompatible, notified, recovered)
	}
}

func TestPointerReadHealthIgnoresTooltipFailures(t *testing.T) {
	var health PointerReadHealth
	start := time.Unix(1_700_000_000, 0)

	for _, elapsed := range []time.Duration{0, 3 * time.Second, 10 * time.Second} {
		if incompatible, notified, recovered := health.Record(start.Add(elapsed), PointerReadTooltip, false); incompatible || notified || recovered {
			t.Fatalf("tooltip failure after %s = incompatible:%v notified:%v recovered:%v", elapsed, incompatible, notified, recovered)
		}
	}

	if health.IncompatibleForTest() {
		t.Fatal("tooltip failures marked the game layout incompatible")
	}
}
