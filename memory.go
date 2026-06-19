package main

import (
	"fmt"
	"strings"
	"time"
	"unsafe"
)

func readHoveredItemID(processHandle uintptr, baseAddress uintptr, offsets []uintptr) (int32, string, int32, bool) {
	const itemKeyOffset uintptr = 0x1A4

	itemObjectPointerAddress, ok := resolvePointerChainAddress(processHandle, baseAddress, offsets)
	if !ok {
		return 0, "", 0, false
	}

	itemObject, ok := readUintptr(processHandle, itemObjectPointerAddress)
	if !ok {
		return 0, "", 0, false
	}
	if itemObject == 0 {
		return 0, "object-pointer", 0, true
	}

	itemKey, ok := readInt32(processHandle, itemObject+itemKeyOffset)
	if !ok {
		return 0, "", 0, false
	}
	if _, exists := AllItemMap[int(itemKey)]; exists {
		return itemKey, "object+0x1A4", 0, true
	}

	return 0, "object+0x1A4", itemKey, true
}

func resolvePointerChainAddress(processHandle uintptr, baseAddress uintptr, offsets []uintptr) (uintptr, bool) {
	currentAddress := baseAddress

	for _, offset := range offsets {
		nextAddress, ok := readUintptr(processHandle, currentAddress)
		if !ok || nextAddress == 0 {
			return 0, false
		}
		currentAddress = nextAddress + offset
	}

	return currentAddress, true
}

func readUintptr(processHandle uintptr, address uintptr) (uintptr, bool) {
	var value uintptr
	var bytesRead uintptr
	ret, _, _ := procReadProcessMemory.Call(processHandle, address, uintptr(unsafe.Pointer(&value)), unsafe.Sizeof(value), uintptr(unsafe.Pointer(&bytesRead)))
	return value, ret != 0 && bytesRead == unsafe.Sizeof(value)
}

func readInt32(processHandle uintptr, address uintptr) (int32, bool) {
	var value int32
	var bytesRead uintptr
	ret, _, _ := procReadProcessMemory.Call(processHandle, address, uintptr(unsafe.Pointer(&value)), unsafe.Sizeof(value), uintptr(unsafe.Pointer(&bytesRead)))
	return value, ret != 0 && bytesRead == unsafe.Sizeof(value)
}

func readFloat32(processHandle uintptr, address uintptr) (float32, bool) {
	var value float32
	var bytesRead uintptr
	ret, _, _ := procReadProcessMemory.Call(processHandle, address, uintptr(unsafe.Pointer(&value)), unsafe.Sizeof(value), uintptr(unsafe.Pointer(&bytesRead)))
	return value, ret != 0 && bytesRead == unsafe.Sizeof(value)
}

func resolveTooltipPointerChain(label string, baseAddress uintptr, offsets []uintptr) (uintptr, bool, string) {
	address, ok, trace := resolveTooltipPointerChainInOrder(label+" listed", baseAddress, offsets)
	if ok {
		return address, true, trace
	}

	reversedOffsets := reversePointerOffsets(offsets)
	reversedAddress, reversedOK, reversedTrace := resolveTooltipPointerChainInOrder(label+" reversed", baseAddress, reversedOffsets)
	if reversedOK {
		return reversedAddress, true, reversedTrace
	}

	return 0, false, trace + " || " + reversedTrace
}

func resolveTooltipPointerChainInOrder(label string, baseAddress uintptr, offsets []uintptr) (uintptr, bool, string) {
	currentAddress := baseAddress
	steps := []string{fmt.Sprintf("%s base=0x%X offsets=%s", label, baseAddress, formatPointerOffsets(offsets))}
	for index, offset := range offsets {
		nextAddress, ok := readUintptr(GameProcessHandle, currentAddress)
		if !ok {
			steps = append(steps, fmt.Sprintf("step%d read[0x%X] FAILED off=0x%X", index+1, currentAddress, offset))
			return 0, false, strings.Join(steps, " | ")
		}
		if nextAddress == 0 {
			steps = append(steps, fmt.Sprintf("step%d read[0x%X]=0x0 NULL off=0x%X", index+1, currentAddress, offset))
			return 0, false, strings.Join(steps, " | ")
		}
		steps = append(steps, fmt.Sprintf("step%d read[0x%X]=0x%X +0x%X => 0x%X", index+1, currentAddress, nextAddress, offset, nextAddress+offset))
		currentAddress = nextAddress + offset
	}
	steps = append(steps, fmt.Sprintf("final=0x%X", currentAddress))
	return currentAddress, true, strings.Join(steps, " | ")
}

func reversePointerOffsets(offsets []uintptr) []uintptr {
	reversed := make([]uintptr, len(offsets))
	for i, offset := range offsets {
		reversed[len(offsets)-1-i] = offset
	}
	return reversed
}

func readTooltipRectFromMemory() (RECT, bool) {
	if GameProcessHandle == 0 || GameAssemblyBase == 0 {
		logTooltipDebug("handle/base missing: handle=0x%X gameAssembly=0x%X", GameProcessHandle, GameAssemblyBase)
		return RECT{}, false
	}

	xBase := GameAssemblyBase + TooltipXPointerBaseOffset
	widthBase := GameAssemblyBase + TooltipWidthBaseOffset
	heightBase := GameAssemblyBase + TooltipHeightBaseOffset

	xAddress, xChainOK, xTrace := resolveTooltipPointerChain("x/y", xBase, TooltipXPointerOffsets)
	widthAddress, widthChainOK, widthTrace := resolveTooltipPointerChain("width", widthBase, TooltipWidthOffsets)
	heightAddress, heightChainOK, heightTrace := resolveTooltipPointerChain("height", heightBase, TooltipHeightOffsets)
	if !xChainOK || !widthChainOK || !heightChainOK {
		logTooltipDebugLines(
			"pointer chain status:",
			xTrace,
			widthTrace,
			heightTrace,
		)
		return RECT{}, false
	}

	x, ok := readFloat32(GameProcessHandle, xAddress)
	if !ok {
		logTooltipDebug("x read failed: xAddr=0x%X yAddr=0x%X", xAddress, xAddress+4)
		return RECT{}, false
	}
	y, ok := readFloat32(GameProcessHandle, xAddress+4)
	if !ok {
		logTooltipDebug("y read failed: xAddr=0x%X yAddr=0x%X", xAddress, xAddress+4)
		return RECT{}, false
	}
	width, ok := readFloat32(GameProcessHandle, widthAddress)
	if !ok {
		logTooltipDebug("width read failed: widthAddr=0x%X", widthAddress)
		return RECT{}, false
	}
	height, ok := readFloat32(GameProcessHandle, heightAddress)
	if !ok {
		logTooltipDebug("height read failed: heightAddr=0x%X", heightAddress)
		return RECT{}, false
	}
	rawX := x
	rawY := y
	x = -x
	y = -y
	logTooltipDebug("base=0x%X xBase=0x%X widthBase=0x%X heightBase=0x%X | xAddr=0x%X yAddr=0x%X widthAddr=0x%X heightAddr=0x%X | raw x=%.2f y=%.2f inverted x=%.2f y=%.2f w=%.2f h=%.2f", GameAssemblyBase, xBase, widthBase, heightBase, xAddress, xAddress+4, widthAddress, heightAddress, rawX, rawY, x, y, width, height)
	if width < 150 || width > 650 || height < 60 || height > 700 {
		logTooltipDebug("values rejected by size range: x=%.2f y=%.2f w=%.2f h=%.2f", x, y, width, height)
		return RECT{}, false
	}

	clientOrigin, ok := gameClientScreenOrigin()
	if !ok {
		logTooltipDebug("game client origin not found: pid=%d hwnd=0x%X", GameProcessID, GameWindowHWND)
		return RECT{}, false
	}

	localLeft := int32(x + 0.5)
	localTop := int32(y + 0.5)
	left := clientOrigin.X + localLeft
	top := clientOrigin.Y + localTop
	right := clientOrigin.X + int32(x+width+0.5)
	bottom := clientOrigin.Y + int32(y+height+0.5)
	if right <= left || bottom <= top {
		logTooltipDebug("rect rejected: rect=(%d,%d,%d,%d) values x=%.2f y=%.2f w=%.2f h=%.2f", left, top, right, bottom, x, y, width, height)
		return RECT{}, false
	}

	screen := virtualScreenRect()
	if right < screen.Left || left > screen.Right || bottom < screen.Top || top > screen.Bottom {
		logTooltipDebug("rect outside screen: rect=(%d,%d,%d,%d) screen=(%d,%d,%d,%d)", left, top, right, bottom, screen.Left, screen.Top, screen.Right, screen.Bottom)
		return RECT{}, false
	}
	logTooltipDebug("tooltip local=(%d,%d,%d,%d) clientOrigin=(%d,%d) screenRect=(%d,%d,%d,%d)", localLeft, localTop, int32(x+width+0.5), int32(y+height+0.5), clientOrigin.X, clientOrigin.Y, left, top, right, bottom)

	return RECT{
		Left:   left,
		Top:    top,
		Right:  right,
		Bottom: bottom,
	}, true
}

func logTooltipDebug(format string, args ...interface{}) {
	now := time.Now()
	if now.Sub(LastTooltipDebugLog) < time.Second {
		return
	}
	LastTooltipDebugLog = now
	fmt.Printf("[TOOLTIP] "+format+"\n", args...)
}

func logTooltipDebugLines(lines ...string) {
	now := time.Now()
	if now.Sub(LastTooltipDebugLog) < time.Second {
		return
	}
	LastTooltipDebugLog = now
	for _, line := range lines {
		fmt.Printf("[TOOLTIP] %s\n", line)
	}
}

func formatPointerOffsets(offsets []uintptr) string {
	parts := make([]string, 0, len(offsets))
	for _, offset := range offsets {
		parts = append(parts, fmt.Sprintf("0x%X", offset))
	}
	return strings.Join(parts, ",")
}
