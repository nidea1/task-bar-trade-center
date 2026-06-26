package app

import (
	"fmt"
	"syscall"
	"unsafe"
)

func redrawOverlay() {
	if !EnablePriceHUD || OverlayHWND == 0 {
		return
	}
	if !OverlayUpdatePending.CompareAndSwap(false, true) {
		return
	}
	procPostMessageW.Call(OverlayHWND, WM_OVERLAY_UPDATE, 0, 0)
}

func createOverlayWindow() {
	className, _ := syscall.UTF16PtrFromString(AppProcessName + "PriceOverlay")
	windowTitle, _ := syscall.UTF16PtrFromString(AppName + " Price HUD")
	hInstance, _, _ := procGetModuleHandleW.Call(0)

	wcex := WNDCLASSEX{
		Style:         CS_HREDRAW | CS_VREDRAW,
		LpfnWndProc:   syscall.NewCallback(wndProc),
		HInstance:     hInstance,
		LpszClassName: className,
	}
	wcex.CbSize = uint32(unsafe.Sizeof(wcex))

	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wcex)))

	OverlayWidth.Store(TooltipOverlayWidth)
	OverlayHeight.Store(TooltipOverlayHeight)

	OverlayHWND, _, _ = procCreateWindowExW.Call(
		WS_EX_TOPMOST|WS_EX_TRANSPARENT|WS_EX_LAYERED|WS_EX_TOOLWINDOW,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowTitle)),
		WS_POPUP,
		0, 0, uintptr(TooltipOverlayWidth), uintptr(TooltipOverlayHeight),
		0, 0, hInstance, 0,
	)
	if OverlayHWND == 0 {
		fmt.Println("Overlay window could not be created.")
		return
	}

	procSetLayeredWindowAttributes.Call(OverlayHWND, 0, 0, LWA_COLORKEY)
	procShowWindow.Call(OverlayHWND, SW_HIDE)
	fmt.Printf("Overlay window ready: hwnd=0x%X size=%dx%d\n", OverlayHWND, TooltipOverlayWidth, TooltipOverlayHeight)
}

func wndProc(hWnd uintptr, msg uint32, wParam uintptr, lParam uintptr) uintptr {
	switch msg {
	case WM_OVERLAY_UPDATE:
		OverlayUpdatePending.Store(false)
		if !ShowOverlay.Load() {
			HasLastOverlayRect = false
			procShowWindow.Call(hWnd, SW_HIDE)
			return 0
		}

		rect, ok := marketOverlayRect()
		if !ok {
			procShowWindow.Call(hWnd, SW_HIDE)
			return 0
		}
		width := rect.Right - rect.Left
		height := rect.Bottom - rect.Top
		if width <= 0 || height <= 0 {
			return 0
		}

		OverlayWidth.Store(width)
		OverlayHeight.Store(height)
		procSetWindowPos.Call(
			hWnd,
			^uintptr(0),
			uintptr(int(rect.Left)),
			uintptr(int(rect.Top)),
			uintptr(int(width)),
			uintptr(int(height)),
			SWP_NOACTIVATE|SWP_SHOWWINDOW,
		)
		procShowWindow.Call(hWnd, SW_SHOWNA)
		procInvalidateRect.Call(hWnd, 0, 1)
		procUpdateWindow.Call(hWnd)
		return 0
	case WM_PAINT:
		var ps PAINTSTRUCT
		hdc, _, _ := procBeginPaint.Call(hWnd, uintptr(unsafe.Pointer(&ps)))

		if ShowOverlay.Load() {
			width := OverlayWidth.Load()
			height := OverlayHeight.Load()
			if width <= 0 {
				width = TooltipOverlayWidth
			}
			if height <= 0 {
				height = TooltipOverlayHeight
			}
			rect := RECT{
				Left:   0,
				Top:    0,
				Right:  width,
				Bottom: height,
			}
			if !OverlayPaintLogged {
				OverlayPaintLogged = true
				fmt.Printf("Overlay paint received: clientRect=(%d,%d,%d,%d)\n", rect.Left, rect.Top, rect.Right, rect.Bottom)
			}

			drawGameMarketOverlay(hdc, rect)
		}

		procEndPaint.Call(hWnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	case WM_DESTROY:
		procPostQuitMessage.Call(0)
		return 0
	}
	return winDefWindowProc(hWnd, msg, wParam, lParam)
}

func winDefWindowProc(hWnd uintptr, msg uint32, wParam uintptr, lParam uintptr) uintptr {
	ret, _, _ := procDefWindowProcW.Call(hWnd, uintptr(msg), wParam, lParam)
	return ret
}
