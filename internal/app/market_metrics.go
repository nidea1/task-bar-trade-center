package app

import (
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/catalog"
	"github.com/nidea1/task-bar-trade-center/internal/market"
)

func init() {
	market.SetSteamRequestMetricLogger(logSteamRequestMetric)
}

func logSteamRequestMetric(metric market.SteamRequestMetric) {
	status := metric.StatusCode
	errorText := metric.Error
	if errorText == "" {
		errorText = "-"
	}
	if metric.Endpoint == "pricehistory" && errorText != "-" && metric.URL != "" {
		logPrintf(
			"[MARKET:metric] steam_request endpoint=%s priority=%s limiter_wait_ms=%d request_ms=%d status=%d retry_after=%q error=%q url=%q appid=%q market_hash_name=%q response_body=%q\n",
			metric.Endpoint,
			metric.Priority.String(),
			durationMillis(metric.LimiterWait),
			durationMillis(metric.RequestDuration),
			status,
			metric.RetryAfter,
			errorText,
			metric.URL,
			metric.AppID,
			metric.MarketHashName,
			metric.ResponseBody,
		)
		return
	}
	logPrintf(
		"[MARKET:metric] steam_request endpoint=%s priority=%s limiter_wait_ms=%d request_ms=%d status=%d retry_after=%q error=%q\n",
		metric.Endpoint,
		metric.Priority.String(),
		durationMillis(metric.LimiterWait),
		durationMillis(metric.RequestDuration),
		status,
		metric.RetryAfter,
		errorText,
	)
}

func logMarketFetchMetric(config catalog.ItemConfig, scope market.MarketScope, marketHashName string, priority market.RequestPriority, joinedInFlight bool, elapsed time.Duration, err error) {
	status := "ok"
	errorText := "-"
	if err != nil {
		status = "error"
		errorText = err.Error()
	}
	logPrintf(
		"[MARKET:metric] market_fetch item_id=%d market_hash_name=%q scope=%s priority=%s joined_inflight=%t total_ms=%d status=%s error=%q\n",
		config.ID,
		marketHashName,
		market.FormatScope(scope),
		priority.String(),
		joinedInFlight,
		durationMillis(elapsed),
		status,
		errorText,
	)
}

func logTooltipCacheMetric(config catalog.ItemConfig, scope market.MarketScope, marketHashName string, state string, refreshQueued bool, age time.Duration) {
	logPrintf(
		"[MARKET:metric] tooltip_cache item_id=%d market_hash_name=%q scope=%s cache_state=%s cache_age_ms=%d refresh_queued=%t\n",
		config.ID,
		marketHashName,
		market.FormatScope(scope),
		state,
		durationMillis(age),
		refreshQueued,
	)
}

func durationMillis(duration time.Duration) int64 {
	if duration < 0 {
		return -1
	}
	return int64(duration / time.Millisecond)
}
