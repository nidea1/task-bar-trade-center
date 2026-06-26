package app

import (
	"github.com/nidea1/task-bar-trade-center/internal/catalog"
	"github.com/nidea1/task-bar-trade-center/internal/game"
	"github.com/nidea1/task-bar-trade-center/internal/market"
	"github.com/nidea1/task-bar-trade-center/internal/overlay"
	"github.com/nidea1/task-bar-trade-center/internal/win32"
)

type RECT = win32.RECT
type POINT = win32.POINT
type PAINTSTRUCT = win32.PAINTSTRUCT
type WNDCLASSEX = win32.WNDCLASSEX
type MSG = win32.MSG
type MSLLHOOKSTRUCT = win32.MSLLHOOKSTRUCT
type PROCESSENTRY32W = win32.PROCESSENTRY32W
type GUID = win32.GUID
type NOTIFYICONDATAW = win32.NOTIFYICONDATAW

type ItemConfig = catalog.ItemConfig
type MarketData = market.MarketData
type MarketOrderBook = market.MarketOrderBook
type MarketSalePoint = market.MarketSalePoint
type MarketAnalysis = market.MarketAnalysis
type MarketCurrency = market.MarketCurrency
type MarketRegion = market.MarketRegion
type MarketScope = market.MarketScope

type PriceOverlayView = overlay.PriceView
type GameLayout = game.GameLayout
type OverlayPlacementCalibration = overlay.PlacementCalibration
type OverlayXCalibration = overlay.XCalibration

type WindowSearchState = game.WindowSearchState
