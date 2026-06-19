# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
