package game

import (
	"encoding/json"
	"testing"
)

func TestParseGameLayoutLoadsAndSortsScaleCalibrationAnchors(t *testing.T) {
	layout, err := ParseGameLayout(EmbeddedLayoutJSON())
	if err != nil {
		t.Fatalf("ParseGameLayout returned error: %v", err)
	}
	if len(layout.ScaleCalibrations) == 0 {
		t.Fatal("scale_calibrations was not parsed")
	}
	profile := layout.ScaleCalibrations[0]
	if profile.ScalePercent != 100 || profile.YOffset != 116 {
		t.Fatalf("profile = %+v, want scale=100 yOffset=116", profile)
	}
	for index := 1; index < len(profile.XAnchors); index++ {
		if profile.XAnchors[index-1].X >= profile.XAnchors[index].X {
			t.Fatalf("anchors are not strictly sorted at index %d", index)
		}
	}
}

func TestParseGameLayoutRejectsDuplicateScaleProfiles(t *testing.T) {
	var document map[string]any
	if err := json.Unmarshal(EmbeddedLayoutJSON(), &document); err != nil {
		t.Fatalf("unmarshal embedded layout: %v", err)
	}
	profiles, ok := document["scale_calibrations"].([]any)
	if !ok || len(profiles) == 0 {
		t.Fatal("embedded scale_calibrations section not found")
	}
	document["scale_calibrations"] = append(profiles, profiles[0])
	raw, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("marshal duplicated layout: %v", err)
	}

	if _, err := ParseGameLayout(raw); err == nil {
		t.Fatal("ParseGameLayout accepted duplicate scale_percent profiles")
	}
}
