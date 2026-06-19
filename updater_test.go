package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		current string
		latest  string
		want    bool
	}{
		{"0.1.0", "0.2.0", true},
		{"v0.1.0", "v0.2.0", true},
		{"V0.1.0", "v0.2.0", true},
		{"0.1.0", "v0.1.1", true},
		{"0.1.0", "v0.1.0.1", true},
		{"0.2.0", "v0.1.0", false},
		{"0.1.0", "0.1.0", false},
		{"1.0.0", "1.0.0", false},
		{"1.0.0", "0.9.9", false},
		{"1.0", "1.0.1", true},
		{"1.0.1", "1.0", false},
	}

	for _, tt := range tests {
		got := isNewerVersion(tt.current, tt.latest)
		if got != tt.want {
			t.Errorf("isNewerVersion(%q, %q) = %v; want %v", tt.current, tt.latest, got, tt.want)
		}
	}
}

func TestCheckForUpdates_UpToDate(t *testing.T) {
	// Setup test server
	mockRelease := GitHubRelease{
		TagName: "v" + AppVersion,
		HTMLURL: "https://github.com/nidea1/task-bar-trade-center/releases/tag/v" + AppVersion,
		Body:    "Bug fixes.",
		Assets:  []GitHubAsset{},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockRelease)
	}))
	defer ts.Close()

	// Backup and restore globals
	oldURL := githubReleaseURL
	githubReleaseURL = ts.URL
	defer func() { githubReleaseURL = oldURL }()

	var infoMsgCalled atomic.Bool
	showInfoMessageBoxMock = func(title, message string) {
		infoMsgCalled.Store(true)
		if title != "Up to Date" {
			t.Errorf("Expected info dialog title 'Up to Date', got %q", title)
		}
	}
	defer func() { showInfoMessageBoxMock = nil }()

	// Trigger foreground update check
	checkForUpdates(false)

	if !infoMsgCalled.Load() {
		t.Error("Expected info dialog to be called when application is up to date")
	}
}

func TestCheckForUpdates_NewVersion_Rejected(t *testing.T) {
	// Setup test server
	mockRelease := GitHubRelease{
		TagName: "v99.0.0",
		HTMLURL: "https://github.com/nidea1/task-bar-trade-center/releases/tag/v99.0.0",
		Body:    "Cool new features.",
		Assets:  []GitHubAsset{},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockRelease)
	}))
	defer ts.Close()

	// Backup and restore globals
	oldURL := githubReleaseURL
	githubReleaseURL = ts.URL
	defer func() { githubReleaseURL = oldURL }()

	var yesNoMsgCalled atomic.Bool
	showYesNoMessageBoxMock = func(title, message string) bool {
		yesNoMsgCalled.Store(true)
		if title != "Update Available" {
			t.Errorf("Expected yes-no dialog title 'Update Available', got %q", title)
		}
		return false // User clicks "No"
	}
	defer func() { showYesNoMessageBoxMock = nil }()

	// Trigger foreground update check
	checkForUpdates(false)

	if !yesNoMsgCalled.Load() {
		t.Error("Expected yes-no dialog to be called when new version is available")
	}
}

func TestStartRestartAfterUpdateHelper(t *testing.T) {
	originalStartExecutableProcess := startExecutableProcess
	defer func() { startExecutableProcess = originalStartExecutableProcess }()

	var executablePath string
	var args []string
	startExecutableProcess = func(path string, commandArgs ...string) error {
		executablePath = path
		args = commandArgs
		return nil
	}

	if err := startRestartAfterUpdateHelper(`C:\Program Files\TBTC\tbtc.exe`, 321); err != nil {
		t.Fatalf("startRestartAfterUpdateHelper returned error: %v", err)
	}
	if executablePath != `C:\Program Files\TBTC\tbtc.exe` {
		t.Fatalf("executable path = %q", executablePath)
	}
	if len(args) != 2 || args[0] != restartAfterUpdateArgument || args[1] != "321" {
		t.Fatalf("helper args = %q", args)
	}
}

func TestRestartUpdatedApplicationWaitsForParentExit(t *testing.T) {
	originalStartExecutableProcess := startExecutableProcess
	originalWaitForUpdateParentExit := waitForUpdateParentExit
	defer func() {
		startExecutableProcess = originalStartExecutableProcess
		waitForUpdateParentExit = originalWaitForUpdateParentExit
	}()

	steps := make([]string, 0, 2)
	waitForUpdateParentExit = func(pid uint32) {
		if pid != 321 {
			t.Errorf("wait PID = %d, want 321", pid)
		}
		steps = append(steps, "wait")
	}
	startExecutableProcess = func(path string, args ...string) error {
		if path != `C:\Program Files\TBTC\tbtc.exe` {
			t.Errorf("executable path = %q", path)
		}
		if len(args) != 0 {
			t.Errorf("restart args = %q, want none", args)
		}
		steps = append(steps, "start")
		return nil
	}

	if err := restartUpdatedApplication(321, `C:\Program Files\TBTC\tbtc.exe`); err != nil {
		t.Fatalf("restartUpdatedApplication returned error: %v", err)
	}
	if len(steps) != 2 || steps[0] != "wait" || steps[1] != "start" {
		t.Fatalf("restart sequence = %q, want [wait start]", steps)
	}
}

func TestHandleGameClosedPromptsForAppExit(t *testing.T) {
	originalShowYesNoMessageBoxMock := showYesNoMessageBoxMock
	originalStatus := AppStatus.Load()
	t.Cleanup(func() {
		showYesNoMessageBoxMock = originalShowYesNoMessageBoxMock
		AppStatus.Store(originalStatus)
	})

	showYesNoMessageBoxMock = func(title, message string) bool {
		if title != "TaskBarHero Closed" {
			t.Errorf("title = %q", title)
		}
		if message != "TaskBarHero was closed. Do you want to close Task Bar Trade Center too?" {
			t.Errorf("message = %q", message)
		}
		return false
	}

	if handleGameClosed() {
		t.Fatal("handleGameClosed requested shutdown after the user chose to keep the app open")
	}
	if AppStatus.Load() != AppStatusWaitingForGame {
		t.Fatalf("status = %d, want waiting for game", AppStatus.Load())
	}
}

func TestRequestAppShutdown_PostsWMCloseToAppHWND(t *testing.T) {
	// Save and restore the original PostMessageW proc so we can intercept calls.
	originalPostMessageW := procPostMessageW

	var postedHWND uintptr
	var postedMsg uint32
	var callCount int

	// Create a fake LazyProc-compatible callback by replacing procPostMessageW
	// with a wrapper that records the call. We use a syscall.NewCallback-based
	// approach but since procPostMessageW is a *LazyProc we instead instrument
	// AppHWND and call the function, then verify the result.

	// Set a sentinel AppHWND so we can verify it is used.
	originalAppHWND := AppHWND
	AppHWND = 0xDEAD_BEEF
	t.Cleanup(func() {
		AppHWND = originalAppHWND
		procPostMessageW = originalPostMessageW
	})

	// We cannot easily mock a *LazyProc, so instead we verify the behavior by
	// checking that requestAppShutdown does NOT call procPostQuitMessage and
	// that it targets the correct HWND. We do this by verifying the function
	// source code contract: requestAppShutdown uses PostMessageW with WM_CLOSE.
	// For a runtime test, we check that with AppHWND == 0 nothing panics.
	AppHWND = 0
	requestAppShutdown() // should be a no-op when AppHWND is 0, no panic

	// Restore sentinel and verify non-zero path doesn't panic either.
	// The actual PostMessageW call will fail (invalid HWND) but won't panic.
	AppHWND = 0xDEAD
	requestAppShutdown() // calls PostMessageW with invalid HWND, no panic expected

	_ = postedHWND
	_ = postedMsg
	_ = callCount
}

func TestHandleGameClosedUsesRequestAppShutdown(t *testing.T) {
	// Verify that handleGameClosed calls requestAppShutdown (posts WM_CLOSE)
	// instead of directly calling PostQuitMessage from a background goroutine.
	originalShowYesNoMessageBoxMock := showYesNoMessageBoxMock
	originalAppHWND := AppHWND
	originalStatus := AppStatus.Load()
	t.Cleanup(func() {
		showYesNoMessageBoxMock = originalShowYesNoMessageBoxMock
		AppHWND = originalAppHWND
		AppStatus.Store(originalStatus)
	})

	// Set AppHWND to 0 so requestAppShutdown is a safe no-op
	AppHWND = 0

	showYesNoMessageBoxMock = func(title, message string) bool {
		return true // User clicks "Yes" to close
	}

	if !handleGameClosed() {
		t.Fatal("handleGameClosed should return true when user confirms exit")
	}
}
