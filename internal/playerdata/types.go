package playerdata

import "time"

type Location string

const (
	LocationInventory Location = "inventory"
	LocationStash     Location = "stash"
	LocationEquipped  Location = "equipped"
)

type ItemMetadata struct {
	Marketable bool
}

type OwnedItem struct {
	ItemID          int      `json:"item_id"`
	UniqueID        uint64   `json:"unique_id"`
	Location        Location `json:"location"`
	EquippedHeroKey int      `json:"equipped_hero_key,omitempty"`
	Marketable      bool     `json:"marketable"`
	SlotIndex       int      `json:"slot_index"`
}

type TradeShipSlot struct {
	Index         int       `json:"index"`
	State         int       `json:"state"`
	CooldownUntil time.Time `json:"cooldown_until" ts_type:"string"`
}

type InventorySnapshot struct {
	ReadAt         time.Time       `json:"read_at"`
	Gold           uint64          `json:"gold"`
	StashPageCount int             `json:"stash_page_count"`
	Items          []OwnedItem     `json:"items"`
	TradeSlots     []TradeShipSlot `json:"trade_slots"`
}

