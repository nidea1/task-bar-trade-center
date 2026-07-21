package overlay

import "testing"

func TestSteamOverviewRowsOnlyIncreaseDetailedHeight(t *testing.T) {
	base := PriceView{Suggested: "$1.00", WeeklyAvg: "$0.90", DailySales: "10 units"}
	withSteam := base
	withSteam.SteamMedian = "$0.95"
	withSteam.SteamVolume = "20 units"

	const compactMode int32 = 1
	if got, want := CalculateRequiredHeight(withSteam, compactMode, compactMode), CalculateRequiredHeight(base, compactMode, compactMode); got != want {
		t.Fatalf("compact height = %d, want unchanged %d", got, want)
	}
	if got, want := CalculateRequiredHeight(withSteam, 2, compactMode), CalculateRequiredHeight(base, 2, compactMode)+34; got != want {
		t.Fatalf("detail height = %d, want %d", got, want)
	}
}

func TestParsePriceViewReadsSteamOverviewRows(t *testing.T) {
	view := ParsePriceView("TBTC Suggested: $1.00\nSteam Median: $0.95\nSteam Volume: 20 units", "TBTC Suggested")
	if view.SteamMedian != "$0.95" || view.SteamVolume != "20 units" {
		t.Fatalf("parsed Steam rows = %+v", view)
	}
}
