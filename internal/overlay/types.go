package overlay

import (
	"strings"

	"github.com/nidea1/task-bar-trade-center/internal/win32"
)

type PriceView struct {
	Suggested      string
	LowestSell     string
	HighestBuy     string
	DailySales     string
	WeeklyAvg      string
	LastSold       string
	Updated        string
	Trend          string
	Spread         string
	DealTag        string
	Confidence     string
	Orders         string
	SaleP75        string
	FallbackNotice string
}

type PlacementConfig struct {
	OffsetX             int32
	OffsetY             int32
	Width               int32
	ReferenceHeight     int32
	ReferencePanelWidth int32
	MinWidth            int32
	MaxWidth            int32
	AnchorOffsetX       int32
	AnchorOffsetY       int32
}

type PlacementCalibration struct {
	TooltipY      int32 `json:"tooltip_y"`
	TooltipHeight int32 `json:"tooltip_height"`
	PanelWidth    int32 `json:"panel_width"`
	OffsetX       int32 `json:"offset_x"`
	OffsetY       int32 `json:"offset_y"`
}

type XCalibration struct {
	X      float32 `json:"x"`
	Offset int32   `json:"offset"`
}

func CalculateRequiredHeight(data PriceView, mode int32, compactMode int32) int32 {
	y := int32(94)

	if mode == compactMode {
		y += 5
		if data.LastSold != "" {
			y += 17
		}
		if data.LowestSell != "" {
			y += 17
		}
		if data.HighestBuy != "" {
			y += 17
		}
		if data.WeeklyAvg != "" {
			y += 17
		}
		if data.DailySales != "" {
			y += 17
		}
	} else {
		y += 3
		if data.DealTag != "" {
			y += 18
		}
		y += 2

		if data.LastSold != "" {
			y += 17
		}
		if data.LowestSell != "" {
			y += 17
		}
		if data.HighestBuy != "" {
			y += 17
		}
		if data.WeeklyAvg != "" {
			y += 17
		}
		if data.SaleP75 != "" {
			y += 17
		}
		if data.Spread != "" {
			y += 17
		}
		if data.Trend != "" {
			y += 17
		}
		if data.DailySales != "" {
			y += 17
		}
		if data.Orders != "" {
			y += 17
		}
	}

	if data.Updated != "" {
		y += 12
	}
	if data.FallbackNotice != "" {
		y += 12
	}

	y += 34
	return y
}

func ParsePriceView(text string, suggestedKey string) PriceView {
	lines := splitOverlayLines(text)
	view := PriceView{Suggested: strings.TrimSpace(text)}
	if len(lines) == 0 {
		return view
	}

	lineMap := make(map[string]string, len(lines))
	for _, line := range lines {
		if idx := strings.Index(line, ":"); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			lineMap[key] = val
		}
	}

	if suggested, ok := lineMap[suggestedKey]; ok {
		parts := strings.SplitN(suggested, " [", 2)
		view.Suggested = strings.TrimSpace(parts[0])
		if len(parts) > 1 {
			view.Confidence = strings.TrimRight(parts[1], "]")
		}
	}
	if v, ok := lineMap["Last Sold"]; ok {
		view.LastSold = v
	}
	if v, ok := lineMap["Lowest Sell"]; ok {
		view.LowestSell = v
	}
	if v, ok := lineMap["Highest Buy"]; ok {
		view.HighestBuy = v
	}
	if v, ok := lineMap["Weekly Avg"]; ok {
		view.WeeklyAvg = v
	}
	if v, ok := lineMap["Daily Sales"]; ok {
		view.DailySales = v
	}
	if v, ok := lineMap["Updated"]; ok {
		view.Updated = v
	}
	if v, ok := lineMap["DealTag"]; ok {
		view.DealTag = v
	}
	if v, ok := lineMap["Sale P75"]; ok {
		view.SaleP75 = v
	}
	if v, ok := lineMap["Spread"]; ok {
		view.Spread = v
	}
	if v, ok := lineMap["Trend"]; ok {
		view.Trend = v
	}
	if v, ok := lineMap["Orders"]; ok {
		view.Orders = v
	}
	return view
}

func splitOverlayLines(text string) []string {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	rawLines := strings.Split(normalized, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func InsetRect(rect win32.RECT, dx int32, dy int32) win32.RECT {
	return win32.RECT{
		Left:   rect.Left + dx,
		Top:    rect.Top + dy,
		Right:  rect.Right - dx,
		Bottom: rect.Bottom - dy,
	}
}
