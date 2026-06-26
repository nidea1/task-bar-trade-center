package app

import (
	"fmt"
	"time"

	"github.com/nidea1/task-bar-trade-center/internal/game"
)

func readTooltipRectFromMemory() (RECT, bool) {
	if GameProcessHandle == 0 || GameAssemblyBase == 0 {
		logTooltipDebug("handle/base missing: handle=0x%X gameAssembly=0x%X", GameProcessHandle, GameAssemblyBase)
		return RECT{}, false
	}

	GameLayoutMu.RLock()
	layout := ActiveGameLayout
	GameLayoutMu.RUnlock()
	xBase := GameAssemblyBase + layout.TooltipXPointerBaseOffset
	yBase := GameAssemblyBase + layout.TooltipYPointerBaseOffset
	heightBase := GameAssemblyBase + layout.TooltipHeightPointerBaseOffset

	xAddress, xChainOK, xTrace := TooltipXAOBResolver.Resolve("x", GameProcessHandle, GameAssemblyBase, layout.TooltipXPointerBaseAOB, layout.TooltipXPointerOffsets)
	yAddress, yChainOK, yTrace := TooltipYAOBResolver.Resolve("y", GameProcessHandle, GameAssemblyBase, layout.TooltipYPointerBaseAOB, layout.TooltipYPointerOffsets)
	heightAddress, heightChainOK, heightTrace := TooltipHeightAOBResolver.Resolve("height", GameProcessHandle, GameAssemblyBase, layout.TooltipHeightPointerBaseAOB, layout.TooltipHeightPointerOffsets)
	if !xChainOK || !yChainOK {
		logTooltipDebugLines(
			"pointer chain status:",
			xTrace,
			yTrace,
			heightTrace,
		)
		return RECT{}, false
	}

	x, ok := game.ReadFloat32(GameProcessHandle, xAddress)
	if !ok {
		logTooltipDebug("x read failed: xAddr=0x%X", xAddress)
		return RECT{}, false
	}
	y, ok := game.ReadFloat32(GameProcessHandle, yAddress)
	if !ok {
		logTooltipDebug("y read failed: yAddr=0x%X", yAddress)
		return RECT{}, false
	}
	width := float32(TooltipOverlayReferenceWidth)
	height := float32(TooltipOverlayReferenceHeight)
	heightSource := "fallback"
	if heightChainOK {
		if value, ok := game.ReadFloat32(GameProcessHandle, heightAddress); ok && value >= 60 && value <= 700 {
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
	logTooltipDebug("base=0x%X xBase=0x%X yBase=0x%X heightBase=0x%X | xAddr=0x%X yAddr=0x%X heightAddr=0x%X heightSource=%s | raw x=%.2f y=%.2f normalized x=%.2f y=%.2f w=%.2f h=%.2f", GameAssemblyBase, xBase, yBase, heightBase, xAddress, yAddress, heightAddress, heightSource, rawX, rawY, x, y, width, height)
	if width < 150 || width > 650 || height < 60 || height > 700 {
		logTooltipDebug("values rejected by size range: x=%.2f y=%.2f w=%.2f h=%.2f", x, y, width, height)
		return RECT{}, false
	}

	clientOrigin, ok := gameClientScreenOrigin()
	if !ok {
		logTooltipDebug("game client origin not found: pid=%d hwnd=0x%X", GameProcessID, GameWindowHWND)
		return RECT{}, false
	}

	localLeft := int32(x + 0.5)
	localTop := int32(y + 0.5)
	left := clientOrigin.X + localLeft
	top := clientOrigin.Y + localTop
	right := clientOrigin.X + int32(x+width+0.5)
	bottom := clientOrigin.Y + int32(y+height+0.5)
	if right <= left || bottom <= top {
		logTooltipDebug("rect rejected: rect=(%d,%d,%d,%d) values x=%.2f y=%.2f w=%.2f h=%.2f", left, top, right, bottom, x, y, width, height)
		return RECT{}, false
	}

	screen := virtualScreenRect()
	if right < screen.Left || left > screen.Right || bottom < screen.Top || top > screen.Bottom {
		logTooltipDebug("rect outside screen: rect=(%d,%d,%d,%d) screen=(%d,%d,%d,%d)", left, top, right, bottom, screen.Left, screen.Top, screen.Right, screen.Bottom)
		return RECT{}, false
	}
	logTooltipDebug("tooltip local=(%d,%d,%d,%d) clientOrigin=(%d,%d) screenRect=(%d,%d,%d,%d)", localLeft, localTop, int32(x+width+0.5), int32(y+height+0.5), clientOrigin.X, clientOrigin.Y, left, top, right, bottom)

	return RECT{
		Left:   left,
		Top:    top,
		Right:  right,
		Bottom: bottom,
	}, true
}

func logTooltipDebug(format string, args ...interface{}) {
	now := time.Now()
	if now.Sub(LastTooltipDebugLog) < time.Second {
		return
	}
	LastTooltipDebugLog = now
	fmt.Printf("[TOOLTIP] "+format+"\n", args...)
}

func logTooltipDebugLines(lines ...string) {
	now := time.Now()
	if now.Sub(LastTooltipDebugLog) < time.Second {
		return
	}
	LastTooltipDebugLog = now
	for _, line := range lines {
		fmt.Printf("[TOOLTIP] %s\n", line)
	}
}
