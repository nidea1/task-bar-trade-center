package app

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/game"
)

const (
	gameLayoutUserAgent = AppShortName + "-game.GameLayout"

	localGameLayoutPollInterval = 100 * time.Millisecond
	localGameLayoutStableDelay  = 250 * time.Millisecond
)

var (
	embeddedGameLayoutJSON = game.EmbeddedLayoutJSON()
	gameLayoutURL          = game.DefaultLayoutURL
	gameLayoutHTTPClient   = &http.Client{Timeout: game.LayoutRequestTimeout}
)

func loadGameLayout() error {
	localLayoutPath := strings.TrimSpace(os.Getenv(game.LayoutPathEnvironment))
	if localLayoutPath != "" {
		layout, err := game.LoadGameLayoutFromFile(localLayoutPath)
		if err != nil {
			return fmt.Errorf("local development game layout %q: %w", localLayoutPath, err)
		}
		setActiveGameLayout(layout, game.LayoutSourceLocalDevelopment)
		fmt.Printf("Game layout loaded from %s: %s\n", game.LayoutSourceLocalDevelopment, localLayoutPath)
		return nil
	}

	layout, source, err := game.ResolveGameLayout(gameLayoutURL, activeApp.gameLayoutCacheFilePath, gameLayoutHTTPClient, embeddedGameLayoutJSON, gameLayoutUserAgent)
	if err != nil {
		return err
	}
	setActiveGameLayout(layout, source)
	fmt.Printf("Game layout loaded from %s.\n", source)
	return nil
}

// loadLocalGameLayout makes startup independent from the network. A validated
// remote version is fetched later by refreshGameLayoutInBackground.
func loadLocalGameLayout() error {
	localLayoutPath := strings.TrimSpace(os.Getenv(game.LayoutPathEnvironment))
	if localLayoutPath != "" {
		layout, err := game.LoadGameLayoutFromFile(localLayoutPath)
		if err != nil {
			return fmt.Errorf("local development game layout %q: %w", localLayoutPath, err)
		}
		setActiveGameLayout(layout, game.LayoutSourceLocalDevelopment)
		setConfigurationStatus(ConfigStatusDevelopment, "")
		fmt.Printf("Game layout loaded from %s: %s\n", game.LayoutSourceLocalDevelopment, localLayoutPath)
		return nil
	}

	if activeApp.gameLayoutCacheFilePath != "" {
		if raw, err := os.ReadFile(activeApp.gameLayoutCacheFilePath); err == nil {
			if layout, parseErr := game.ParseGameLayout(raw); parseErr == nil {
				layout, parseErr = game.ApplyEmbeddedAOBFallback(layout, embeddedGameLayoutJSON)
				if parseErr == nil {
					setActiveGameLayout(layout, game.LayoutSourceCache)
					setConfigurationStatus(ConfigStatusLocalCache, "")
					fmt.Println("Game layout loaded from cache.")
					return nil
				}
			}
			fmt.Println("Game layout cache is invalid; using embedded layout.")
		}
	}

	layout, err := game.ParseGameLayout(embeddedGameLayoutJSON)
	if err != nil {
		return fmt.Errorf("embedded game layout is invalid: %w", err)
	}
	setActiveGameLayout(layout, game.LayoutSourceEmbeddedDefault)
	setConfigurationStatus(ConfigStatusEmbedded, "")
	fmt.Println("Game layout loaded from embedded defaults.")
	return nil
}

func watchLocalGameLayoutChanges() {
	localLayoutPath := strings.TrimSpace(os.Getenv(game.LayoutPathEnvironment))
	if localLayoutPath == "" {
		return
	}

	if absolutePath, err := filepath.Abs(localLayoutPath); err == nil {
		localLayoutPath = absolutePath
	}

	initialRaw, err := os.ReadFile(localLayoutPath)
	if err != nil {
		fmt.Printf("Local game layout watcher could not read %q: %v\n", localLayoutPath, err)
		return
	}

	lastProcessedHash := sha256.Sum256(initialRaw)
	var pendingHash [32]byte
	var pendingSince time.Time
	pending := false
	lastReadError := ""

	ticker := time.NewTicker(localGameLayoutPollInterval)
	defer ticker.Stop()

	fmt.Printf("Watching local game layout for changes: %s\n", localLayoutPath)

	for range ticker.C {
		raw, err := os.ReadFile(localLayoutPath)
		if err != nil {
			errorMessage := err.Error()
			if errorMessage != lastReadError {
				fmt.Printf("Local game layout watcher read failed: %v\n", err)
				lastReadError = errorMessage
			}
			continue
		}

		if lastReadError != "" {
			fmt.Println("Local game layout watcher resumed.")
			lastReadError = ""
		}

		currentHash := sha256.Sum256(raw)
		if currentHash == lastProcessedHash {
			pending = false
			continue
		}

		now := time.Now()
		if !pending || currentHash != pendingHash {
			pendingHash = currentHash
			pendingSince = now
			pending = true
			continue
		}

		if now.Sub(pendingSince) < localGameLayoutStableDelay {
			continue
		}

		lastProcessedHash = currentHash
		pending = false

		layout, err := game.ParseGameLayout(raw)
		if err != nil {
			fmt.Printf("Local game layout change rejected; last valid layout remains active: %v\n", err)
			continue
		}

		applyReloadedLocalGameLayout(layout, localLayoutPath)
	}
}

func applyReloadedLocalGameLayout(layout game.GameLayout, localLayoutPath string) {
	setActiveGameLayout(layout, game.LayoutSourceLocalDevelopment)

	activeApp.gameLayoutReadHealth.Reset()
	activeApp.tooltipXAOBResolver.Reset()
	activeApp.tooltipYAOBResolver.Reset()
	activeApp.tooltipHeightAOBResolver.Reset()

	setConfigurationStatus(ConfigStatusDevelopment, "")
	if activeApp.gameReady.Load() {
		setAppStatus(AppStatusReady)
	} else {
		setAppStatus(AppStatusWaitingForGame)
	}

	if activeApp.showOverlay.Load() {
		redrawOverlay()
	}

	fmt.Printf("Local game layout reloaded and applied: %s\n", localLayoutPath)
}

func refreshGameLayoutInBackground() {
	if strings.TrimSpace(os.Getenv(game.LayoutPathEnvironment)) != "" {
		return
	}
	startedAt := time.Now()
	setConfigurationStatus(ConfigStatusRefreshing, "")
	activeApp.gameLayoutReadHealth.Reset()
	defer func() { fmt.Printf("startup remote_config_finished=%s\n", time.Since(startedAt)) }()
	raw, err := game.DownloadGameLayout(gameLayoutURL, gameLayoutHTTPClient, gameLayoutUserAgent)
	if err != nil {
		fmt.Printf("Game layout refresh failed: %v\n", err)
		activeApp.gameLayoutReadHealth.Reset()
		setConfigurationStatus(ConfigStatusRefreshFailed, "")
		return
	}
	layout, err := game.ParseGameLayout(raw)
	if err == nil {
		layout, err = game.ApplyEmbeddedAOBFallback(layout, embeddedGameLayoutJSON)
	}
	if err != nil {
		fmt.Printf("Downloaded game layout is invalid: %v\n", err)
		activeApp.gameLayoutReadHealth.Reset()
		setConfigurationStatus(ConfigStatusRefreshFailed, "")
		return
	}
	if activeApp.gameLayoutCacheFilePath != "" {
		if err := game.WriteGameLayoutCache(activeApp.gameLayoutCacheFilePath, raw); err != nil {
			fmt.Printf("Game layout cache could not be written: %v\n", err)
		}
	}
	setActiveGameLayout(layout, game.LayoutSourceRemote)
	activeApp.gameLayoutReadHealth.Reset()
	setConfigurationStatus(ConfigStatusCurrent, "")
	fmt.Println("Game layout refreshed from remote.")
}

func recordPointerReadResult(kind game.PointerReadKind, success bool) {
	recordPointerReadResultAt(time.Now(), kind, success)
}

func recordPointerReadResultAt(now time.Time, kind game.PointerReadKind, success bool) {
	becameIncompatible, shouldNotify, recovered := activeApp.gameLayoutReadHealth.Record(now, kind, success)
	if recovered {
		setAppStatus(AppStatusReady)
		return
	}
	if !becameIncompatible {
		return
	}
	if activeApp.configurationStatus.Load() == ConfigStatusRefreshing {
		activeApp.gameLayoutReadHealth.Reset()
		return
	}

	activeApp.showOverlay.Store(false)
	redrawOverlay()
	setAppStatus(AppStatusGameLayoutIncompatible)
	fmt.Println("Game memory layout could not be read continuously; overlay disabled.")
	if !shouldNotify {
		return
	}
	showErrorMessageBox(
		tr("dialog.layout_incompatible.title"),
		tr("dialog.layout_incompatible.body", activeApp.logFilePath),
	)
}

// updateGameLayoutConfigs reloads the game layout configuration from remote.
func updateGameLayoutConfigs() {
	setConfigurationStatus(ConfigStatusRefreshing, "")
	localLayoutPath := strings.TrimSpace(os.Getenv(game.LayoutPathEnvironment))
	var raw []byte
	var layout game.GameLayout
	var err error
	var loadedFromLocal bool

	if localLayoutPath != "" {
		layout, err = game.LoadGameLayoutFromFile(localLayoutPath)
		loadedFromLocal = true
	} else {
		bustedURL := fmt.Sprintf("%s?nocache=%d", gameLayoutURL, time.Now().UnixNano())
		raw, err = game.DownloadGameLayout(bustedURL, gameLayoutHTTPClient, gameLayoutUserAgent)
		if err == nil {
			layout, err = game.ParseGameLayout(raw)
			if err == nil {
				layout, err = game.ApplyEmbeddedAOBFallback(layout, embeddedGameLayoutJSON)
			}
		}
	}

	if err != nil {
		setConfigurationStatus(ConfigStatusRefreshFailed, err.Error())
		return
	}

	if !loadedFromLocal && activeApp.gameLayoutCacheFilePath != "" {
		if err := game.WriteGameLayoutCache(activeApp.gameLayoutCacheFilePath, raw); err != nil {
			fmt.Printf("Game layout cache could not be written: %v\n", err)
		}
	}

	if loadedFromLocal {
		setActiveGameLayout(layout, game.LayoutSourceLocalDevelopment)
	} else {
		setActiveGameLayout(layout, game.LayoutSourceRemote)
	}
	activeApp.gameLayoutReadHealth.Reset()
	if activeApp.gameReady.Load() {
		setAppStatus(AppStatusReady)
	} else {
		setAppStatus(AppStatusWaitingForGame)
	}
	if activeApp.showOverlay.Load() {
		redrawOverlay()
	}
	setConfigurationStatus(ConfigStatusCurrent, "")
}

func setActiveGameLayout(layout game.GameLayout, source string) {
	activeApp.gameLayoutMu.Lock()
	activeApp.activeGameLayout = layout
	activeApp.gameLayoutSource = source
	activeApp.gameLayoutMu.Unlock()
}
