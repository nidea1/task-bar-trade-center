package main

import (
	"fmt"
	"sync"
)

var appStateDetails = struct {
	sync.RWMutex
	update      string
	downloadURL string
	releaseURL  string
}{}

var trayNotifications = struct {
	sync.Mutex
	pending []trayNotification
	started bool
}{}

type trayNotification struct {
	title   string
	message string
}

var publishTrayNotification = showTrayNotification

func setConfigurationStatus(status int32, detail string) {
	previous := ConfigurationStatus.Swap(status)
	if detail != "" {
		fmt.Printf("[CONFIG:%d] %s\n", status, detail)
	}
	requestStatusRefresh()
	if previous != status && TrayIconAdded && configurationNeedsAction(status) {
		queueTrayNotification(tr("notification.configuration", configurationStatusText()), true)
	}
}

func configurationStatusText() string {
	switch ConfigurationStatus.Load() {
	case ConfigStatusLocalCache:
		return tr("config.local_cache")
	case ConfigStatusEmbedded:
		return tr("config.embedded")
	case ConfigStatusDevelopment:
		return tr("config.development")
	case ConfigStatusRefreshing:
		return tr("config.refreshing")
	case ConfigStatusCurrent:
		return tr("config.current")
	case ConfigStatusRefreshFailed:
		return tr("config.failed")
	default:
		return tr("config.unknown")
	}
}

func setUpdateState(status int32, detail, downloadURL, releaseURL string) {
	previous := UpdateStatus.Swap(status)
	if detail != "" {
		fmt.Printf("[UPDATE:%d] %s\n", status, detail)
	}
	appStateDetails.Lock()
	appStateDetails.update = detail
	if downloadURL != "" || status != UpdateStatusAvailable {
		appStateDetails.downloadURL = downloadURL
	}
	if releaseURL != "" || status != UpdateStatusAvailable {
		appStateDetails.releaseURL = releaseURL
	}
	appStateDetails.Unlock()
	requestStatusRefresh()
	if previous != status && TrayIconAdded && updateNeedsAction(status) {
		queueTrayNotification(tr("notification.update", updateStatusText()), true)
	}
}

func updateStatusText() string {
	appStateDetails.RLock()
	detail := appStateDetails.update
	appStateDetails.RUnlock()
	if UpdateStatus.Load() == UpdateStatusAvailable && detail != "" {
		return detail
	}
	switch UpdateStatus.Load() {
	case UpdateStatusChecking:
		return tr("update.checking")
	case UpdateStatusUpToDate:
		return tr("update.current")
	case UpdateStatusAvailable:
		return tr("update.available", "")
	case UpdateStatusFailed:
		return tr("update.failed")
	case UpdateStatusDownloading:
		return tr("update.downloading")
	default:
		return tr("update.unknown")
	}
}

func updateActionURLs() (downloadURL string, releaseURL string) {
	appStateDetails.RLock()
	defer appStateDetails.RUnlock()
	return appStateDetails.downloadURL, appStateDetails.releaseURL
}

func setElevationError(errText string) {
	if errText != "" && TrayIconAdded {
		queueTrayNotification(tr("notification.admin_restart_failed"), true)
	}
}

func appStatusText() string {
	switch AppStatus.Load() {
	case AppStatusStarting:
		return tr("status.starting")
	case AppStatusWaitingForGame:
		return tr("status.waiting_game")
	case AppStatusWaitingForGameAssembly:
		return tr("status.attaching")
	case AppStatusReady:
		return tr("status.ready")
	case AppStatusAttachFailed:
		return tr("status.admin_required")
	case AppStatusGameLayoutIncompatible:
		return tr("status.layout_incompatible")
	case AppStatusInitializationFailed:
		return tr("status.initialization_failed")
	default:
		return AppName
	}
}

func notifyApplicationStarted() {
	if !TrayIconAdded {
		return
	}
	trayNotifications.Lock()
	if trayNotifications.started {
		trayNotifications.Unlock()
		return
	}
	trayNotifications.started = true
	trayNotifications.Unlock()
	queueRawTrayNotification(tr("status.starting"))
}

func notifyRuntimeStateChange(previous int32, status int32) {
	if previous == status || !TrayIconAdded {
		return
	}
	if runtimeNeedsAction(status) {
		queueTrayNotification(tr("notification.runtime", appStatusText()), true)
		return
	}
	if runtimeShouldNotify(status) {
		queueRawTrayNotification(appStatusText())
	}
}

func runtimeShouldNotify(status int32) bool {
	return status == AppStatusWaitingForGame || status == AppStatusReady
}

func queueTrayNotification(message string, actionRequired bool) {
	if actionRequired {
		message += "\n" + tr("notification.action_required")
	}
	queueRawTrayNotification(message)
}

func queueRawTrayNotification(message string) {
	if AppHWND == 0 || !TrayIconAdded {
		return
	}
	trayNotifications.Lock()
	trayNotifications.pending = append(trayNotifications.pending, trayNotification{title: AppName, message: message})
	trayNotifications.Unlock()
	procPostMessageW.Call(AppHWND, WM_APP_TRAY_NOTIFICATION, 0, 0)
}

func flushTrayNotifications() {
	trayNotifications.Lock()
	pending := trayNotifications.pending
	trayNotifications.pending = nil
	trayNotifications.Unlock()
	for _, notification := range pending {
		publishTrayNotification(notification.title, notification.message)
	}
}

func runtimeNeedsAction(status int32) bool {
	return status == AppStatusAttachFailed || status == AppStatusGameLayoutIncompatible || status == AppStatusInitializationFailed
}

func configurationNeedsAction(status int32) bool {
	return status == ConfigStatusRefreshFailed
}

func updateNeedsAction(status int32) bool {
	return status == UpdateStatusAvailable || status == UpdateStatusFailed
}
