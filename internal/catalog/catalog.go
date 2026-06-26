package catalog

import (
	"encoding/json"
	"fmt"

	_ "embed"
)

//go:embed items.json
var embeddedItemsJSON []byte

type ItemConfig struct {
	ID         int               `json:"id"`
	Name       map[string]string `json:"name"`
	Grade      string            `json:"grade"`
	Type       string            `json:"type"`
	Gear       *string           `json:"gear"`
	Marketable bool              `json:"marketable"`
}

func LoadItems() ([]ItemConfig, error) {
	var items []ItemConfig
	if err := json.Unmarshal(embeddedItemsJSON, &items); err != nil {
		return nil, fmt.Errorf("embedded items database could not be loaded: %w", err)
	}
	return items, nil
}
