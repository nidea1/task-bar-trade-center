package app

import (
	"testing"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/catalog"
	"github.com/nidea1/task-bar-trade-center/internal/game"
	"github.com/nidea1/task-bar-trade-center/internal/inventory"
	"github.com/nidea1/task-bar-trade-center/internal/playerdata"
)

func TestInventoryInteractionResultSource(t *testing.T) {
	tests := []struct {
		name   string
		source string
		ok     bool
	}{
		{name: "SynthesisResultLog", source: "synthesis", ok: true},
		{name: "SynthesisResult", source: "synthesis", ok: true},
		{name: "CraftingResultLog", source: "craft", ok: true},
		{name: "CraftingResult", source: "craft", ok: true},
		{name: "CubeResultLog", source: "craft", ok: true},
		{name: "SomeCubeRewardLog", source: "craft", ok: true},
		{name: "CubeDecorationLog", ok: false},
		{name: "CubeEngravingLog", ok: false},
		{name: "CubeInscriptionLog", ok: false},
		{name: "CubeExtractionLog", ok: false},
		{name: "SomeCubeDecorationRewardLog", ok: false},
		{name: "SomeCubeExtractionRewardLog", ok: false},
		{name: "OfferingResultLog", source: "offering", ok: true},
		{name: "OfferingResult", source: "offering", ok: true},
		{name: "BoxOpenLog", ok: false},
		{name: "StageClearLog", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, ok := inventoryInteractionResultSource(tt.name)
			if ok != tt.ok || source != tt.source {
				t.Fatalf("inventoryInteractionResultSource(%q) = %q, %t; want %q, %t", tt.name, source, ok, tt.source, tt.ok)
			}
		})
	}
}

func TestChestDropTrackerResetsOnlyWhenRegionChanges(t *testing.T) {
	previousApp := activeApp
	activeApp = &App{}
	resetChestDropTracker()
	t.Cleanup(func() { activeApp = previousApp; resetChestDropTracker() })
	now := time.Now()
	normal := playerdata.InventorySnapshot{Region: playerdata.RegionNormal, ReadAt: now}
	storeInventorySnapshot(normal)
	recordChestDropLog(getBoxLogMonster, now)
	if s := currentInventoryFillDropSchedule(now); s.NextCommonDropSeconds != 180 || s.NextStageDropSeconds != 300 || !s.CommonDropObserved {
		t.Fatalf("normal schedule=%+v", s)
	}
	normal.ReadAt = now.Add(10 * time.Second)
	normal.StageKey = 1202
	storeInventorySnapshot(normal)
	if s := currentInventoryFillDropSchedule(normal.ReadAt); s.NextCommonDropSeconds != 170 || !s.CommonDropObserved {
		t.Fatalf("same-region stage advance reset countdown: %+v", s)
	}
	plague := normal
	plague.Region = playerdata.RegionPlaguelands
	storeInventorySnapshot(plague)
	if !inventorySnapshotChanged(&normal, &plague) {
		t.Fatal("region switch did not trigger dashboard update")
	}
	if s := currentInventoryFillDropSchedule(plague.ReadAt); s.NextCommonDropSeconds != 240 || s.NextStageDropSeconds != 360 || s.CommonDropObserved || s.StageDropObserved {
		t.Fatalf("plague schedule inherited previous region: %+v", s)
	}
	normal.ReadAt = now.Add(20 * time.Second)
	storeInventorySnapshot(normal)
	if s := currentInventoryFillDropSchedule(normal.ReadAt); s.NextCommonDropSeconds != 180 || s.NextStageDropSeconds != 300 {
		t.Fatalf("return schedule=%+v", s)
	}
}

func TestRequestSavePollPreservesBoxOpenSource(t *testing.T) {
	originalTriggered := triggerSavePollNow
	originalSource := triggerSavePollSource
	originalHasBoxDrop := triggerSavePollHasBoxDrop
	t.Cleanup(func() {
		triggerSavePollNow = originalTriggered
		triggerSavePollSource = originalSource
		triggerSavePollHasBoxDrop = originalHasBoxDrop
	})

	triggerSavePollNow = false
	triggerSavePollSource = ""
	triggerSavePollHasBoxDrop = false
	requestSavePoll("StageFailedLog")
	if !triggerSavePollNow || triggerSavePollSource != "StageFailedLog" {
		t.Fatalf("failed stage poll state = triggered %t source %q", triggerSavePollNow, triggerSavePollSource)
	}

	triggerSavePollNow = false
	triggerSavePollSource = ""
	triggerSavePollHasBoxDrop = false
	requestSavePoll("BoxOpenLog")
	requestSavePoll("GetBoxLog")
	if triggerSavePollSource != "BoxOpenLog" {
		t.Fatalf("box open source = %q, want BoxOpenLog", triggerSavePollSource)
	}
	if !triggerSavePollHasBoxDrop {
		t.Fatal("GetBoxLog should mark the pending save poll as a box drop")
	}
}

func TestChestDropTrackerUpdatesAfterStageClearChestIncrease(t *testing.T) {
	originalAllItemMap := activeApp.allItemMap
	activeApp.allItemMap = map[int]catalog.ItemConfig{
		910011: {ID: 910011, Name: map[string]string{"en-US": "Normal Monster Box 1"}, Type: "STAGEBOX"},
		920001: {ID: 920001, Name: map[string]string{"en-US": "Stage Boss Box 1"}, Type: "STAGEBOX"},
	}
	t.Cleanup(func() {
		activeApp.allItemMap = originalAllItemMap
		resetChestDropTracker()
	})

	resetChestDropTracker()
	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	before := playerdata.InventorySnapshot{}
	after := playerdata.InventorySnapshot{
		Items: []playerdata.OwnedItem{
			{ItemID: 910011, UniqueID: 1, Location: playerdata.LocationInventory},
			{ItemID: 920001, UniqueID: 2, Location: playerdata.LocationInventory},
		},
	}

	updateChestDropTracker(nil, &before, "", now)
	initial := currentInventoryFillDropSchedule(now)
	if initial.CommonDropObserved || initial.StageDropObserved {
		t.Fatalf("initial observed flags = common %t stage %t, want false/false", initial.CommonDropObserved, initial.StageDropObserved)
	}

	updateChestDropTracker(&before, &after, "StageClearLog", now.Add(10*time.Second))
	schedule := currentInventoryFillDropSchedule(now.Add(10 * time.Second))
	if !schedule.CommonDropObserved || !schedule.StageDropObserved {
		t.Fatalf("observed flags = common %t stage %t, want true/true", schedule.CommonDropObserved, schedule.StageDropObserved)
	}
	if schedule.NextCommonDropSeconds != 240 || schedule.NextStageDropSeconds != 360 {
		t.Fatalf("next drops = common %d stage %d, want 240/360", schedule.NextCommonDropSeconds, schedule.NextStageDropSeconds)
	}
}

func TestChestDropTrackerUpdatesFromGetBoxLogMonsterType(t *testing.T) {
	resetChestDropTracker()
	t.Cleanup(resetChestDropTracker)

	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	if !recordChestDropLog(getBoxLogMonster, now) {
		t.Fatal("monster box log should be recorded")
	}
	schedule := currentInventoryFillDropSchedule(now)
	if !schedule.CommonDropObserved || schedule.StageDropObserved {
		t.Fatalf("observed flags after monster box = common %t stage %t, want true/false", schedule.CommonDropObserved, schedule.StageDropObserved)
	}
	if schedule.NextCommonDropSeconds != 240 || schedule.NextStageDropSeconds != 360 {
		t.Fatalf("next drops after monster box = common %d stage %d, want 240/360", schedule.NextCommonDropSeconds, schedule.NextStageDropSeconds)
	}

	if !recordChestDropLog(getBoxLogStageBoss, now.Add(30*time.Second)) {
		t.Fatal("stage boss box log should be recorded")
	}
	schedule = currentInventoryFillDropSchedule(now.Add(30 * time.Second))
	if !schedule.CommonDropObserved || !schedule.StageDropObserved {
		t.Fatalf("observed flags after stage box = common %t stage %t, want true/true", schedule.CommonDropObserved, schedule.StageDropObserved)
	}
	if schedule.NextCommonDropSeconds != 210 || schedule.NextStageDropSeconds != 360 {
		t.Fatalf("next drops after stage box = common %d stage %d, want 210/360", schedule.NextCommonDropSeconds, schedule.NextStageDropSeconds)
	}

	if recordChestDropLog(getBoxLogActBoss, now) {
		t.Fatal("act boss box should not be recorded for inventory fill")
	}
}

func TestChestDropTrackerMarksReadyWithoutAutoAdvance(t *testing.T) {
	resetChestDropTracker()
	t.Cleanup(resetChestDropTracker)

	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	commonChestDropInterval := time.Duration(inventory.DefaultChestTimingConfig().CommonDropSeconds) * time.Second
	if !recordChestDropLog(getBoxLogMonster, now) {
		t.Fatal("monster box log should be recorded")
	}

	schedule := currentInventoryFillDropSchedule(now.Add(commonChestDropInterval + time.Second))
	if !schedule.CommonDropReady || schedule.NextCommonDropSeconds != 0 {
		t.Fatalf("common ready schedule = ready %t next %d, want true/0", schedule.CommonDropReady, schedule.NextCommonDropSeconds)
	}

	schedule = currentInventoryFillDropSchedule(now.Add(2 * commonChestDropInterval))
	if !schedule.CommonDropReady || schedule.NextCommonDropSeconds != 0 {
		t.Fatalf("common ready schedule after another interval = ready %t next %d, want true/0", schedule.CommonDropReady, schedule.NextCommonDropSeconds)
	}

	if !recordChestDropLog(getBoxLogMonster, now.Add(2*commonChestDropInterval)) {
		t.Fatal("second monster box log should be recorded")
	}
	schedule = currentInventoryFillDropSchedule(now.Add(2 * commonChestDropInterval))
	if schedule.CommonDropReady || schedule.NextCommonDropSeconds != int(commonChestDropInterval/time.Second) {
		t.Fatalf("common schedule after reset = ready %t next %d, want false/%d",
			schedule.CommonDropReady,
			schedule.NextCommonDropSeconds,
			int(commonChestDropInterval/time.Second),
		)
	}
}

func TestCurrentInventoryFillDropScheduleUsesCustomChestTiming(t *testing.T) {
	resetChestDropTracker()
	t.Cleanup(resetChestDropTracker)

	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	timing := inventory.ChestTimingConfig{
		CommonDropSeconds:         300,
		StageDropSeconds:          420,
		CommonBaseAutoOpenSeconds: 360,
		StageBaseAutoOpenSeconds:  700,
	}

	schedule := currentInventoryFillDropScheduleWithTiming(now, timing)

	if schedule.NextCommonDropSeconds != 300 || schedule.NextStageDropSeconds != 420 {
		t.Fatalf("next drops = common %d stage %d, want 300/420", schedule.NextCommonDropSeconds, schedule.NextStageDropSeconds)
	}
}

func TestCurrentChestTimingConfigUsesActiveGameLayout(t *testing.T) {
	activeApp.gameLayoutMu.Lock()
	previousLayout := activeApp.activeGameLayout
	activeApp.activeGameLayout = game.GameLayout{
		ChestTiming: game.ChestTiming{
			CommonChestDropSeconds:         301,
			CommonChestBaseAutoOpenSeconds: 302,
			StageChestDropSeconds:          601,
			StageChestBaseAutoOpenSeconds:  602,
		},
	}
	activeApp.gameLayoutMu.Unlock()
	t.Cleanup(func() {
		activeApp.gameLayoutMu.Lock()
		activeApp.activeGameLayout = previousLayout
		activeApp.gameLayoutMu.Unlock()
	})

	timing := currentChestTimingConfig()

	if timing.CommonDropSeconds != 301 || timing.CommonBaseAutoOpenSeconds != 302 {
		t.Fatalf("common timing = drop %d open %d, want 301/302", timing.CommonDropSeconds, timing.CommonBaseAutoOpenSeconds)
	}
	if timing.StageDropSeconds != 601 || timing.StageBaseAutoOpenSeconds != 602 {
		t.Fatalf("stage timing = drop %d open %d, want 601/602", timing.StageDropSeconds, timing.StageBaseAutoOpenSeconds)
	}
}

func TestItemIDFromItemNameKey(t *testing.T) {
	tests := []struct {
		value  string
		itemID int
		ok     bool
	}{
		{value: "ItemName_12345", itemID: 12345, ok: true},
		{value: "ItemName_0", ok: false},
		{value: "ItemName_12x", ok: false},
		{value: "MonsterName_12345", ok: false},
		{value: "", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			itemID, ok := itemIDFromItemNameKey(tt.value)
			if itemID != tt.itemID || ok != tt.ok {
				t.Fatalf("itemIDFromItemNameKey(%q) = %d, %t; want %d, %t", tt.value, itemID, ok, tt.itemID, tt.ok)
			}
		})
	}
}

func TestTradeSlotCooldownNotificationRequiresFutureObservation(t *testing.T) {
	resetTradeSlotNotificationStateForTest(t)

	now := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)
	alreadyDue := now.Add(-time.Minute)
	if shouldNotifyTradeSlotCooldown(0, alreadyDue, now) {
		t.Fatal("first observation of an already-due cooldown should not notify")
	}
	if shouldNotifyTradeSlotCooldown(0, alreadyDue, now.Add(time.Second)) {
		t.Fatal("already suppressed cooldown should not notify later")
	}

	future := now.Add(time.Minute)
	if shouldNotifyTradeSlotCooldown(1, future, now) {
		t.Fatal("future cooldown should not notify before it is due")
	}
	if !shouldNotifyTradeSlotCooldown(1, future, future.Add(time.Second)) {
		t.Fatal("previously observed future cooldown should notify after it is due")
	}
	if shouldNotifyTradeSlotCooldown(1, future, future.Add(2*time.Second)) {
		t.Fatal("same due cooldown should notify only once")
	}
}

func TestInventoryFullNotificationRequiresObservedTransition(t *testing.T) {
	originalAppHWND := activeApp.appHWND
	originalTrayIconAdded := activeApp.trayIconAdded
	originalPublisher := publishTrayNotification
	originalPreference := currentDisplayLanguagePreference()

	var receivedTitles []string
	var receivedBodies []string
	activeApp.appHWND = 1
	activeApp.trayIconAdded = true
	publishTrayNotification = func(title string, message string, _ uintptr) {
		receivedTitles = append(receivedTitles, title)
		receivedBodies = append(receivedBodies, message)
	}
	applyDisplayLanguagePreference("en-US")
	resetInventoryFullNotification()
	clearPendingTrayNotifications()
	t.Cleanup(func() {
		activeApp.appHWND = originalAppHWND
		activeApp.trayIconAdded = originalTrayIconAdded
		publishTrayNotification = originalPublisher
		applyDisplayLanguagePreference(originalPreference)
		resetInventoryFullNotification()
		clearPendingTrayNotifications()
	})

	fullSnapshot := playerdata.InventorySnapshot{
		InventorySlots: playerdata.InventorySlotSummary{Total: 3, Used: 3, Free: 0},
	}
	notFullSnapshot := playerdata.InventorySnapshot{
		InventorySlots: playerdata.InventorySlotSummary{Total: 3, Used: 2, Free: 0},
	}

	updateInventoryFullNotification(fullSnapshot)
	flushTrayNotifications()
	if len(receivedTitles) != 0 {
		t.Fatalf("first full observation emitted notification: %q", receivedTitles)
	}

	updateInventoryFullNotification(fullSnapshot)
	flushTrayNotifications()
	if len(receivedTitles) != 0 {
		t.Fatalf("unchanged full inventory emitted notification: %q", receivedTitles)
	}

	updateInventoryFullNotification(notFullSnapshot)
	flushTrayNotifications()
	if len(receivedTitles) != 0 {
		t.Fatalf("not-full inventory emitted notification: %q", receivedTitles)
	}

	updateInventoryFullNotification(fullSnapshot)
	flushTrayNotifications()
	if len(receivedTitles) != 1 || receivedTitles[0] != "Inventory Full" {
		t.Fatalf("full transition notification title = %q, want Inventory Full", receivedTitles)
	}
	if len(receivedBodies) != 1 || receivedBodies[0] != "Your inventory has no free slots left." {
		t.Fatalf("full transition notification body = %q", receivedBodies)
	}

	updateInventoryFullNotification(fullSnapshot)
	flushTrayNotifications()
	if len(receivedTitles) != 1 {
		t.Fatalf("repeated full inventory emitted notification: %q", receivedTitles)
	}
}

func TestInventoryFullNotificationUsesTurkishText(t *testing.T) {
	originalPreference := currentDisplayLanguagePreference()
	t.Cleanup(func() { applyDisplayLanguagePreference(originalPreference) })

	applyDisplayLanguagePreference("tr-TR")
	title, body := inventoryFullNotificationContent()

	if title != "Envanter Dolu" || body != "Envanterinde boş slot kalmadı." {
		t.Fatalf("Turkish inventory full notification = %q / %q", title, body)
	}
}

func resetTradeSlotNotificationStateForTest(t *testing.T) {
	t.Helper()
	tradeSlotsNotifyMu.Lock()
	originalObserved := observedTradeSlots
	originalNotified := notifiedTradeSlots
	observedTradeSlots = make(map[int]time.Time)
	notifiedTradeSlots = make(map[int]time.Time)
	tradeSlotsNotifyMu.Unlock()

	t.Cleanup(func() {
		tradeSlotsNotifyMu.Lock()
		observedTradeSlots = originalObserved
		notifiedTradeSlots = originalNotified
		tradeSlotsNotifyMu.Unlock()
	})
}

func TestNormalizeNotificationSources(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "legacy empty defaults to all", in: "", want: notificationSourcesAll},
		{name: "canonical order", in: "offering,box,craft", want: "box,craft,offering"},
		{name: "unknown ignored", in: "box,unknown", want: "box"},
		{name: "none", in: "none", want: notificationSourcesNone},
		{name: "only unknown becomes none", in: "unknown", want: notificationSourcesNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeNotificationSources(tt.in); got != tt.want {
				t.Fatalf("normalizeNotificationSources(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestResolveGradedItemID(t *testing.T) {
	tests := []struct {
		name       string
		baseItemID int
		grade      int
		want       int
	}{
		{name: "Mystic Boots Legendary", baseItemID: 530011, grade: 3, want: 533111},
		{name: "Mystic Boots Arcana", baseItemID: 530011, grade: 5, want: 535111},
		{name: "Dimensional Boots Rare", baseItemID: 530017, grade: 2, want: 532171},
		{name: "Storm Sword Legendary", baseItemID: 300013, grade: 3, want: 303131},
		{name: "Iron Plate Uncommon", baseItemID: 510003, grade: 1, want: 511031},
		{name: "Non-gear / material", baseItemID: 110001, grade: 3, want: 110001},
		{name: "Already graded item", baseItemID: 533111, grade: 5, want: 533111},
		{name: "Zero grade", baseItemID: 530011, grade: 0, want: 530011},
		{name: "Negative grade", baseItemID: 530011, grade: -1, want: 530011},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveGradedItemID(tt.baseItemID, tt.grade); got != tt.want {
				t.Fatalf("resolveGradedItemID(%d, %d) = %d, want %d", tt.baseItemID, tt.grade, got, tt.want)
			}
		})
	}
}
