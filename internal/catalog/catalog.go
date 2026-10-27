package catalog

import (
	"encoding/json"
	"fmt"

	_ "embed"
)

//go:embed items.json
var embeddedItemsJSON []byte

type ItemConfig struct {
	ID             int               `json:"id"`
	Name           map[string]string `json:"name"`
	Grade          string            `json:"grade"`
	Type           string            `json:"type"`
	Level          int               `json:"level,omitempty"`
	Gear           *string           `json:"gear"`
	Marketable     bool              `json:"marketable"`
	MarketHashName string            `json:"market_hash_name,omitempty"`
}

func (config ItemConfig) MarketVariant() string {
	// shortcut: this follows the observed level cutoff; update if Steam changes its variant naming.
	if config.Type == "GEAR" && config.Level >= 90 {
		return "C"
	}
	return "A"
}

func LoadItems() ([]ItemConfig, error) {
	var items []ItemConfig
	if err := json.Unmarshal(embeddedItemsJSON, &items); err != nil {
		return nil, fmt.Errorf("embedded items database could not be loaded: %w", err)
	}
	return items, nil
}
