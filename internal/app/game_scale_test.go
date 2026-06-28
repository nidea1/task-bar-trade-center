package app

import "testing"

func TestNormalizeGameScale(t *testing.T) {
	tests := []struct {
		input int32
		want  int32
	}{
		{input: GameScale100, want: GameScale100},
		{input: GameScale125, want: GameScale125},
		{input: GameScale150, want: GameScale150},
		{input: 0, want: GameScale100},
		{input: 110, want: GameScale100},
	}

	for _, tt := range tests {
		if got := normalizeGameScale(tt.input); got != tt.want {
			t.Fatalf(
				"normalizeGameScale(%d) = %d, want %d",
				tt.input,
				got,
				tt.want,
			)
		}
	}
}

func TestGameScaleForMenuCommand(t *testing.T) {
	scale, ok := gameScaleForMenuCommand(MenuGameScaleBase + 1)
	if !ok {
		t.Fatal("gameScaleForMenuCommand returned no scale")
	}
	if scale != GameScale125 {
		t.Fatalf("scale = %d, want %d", scale, GameScale125)
	}
}
