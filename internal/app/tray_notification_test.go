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
	originalShutdownRequested := activeApp.shutdownRequested.Load()

	var received []string
	var receivedTitles []string
	activeApp.appHWND = 1
	activeApp.trayIconAdded = true
	publishTrayNotification = func(title string, message string, _ uintptr) {
		receivedTitles = append(receivedTitles, title)
		received = append(received, message)
	}
	applyDisplayLanguagePreference("tr-TR")
	activeApp.appStatus.Store(AppStatusStarting)
	activeApp.configurationStatus.Store(ConfigStatusUnknown)
	activeApp.updateStatus.Store(UpdateStatusUnknown)
	activeApp.shutdownRequested.Store(false)
	clearPendingTrayNotifications()

	t.Cleanup(func() {
		activeApp.appHWND = originalAppHWND
		activeApp.trayIconAdded = originalTrayIconAdded
		publishTrayNotification = originalPublisher
		activeApp.appStatus.Store(originalStatus)
		activeApp.configurationStatus.Store(originalConfigStatus)
		activeApp.updateStatus.Store(originalUpdateStatus)
		activeApp.shutdownRequested.Store(originalShutdownRequested)
		applyDisplayLanguagePreference(originalPreference)
		clearPendingTrayNotifications()
	})

	setAppStatus(AppStatusWaitingForGame)
	flushTrayNotifications()
	if len(receivedTitles) != 1 || !strings.Contains(receivedTitles[0], "TaskBarHero") {
		t.Fatalf("WaitingForGame notification title = %q", receivedTitles)
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
	allNotificationText := strings.Join(append(receivedTitles, received...), "\n")
	if strings.Contains(allNotificationText, "diagnostic detail") {
		t.Fatalf("technical diagnostics leaked into tray notification: %q", allNotificationText)
	}
}

func TestRuntimeNotificationsAreSuppressedDuringShutdown(t *testing.T) {
	originalAppHWND := activeApp.appHWND
	originalTrayIconAdded := activeApp.trayIconAdded
	originalPublisher := publishTrayNotification
	originalStatus := activeApp.appStatus.Load()
	originalShutdownRequested := activeApp.shutdownRequested.Load()

	var received []string
	activeApp.appHWND = 1
	activeApp.trayIconAdded = true
	activeApp.appStatus.Store(AppStatusReady)
	activeApp.shutdownRequested.Store(true)
	publishTrayNotification = func(title string, message string, _ uintptr) {
		received = append(received, title, message)
	}
	clearPendingTrayNotifications()
	t.Cleanup(func() {
		activeApp.appHWND = originalAppHWND
		activeApp.trayIconAdded = originalTrayIconAdded
		publishTrayNotification = originalPublisher
		activeApp.appStatus.Store(originalStatus)
		activeApp.shutdownRequested.Store(originalShutdownRequested)
		clearPendingTrayNotifications()
	})

	setAppStatus(AppStatusWaitingForGame)
	flushTrayNotifications()
	if len(received) != 0 {
		t.Fatalf("shutdown emitted runtime notifications: %q", received)
	}
}

func TestStartupNotificationAndLanguageMenuMapping(t *testing.T) {
	originalAppHWND := activeApp.appHWND
	originalTrayIconAdded := activeApp.trayIconAdded
	originalPublisher := publishTrayNotification
	originalPreference := currentDisplayLanguagePreference()

	var received []string
	var receivedTitles []string
	activeApp.appHWND = 1
	activeApp.trayIconAdded = true
	publishTrayNotification = func(title string, message string, _ uintptr) {
		receivedTitles = append(receivedTitles, title)
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
	if len(receivedTitles) != 1 || receivedTitles[0] != tr("status.starting") {
		t.Fatalf("startup notification title should be %q, got = %q", tr("status.starting"), receivedTitles)
	}

	if received[0] != " " {
		t.Fatalf("startup notification body should be blank, got = %q", received[0])
	}
	if strings.Contains(receivedTitles[0]+received[0], AppName) {
		t.Fatalf("startup notification content should not repeat app name, got title=%q body=%q", receivedTitles[0], received[0])
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
