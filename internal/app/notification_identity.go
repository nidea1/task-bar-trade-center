package app

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/nidea1/task-bar-trade-center/internal/win32"
	"golang.org/x/sys/windows/registry"
)

//go:embed icon.ico
var defaultIconBytes []byte

func configureNotificationIdentity() {
	appID, _ := syscall.UTF16PtrFromString(AppUserModelID)
	if result, _, _ := win32.ProcSetCurrentProcessExplicitAppUserModelID.Call(uintptr(unsafe.Pointer(appID))); result != 0 {
		fmt.Printf("Could not set notification app identity: HRESULT 0x%X\n", result)
		return
	}

	iconPath, err := resolveNotificationIconPath()
	if err != nil {
		fmt.Printf("Could not resolve notification icon path: %v\n", err)
		return
	}

	if err := registerNotificationIdentity(iconPath); err != nil {
		fmt.Printf("Could not register notification app identity: %v\n", err)
	}
}

func resolveNotificationIconPath() (string, error) {
	// First, check if there's a local assets folder (development mode)
	executablePath, err := os.Executable()
	if err == nil {
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
				return absolute, nil
			}
		}
	}

	// Otherwise, write the embedded icon to %LOCALAPPDATA%\[AppName]\icon.ico
	baseDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("user cache dir: %w", err)
	}
	appDataDir := filepath.Join(baseDir, AppName)
	if err := os.MkdirAll(appDataDir, 0700); err != nil {
		return "", fmt.Errorf("mkdir app data dir: %w", err)
	}
	iconPath := filepath.Join(appDataDir, "icon.ico")

	// Write the embedded icon bytes if they differ or if file doesn't exist
	writeNeeded := true
	if info, err := os.Stat(iconPath); err == nil {
		if info.Size() == int64(len(defaultIconBytes)) {
			writeNeeded = false
		}
	}
	if writeNeeded {
		if err := os.WriteFile(iconPath, defaultIconBytes, 0600); err != nil {
			return "", fmt.Errorf("write icon file: %w", err)
		}
	}
	return iconPath, nil
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
