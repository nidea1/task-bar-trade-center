# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
