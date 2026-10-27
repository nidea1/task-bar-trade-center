package catalog

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const SteamAppID = 3678970

// MarketDefinition is the Steam identity supplied by the client SDK adapter.
type MarketDefinition struct {
	AppID      int
	ID         int
	Name       string
	Marketable bool
	Tags       map[string]string
}

func MarketDefinitionID(itemID int) (int, bool) {
	if itemID <= 0 || itemID >= 1_000_000_000 {
		return 0, false
	}
	// The game's Steam construction appends 900 to base IDs; encoded IDs are retained.
	if itemID < 10_000_000 {
		if itemID > (999_999_999-900)/1000 {
			return 0, false
		}
		return itemID*1000 + 900, true
	}
	return itemID, true
}

func (config ItemConfig) HasMarketIdentity() bool {
	id, ok := MarketDefinitionID(config.ID)
	return ok && config.SteamDefinitionID == id && validMarketName(config.MarketHashName) && marketTagsMatchItem(config, config.SteamTags)
}

func ApplyMarketDefinitions(items []ItemConfig, definitions map[int]MarketDefinition) []ItemConfig {
	result := append([]ItemConfig(nil), items...)
	for index := range result {
		item := &result[index]
		item.MarketHashName, item.SteamDefinitionID, item.SteamTags = "", 0, nil
		if !item.Marketable {
			continue
		}
		id, ok := MarketDefinitionID(item.ID)
		definition, exists := definitions[id]
		if !ok || !exists || definition.AppID != SteamAppID || definition.ID != id || !validMarketName(definition.Name) {
			continue
		}
		if !definition.Marketable {
			continue
		}
		if !marketTagsMatchItem(*item, definition.Tags) {
			continue
		}
		item.MarketHashName, item.SteamDefinitionID = definition.Name, id
		item.SteamTags = make(map[string]string)
		for _, key := range []string{"type", "rarity", "parts", "class", "level"} {
			if value := definition.Tags[key]; value != "" {
				item.SteamTags[key] = value
			}
		}
	}
	return result
}

func validMarketName(name string) bool {
	return name != "" && len(name) <= 512 && utf8.ValidString(name) && strings.TrimSpace(name) == name && !strings.ContainsFunc(name, unicode.IsControl)
}

func marketTagsMatchItem(item ItemConfig, tags map[string]string) bool {
	if item.Type == "" || item.Grade == "" || tags["type"] != strings.ToLower(item.Type) || tags["rarity"] != strings.ToLower(item.Grade) {
		return false
	}
	return item.Type != "GEAR" || (item.Level > 0 && tags["level"] == strconv.Itoa(item.Level))
}
