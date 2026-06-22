package main

import (
	"encoding/json"
	"fmt"

	_ "embed"
)

//go:embed items.json
var embeddedItemsJSON []byte

func loadItemsJSON() error {
	var db []ItemConfig
	if err := json.Unmarshal(embeddedItemsJSON, &db); err != nil {
		fmt.Printf("Embedded items database could not be loaded: %v\n", err)
		return err
	}
	for _, item := range db {
		AllItemMap[item.ID] = item
		if item.Marketable {
			ItemMap[item.ID] = item
		}
	}
	fmt.Printf("Database loaded: %d marketable items active.\n", len(ItemMap))
	return nil
}
