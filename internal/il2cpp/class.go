package il2cpp

import "encoding/binary"

const (
	ObjectClassOffset = 0x0

	ClassNameOffset         = 0x10
	ClassElementClassOffset = 0x40
	ClassCastClassOffset    = 0x48

	ListItemsOffset = 0x10
	ListSizeOffset  = 0x18
	ArrayMaxOffset  = 0x18
	ArrayDataOffset = 0x20
)

type Memory interface {
	ReadUintptr(address uintptr) (uintptr, bool)
	ScanPattern(pattern []byte, maxResults int) ([]uintptr, uint64)
}

func ResolveClassByName(memory Memory, name string) ([]uintptr, bool) {
	stringAddresses, _ := memory.ScanPattern([]byte(name+"\x00"), 64)
	if len(stringAddresses) == 0 {
		stringAddresses, _ = memory.ScanPattern([]byte(name), 64)
	}
	if len(stringAddresses) == 0 {
		return nil, false
	}

	seen := make(map[uintptr]struct{})
	classes := make([]uintptr, 0, 1)
	for _, stringAddress := range stringAddresses {
		ptrBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(ptrBytes, uint64(stringAddress))
		refs, _ := memory.ScanPattern(ptrBytes, 5000)
		for _, ref := range refs {
			if ref < ClassNameOffset {
				continue
			}
			classPtr := ref - ClassNameOffset
			if _, exists := seen[classPtr]; exists {
				continue
			}
			if validateClass(memory, classPtr, stringAddress) {
				seen[classPtr] = struct{}{}
				classes = append(classes, classPtr)
			}
		}
	}
	return classes, len(classes) > 0
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
