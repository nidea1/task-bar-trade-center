# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.9.3] - 2026-06-27

### Fixed
- Fixed typescript binding destination directory by changing `wailsjsdir` in `wails.json` to target `./frontend/wailsjs` directly instead of its parent directory.

## [0.9.2] - 2026-06-27

### Fixed
- Fixed Go build embed issues in CI/CD pipeline by adding a step that creates a dummy `frontend/dist` directory before executing `wails generate bindings`.

## [0.9.1] - 2026-06-27

### Fixed
- Fixed typescript frontend compilation in CI/CD pipeline by configuring `wailsjsdir` in `wails.json` to generate Wails bindings in the correct directory (`./frontend/wailsjs`).

## [0.9.0] - 2026-06-27

### Added
- Migrated the application to the Wails framework, replacing the legacy window overlay/rendering system with a React, Vite, and TypeScript frontend.
- Created a game-themed dashboard UI (located in `frontend/`) featuring RPG aesthetic design, custom fonts, animations, and layouts.
- Added new dashboard metrics: Stash/Inventory Value, Hero Gear Value, and Total Value.
- Implemented game-themed custom selectors (`GameDropdown`) for Language and Region/Currency selection.
- Static integration of 6 animated pixel-art hero gifs (`Hero_101.gif` to `Hero_601.gif`) corresponding to active character classes.
- Added localization support for all 18 languages across the entire React dashboard, including metrics, item locations, buttons, relative times, and notifications.
- Added a localization helper supporting comma-separated locations splitting and locale-aware capitalization (correctly mapping Turkish dotted/dotless `I`/`İ` rules).
- Implemented backend APIs for preferences retrieval and state propagation: `GetDisplayLanguages`, `GetMarketCurrencies`, `GetMarketRegions`, `GetCurrentLanguage`, `GetCurrentMarketScope`, `SetDisplayLanguage`, `SetMarketScope`, and `GetTranslations`.
- Added localized strings for `location.stash`, `location.inventory`, and `location.equipped` across all 18 JSON translation catalog files.
- Added `SlotIndex` to `OwnedItem` and `StashPageCount` calculation to `InventorySnapshot` (dividing slot counts by 100 slots per page).
- Enabled fetching of `IconURL` from Steam listing body (`ParseIconURL`) and storing it in `Analysis.IconURL`.
- Added stash page item counts to inventory dashboard totals and the React dashboard stash page summary.
- Added a missing-prices side panel next to the all-marketable-items dashboard view.
- Added regression coverage for market-scope inventory repricing, stash page counts, refresh queue cleanup, and USD fallback currency formatting.
- Added dashboard loading skeletons and a shell dashboard state so the Wails UI can open before inventory memory is readable.
- Added tray notifications for newly acquired marketable inventory items, including localized item details and price-refresh fallback text.
- Added a Wails dashboard window icon helper that applies the app icon to the custom dashboard window class.
- Added regression coverage for dashboard shell responses, marketable item notifications, sparse stash slot indexes, stale equipped references, and PlayerSaveData candidate selection.
- Added a "Best to Sell Now" dashboard panel displaying recommendations based on confidence, sales volume, buy orders, spread size, and weekly averages.
- Added a scoring algorithm for marketable items (`sellNowScore`) that rates selling conditions out of 100 with localized reasons (e.g. narrow spread, high sales volume, above average).
- Added Steam community market icon caching and `.ico` conversion (`pngToICO`), enabling Windows balloon notifications to display item-specific icons instead of generic icons.
- Added async price resolution and retry loop (`processNewMarketableInventoryItems`) when a new marketable item is acquired, automatically retrying item price resolution in the background for up to 15 seconds before notifying.
- Added unit tests for async price resolution and polling behavior under `internal/app/inventory_notifications_test.go`.
- Added new screenshots demonstrating dashboard views: `assets/dashboard-all.png`, `assets/dashboard-bits.png`, and `assets/dashboard-syncing.png`.

### Changed
- Structured the codebase into internal packages inside `internal/` (such as `internal/app`, `internal/catalog`, `internal/game`, `internal/il2cpp`, `internal/inventory`, `internal/localization`, `internal/market`, `internal/playerdata`, `internal/win32`, `internal/winapp`, `internal/updater`, `internal/tbhmem`, `internal/overlay`).
- Consolidated global state and Wails backend bindings inside a stateful `App` struct.
- Rebuilt inventory dashboard caching logic, utilizing `activeApp.lastSnapshot` to cache active inventory snapshot data.
- Refactored `RefreshQueue` fields `backoffUntil`, `lastStartedAt`, and `lastFinishedAt` to use `time.Time` and serialize them as RFC3339 strings in Wails JS bindings.
- Enabled loading application icon from disk (`assets/icon.ico`) as a fallback in development for the system tray menu.
- Hooked settings adjustments from both the tray menu and the frontend selectors to trigger `rebuildDashboardState`.
- Cleaned up obsolete package types and redundant type alias wrappers from Phase 1 of migration.
- Updated grid column breakpoints in the items panel to `md:grid-cols-2` to support dual-column layouts at the minimum dashboard width of 960px.
- Replaced the dashboard header logo and drag-bar icons with the application icon asset.
- Reworked inventory dashboard polling to reuse fresh cached state, guard concurrent dashboard builds, and queue repricing after market scope changes.
- Throttled Steam Market HTTP requests and shortened the inventory price refresh queue base delay for responsive repricing with less rate-limit pressure.
- Reworked the item dashboard layout around the filtered all-items grid, compact controls, stable dropdown labels, and better overflow/tooltips.
- Configured Vite and Tailwind development scanning to ignore generated, backend, build, and `ext-project-refs` paths.
- Changed the system display-language preference sentinel from `system` to `sys`.
- Changed inventory dashboard reads to return cached or shell state immediately while refreshing snapshots in the background.
- Added periodic inventory dashboard monitoring while TaskBarHero is attached.
- Improved PlayerSaveData resolution by preferring higher-gold candidates, briefly reusing fresh resolved objects, and reading sparse save-slot indexes when available.
- Updated dashboard polling cadence and icon sizing/branding styles.
- Modularized React frontend by refactoring `App.tsx` and extracting separate components, constants, types, and utility files into `frontend/src/components/`, `types/`, `utils/`, and `constants/`.
- Reworked background inventory checking to only poll and process notifications via `pollInventoryNotifications()`, preventing heavy dashboard recalculation/rebuilding cycles during background monitoring.
- Changed single-item tray notifications to exclude the item name in the body (since it is already in the title).
- Changed tray notifications for multiple items to display the small application icon (`activeApp.appIconSmall`) as the tray icon.
- Made the dashboard update callback execution asynchronous (`go fn(state)`) in `callbacks.go` to prevent blockages on caller threads.

### Removed
- Removed the obsolete Windows-only `tools/playerdiag` diagnostic utility.

### Fixed
- Fixed select dropdown rendering to prevent the layout panel height from expanding vertically when menus are opened.
- Resolved Turkish locale lowercase/uppercase mapping issues by dynamically setting the document element lang attribute and using `toLocaleUpperCase(currentLanguage)`.
- Fixed unused sync import compile error in `inventory_integration.go`.
- Pre-initialized activeApp to prevent nil-pointer dereferences in tests.
- Fixed mixed currency prefix/suffix output in parsed and USD-fallback market analysis/order book data.
- Fixed empty stash pages so pages with zero items report zero value instead of carrying stale page totals.
- Fixed refresh queue pending storage being retained after queued work drains.
- Fixed dashboard async loads from updating React state after unmount or while another load is already in flight.
- Fixed Turkish hatchet labeling from `Nacak` to `Balta`.
- Fixed dashboard API calls failing before the game process is attached by returning localized runtime fields with an empty shell state.
- Fixed stale hero-equipped references overriding current inventory/stash slot locations.
- Fixed stash page value tests and slot mapping for sparse pages beyond the compressed slot list.

## [0.8.1] - 2026-06-25

### Fixed
- Fixed Steam Market SSR order book parsing so cached order book and overlay analysis prices keep the selected currency prefix/suffix instead of defaulting to `$` for non-USD markets.

## [0.8.0] - 2026-06-24

### Added
- Added order quantities to the highest buy and lowest sell price indicators on the overlay HUD when they exceed 1 (displayed as `Price (Quantity)`).
- Documented step-by-step download, installation, and troubleshooting instructions in the README.

### Changed
- Reduced market order cache TTL (`marketOrderCacheTTL`) from 30 minutes to 5 minutes to provide more up-to-date pricing.
- Extracted highest buy/lowest sell order quantities from both Steam Market SSR scripts and JSON API histograms.

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
