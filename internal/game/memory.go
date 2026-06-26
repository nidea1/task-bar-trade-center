package game

import (
	"fmt"
	"strings"
	"unsafe"

	"github.com/nidea1/task-bar-trade-center/internal/win32"
)

func ReadHoveredItemID(processHandle uintptr, baseAddress uintptr, offsets []uintptr, itemPtrOffset uintptr, itemKeyOffset uintptr, itemExists func(int32) bool) (int32, string, int32, bool) {
	itemObjectPointerAddress, ok := ResolvePointerChainAddress(processHandle, baseAddress, offsets)
	if !ok {
		return 0, "", 0, false
	}

	itemObject, ok := ReadUintptr(processHandle, itemObjectPointerAddress)
	if !ok {
		return 0, "", 0, false
	}
	if itemObject == 0 {
		return 0, "object-pointer", 0, true
	}

	targetObject := itemObject
	if itemPtrOffset != 0 {
		subPtr, ok := ReadUintptr(processHandle, itemObject+itemPtrOffset)
		if !ok {
			return 0, "", 0, false
		}
		if subPtr == 0 {
			return 0, "sub-pointer", 0, true
		}
		targetObject = subPtr
	}

	itemKey, ok := ReadInt32(processHandle, targetObject+itemKeyOffset)
	if !ok {
		return 0, "", 0, false
	}
	if itemExists != nil && itemExists(itemKey) {
		return itemKey, fmt.Sprintf("object+0x%X", itemKeyOffset), 0, true
	}

	return 0, fmt.Sprintf("object+0x%X", itemKeyOffset), itemKey, true
}

func DescribePointerChainFailure(processHandle uintptr, baseAddress uintptr, offsets []uintptr, itemKeyOffset uintptr) string {
	currentAddress := baseAddress
	for index, offset := range offsets {
		nextAddress, ok := ReadUintptr(processHandle, currentAddress)
		if !ok {
			return fmt.Sprintf("hovered-item chain: base=0x%X step=%d read[0x%X] failed offset=0x%X offsets=%s", baseAddress, index+1, currentAddress, offset, FormatPointerOffsets(offsets))
		}
		if nextAddress == 0 {
			return fmt.Sprintf("hovered-item chain: base=0x%X step=%d read[0x%X]=NULL offset=0x%X offsets=%s", baseAddress, index+1, currentAddress, offset, FormatPointerOffsets(offsets))
		}
		currentAddress = nextAddress + offset
	}

	itemObject, ok := ReadUintptr(processHandle, currentAddress)
	if !ok {
		return fmt.Sprintf("hovered-item object read failed: address=0x%X keyOffset=0x%X", currentAddress, itemKeyOffset)
	}
	if itemObject == 0 {
		return fmt.Sprintf("hovered-item object pointer is NULL: address=0x%X keyOffset=0x%X", currentAddress, itemKeyOffset)
	}
	if _, ok := ReadInt32(processHandle, itemObject+itemKeyOffset); !ok {
		return fmt.Sprintf("hovered-item key read failed: object=0x%X keyAddress=0x%X", itemObject, itemObject+itemKeyOffset)
	}
	return fmt.Sprintf("hovered-item chain recovered during diagnostic: base=0x%X offsets=%s", baseAddress, FormatPointerOffsets(offsets))
}

func ResolvePointerChainAddress(processHandle uintptr, baseAddress uintptr, offsets []uintptr) (uintptr, bool) {
	currentAddress := baseAddress

	for _, offset := range offsets {
		nextAddress, ok := ReadUintptr(processHandle, currentAddress)
		if !ok || nextAddress == 0 {
			return 0, false
		}
		currentAddress = nextAddress + offset
	}

	return currentAddress, true
}

func ReadUintptr(processHandle uintptr, address uintptr) (uintptr, bool) {
	var value uintptr
	var bytesRead uintptr
	ret, _, _ := win32.ProcReadProcessMemory.Call(processHandle, address, uintptr(unsafe.Pointer(&value)), unsafe.Sizeof(value), uintptr(unsafe.Pointer(&bytesRead)))
	return value, ret != 0 && bytesRead == unsafe.Sizeof(value)
}

func ReadInt32(processHandle uintptr, address uintptr) (int32, bool) {
	var value int32
	var bytesRead uintptr
	ret, _, _ := win32.ProcReadProcessMemory.Call(processHandle, address, uintptr(unsafe.Pointer(&value)), unsafe.Sizeof(value), uintptr(unsafe.Pointer(&bytesRead)))
	return value, ret != 0 && bytesRead == unsafe.Sizeof(value)
}

func ReadFloat32(processHandle uintptr, address uintptr) (float32, bool) {
	var value float32
	var bytesRead uintptr
	ret, _, _ := win32.ProcReadProcessMemory.Call(processHandle, address, uintptr(unsafe.Pointer(&value)), unsafe.Sizeof(value), uintptr(unsafe.Pointer(&bytesRead)))
	return value, ret != 0 && bytesRead == unsafe.Sizeof(value)
}

func ResolveTooltipPointerChain(processHandle uintptr, label string, baseAddress uintptr, offsets []uintptr) (uintptr, bool, string) {
	address, ok, trace := resolveTooltipPointerChainInOrder(processHandle, label+" listed", baseAddress, offsets)
	if ok {
		return address, true, trace
	}

	reversedOffsets := ReversePointerOffsets(offsets)
	reversedAddress, reversedOK, reversedTrace := resolveTooltipPointerChainInOrder(processHandle, label+" reversed", baseAddress, reversedOffsets)
	if reversedOK {
		return reversedAddress, true, reversedTrace
	}

	return 0, false, trace + " || " + reversedTrace
}

func resolveTooltipPointerChainInOrder(processHandle uintptr, label string, baseAddress uintptr, offsets []uintptr) (uintptr, bool, string) {
	currentAddress := baseAddress
	steps := []string{fmt.Sprintf("%s base=0x%X offsets=%s", label, baseAddress, FormatPointerOffsets(offsets))}
	for index, offset := range offsets {
		nextAddress, ok := ReadUintptr(processHandle, currentAddress)
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

func ReversePointerOffsets(offsets []uintptr) []uintptr {
	reversed := make([]uintptr, len(offsets))
	for i, offset := range offsets {
		reversed[len(offsets)-1-i] = offset
	}
	return reversed
}

func FormatPointerOffsets(offsets []uintptr) string {
	parts := make([]string, 0, len(offsets))
	for _, offset := range offsets {
		parts = append(parts, fmt.Sprintf("0x%X", offset))
	}
	return strings.Join(parts, ",")
}

func formatPointerOffsets(offsets []uintptr) string {
	return FormatPointerOffsets(offsets)
}
