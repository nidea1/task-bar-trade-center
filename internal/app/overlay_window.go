package app

import (
	"github.com/nidea1/task-bar-trade-center/internal/win32"

	"fmt"
	"syscall"
	"time"
	"unsafe"
)

const overlayRedrawMinInterval = 75 * time.Millisecond

func redrawOverlay() {
	if !EnablePriceHUD {
		return
	}
	overlayHWND := activeApp.overlayHWND
	if overlayHWND == 0 {
		requestOverlayWindowRecreate()
		return
	}
	now := time.Now()
	activeApp.overlayRedrawMu.Lock()
	if now.Sub(activeApp.lastOverlayRedrawAt) < overlayRedrawMinInterval {
		activeApp.overlayRedrawMu.Unlock()
		return
	}
	activeApp.lastOverlayRedrawAt = now
	activeApp.overlayRedrawMu.Unlock()
	if !activeApp.overlayUpdatePending.CompareAndSwap(false, true) {
		return
	}
	result, _, _ := win32.ProcPostMessageW.Call(overlayHWND, WM_OVERLAY_UPDATE, 0, 0)
	if result != 0 {
		return
	}
	activeApp.overlayUpdatePending.Store(false)
	if !isWindow(overlayHWND) {
		clearOverlayWindowState(overlayHWND)
		requestOverlayWindowRecreate()
	}
}

func createOverlayWindow() {
	if activeApp.overlayHWND != 0 {
		return
	}
	className, _ := syscall.UTF16PtrFromString(AppProcessName + "PriceOverlay")
	windowTitle, _ := syscall.UTF16PtrFromString(AppName + " Price HUD")
	hInstance, _, _ := win32.ProcGetModuleHandleW.Call(0)

	wcex := win32.WNDCLASSEX{
		Style:         CS_HREDRAW | CS_VREDRAW,
		LpfnWndProc:   syscall.NewCallback(wndProc),
		HInstance:     hInstance,
		LpszClassName: className,
	}
	wcex.CbSize = uint32(unsafe.Sizeof(wcex))

	win32.ProcRegisterClassExW.Call(uintptr(unsafe.Pointer(&wcex)))

	activeApp.overlayWidth.Store(TooltipOverlayWidth)
	activeApp.overlayHeight.Store(TooltipOverlayHeight)

	activeApp.overlayHWND, _, _ = win32.ProcCreateWindowExW.Call(
		WS_EX_TOPMOST|WS_EX_TRANSPARENT|WS_EX_LAYERED|WS_EX_TOOLWINDOW,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowTitle)),
		WS_POPUP,
		0, 0, uintptr(TooltipOverlayWidth), uintptr(TooltipOverlayHeight),
		0, 0, hInstance, 0,
	)
	if activeApp.overlayHWND == 0 {
		fmt.Println("Overlay window could not be created.")
		return
	}

	win32.ProcSetLayeredWindowAttributes.Call(activeApp.overlayHWND, 0, 0, LWA_COLORKEY)
	win32.ProcShowWindow.Call(activeApp.overlayHWND, SW_HIDE)
	fmt.Printf("Overlay window ready: hwnd=0x%X size=%dx%d\n", activeApp.overlayHWND, TooltipOverlayWidth, TooltipOverlayHeight)
}

func requestOverlayWindowRecreate() {
	if activeApp.appHWND == 0 || activeApp.shutdownRequested.Load() {
		return
	}
	win32.ProcPostMessageW.Call(activeApp.appHWND, WM_APP_RECREATE_OVERLAY, 0, 0)
}

func wndProc(hWnd uintptr, msg uint32, wParam uintptr, lParam uintptr) uintptr {
	switch msg {
	case WM_OVERLAY_UPDATE:
		activeApp.overlayUpdatePending.Store(false)
		if !activeApp.showOverlay.Load() {
			activeApp.hasLastOverlayRect = false
			activeApp.hasOverlayWindowRect = false
			activeApp.overlayWindowVisible = false
			win32.ProcShowWindow.Call(hWnd, SW_HIDE)
			return 0
		}

		rect, ok := marketOverlayRect()
		if !ok {
			activeApp.hasOverlayWindowRect = false
			activeApp.overlayWindowVisible = false
			win32.ProcShowWindow.Call(hWnd, SW_HIDE)
			return 0
		}
		width := rect.Right - rect.Left
		height := rect.Bottom - rect.Top
		if width <= 0 || height <= 0 {
			return 0
		}

		activeApp.overlayWidth.Store(width)
		activeApp.overlayHeight.Store(height)
		contentVersion := activeApp.overlayContentVersion.Load()
		moved := !activeApp.hasOverlayWindowRect || !rectsClose(activeApp.overlayWindowRect, rect, 3)
		contentChanged := activeApp.overlayPaintedContentVersion != contentVersion
		if !moved && !contentChanged && activeApp.overlayWindowVisible {
			return 0
		}
		if moved {
			win32.ProcSetWindowPos.Call(
				hWnd,
				^uintptr(0),
				uintptr(int(rect.Left)),
				uintptr(int(rect.Top)),
				uintptr(int(width)),
				uintptr(int(height)),
				SWP_NOACTIVATE|SWP_SHOWWINDOW,
			)
			activeApp.overlayWindowRect = rect
			activeApp.hasOverlayWindowRect = true
		}
		if !activeApp.overlayWindowVisible {
			win32.ProcShowWindow.Call(hWnd, SW_SHOWNA)
			activeApp.overlayWindowVisible = true
		}
		if contentChanged || moved {
			activeApp.overlayPaintedContentVersion = contentVersion
			win32.ProcInvalidateRect.Call(hWnd, 0, 1)
			win32.ProcUpdateWindow.Call(hWnd)
		}
		return 0
	case WM_PAINT:
		var ps win32.PAINTSTRUCT
		hdc, _, _ := win32.ProcBeginPaint.Call(hWnd, uintptr(unsafe.Pointer(&ps)))

		if activeApp.showOverlay.Load() {
			width := activeApp.overlayWidth.Load()
			height := activeApp.overlayHeight.Load()
			if width <= 0 {
				width = TooltipOverlayWidth
			}
			if height <= 0 {
				height = TooltipOverlayHeight
			}
			rect := win32.RECT{
				Left:   0,
				Top:    0,
				Right:  width,
				Bottom: height,
			}
			if !activeApp.overlayPaintLogged {
				activeApp.overlayPaintLogged = true
				fmt.Printf("Overlay paint received: clientRect=(%d,%d,%d,%d)\n", rect.Left, rect.Top, rect.Right, rect.Bottom)
			}

			drawGameMarketOverlay(hdc, rect)
		}

		win32.ProcEndPaint.Call(hWnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	case WM_DESTROY:
		clearOverlayWindowState(hWnd)
		return 0
	}
	return winDefWindowProc(hWnd, msg, wParam, lParam)
}

func clearOverlayWindowState(hwnd uintptr) {
	if activeApp.overlayHWND == hwnd {
		activeApp.overlayHWND = 0
	}
	activeApp.overlayUpdatePending.Store(false)
	activeApp.hasOverlayWindowRect = false
	activeApp.overlayWindowVisible = false
}

func isWindow(hwnd uintptr) bool {
	if hwnd == 0 {
		return false
	}
	result, _, _ := win32.ProcIsWindow.Call(hwnd)
	return result != 0
}

func rectsClose(a win32.RECT, b win32.RECT, tolerance int32) bool {
	return absInt32(a.Left-b.Left) <= tolerance &&
		absInt32(a.Top-b.Top) <= tolerance &&
		absInt32(a.Right-b.Right) <= tolerance &&
		absInt32(a.Bottom-b.Bottom) <= tolerance
}

func winDefWindowProc(hWnd uintptr, msg uint32, wParam uintptr, lParam uintptr) uintptr {
	ret, _, _ := win32.ProcDefWindowProcW.Call(hWnd, uintptr(msg), wParam, lParam)
	return ret
}
