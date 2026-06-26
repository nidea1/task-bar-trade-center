package winapp

import (
	"syscall"
	"unsafe"

	"github.com/nidea1/task-bar-trade-center/internal/win32"
)

func LoadIconResource(hInstance uintptr, resourceID uintptr, size int32, fallbackID uintptr) (uintptr, bool) {
	icon, _, _ := win32.ProcLoadImageW.Call(
		hInstance,
		resourceID,
		1, // IMAGE_ICON
		uintptr(size),
		uintptr(size),
		0,
	)
	if icon != 0 {
		return icon, true
	}

	// Fallback to load from assets/icon.ico on disk (useful in development)
	if pathPtr, err := syscall.UTF16PtrFromString("assets/icon.ico"); err == nil {
		fileIcon, _, _ := win32.ProcLoadImageW.Call(
			0,
			uintptr(unsafe.Pointer(pathPtr)),
			1, // IMAGE_ICON
			uintptr(size),
			uintptr(size),
			0x0010, // LR_LOADFROMFILE
		)
		if fileIcon != 0 {
			return fileIcon, true
		}
	}

	fallbackIcon, _, _ := win32.ProcLoadIconW.Call(0, fallbackID)
	return fallbackIcon, false
}

func LoadIconFile(path string, size int32) uintptr {
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0
	}
	icon, _, _ := win32.ProcLoadImageW.Call(
		0,
		uintptr(unsafe.Pointer(pathPtr)),
		1, // IMAGE_ICON
		uintptr(size),
		uintptr(size),
		0x0010, // LR_LOADFROMFILE
	)
	return icon
}

func AddNotifyIcon(nid *win32.NOTIFYICONDATAW) bool {
	ret, _, _ := win32.ProcShellNotifyIcon.Call(0, uintptr(unsafe.Pointer(nid)))
	return ret != 0
}

func SetNotifyIconVersion(nid *win32.NOTIFYICONDATAW) {
	win32.ProcShellNotifyIcon.Call(4, uintptr(unsafe.Pointer(nid)))
}

func ModifyNotifyIcon(nid *win32.NOTIFYICONDATAW) {
	win32.ProcShellNotifyIcon.Call(1, uintptr(unsafe.Pointer(nid)))
}

func DeleteNotifyIcon(nid *win32.NOTIFYICONDATAW) {
	win32.ProcShellNotifyIcon.Call(2, uintptr(unsafe.Pointer(nid)))
}

func NewPopupMenu() uintptr {
	menu, _, _ := win32.ProcCreatePopupMenu.Call()
	return menu
}

func DestroyMenu(menu uintptr) {
	if menu != 0 {
		win32.ProcDestroyMenu.Call(menu)
	}
}

func AppendMenuItem(menu uintptr, flags uint32, id uint32, text string) {
	textUTF16, _ := syscall.UTF16PtrFromString(text)
	win32.ProcAppendMenuW.Call(menu, uintptr(flags), uintptr(id), uintptr(unsafe.Pointer(textUTF16)))
}

func AppendPopupMenu(menu uintptr, popupMenu uintptr, text string, popupFlag uint32) {
	textUTF16, _ := syscall.UTF16PtrFromString(text)
	win32.ProcAppendMenuW.Call(menu, uintptr(popupFlag), popupMenu, uintptr(unsafe.Pointer(textUTF16)))
}

func AppendSeparator(menu uintptr, separatorFlag uint32) {
	win32.ProcAppendMenuW.Call(menu, uintptr(separatorFlag), 0, 0)
}

func TrackPopupAtCursor(menu uintptr, owner uintptr, flags uint32) {
	var cursor win32.POINT
	win32.ProcGetCursorPos.Call(uintptr(unsafe.Pointer(&cursor)))
	win32.ProcSetForegroundWindow.Call(owner)
	win32.ProcTrackPopupMenu.Call(menu, uintptr(flags), uintptr(int(cursor.X)), uintptr(int(cursor.Y)), 0, owner, 0)
	win32.ProcPostMessageW.Call(owner, 0, 0, 0)
}

func CopyUTF16(destination []uint16, value string) {
	if len(destination) == 0 {
		return
	}
	encoded, _ := syscall.UTF16FromString(value)
	if len(encoded) > len(destination) {
		encoded = encoded[:len(destination)]
		encoded[len(encoded)-1] = 0
	}
	copy(destination, encoded)
}
