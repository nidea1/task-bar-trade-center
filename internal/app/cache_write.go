package app

import "time"

const cacheWriteDebounce = 1500 * time.Millisecond

func schedulePriceCacheWriteLocked() {
	if activeApp.priceCacheFilePath == "" {
		return
	}

	activeApp.priceCacheWriteMu.Lock()
	activeApp.priceCacheWritePending = true
	if activeApp.priceCacheWriteTimer == nil {
		activeApp.priceCacheWriteTimer = time.AfterFunc(cacheWriteDebounce, flushScheduledPriceCacheWrite)
	} else {
		activeApp.priceCacheWriteTimer.Reset(cacheWriteDebounce)
	}
	activeApp.priceCacheWriteMu.Unlock()
}

func flushScheduledPriceCacheWrite() {
	activeApp.priceCacheWriteMu.Lock()
	activeApp.priceCacheWriteTimer = nil
	pending := activeApp.priceCacheWritePending
	activeApp.priceCacheWritePending = false
	activeApp.priceCacheWriteMu.Unlock()

	if !pending {
		return
	}
	activeApp.priceCacheMu.Lock()
	writePriceCacheFileLocked()
	activeApp.priceCacheMu.Unlock()
}

func flushPriceCacheWriteNow() {
	activeApp.priceCacheWriteMu.Lock()
	if activeApp.priceCacheWriteTimer != nil {
		activeApp.priceCacheWriteTimer.Stop()
		activeApp.priceCacheWriteTimer = nil
	}
	pending := activeApp.priceCacheWritePending
	activeApp.priceCacheWritePending = false
	activeApp.priceCacheWriteMu.Unlock()

	if !pending {
		return
	}
	activeApp.priceCacheMu.Lock()
	writePriceCacheFileLocked()
	activeApp.priceCacheMu.Unlock()
}

func flushCacheWritesNow() {
	flushPriceCacheWriteNow()
	flushIconMetadataWriteNow()
}
