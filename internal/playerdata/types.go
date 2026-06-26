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
}

type InventorySnapshot struct {
	ReadAt time.Time   `json:"read_at"`
	Gold   uint64      `json:"gold"`
	Items  []OwnedItem `json:"items"`
}
