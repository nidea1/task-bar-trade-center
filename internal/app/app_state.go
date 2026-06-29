package app

import (
	"github.com/nidea1/task-bar-trade-center/internal/win32"

	"fmt"
	"strings"
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
	icon    uintptr
}

var publishTrayNotification = showTrayNotification

func setConfigurationStatus(status int32, detail string) {
	previous := activeApp.configurationStatus.Swap(status)
	if detail != "" {
		fmt.Printf("[CONFIG:%d] %s\n", status, detail)
	}
	requestStatusRefresh()
	if previous != status && activeApp.trayIconAdded && configurationNeedsAction(status) {
		queueTrayNotification(tr("notification.configuration", configurationStatusText()), true)
	}
}

func configurationStatusText() string {
	switch activeApp.configurationStatus.Load() {
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
	previous := activeApp.updateStatus.Swap(status)
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
	callDashboardFooterUpdated(GetDashboardFooterInfo())
	if previous != status && status == UpdateStatusAvailable {
		callOpenDashboard()
	}
	if previous != status && activeApp.trayIconAdded && updateNeedsAction(status) {
		queueTrayNotification(tr("notification.update", updateStatusText()), true)
	}
}

func updateStatusText() string {
	appStateDetails.RLock()
	detail := appStateDetails.update
	appStateDetails.RUnlock()
	if activeApp.updateStatus.Load() == UpdateStatusAvailable && detail != "" {
		return detail
	}
	switch activeApp.updateStatus.Load() {
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
	if errText != "" && activeApp.trayIconAdded {
		queueTrayNotification(tr("notification.admin_restart_failed"), true)
	}
}

func appStatusText() string {
	switch activeApp.appStatus.Load() {
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
	if !activeApp.trayIconAdded {
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
	if previous == status || !activeApp.trayIconAdded || activeApp.shutdownRequested.Load() {
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
	queueRawTrayNotificationWithIcon(message, 0)
}

func queueRawTrayNotificationWithIcon(message string, icon uintptr) {
	if activeApp.appHWND == 0 || !activeApp.trayIconAdded {
		return
	}
	title, body := trayNotificationContent(message)
	trayNotifications.Lock()
	trayNotifications.pending = append(trayNotifications.pending, trayNotification{title: title, message: body, icon: icon})
	trayNotifications.Unlock()
	win32.ProcPostMessageW.Call(activeApp.appHWND, WM_APP_TRAY_NOTIFICATION, 0, 0)
}

func trayNotificationContent(message string) (string, string) {
	title, body, hasBody := strings.Cut(message, "\n")
	title = strings.TrimSpace(title)
	body = strings.TrimSpace(body)
	if title == "" {
		title = AppShortName
	}
	if !hasBody || body == "" {
		body = " "
	}
	return title, body
}

func flushTrayNotifications() {
	trayNotifications.Lock()
	pending := trayNotifications.pending
	trayNotifications.pending = nil
	trayNotifications.Unlock()
	for _, notification := range pending {
		publishTrayNotification(notification.title, notification.message, notification.icon)
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
