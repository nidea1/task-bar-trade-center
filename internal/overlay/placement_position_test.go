package overlay

import "testing"

func TestResolveScalePositionOffsetInterpolatesWithinSelectedScale(t *testing.T) {
	profiles := []ScaleCalibrationProfile{
		{
			ScalePercent: 100,
			YOffset:      116,
			XAnchors: []XCalibrationAnchor{
				{X: -270, Offset: -176},
				{X: -230, Offset: -96},
			},
		},
		{
			ScalePercent: 125,
			YOffset:      20,
			XAnchors: []XCalibrationAnchor{
				{X: -270, Offset: -217},
				{X: -230, Offset: -137},
			},
		},
	}

	got, ok := ResolveScalePositionOffset(-250, 125, profiles)
	if !ok {
		t.Fatal("ResolveScalePositionOffset returned no match")
	}
	if got.ScalePercent != 125 || got.XOffset != -177 || got.YOffset != 20 {
		t.Fatalf("offset = %+v, want scale=125 xOffset=-177 yOffset=20", got)
	}
}

func TestInterpolateXOffsetReturnsExactAnchor(t *testing.T) {
	anchors := []XCalibrationAnchor{
		{X: -270, Offset: -176},
		{X: -230, Offset: -96},
	}

	got, ok := InterpolateXOffset(-230, anchors)
	if !ok || got != -96 {
		t.Fatalf("offset = %d, ok=%t; want -96, true", got, ok)
	}
}

func TestInterpolateXOffsetClampsOutsideRange(t *testing.T) {
	anchors := []XCalibrationAnchor{
		{X: -270, Offset: -176},
		{X: -230, Offset: -96},
	}

	left, leftOK := InterpolateXOffset(-300, anchors)
	right, rightOK := InterpolateXOffset(-200, anchors)
	if !leftOK || left != -176 {
		t.Fatalf("left offset = %d, ok=%t; want -176, true", left, leftOK)
	}
	if !rightOK || right != -96 {
		t.Fatalf("right offset = %d, ok=%t; want -96, true", right, rightOK)
	}
}

func TestResolveScalePositionOffsetDoesNotUseAnotherScale(t *testing.T) {
	profiles := []ScaleCalibrationProfile{
		{
			ScalePercent: 100,
			YOffset:      116,
			XAnchors: []XCalibrationAnchor{
				{X: -270, Offset: -176},
				{X: -230, Offset: -96},
			},
		},
	}

	if _, ok := ResolveScalePositionOffset(-250, 150, profiles); ok {
		t.Fatal("matched a profile belonging to another scale")
	}
}

func TestFindClosestPositionOffsetForScaleLegacyFallback(t *testing.T) {
	calibrations := []PositionCalibration{
		{Scale: 1.0, X: -270, Y: -173, XOffset: -176, YOffset: 116},
		{Scale: 1.25, X: -270, Y: -173, XOffset: -217, YOffset: 20},
	}

	got, ok := FindClosestPositionOffsetForScale(-270, -173, 125, calibrations)
	if !ok {
		t.Fatal("legacy fallback returned no match")
	}
	if got.ScalePercent != 125 || got.XOffset != -217 || got.YOffset != 20 {
		t.Fatalf("legacy offset = %+v", got)
	}
}
