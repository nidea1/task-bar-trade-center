package app

import (
	"fmt"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/game"
	"github.com/nidea1/task-bar-trade-center/internal/win32"
)

type tooltipMemorySnapshot struct {
	Rect    win32.RECT
	MemoryX float32
	MemoryY float32
}

func readTooltipRectFromMemory() (win32.RECT, bool) {
	snapshot, ok := readTooltipSnapshotFromMemory()
	if !ok {
		return win32.RECT{}, false
	}
	return snapshot.Rect, true
}

func readTooltipSnapshotFromMemory() (tooltipMemorySnapshot, bool) {
	if activeApp.gameProcessHandle == 0 || activeApp.gameAssemblyBase == 0 {
		logTooltipDebug("handle/base missing: handle=0x%X gameAssembly=0x%X", activeApp.gameProcessHandle, activeApp.gameAssemblyBase)
		return tooltipMemorySnapshot{}, false
	}

	activeApp.gameLayoutMu.RLock()
	layout := activeApp.activeGameLayout
	activeApp.gameLayoutMu.RUnlock()
	xBase := activeApp.gameAssemblyBase + layout.TooltipXPointerBaseOffset
	yBase := activeApp.gameAssemblyBase + layout.TooltipYPointerBaseOffset
	heightBase := activeApp.gameAssemblyBase + layout.TooltipHeightPointerBaseOffset

	xAddress, xChainOK, xTrace := activeApp.tooltipXAOBResolver.Resolve("x", activeApp.gameProcessHandle, activeApp.gameAssemblyBase, layout.TooltipXPointerBaseAOB, layout.TooltipXPointerOffsets)
	yAddress, yChainOK, yTrace := activeApp.tooltipYAOBResolver.Resolve("y", activeApp.gameProcessHandle, activeApp.gameAssemblyBase, layout.TooltipYPointerBaseAOB, layout.TooltipYPointerOffsets)
	heightAddress, heightChainOK, heightTrace := activeApp.tooltipHeightAOBResolver.Resolve("height", activeApp.gameProcessHandle, activeApp.gameAssemblyBase, layout.TooltipHeightPointerBaseAOB, layout.TooltipHeightPointerOffsets)
	if !xChainOK || !yChainOK {
		logTooltipDebugLines(
			"pointer chain status:",
			xTrace,
			yTrace,
			heightTrace,
		)
		return tooltipMemorySnapshot{}, false
	}

	x, ok := game.ReadFloat32(activeApp.gameProcessHandle, xAddress)
	if !ok {
		logTooltipDebug("x read failed: xAddr=0x%X", xAddress)
		return tooltipMemorySnapshot{}, false
	}
	y, ok := game.ReadFloat32(activeApp.gameProcessHandle, yAddress)
	if !ok {
		logTooltipDebug("y read failed: yAddr=0x%X", yAddress)
		return tooltipMemorySnapshot{}, false
	}
	width := float32(TooltipOverlayReferenceWidth)
	height := float32(TooltipOverlayReferenceHeight)
	heightSource := "fallback"
	if heightChainOK {
		if value, ok := game.ReadFloat32(activeApp.gameProcessHandle, heightAddress); ok && value >= 60 && value <= 700 {
			height = value
			heightSource = "memory"
		} else {
			logTooltipDebug("height read invalid: heightAddr=0x%X; using fallback=%.2f", heightAddress, height)
		}
	} else {
		logTooltipDebug("height pointer unavailable; using fallback=%.2f", height)
	}
	rawX := x
	rawY := y
	x = -x
	y = -y
	logTooltipDebug("base=0x%X xBase=0x%X yBase=0x%X heightBase=0x%X | xAddr=0x%X yAddr=0x%X heightAddr=0x%X heightSource=%s | raw x=%.2f y=%.2f normalized x=%.2f y=%.2f w=%.2f h=%.2f", activeApp.gameAssemblyBase, xBase, yBase, heightBase, xAddress, yAddress, heightAddress, heightSource, rawX, rawY, x, y, width, height)
	if width < 150 || width > 650 || height < 60 || height > 700 {
		logTooltipDebug("values rejected by size range: x=%.2f y=%.2f w=%.2f h=%.2f", x, y, width, height)
		return tooltipMemorySnapshot{}, false
	}

	clientOrigin, ok := gameClientScreenOrigin()
	if !ok {
		logTooltipDebug("game client origin not found: pid=%d hwnd=0x%X", activeApp.gameProcessID, activeApp.gameWindowHWND)
		return tooltipMemorySnapshot{}, false
	}

	localLeft := int32(x + 0.5)
	localTop := int32(y + 0.5)
	left := clientOrigin.X + localLeft
	top := clientOrigin.Y + localTop
	right := clientOrigin.X + int32(x+width+0.5)
	bottom := clientOrigin.Y + int32(y+height+0.5)
	if right <= left || bottom <= top {
		logTooltipDebug("rect rejected: rect=(%d,%d,%d,%d) values x=%.2f y=%.2f w=%.2f h=%.2f", left, top, right, bottom, x, y, width, height)
		return tooltipMemorySnapshot{}, false
	}

	screen := virtualScreenRect()
	if right < screen.Left || left > screen.Right || bottom < screen.Top || top > screen.Bottom {
		logTooltipDebug("rect outside screen: rect=(%d,%d,%d,%d) screen=(%d,%d,%d,%d)", left, top, right, bottom, screen.Left, screen.Top, screen.Right, screen.Bottom)
		return tooltipMemorySnapshot{}, false
	}
	logTooltipDebug("tooltip local=(%d,%d,%d,%d) clientOrigin=(%d,%d) screenRect=(%d,%d,%d,%d)", localLeft, localTop, int32(x+width+0.5), int32(y+height+0.5), clientOrigin.X, clientOrigin.Y, left, top, right, bottom)

	return tooltipMemorySnapshot{
		Rect: win32.RECT{
			Left:   left,
			Top:    top,
			Right:  right,
			Bottom: bottom,
		},
		MemoryX: rawX,
		MemoryY: rawY,
	}, true
}

func logCurrentTooltipCalibration() {
	snapshot, ok := readTooltipSnapshotFromMemory()
	if !ok {
		logPrintln("[TOOLTIP_CALIBRATION] coordinates could not be read")
		return
	}

	width := snapshot.Rect.Right - snapshot.Rect.Left
	height := snapshot.Rect.Bottom - snapshot.Rect.Top
	scale := currentGameScaleFactor()

	logPrintf(
		"[TOOLTIP_CALIBRATION] scale=%.2f raw=(%.3f, %.3f) rect=(%d,%d,%d,%d) size=(%d,%d)\n",
		scale,
		snapshot.MemoryX,
		snapshot.MemoryY,
		snapshot.Rect.Left,
		snapshot.Rect.Top,
		snapshot.Rect.Right,
		snapshot.Rect.Bottom,
		width,
		height,
	)
	logPrintf(
		"[TOOLTIP_CALIBRATION_ANCHOR] {\"x\": %.3f, \"offset\": 0} | scale_percent=%d raw_y=%.3f\n",
		snapshot.MemoryX,
		currentGameScale(),
		snapshot.MemoryY,
	)
}

func logTooltipDebug(format string, args ...interface{}) {
	now := time.Now()
	if now.Sub(activeApp.lastTooltipDebugLog) < 5*time.Second {
		return
	}
	activeApp.lastTooltipDebugLog = now
	fmt.Printf("[TOOLTIP] "+format+"\n", args...)
}

func logTooltipDebugLines(lines ...string) {
	now := time.Now()
	if now.Sub(activeApp.lastTooltipDebugLog) < 5*time.Second {
		return
	}
	activeApp.lastTooltipDebugLog = now
	for _, line := range lines {
		fmt.Printf("[TOOLTIP] %s\n", line)
	}
}
