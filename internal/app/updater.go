package app

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"
	"unsafe"

	updatecore "github.com/nidea1/task-bar-trade-center/internal/updater"
)

type GitHubAsset = updatecore.Asset
type GitHubRelease = updatecore.Release

var githubReleaseURL = "https://api.github.com/repos/nidea1/task-bar-trade-center/releases/latest"

const (
	userAgent                     = "TaskBarTradeCenter-Updater"
	restartAfterUpdateArgument    = "--restart-after-update"
	restartAfterElevationArgument = "--restart-after-elevation"
)

var (
	startExecutableProcess = func(executablePath string, args ...string) error {
		return exec.Command(executablePath, args...).Start()
	}
	waitForUpdateParentExit = waitForProcessExit
	launchElevatedRestart   = launchElevatedRestartProcess
)

// cleanOldVersion deletes the temporary .old file left behind from self-update.
func cleanOldVersion() {
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	oldPath := exePath + ".old"
	if _, err := os.Stat(oldPath); err != nil {
		return
	}

	go func() {
		// Retry up to 5 times to let the OS release file locks on the terminated old process
		for i := 0; i < 5; i++ {
			time.Sleep(1 * time.Second)
			err := os.Remove(oldPath)
			if err == nil {
				fmt.Printf("Successfully removed old version executable: %s\n", oldPath)
				return
			}
		}
		fmt.Printf("Could not remove old version executable: %s\n", oldPath)
	}()
}

// checkUpdatesOnStartup runs a silent, delayed background check on app startup.
func checkUpdatesOnStartup() {
	time.Sleep(7 * time.Second) // wait for app to stabilize and load before calling GitHub
	fmt.Println("Running startup update check...")
	checkForUpdates(true)
}

// runManualUpdateCheck runs a foreground check triggered by the user from the tray.
func runManualUpdateCheck() {
	fmt.Println("Running manual update check...")
	checkForUpdates(false)
}

// isNewerVersion returns true if latest is semantically newer than current.
func isNewerVersion(current, latest string) bool {
	return updatecore.IsNewerVersion(current, latest)
}

// checkForUpdates checks for updates on GitHub.
// if silent is true, it only prompts if an update is available.
// if silent is false, it prompts for update or shows "Up to date" or error dialog.
func checkForUpdates(silent bool) {
	setUpdateState(UpdateStatusChecking, "", "", "")
	client := &http.Client{Timeout: 10 * time.Second}
	release, statusCode, err := updatecore.FetchLatestRelease(githubReleaseURL, client, userAgent)
	if err != nil {
		if statusCode == http.StatusNotFound {
			setUpdateState(UpdateStatusUpToDate, "", "", "")
			return
		}
		setUpdateState(UpdateStatusFailed, err.Error(), "", "")
		return
	}

	if !isNewerVersion(AppVersion, release.TagName) {
		setUpdateState(UpdateStatusUpToDate, "", "", release.HTMLURL)
		return
	}

	// Store the action instead of interrupting startup with a modal prompt.
	exeAsset, ok := updatecore.ExecutableAsset(release)
	if !ok {
		setUpdateState(UpdateStatusFailed, "No installable executable was found for the available update.", "", release.HTMLURL)
		return
	}

	setUpdateState(UpdateStatusAvailable, tr("update.available", release.TagName), exeAsset.BrowserDownloadURL, release.HTMLURL)
}

func installAvailableUpdate() {
	downloadURL, releaseURL := updateActionURLs()
	if downloadURL == "" {
		return
	}
	if UpdateStatus.Load() != UpdateStatusAvailable {
		return
	}
	setUpdateState(UpdateStatusDownloading, "", downloadURL, releaseURL)
	go performUpdate(downloadURL, releaseURL)
}

func performUpdate(downloadURL, releasePageURL string) {
	setUpdateState(UpdateStatusDownloading, "", downloadURL, releasePageURL)

	err := downloadAndSwapExecutable(downloadURL)
	if err != nil {
		fmt.Printf("Update failed: %v\n", err)
		setUpdateState(UpdateStatusFailed, err.Error(), "", releasePageURL)
		return
	}

	// Start the replacement executable in helper mode before shutting down. The
	// helper waits for this process to release the single-instance mutex.
	exePath, err := os.Executable()
	if err != nil {
		setUpdateState(UpdateStatusFailed, err.Error(), "", releasePageURL)
		return
	}
	if err := startRestartAfterUpdateHelper(exePath, uint32(os.Getpid())); err != nil {
		setUpdateState(UpdateStatusFailed, err.Error(), "", releasePageURL)
		return
	}

	requestAppShutdown()
}

func startRestartAfterUpdateHelper(executablePath string, parentPID uint32) error {
	return startExecutableProcess(executablePath, restartAfterUpdateArgument, strconv.FormatUint(uint64(parentPID), 10))
}

func runRestartAfterUpdateHelper() bool {
	if len(os.Args) != 3 || os.Args[1] != restartAfterUpdateArgument {
		return false
	}

	parentPID, err := strconv.ParseUint(os.Args[2], 10, 32)
	if err != nil || parentPID == 0 {
		showErrorMessageBox(tr("dialog.update_restart_failed.title"), tr("dialog.update_restart_failed.invalid"))
		return true
	}

	executablePath, err := os.Executable()
	if err != nil {
		showErrorMessageBox(tr("dialog.update_restart_failed.title"), tr("dialog.update_restart_failed.error", err))
		return true
	}
	if err := restartUpdatedApplication(uint32(parentPID), executablePath); err != nil {
		showErrorMessageBox(tr("dialog.update_restart_failed.title"), tr("dialog.update_restart_failed.error", err))
	}
	return true
}

func requestElevatedRestart() {
	executablePath, err := os.Executable()
	if err != nil {
		setElevationError(err.Error())
		return
	}
	if err := launchElevatedRestart(executablePath, uint32(os.Getpid())); err != nil {
		setElevationError(err.Error())
		return
	}
	setElevationError("")
	requestAppShutdown()
}

func launchElevatedRestartProcess(executablePath string, parentPID uint32) error {
	operation, _ := syscall.UTF16PtrFromString("runas")
	executable, _ := syscall.UTF16PtrFromString(executablePath)
	arguments, _ := syscall.UTF16PtrFromString(restartAfterElevationArgument + " " + strconv.FormatUint(uint64(parentPID), 10))
	result, _, _ := procShellExecuteW.Call(
		AppHWND,
		uintptr(unsafe.Pointer(operation)),
		uintptr(unsafe.Pointer(executable)),
		uintptr(unsafe.Pointer(arguments)),
		0,
		SW_SHOWDEFAULT,
	)
	if result <= 32 {
		return fmt.Errorf("administrator restart was cancelled or could not be started (ShellExecute=%d)", result)
	}
	return nil
}

func runRestartAfterElevationHelper() bool {
	if len(os.Args) != 3 || os.Args[1] != restartAfterElevationArgument {
		return false
	}

	parentPID, err := strconv.ParseUint(os.Args[2], 10, 32)
	if err != nil || parentPID == 0 {
		showErrorMessageBox(tr("dialog.restart_failed.title"), tr("dialog.restart_failed.invalid"))
		return true
	}
	executablePath, err := os.Executable()
	if err != nil {
		showErrorMessageBox(tr("dialog.restart_failed.title"), tr("dialog.restart_failed.error", err))
		return true
	}
	if err := restartUpdatedApplication(uint32(parentPID), executablePath); err != nil {
		showErrorMessageBox(tr("dialog.restart_failed.title"), tr("dialog.restart_failed.error", err))
	}
	return true
}

func restartUpdatedApplication(parentPID uint32, executablePath string) error {
	waitForUpdateParentExit(parentPID)
	return startExecutableProcess(executablePath)
}

func downloadAndSwapExecutable(downloadURL string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	client := &http.Client{Timeout: 60 * time.Second}
	return updatecore.DownloadAndSwapExecutable(downloadURL, client, userAgent, exePath)
}

var (
	showYesNoMessageBoxMock func(title, message string) bool
	showInfoMessageBoxMock  func(title, message string)
	showErrorMessageBoxMock func(title, message string)
)

func showYesNoMessageBox(title, message string) bool {
	if showYesNoMessageBoxMock != nil {
		return showYesNoMessageBoxMock(title, message)
	}
	tPtr, _ := syscall.UTF16PtrFromString(title)
	mPtr, _ := syscall.UTF16PtrFromString(message)
	// MB_YESNO (0x4) | MB_ICONINFORMATION (0x40)
	ret, _, _ := procMessageBoxW.Call(messageBoxOwner(), uintptr(unsafe.Pointer(mPtr)), uintptr(unsafe.Pointer(tPtr)), 0x00000004|0x00000040)
	return ret == 6 // IDYES is 6
}

func showInfoMessageBox(title, message string) {
	if showInfoMessageBoxMock != nil {
		showInfoMessageBoxMock(title, message)
		return
	}
	tPtr, _ := syscall.UTF16PtrFromString(title)
	mPtr, _ := syscall.UTF16PtrFromString(message)
	// MB_OK (0x0) | MB_ICONINFORMATION (0x40)
	procMessageBoxW.Call(messageBoxOwner(), uintptr(unsafe.Pointer(mPtr)), uintptr(unsafe.Pointer(tPtr)), 0x00000000|0x00000040)
}

func showErrorMessageBox(title, message string) {
	if showErrorMessageBoxMock != nil {
		showErrorMessageBoxMock(title, message)
		return
	}
	tPtr, _ := syscall.UTF16PtrFromString(title)
	mPtr, _ := syscall.UTF16PtrFromString(message)
	// MB_OK (0x0) | MB_ICONERROR (0x10)
	procMessageBoxW.Call(messageBoxOwner(), uintptr(unsafe.Pointer(mPtr)), uintptr(unsafe.Pointer(tPtr)), 0x00000000|0x00000010)
}

func messageBoxOwner() uintptr {
	return AppHWND
}
