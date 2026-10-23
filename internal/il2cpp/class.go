package il2cpp

import "encoding/binary"

const (
	ObjectClassOffset = 0x0

	ClassNameOffset         = 0x10
	ClassElementClassOffset = 0x40
	ClassCastClassOffset    = 0x48
	ClassInstanceSizeOffset = 0xF8

	ListItemsOffset = 0x10
	ListSizeOffset  = 0x18
	ArrayMaxOffset  = 0x18
	ArrayDataOffset = 0x20
)

type Memory interface {
	ReadUintptr(address uintptr) (uintptr, bool)
	ScanPattern(pattern []byte, maxResults int) ([]uintptr, uint64)
}

type multiPatternMemory interface {
	ScanPatterns(patterns [][]byte, maxResults int) ([][]uintptr, uint64)
}

func ResolveClassByName(memory Memory, name string) ([]uintptr, bool) {
	classes := ResolveClassesByName(memory, []string{name})[name]
	return classes, len(classes) > 0
}

// ResolveClassesByName reads the process once for all names and once for all
// name pointers, instead of making two full memory passes for every class.
func ResolveClassesByName(memory Memory, names []string) map[string][]uintptr {
	result := make(map[string][]uintptr, len(names))
	unique := make([]string, 0, len(names))
	patterns := make([][]byte, 0, len(names))
	for _, name := range names {
		if name == "" {
			continue
		}
		if _, exists := result[name]; exists {
			continue
		}
		result[name] = nil
		unique = append(unique, name)
		patterns = append(patterns, []byte(name+"\x00"))
	}
	if len(unique) == 0 {
		return result
	}
	strings := scanPatternBatch(memory, patterns, 64)
	var fallbackNames []int
	patterns = nil
	for i, addresses := range strings {
		if len(addresses) == 0 {
			fallbackNames = append(fallbackNames, i)
			patterns = append(patterns, []byte(unique[i]))
		}
	}
	if len(patterns) > 0 {
		fallback := scanPatternBatch(memory, patterns, 64)
		for i, addresses := range fallback {
			strings[fallbackNames[i]] = addresses
		}
	}
	patterns = nil
	var owners []string
	var addresses []uintptr
	for i, found := range strings {
		for _, address := range found {
			patterns = append(patterns, uintptrPattern(address))
			owners = append(owners, unique[i])
			addresses = append(addresses, address)
		}
	}
	if len(patterns) == 0 {
		return result
	}
	refs := scanPatternBatch(memory, patterns, 5000)
	seen := make(map[uintptr]bool)
	for i, matches := range refs {
		for _, ref := range matches {
			if ref < ClassNameOffset {
				continue
			}
			class := ref - ClassNameOffset
			if !seen[class] && validateClass(memory, class, addresses[i]) {
				seen[class] = true
				result[owners[i]] = append(result[owners[i]], class)
			}
		}
	}
	return result
}

// ClassHasName validates imported heap pointers before they can bypass scanning.
func ClassHasName(memory Memory, class uintptr, name string) bool {
	reader, ok := memory.(interface{ ReadBytes(uintptr, []byte) bool })
	if !ok {
		return false
	}
	address, ok := memory.ReadUintptr(class + ClassNameOffset)
	if !ok || !validateClass(memory, class, address) {
		return false
	}
	buffer := make([]byte, len(name)+1)
	return reader.ReadBytes(address, buffer) && string(buffer) == name+"\x00"
}

func scanPatternBatch(memory Memory, patterns [][]byte, maxResults int) [][]uintptr {
	if scanner, ok := memory.(multiPatternMemory); ok && len(patterns) > 1 {
		results, _ := scanner.ScanPatterns(patterns, maxResults)
		return results
	}
	results := make([][]uintptr, len(patterns))
	for index, pattern := range patterns {
		results[index], _ = memory.ScanPattern(pattern, maxResults)
	}
	return results
}

func uintptrPattern(value uintptr) []byte {
	ptrBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(ptrBytes, uint64(value))
	return ptrBytes
}

func validateClass(memory Memory, classPtr uintptr, stringAddress uintptr) bool {
	namePtr, ok := memory.ReadUintptr(classPtr + ClassNameOffset)
	if !ok || namePtr != stringAddress {
		return false
	}
	element, ok := memory.ReadUintptr(classPtr + ClassElementClassOffset)
	if !ok || element != classPtr {
		return false
	}
	cast, ok := memory.ReadUintptr(classPtr + ClassCastClassOffset)
	return ok && cast == classPtr
}
