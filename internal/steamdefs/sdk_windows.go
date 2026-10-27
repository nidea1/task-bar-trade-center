package steamdefs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/nidea1/task-bar-trade-center/internal/catalog"
	"golang.org/x/sys/windows"
)

var sdkMu sync.Mutex

// Read opens one SDK session for the batch and closes only that session on return.
func Read(ctx context.Context, libraryPath string, ids []int) (map[int]catalog.MarketDefinition, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	sdkMu.Lock()
	defer sdkMu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !filepath.IsAbs(libraryPath) {
		return nil, fmt.Errorf("Steam SDK path must be absolute")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	module, err := windows.LoadLibraryEx(libraryPath, 0, windows.LOAD_LIBRARY_SEARCH_DLL_LOAD_DIR|windows.LOAD_LIBRARY_SEARCH_DEFAULT_DIRS)
	if err != nil {
		return nil, fmt.Errorf("load Steam SDK: %w", err)
	}
	defer windows.FreeLibrary(module)
	procedures := make(map[string]uintptr)
	for _, name := range []string{"SteamAPI_InitFlat", "SteamAPI_Shutdown", "SteamAPI_RunCallbacks", "SteamAPI_SteamInventory_v003", "SteamAPI_SteamUtils_v010", "SteamAPI_ISteamUtils_GetAppID", "SteamAPI_ISteamInventory_LoadItemDefinitions", "SteamAPI_ISteamInventory_GetItemDefinitionIDs", "SteamAPI_ISteamInventory_GetItemDefinitionProperty"} {
		address, err := windows.GetProcAddress(module, name)
		if err != nil {
			return nil, fmt.Errorf("Steam SDK %s: %w", name, err)
		}
		procedures[name] = address
	}
	for _, key := range []string{"SteamAppId", "SteamGameId"} {
		previous, existed := os.LookupEnv(key)
		if err := os.Setenv(key, strconv.Itoa(catalog.SteamAppID)); err != nil {
			return nil, err
		}
		defer func() {
			if existed {
				os.Setenv(key, previous)
			} else {
				os.Unsetenv(key)
			}
		}()
	}
	var message [1024]byte
	result, _, _ := syscall.SyscallN(procedures["SteamAPI_InitFlat"], uintptr(unsafe.Pointer(&message[0])))
	if result != 0 {
		return nil, fmt.Errorf("Steam SDK initialization failed (code %d)", result)
	}
	defer syscall.SyscallN(procedures["SteamAPI_Shutdown"])
	inventory, _, _ := syscall.SyscallN(procedures["SteamAPI_SteamInventory_v003"])
	utils, _, _ := syscall.SyscallN(procedures["SteamAPI_SteamUtils_v010"])
	if inventory == 0 || utils == 0 {
		return nil, fmt.Errorf("Steam SDK interfaces unavailable")
	}
	appID, _, _ := syscall.SyscallN(procedures["SteamAPI_ISteamUtils_GetAppID"], utils)
	if appID != catalog.SteamAppID {
		return nil, fmt.Errorf("Steam SDK app mismatch: got %d, want %d", appID, catalog.SteamAppID)
	}
	loaded, _, _ := syscall.SyscallN(procedures["SteamAPI_ISteamInventory_LoadItemDefinitions"], inventory)
	if loaded == 0 {
		return nil, fmt.Errorf("Steam definition loading failed")
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		syscall.SyscallN(procedures["SteamAPI_RunCallbacks"])
		var count uint32
		ready, _, _ := syscall.SyscallN(procedures["SteamAPI_ISteamInventory_GetItemDefinitionIDs"], inventory, 0, uintptr(unsafe.Pointer(&count)))
		if ready != 0 && count > 0 {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	property := func(id int, key string) (string, bool) {
		name, _ := syscall.BytePtrFromString(key)
		var size uint32
		syscall.SyscallN(procedures["SteamAPI_ISteamInventory_GetItemDefinitionProperty"], inventory, uintptr(id), uintptr(unsafe.Pointer(name)), 0, uintptr(unsafe.Pointer(&size)))
		if size == 0 || size > 64<<10 {
			return "", false
		}
		buffer := make([]byte, size)
		copied, _, _ := syscall.SyscallN(procedures["SteamAPI_ISteamInventory_GetItemDefinitionProperty"], inventory, uintptr(id), uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&size)))
		runtime.KeepAlive(name)
		if copied == 0 || size == 0 || size > uint32(len(buffer)) || buffer[size-1] != 0 {
			return "", false
		}
		return string(buffer[:size-1]), true
	}
	definitions := make(map[int]catalog.MarketDefinition)
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if id <= 0 || id >= 1_000_000_000 {
			continue
		}
		if _, exists := definitions[id]; exists {
			continue
		}
		if definition, ok := readDefinition(id, property); ok {
			definitions[id] = definition
		}
	}
	if len(ids) > 0 && len(definitions) == 0 {
		return nil, fmt.Errorf("Steam returned no usable market definitions")
	}
	return definitions, nil
}
