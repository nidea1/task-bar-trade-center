package steamdefs

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/nidea1/task-bar-trade-center/internal/catalog"
)

func readDefinition(id int, property func(int, string) (string, bool)) (catalog.MarketDefinition, bool) {
	values := make(map[string]string)
	for _, key := range []string{"appid", "itemdefid", "name", "marketable", "tags"} {
		value, ok := property(id, key)
		if !ok || !utf8.ValidString(value) {
			return catalog.MarketDefinition{}, false
		}
		values[key] = value
	}
	appID, err := strconv.Atoi(values["appid"])
	if err != nil || appID != catalog.SteamAppID {
		return catalog.MarketDefinition{}, false
	}
	definitionID, err := strconv.Atoi(values["itemdefid"])
	if err != nil || definitionID != id || values["name"] == "" || (values["marketable"] != "0" && values["marketable"] != "1") {
		return catalog.MarketDefinition{}, false
	}
	tags := make(map[string]string)
	for _, tag := range strings.Split(values["tags"], ";") {
		key, value, ok := strings.Cut(tag, ":")
		if !ok || key == "" || value == "" {
			return catalog.MarketDefinition{}, false
		}
		if _, duplicate := tags[key]; duplicate {
			return catalog.MarketDefinition{}, false
		}
		tags[key] = value
	}
	return catalog.MarketDefinition{AppID: appID, ID: id, Name: values["name"], Marketable: values["marketable"] == "1", Tags: tags}, true
}
