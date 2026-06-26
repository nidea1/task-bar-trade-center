package winapp

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/nidea1/task-bar-trade-center/internal/win32"
)

const swShowDefault = 10

func OpenURL(targetURL string) bool {
	operation, _ := syscall.UTF16PtrFromString("open")
	target, _ := syscall.UTF16PtrFromString(targetURL)
	ret, _, _ := win32.ProcShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(operation)),
		uintptr(unsafe.Pointer(target)),
		0,
		0,
		swShowDefault,
	)
	if ret <= 32 {
		fmt.Printf("Could not open browser for Steam market listing. ShellExecute result=%d\n", ret)
		return false
	}
	return true
}
