package game

import (
	"encoding/binary"
	"math"
	"runtime"
	"testing"
	"unsafe"
)

type rectTransformFixture struct {
	native     [0xC0]byte
	hierarchy  [0x28]byte
	transforms [3][48]byte
	parents    [3]int32
}

func newRectTransformFixture() *rectTransformFixture {
	f := &rectTransformFixture{parents: [3]int32{-1, 0, 1}}
	binary.LittleEndian.PutUint64(f.native[0x28:], uint64(uintptr(unsafe.Pointer(&f.hierarchy[0]))))
	binary.LittleEndian.PutUint32(f.native[0x30:], 2)
	binary.LittleEndian.PutUint32(f.hierarchy[0x10:], 3)
	binary.LittleEndian.PutUint64(f.hierarchy[0x18:], uint64(uintptr(unsafe.Pointer(&f.transforms[0][0]))))
	binary.LittleEndian.PutUint64(f.hierarchy[0x20:], uint64(uintptr(unsafe.Pointer(&f.parents[0]))))
	for i := range f.transforms {
		for _, offset := range []int{28, 32, 36, 40} {
			putUnityFloat(f.transforms[i][:], offset, 1)
		}
	}
	// Canvas -> ItemTooltip -> visible Window, as observed in the live game.
	putUnityFloat(f.transforms[0][:], 0, 485)
	putUnityFloat(f.transforms[0][:], 4, 446)
	putUnityFloat(f.transforms[1][:], 0, -310)
	putUnityFloat(f.transforms[1][:], 4, 240)
	putUnityFloat(f.transforms[2][:], 4, -240)
	putUnityFloat(f.native[:], 0xA8, -114)
	putUnityFloat(f.native[:], 0xAC, -180)
	putUnityFloat(f.native[:], 0xB0, 228)
	putUnityFloat(f.native[:], 0xB4, 360)
	return f
}

func putUnityFloat(data []byte, offset int, value float32) {
	binary.LittleEndian.PutUint32(data[offset:], math.Float32bits(value))
}

func (f *rectTransformFixture) bounds() (UnityRectBounds, bool) {
	bounds, ok := ReadRectTransformBounds(^uintptr(0), uintptr(unsafe.Pointer(&f.native[0])))
	runtime.KeepAlive(f)
	return bounds, ok
}

func TestRectTransformBoundsFollowCanvasAndTooltipSize(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5} {
		for _, height := range []float32{200, 360, 550} {
			f := newRectTransformFixture()
			putUnityFloat(f.transforms[0][:], 32, scale)
			putUnityFloat(f.transforms[0][:], 36, scale)
			putUnityFloat(f.native[:], 0xAC, -height/2)
			putUnityFloat(f.native[:], 0xB4, height)
			bounds, ok := f.bounds()
			s := float64(scale)
			want := UnityRectBounds{485 - 424*s, 446 - float64(height)*s/2, 485 - 196*s, 446 + float64(height)*s/2}
			if !ok || bounds != want {
				t.Errorf("scale=%g height=%g: bounds=%+v ok=%t, want %+v", scale, height, bounds, ok, want)
			}
		}
	}
}

func TestRectTransformBoundsFollowParentMovement(t *testing.T) {
	f := newRectTransformFixture()
	before, _ := f.bounds()
	putUnityFloat(f.transforms[1][:], 0, -230)
	putUnityFloat(f.transforms[1][:], 4, 300)
	after, ok := f.bounds()
	if !ok || after.MinX-before.MinX != 80 || after.MaxX-before.MaxX != 80 ||
		after.MinY-before.MinY != 60 || after.MaxY-before.MaxY != 60 {
		t.Fatalf("parent movement: before=%+v after=%+v ok=%t", before, after, ok)
	}
}

func TestRectTransformBoundsIncludeRotation(t *testing.T) {
	f := newRectTransformFixture()
	putUnityFloat(f.transforms[2][:], 24, float32(math.Sqrt(0.5)))
	putUnityFloat(f.transforms[2][:], 28, float32(math.Sqrt(0.5)))
	bounds, ok := f.bounds()
	if !ok || math.Abs((bounds.MaxX-bounds.MinX)-360) > 0.001 || math.Abs((bounds.MaxY-bounds.MinY)-228) > 0.001 {
		t.Fatalf("rotated bounds=%+v ok=%t", bounds, ok)
	}
}

func TestRectTransformBoundsRejectInvalidMemory(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*rectTransformFixture)
	}{
		{"missing hierarchy", func(f *rectTransformFixture) { binary.LittleEndian.PutUint64(f.native[0x28:], 0) }},
		{"bad index", func(f *rectTransformFixture) { binary.LittleEndian.PutUint32(f.native[0x30:], 3) }},
		{"bad count", func(f *rectTransformFixture) { binary.LittleEndian.PutUint32(f.hierarchy[0x10:], 0x7FFFFFFF) }},
		{"unreadable matrices", func(f *rectTransformFixture) { binary.LittleEndian.PutUint64(f.hierarchy[0x18:], 1) }},
		{"cycle", func(f *rectTransformFixture) { f.parents[0] = 2 }},
		{"bad parent", func(f *rectTransformFixture) { f.parents[2] = -2 }},
		{"hidden scale", func(f *rectTransformFixture) { putUnityFloat(f.transforms[0][:], 32, 0) }},
		{"invalid quaternion", func(f *rectTransformFixture) { putUnityFloat(f.transforms[0][:], 28, 0) }},
		{"NaN rect", func(f *rectTransformFixture) { putUnityFloat(f.native[:], 0xA8, float32(math.NaN())) }},
		{"NaN position", func(f *rectTransformFixture) { putUnityFloat(f.transforms[0][:], 0, float32(math.NaN())) }},
		{"empty rect", func(f *rectTransformFixture) { putUnityFloat(f.native[:], 0xB4, 0) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newRectTransformFixture()
			test.change(f)
			if bounds, ok := f.bounds(); ok {
				t.Fatalf("invalid memory produced bounds %+v", bounds)
			}
		})
	}
}
