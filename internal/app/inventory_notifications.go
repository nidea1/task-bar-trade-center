package app

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/market"
	"github.com/nidea1/task-bar-trade-center/internal/playerdata"
	"github.com/nidea1/task-bar-trade-center/internal/winapp"
)

type marketableInventoryItem struct {
	itemID   int
	name     string
	rarity   string
	price    string
	hasPrice bool
}

var fetchUncachedItemPrice func(context.Context, int) error

func init() {
	fetchUncachedItemPrice = fetchInventoryMarketPrice
}

func recordMarketableInventoryItems(snapshot playerdata.InventorySnapshot) []marketableInventoryItem {
	activeApp.inventoryMu.Lock()

	if activeApp.marketableInventorySeen == nil {
		activeApp.marketableInventorySeen = make(map[uint64]struct{})
	}

	newItems := make([]marketableInventoryItem, 0)
	for _, item := range snapshot.Items {
		if item.UniqueID == 0 {
			continue
		}
		if _, seen := activeApp.marketableInventorySeen[item.UniqueID]; seen {
			continue
		}
		activeApp.marketableInventorySeen[item.UniqueID] = struct{}{}
		if !item.Marketable {
			logPrintf("[NOTIFY] Ignored new item ID %d (UniqueID: %d) because it is not marketable\n", item.ItemID, item.UniqueID)
			continue
		}
		if activeApp.marketableInventorySeeded {
			notifiedBoxItemsMu.Lock()
			tracker := recentlyNotifiedBoxItems[item.ItemID]
			if tracker.count > 0 && time.Since(tracker.lastNotified) < directInventoryNotificationWindow {
				logPrintf("[NOTIFY] Skipping duplicate notification for itemID=%d, UniqueID=%d (already notified %d time(s) via LogManager)\n", item.ItemID, item.UniqueID, tracker.count)
				tracker.count--
				recentlyNotifiedBoxItems[item.ItemID] = tracker
				notifiedBoxItemsMu.Unlock()
				continue
			}
			notifiedBoxItemsMu.Unlock()

			logPrintf("[NOTIFY] Detected new marketable item: itemID=%d, UniqueID=%d\n", item.ItemID, item.UniqueID)
			newItems = append(newItems, marketableInventoryItem{
				itemID: item.ItemID,
			})
		}
	}

	if !activeApp.marketableInventorySeeded {
		activeApp.marketableInventorySeeded = true
		logPrintf("[NOTIFY] Seeding inventory notification tracker with %d item(s)\n", len(snapshot.Items))
		activeApp.inventoryMu.Unlock()
		return nil
	}

	activeApp.inventoryMu.Unlock()
	for index := range newItems {
		fillMarketableInventoryItemDetails(&newItems[index])
	}
	return newItems
}

func resetMarketableInventoryNotifications() {
	activeApp.inventoryMu.Lock()
	activeApp.marketableInventorySeen = nil
	activeApp.marketableInventorySeeded = false
	activeApp.inventoryMu.Unlock()
}

func processNewMarketableInventoryItems(newItems []marketableInventoryItem) {
	// Filter based on rarity setting
	filtered := make([]marketableInventoryItem, 0, len(newItems))
	for _, item := range newItems {
		if shouldNotifyItem(item.itemID) {
			filtered = append(filtered, item)
		} else {
			logPrintf("[NOTIFY] Item ID %d skipped by rarity notification filter.\n", item.itemID)
		}
	}

	if len(filtered) == 0 {
		return
	}
	newItems = filtered

	logPrintf("[NOTIFY] Processing %d new marketable item(s)\n", len(newItems))

	cachedItems := make([]marketableInventoryItem, 0, len(newItems))
	uncachedItems := make([]marketableInventoryItem, 0, len(newItems))

	for _, item := range newItems {
		if item.hasPrice {
			logPrintf("[NOTIFY] Item %s (ID: %d) has cached price: %s. Notifying immediately.\n", item.name, item.itemID, item.price)
			cachedItems = append(cachedItems, item)
		} else {
			logPrintf("[NOTIFY] Item %s (ID: %d) has no cached price. Spawning async fetch.\n", item.name, item.itemID)
			uncachedItems = append(uncachedItems, item)
		}
	}

	if len(cachedItems) > 0 {
		notifyMarketableInventoryItems(cachedItems)
	}

	if len(uncachedItems) > 0 {
		go func(items []marketableInventoryItem) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			logPrintf("[NOTIFY] Starting async price fetch for %d item(s)\n", len(items))
			queue := currentInventoryPriceQueue()
			for i := range items {
				if ctx.Err() != nil {
					logPrintf("[NOTIFY] Price fetch context error for item ID %d: %v\n", items[i].itemID, ctx.Err())
					break
				}
				if queue != nil {
					backoffUntil := queue.BackoffUntil()
					if !backoffUntil.IsZero() && time.Now().Before(backoffUntil) {
						logPrintf("[NOTIFY] Skipping price fetch for item ID %d because queue is in backoff until %s\n", items[i].itemID, backoffUntil.Format(time.RFC3339))
						fillMarketableInventoryItemDetails(&items[i])
						continue
					}
				}
				logPrintf("[NOTIFY] Fetching price for item ID %d...\n", items[i].itemID)
				err := fetchUncachedItemPrice(ctx, items[i].itemID)
				if err != nil {
					logPrintf("[NOTIFY] Price fetch failed for item ID %d: %v\n", items[i].itemID, err)
					if queue != nil {
						queue.TriggerBackoff(items[i].itemID, err)
					}
				}
				fillMarketableInventoryItemDetails(&items[i])
				logPrintf("[NOTIFY] Fetch completed for item ID %d. price=%s hasPrice=%t\n", items[i].itemID, items[i].price, items[i].hasPrice)
			}
			notifyMarketableInventoryItems(items)
		}(uncachedItems)
	}
}

func notifyMarketableInventoryItems(items []marketableInventoryItem) {
	if len(items) == 0 {
		return
	}
	logPrintf("[NOTIFY] Queuing tray notification for %d item(s)\n", len(items))
	if len(items) == 1 {
		item := items[0]
		title := tr("notification.item_acquired_title", item.name)
		if title == "" || title == "notification.item_acquired_title" {
			title = fmt.Sprintf("%s Acquired", item.name)
		}
		queueRawTrayNotificationWithIcon(
			fmt.Sprintf("%s\n%s", title, tr("notification.marketable_item_acquired_body", item.rarity, item.price)),
			inventoryNotificationItemIcon(item.itemID),
		)
		return
	}
	queueRawTrayNotificationWithIcon(
		fmt.Sprintf(
			"%s\n%s",
			tr("notification.marketable_items_acquired"),
			tr("notification.marketable_items_acquired_body", len(items), marketableInventoryItemSummary(items)),
		),
		activeApp.appIconSmall,
	)
}

func fillMarketableInventoryItemDetails(item *marketableInventoryItem) {
	item.name = inventoryNotificationItemName(item.itemID)
	item.rarity = inventoryNotificationItemRarity(item.itemID)
	item.price, item.hasPrice = inventoryNotificationItemPrice(item.itemID)
}

func inventoryNotificationItemName(itemID int) string {
	if config, ok := activeApp.allItemMap[itemID]; ok {
		lang := currentDisplayLanguage()
		if name := config.Name[lang]; name != "" {
			return name
		}
		if name := config.Name["en-US"]; name != "" {
			return name
		}
	}
	return fmt.Sprintf("#%d", itemID)
}

func inventoryNotificationItemRarity(itemID int) string {
	config, ok := activeApp.allItemMap[itemID]
	if !ok || config.Grade == "" {
		return tr("value.na")
	}
	if localized := tr("rarity." + config.Grade); localized != "" {
		return localized
	}
	return config.Grade
}

func inventoryNotificationItemPrice(itemID int) (string, bool) {
	config, ok := activeApp.itemMap[itemID]
	if !ok {
		return tr("notification.price_updating"), false
	}
	scope := market.CurrentScope()
	data, exists := marketCacheEntry(scope, buildMarketHashName(config))
	if !exists || data.Analysis.UpdatedAt.IsZero() {
		return tr("notification.price_updating"), false
	}
	analysis := data.Analysis
	if analysis.HasSuggested {
		return market.FormatAnalysisPrice(analysis.SuggestedPrice, true, analysis), true
	}
	if analysis.HasHighestBuy {
		return market.FormatAnalysisPrice(analysis.HighestBuyPrice, true, analysis), true
	}
	return tr("notification.price_updating"), false
}

func inventoryNotificationItemIcon(itemID int) uintptr {
	config, ok := activeApp.itemMap[itemID]
	if !ok {
		return 0
	}
	marketHashName := buildMarketHashName(config)
	if iconPath, ok := marketIconPath(marketHashName); ok {
		return cachedNotificationIcon(iconPath)
	}
	scope := market.CurrentScope()
	data, exists := marketCacheEntry(scope, marketHashName)
	if !exists || data.Analysis.IconURL == "" {
		return 0
	}
	return cachedNotificationIcon(data.Analysis.IconURL)
}

func cachedNotificationIcon(iconPath string) uintptr {
	activeApp.inventoryMu.Lock()
	if activeApp.notificationIconCache == nil {
		activeApp.notificationIconCache = make(map[string]uintptr)
	}
	if icon := activeApp.notificationIconCache[iconPath]; icon != 0 {
		activeApp.inventoryMu.Unlock()
		return icon
	}

	// Check if the icon is currently being prepared in a background thread
	preparing := false
	if activeApp.notificationIconPreparing != nil {
		_, preparing = activeApp.notificationIconPreparing[iconPath]
	}
	activeApp.inventoryMu.Unlock()

	if preparing {
		// Wait for the background preparation goroutine to complete (up to 5 seconds)
		start := time.Now()
		for time.Since(start) < 5*time.Second {
			time.Sleep(50 * time.Millisecond)
			activeApp.inventoryMu.Lock()
			_, preparing = activeApp.notificationIconPreparing[iconPath]
			activeApp.inventoryMu.Unlock()
			if !preparing {
				break
			}
		}

		// Re-check the cache in case the background thread succeeded
		activeApp.inventoryMu.Lock()
		if icon := activeApp.notificationIconCache[iconPath]; icon != 0 {
			activeApp.inventoryMu.Unlock()
			return icon
		}
		activeApp.inventoryMu.Unlock()
	}

	// Lock and mark as preparing to prevent concurrent duplicate prep/downloads
	activeApp.inventoryMu.Lock()
	if activeApp.notificationIconPreparing == nil {
		activeApp.notificationIconPreparing = make(map[string]struct{})
	}
	if _, stillPreparing := activeApp.notificationIconPreparing[iconPath]; stillPreparing {
		activeApp.inventoryMu.Unlock()
		// Wait again briefly
		start := time.Now()
		for time.Since(start) < 5*time.Second {
			time.Sleep(50 * time.Millisecond)
			activeApp.inventoryMu.Lock()
			_, stillPreparing = activeApp.notificationIconPreparing[iconPath]
			activeApp.inventoryMu.Unlock()
			if !stillPreparing {
				break
			}
		}
		activeApp.inventoryMu.Lock()
		icon := activeApp.notificationIconCache[iconPath]
		activeApp.inventoryMu.Unlock()
		return icon
	}
	activeApp.notificationIconPreparing[iconPath] = struct{}{}
	activeApp.inventoryMu.Unlock()

	defer func() {
		activeApp.inventoryMu.Lock()
		delete(activeApp.notificationIconPreparing, iconPath)
		activeApp.inventoryMu.Unlock()
	}()

	iconFile, ok := notificationIconFile(iconPath)
	if !ok {
		return 0
	}
	icon := winapp.LoadIconFile(iconFile, getSystemMetric(SM_CXICON))
	if icon == 0 {
		return 0
	}

	activeApp.inventoryMu.Lock()
	if activeApp.notificationIconCache == nil {
		activeApp.notificationIconCache = make(map[string]uintptr)
	}
	activeApp.notificationIconCache[iconPath] = icon
	activeApp.inventoryMu.Unlock()
	return icon
}

func queueNotificationIconPrepare(iconPath string) {
	if iconPath == "" {
		return
	}
	activeApp.inventoryMu.Lock()
	if activeApp.notificationIconPreparing == nil {
		activeApp.notificationIconPreparing = make(map[string]struct{})
	}
	if _, preparing := activeApp.notificationIconPreparing[iconPath]; preparing {
		activeApp.inventoryMu.Unlock()
		return
	}
	activeApp.notificationIconPreparing[iconPath] = struct{}{}
	activeApp.inventoryMu.Unlock()

	go func() {
		defer func() {
			activeApp.inventoryMu.Lock()
			delete(activeApp.notificationIconPreparing, iconPath)
			activeApp.inventoryMu.Unlock()
		}()

		iconFile, ok := notificationIconFile(iconPath)
		if !ok {
			return
		}
		icon := winapp.LoadIconFile(iconFile, getSystemMetric(SM_CXICON))
		if icon == 0 {
			return
		}
		activeApp.inventoryMu.Lock()
		if activeApp.notificationIconCache == nil {
			activeApp.notificationIconCache = make(map[string]uintptr)
		}
		activeApp.notificationIconCache[iconPath] = icon
		activeApp.inventoryMu.Unlock()
	}()
}

func notificationIconCachePath(iconPath string) (string, bool) {
	if iconPath == "" {
		return "", false
	}
	cacheDir := activeApp.appDataDir
	if cacheDir == "" {
		cacheDir = os.TempDir()
	}
	iconDir := filepath.Join(cacheDir, "cache", "notification-icons")
	sum := sha1.Sum([]byte(iconPath))
	return filepath.Join(iconDir, hex.EncodeToString(sum[:])+".ico"), true
}

func existingNotificationIconFile(iconPath string) (string, bool) {
	iconFile, ok := notificationIconCachePath(iconPath)
	if !ok {
		return "", false
	}
	if _, err := os.Stat(iconFile); err == nil {
		return iconFile, true
	}
	return "", false
}

func notificationIconFile(iconPath string) (string, bool) {
	iconFile, ok := notificationIconCachePath(iconPath)
	if !ok {
		return "", false
	}
	if _, err := os.Stat(iconFile); err == nil {
		return iconFile, true
	}
	iconDir := filepath.Dir(iconFile)
	if err := os.MkdirAll(iconDir, 0700); err != nil {
		return "", false
	}

	body, ok := downloadNotificationIconPNG(iconPath)
	if !ok {
		return "", false
	}
	ico, ok := pngToICO(body)
	if !ok {
		return "", false
	}
	if err := os.WriteFile(iconFile, ico, 0600); err != nil {
		return "", false
	}
	return iconFile, true
}

func downloadNotificationIconPNG(iconPath string) ([]byte, bool) {
	client := &http.Client{Timeout: 4 * time.Second}
	req, err := http.NewRequest(http.MethodGet, "https://community.cloudflare.steamstatic.com/economy/image/"+iconPath, nil)
	if err != nil {
		return nil, false
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) TaskBarTradeCenter/0.1")
	resp, err := client.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil || len(body) == 0 {
		return nil, false
	}
	return body, true
}

func pngToICO(pngBytes []byte) ([]byte, bool) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(pngBytes))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, false
	}
	width := byte(cfg.Width)
	height := byte(cfg.Height)
	if cfg.Width >= 256 {
		width = 0
	}
	if cfg.Height >= 256 {
		height = 0
	}

	buf := bytes.NewBuffer(make([]byte, 0, len(pngBytes)+22))
	_ = binary.Write(buf, binary.LittleEndian, uint16(0))
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))
	buf.WriteByte(width)
	buf.WriteByte(height)
	buf.WriteByte(0)
	buf.WriteByte(0)
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(buf, binary.LittleEndian, uint16(32))
	_ = binary.Write(buf, binary.LittleEndian, uint32(len(pngBytes)))
	_ = binary.Write(buf, binary.LittleEndian, uint32(22))
	buf.Write(pngBytes)
	return buf.Bytes(), true
}

func queueMarketableInventoryPriceRefresh(items []marketableInventoryItem) {
	ids := make([]int, 0)
	for _, item := range items {
		if item.hasPrice {
			continue
		}
		if _, marketable := activeApp.itemMap[item.itemID]; marketable {
			ids = append(ids, item.itemID)
		}
	}
	if len(ids) > 0 {
		currentInventoryPriceQueue().EnqueuePriority(ids)
	}
}

func marketableInventoryItemSummary(items []marketableInventoryItem) string {
	limit := len(items)
	if limit > 3 {
		limit = 3
	}
	lines := make([]string, 0, limit+1)
	for index := 0; index < limit; index++ {
		item := items[index]
		lines = append(lines, fmt.Sprintf("%s - %s - %s", item.name, item.rarity, item.price))
	}
	if remaining := len(items) - limit; remaining > 0 {
		lines = append(lines, tr("notification.marketable_items_more", remaining))
	}
	return strings.Join(lines, "\n")
}

func shouldNotifyItem(itemID int) bool {
	config, ok := activeApp.allItemMap[itemID]
	if !ok {
		return true
	}
	minLevel := activeApp.minRarityNotifyLevel.Load()
	return int32(rarityLevel(config.Grade)) >= minLevel
}
