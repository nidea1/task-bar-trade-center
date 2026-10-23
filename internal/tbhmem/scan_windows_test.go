//go:build windows

package tbhmem

import (
	"bytes"
	"math/rand"
	"reflect"
	"testing"
)

func scanFixture(data []byte, patterns [][]byte, limit, chunk int, failAt uintptr) ([][]uintptr, uint64) {
	results := make([][]uintptr, len(patterns))
	read := func(address uintptr, dst []byte) bool {
		if failAt > 0 && address <= failAt && failAt < address+uintptr(len(dst)) {
			return false
		}
		copy(dst, data[int(address-0x10000):int(address-0x10000)+len(dst)])
		return true
	}
	scanned := scanMemoryRegion(read, 0x10000, uintptr(len(data)), patterns, limit, results, make([]byte, chunk+longestPattern(patterns)), chunk)
	return results, scanned
}

func TestScanMemoryRegionMatchesAcrossChunksWithoutDuplicates(t *testing.T) {
	data := []byte("aaaaabbabaababaaaaa")
	patterns := [][]byte{[]byte("a"), []byte("aaa"), []byte("abbab"), nil, []byte("absent")}
	for chunk := 1; chunk <= len(data); chunk++ {
		got, _ := scanFixture(data, patterns, 100, chunk, 0)
		for i, pattern := range patterns {
			var want []uintptr
			if len(pattern) > 0 {
				for offset := 0; offset+len(pattern) <= len(data); offset++ {
					if bytes.Equal(data[offset:offset+len(pattern)], pattern) {
						want = append(want, 0x10000+uintptr(offset))
					}
				}
			}
			if !reflect.DeepEqual(got[i], want) {
				t.Fatalf("chunk=%d pattern=%q got=%v want=%v", chunk, pattern, got[i], want)
			}
		}
	}
}

func TestScanMemoryRegionRandomizedMatchesAndLimits(t *testing.T) {
	rng := rand.New(rand.NewSource(23))
	for trial := 0; trial < 100; trial++ {
		data := make([]byte, 512)
		for i := range data {
			data[i] = byte('a' + rng.Intn(3))
		}
		patterns := [][]byte{data[5:6], data[10:13], data[29:47], []byte("missing")}
		limit := 1 + rng.Intn(8)
		got, _ := scanFixture(data, patterns, limit, 1+rng.Intn(31), 0)
		for i, pattern := range patterns {
			var want []uintptr
			for offset := 0; offset+len(pattern) <= len(data) && len(want) < limit; offset++ {
				if bytes.Equal(data[offset:offset+len(pattern)], pattern) {
					want = append(want, 0x10000+uintptr(offset))
				}
			}
			if !reflect.DeepEqual(got[i], want) {
				t.Fatalf("trial=%d pattern=%q got=%v want=%v", trial, pattern, got[i], want)
			}
		}
	}
}

func TestScanMemoryRegionDoesNotJoinUnreadableGap(t *testing.T) {
	got, scanned := scanFixture([]byte("aaXaaa"), [][]byte{[]byte("aaa")}, 10, 2, 0x10002)
	if !reflect.DeepEqual(got[0], []uintptr(nil)) || scanned != 4 {
		t.Fatalf("matches=%v bytes=%d; matched across an unreadable chunk", got, scanned)
	}
}

func TestScannerRejectsEmptyPatternsAndInvalidLimits(t *testing.T) {
	for _, limit := range []int{-1, 0, 1} {
		results, scanned := (*Process)(nil).ScanPatterns([][]byte{nil}, limit)
		if len(results) != 1 || len(results[0]) != 0 || scanned != 0 {
			t.Fatalf("limit=%d results=%v bytes=%d", limit, results, scanned)
		}
	}
	got, _ := scanFixture([]byte("aaaaaaaa"), [][]byte{nil, []byte("a")}, 2, 2, 0)
	if !reflect.DeepEqual(got[1], []uintptr{0x10000, 0x10001}) {
		t.Fatalf("limits=%v", got)
	}
}

func BenchmarkScanMemoryRegion(b *testing.B) {
	data := bytes.Repeat([]byte{'x'}, 16<<20)
	patterns := [][]byte{[]byte("PlayerSaveData\x00"), []byte("StageBox\x00"), []byte("TradingStashSlot\x00")}
	buffer := make([]byte, maxScanChunkBytes+longestPattern(patterns))
	read := func(address uintptr, dst []byte) bool {
		copy(dst, data[int(address):int(address)+len(dst)])
		return true
	}
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results := make([][]uintptr, len(patterns))
		scanMemoryRegion(read, 0, uintptr(len(data)), patterns, 64, results, buffer, maxScanChunkBytes)
	}
}
