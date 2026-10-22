package game

import (
	"encoding/binary"
	"math"
	"runtime"
	"testing"
	"unsafe"
)

func TestReadTooltipTaskBarHero130(t *testing.T) {
	layout, err := ParseGameLayout(EmbeddedLayoutJSON())
	if err != nil {
		t.Fatal(err)
	}
	// The root's fixed height is 480; the visible Window child has the
	// content-dependent height. Reading the root places the HUD incorrectly.
	var blocks [8][0x100]byte
	link := func(from int, offset int, to int) {
		binary.LittleEndian.PutUint64(blocks[from][offset:], uint64(uintptr(unsafe.Pointer(&blocks[to][0]))))
	}
	link(0, 0xB8, 1) // class -> static fields
	link(1, 0x08, 2) // static fields -> hover callback
	link(2, 0x20, 3) // callback -> ItemTooltip
	link(3, 0x28, 4) // ItemTooltip -> RectTransform
	link(4, 0x10, 5) // RectTransform -> native root
	link(5, 0x60, 6) // root -> child array
	link(6, 0x00, 7) // first child -> Window
	binary.LittleEndian.PutUint32(blocks[5][0xC8:], math.Float32bits(-310))
	binary.LittleEndian.PutUint32(blocks[5][0xB4:], math.Float32bits(480))
	binary.LittleEndian.PutUint32(blocks[7][0xAC:], math.Float32bits(-179.4))
	binary.LittleEndian.PutUint32(blocks[7][0xB4:], math.Float32bits(358.8))
	root := uintptr(unsafe.Pointer(&blocks[0][0]))
	for _, test := range []struct {
		label   string
		offsets []uintptr
		want    float32
	}{
		{"x", layout.TooltipXPointerOffsets, -310},
		{"y", layout.TooltipYPointerOffsets, -179.4},
		{"height", layout.TooltipHeightPointerOffsets, 358.8},
	} {
		address, ok := ResolvePointerChainAddress(^uintptr(0), uintptr(unsafe.Pointer(&root)), test.offsets)
		if !ok {
			t.Fatalf("%s pointer chain could not be resolved", test.label)
		}
		value, ok := ReadFloat32(^uintptr(0), address)
		if !ok || value != test.want || !ValidTooltipAOBValue(test.label, value) {
			t.Errorf("%s = %f, ok=%t; want %f", test.label, value, ok, test.want)
		}
	}
	runtime.KeepAlive(&blocks)
}

func TestReadHoveredItemTaskBarHero130(t *testing.T) {
	layout, err := ParseGameLayout(EmbeddedLayoutJSON())
	if err != nil {
		t.Fatal(err)
	}
	// Reproduce the 1.3.0 object layout: the old item field is now UI text.
	var tooltip [0x400 / 8]uintptr
	var oldText [0x40 / 8]uintptr
	var item [0x18 / 8]uintptr
	var info [0x38 / 8]uintptr
	const itemID = 315111
	info[0x30/8] = itemID
	item[0x10/8] = uintptr(unsafe.Pointer(&info[0]))
	tooltip[0x338/8] = uintptr(unsafe.Pointer(&oldText[0]))
	tooltip[0x3F0/8] = uintptr(unsafe.Pointer(&item[0]))

	nodes := make([][0x100 / 8]uintptr, len(layout.HoveredItemPointerOffsets)-1)
	root := uintptr(unsafe.Pointer(&nodes[0][0]))
	for index := range nodes {
		next := uintptr(unsafe.Pointer(&tooltip[0]))
		if index+1 < len(nodes) {
			next = uintptr(unsafe.Pointer(&nodes[index+1][0]))
		}
		nodes[index][layout.HoveredItemPointerOffsets[index]/8] = next
	}
	defer func() {
		runtime.KeepAlive(nodes)
		runtime.KeepAlive(&tooltip)
		runtime.KeepAlive(&oldText)
		runtime.KeepAlive(&item)
		runtime.KeepAlive(&info)
	}()

	for _, test := range []struct {
		name        string
		itemPointer uintptr
		infoPointer uintptr
		known       bool
		wantID      int32
		wantRaw     int32
	}{
		{"known item", uintptr(unsafe.Pointer(&item[0])), uintptr(unsafe.Pointer(&info[0])), true, itemID, 0},
		{"unknown item", uintptr(unsafe.Pointer(&item[0])), uintptr(unsafe.Pointer(&info[0])), false, 0, itemID},
		{"no hover", 0, 0, true, 0, 0},
		{"no item info", uintptr(unsafe.Pointer(&item[0])), 0, true, 0, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			tooltip[0x3F0/8] = test.itemPointer
			item[0x10/8] = test.infoPointer
			id, _, raw, ok := ReadHoveredItemID(^uintptr(0), uintptr(unsafe.Pointer(&root)), layout.HoveredItemPointerOffsets, layout.HoveredItemItemPtrOffset, layout.HoveredItemKeyOffset, func(value int32) bool {
				return test.known && value == itemID
			})
			if !ok || id != test.wantID || raw != test.wantRaw {
				t.Fatalf("ReadHoveredItemID = (%d, raw=%d, ok=%t), want (%d, raw=%d, ok=true)", id, raw, ok, test.wantID, test.wantRaw)
			}
		})
	}
}
