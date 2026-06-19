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
