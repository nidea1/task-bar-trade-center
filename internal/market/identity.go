package market

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/nidea1/task-bar-trade-center/internal/catalog"
	nethtml "golang.org/x/net/html"
)

var ErrIdentityUnavailable = errors.New("Steam market identity is unavailable")
var ErrIdentityMismatch = errors.New("Steam market identity does not match the item")

type listingQuery struct {
	QueryKey []json.RawMessage `json:"queryKey"`
	State    struct {
		Data json.RawMessage `json:"data"`
	} `json:"state"`
}

type itemDescription struct {
	AppID      int    `json:"appid"`
	HashName   string `json:"market_hash_name"`
	Marketable bool   `json:"marketable"`
	Tags       []struct {
		Category string `json:"category"`
		Name     string `json:"internal_name"`
	} `json:"tags"`
}

func validateDescription(config catalog.ItemConfig, raw json.RawMessage) error {
	var description itemDescription
	if json.Unmarshal(raw, &description) != nil || description.AppID != SteamAppID || !description.Marketable || description.HashName != config.MarketHashName {
		return fmt.Errorf("%w: item %d, expected %q", ErrIdentityMismatch, config.ID, config.MarketHashName)
	}
	tags := make(map[string]string)
	for _, tag := range description.Tags {
		if _, duplicate := tags[tag.Category]; duplicate {
			return fmt.Errorf("%w: duplicate description tag %q", ErrIdentityMismatch, tag.Category)
		}
		tags[tag.Category] = tag.Name
	}
	for key, expected := range config.SteamTags {
		if tags[key] != expected {
			return fmt.Errorf("%w: item %d tag %s", ErrIdentityMismatch, config.ID, key)
		}
	}
	return nil
}

// Keep only the exact item's queries so unrelated SSR cache entries cannot supply prices.
func verifiedListingSources(config catalog.ItemConfig, body []byte) ([]byte, error) {
	var sources []listingQuery
	hasDescription := false
	seen := make(map[string]bool)
	kinds := make(map[string]listingQuery)
	for _, query := range listingQueries(body) {
		if len(query.QueryKey) < 4 {
			continue
		}
		var namespace, kind, name string
		var appID int
		if json.Unmarshal(query.QueryKey[0], &namespace) != nil || namespace != "market" ||
			json.Unmarshal(query.QueryKey[1], &kind) != nil || json.Unmarshal(query.QueryKey[2], &appID) != nil || appID != SteamAppID ||
			json.Unmarshal(query.QueryKey[3], &name) != nil || name != config.MarketHashName {
			continue
		}
		if kind != "description" && kind != "orderbook" && kind != "pricehistory" {
			continue
		}
		if kind == "description" {
			if err := validateDescription(config, query.State.Data); err != nil {
				return nil, err
			}
			hasDescription = true
		}
		encoded, _ := json.Marshal(query)
		if !seen[string(encoded)] {
			seen[string(encoded)] = true
			if _, duplicate := kinds[kind]; duplicate {
				return nil, fmt.Errorf("%w: ambiguous %s data", ErrIdentityMismatch, kind)
			}
			kinds[kind] = query
		}
	}
	if !hasDescription {
		return nil, fmt.Errorf("%w: listing has no matching description for item %d", ErrIdentityMismatch, config.ID)
	}
	for _, kind := range []string{"orderbook", "pricehistory", "description"} {
		if query, exists := kinds[kind]; exists {
			sources = append(sources, query)
		}
	}
	return json.Marshal(struct {
		Queries []listingQuery `json:"queries"`
	}{sources})
}

func verifiedSearchSources(config catalog.ItemConfig, body []byte) ([]byte, error) {
	for _, query := range listingQueries(body) {
		var page struct {
			Pages []struct {
				Results []json.RawMessage `json:"results"`
			} `json:"pages"`
		}
		if json.Unmarshal(query.State.Data, &page) != nil {
			continue
		}
		for _, page := range page.Pages {
			for _, raw := range page.Results {
				var result struct {
					Name        string          `json:"strHash"`
					Description json.RawMessage `json:"asset_description"`
				}
				if json.Unmarshal(raw, &result) != nil || result.Name != config.MarketHashName {
					continue
				}
				if err := validateDescription(config, result.Description); err != nil {
					return nil, err
				}
				return raw, nil
			}
		}
	}
	return nil, fmt.Errorf("%w: search has no verified result for item %d", ErrIdentityMismatch, config.ID)
}

func listingQueries(body []byte) []listingQuery {
	var queries []listingQuery
	var collect func(json.RawMessage, int)
	collect = func(raw json.RawMessage, depth int) {
		if depth > 10 || len(raw) == 0 {
			return
		}
		var text string
		if json.Unmarshal(raw, &text) == nil {
			collect(json.RawMessage(text), depth+1)
			return
		}
		var array []json.RawMessage
		if json.Unmarshal(raw, &array) == nil {
			for _, child := range array {
				collect(child, depth+1)
			}
			return
		}
		var object map[string]json.RawMessage
		if json.Unmarshal(raw, &object) != nil {
			return
		}
		if _, ok := object["queryKey"]; ok {
			var query listingQuery
			if json.Unmarshal(raw, &query) == nil {
				queries = append(queries, query)
			}
			return
		}
		for _, key := range []string{"queries", "dehydratedState", "renderContext", "queryData", "loaderData", "searchState"} {
			if child, ok := object[key]; ok {
				collect(child, depth+1)
			}
		}
	}
	tokens := nethtml.NewTokenizer(bytes.NewReader(body))
	for {
		kind := tokens.Next()
		if kind == nethtml.ErrorToken {
			break
		}
		if kind != nethtml.StartTagToken {
			continue
		}
		if tokens.Token().Data != "script" {
			continue
		}
		if tokens.Next() != nethtml.TextToken {
			continue
		}
		text := string(tokens.Text())
		collect(json.RawMessage(text), 0)
		for _, match := range jsonParseCallPattern.FindAllStringSubmatch(text, -1) {
			var decoded string
			if json.Unmarshal([]byte(`"`+match[1]+`"`), &decoded) == nil {
				collect(json.RawMessage(decoded), 0)
			}
		}
		for _, match := range ssrLoaderDataPattern.FindAllStringIndex(text, -1) {
			decoder := json.NewDecoder(strings.NewReader(text[match[1]:]))
			var raw json.RawMessage
			if decoder.Decode(&raw) == nil {
				collect(raw, 0)
			}
		}
	}
	return queries
}
