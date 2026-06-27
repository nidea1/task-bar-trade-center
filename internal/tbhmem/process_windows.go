//go:build windows

package tbhmem

import (
	"bytes"
	"math"
	"strings"
	"syscall"
	"unsafe"
)

const (
	processVMRead           = 0x0010
	processQueryInformation = 0x0400
	th32csSnapProcess       = 0x00000002
	memCommit               = 0x1000
	pageNoAccess            = 0x01
	pageGuard               = 0x100
	maxScanChunkBytes       = 4 << 20
)

var (
	kernel32                     = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess              = kernel32.NewProc("OpenProcess")
	procReadProcessMemory        = kernel32.NewProc("ReadProcessMemory")
	procCloseHandle              = kernel32.NewProc("CloseHandle")
	procCreateToolhelp32Snapshot = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW          = kernel32.NewProc("Process32FirstW")
	procProcess32NextW           = kernel32.NewProc("Process32NextW")
	procVirtualQueryEx           = kernel32.NewProc("VirtualQueryEx")
	procGetNativeSystemInfo      = kernel32.NewProc("GetNativeSystemInfo")
	procEnumProcessModules       = kernel32.NewProc("K32EnumProcessModules")
	procGetModuleBaseNameW       = kernel32.NewProc("K32GetModuleBaseNameW")
)

type Process struct {
	PID    uint32
	Handle uintptr
	own    bool
}

type processEntry32W struct {
	DwSize              uint32
	CntUsage            uint32
	Th32ProcessID       uint32
	Th32DefaultHeapID   uintptr
	Th32ModuleID        uint32
	CntThreads          uint32
	Th32ParentProcessID uint32
	PcPriClassBase      int32
	DwFlags             uint32
	SzExeFile           [260]uint16
}

type memoryBasicInformation struct {
	BaseAddress       uintptr
	AllocationBase    uintptr
	AllocationProtect uint32
	PartitionID       uint16
	_                 uint16
	RegionSize        uintptr
	State             uint32
	Protect           uint32
	Type              uint32
	_                 uint32
}

type systemInfo struct {
	ProcessorArchitecture     uint16
	Reserved                  uint16
	PageSize                  uint32
	MinimumApplicationAddress uintptr
	MaximumApplicationAddress uintptr
	ActiveProcessorMask       uintptr
	NumberOfProcessors        uint32
	ProcessorType             uint32
	AllocationGranularity     uint32
	ProcessorLevel            uint16
	ProcessorRevision         uint16
}

func FromHandle(pid uint32, handle uintptr) *Process {
	if handle == 0 {
		return nil
	}
	return &Process{PID: pid, Handle: handle}
}

func OpenByName(processName string) (*Process, bool) {
	pid := FindProcessID(processName)
	if pid == 0 {
		return nil, false
	}
	handle, _, _ := procOpenProcess.Call(processVMRead|processQueryInformation, 0, uintptr(pid))
	if handle == 0 {
		return nil, false
	}
	return &Process{PID: pid, Handle: handle, own: true}, true
}

func (process *Process) Close() {
	if process == nil || !process.own || process.Handle == 0 {
		return
	}
	procCloseHandle.Call(process.Handle)
	process.Handle = 0
}

func FindProcessID(processName string) uint32 {
	snapshot, _, _ := procCreateToolhelp32Snapshot.Call(th32csSnapProcess, 0)
	if snapshot == ^uintptr(0) {
		return 0
	}
	defer procCloseHandle.Call(snapshot)

	var entry processEntry32W
	entry.DwSize = uint32(unsafe.Sizeof(entry))
	ret, _, _ := procProcess32FirstW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	for ret != 0 {
		exeName := syscall.UTF16ToString(entry.SzExeFile[:])
		if strings.EqualFold(exeName, processName) {
			return entry.Th32ProcessID
		}
		ret, _, _ = procProcess32NextW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	}
	return 0
}

func (process *Process) ModuleBase(moduleName string) uintptr {
	if process == nil || process.Handle == 0 {
		return 0
	}
	var modules [1024]uintptr
	var cbNeeded uint32
	res, _, _ := procEnumProcessModules.Call(process.Handle, uintptr(unsafe.Pointer(&modules[0])), unsafe.Sizeof(modules), uintptr(unsafe.Pointer(&cbNeeded)))
	if res == 0 {
		return 0
	}
	count := cbNeeded / uint32(unsafe.Sizeof(modules[0]))
	for i := uint32(0); i < count; i++ {
		var name [266]uint16
		procGetModuleBaseNameW.Call(process.Handle, modules[i], uintptr(unsafe.Pointer(&name[0])), unsafe.Sizeof(name))
		if strings.EqualFold(syscall.UTF16ToString(name[:]), moduleName) {
			return modules[i]
		}
	}
	return 0
}

func (process *Process) ReadBytes(address uintptr, destination []byte) bool {
	if process == nil || process.Handle == 0 || len(destination) == 0 {
		return len(destination) == 0
	}
	var bytesRead uintptr
	ret, _, _ := procReadProcessMemory.Call(
		process.Handle,
		address,
		uintptr(unsafe.Pointer(&destination[0])),
		uintptr(len(destination)),
		uintptr(unsafe.Pointer(&bytesRead)),
	)
	return ret != 0 && bytesRead == uintptr(len(destination))
}

func (process *Process) ReadUintptr(address uintptr) (uintptr, bool) {
	var value uintptr
	return value, process.readValue(address, unsafe.Pointer(&value), unsafe.Sizeof(value))
}

func (process *Process) ReadUint64(address uintptr) (uint64, bool) {
	var value uint64
	return value, process.readValue(address, unsafe.Pointer(&value), unsafe.Sizeof(value))
}

func (process *Process) ReadInt32(address uintptr) (int32, bool) {
	var value int32
	return value, process.readValue(address, unsafe.Pointer(&value), unsafe.Sizeof(value))
}

func (process *Process) readValue(address uintptr, pointer unsafe.Pointer, size uintptr) bool {
	if process == nil || process.Handle == 0 {
		return false
	}
	var bytesRead uintptr
	ret, _, _ := procReadProcessMemory.Call(process.Handle, address, uintptr(pointer), size, uintptr(unsafe.Pointer(&bytesRead)))
	return ret != 0 && bytesRead == size
}

func (process *Process) ScanPattern(pattern []byte, maxResults int) ([]uintptr, uint64) {
	minAddr, maxAddr := applicationAddressRange()
	var results []uintptr
	var scanned uint64
	for address := minAddr; address < maxAddr; {
		var mbi memoryBasicInformation
		ret, _, _ := procVirtualQueryEx.Call(process.Handle, address, uintptr(unsafe.Pointer(&mbi)), unsafe.Sizeof(mbi))
		if ret == 0 {
			break
		}
		next := mbi.BaseAddress + mbi.RegionSize
		if next <= address {
			break
		}
		if isReadableCommitted(mbi) {
			matches, bytesRead := process.scanRegion(mbi.BaseAddress, mbi.RegionSize, pattern, maxResults-len(results))
			results = append(results, matches...)
			scanned += bytesRead
			if maxResults > 0 && len(results) >= maxResults {
				return results, scanned
			}
		}
		address = next
	}
	return results, scanned
}

func (process *Process) scanRegion(base uintptr, size uintptr, pattern []byte, remaining int) ([]uintptr, uint64) {
	if len(pattern) == 0 || remaining == 0 {
		return nil, 0
	}
	var results []uintptr
	var scanned uint64
	tail := make([]byte, 0, len(pattern)-1)
	for offset := uintptr(0); offset < size; {
		chunkSize := uintptr(maxScanChunkBytes)
		if size-offset < chunkSize {
			chunkSize = size - offset
		}
		chunk := make([]byte, int(chunkSize))
		if !process.ReadBytes(base+offset, chunk) {
			offset += chunkSize
			tail = tail[:0]
			continue
		}
		scanned += uint64(len(chunk))
		combined := make([]byte, len(tail)+len(chunk))
		copy(combined, tail)
		copy(combined[len(tail):], chunk)
		combinedBase := base + offset - uintptr(len(tail))
		searchFrom := 0
		for {
			index := bytes.Index(combined[searchFrom:], pattern)
			if index < 0 {
				break
			}
			matchOffset := searchFrom + index
			if matchOffset+len(pattern) > len(tail) {
				results = append(results, combinedBase+uintptr(matchOffset))
				if len(results) >= remaining {
					return results, scanned
				}
			}
			searchFrom = matchOffset + 1
		}
		tailLen := len(pattern) - 1
		if tailLen > len(combined) {
			tailLen = len(combined)
		}
		tail = append(tail[:0], combined[len(combined)-tailLen:]...)
		offset += chunkSize
	}
	return results, scanned
}

func PlausibleAddress(address uintptr) bool {
	return address > 0x10000 && address < uintptr(math.MaxUint64>>17)
}

func applicationAddressRange() (uintptr, uintptr) {
	var info systemInfo
	procGetNativeSystemInfo.Call(uintptr(unsafe.Pointer(&info)))
	minAddr := info.MinimumApplicationAddress
	maxAddr := info.MaximumApplicationAddress
	if minAddr == 0 {
		minAddr = 0x10000
	}
	if maxAddr == 0 {
		maxAddr = uintptr(math.MaxUint64 >> 17)
	}
	return minAddr, maxAddr
}

func isReadableCommitted(mbi memoryBasicInformation) bool {
	return mbi.State == memCommit && mbi.Protect&pageGuard == 0 && mbi.Protect&pageNoAccess == 0
}
