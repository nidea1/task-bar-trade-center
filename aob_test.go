package main

import "testing"

func TestParseOptionalAOBPattern(t *testing.T) {
	pattern, err := parseOptionalAOBPattern("test", "48 8B 05 ?? ?? ?? ?? C3", 3, 7)
	if err != nil {
		t.Fatalf("parseOptionalAOBPattern returned error: %v", err)
	}
	if len(pattern.Bytes) != 8 || !pattern.Wildcards[3] || !pattern.Wildcards[6] {
		t.Fatalf("parsed pattern = %+v", pattern)
	}
	if _, err := parseOptionalAOBPattern("test", "48 8B", 0, 4); err == nil {
		t.Fatal("invalid displacement range was accepted")
	}
}

func TestAOBMatchOffsetsAndPointerResolution(t *testing.T) {
	pattern, err := parseOptionalAOBPattern("test", "48 8B 05 ?? ?? ?? ?? C3", 3, 7)
	if err != nil {
		t.Fatalf("parse pattern: %v", err)
	}
	data := []byte{
		0x90,
		0x48, 0x8B, 0x05, 0xF8, 0x0F, 0x00, 0x00, 0xC3,
		0x90,
		0x48, 0x8B, 0x05, 0x00, 0x00, 0x00, 0x00, 0x90,
	}
	matches := aobMatchOffsets(data, pattern)
	if len(matches) != 1 || matches[0] != 1 {
		t.Fatalf("matches = %v, want [1]", matches)
	}
	base, ok := resolveAOBPointerBase(0x1000+uintptr(matches[0]), data[matches[0]:], pattern)
	if !ok || base != 0x2000 {
		t.Fatalf("resolved base = 0x%X, ok=%t; want 0x2000, true", base, ok)
	}
}

func TestValidTooltipAOBValue(t *testing.T) {
	tests := []struct {
		label string
		value float32
		valid bool
	}{
		{label: "x", value: -245, valid: true},
		{label: "x", value: 0, valid: false},
		{label: "y", value: -174, valid: true},
		{label: "height", value: 348, valid: true},
		{label: "height", value: 0, valid: false},
	}
	for _, tt := range tests {
		if got := validTooltipAOBValue(tt.label, tt.value); got != tt.valid {
			t.Errorf("validTooltipAOBValue(%q, %v) = %t, want %t", tt.label, tt.value, got, tt.valid)
		}
	}
}
