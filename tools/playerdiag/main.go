//go:build windows

package main

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/catalog"
	"github.com/nidea1/task-bar-trade-center/internal/playerdata"
	"github.com/nidea1/task-bar-trade-center/internal/tbhmem"
)

const (
	processName = "TaskBarHero.exe"
	moduleName  = "GameAssembly.dll"
)

func main() {
	process, ok := tbhmem.OpenByName(processName)
	if !ok {
		fmt.Printf("%s is not running or could not be opened.\n", processName)
		os.Exit(1)
	}
	defer process.Close()

	fmt.Printf("Process: %s pid=%d\n", processName, process.PID)
	if base := process.ModuleBase(moduleName); base != 0 {
		fmt.Printf("%s base=0x%X\n", moduleName, base)
	}

	resolver := playerdata.NewResolver(loadItemMetadata())
	startedAt := time.Now()
	snapshot, ok := resolver.ReadSnapshot(process, time.Now())
	if !ok {
		fmt.Println("PlayerSaveData could not be resolved.")
		os.Exit(2)
	}

	fmt.Printf("Resolved PlayerSaveData in %s\n", time.Since(startedAt))
	fmt.Printf("Gold: %d\n", snapshot.Gold)
	fmt.Printf("Owned item references: %d\n", len(snapshot.Items))
	printLocationCounts(snapshot)
}

func loadItemMetadata() map[int]playerdata.ItemMetadata {
	items, err := catalog.LoadItems()
	if err != nil {
		return nil
	}
	metadata := make(map[int]playerdata.ItemMetadata, len(items))
	for _, item := range items {
		metadata[item.ID] = playerdata.ItemMetadata{Marketable: item.Marketable}
	}
	return metadata
}

func printLocationCounts(snapshot playerdata.InventorySnapshot) {
	counts := make(map[playerdata.Location]int)
	marketable := 0
	for _, item := range snapshot.Items {
		counts[item.Location]++
		if item.Marketable {
			marketable++
		}
	}
	locations := make([]string, 0, len(counts))
	for location := range counts {
		locations = append(locations, string(location))
	}
	sort.Strings(locations)
	for _, location := range locations {
		fmt.Printf("%s: %d\n", location, counts[playerdata.Location(location)])
	}
	fmt.Printf("Marketable item references: %d\n", marketable)
}
