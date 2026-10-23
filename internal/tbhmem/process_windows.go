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

// ScanPattern and ScanPatterns require a positive per-pattern result limit.
func (process *Process) ScanPattern(pattern []byte, maxResults int) ([]uintptr, uint64) {
	results, scanned := process.ScanPatterns([][]byte{pattern}, maxResults)
	return results[0], scanned
}

func (process *Process) ScanPatterns(patterns [][]byte, maxResults int) ([][]uintptr, uint64) {
	results := make([][]uintptr, len(patterns))
	longest := longestPattern(patterns)
	if process == nil || process.Handle == 0 || longest == 0 || maxResults <= 0 {
		return results, 0
	}
	// One allocation per scan, shared by every pattern and memory region. The
	// prefix holds only the overlap needed to match across chunk boundaries.
	buffer := make([]byte, maxScanChunkBytes+longest-1)
	minAddr, maxAddr := applicationAddressRange()
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
			scanned += scanMemoryRegion(process.ReadBytes, mbi.BaseAddress, mbi.RegionSize, patterns, maxResults, results, buffer, maxScanChunkBytes)
			if allPatternLimitsReached(patterns, results, maxResults) {
				break
			}
		}
		address = next
	}
	return results, scanned
}

func longestPattern(patterns [][]byte) int {
	longest := 0
	for _, pattern := range patterns {
		if len(pattern) > longest {
			longest = len(pattern)
		}
	}
	return longest
}

// read must fill the requested buffer completely or return false. Failed chunks
// break the overlap, so a match is never assembled across an unreadable gap.
func scanMemoryRegion(read func(uintptr, []byte) bool, base, size uintptr, patterns [][]byte, limit int, results [][]uintptr, buffer []byte, chunkBytes int) uint64 {
	overlap := longestPattern(patterns) - 1
	if overlap < 0 || limit <= 0 || chunkBytes <= 0 || len(buffer) < chunkBytes+overlap {
		return 0
	}
	tail := 0
	var scanned uint64
	for offset := uintptr(0); offset < size; {
		count := chunkBytes
		if size-offset < uintptr(count) {
			count = int(size - offset)
		}
		if !read(base+offset, buffer[tail:tail+count]) {
			tail = 0
			offset += uintptr(count)
			continue
		}
		scanned += uint64(count)
		data := buffer[:tail+count]
		start := base + offset - uintptr(tail)
		for i, pattern := range patterns {
			if len(pattern) == 0 || len(results[i]) >= limit {
				continue
			}
			// A shorter pattern may already have matched wholly inside the
			// shared overlap; start at the first newly readable match position.
			from := tail - len(pattern) + 1
			if from < 0 {
				from = 0
			}
			for from+len(pattern) <= len(data) {
				index := bytes.Index(data[from:], pattern)
				if index < 0 {
					break
				}
				match := from + index
				results[i] = append(results[i], start+uintptr(match))
				if len(results[i]) >= limit {
					break
				}
				from = match + 1
			}
		}
		if allPatternLimitsReached(patterns, results, limit) {
			break
		}
		tail = overlap
		if len(data) < tail {
			tail = len(data)
		}
		copy(buffer[:tail], data[len(data)-tail:])
		offset += uintptr(count)
	}
	return scanned
}

func allPatternLimitsReached(patterns [][]byte, results [][]uintptr, limit int) bool {
	if limit <= 0 {
		return false
	}
	for i, pattern := range patterns {
		if len(pattern) > 0 && len(results[i]) < limit {
			return false
		}
	}
	return true
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
