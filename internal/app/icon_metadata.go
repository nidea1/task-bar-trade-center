package app

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	filestore "github.com/nidea1/task-bar-trade-center/internal/storage"
)

const iconMetadataTTL = 7 * 24 * time.Hour

type iconMetadataEntry struct {
	IconPath      string    `json:"icon_path"`
	LastFetchedAt time.Time `json:"last_fetched_at"`
}

func loadIconMetadataFromDisk() int {
	if activeApp.iconMetadataFilePath == "" {
		return 0
	}

	bytes, err := os.ReadFile(activeApp.iconMetadataFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			activeApp.iconMetadataMu.Lock()
			writeIconMetadataFileLocked()
			activeApp.iconMetadataMu.Unlock()
			fmt.Println("Icon metadata cache file created.")
		} else {
			fmt.Printf("Icon metadata cache file could not be read: %v\n", err)
		}
		return 0
	}

	var diskCache map[string]iconMetadataEntry
	if err := json.Unmarshal(bytes, &diskCache); err != nil {
		fmt.Printf("Icon metadata cache file could not be parsed: %v\n", err)
		return 0
	}

	activeApp.iconMetadataMu.Lock()
	if activeApp.iconMetadata == nil {
		activeApp.iconMetadata = make(map[string]iconMetadataEntry)
	}
	for marketHashName, entry := range diskCache {
		if marketHashName == "" || entry.IconPath == "" {
			continue
		}
		activeApp.iconMetadata[marketHashName] = entry
	}
	count := len(activeApp.iconMetadata)
	activeApp.iconMetadataMu.Unlock()

	fmt.Printf("Icon metadata cache loaded from disk: %d item(s).\n", count)
	return count
}

func writeIconMetadataFileLocked() {
	if activeApp.iconMetadataFilePath == "" {
		return
	}
	if activeApp.iconMetadata == nil {
		activeApp.iconMetadata = make(map[string]iconMetadataEntry)
	}
	if err := filestore.WriteJSON(activeApp.iconMetadataFilePath, activeApp.iconMetadata); err != nil {
		fmt.Printf("Icon metadata cache file could not be written: %v\n", err)
	}
}

func scheduleIconMetadataWriteLocked() {
	if activeApp.iconMetadataFilePath == "" {
		return
	}

	activeApp.iconMetadataWriteMu.Lock()
	activeApp.iconMetadataWritePending = true
	if activeApp.iconMetadataWriteTimer == nil {
		activeApp.iconMetadataWriteTimer = time.AfterFunc(cacheWriteDebounce, flushScheduledIconMetadataWrite)
	} else {
		activeApp.iconMetadataWriteTimer.Reset(cacheWriteDebounce)
	}
	activeApp.iconMetadataWriteMu.Unlock()
}

func flushScheduledIconMetadataWrite() {
	activeApp.iconMetadataWriteMu.Lock()
	activeApp.iconMetadataWriteTimer = nil
	pending := activeApp.iconMetadataWritePending
	activeApp.iconMetadataWritePending = false
	activeApp.iconMetadataWriteMu.Unlock()

	if !pending {
		return
	}
	activeApp.iconMetadataMu.Lock()
	writeIconMetadataFileLocked()
	activeApp.iconMetadataMu.Unlock()
}

func flushIconMetadataWriteNow() {
	activeApp.iconMetadataWriteMu.Lock()
	if activeApp.iconMetadataWriteTimer != nil {
		activeApp.iconMetadataWriteTimer.Stop()
		activeApp.iconMetadataWriteTimer = nil
	}
	pending := activeApp.iconMetadataWritePending
	activeApp.iconMetadataWritePending = false
	activeApp.iconMetadataWriteMu.Unlock()

	if !pending {
		return
	}
	activeApp.iconMetadataMu.Lock()
	writeIconMetadataFileLocked()
	activeApp.iconMetadataMu.Unlock()
}

func recordMarketIcon(marketHashName string, iconPath string, fetchedAt time.Time) {
	if marketHashName == "" || iconPath == "" {
		return
	}
	if fetchedAt.IsZero() {
		fetchedAt = time.Now()
	}
	activeApp.iconMetadataMu.Lock()
	if activeApp.iconMetadata == nil {
		activeApp.iconMetadata = make(map[string]iconMetadataEntry)
	}
	existing := activeApp.iconMetadata[marketHashName]
	if existing.IconPath == iconPath && !existing.LastFetchedAt.IsZero() && fetchedAt.Sub(existing.LastFetchedAt) < time.Minute {
		activeApp.iconMetadataMu.Unlock()
		return
	}
	activeApp.iconMetadata[marketHashName] = iconMetadataEntry{
		IconPath:      iconPath,
		LastFetchedAt: fetchedAt,
	}
	scheduleIconMetadataWriteLocked()
	activeApp.iconMetadataMu.Unlock()
	queueNotificationIconPrepare(iconPath)
}

func marketIconPath(marketHashName string) (string, bool) {
	activeApp.iconMetadataMu.RLock()
	defer activeApp.iconMetadataMu.RUnlock()
	entry, exists := activeApp.iconMetadata[marketHashName]
	if !exists || entry.IconPath == "" {
		return "", false
	}
	return entry.IconPath, true
}

func marketIconMetadataFresh(marketHashName string, now time.Time) bool {
	activeApp.iconMetadataMu.RLock()
	defer activeApp.iconMetadataMu.RUnlock()
	entry, exists := activeApp.iconMetadata[marketHashName]
	if !exists || entry.IconPath == "" || entry.LastFetchedAt.IsZero() {
		return false
	}
	return now.Sub(entry.LastFetchedAt) < iconMetadataTTL
}

func steamIconImageURL(iconPath string) string {
	if iconPath == "" {
		return ""
	}
	return "https://community.cloudflare.steamstatic.com/economy/image/" + iconPath
}
