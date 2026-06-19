package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	gameLayoutSchemaVersion       = 2
	gameLayoutRequestTimeout      = 5 * time.Second
	gameLayoutPointerFailureAfter = 3 * time.Second

	gameLayoutSourceRemote           = "remote"
	gameLayoutSourceCache            = "cache"
	gameLayoutSourceEmbeddedDefault  = "embedded-default"
	gameLayoutSourceLocalDevelopment = "local-development"

	gameLayoutPathEnvironment = "TBTC_GAME_LAYOUT_PATH"
)

var (
	//go:embed game-layout.json
	embeddedGameLayoutJSON []byte

	gameLayoutURL        = "https://raw.githubusercontent.com/nidea1/task-bar-trade-center/main/game-layout.json"
	gameLayoutHTTPClient = &http.Client{Timeout: gameLayoutRequestTimeout}
)

type gameLayoutDocument struct {
	SchemaVersion int `json:"schema_version"`
	HoveredItem   struct {
		PointerBaseOffset string   `json:"pointer_base_offset"`
		PointerOffsets    []string `json:"pointer_offsets"`
		KeyOffset         string   `json:"key_offset"`
	} `json:"hovered_item"`
	Tooltip struct {
		XPointerBaseOffset      string   `json:"x_pointer_base_offset"`
		XPointerOffsets         []string `json:"x_pointer_offsets"`
		YPointerBaseOffset      string   `json:"y_pointer_base_offset"`
		YPointerOffsets         []string `json:"y_pointer_offsets"`
		WidthPointerBaseOffset  string   `json:"width_pointer_base_offset"`
		WidthPointerOffsets     []string `json:"width_pointer_offsets"`
		HeightPointerBaseOffset string   `json:"height_pointer_base_offset"`
		HeightPointerOffsets    []string `json:"height_pointer_offsets"`
	} `json:"tooltip"`
	PlacementCalibrations []OverlayPlacementCalibration `json:"placement_calibrations"`
	XCalibrations         []OverlayXCalibration         `json:"x_calibrations"`
}

type GameLayout struct {
	HoveredItemPointerBaseOffset uintptr
	HoveredItemPointerOffsets    []uintptr
	HoveredItemKeyOffset         uintptr

	TooltipXPointerBaseOffset      uintptr
	TooltipXPointerOffsets         []uintptr
	TooltipYPointerBaseOffset      uintptr
	TooltipYPointerOffsets         []uintptr
	TooltipWidthPointerBaseOffset  uintptr
	TooltipWidthPointerOffsets     []uintptr
	TooltipHeightPointerBaseOffset uintptr
	TooltipHeightPointerOffsets    []uintptr

	PlacementCalibrations []OverlayPlacementCalibration
	XCalibrations         []OverlayXCalibration
}

func loadGameLayout() error {
	localLayoutPath := strings.TrimSpace(os.Getenv(gameLayoutPathEnvironment))
	if localLayoutPath != "" {
		layout, err := loadGameLayoutFromFile(localLayoutPath)
		if err != nil {
			return fmt.Errorf("local development game layout %q: %w", localLayoutPath, err)
		}
		GameLayoutMu.Lock()
		ActiveGameLayout = layout
		GameLayoutSource = gameLayoutSourceLocalDevelopment
		GameLayoutMu.Unlock()
		fmt.Printf("Game layout loaded from %s: %s\n", gameLayoutSourceLocalDevelopment, localLayoutPath)
		return nil
	}

	layout, source, err := resolveGameLayout(gameLayoutURL, GameLayoutCacheFilePath, gameLayoutHTTPClient, embeddedGameLayoutJSON)
	if err != nil {
		return err
	}

	GameLayoutMu.Lock()
	ActiveGameLayout = layout
	GameLayoutSource = source
	GameLayoutMu.Unlock()
	fmt.Printf("Game layout loaded from %s.\n", source)
	return nil
}

func loadGameLayoutFromFile(filePath string) (GameLayout, error) {
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return GameLayout{}, err
	}
	return parseGameLayout(raw)
}

func resolveGameLayout(remoteURL, cacheFilePath string, client *http.Client, embeddedDefaults []byte) (GameLayout, string, error) {
	remoteBytes, err := downloadGameLayout(remoteURL, client)
	if err == nil {
		layout, parseErr := parseGameLayout(remoteBytes)
		if parseErr == nil {
			if cacheFilePath != "" {
				if writeErr := writeGameLayoutCache(cacheFilePath, remoteBytes); writeErr != nil {
					fmt.Printf("Game layout cache could not be written: %v\n", writeErr)
				}
			}
			return layout, gameLayoutSourceRemote, nil
		}
		err = fmt.Errorf("remote layout is invalid: %w", parseErr)
	}
	fmt.Printf("Game layout remote read failed: %v\n", err)

	if cacheFilePath != "" {
		if cachedBytes, readErr := os.ReadFile(cacheFilePath); readErr == nil {
			if layout, parseErr := parseGameLayout(cachedBytes); parseErr == nil {
				return layout, gameLayoutSourceCache, nil
			} else {
				fmt.Printf("Game layout cache is invalid: %v\n", parseErr)
			}
		} else if !os.IsNotExist(readErr) {
			fmt.Printf("Game layout cache could not be read: %v\n", readErr)
		}
	}

	layout, parseErr := parseGameLayout(embeddedDefaults)
	if parseErr != nil {
		return GameLayout{}, "", fmt.Errorf("embedded game layout is invalid: %w", parseErr)
	}
	return layout, gameLayoutSourceEmbeddedDefault, nil
}

func downloadGameLayout(remoteURL string, client *http.Client) ([]byte, error) {
	if client == nil {
		return nil, fmt.Errorf("HTTP client is not configured")
	}

	req, err := http.NewRequest(http.MethodGet, remoteURL, nil)
	if err != nil {
		return nil, fmt.Errorf("could not create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not contact GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub returned status %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("could not read response: %w", err)
	}
	return body, nil
}

func parseGameLayout(raw []byte) (GameLayout, error) {
	var document gameLayoutDocument
	if err := json.Unmarshal(raw, &document); err != nil {
		return GameLayout{}, err
	}
	if document.SchemaVersion != gameLayoutSchemaVersion {
		return GameLayout{}, fmt.Errorf("unsupported schema_version %d", document.SchemaVersion)
	}
	if document.PlacementCalibrations == nil {
		return GameLayout{}, fmt.Errorf("placement_calibrations is required")
	}

	hoveredBase, err := parseLayoutOffset("hovered_item.pointer_base_offset", document.HoveredItem.PointerBaseOffset)
	if err != nil {
		return GameLayout{}, err
	}
	hoveredOffsets, err := parseLayoutOffsets("hovered_item.pointer_offsets", document.HoveredItem.PointerOffsets)
	if err != nil {
		return GameLayout{}, err
	}
	hoveredKeyOffset, err := parseLayoutOffset("hovered_item.key_offset", document.HoveredItem.KeyOffset)
	if err != nil {
		return GameLayout{}, err
	}

	xBase, err := parseLayoutOffset("tooltip.x_pointer_base_offset", document.Tooltip.XPointerBaseOffset)
	if err != nil {
		return GameLayout{}, err
	}
	xOffsets, err := parseLayoutOffsets("tooltip.x_pointer_offsets", document.Tooltip.XPointerOffsets)
	if err != nil {
		return GameLayout{}, err
	}
	yBase, err := parseLayoutOffset("tooltip.y_pointer_base_offset", document.Tooltip.YPointerBaseOffset)
	if err != nil {
		return GameLayout{}, err
	}
	yOffsets, err := parseLayoutOffsets("tooltip.y_pointer_offsets", document.Tooltip.YPointerOffsets)
	if err != nil {
		return GameLayout{}, err
	}
	widthBase, err := parseLayoutOffset("tooltip.width_pointer_base_offset", document.Tooltip.WidthPointerBaseOffset)
	if err != nil {
		return GameLayout{}, err
	}
	widthOffsets, err := parseLayoutOffsets("tooltip.width_pointer_offsets", document.Tooltip.WidthPointerOffsets)
	if err != nil {
		return GameLayout{}, err
	}
	heightBase, err := parseLayoutOffset("tooltip.height_pointer_base_offset", document.Tooltip.HeightPointerBaseOffset)
	if err != nil {
		return GameLayout{}, err
	}
	heightOffsets, err := parseLayoutOffsets("tooltip.height_pointer_offsets", document.Tooltip.HeightPointerOffsets)
	if err != nil {
		return GameLayout{}, err
	}

	for index, calibration := range document.PlacementCalibrations {
		if calibration.TooltipWidth <= 0 || calibration.TooltipHeight <= 0 || calibration.PanelWidth <= 0 {
			return GameLayout{}, fmt.Errorf("placement_calibrations[%d] has invalid dimensions", index)
		}
	}

	return GameLayout{
		HoveredItemPointerBaseOffset:   hoveredBase,
		HoveredItemPointerOffsets:      hoveredOffsets,
		HoveredItemKeyOffset:           hoveredKeyOffset,
		TooltipXPointerBaseOffset:      xBase,
		TooltipXPointerOffsets:         xOffsets,
		TooltipYPointerBaseOffset:      yBase,
		TooltipYPointerOffsets:         yOffsets,
		TooltipWidthPointerBaseOffset:  widthBase,
		TooltipWidthPointerOffsets:     widthOffsets,
		TooltipHeightPointerBaseOffset: heightBase,
		TooltipHeightPointerOffsets:    heightOffsets,
		PlacementCalibrations:          append([]OverlayPlacementCalibration(nil), document.PlacementCalibrations...),
		XCalibrations:                  append([]OverlayXCalibration(nil), document.XCalibrations...),
	}, nil
}

func parseLayoutOffsets(name string, values []string) ([]uintptr, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("%s must not be empty", name)
	}

	offsets := make([]uintptr, len(values))
	for index, value := range values {
		offset, err := parseLayoutOffset(fmt.Sprintf("%s[%d]", name, index), value)
		if err != nil {
			return nil, err
		}
		offsets[index] = offset
	}
	return offsets, nil
}

func parseLayoutOffset(name, value string) (uintptr, error) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(strings.ToLower(value), "0x") {
		return 0, fmt.Errorf("%s must be a hexadecimal string", name)
	}

	parsed, err := strconv.ParseUint(value[2:], 16, strconv.IntSize)
	if err != nil {
		return 0, fmt.Errorf("%s is invalid: %w", name, err)
	}
	return uintptr(parsed), nil
}

func writeGameLayoutCache(cacheFilePath string, raw []byte) error {
	directory := filepath.Dir(cacheFilePath)
	temporaryFile, err := os.CreateTemp(directory, filepath.Base(cacheFilePath)+".tmp-*")
	if err != nil {
		return err
	}
	temporaryPath := temporaryFile.Name()
	defer os.Remove(temporaryPath)

	if err := temporaryFile.Chmod(0600); err != nil {
		temporaryFile.Close()
		return err
	}
	if _, err := temporaryFile.Write(raw); err != nil {
		temporaryFile.Close()
		return err
	}
	if err := temporaryFile.Sync(); err != nil {
		temporaryFile.Close()
		return err
	}
	if err := temporaryFile.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, cacheFilePath)
}

type pointerReadKind int

const (
	pointerReadHoveredItem pointerReadKind = iota
	pointerReadTooltip
)

type pointerReadHealth struct {
	mu                  sync.Mutex
	hoveredFailureSince time.Time
	tooltipFailureSince time.Time
	incompatible        bool
	notified            bool
}

func (health *pointerReadHealth) record(now time.Time, kind pointerReadKind, success bool) (bool, bool, bool) {
	health.mu.Lock()
	defer health.mu.Unlock()

	failureSince := &health.hoveredFailureSince
	if kind == pointerReadTooltip {
		failureSince = &health.tooltipFailureSince
	}

	if success {
		*failureSince = time.Time{}
		if health.incompatible && health.hoveredFailureSince.IsZero() && health.tooltipFailureSince.IsZero() {
			health.incompatible = false
			return false, false, true
		}
		return false, false, false
	}

	if failureSince.IsZero() {
		*failureSince = now
		return false, false, false
	}
	if !health.incompatible && now.Sub(*failureSince) >= gameLayoutPointerFailureAfter {
		health.incompatible = true
		if !health.notified {
			health.notified = true
			return true, true, false
		}
		return true, false, false
	}
	return false, false, false
}

func (health *pointerReadHealth) reset() {
	health.mu.Lock()
	defer health.mu.Unlock()
	health.hoveredFailureSince = time.Time{}
	health.tooltipFailureSince = time.Time{}
	health.incompatible = false
	health.notified = false
}

func recordPointerReadResult(kind pointerReadKind, success bool) {
	recordPointerReadResultAt(time.Now(), kind, success)
}

func recordPointerReadResultAt(now time.Time, kind pointerReadKind, success bool) {
	becameIncompatible, shouldNotify, recovered := GameLayoutReadHealth.record(now, kind, success)
	if recovered {
		setAppStatus(AppStatusReady)
		return
	}
	if !becameIncompatible {
		return
	}

	ShowOverlay.Store(false)
	redrawOverlay()
	setAppStatus(AppStatusGameLayoutIncompatible)
	fmt.Println("Game memory layout could not be read continuously; overlay disabled.")
	if !shouldNotify {
		return
	}
	showErrorMessageBox(
		"Game Memory Layout Update Required",
		fmt.Sprintf("Task Bar Trade Center could not read the game's memory layout continuously. A TaskBarHero update may have changed it.\n\nThe price HUD has been disabled. Connect to the internet and restart Task Bar Trade Center to download the latest layout, or update the application.\n\nDiagnostic log: %s", LogFilePath),
	)
}

func reportTooltipPointerRead(success bool) {
	if ShowOverlay.Load() {
		recordPointerReadResult(pointerReadTooltip, success)
	}
}

// updateGameLayoutConfigs reloads the game layout configuration from remote.
// It appends a cache-busting timestamp parameter to the URL to bypass GitHub raw CDN cache,
// resets the layout pointer read health, and updates the app status.
func updateGameLayoutConfigs() {
	bustedURL := fmt.Sprintf("%s?nocache=%d", gameLayoutURL, time.Now().UnixNano())
	layout, source, err := resolveGameLayout(bustedURL, GameLayoutCacheFilePath, gameLayoutHTTPClient, embeddedGameLayoutJSON)
	if err != nil {
		showErrorMessageBox("Update Configs Failed", fmt.Sprintf("Failed to update configurations:\n%v", err))
		return
	}

	GameLayoutMu.Lock()
	ActiveGameLayout = layout
	GameLayoutSource = source
	GameLayoutMu.Unlock()

	GameLayoutReadHealth.reset()
	if GameReady.Load() {
		setAppStatus(AppStatusReady)
	} else {
		setAppStatus(AppStatusWaitingForGame)
	}
	if ShowOverlay.Load() {
		redrawOverlay()
	}

	showInfoMessageBox("Update Configs", fmt.Sprintf("Configurations updated successfully from %s.", source))
}
