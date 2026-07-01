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

const inventoryResolverCacheVersion = 1

type inventoryResolverDiskCache struct {
	Version           int                  `json:"version"`
	GameAssemblyPath  string               `json:"game_assembly_path"`
	GameAssemblySize  int64                `json:"game_assembly_size"`
	GameAssemblyModNS int64                `json:"game_assembly_mod_ns"`
	ClassOffsets      map[string][]uintptr `json:"class_offsets"`
	UpdatedAt         string               `json:"updated_at"`
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

	imported := resolverCacheFromDisk(cache, gameAssemblyBase)
	if len(imported.ClassCache) == 0 {
		return
	}
	resolver := currentInventoryResolver()
	resolver.ImportCache(imported)
	logPrintf("[INVENTORY:cache] Loaded persistent resolver cache: classes=%d\n", len(imported.ClassCache))
}

func saveInventoryResolverCache(processHandle uintptr, gameAssemblyBase uintptr, resolver *playerdata.Resolver) {
	if resolver == nil || activeApp.inventoryResolverCacheFilePath == "" || gameAssemblyBase == 0 {
		return
	}
	fingerprint, ok := gameAssemblyFingerprint(processHandle)
	if !ok {
		return
	}
	cache := inventoryResolverDiskCache{
		Version:           inventoryResolverCacheVersion,
		GameAssemblyPath:  fingerprint.path,
		GameAssemblySize:  fingerprint.size,
		GameAssemblyModNS: fingerprint.modNS,
		ClassOffsets:      classOffsetsForDisk(resolver.ExportCache(), gameAssemblyBase),
		UpdatedAt:         time.Now().Format(time.RFC3339),
	}
	if len(cache.ClassOffsets) == 0 {
		return
	}
	if err := filestore.WriteJSONAtomic(activeApp.inventoryResolverCacheFilePath, cache); err != nil {
		logPrintf("[INVENTORY:cache] Persistent resolver cache write failed: %v\n", err)
		return
	}
	logPrintf("[INVENTORY:cache] Saved persistent resolver cache: classes=%d\n", len(cache.ClassOffsets))
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
