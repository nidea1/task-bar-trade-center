# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.7.1] - 2026-06-24

### Fixed
- Fixed parsing of Steam Market Server-Side Rendered (SSR) order book payloads when the maximum buy order or minimum sell order prices are `null`.

## [0.7.0] - 2026-06-24

### Added
- Upgraded game layout configuration to schema version 3 to support dereferencing of sub-pointers through the introduction of `item_ptr_offset` for hovered items.
- Added `TestScanOffsets` in `game_layout_test.go` to inspect active hovered items and scan nested memory object paths.

### Changed
- Refactored hovered item scanning in `app.go` to delegate entirely to the AOB pointer resolver.
- Refactored tooltip position lookup in `memory.go` to directly invoke AOB resolution.
- Updated `readHoveredItemID` in `memory.go` to resolve double-dereferenced hovered item keys.
- Updated embedded fallback configuration `game-layout.json` to schema version 3.

## [0.6.2] - 2026-06-24

### Changed
- Localized the hovered item name displayed on the overlay HUD according to the selected display language preference, falling back to English (en-US) if the translation is unavailable.

### Fixed
- Fixed update loop by bumping the hardcoded version in constants to match the release tag.

## [0.6.1] - 2026-06-22

### Added
- Completed 100% translation coverage for all 16 other supported languages (German, French, Italian, Spanish, Dutch, Portuguese-PT, Portuguese-BR, Finnish, Japanese, Korean, Simplified Chinese, Hindi, Indonesian, Thai, Vietnamese, Polish).

## [0.6.0] - 2026-06-22

### Added
- Completed 100% of Turkish (`tr-TR`) translations, filling in all missing keys.
- Integrated dynamic localization utilizing Go's `embed` package to bundle JSON translation assets directly inside the compiled executable.

### Changed
- Refactored hardcoded dictionary maps out of Go code into structured JSON files under a new `locales/` directory.

## [0.5.0] - 2026-06-22

### Added
- Implemented USD market data fallback for region/currency selections returning incomplete Steam Market API responses. Missing metrics (such as order books, histories, and volumes) are automatically fetched in USD, converted using exchange rates, and marked as estimated.
- Added live exchange rate tracking fetched from the Frankfurter API (`https://api.frankfurter.dev`) on startup with local hardcoded fallbacks for offline support.
- Added localization for the USD fallback warning on the overlay HUD in both English and Turkish.
- Added support for currency-specific prefixes/suffixes (e.g. `€`, `₱`, `¥`, `₹`, `Rp`, `฿`, `₫`, `zł`, `CDN$`, `A$`) for proper price display.

### Changed
- Appended `country` and `currency` query parameters to Steam Market links opened via hotkeys to ensure users land on their selected market page.
- Refactored `cursorScreenPosition` in `overlay_position.go` as a variable to improve unit test control.

## [0.4.1] - 2026-06-22

### Changed
- Reordered USD market regions to list `Türkiye/MENA` first and `United States` second.
- Standardized the Turkey region name to `Türkiye/MENA` globally across all display language preferences.

## [0.4.0] - 2026-06-22

### Added
- Added Steam Market currency selection for USD, EUR, GBP, PHP, JPY, KRW, CNY, INR, IDR, THB, VND, BRL, PLN, CAD, and AUD.
- Added country-aware market requests, with an EUR country submenu for Germany, France, Italy, Spain, Netherlands, Austria, Belgium, Portugal, Finland, and Ireland.
- Added Turkey (TR) as a USD market region (MENA-USD).
- Listed USD regions (`USD — United States` and `USD — Türkiye/MENA`) directly in the main currency menu instead of a sub-menu.
- Added status transition notifications (`Başlatılıyor...` -> `TaskBarHero bekleniyor` -> `Hazır`).

### Changed
- Separated price cache entries by selected currency and country, migrating existing entries to USD/United States.
- Prevented stale price responses from a previous currency or country selection from updating the overlay.
- Reduced tray status notification count (only action-required events trigger notifications by default).
- Simplified startup tray status notification by removing the redundant "Started —" prefix.
- Cleaned up redundant "USD" suffix (e.g. `"$0.21 USD"` to `"$0.21"`) in overlay HUD when prefix is `$`.

### Fixed
- Fixed tray icon tooltip not opening on hover after application startup.
- Changed tray menu status format to `"Durum: <durum>"` (Status: <status>) across 18 languages.
- Fixed the application shutdown confirmation dialog when TaskBarHero is closed.

## [0.3.6] - 2026-06-20

### Added
- Added AOB-based fallback scanning for hovered-item and tooltip X/Y/height pointer roots when a game update invalidates the configured pointer bases.

### Fixed
- Updated the hovered-item and tooltip pointer bases for the current TaskbarHero build, validating their memory chains against the running game.
- Validated AOB candidates against item IDs and expected tooltip coordinate or height ranges before accepting them.

## [0.3.5] - 2026-06-20

### Fixed
- Updated the tooltip X pointer chain and expanded its placement calibrations using values verified on two computers.
- Kept the price HUD visible with cursor-based placement when tooltip coordinate memory cannot be read; only sustained hovered-item pointer failures now mark the game layout as incompatible.

## [0.3.4] - 2026-06-19

### Fixed
- Removed the tooltip width memory pointer dependency; the HUD now uses the game's fixed tooltip width and selects placement calibrations by height and Y position.

## [0.3.3] - 2026-06-19

### Added
- Added column/X coordinate-based calibration mapping (`x_calibrations` in `game-layout.json`) to dynamically adjust the overlay horizontal position by matching the nearest game X coordinate.

### Changed
- Removed `offset_x` from `placement_calibrations` in the game layout, delegating horizontal offset calculation entirely to the new X-coordinate calibration mapping.

## [0.3.2] - 2026-06-19

### Added
- Added "Update configurations" to the tray menu to dynamically fetch the latest game memory layout from remote using a cache-busting query parameter, resetting the memory layout reading health status immediately.

## [0.3.1] - 2026-06-19

### Fixed
- Auto-update no longer leaves the old process running: shutdown requests from background goroutines now post `WM_CLOSE` to the UI thread instead of calling `PostQuitMessage` on the wrong thread, ensuring the main message loop exits and the single-instance mutex is released before the updated executable starts.

## [0.3.0] - 2026-06-19

### Added
- Development builds can load an explicit local game layout through `TBTC_GAME_LAYOUT_PATH`.

### Fixed
- Automatic updates now wait for the previous TBTC process to exit before starting the updated executable.
- Tooltip placement calibrations now match their tooltip dimensions when the tooltip Y position changes.
- Closing TaskBarHero no longer triggers a game memory layout warning.

### Changed
- Game layout schema v2 reads tooltip X and Y coordinates from independent pointer chains.

## [0.2.0] - 2026-06-19
### Added
- GitHub-hosted game memory layout configuration for pointer chains and tooltip placement calibrations.
- Local validated layout cache with an embedded fallback for offline startup.
- One-time user notification and tray status when the game memory layout can no longer be read.

### Changed
- Game pointer and placement values are now loaded from the versioned layout configuration instead of being hard-coded.

## [0.1.0] - 2026-06-19
### Added
- Initial release of Task Bar Trade Center.
- Market analysis and item searching helper overlay.
- Dynamic game memory scanning and status tracking.
- Automated update checking system.
- Tray icon integration with hotkey support.
