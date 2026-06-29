package app

import (
	"testing"
	"time"
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
