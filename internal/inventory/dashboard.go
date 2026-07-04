package inventory

import (
	"math"
	"sort"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/playerdata"
)

type DashboardOptions struct {
	MarketScope           string
	CurrencyCode          string
	PricePrefix           string
	PriceSuffix           string
	Refresh               RefreshStatus
	Now                   time.Time
	InventoryFillSchedule ChestDropSchedule
	InventoryFillTiming   ChestTimingConfig
}

const (
	stashSlotsPerPage = 100

	commonChestDropSeconds         = 240
	stageChestDropSeconds          = 360
	commonChestBaseAutoOpenSeconds = 300
	stageChestBaseAutoOpenSeconds  = 600
	minChestAutoOpenSeconds        = 1

	inventoryFillEventQueueModel = "event_queue_first_drop_auto_open"
)

type groupAccumulator struct {
	item     DashboardItem
	seen     map[string]struct{}
	equipped bool
}

func BuildDashboard(snapshot playerdata.InventorySnapshot, catalog map[int]ItemDescriptor, quotes QuoteProvider, options DashboardOptions) DashboardState {
	now := options.Now
	if now.IsZero() {
		now = time.Now()
	}
	groups := make(map[int]*groupAccumulator)
	stashPageCount := dashboardStashPageCount(snapshot)
	totals := DashboardTotals{
		TotalItemCount: len(snapshot.Items),
		StashPageCount: stashPageCount,
		HeroEquippedValues: map[int]float64{
			1: 0.0,
			2: 0.0,
			3: 0.0,
			4: 0.0,
			5: 0.0,
			6: 0.0,
		},
		StashPageValues: make(map[int]float64),
		StashPageCounts: make(map[int]int),
	}
	for page := 1; page <= stashPageCount; page++ {
		totals.StashPageValues[page] = 0
		totals.StashPageCounts[page] = 0
	}

	for _, owned := range snapshot.Items {
		if owned.Location == playerdata.LocationStash {
			page := stashPageForSlot(owned.SlotIndex)
			totals.StashPageCounts[page]++
			if _, exists := totals.StashPageValues[page]; !exists {
				totals.StashPageValues[page] = 0
			}
		}
		desc := catalog[owned.ItemID]
		if !owned.Marketable || !desc.Marketable {
			continue
		}
		totals.MarketableItemCount++
		acc := groups[owned.ItemID]
		if acc == nil {
			acc = &groupAccumulator{
				item: DashboardItem{
					ItemID:         owned.ItemID,
					Name:           desc.Name,
					MarketHashName: desc.MarketHashName,
					MarketURL:      desc.MarketURL,
					IconURL:        desc.IconURL,
					Grade:          desc.Grade,
					Type:           desc.Type,
					Gear:           desc.Gear,
				},
				seen: make(map[string]struct{}),
			}
			groups[owned.ItemID] = acc
		}
		acc.item.Count++
		if owned.Location == playerdata.LocationEquipped {
			acc.equipped = true
			acc.item.Equipped = true
		}
		acc.seen[string(owned.Location)] = struct{}{}
		quote, ok := quotes.Quote(owned.ItemID)
		if ok && (quote.HasSuggested || quote.HasInstant) {
			acc.item.HasPrice = true
			acc.item.Suggested = quote.Suggested
			acc.item.Instant = quote.Instant
			acc.item.WeeklyAveragePrice = quote.WeeklyAveragePrice
			acc.item.SpreadPercent = quote.SpreadPercent
			acc.item.DailySalesVolume = quote.DailySalesVolume
			acc.item.BuyOrderCount = quote.BuyOrderCount
			acc.item.SellOrderCount = quote.SellOrderCount
			acc.item.HasWeeklyAverage = quote.HasWeeklyAverage
			acc.item.HasSpread = quote.HasSpread
			acc.item.HasDailySales = quote.HasDailySales
			acc.item.HasOrderBook = quote.HasOrderBook
			acc.item.Confidence = quote.Confidence
			acc.item.HasConfidence = quote.HasConfidence
			acc.item.PricePrefix = quote.PricePrefix
			acc.item.PriceSuffix = quote.PriceSuffix
			acc.item.UpdatedAt = quote.UpdatedAt
			if quote.HasSuggested {
				acc.item.TotalSuggested += quote.Suggested
				totals.SuggestedListingValue += quote.Suggested
				addLocationValue(&totals, owned.Location, quote.Suggested)
				if owned.Location == playerdata.LocationStash {
					page := stashPageForSlot(owned.SlotIndex)
					totals.StashPageValues[page] += quote.Suggested
				}
				if owned.Location == playerdata.LocationEquipped && owned.EquippedHeroKey > 0 {
					classID := mapHeroKeyToClassID(owned.EquippedHeroKey)
					if _, exists := totals.HeroEquippedValues[classID]; exists {
						totals.HeroEquippedValues[classID] += quote.Suggested
					}
				}
			}
			if quote.HasInstant {
				acc.item.TotalInstant += quote.Instant
				totals.InstantSellValue += quote.Instant
			}
			totals.PricedItemCount++
		} else {
			totals.UnknownItemCount++
		}
	}
	items := make([]DashboardItem, 0, len(groups))
	for _, acc := range groups {
		acc.item.Location = joinLocations(acc.seen)
		acc.item.SellScore, acc.item.SellReasons = sellNowScore(acc.item)
		items = append(items, acc.item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].TotalSuggested == items[j].TotalSuggested {
			return items[i].Name < items[j].Name
		}
		return items[i].TotalSuggested > items[j].TotalSuggested
	})

	state := DashboardState{
		UpdatedAt:      now.Format(time.RFC3339),
		SnapshotReadAt: snapshot.ReadAt.Format(time.RFC3339),
		MarketScope:    options.MarketScope,
		CurrencyCode:   options.CurrencyCode,
		PricePrefix:    options.PricePrefix,
		PriceSuffix:    options.PriceSuffix,
		Gold:           snapshot.Gold,
		Totals:         totals,
		Items:          items,
		MostValuable:   limitItems(items, 25),
		BestToSellNow:  bestItemsToSellNow(items),
		Duplicates:     duplicateItems(items, 25),
		Equipped:       equippedItems(items, 25),
		MissingPrices:  missingPriceItems(items, 25),
		Refresh:        options.Refresh,
		TradeSlots:     snapshot.TradeSlots,
		InventoryFill:  BuildInventoryFillEstimateWithTiming(snapshot, options.InventoryFillTiming, options.InventoryFillSchedule),
		ChestCounts:    snapshot.ChestCounts,
	}
	return state
}

func DefaultChestTimingConfig() ChestTimingConfig {
	return ChestTimingConfig{
		CommonDropSeconds:         commonChestDropSeconds,
		StageDropSeconds:          stageChestDropSeconds,
		CommonBaseAutoOpenSeconds: commonChestBaseAutoOpenSeconds,
		StageBaseAutoOpenSeconds:  stageChestBaseAutoOpenSeconds,
	}
}

func NormalizeChestTimingConfig(config ChestTimingConfig) ChestTimingConfig {
	defaults := DefaultChestTimingConfig()
	if config.CommonDropSeconds <= 0 {
		config.CommonDropSeconds = defaults.CommonDropSeconds
	}
	if config.StageDropSeconds <= 0 {
		config.StageDropSeconds = defaults.StageDropSeconds
	}
	if config.CommonBaseAutoOpenSeconds <= 0 {
		config.CommonBaseAutoOpenSeconds = defaults.CommonBaseAutoOpenSeconds
	}
	if config.StageBaseAutoOpenSeconds <= 0 {
		config.StageBaseAutoOpenSeconds = defaults.StageBaseAutoOpenSeconds
	}
	return config
}

func BuildInventoryFillEstimate(snapshot playerdata.InventorySnapshot, schedules ...ChestDropSchedule) InventoryFillEstimate {
	return BuildInventoryFillEstimateWithTiming(snapshot, ChestTimingConfig{}, schedules...)
}

func BuildInventoryFillEstimateWithTiming(snapshot playerdata.InventorySnapshot, timing ChestTimingConfig, schedules ...ChestDropSchedule) InventoryFillEstimate {
	timing = NormalizeChestTimingConfig(timing)
	slotCount := snapshot.InventorySlots.Total
	usedSlots := snapshot.InventorySlots.Used
	if usedSlots == 0 {
		usedSlots = countInventoryItems(snapshot.Items)
	}
	if slotCount < usedSlots {
		slotCount = usedSlots
	}
	freeSlots := snapshot.InventorySlots.Free
	if freeSlots == 0 && slotCount > usedSlots {
		freeSlots = slotCount - usedSlots
	}
	if freeSlots < 0 {
		freeSlots = 0
	}

	commonReduction := positiveSeconds(snapshot.ChestAutomation.CommonRuneReductionSeconds)
	stageReduction := positiveSeconds(snapshot.ChestAutomation.StageRuneReductionSeconds)

	var commonAutoOpen int
	if snapshot.ChestAutomation.HasCommonAutoOpenRune {
		commonAutoOpen = reducedDuration(timing.CommonBaseAutoOpenSeconds, commonReduction)
	} else {
		commonAutoOpen = 0
	}

	var stageAutoOpen int
	if snapshot.ChestAutomation.HasStageAutoOpenRune {
		stageAutoOpen = reducedDuration(timing.StageBaseAutoOpenSeconds, stageReduction)
	} else {
		stageAutoOpen = 0
	}

	commonFillPerSecond := chestItemFillRate(timing.CommonDropSeconds, commonAutoOpen)
	stageFillPerSecond := chestItemFillRate(timing.StageDropSeconds, stageAutoOpen)
	netPerSecond := commonFillPerSecond + stageFillPerSecond
	schedule := normalizedChestDropSchedule(firstChestDropSchedule(schedules), timing)

	estimate := InventoryFillEstimate{
		Available:                  slotCount > 0,
		FillModel:                  inventoryFillEventQueueModel,
		SlotCount:                  slotCount,
		UsedSlots:                  usedSlots,
		FreeSlots:                  freeSlots,
		CommonDropSeconds:          timing.CommonDropSeconds,
		StageDropSeconds:           timing.StageDropSeconds,
		NextCommonDropSeconds:      schedule.NextCommonDropSeconds,
		NextStageDropSeconds:       schedule.NextStageDropSeconds,
		CommonAutoOpenSeconds:      commonAutoOpen,
		StageAutoOpenSeconds:       stageAutoOpen,
		CommonRuneReductionSeconds: commonReduction,
		StageRuneReductionSeconds:  stageReduction,
		NetFillPerHour:             netPerSecond * 3600,
		RuneResolved:               snapshot.ChestAutomation.RuneResolved,
		HasCommonAutoOpenRune:      snapshot.ChestAutomation.HasCommonAutoOpenRune,
		HasStageAutoOpenRune:       snapshot.ChestAutomation.HasStageAutoOpenRune,
		CommonDropObserved:         schedule.CommonDropObserved,
		StageDropObserved:          schedule.StageDropObserved,
		CommonDropReady:            schedule.CommonDropReady,
		StageDropReady:             schedule.StageDropReady,
		CommonChestCount:           snapshot.ChestCounts.NormalChests,
		StageChestCount:            snapshot.ChestCounts.StageChests,
	}
	if timeToFull, fills := timeToFillSlots(freeSlots, commonAutoOpen, stageAutoOpen, netPerSecond, schedule, snapshot.ChestCounts, timing); fills {
		estimate.Fills = true
		estimate.TimeToFullSeconds = timeToFull
	}
	return estimate
}

func firstChestDropSchedule(schedules []ChestDropSchedule) ChestDropSchedule {
	if len(schedules) == 0 {
		return ChestDropSchedule{}
	}
	return schedules[0]
}

func normalizedChestDropSchedule(schedule ChestDropSchedule, timing ChestTimingConfig) ChestDropSchedule {
	if schedule.CommonDropReady {
		schedule.NextCommonDropSeconds = 0
	} else if schedule.NextCommonDropSeconds <= 0 {
		schedule.NextCommonDropSeconds = timing.CommonDropSeconds
	}
	if schedule.StageDropReady {
		schedule.NextStageDropSeconds = 0
	} else if schedule.NextStageDropSeconds <= 0 {
		schedule.NextStageDropSeconds = timing.StageDropSeconds
	}
	return schedule
}

func timeToFillSlots(freeSlots int, commonAutoOpenSeconds int, stageAutoOpenSeconds int, itemRate float64, schedule ChestDropSchedule, chestCounts playerdata.ChestCounts, timing ChestTimingConfig) (int, bool) {
	if freeSlots <= 0 {
		return 0, false
	}
	if itemRate <= 0 {
		return 0, false
	}
	horizon := boundedFillSimulationHorizon(schedule, commonAutoOpenSeconds, stageAutoOpenSeconds, timing)
	horizon += int(math.Ceil(float64(freeSlots)/itemRate)) + maxInt(commonAutoOpenSeconds, stageAutoOpenSeconds)

	return simulateTimeToFillSlots(freeSlots, schedule, commonAutoOpenSeconds, stageAutoOpenSeconds, horizon, chestCounts, timing)
}

type fillQueue struct {
	dropInterval int
	openInterval int
	nextDrop     int
	nextOpen     int
	queued       int
}

func simulateTimeToFillSlots(freeSlots int, schedule ChestDropSchedule, commonAutoOpenSeconds int, stageAutoOpenSeconds int, horizon int, chestCounts playerdata.ChestCounts, timing ChestTimingConfig) (int, bool) {
	initialCommonQueued := chestCounts.NormalChests
	initialStageQueued := chestCounts.StageChests

	commonNextOpen := inactiveRateStart()
	if initialCommonQueued > 0 && commonAutoOpenSeconds > 0 {
		commonNextOpen = commonAutoOpenSeconds
	}
	stageNextOpen := inactiveRateStart()
	if initialStageQueued > 0 && stageAutoOpenSeconds > 0 {
		stageNextOpen = stageAutoOpenSeconds
	}

	queues := []fillQueue{
		{
			dropInterval: timing.CommonDropSeconds,
			openInterval: commonAutoOpenSeconds,
			nextDrop:     schedule.NextCommonDropSeconds,
			nextOpen:     commonNextOpen,
			queued:       initialCommonQueued,
		},
		{
			dropInterval: timing.StageDropSeconds,
			openInterval: stageAutoOpenSeconds,
			nextDrop:     schedule.NextStageDropSeconds,
			nextOpen:     stageNextOpen,
			queued:       initialStageQueued,
		},
	}

	filledSlots := 0
	for {
		nextEvent := inactiveRateStart()
		for _, queue := range queues {
			nextEvent = minInt(nextEvent, queue.nextDrop)
			nextEvent = minInt(nextEvent, queue.nextOpen)
		}
		if nextEvent == inactiveRateStart() || nextEvent > horizon {
			return 0, false
		}

		for i := range queues {
			if queues[i].nextOpen != nextEvent {
				continue
			}
			if queues[i].queued > 0 && queues[i].openInterval > 0 {
				queues[i].queued--
				filledSlots++
			}
			if queues[i].queued > 0 && queues[i].openInterval > 0 {
				queues[i].nextOpen = nextEvent + queues[i].openInterval
			} else {
				queues[i].nextOpen = inactiveRateStart()
			}
		}

		for i := range queues {
			if queues[i].nextDrop != nextEvent {
				continue
			}
			queues[i].queued++
			queues[i].nextDrop = nextEvent + queues[i].dropInterval
			if queues[i].queued == 1 && queues[i].openInterval > 0 {
				queues[i].nextOpen = nextEvent + queues[i].openInterval
			}
		}

		if filledSlots >= freeSlots {
			return nextEvent, true
		}
	}
}

func boundedFillSimulationHorizon(schedule ChestDropSchedule, commonAutoOpenSeconds int, stageAutoOpenSeconds int, timing ChestTimingConfig) int {
	lastFirstOpen := maxInt(
		schedule.NextCommonDropSeconds+commonAutoOpenSeconds,
		schedule.NextStageDropSeconds+stageAutoOpenSeconds,
	)
	return lastFirstOpen + maxInt(timing.CommonDropSeconds, timing.StageDropSeconds) + maxInt(commonAutoOpenSeconds, stageAutoOpenSeconds)
}

func countInventoryItems(items []playerdata.OwnedItem) int {
	count := 0
	for _, item := range items {
		if item.Location == playerdata.LocationInventory {
			count++
		}
	}
	return count
}

func positiveSeconds(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func reducedDuration(baseSeconds int, reductionSeconds int) int {
	seconds := baseSeconds - reductionSeconds
	if seconds < minChestAutoOpenSeconds {
		return minChestAutoOpenSeconds
	}
	return seconds
}

func inactiveRateStart() int {
	return int(^uint(0) >> 1)
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}

func chestItemFillRate(dropSeconds int, autoOpenSeconds int) float64 {
	if dropSeconds <= 0 || autoOpenSeconds <= 0 {
		return 0
	}
	dropRate := 1 / float64(dropSeconds)
	openRate := 1 / float64(autoOpenSeconds)
	if dropRate < openRate {
		return dropRate
	}
	return openRate
}

func dashboardStashPageCount(snapshot playerdata.InventorySnapshot) int {
	pageCount := snapshot.StashPageCount
	for _, owned := range snapshot.Items {
		if owned.Location != playerdata.LocationStash {
			continue
		}
		page := stashPageForSlot(owned.SlotIndex)
		if page > pageCount {
			pageCount = page
		}
	}
	return pageCount
}

func stashPageForSlot(slotIndex int) int {
	if slotIndex < 0 {
		return 1
	}
	return (slotIndex / stashSlotsPerPage) + 1
}

func addLocationValue(totals *DashboardTotals, location playerdata.Location, value float64) {
	switch location {
	case playerdata.LocationInventory:
		totals.InventoryValue += value
	case playerdata.LocationStash:
		totals.StashValue += value
	case playerdata.LocationEquipped:
		totals.EquippedValue += value
	}
}

func joinLocations(locations map[string]struct{}) string {
	order := []string{string(playerdata.LocationEquipped), string(playerdata.LocationStash), string(playerdata.LocationInventory)}
	result := ""
	for _, location := range order {
		if _, exists := locations[location]; !exists {
			continue
		}
		if result != "" {
			result += ", "
		}
		result += location
	}
	return result
}

func limitItems(items []DashboardItem, limit int) []DashboardItem {
	if len(items) <= limit {
		return append([]DashboardItem(nil), items...)
	}
	return append([]DashboardItem(nil), items[:limit]...)
}

func duplicateItems(items []DashboardItem, limit int) []DashboardItem {
	filtered := make([]DashboardItem, 0)
	for _, item := range items {
		if item.Count > 1 {
			filtered = append(filtered, item)
		}
	}
	return limitItems(filtered, limit)
}

func equippedItems(items []DashboardItem, limit int) []DashboardItem {
	filtered := make([]DashboardItem, 0)
	for _, item := range items {
		if item.Equipped {
			filtered = append(filtered, item)
		}
	}
	return limitItems(filtered, limit)
}

func missingPriceItems(items []DashboardItem, limit int) []DashboardItem {
	filtered := make([]DashboardItem, 0)
	for _, item := range items {
		if !item.HasPrice {
			filtered = append(filtered, item)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Count == filtered[j].Count {
			return filtered[i].Name < filtered[j].Name
		}
		return filtered[i].Count > filtered[j].Count
	})
	return limitItems(filtered, limit)
}

func bestItemsToSellNow(items []DashboardItem) []DashboardItem {
	filtered := make([]DashboardItem, 0)
	for _, item := range items {
		if item.SellScore >= 45 {
			filtered = append(filtered, item)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].SellScore == filtered[j].SellScore {
			return filtered[i].TotalSuggested > filtered[j].TotalSuggested
		}
		return filtered[i].SellScore > filtered[j].SellScore
	})
	return filtered
}

func sellNowScore(item DashboardItem) (float64, []string) {
	if !item.HasPrice {
		return 0, nil
	}
	score := 0.0
	reasons := make([]string, 0, 5)

	if item.HasDailySales && item.DailySalesVolume > 0 {
		switch {
		case item.DailySalesVolume >= 100:
			score += 25
		case item.DailySalesVolume >= 30:
			score += 18
		case item.DailySalesVolume >= 10:
			score += 10
		}
		if item.DailySalesVolume >= 10 {
			reasons = append(reasons, "high_daily_sales")
		}
	}

	if item.HasSpread && item.SpreadPercent >= 0 {
		switch {
		case item.SpreadPercent <= 8:
			score += 25
		case item.SpreadPercent <= 15:
			score += 18
		case item.SpreadPercent <= 25:
			score += 10
		}
		if item.SpreadPercent <= 25 {
			reasons = append(reasons, "narrow_spread")
		}
	}

	if item.HasOrderBook && item.BuyOrderCount > 0 {
		switch {
		case item.BuyOrderCount >= 100:
			score += 20
		case item.BuyOrderCount >= 30:
			score += 14
		case item.BuyOrderCount >= 10:
			score += 8
		}
		if item.BuyOrderCount >= 10 {
			reasons = append(reasons, "high_buy_orders")
		}
	}

	if item.HasConfidence {
		switch item.Confidence {
		case "verified":
			score += 20
			reasons = append(reasons, "high_confidence")
		case "estimated":
			score += 10
		}
	}

	if item.HasWeeklyAverage && item.WeeklyAveragePrice > 0 && item.Suggested > item.WeeklyAveragePrice {
		score += 15
		reasons = append(reasons, "above_weekly_average")
	}

	if len(reasons) == 0 {
		return 0, nil
	}
	return score, reasons
}

func mapHeroKeyToClassID(heroKey int) int {
	if heroKey >= 101 && heroKey <= 601 && heroKey%100 == 1 {
		return heroKey / 100
	}
	if heroKey >= 1 && heroKey <= 6 {
		return heroKey
	}
	return 0
}
