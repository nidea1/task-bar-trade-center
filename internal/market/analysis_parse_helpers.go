package market

import (
	"encoding/json"
	"html"
	"strconv"
	"strings"
	"time"
)

func searchableListingText(body []byte) string {
	raw := html.UnescapeString(string(body))
	var builder strings.Builder
	builder.WriteString(raw)
	appendDecodedJSONParseStrings(&builder, raw, 0)
	return builder.String()
}

func appendDecodedJSONParseStrings(builder *strings.Builder, text string, depth int) {
	if depth > 2 {
		return
	}
	for _, match := range jsonParseCallPattern.FindAllStringSubmatch(text, -1) {
		if len(match) != 2 {
			continue
		}
		decoded, err := strconv.Unquote(`"` + match[1] + `"`)
		if err != nil {
			continue
		}
		normalized := strings.ReplaceAll(decoded, `\"`, `"`)
		builder.WriteByte('\n')
		builder.WriteString(decoded)
		builder.WriteByte('\n')
		builder.WriteString(normalized)
		appendDecodedJSONParseStrings(builder, decoded, depth+1)
		appendDecodedJSONParseStrings(builder, normalized, depth+1)
	}
}

func unwrapJSONResponseObject(body []byte) (map[string]json.RawMessage, bool) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		return nil, false
	}
	if successRaw, exists := top["success"]; exists {
		if success, ok := parseJSONBool(successRaw); ok && !success {
			return nil, false
		}
	}
	if responseRaw, exists := top["response"]; exists {
		var response map[string]json.RawMessage
		if err := json.Unmarshal(responseRaw, &response); err == nil {
			return response, true
		}
	}
	return top, true
}

func parseHistogramPrice(raw json.RawMessage) (float64, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return parseSteamPriceString(text, true)
	}
	var value float64
	if err := json.Unmarshal(raw, &value); err == nil {
		return value, true
	}
	return 0, false
}

func parseJSONBool(raw json.RawMessage) (bool, bool) {
	var value bool
	if err := json.Unmarshal(raw, &value); err == nil {
		return value, true
	}
	var numeric float64
	if err := json.Unmarshal(raw, &numeric); err == nil {
		return numeric != 0, true
	}
	return false, false
}

func parseJSONInt(raw json.RawMessage) (int, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	var value float64
	if err := json.Unmarshal(raw, &value); err == nil {
		return int(value), true
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return parseFirstInt(text)
	}
	return 0, false
}

func parseJSONString(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}
	return value, true
}

func parseSaleHistoryValue(value interface{}) []MarketSalePoint {
	items, ok := value.([]interface{})
	if !ok {
		return nil
	}
	points := make([]MarketSalePoint, 0, len(items))
	for _, item := range items {
		switch typed := item.(type) {
		case map[string]interface{}:
			point, ok := parseSaleHistoryObject(typed)
			if ok {
				points = append(points, point)
			}
		case []interface{}:
			point, ok := parseSaleHistoryArray(typed)
			if ok {
				points = append(points, point)
			}
		}
	}
	return points
}

func parseSteamSaleArray(raw []byte) []MarketSalePoint {
	var items []interface{}
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil
	}
	return parseSaleHistoryValue(items)
}

func parseSaleHistoryObject(item map[string]interface{}) (MarketSalePoint, bool) {
	unixTime, ok := interfaceInt64(item["time"])
	if !ok {
		return MarketSalePoint{}, false
	}
	price, ok := interfaceFloat(item["price"])
	if !ok || price <= 0 {
		return MarketSalePoint{}, false
	}
	volume, ok := interfaceInt(item["volume"])
	if !ok {
		return MarketSalePoint{}, false
	}
	return MarketSalePoint{Time: unixTime, Price: price, Volume: volume}, true
}

func parseSaleHistoryArray(item []interface{}) (MarketSalePoint, bool) {
	if len(item) < 3 {
		return MarketSalePoint{}, false
	}

	var unixTime int64
	switch value := item[0].(type) {
	case string:
		parsedTime, ok := parseSteamHistoryTime(value)
		if !ok {
			return MarketSalePoint{}, false
		}
		unixTime = parsedTime
	case float64:
		unixTime = int64(value)
	default:
		return MarketSalePoint{}, false
	}

	price, ok := interfaceFloat(item[1])
	if !ok || price <= 0 {
		return MarketSalePoint{}, false
	}
	volume, ok := interfaceInt(item[2])
	if !ok {
		return MarketSalePoint{}, false
	}
	return MarketSalePoint{Time: unixTime, Price: price, Volume: volume}, true
}

func parseSteamHistoryTime(value string) (int64, bool) {
	fields := strings.Fields(value)
	if len(fields) < 3 {
		return 0, false
	}
	parsed, err := time.ParseInLocation("Jan 02 2006", fields[0]+" "+fields[1]+" "+fields[2], time.UTC)
	if err != nil {
		return 0, false
	}
	return parsed.Unix(), true
}

func interfaceFloat(value interface{}) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case string:
		return parseSteamPriceString(typed, false)
	default:
		return 0, false
	}
}

func interfaceInt(value interface{}) (int, bool) {
	switch typed := value.(type) {
	case float64:
		return int(typed), true
	case string:
		return parseFirstInt(typed)
	default:
		return 0, false
	}
}

func interfaceInt64(value interface{}) (int64, bool) {
	switch typed := value.(type) {
	case float64:
		return int64(typed), true
	case string:
		return parseInt64(typed)
	default:
		return 0, false
	}
}

func parseSteamPriceString(value string, integerStringIsCents bool) (float64, bool) {
	value = strings.TrimSpace(html.UnescapeString(value))
	if value == "" {
		return 0, false
	}
	if integerStringIsCents && pureDigitsPattern.MatchString(value) {
		cents, ok := parseInt(value)
		if !ok {
			return 0, false
		}
		return centsToPrice(cents), true
	}

	match := numberPattern.FindString(value)
	if match == "" {
		return 0, false
	}
	normalized := normalizeDecimalString(match)
	price, err := strconv.ParseFloat(normalized, 64)
	if err != nil {
		return 0, false
	}
	return price, true
}

func parseSteamFormattedPrice(value string) (float64, string, string, bool) {
	value = strings.TrimSpace(html.UnescapeString(value))
	matchIndex := numberPattern.FindStringIndex(value)
	if matchIndex == nil {
		return 0, "", "", false
	}
	price, ok := parseSteamPriceString(value, false)
	if !ok {
		return 0, "", "", false
	}
	prefix := strings.TrimSpace(value[:matchIndex[0]])
	suffix := strings.TrimSpace(value[matchIndex[1]:])
	if prefix == "$" && strings.TrimSpace(suffix) == "USD" {
		suffix = ""
	}
	return price, prefix, suffix, true
}

func normalizeDecimalString(value string) string {
	if strings.Contains(value, ".") && strings.Contains(value, ",") {
		return strings.ReplaceAll(value, ",", "")
	}
	if strings.Contains(value, ",") && !strings.Contains(value, ".") {
		lastComma := strings.LastIndex(value, ",")
		if len(value)-lastComma-1 == 2 {
			value = strings.ReplaceAll(value, ".", "")
			return strings.Replace(value, ",", ".", 1)
		}
		return strings.ReplaceAll(value, ",", "")
	}
	return value
}

func parseFirstInt(value string) (int, bool) {
	matches := digitsPattern.FindAllString(value, -1)
	if len(matches) == 0 {
		return 0, false
	}
	joined := strings.Join(matches, "")
	return parseInt(joined)
}

func parseInt(value string) (int, bool) {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func parseInt64(value string) (int64, bool) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func parseFloat(value string) (float64, bool) {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func centsToPrice(cents int) float64 {
	return float64(cents) / 100
}

func parseGraphFirstQuantity(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	var arrays [][]interface{}
	if err := json.Unmarshal(raw, &arrays); err == nil && len(arrays) > 0 && len(arrays[0]) >= 2 {
		if qty, ok := parseInterfaceInt(arrays[0][1]); ok {
			return qty
		}
	}
	var objects []map[string]interface{}
	if err := json.Unmarshal(raw, &objects); err == nil && len(objects) > 0 {
		if qtyVal, exists := objects[0]["volume"]; exists {
			if qty, ok := parseInterfaceInt(qtyVal); ok {
				return qty
			}
		}
		if qtyVal, exists := objects[0]["quantity"]; exists {
			if qty, ok := parseInterfaceInt(qtyVal); ok {
				return qty
			}
		}
	}
	return 0
}

func parseInterfaceInt(val interface{}) (int, bool) {
	switch v := val.(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	case string:
		if parsed, err := strconv.Atoi(v); err == nil {
			return parsed, true
		}
	}
	return 0, false
}
