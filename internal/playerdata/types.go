package playerdata

import "time"

type Location string

const (
	LocationInventory     Location = "inventory"
	LocationStash         Location = "stash"
	LocationMaterialStash Location = "material_stash"
	LocationEquipped      Location = "equipped"
)

type ItemMetadata struct {
	Marketable bool
}

type OwnedItem struct {
	Quantity        int      `json:"quantity,omitempty"`
	ItemID          int      `json:"item_id"`
	UniqueID        uint64   `json:"unique_id"`
	Location        Location `json:"location"`
	EquippedHeroKey int      `json:"equipped_hero_key,omitempty"`
	Marketable      bool     `json:"marketable"`
	SlotIndex       int      `json:"slot_index"`
}

func (item OwnedItem) Units() int {
	if item.Quantity > 0 {
		return item.Quantity
	}
	return 1
}

type TradeShipSlot struct {
	Index         int       `json:"index"`
	State         int       `json:"state"`
	CooldownUntil time.Time `json:"cooldown_until" ts_type:"string"`
}

type InventorySlotSummary struct {
	Total           int    `json:"total"`
	BackingTotal    int    `json:"backing_total,omitempty"`
	Used            int    `json:"used"`
	Free            int    `json:"free"`
	EmptySlotRanges string `json:"empty_slot_ranges,omitempty"`
	Source          string `json:"source,omitempty"`
}

type ChestAutomation struct {
	CommonBaseAutoOpenSeconds  int      `json:"common_base_auto_open_seconds,omitempty"`
	StageBaseAutoOpenSeconds   int      `json:"stage_base_auto_open_seconds,omitempty"`
	CommonRuneReductionSeconds int      `json:"common_rune_reduction_seconds"`
	StageRuneReductionSeconds  int      `json:"stage_rune_reduction_seconds"`
	RuneResolved               bool     `json:"rune_resolved"`
	HasCommonAutoOpenRune      bool     `json:"has_common_auto_open_rune"`
	HasStageAutoOpenRune       bool     `json:"has_stage_auto_open_rune"`
	CommonReductionBuffSummary string   `json:"-"`
	StageReductionBuffSummary  string   `json:"-"`
	ReductionBuffSummary       string   `json:"-"`
	Diagnostics                []string `json:"-"`
}

type ChestCounts struct {
	NormalChests int `json:"normal_chests"`
	StageChests  int `json:"stage_chests"`
	TotalChests  int `json:"total_chests"`
}

type ChestTimer struct {
	QueueCount       int     `json:"queue_count"`
	QueueResolved    bool    `json:"queue_resolved"`
	Available        bool    `json:"available"`
	Enabled          bool    `json:"enabled"`
	DurationSeconds  int     `json:"duration_seconds"`
	RemainingSeconds float64 `json:"remaining_seconds"`
}

type ChestTimers struct {
	Common ChestTimer `json:"common"`
	Stage  ChestTimer `json:"stage"`
}

type InventorySnapshot struct {
	Region          Region               `json:"region,omitempty"`
	StageKey        int                  `json:"stage_key,omitempty"`
	ReadAt          time.Time            `json:"read_at"`
	Gold            uint64               `json:"gold"`
	StashPageCount  int                  `json:"stash_page_count"`
	HasMaterialStash bool                `json:"has_material_stash"`
	InventorySlots  InventorySlotSummary `json:"inventory_slots"`
	ChestAutomation ChestAutomation      `json:"chest_automation"`
	ChestCounts     ChestCounts          `json:"chest_counts"`
	ChestTimers     ChestTimers          `json:"chest_timers"`
	Items           []OwnedItem          `json:"items"`
	TradeSlots      []TradeShipSlot      `json:"trade_slots"`
}

type Region string

const (
	RegionNormal      Region = "normal"
	RegionPlaguelands Region = "plaguelands"
)
