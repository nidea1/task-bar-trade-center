package playerdata

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nidea1/task-bar-trade-center/internal/il2cpp"
)

const (
	chestRuneListScanLimit = 5000

	runeSaveDataKeyOffset   = 0x10
	runeSaveDataLevelOffset = 0x14
)

type chestRuneKind string

const (
	chestRuneKindCommon chestRuneKind = "common"
	chestRuneKindStage  chestRuneKind = "stage"
)

type chestRuneDefinition struct {
	IDs             []int32
	SecondsPerStage int
	MaxStage        int
	Kind            chestRuneKind
	Name            string
}

var chestRuneDefinitions = []chestRuneDefinition{
	{
		IDs:             []int32{130021},
		SecondsPerStage: 3,
		MaxStage:        3,
		Kind:            chestRuneKindCommon,
		Name:            "Common chest auto-open rune",
	},
	{
		IDs:             []int32{190301, 190501, 1905011},
		SecondsPerStage: 2,
		MaxStage:        5,
		Kind:            chestRuneKindCommon,
		Name:            "Common chest auto-open rune",
	},
	{
		IDs:             []int32{150011},
		SecondsPerStage: 5,
		MaxStage:        3,
		Kind:            chestRuneKindStage,
		Name:            "Stage chest auto-open rune",
	},
	{
		IDs:             []int32{1905021, 190502, 190302},
		SecondsPerStage: 4,
		MaxStage:        5,
		Kind:            chestRuneKindStage,
		Name:            "Stage chest auto-open rune",
	},
}

type chestRuneEntry struct {
	index  int
	object uintptr
	runeID int32
	level  int
	def    chestRuneDefinition
}

type chestRuneListScan struct {
	listOffset uintptr
	listSize   int
	entries    []chestRuneEntry
	score      int
}

type chestRuneScanResult struct {
	entries     []chestRuneEntry
	diagnostics []string
	resolved    map[chestRuneKind]bool
}

func (resolver *Resolver) readChestAutomationWithLayout(memory Memory, object uintptr, layout saveDataLayout) ChestAutomation {
	definitionsByID := chestRuneDefinitionsByID()
	result := resolver.scanChestRuneLists(memory, object, definitionsByID)
	automation := ChestAutomation{
		Diagnostics: result.diagnostics,
	}

	for _, entry := range result.entries {
		reduction := entry.level * entry.def.SecondsPerStage
		switch entry.def.Kind {
		case chestRuneKindCommon:
			automation.CommonRuneReductionSeconds += reduction
			if entry.runeID == 130021 && entry.level > 0 {
				automation.HasCommonAutoOpenRune = true
			}
		case chestRuneKindStage:
			automation.StageRuneReductionSeconds += reduction
			if entry.runeID == 150011 && entry.level > 0 {
				automation.HasStageAutoOpenRune = true
			}
		}
	}
	automation.CommonReductionBuffSummary = formatChestRuneBuffSummaryForKind(result.entries, chestRuneKindCommon)
	automation.StageReductionBuffSummary = formatChestRuneBuffSummaryForKind(result.entries, chestRuneKindStage)
	automation.ReductionBuffSummary = formatChestRuneBuffSummary(automation.CommonReductionBuffSummary, automation.StageReductionBuffSummary)

	maxReductions := maxChestRuneReductionsByKind()
	automation.CommonRuneReductionSeconds = clampChestRuneReduction(automation.CommonRuneReductionSeconds, maxReductions[chestRuneKindCommon])
	automation.StageRuneReductionSeconds = clampChestRuneReduction(automation.StageRuneReductionSeconds, maxReductions[chestRuneKindStage])

	commonConfigured := chestRuneKindConfigured(chestRuneKindCommon)
	stageConfigured := chestRuneKindConfigured(chestRuneKindStage)
	commonResolved := commonConfigured && result.resolved[chestRuneKindCommon]
	stageResolved := stageConfigured && result.resolved[chestRuneKindStage]
	automation.RuneResolved = commonResolved && stageResolved

	if !commonConfigured {
		automation.Diagnostics = append(automation.Diagnostics, "common chest rune definitions are not configured")
	} else if !commonResolved {
		automation.Diagnostics = append(automation.Diagnostics, "configured common chest RuneSaveData entries were not found")
	}
	if !stageConfigured {
		automation.Diagnostics = append(automation.Diagnostics, "stage chest rune definitions are not configured")
	} else if !stageResolved {
		automation.Diagnostics = append(automation.Diagnostics, "configured stage chest RuneSaveData entries were not found")
	}

	return automation
}

func (resolver *Resolver) scanChestRuneLists(memory Memory, object uintptr, definitionsByID map[int32]chestRuneDefinition) chestRuneScanResult {
	result := chestRuneScanResult{
		resolved: make(map[chestRuneKind]bool),
	}
	if len(definitionsByID) == 0 {
		result.diagnostics = append(result.diagnostics, "no chest rune definitions are configured")
		return result
	}

	lists := discoverObjectLists(memory, object)
	runeSaveDataClasses := resolver.resolveRuneSaveDataClasses(memory)
	bestByKind := make(map[chestRuneKind]chestRuneListScan)
	for _, list := range lists {
		next := analyzeRuneSaveDataList(memory, list, definitionsByID, runeSaveDataClasses)
		for kind := range next.kinds() {
			subset := next.onlyKind(kind)
			if subset.betterThan(bestByKind[kind]) {
				bestByKind[kind] = subset
			}
		}
	}

	if len(bestByKind) == 0 {
		result.diagnostics = append(result.diagnostics, fmt.Sprintf(
			"no configured RuneSaveData entries found; scanned_lists=%d class_filter=%t key_offset=0x%X level_offset=0x%X",
			len(lists),
			len(runeSaveDataClasses) > 0,
			runeSaveDataKeyOffset,
			runeSaveDataLevelOffset,
		))
		return result
	}

	kinds := make([]string, 0, len(bestByKind))
	for kind := range bestByKind {
		kinds = append(kinds, string(kind))
	}
	sort.Strings(kinds)
	for _, kindName := range kinds {
		kind := chestRuneKind(kindName)
		scan := bestByKind[kind]
		result.entries = append(result.entries, scan.entries...)
		for _, entry := range scan.entries {
			result.resolved[entry.def.Kind] = true
		}
		result.diagnostics = append(result.diagnostics, fmt.Sprintf(
			"selected %s chest RuneSaveData list offset=0x%X size=%d entries=%d class_filter=%t key_offset=0x%X level_offset=0x%X",
			kind,
			scan.listOffset,
			scan.listSize,
			len(scan.entries),
			len(runeSaveDataClasses) > 0,
			runeSaveDataKeyOffset,
			runeSaveDataLevelOffset,
		))
	}
	return result
}

func (resolver *Resolver) resolveRuneSaveDataClasses(memory Memory) map[uintptr]struct{} {
	classes, ok := resolver.resolveClassByName(memory, "RuneSaveData")
	if !ok {
		return nil
	}
	result := make(map[uintptr]struct{}, len(classes))
	for _, classPtr := range classes {
		result[classPtr] = struct{}{}
	}
	return result
}

func analyzeRuneSaveDataList(memory Memory, list objectListCandidate, definitionsByID map[int32]chestRuneDefinition, runeSaveDataClasses map[uintptr]struct{}) chestRuneListScan {
	entries := readRuneSaveDataEntries(memory, list.info, definitionsByID, runeSaveDataClasses)
	if len(entries) == 0 {
		return chestRuneListScan{}
	}

	scan := chestRuneListScan{
		listOffset: list.offset,
		listSize:   list.info.size,
		entries:    entries,
	}
	scan.score = chestRuneListScore(scan)
	return scan
}

func readRuneSaveDataEntries(memory Memory, list listInfo, definitionsByID map[int32]chestRuneDefinition, runeSaveDataClasses map[uintptr]struct{}) []chestRuneEntry {
	if !list.ok || list.size <= 0 {
		return nil
	}

	limit := sampleLimit(list.size, chestRuneListScanLimit)
	entriesByID := make(map[int32]chestRuneEntry)
	for index := 0; index < limit; index++ {
		object, ok := listObjectAt(memory, list, index)
		if !ok {
			continue
		}
		if len(runeSaveDataClasses) > 0 && !objectHasAnyClass(memory, object, runeSaveDataClasses) {
			continue
		}
		runeID, ok := memory.ReadInt32(object + runeSaveDataKeyOffset)
		if !ok {
			continue
		}
		definition, exists := definitionsByID[runeID]
		if !exists {
			continue
		}
		level, ok := memory.ReadInt32(object + runeSaveDataLevelOffset)
		if !ok || level < 0 || level > int32(definition.MaxStage) {
			continue
		}
		entry := chestRuneEntry{
			index:  index,
			object: object,
			runeID: runeID,
			level:  int(level),
			def:    definition,
		}
		if existing, exists := entriesByID[runeID]; !exists || entry.level > existing.level {
			entriesByID[runeID] = entry
		}
	}

	ids := make([]int, 0, len(entriesByID))
	for runeID := range entriesByID {
		ids = append(ids, int(runeID))
	}
	sort.Ints(ids)
	entries := make([]chestRuneEntry, 0, len(ids))
	for _, runeID := range ids {
		entries = append(entries, entriesByID[int32(runeID)])
	}
	return entries
}

func objectHasAnyClass(memory Memory, object uintptr, classes map[uintptr]struct{}) bool {
	classPtr, ok := memory.ReadUintptr(object + il2cpp.ObjectClassOffset)
	if !ok {
		return false
	}
	_, exists := classes[classPtr]
	return exists
}

func chestRuneListScore(scan chestRuneListScan) int {
	score := len(scan.entries) * 100
	totalLevels := 0
	for _, entry := range scan.entries {
		totalLevels += entry.level
	}
	return score + totalLevels*20
}

func (scan chestRuneListScan) betterThan(other chestRuneListScan) bool {
	if len(scan.entries) == 0 {
		return false
	}
	if len(other.entries) == 0 {
		return true
	}
	if scan.score != other.score {
		return scan.score > other.score
	}
	return len(scan.entries) > len(other.entries)
}

func (scan chestRuneListScan) kinds() map[chestRuneKind]struct{} {
	kinds := make(map[chestRuneKind]struct{})
	for _, entry := range scan.entries {
		kinds[entry.def.Kind] = struct{}{}
	}
	return kinds
}

func (scan chestRuneListScan) onlyKind(kind chestRuneKind) chestRuneListScan {
	filtered := scan
	filtered.entries = nil
	for _, entry := range scan.entries {
		if entry.def.Kind == kind {
			filtered.entries = append(filtered.entries, entry)
		}
	}
	filtered.score = chestRuneListScore(filtered)
	return filtered
}

func chestRuneDefinitionsByID() map[int32]chestRuneDefinition {
	definitionsByID := make(map[int32]chestRuneDefinition)
	for _, definition := range chestRuneDefinitions {
		for _, id := range definition.IDs {
			definitionsByID[id] = definition
		}
	}
	return definitionsByID
}

func maxChestRuneReductionsByKind() map[chestRuneKind]int {
	maxReductions := make(map[chestRuneKind]int)
	for _, definition := range chestRuneDefinitions {
		maxReductions[definition.Kind] += len(definition.IDs) * definition.MaxStage * definition.SecondsPerStage
	}
	return maxReductions
}

func chestRuneKindConfigured(kind chestRuneKind) bool {
	for _, definition := range chestRuneDefinitions {
		if definition.Kind == kind {
			return true
		}
	}
	return false
}

func clampChestRuneReduction(value int, maxValue int) int {
	if maxValue > 0 && value > maxValue {
		return maxValue
	}
	return value
}

func formatChestRuneBuffSummaryForKind(entries []chestRuneEntry, kind chestRuneKind) string {
	parts := make([]string, 0)
	for _, entry := range entries {
		if entry.def.Kind != kind {
			continue
		}
		reduction := entry.level * entry.def.SecondsPerStage
		parts = append(parts, fmt.Sprintf("%d lv%d -%ds", entry.runeID, entry.level, reduction))
	}
	return strings.Join(parts, ",")
}

func formatChestRuneBuffSummary(common string, stage string) string {
	sections := make([]string, 0, 2)
	if common != "" {
		sections = append(sections, fmt.Sprintf("%s:%s", chestRuneKindCommon, common))
	}
	if stage != "" {
		sections = append(sections, fmt.Sprintf("%s:%s", chestRuneKindStage, stage))
	}
	return strings.Join(sections, "; ")
}
