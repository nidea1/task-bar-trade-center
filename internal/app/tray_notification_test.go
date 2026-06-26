package app

import (
	"strings"
	"testing"
)

func TestTrayNotificationsEmitOnlyForDistinctStateTransitions(t *testing.T) {
	originalAppHWND := activeApp.appHWND
	originalTrayIconAdded := activeApp.trayIconAdded
	originalPublisher := publishTrayNotification
	originalPreference := currentDisplayLanguagePreference()
	originalStatus := activeApp.appStatus.Load()
	originalConfigStatus := activeApp.configurationStatus.Load()
	originalUpdateStatus := activeApp.updateStatus.Load()

	var received []string
	activeApp.appHWND = 1
	activeApp.trayIconAdded = true
	publishTrayNotification = func(_ string, message string) {
		received = append(received, message)
	}
	applyDisplayLanguagePreference("tr-TR")
	activeApp.appStatus.Store(AppStatusStarting)
	activeApp.configurationStatus.Store(ConfigStatusUnknown)
	activeApp.updateStatus.Store(UpdateStatusUnknown)
	clearPendingTrayNotifications()

	t.Cleanup(func() {
		activeApp.appHWND = originalAppHWND
		activeApp.trayIconAdded = originalTrayIconAdded
		publishTrayNotification = originalPublisher
		activeApp.appStatus.Store(originalStatus)
		activeApp.configurationStatus.Store(originalConfigStatus)
		activeApp.updateStatus.Store(originalUpdateStatus)
		applyDisplayLanguagePreference(originalPreference)
		clearPendingTrayNotifications()
	})

	setAppStatus(AppStatusWaitingForGame)
	flushTrayNotifications()
	if len(received) != 1 || !strings.Contains(received[0], "TaskBarHero") {
		t.Fatalf("WaitingForGame notification = %q", received)
	}

	setAppStatus(AppStatusWaitingForGame)
	flushTrayNotifications()
	if len(received) != 1 {
		t.Fatalf("duplicate runtime state emitted a notification: %q", received)
	}

	setConfigurationStatus(ConfigStatusLocalCache, "diagnostic detail")
	setConfigurationStatus(ConfigStatusLocalCache, "changed diagnostic detail")
	setUpdateState(UpdateStatusChecking, "", "", "")
	setUpdateState(UpdateStatusChecking, "", "", "")
	setUpdateState(UpdateStatusFailed, "network diagnostic detail", "", "https://example.com/release")
	flushTrayNotifications()
	if len(received) != 2 {
		t.Fatalf("expected WaitingForGame + UpdateFailed notifications, got = %q", received)
	}
	if strings.Contains(strings.Join(received, "\n"), "diagnostic detail") {
		t.Fatalf("technical diagnostics leaked into tray notification: %q", received)
	}
}

func TestStartupNotificationAndLanguageMenuMapping(t *testing.T) {
	originalAppHWND := activeApp.appHWND
	originalTrayIconAdded := activeApp.trayIconAdded
	originalPublisher := publishTrayNotification
	originalPreference := currentDisplayLanguagePreference()

	var received []string
	activeApp.appHWND = 1
	activeApp.trayIconAdded = true
	publishTrayNotification = func(_ string, message string) {
		received = append(received, message)
	}
	applyDisplayLanguagePreference("en-US")
	clearPendingTrayNotifications()
	t.Cleanup(func() {
		activeApp.appHWND = originalAppHWND
		activeApp.trayIconAdded = originalTrayIconAdded
		publishTrayNotification = originalPublisher
		applyDisplayLanguagePreference(originalPreference)
		clearPendingTrayNotifications()
	})

	notifyApplicationStarted()
	flushTrayNotifications()
	if len(received) != 1 || received[0] != "Starting…" {
		t.Fatalf("startup notification should be 'Starting…', got = %q", received)
	}

	if language, ok := appLanguageForMenuCommand(MenuLanguageBase + 8); !ok || language != "ja-JP" {
		t.Fatalf("language menu mapping = %q, %v", language, ok)
	}
	if _, ok := appLanguageForMenuCommand(MenuLanguageBase + uint32(len(supportedAppLocales))); ok {
		t.Fatal("out-of-range language menu command was accepted")
	}
}

func clearPendingTrayNotifications() {
	trayNotifications.Lock()
	trayNotifications.pending = nil
	trayNotifications.started = false
	trayNotifications.Unlock()
}
