package game

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"github.com/nidea1/task-bar-trade-center/internal/win32"
)

const (
	imageScnMemExecute = 0x20000000
	aobScanChunkSize   = 1 << 20
)

type AOBPattern struct {
	Source               string
	Bytes                []byte
	Wildcards            []bool
	DisplacementOffset   int
	InstructionEndOffset int
}

func parseOptionalAOBPattern(name string, source string, displacementOffset int, instructionEndOffset int) (AOBPattern, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return AOBPattern{}, nil
	}

	fields := strings.Fields(source)
	pattern := AOBPattern{
		Source:               source,
		Bytes:                make([]byte, len(fields)),
		Wildcards:            make([]bool, len(fields)),
		DisplacementOffset:   displacementOffset,
		InstructionEndOffset: instructionEndOffset,
	}
	for index, field := range fields {
		if field == "?" || field == "??" {
			pattern.Wildcards[index] = true
			continue
		}
		if len(field) != 2 {
			return AOBPattern{}, fmt.Errorf("%s pattern token %d must be a byte or wildcard", name, index)
		}
		value, err := strconv.ParseUint(field, 16, 8)
		if err != nil {
			return AOBPattern{}, fmt.Errorf("%s pattern token %d is invalid: %w", name, index, err)
		}
		pattern.Bytes[index] = byte(value)
	}

	if displacementOffset < 0 || displacementOffset+4 > len(pattern.Bytes) {
		return AOBPattern{}, fmt.Errorf("%s displacement_offset must select four bytes within the pattern", name)
	}
	if instructionEndOffset < displacementOffset+4 || instructionEndOffset > len(pattern.Bytes) {
		return AOBPattern{}, fmt.Errorf("%s instruction_end_offset must follow the displacement within the pattern", name)
	}
	return pattern, nil
}

func (pattern AOBPattern) configured() bool {
	return len(pattern.Bytes) > 0
}

func (pattern AOBPattern) Configured() bool {
	return pattern.configured()
}

func (pattern AOBPattern) matches(data []byte, offset int) bool {
	if offset < 0 || offset+len(pattern.Bytes) > len(data) {
		return false
	}
	for index, value := range pattern.Bytes {
		if !pattern.Wildcards[index] && data[offset+index] != value {
			return false
		}
	}
	return true
}

func aobMatchOffsets(data []byte, pattern AOBPattern) []int {
	if !pattern.configured() || len(data) < len(pattern.Bytes) {
		return nil
	}

	matches := make([]int, 0)
	for offset := 0; offset+len(pattern.Bytes) <= len(data); offset++ {
		if pattern.matches(data, offset) {
			matches = append(matches, offset)
		}
	}
	return matches
}

func resolveAOBPointerBase(matchAddress uintptr, matchedBytes []byte, pattern AOBPattern) (uintptr, bool) {
	if len(matchedBytes) < len(pattern.Bytes) || pattern.DisplacementOffset+4 > len(matchedBytes) {
		return 0, false
	}
	displacement := int64(int32(binary.LittleEndian.Uint32(matchedBytes[pattern.DisplacementOffset:])))
	target := int64(matchAddress) + int64(pattern.InstructionEndOffset) + displacement
	if target <= 0 {
		return 0, false
	}
	return uintptr(target), true
}

type moduleMemorySection struct {
	address uintptr
	size    uintptr
}

func executableModuleSections(processHandle uintptr, moduleBase uintptr) ([]moduleMemorySection, uintptr, error) {
	initialHeader := make([]byte, 0x1000)
	if !readProcessBytes(processHandle, moduleBase, initialHeader) {
		return nil, 0, fmt.Errorf("could not read module header")
	}
	if string(initialHeader[:2]) != "MZ" {
		return nil, 0, fmt.Errorf("module does not have an MZ header")
	}
	peOffset := int(binary.LittleEndian.Uint32(initialHeader[0x3C:]))
	if peOffset < 0 || peOffset > 1<<20 {
		return nil, 0, fmt.Errorf("PE header offset is invalid")
	}

	headerSize := peOffset + 24
	if headerSize > len(initialHeader) {
		initialHeader = make([]byte, headerSize)
		if !readProcessBytes(processHandle, moduleBase, initialHeader) {
			return nil, 0, fmt.Errorf("could not read PE header")
		}
	}
	if string(initialHeader[peOffset:peOffset+4]) != "PE\x00\x00" {
		return nil, 0, fmt.Errorf("module does not have a PE header")
	}

	sectionCount := int(binary.LittleEndian.Uint16(initialHeader[peOffset+6:]))
	optionalHeaderSize := int(binary.LittleEndian.Uint16(initialHeader[peOffset+20:]))
	sectionTableOffset := peOffset + 24 + optionalHeaderSize
	if sectionCount == 0 || sectionCount > 96 || sectionTableOffset+sectionCount*40 > 1<<20 {
		return nil, 0, fmt.Errorf("PE section table is invalid")
	}
	headerSize = sectionTableOffset + sectionCount*40
	if headerSize > len(initialHeader) {
		initialHeader = make([]byte, headerSize)
		if !readProcessBytes(processHandle, moduleBase, initialHeader) {
			return nil, 0, fmt.Errorf("could not read PE section table")
		}
	}

	sections := make([]moduleMemorySection, 0, sectionCount)
	var moduleSize uintptr
	for index := 0; index < sectionCount; index++ {
		offset := sectionTableOffset + index*40
		virtualSize := uintptr(binary.LittleEndian.Uint32(initialHeader[offset+8:]))
		virtualAddress := uintptr(binary.LittleEndian.Uint32(initialHeader[offset+12:]))
		rawSize := uintptr(binary.LittleEndian.Uint32(initialHeader[offset+16:]))
		characteristics := binary.LittleEndian.Uint32(initialHeader[offset+36:])
		if virtualSize == 0 {
			virtualSize = rawSize
		}
		if virtualAddress+virtualSize > moduleSize {
			moduleSize = virtualAddress + virtualSize
		}
		if characteristics&imageScnMemExecute != 0 && virtualSize > 0 {
			sections = append(sections, moduleMemorySection{address: moduleBase + virtualAddress, size: virtualSize})
		}
	}
	if len(sections) == 0 || moduleSize == 0 {
		return nil, 0, fmt.Errorf("module has no executable sections")
	}
	return sections, moduleSize, nil
}

func readProcessBytes(processHandle uintptr, address uintptr, destination []byte) bool {
	if len(destination) == 0 {
		return true
	}
	var bytesRead uintptr
	ret, _, _ := win32.ProcReadProcessMemory.Call(
		processHandle,
		address,
		uintptr(unsafe.Pointer(&destination[0])),
		uintptr(len(destination)),
		uintptr(unsafe.Pointer(&bytesRead)),
	)
	return ret != 0 && bytesRead == uintptr(len(destination))
}

func findAOBPointerBaseCandidates(processHandle uintptr, moduleBase uintptr, pattern AOBPattern) ([]uintptr, error) {
	if !pattern.configured() {
		return nil, nil
	}
	sections, moduleSize, err := executableModuleSections(processHandle, moduleBase)
	if err != nil {
		return nil, err
	}

	seenCandidates := make(map[uintptr]struct{})
	addresses := make([]uintptr, 0)
	readableSections := 0
	for _, section := range sections {
		matches, ok := scanRemoteSectionForAOB(processHandle, section, pattern)
		if !ok {
			continue
		}
		readableSections++
		for _, match := range matches {
			baseAddress, ok := resolveAOBPointerBase(match.address, match.bytes, pattern)
			if !ok || baseAddress < moduleBase || baseAddress >= moduleBase+moduleSize {
				continue
			}
			if _, seen := seenCandidates[baseAddress]; !seen {
				seenCandidates[baseAddress] = struct{}{}
				addresses = append(addresses, baseAddress)
			}
		}
	}
	if readableSections == 0 {
		return nil, fmt.Errorf("could not read an executable module section")
	}
	return addresses, nil
}

type aobMatch struct {
	address uintptr
	bytes   []byte
}

func scanRemoteSectionForAOB(processHandle uintptr, section moduleMemorySection, pattern AOBPattern) ([]aobMatch, bool) {
	var matches []aobMatch
	tail := make([]byte, 0, len(pattern.Bytes)-1)
	for offset := uintptr(0); offset < section.size; {
		chunkSize := uintptr(aobScanChunkSize)
		if remaining := section.size - offset; remaining < chunkSize {
			chunkSize = remaining
		}
		chunk := make([]byte, int(chunkSize))
		if !readProcessBytes(processHandle, section.address+offset, chunk) {
			return nil, false
		}

		combined := make([]byte, len(tail)+len(chunk))
		copy(combined, tail)
		copy(combined[len(tail):], chunk)
		combinedAddress := section.address + offset - uintptr(len(tail))
		for _, matchOffset := range aobMatchOffsets(combined, pattern) {
			if matchOffset+len(pattern.Bytes) <= len(tail) {
				continue
			}
			matchedBytes := append([]byte(nil), combined[matchOffset:matchOffset+len(pattern.Bytes)]...)
			matches = append(matches, aobMatch{address: combinedAddress + uintptr(matchOffset), bytes: matchedBytes})
		}

		tailLength := len(pattern.Bytes) - 1
		if tailLength > len(combined) {
			tailLength = len(combined)
		}
		tail = append(tail[:0], combined[len(combined)-tailLength:]...)
		offset += chunkSize
	}
	return matches, true
}

type HoveredItemAOBResolver struct {
	configurationKey string
	scanned          bool
	candidates       []uintptr
	resolvedBase     uintptr
}

func (resolver *HoveredItemAOBResolver) Read(processHandle uintptr, moduleBase uintptr, layout GameLayout, itemExists func(int32) bool) (int32, string, int32, bool) {
	pattern := layout.HoveredItemPointerBaseAOB
	if !pattern.configured() {
		return 0, "", 0, false
	}

	configurationKey := fmt.Sprintf("%s:%d:%d:%X:%s", pattern.Source, pattern.DisplacementOffset, pattern.InstructionEndOffset, layout.HoveredItemKeyOffset, formatPointerOffsets(layout.HoveredItemPointerOffsets))
	if resolver.configurationKey != configurationKey {
		*resolver = HoveredItemAOBResolver{configurationKey: configurationKey}
	}
	if !resolver.scanned {
		candidates, err := findAOBPointerBaseCandidates(processHandle, moduleBase, pattern)
		resolver.scanned = true
		if err != nil {
			fmt.Printf("Hovered item AOB scan failed: %v\n", err)
			return 0, "", 0, false
		}
		resolver.candidates = candidates
		fmt.Printf("Hovered item AOB scan found %d pointer-base candidate(s).\n", len(candidates))
	}

	if resolver.resolvedBase != 0 {
		return ReadHoveredItemID(processHandle, resolver.resolvedBase, layout.HoveredItemPointerOffsets, layout.HoveredItemItemPtrOffset, layout.HoveredItemKeyOffset, itemExists)
	}

	var readableID int32
	var readableMode string
	var readableRaw int32
	readableCandidateFound := false
	for _, candidate := range resolver.candidates {
		itemID, readMode, rawValue, ok := ReadHoveredItemID(processHandle, candidate, layout.HoveredItemPointerOffsets, layout.HoveredItemItemPtrOffset, layout.HoveredItemKeyOffset, itemExists)
		if !ok {
			continue
		}
		if itemID > 0 {
			resolver.resolvedBase = candidate
			fmt.Printf("Hovered item AOB selected pointer base: 0x%X\n", candidate)
			return itemID, readMode, rawValue, true
		}
		if !readableCandidateFound {
			readableID = itemID
			readableMode = readMode
			readableRaw = rawValue
			readableCandidateFound = true
		}
	}
	return readableID, readableMode, readableRaw, readableCandidateFound
}

type TooltipAOBResolver struct {
	mu               sync.Mutex
	configurationKey string
	scanned          bool
	candidates       []uintptr
	resolvedBase     uintptr
}

func (resolver *TooltipAOBResolver) Reset() {
	resolver.mu.Lock()
	defer resolver.mu.Unlock()
	resolver.configurationKey = ""
	resolver.scanned = false
	resolver.candidates = nil
	resolver.resolvedBase = 0
}

func (resolver *TooltipAOBResolver) Resolve(label string, processHandle uintptr, moduleBase uintptr, pattern AOBPattern, offsets []uintptr) (uintptr, bool, string) {
	if !pattern.configured() {
		return 0, false, ""
	}

	resolver.mu.Lock()
	defer resolver.mu.Unlock()

	configurationKey := fmt.Sprintf("%X:%s:%d:%d:%s", moduleBase, pattern.Source, pattern.DisplacementOffset, pattern.InstructionEndOffset, formatPointerOffsets(offsets))
	if resolver.configurationKey != configurationKey {
		resolver.configurationKey = configurationKey
		resolver.scanned = false
		resolver.candidates = nil
		resolver.resolvedBase = 0
	}
	if !resolver.scanned {
		candidates, err := findAOBPointerBaseCandidates(processHandle, moduleBase, pattern)
		resolver.scanned = true
		if err != nil {
			return 0, false, fmt.Sprintf("%s AOB scan failed: %v", label, err)
		}
		resolver.candidates = candidates
		fmt.Printf("Tooltip %s AOB scan found %d pointer-base candidate(s).\n", label, len(candidates))
	}

	if resolver.resolvedBase != 0 {
		if address, ok, trace := ResolveTooltipPointerChain(processHandle, label+" AOB", resolver.resolvedBase, offsets); ok {
			return address, true, trace
		}
		resolver.resolvedBase = 0
	}

	for _, candidate := range resolver.candidates {
		address, ok, trace := ResolveTooltipPointerChain(processHandle, label+" AOB", candidate, offsets)
		if !ok {
			continue
		}
		value, ok := ReadFloat32(processHandle, address)
		if !ok || !validTooltipAOBValue(label, value) {
			continue
		}
		resolver.resolvedBase = candidate
		fmt.Printf("Tooltip %s AOB selected pointer base: 0x%X\n", label, candidate)
		return address, true, trace
	}
	return 0, false, fmt.Sprintf("%s AOB candidates did not yield a valid value", label)
}

func validTooltipAOBValue(label string, value float32) bool {
	switch label {
	case "x":
		return value >= -600 && value <= 600 && (value <= -1 || value >= 1)
	case "y":
		return value >= -1000 && value <= 1000 && (value <= -1 || value >= 1)
	case "height":
		return value >= 60 && value <= 700
	default:
		return false
	}
}

func ValidTooltipAOBValue(label string, value float32) bool {
	return validTooltipAOBValue(label, value)
}
