package app

import (
	"fmt"
	"strings"

	"github.com/nidea1/task-bar-trade-center/internal/market"
	"github.com/nidea1/task-bar-trade-center/internal/playerdata"
)

type marketableInventoryItem struct {
	itemID   int
	name     string
	rarity   string
	price    string
	hasPrice bool
}

func recordMarketableInventoryItems(snapshot playerdata.InventorySnapshot) []marketableInventoryItem {
	activeApp.inventoryMu.Lock()

	if activeApp.marketableInventorySeen == nil {
		activeApp.marketableInventorySeen = make(map[uint64]struct{})
	}

	newItems := make([]marketableInventoryItem, 0)
	for _, item := range snapshot.Items {
		if item.UniqueID == 0 || !item.Marketable {
			continue
		}
		if _, seen := activeApp.marketableInventorySeen[item.UniqueID]; seen {
			continue
		}
		activeApp.marketableInventorySeen[item.UniqueID] = struct{}{}
		if activeApp.marketableInventorySeeded {
			newItems = append(newItems, marketableInventoryItem{
				itemID: item.ItemID,
			})
		}
	}

	if !activeApp.marketableInventorySeeded {
		activeApp.marketableInventorySeeded = true
		activeApp.inventoryMu.Unlock()
		return nil
	}

	activeApp.inventoryMu.Unlock()
	for index := range newItems {
		fillMarketableInventoryItemDetails(&newItems[index])
	}
	queueMarketableInventoryPriceRefresh(newItems)
	return newItems
}

func resetMarketableInventoryNotifications() {
	activeApp.inventoryMu.Lock()
	activeApp.marketableInventorySeen = nil
	activeApp.marketableInventorySeeded = false
	activeApp.inventoryMu.Unlock()
}

func notifyMarketableInventoryItems(items []marketableInventoryItem) {
	if len(items) == 0 {
		return
	}
	if len(items) == 1 {
		item := items[0]
		queueRawTrayNotification(fmt.Sprintf(
			"%s\n%s",
			tr("notification.marketable_item_acquired"),
			tr("notification.marketable_item_acquired_body", item.name, item.rarity, item.price),
		))
		return
	}
	queueRawTrayNotification(fmt.Sprintf(
		"%s\n%s",
		tr("notification.marketable_items_acquired"),
		tr("notification.marketable_items_acquired_body", len(items), marketableInventoryItemSummary(items)),
	))
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
		currentInventoryPriceQueue().Enqueue(ids)
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
