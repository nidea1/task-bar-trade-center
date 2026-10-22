package market

import (
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestSteamCooldownAppliesToAllEndpointsAndPriorities(t *testing.T) {
	originalLimiter := steamRequestLimiter
	t.Cleanup(func() { steamRequestLimiter = originalLimiter })
	steamRequestLimiter = newSteamRateLimiter()
	requests := 0
	client := &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			response := testHTTPResponse(http.StatusTooManyRequests, "")
			response.Header.Set("Retry-After", "60")
			return response, nil
		}
		return testHTTPResponse(http.StatusOK, "updated"), nil
	})}
	_, _, err := steamGetWithPriority(client, "https://steamcommunity.com/market/listings/1/test", "", RequestPriorityNormal)
	var rateErr *SteamError
	if !errors.As(err, &rateErr) || rateErr.RetryAt().Before(time.Now().Add(59*time.Second)) {
		t.Fatalf("expected timed rate limit, got %v", err)
	}
	for _, priority := range []RequestPriority{RequestPriorityNormal, RequestPriorityHigh} {
		_, status, err := steamGetWithPriority(client, "https://steamcommunity.com/market/search", "", priority)
		if status != http.StatusTooManyRequests || !errors.As(err, &rateErr) || rateErr.Endpoint != "search-page" {
			t.Fatalf("cooldown status=%d err=%v", status, err)
		}
	}
	if requests != 1 {
		t.Fatalf("sent %d HTTP requests during cooldown; want 1", requests)
	}
	steamRequestLimiter.mu.Lock()
	steamRequestLimiter.backoffUntil = time.Now().Add(-time.Second)
	steamRequestLimiter.lastStartedAt = time.Time{}
	steamRequestLimiter.mu.Unlock()
	body, _, err := steamGetWithPriority(client, "https://steamcommunity.com/market/search", "", RequestPriorityNormal)
	if err != nil || string(body) != "updated" || requests != 2 {
		t.Fatalf("request did not recover: body=%q requests=%d err=%v", body, requests, err)
	}
	if steamRequestLimiter.waitingHigh != 0 {
		t.Fatal("cooldown leaked a high priority waiter")
	}
}

func TestSteamCooldownRetryAfter(t *testing.T) {
	now := time.Unix(1792691084, 0).UTC()
	for _, tt := range []struct {
		name, header string
		delay        time.Duration
	}{
		{"seconds", "120", 2 * time.Minute},
		{"date", now.Add(5 * time.Minute).Format(http.TimeFormat), 5 * time.Minute},
		{"missing", "", 15 * time.Minute},
		{"invalid", "bad", 15 * time.Minute},
		{"overflow", "9223372036854775807", 15 * time.Minute},
		{"expired", now.Add(-time.Minute).Format(http.TimeFormat), 15 * time.Minute},
	} {
		t.Run(tt.name, func(t *testing.T) {
			limiter := newSteamRateLimiter()
			if got := limiter.enterBackoff(tt.header, now); !got.Equal(now.Add(tt.delay)) {
				t.Fatalf("deadline=%s want=%s", got, now.Add(tt.delay))
			}
		})
	}
	limiter := newSteamRateLimiter()
	for _, delay := range []time.Duration{15 * time.Minute, 30 * time.Minute, time.Hour, time.Hour} {
		if got := limiter.enterBackoff("", now); !got.Equal(now.Add(delay)) {
			t.Fatalf("backoff=%s want=%s", got.Sub(now), delay)
		}
		now = now.Add(delay)
	}
}
