package il2cpp

import (
	"reflect"
	"testing"
)

type classMemory struct {
	ptrs     map[uintptr]uintptr
	names    map[uintptr][]byte
	patterns map[string][]uintptr
	scans    int
}

func (m *classMemory) ReadUintptr(address uintptr) (uintptr, bool) {
	value, ok := m.ptrs[address]
	return value, ok
}

func (m *classMemory) ReadBytes(address uintptr, dst []byte) bool {
	data := m.names[address]
	if len(data) < len(dst) {
		return false
	}
	copy(dst, data)
	return true
}

func (m *classMemory) ScanPattern(pattern []byte, _ int) ([]uintptr, uint64) {
	m.scans++
	return m.patterns[string(pattern)], 0
}

func (m *classMemory) ScanPatterns(patterns [][]byte, _ int) ([][]uintptr, uint64) {
	m.scans++
	result := make([][]uintptr, len(patterns))
	for i, pattern := range patterns {
		result[i] = m.patterns[string(pattern)]
	}
	return result, 0
}

func TestResolveClassesByNameUsesTwoPassesAndValidatesCandidates(t *testing.T) {
	m := &classMemory{ptrs: map[uintptr]uintptr{}, names: map[uintptr][]byte{}, patterns: map[string][]uintptr{}}
	names := []string{"PlayerSaveData", "StageBox", "TradingStashSlot"}
	for i, name := range names {
		address, class := uintptr(0x10000+i*0x100), uintptr(0x20000+i*0x100)
		m.names[address] = []byte(name + "\x00")
		m.patterns[name+"\x00"] = []uintptr{address}
		m.patterns[string(uintptrPattern(address))] = []uintptr{class + ClassNameOffset, class + ClassNameOffset, 0x90000}
		m.ptrs[class+ClassNameOffset] = address
		m.ptrs[class+ClassElementClassOffset] = class
		m.ptrs[class+ClassCastClassOffset] = class
	}
	got := ResolveClassesByName(m, append(names, "StageBox", ""))
	if m.scans != 2 {
		t.Fatalf("memory passes=%d, want 2", m.scans)
	}
	for i, name := range names {
		want := uintptr(0x20000 + i*0x100)
		if !reflect.DeepEqual(got[name], []uintptr{want}) {
			t.Fatalf("%s=%v", name, got[name])
		}
		if !ClassHasName(m, want, name) || ClassHasName(m, want, name[:len(name)-1]) {
			t.Fatalf("cached class name validation failed: %s", name)
		}
	}
	m.ptrs[0x20000+ClassCastClassOffset] = 0
	if ClassHasName(m, 0x20000, names[0]) {
		t.Fatal("stale class accepted")
	}
}

func TestResolveClassesByNameKeepsLegacyNameFallback(t *testing.T) {
	m := &classMemory{ptrs: map[uintptr]uintptr{0x20010: 0x10000, 0x20040: 0x20000, 0x20048: 0x20000}, patterns: map[string][]uintptr{"Example": {0x10000}, string(uintptrPattern(0x10000)): {0x20010}}}
	got, ok := ResolveClassByName(m, "Example")
	if !ok || !reflect.DeepEqual(got, []uintptr{0x20000}) {
		t.Fatalf("fallback=%v ok=%t", got, ok)
	}
	if got := ResolveClassesByName(m, nil); len(got) != 0 {
		t.Fatalf("empty names=%v", got)
	}
}
