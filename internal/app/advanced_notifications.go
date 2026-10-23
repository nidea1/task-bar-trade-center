package app

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/inventory"
	"github.com/nidea1/task-bar-trade-center/internal/playerdata"
)

type targetObservation struct{ goal, count int }
type advancedAlert struct {
	kind                         string
	itemID, count, goal, seconds int
}
type advancedAlertTracker struct {
	lastReadAt     time.Time
	warningValid   bool
	warningSent    bool
	warningMinutes int
	lastETA        int
	region         playerdata.Region
	targets        map[int]targetObservation
}

func ownedItemCounts(snapshot playerdata.InventorySnapshot) map[int]int {
	counts := map[int]int{}
	seen := map[uint64]bool{}
	for _, item := range snapshot.Items {
		if item.UniqueID == 0 || seen[item.UniqueID] {
			continue
		}
		seen[item.UniqueID] = true
		counts[item.ItemID] += item.Units()
	}
	return counts
}

func (tracker *advancedAlertTracker) evaluate(snapshot playerdata.InventorySnapshot, settings DashboardSettings, estimate inventory.InventoryFillEstimate) []advancedAlert {
	if !snapshot.ReadAt.IsZero() && snapshot.ReadAt.Before(tracker.lastReadAt) {
		return nil
	}
	tracker.lastReadAt = snapshot.ReadAt
	var alerts []advancedAlert
	threshold := settings.InventoryWarningMinutes * 60
	valid := threshold > 0 && estimate.Available && estimate.Fills && estimate.TimeToFullSeconds > 0 && (estimate.RuneResolved || estimate.CommonOpenObserved || estimate.StageOpenObserved)
	if tracker.warningMinutes != settings.InventoryWarningMinutes || tracker.region != snapshot.Region {
		tracker.warningValid = false
		tracker.warningSent = false
	}
	if valid {
		if tracker.warningValid && !tracker.warningSent && tracker.lastETA > threshold && estimate.TimeToFullSeconds <= threshold {
			alerts = append(alerts, advancedAlert{kind: "inventory", seconds: estimate.TimeToFullSeconds})
			tracker.warningSent = true
		}
		if estimate.TimeToFullSeconds > threshold+120 {
			tracker.warningSent = false
		}
		tracker.lastETA = estimate.TimeToFullSeconds
	}
	tracker.warningValid = valid
	tracker.warningMinutes = settings.InventoryWarningMinutes
	tracker.region = snapshot.Region
	counts := ownedItemCounts(snapshot)
	next := map[int]targetObservation{}
	for _, target := range settings.NotificationTargets {
		count := counts[target.ItemID]
		previous, seen := tracker.targets[target.ItemID]
		if seen && previous.goal == target.Count && previous.count < target.Count && count >= target.Count {
			alerts = append(alerts, advancedAlert{kind: "target", itemID: target.ItemID, count: count, goal: target.Count})
		}
		next[target.ItemID] = targetObservation{target.Count, count}
	}
	tracker.targets = next
	return alerts
}

var advancedAlerts struct {
	sync.Mutex
	tracker advancedAlertTracker
}

func updateAdvancedNotifications(snapshot playerdata.InventorySnapshot) {
	now := time.Now()
	timing := chestTimingConfigForRegion(snapshot.Region)
	estimate := inventory.BuildInventoryFillEstimateWithTiming(forecastSnapshot(snapshot, now), timing, currentInventoryFillDropScheduleWithTiming(now, timing))
	advancedAlerts.Lock()
	alerts := advancedAlerts.tracker.evaluate(snapshot, currentDashboardSettings(), estimate)
	advancedAlerts.Unlock()
	for _, alert := range alerts {
		var title, body string
		tr := strings.HasPrefix(strings.ToLower(currentDisplayLanguage()), "tr")
		if alert.kind == "inventory" {
			title = "Inventory Filling Soon"
			body = fmt.Sprintf("Estimated time remaining: %d min. Free slots: %d.", (alert.seconds+59)/60, estimate.FreeSlots)
			if tr {
				title = "Envanter Dolmak Üzere"
				body = fmt.Sprintf("Tahmini kalan süre: %d dk. Boş slot: %d.", (alert.seconds+59)/60, estimate.FreeSlots)
			}
		} else {
			title = "Item Target Reached"
			body = fmt.Sprintf("%s: %d/%d", notificationTargetName(alert.itemID), alert.count, alert.goal)
			if tr {
				title = "Eşya Hedefi Tamamlandı"
			}
		}
		queueRawTrayNotification(title + "\n" + body)
	}
}

func resetAdvancedNotifications() {
	advancedAlerts.Lock()
	advancedAlerts.tracker = advancedAlertTracker{}
	advancedAlerts.Unlock()
}

type CatalogItemOption struct {
	ItemID int    `json:"item_id"`
	Name   string `json:"name"`
	Grade  string `json:"grade"`
}

func SearchCatalog(query string) []CatalogItemOption {
	query = strings.ToLower(strings.TrimSpace(query))
	result := []CatalogItemOption{}
	if len([]rune(query)) < 2 {
		return result
	}
	language := currentDisplayLanguage()
	for _, item := range allCatalogItems() {
		name := item.Name[language]
		if name == "" {
			name = item.Name["en-US"]
		}
		if name != "" && (strings.Contains(strings.ToLower(name), query) || strings.Contains(strings.ToLower(item.Name["en-US"]), query) || strings.Contains(fmt.Sprint(item.ID), query)) {
			result = append(result, CatalogItemOption{item.ID, name, item.Grade})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Name == result[j].Name {
			return result[i].ItemID < result[j].ItemID
		}
		return result[i].Name < result[j].Name
	})
	if len(result) > 30 {
		result = result[:30]
	}
	return result
}

func notificationProgress(snapshot playerdata.InventorySnapshot) []inventory.NotificationTargetProgress {
	counts := ownedItemCounts(snapshot)
	result := []inventory.NotificationTargetProgress{}
	for _, target := range currentDashboardSettings().NotificationTargets {
		result = append(result, inventory.NotificationTargetProgress{ItemID: target.ItemID, Name: notificationTargetName(target.ItemID), Count: counts[target.ItemID], Goal: target.Count})
	}
	return result
}

func notificationTargetName(itemID int) string {
	return fmt.Sprintf("%s (%s)", inventoryNotificationItemName(itemID), inventoryNotificationItemRarity(itemID))
}
