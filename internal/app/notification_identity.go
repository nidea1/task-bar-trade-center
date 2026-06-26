package app

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/nidea1/task-bar-trade-center/internal/win32"
	"golang.org/x/sys/windows/registry"
)

func configureNotificationIdentity() {
	appID, _ := syscall.UTF16PtrFromString(AppUserModelID)
	if result, _, _ := win32.ProcSetCurrentProcessExplicitAppUserModelID.Call(uintptr(unsafe.Pointer(appID))); result != 0 {
		fmt.Printf("Could not set notification app identity: HRESULT 0x%X\n", result)
		return
	}

	executablePath, err := os.Executable()
	if err != nil {
		fmt.Printf("Could not resolve executable path for notification icon: %v\n", err)
		return
	}
	if err := registerNotificationIdentity(notificationIconPath(executablePath)); err != nil {
		fmt.Printf("Could not register notification app identity: %v\n", err)
	}
}

func notificationIconPath(executablePath string) string {
	executableDir := filepath.Dir(executablePath)
	candidates := []string{
		filepath.Join(executableDir, "assets", "icon.ico"),
		filepath.Join(executableDir, "..", "assets", "icon.ico"),
		filepath.Join("assets", "icon.ico"),
	}
	for _, candidate := range candidates {
		absolute, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if _, err := os.Stat(absolute); err == nil {
			return absolute
		}
	}
	return executablePath
}

func registerNotificationIdentity(iconPath string) error {
	key, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		filepath.Join("SOFTWARE", "Classes", "AppUserModelId", AppUserModelID),
		registry.SET_VALUE,
	)
	if err != nil {
		return err
	}
	defer key.Close()

	if err := key.SetStringValue("DisplayName", AppName); err != nil {
		return err
	}
	if err := key.SetStringValue("IconUri", iconPath); err != nil {
		return err
	}
	return nil
}
