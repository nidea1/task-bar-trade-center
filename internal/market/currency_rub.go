package market

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sync"
	"time"
)

const rubExchangeRateMaxAge = 7 * 24 * time.Hour

var (
	exchangeRateHTTPClient       = &http.Client{Timeout: 10 * time.Second}
	rubExchangeRateRefreshMu     sync.Mutex
	rubExchangeRateLastAttempt   time.Time
	rubExchangeRateRefreshFailed bool
)

func validExchangeRate(code string, rate float64) bool {
	if rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
		return false
	}
	return code != "RUB" || (rate > 1 && rate <= 1000)
}

func validRUBRateDate(date time.Time, now time.Time) bool {
	return !date.IsZero() && !date.After(now) && now.Sub(date) <= rubExchangeRateMaxAge
}

func refreshRUBExchangeRate() {
	rubExchangeRateRefreshMu.Lock()
	defer rubExchangeRateRefreshMu.Unlock()
	now := time.Now()
	interval := 5 * time.Minute
	if _, ok := configuredExchangeRate("RUB"); ok && !rubExchangeRateRefreshFailed {
		interval = 6 * time.Hour
	}
	if now.Sub(rubExchangeRateLastAttempt) < interval {
		return
	}
	rubExchangeRateLastAttempt = now
	if err := fetchRUBExchangeRate(exchangeRateHTTPClient, now); err != nil {
		rubExchangeRateRefreshFailed = true
		fmt.Printf("[CURRENCY] Failed to fetch USD/RUB from Frankfurter v2: %v\n", err)
	} else {
		rubExchangeRateRefreshFailed = false
	}
}

func fetchRUBExchangeRate(client *http.Client, now time.Time) error {
	response, err := client.Get("https://api.frankfurter.dev/v2/rates?base=USD&quotes=RUB")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP status %d", response.StatusCode)
	}
	var rows []struct {
		Date  string  `json:"date"`
		Base  string  `json:"base"`
		Quote string  `json:"quote"`
		Rate  float64 `json:"rate"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 64*1024)).Decode(&rows); err != nil {
		return err
	}
	if len(rows) != 1 || rows[0].Base != "USD" || rows[0].Quote != "RUB" || !validExchangeRate("RUB", rows[0].Rate) {
		return fmt.Errorf("invalid USD/RUB rate response")
	}
	date, err := time.Parse("2006-01-02", rows[0].Date)
	if err != nil || !validRUBRateDate(date, now) {
		return fmt.Errorf("invalid or stale USD/RUB observation date %q", rows[0].Date)
	}
	exchangeRateMu.Lock()
	if !date.Before(rubExchangeRateDate) {
		exchangeRateCache["RUB"] = rows[0].Rate
		rubExchangeRateDate = date
	}
	exchangeRateMu.Unlock()
	return nil
}

func validRUBConversion(analysis MarketAnalysis, now time.Time) bool {
	return analysis.PriceSuffix != " ₽" || !analysis.PriceIsEstimated || validRUBRateDate(analysis.ExchangeRateDate, now)
}
