package game

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/nidea1/task-bar-trade-center/internal/overlay"
)

const (
	layoutSchemaVersion = 3

	LayoutSourceRemote           = "remote"
	LayoutSourceCache            = "cache"
	LayoutSourceEmbeddedDefault  = "embedded-default"
	LayoutSourceLocalDevelopment = "local-development"

	LayoutPathEnvironment = "TBTC_GAME_LAYOUT_PATH"
	DefaultLayoutURL      = "https://raw.githubusercontent.com/nidea1/task-bar-trade-center/main/internal/game/game-layout.json"
)

var (
	//go:embed game-layout.json
	embeddedLayoutJSON []byte
)

type layoutDocument struct {
	SchemaVersion int `json:"schema_version"`
	HoveredItem   struct {
		PointerBaseOffset string            `json:"pointer_base_offset"`
		PointerOffsets    []string          `json:"pointer_offsets"`
		ItemPtrOffset     string            `json:"item_ptr_offset"`
		KeyOffset         string            `json:"key_offset"`
		PointerBaseAOB    layoutAOBDocument `json:"pointer_base_aob"`
	} `json:"hovered_item"`
	Tooltip struct {
		XPointerBaseOffset      string            `json:"x_pointer_base_offset"`
		XPointerOffsets         []string          `json:"x_pointer_offsets"`
		XPointerBaseAOB         layoutAOBDocument `json:"x_pointer_base_aob"`
		YPointerBaseOffset      string            `json:"y_pointer_base_offset"`
		YPointerOffsets         []string          `json:"y_pointer_offsets"`
		YPointerBaseAOB         layoutAOBDocument `json:"y_pointer_base_aob"`
		HeightPointerBaseOffset string            `json:"height_pointer_base_offset"`
		HeightPointerOffsets    []string          `json:"height_pointer_offsets"`
		HeightPointerBaseAOB    layoutAOBDocument `json:"height_pointer_base_aob"`
	} `json:"tooltip"`
	PlacementCalibrations []overlay.PlacementCalibration    `json:"placement_calibrations"`
	XCalibrations         []overlay.XCalibration            `json:"x_calibrations"`
	ScaleCalibrations     []overlay.ScaleCalibrationProfile `json:"scale_calibrations"`
	PositionCalibrations  []overlay.PositionCalibration     `json:"position_calibrations"`
}

type layoutAOBDocument struct {
	Pattern              string `json:"pattern"`
	DisplacementOffset   int    `json:"displacement_offset"`
	InstructionEndOffset int    `json:"instruction_end_offset"`
}

type GameLayout struct {
	HoveredItemPointerBaseOffset uintptr
	HoveredItemPointerOffsets    []uintptr
	HoveredItemItemPtrOffset     uintptr
	HoveredItemKeyOffset         uintptr
	HoveredItemPointerBaseAOB    AOBPattern

	TooltipXPointerBaseOffset      uintptr
	TooltipXPointerOffsets         []uintptr
	TooltipXPointerBaseAOB         AOBPattern
	TooltipYPointerBaseOffset      uintptr
	TooltipYPointerOffsets         []uintptr
	TooltipYPointerBaseAOB         AOBPattern
	TooltipHeightPointerBaseOffset uintptr
	TooltipHeightPointerOffsets    []uintptr
	TooltipHeightPointerBaseAOB    AOBPattern

	PlacementCalibrations []overlay.PlacementCalibration
	XCalibrations         []overlay.XCalibration
	ScaleCalibrations     []overlay.ScaleCalibrationProfile
	PositionCalibrations  []overlay.PositionCalibration
}

func EmbeddedLayoutJSON() []byte {
	return append([]byte(nil), embeddedLayoutJSON...)
}

func ParseGameLayout(raw []byte) (GameLayout, error) {
	var document layoutDocument
	if err := json.Unmarshal(raw, &document); err != nil {
		return GameLayout{}, err
	}
	if document.SchemaVersion != layoutSchemaVersion {
		return GameLayout{}, fmt.Errorf("unsupported schema_version %d", document.SchemaVersion)
	}
	if document.PlacementCalibrations == nil {
		return GameLayout{}, fmt.Errorf("placement_calibrations is required")
	}

	hoveredBase, err := parseLayoutOffset("hovered_item.pointer_base_offset", document.HoveredItem.PointerBaseOffset)
	if err != nil {
		return GameLayout{}, err
	}
	hoveredOffsets, err := parseLayoutOffsets("hovered_item.pointer_offsets", document.HoveredItem.PointerOffsets)
	if err != nil {
		return GameLayout{}, err
	}
	hoveredItemPtrOffset := uintptr(0)
	if document.HoveredItem.ItemPtrOffset != "" {
		value, err := parseLayoutOffset("hovered_item.item_ptr_offset", document.HoveredItem.ItemPtrOffset)
		if err != nil {
			return GameLayout{}, err
		}
		hoveredItemPtrOffset = value
	}
	hoveredKeyOffset, err := parseLayoutOffset("hovered_item.key_offset", document.HoveredItem.KeyOffset)
	if err != nil {
		return GameLayout{}, err
	}
	hoveredPointerBaseAOB, err := parseOptionalAOBPattern(
		"hovered_item.pointer_base_aob",
		document.HoveredItem.PointerBaseAOB.Pattern,
		document.HoveredItem.PointerBaseAOB.DisplacementOffset,
		document.HoveredItem.PointerBaseAOB.InstructionEndOffset,
	)
	if err != nil {
		return GameLayout{}, err
	}

	xBase, err := parseLayoutOffset("tooltip.x_pointer_base_offset", document.Tooltip.XPointerBaseOffset)
	if err != nil {
		return GameLayout{}, err
	}
	xOffsets, err := parseLayoutOffsets("tooltip.x_pointer_offsets", document.Tooltip.XPointerOffsets)
	if err != nil {
		return GameLayout{}, err
	}
	xPointerBaseAOB, err := parseOptionalAOBPattern(
		"tooltip.x_pointer_base_aob",
		document.Tooltip.XPointerBaseAOB.Pattern,
		document.Tooltip.XPointerBaseAOB.DisplacementOffset,
		document.Tooltip.XPointerBaseAOB.InstructionEndOffset,
	)
	if err != nil {
		return GameLayout{}, err
	}
	yBase, err := parseLayoutOffset("tooltip.y_pointer_base_offset", document.Tooltip.YPointerBaseOffset)
	if err != nil {
		return GameLayout{}, err
	}
	yOffsets, err := parseLayoutOffsets("tooltip.y_pointer_offsets", document.Tooltip.YPointerOffsets)
	if err != nil {
		return GameLayout{}, err
	}
	yPointerBaseAOB, err := parseOptionalAOBPattern(
		"tooltip.y_pointer_base_aob",
		document.Tooltip.YPointerBaseAOB.Pattern,
		document.Tooltip.YPointerBaseAOB.DisplacementOffset,
		document.Tooltip.YPointerBaseAOB.InstructionEndOffset,
	)
	if err != nil {
		return GameLayout{}, err
	}
	heightBase, err := parseLayoutOffset("tooltip.height_pointer_base_offset", document.Tooltip.HeightPointerBaseOffset)
	if err != nil {
		return GameLayout{}, err
	}
	heightOffsets, err := parseLayoutOffsets("tooltip.height_pointer_offsets", document.Tooltip.HeightPointerOffsets)
	if err != nil {
		return GameLayout{}, err
	}
	heightPointerBaseAOB, err := parseOptionalAOBPattern(
		"tooltip.height_pointer_base_aob",
		document.Tooltip.HeightPointerBaseAOB.Pattern,
		document.Tooltip.HeightPointerBaseAOB.DisplacementOffset,
		document.Tooltip.HeightPointerBaseAOB.InstructionEndOffset,
	)
	if err != nil {
		return GameLayout{}, err
	}

	for index, calibration := range document.PlacementCalibrations {
		if calibration.TooltipHeight <= 0 || calibration.PanelWidth <= 0 {
			return GameLayout{}, fmt.Errorf("placement_calibrations[%d] has invalid dimensions", index)
		}
	}

	scaleCalibrations, err := normalizeScaleCalibrations(document.ScaleCalibrations)
	if err != nil {
		return GameLayout{}, err
	}

	return GameLayout{
		HoveredItemPointerBaseOffset:   hoveredBase,
		HoveredItemPointerOffsets:      hoveredOffsets,
		HoveredItemItemPtrOffset:       hoveredItemPtrOffset,
		HoveredItemKeyOffset:           hoveredKeyOffset,
		HoveredItemPointerBaseAOB:      hoveredPointerBaseAOB,
		TooltipXPointerBaseOffset:      xBase,
		TooltipXPointerOffsets:         xOffsets,
		TooltipXPointerBaseAOB:         xPointerBaseAOB,
		TooltipYPointerBaseOffset:      yBase,
		TooltipYPointerOffsets:         yOffsets,
		TooltipYPointerBaseAOB:         yPointerBaseAOB,
		TooltipHeightPointerBaseOffset: heightBase,
		TooltipHeightPointerOffsets:    heightOffsets,
		TooltipHeightPointerBaseAOB:    heightPointerBaseAOB,
		PlacementCalibrations:          append([]overlay.PlacementCalibration(nil), document.PlacementCalibrations...),
		XCalibrations:                  append([]overlay.XCalibration(nil), document.XCalibrations...),
		ScaleCalibrations:              scaleCalibrations,
		PositionCalibrations:           append([]overlay.PositionCalibration(nil), document.PositionCalibrations...),
	}, nil
}

func normalizeScaleCalibrations(
	profiles []overlay.ScaleCalibrationProfile,
) ([]overlay.ScaleCalibrationProfile, error) {
	if len(profiles) == 0 {
		return nil, nil
	}

	result := make([]overlay.ScaleCalibrationProfile, 0, len(profiles))
	seenScales := make(map[int32]struct{}, len(profiles))

	for profileIndex, profile := range profiles {
		if profile.ScalePercent <= 0 {
			return nil, fmt.Errorf("scale_calibrations[%d].scale_percent must be positive", profileIndex)
		}
		if _, exists := seenScales[profile.ScalePercent]; exists {
			return nil, fmt.Errorf("scale_calibrations contains duplicate scale_percent %d", profile.ScalePercent)
		}
		seenScales[profile.ScalePercent] = struct{}{}

		if len(profile.XAnchors) < 2 {
			return nil, fmt.Errorf("scale_calibrations[%d].x_anchors must contain at least two anchors", profileIndex)
		}

		anchors := append([]overlay.XCalibrationAnchor(nil), profile.XAnchors...)
		sort.Slice(anchors, func(left, right int) bool {
			return anchors[left].X < anchors[right].X
		})

		normalizedAnchors := make([]overlay.XCalibrationAnchor, 0, len(anchors))
		for anchorIndex, anchor := range anchors {
			if len(normalizedAnchors) > 0 && anchor.X == normalizedAnchors[len(normalizedAnchors)-1].X {
				previous := normalizedAnchors[len(normalizedAnchors)-1]
				if anchor.Offset != previous.Offset {
					return nil, fmt.Errorf(
						"scale_calibrations[%d].x_anchors[%d] conflicts with another anchor at x %.6f",
						profileIndex,
						anchorIndex,
						anchor.X,
					)
				}
				continue
			}
			normalizedAnchors = append(normalizedAnchors, anchor)
		}

		if len(normalizedAnchors) < 2 {
			return nil, fmt.Errorf("scale_calibrations[%d] must contain at least two unique x anchors", profileIndex)
		}

		result = append(result, overlay.ScaleCalibrationProfile{
			ScalePercent: profile.ScalePercent,
			YOffset:      profile.YOffset,
			XAnchors:     normalizedAnchors,
		})
	}

	sort.Slice(result, func(left, right int) bool {
		return result[left].ScalePercent < result[right].ScalePercent
	})
	return result, nil
}

func ApplyEmbeddedAOBFallback(layout GameLayout, embeddedDefaults []byte) (GameLayout, error) {
	embeddedLayout, err := ParseGameLayout(embeddedDefaults)
	if err != nil {
		return GameLayout{}, fmt.Errorf("embedded AOB fallback is invalid: %w", err)
	}
	if !layout.HoveredItemPointerBaseAOB.configured() {
		layout.HoveredItemPointerBaseAOB = embeddedLayout.HoveredItemPointerBaseAOB
	}
	if !layout.TooltipXPointerBaseAOB.configured() {
		layout.TooltipXPointerBaseAOB = embeddedLayout.TooltipXPointerBaseAOB
	}
	if !layout.TooltipYPointerBaseAOB.configured() {
		layout.TooltipYPointerBaseAOB = embeddedLayout.TooltipYPointerBaseAOB
	}
	if !layout.TooltipHeightPointerBaseAOB.configured() {
		layout.TooltipHeightPointerBaseAOB = embeddedLayout.TooltipHeightPointerBaseAOB
	}
	return layout, nil
}

func parseLayoutOffsets(name string, values []string) ([]uintptr, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("%s must not be empty", name)
	}

	offsets := make([]uintptr, len(values))
	for index, value := range values {
		offset, err := parseLayoutOffset(fmt.Sprintf("%s[%d]", name, index), value)
		if err != nil {
			return nil, err
		}
		offsets[index] = offset
	}
	return offsets, nil
}

func parseLayoutOffset(name, value string) (uintptr, error) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(strings.ToLower(value), "0x") {
		return 0, fmt.Errorf("%s must be a hexadecimal string", name)
	}

	parsed, err := strconv.ParseUint(value[2:], 16, strconv.IntSize)
	if err != nil {
		return 0, fmt.Errorf("%s is invalid: %w", name, err)
	}
	return uintptr(parsed), nil
}
