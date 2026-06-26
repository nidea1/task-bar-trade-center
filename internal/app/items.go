package app

import (
	"fmt"

	"github.com/nidea1/task-bar-trade-center/internal/catalog"
)

func loadItemsJSON() error {
	db, err := catalog.LoadItems()
	if err != nil {
		fmt.Printf("%v\n", err)
		return err
	}
	for _, item := range db {
		activeApp.allItemMap[item.ID] = item
		if item.Marketable {
			activeApp.itemMap[item.ID] = item
		}
	}
	fmt.Printf("Database loaded: %d marketable items active.\n", len(activeApp.itemMap))
	return nil
}
