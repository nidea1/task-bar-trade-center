package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/game"
	"github.com/nidea1/task-bar-trade-center/internal/playerdata"
	filestore "github.com/nidea1/task-bar-trade-center/internal/storage"
)

const inventoryResolverCacheVersion = 2

type inventoryResolverDiskCache struct {
	Version           int                       `json:"version"`
	GameAssemblyPath  string                    `json:"game_assembly_path"`
	GameAssemblySize  int64                     `json:"game_assembly_size"`
	GameAssemblyModNS int64                     `json:"game_assembly_mod_ns"`
	GameProcessID     uint32                    `json:"game_process_id"`
	GameAssemblyBase  uintptr                   `json:"game_assembly_base"`
	CachedObject      uintptr                   `json:"cached_object"`
	ClassCache        map[string][]uintptr      `json:"class_cache"`
	Layout            playerdata.SaveDataLayout `json:"layout"`
	ClassOffsets      map[string][]uintptr      `json:"class_offsets"`
	UpdatedAt         string                    `json:"updated_at"`
}

func configureInventoryResolverCache(processHandle uintptr, gameAssemblyBase uintptr) {
	fingerprint, ok := gameAssemblyFingerprint(processHandle)
	activeApp.gameAssemblyFingerprint = fingerprint.key()
	if !ok {
		logPrintln("[INVENTORY:cache] GameAssembly fingerprint unavailable; persistent resolver cache disabled for this session.")
		return
	}

	cache, ok := readInventoryResolverDiskCache()
	if !ok {
		return
	}
	if !cache.matches(fingerprint) {
		logPrintln("[INVENTORY:cache] Persistent resolver cache ignored: GameAssembly fingerprint changed.")
		return
	}

	resolver := currentInventoryResolver()

	// If the game process matches, we can import absolute pointers (including cachedObject).
	pid := activeApp.gameProcessID
	if cache.GameProcessID == pid && cache.GameAssemblyBase == gameAssemblyBase {
		imported := playerdata.ResolverCache{
			CachedObject: cache.CachedObject,
			ClassCache:   make(map[string][]uintptr),
			Layout:       cache.Layout,
		}
		for name, addresses := range cache.ClassCache {
			imported.ClassCache[name] = append([]uintptr(nil), addresses...)
		}
		resolver.ImportCache(imported)
		logPrintf("[INVENTORY:cache] Loaded persistent resolver cache (same game session): classes=%d cached_object=0x%X\n", len(imported.ClassCache), imported.CachedObject)
		return
	}

	// If the game process restarted, absolute heap addresses are invalid.
	// We fall back to loading class offsets (relative to gameAssemblyBase) if any exist.
	if len(cache.ClassOffsets) > 0 {
		imported := resolverCacheFromDisk(cache, gameAssemblyBase)
		resolver.ImportCache(imported)
		logPrintf("[INVENTORY:cache] Loaded persistent resolver cache relative offsets (new game session): classes=%d\n", len(imported.ClassCache))
	} else {
		logPrintln("[INVENTORY:cache] New game session detected; no relative class offsets found, full scan will be performed.")
	}
}

func saveInventoryResolverCache(processHandle uintptr, gameAssemblyBase uintptr, resolver *playerdata.Resolver) {
	if resolver == nil || activeApp.inventoryResolverCacheFilePath == "" || gameAssemblyBase == 0 {
		return
	}
	fingerprint, ok := gameAssemblyFingerprint(processHandle)
	if !ok {
		return
	}

	export := resolver.ExportCache()

	cache := inventoryResolverDiskCache{
		Version:           inventoryResolverCacheVersion,
		GameAssemblyPath:  fingerprint.path,
		GameAssemblySize:  fingerprint.size,
		GameAssemblyModNS: fingerprint.modNS,
		GameProcessID:     activeApp.gameProcessID,
		GameAssemblyBase:  gameAssemblyBase,
		CachedObject:      export.CachedObject,
		ClassCache:        export.ClassCache,
		Layout:            export.Layout,
		ClassOffsets:      classOffsetsForDisk(export, gameAssemblyBase),
		UpdatedAt:         time.Now().Format(time.RFC3339),
	}

	// Save if we have either absolute class cache or cached object resolved
	if len(cache.ClassCache) == 0 && cache.CachedObject == 0 {
		return
	}

	if err := filestore.WriteJSONAtomic(activeApp.inventoryResolverCacheFilePath, cache); err != nil {
		logPrintf("[INVENTORY:cache] Persistent resolver cache write failed: %v\n", err)
		return
	}
	logPrintf("[INVENTORY:cache] Saved persistent resolver cache: classes=%d cached_object=0x%X\n", len(cache.ClassCache), cache.CachedObject)
}

func readInventoryResolverDiskCache() (inventoryResolverDiskCache, bool) {
	if activeApp.inventoryResolverCacheFilePath == "" {
		return inventoryResolverDiskCache{}, false
	}
	var cache inventoryResolverDiskCache
	if err := filestore.ReadJSON(activeApp.inventoryResolverCacheFilePath, &cache); err != nil {
		if !os.IsNotExist(err) {
			logPrintf("[INVENTORY:cache] Persistent resolver cache read failed: %v\n", err)
		}
		return inventoryResolverDiskCache{}, false
	}
	if cache.Version != inventoryResolverCacheVersion {
		return inventoryResolverDiskCache{}, false
	}
	return cache, true
}

type gameAssemblyCacheFingerprint struct {
	path  string
	size  int64
	modNS int64
}

func (fingerprint gameAssemblyCacheFingerprint) key() string {
	if fingerprint.path == "" {
		return ""
	}
	return fmt.Sprintf("%s|%d|%d", fingerprint.path, fingerprint.size, fingerprint.modNS)
}

func (cache inventoryResolverDiskCache) matches(fingerprint gameAssemblyCacheFingerprint) bool {
	return cache.Version == inventoryResolverCacheVersion &&
		strings.EqualFold(cache.GameAssemblyPath, fingerprint.path) &&
		cache.GameAssemblySize == fingerprint.size &&
		cache.GameAssemblyModNS == fingerprint.modNS
}

func gameAssemblyFingerprint(processHandle uintptr) (gameAssemblyCacheFingerprint, bool) {
	path := strings.TrimSpace(game.ModuleFilePath(processHandle, "GameAssembly.dll"))
	if path == "" {
		return gameAssemblyCacheFingerprint{}, false
	}
	info, err := os.Stat(path)
	if err != nil {
		return gameAssemblyCacheFingerprint{}, false
	}
	absPath, err := filepath.Abs(path)
	if err == nil {
		path = absPath
	}
	return gameAssemblyCacheFingerprint{
		path:  strings.ToLower(path),
		size:  info.Size(),
		modNS: info.ModTime().UnixNano(),
	}, true
}

func resolverCacheFromDisk(cache inventoryResolverDiskCache, gameAssemblyBase uintptr) playerdata.ResolverCache {
	imported := playerdata.ResolverCache{ClassCache: make(map[string][]uintptr, len(cache.ClassOffsets))}
	for name, offsets := range cache.ClassOffsets {
		for _, offset := range offsets {
			if offset == 0 {
				continue
			}
			imported.ClassCache[name] = append(imported.ClassCache[name], gameAssemblyBase+offset)
		}
	}
	return imported
}

func classOffsetsForDisk(cache playerdata.ResolverCache, gameAssemblyBase uintptr) map[string][]uintptr {
	offsets := make(map[string][]uintptr, len(cache.ClassCache))
	for name, addresses := range cache.ClassCache {
		for _, address := range addresses {
			if address <= gameAssemblyBase {
				continue
			}
			offset := address - gameAssemblyBase
			// Class metadata should live near GameAssembly; avoid persisting heap-looking addresses.
			if offset > 2*1024*1024*1024 {
				continue
			}
			offsets[name] = append(offsets[name], offset)
		}
	}
	return offsets
}
