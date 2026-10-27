# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.14.3] - 2026-10-28

### Added
- Integrated Steam client SDK adapter (`steamdefs`) to query item definition IDs, verified market names, marketability flags, and identifying metadata tags directly via `ISteamInventory::GetItemDefinitionProperty` during catalog refresh.
- Linked game item keys to Steam market definition IDs using the native `ItemKey * 1000 + 900` definition convention.
- Added strict response verification (`verifiedListingSources`, `verifiedSearchSources`) checking matching AppID (3678970), canonical `market_hash_name`, and item tags on Steam Community Market SSR listing and search data.

### Changed
- Replaced the temporary level-based equipment variant guessing rule with verified Steam market identity resolution. Equipment sharing identical levels (such as level-80 `A` and `C` variants) are now resolved distinctly from Steam's definition inventory.
- Bound the price cache and market analysis model to Steam definition IDs. Items without a verified identity or with conflicting definitions are marked as unavailable rather than falling back to guessed names or stale cross-variant prices.
- Bumped installed catalog cache format to version 3, automatically invalidating legacy guessed-variant entries.

### Fixed
- Fixed cross-variant price bleed where prices from an obsolete variant could display for a different variant of the same gear.
- Added immediate overlay HUD invalidation when catalog definitions update or when an identity mismatch occurs during fetch.

### Tests
- Added regression tests for Steam SDK definition extraction, catalog identity binding, old-variant overlay rejection, legacy cache invalidation, and mismatch error handling.

## [0.14.2] - 2026-10-27

### Fixed
- Fixed inflated prices for level-90 and higher equipment, including Eternal Armor (Beyond), caused by querying obsolete Steam Market `A` listings and their historical sales instead of the current `C` listings.
- Kept equipment below level 90 on Steam's `A` variant. Price requests, market links, and price-cache lookups now use the equipment's level to select the variant.
- Preserved item levels when refreshing the installed-game catalog and rebuilt older catalog caches that omitted this information.

### Temporary workaround
- Prices come directly from Steam Community Market. This patch follows the currently observed naming rule: level 90 and above uses `C`, and lower-level equipment uses `A`. Future changes to Steam's variant naming will require another update.

### Tests
- Added regression coverage for the level-90 boundary, correct market links, old-variant price-cache isolation, and catalog cache rebuilding with installed-game data.

### Acknowledgements
- Thank you to the user who reported the Eternal Armor (Beyond) price discrepancy and helped uncover the incorrect Steam listing lookup.

## [0.14.1] - 2026-10-23

### Fixed
- Corrected stash page calculation to 49 slots per page (7x7 grid) instead of 100, ensuring items on stash pages 2 and 3 are mapped to their correct pages.
- Resolved active unlocked stash page count directly from `TaskbarHero.System.Stash` static fields (`bjas`), displaying only unlocked stash pages (e.g. 3 pages) instead of pre-allocated list capacity.
- Only display the Material Stash row in the Stash Value tooltip when material stash is unlocked and contains items.
- Prevented redundant IL2CPP class discovery memory scans on warm snapshots when optional classes are absent.

### Tests
- Added test coverage for 49-slot stash page breakdowns and material stash visibility.

## [0.14.0] - 2026-10-23

### Added
- Added automatic background catalog updates from the installed game's item and language assets. Validated updates are cached locally, with the last good catalog retained if a refresh fails; Python and REA are not required at runtime.
- Updated the embedded catalog to 7,031 items, including 1,087 new records and 182 new marketable items.
- Added live chest-opening countdowns and waiting chest counts to the Inventory Fill tooltip, while retaining the Auto Open label and estimated intervals when live observations are unavailable.
- Added a configurable inventory-fill warning to Dashboard Settings, with a five-minute default, an off option, and repeated-alert suppression.
- Added a Stash Value tooltip showing each page's value and occupied slots, empty pages, the material-stash subtotal, and the combined total.
- Added stack quantity and material-stash support to portfolio values and owned-item quantity tracking.

### Changed
- Batched IL2CPP class and object discovery and reused scan buffers to reduce startup work and temporary memory allocation.
- Reused validated live trade-slot objects between reads instead of scanning the whole process on every inventory poll. Destroyed or missing objects trigger bounded rediscovery; locked backing slots no longer cause unnecessary scans.
- Local measurements reduced cold inventory resolution from 15.27 seconds to 4.17–4.48 seconds and subsequent reads from 685–696 milliseconds to approximately 17–27 milliseconds. These are inventory timings on one machine, not total application startup guarantees.

### Fixed
- Corrected exact Steam listing names for Plague Fruit, Contaminated Reliquary, and Contaminated Relic variants, while preserving verified listing names across catalog updates.
- Recognized the current normal chest auto-open unlock runes separately from reduction runes and included the new PlagueLands reduction runes in region-specific forecasts.
- Used the game's actual remaining opening time, automatic-opening toggle, and waiting chest count when calculating inventory fill time; stale timer observations fall back to estimates.
- Updated trade-slot reads for the current named runtime fields and prevented current save-data item IDs from being interpreted as legacy cooldown timestamps.
- Preserved catalog status and quantity-target progress when market price updates rebuild dashboard data.

### Tests
- Added coverage for installed-game catalog extraction, unchanged-source caching, corrupt-cache rejection, and retention of the last good catalog after refresh failures.
- Added regression coverage for live chest timing, rune unlocks and reductions, quantity-aware valuation, stash-page totals, warning thresholds, and persisted notification settings.
- Added memory-scanner coverage for chunk boundaries, overlapping matches, unreadable gaps, result limits, batched class discovery, stale class pointers, live cooldown updates, and destroyed-object recovery.

## [0.13.0] - 2026-10-22

### Added
- Added automatic region detection for chest forecasts, including PlagueLands Nightmare, Hell, and Torment stages.
- Added separate PlagueLands chest auto-open times and reductions so normal chest runes are not applied to the new region's chests.

### Changed
- Chest forecasts now use 3/5 minute drops in normal regions and 4/6 minute drops in PlagueLands. Changing regions updates the chest labels and restarts the drop countdowns.
- Tooltip placement now follows the game's actual tooltip bounds and scale automatically. Removed manual scale selectors while retaining existing settings compatibility.
- Show chest drop intervals and countdowns even when auto-open runes are missing.

### Fixed
- Restored hovered-item detection and tooltip tracking after the TaskBarHero 1.3.0 update.
- Fixed item-dependent overlay drift by reading the visible tooltip's native transform hierarchy. The overlay attaches directly below the tooltip, or above it when there is insufficient room on the current monitor.
- Restored Steam price updates by removing request headers that caused valid listing requests to return HTTP 429, and by reading the current `valve-ssr-data` query cache.
- Applied Steam rate-limit cooldowns to all price requests. Pending inventory updates resume automatically afterward, including items added while paused.

### Tests
- Added regression coverage for tooltip bounds and scaling, monitor edges, region changes across all 60 PlagueLands stages, separate chest automation, current Steam listing responses, and shared cooldown recovery.

## [0.12.8] - 2026-07-21

### Added
- Added explicit Steam 24-hour median and volume fields to the detailed price overlay and inventory dashboard, with labels in all 18 supported languages.
- Added estimated-price indicators for USD market data converted into the selected local currency.

### Changed
- Replaced the Steam `priceoverview`, `pricehistory`, `itemordershistogram`, and `search/render` request chain with the listing page's React SSR order book and price history data.
- Added an exact-name and AppID-validated Steam Market search-page fallback that supplies only the lowest sell price when listing SSR data is unavailable.
- Requested the complete uncompressed Steam listing response because Steam's compressed cache variant can return a successful page shell without SSR market data.
- Derived Steam median and volume from the listing's actual last-24-hour sales history instead of treating `priceoverview` median and volume as weekly average and daily sales metrics.
- Accepted USD listing SSR for non-USD scopes, converted it through the existing exchange-rate cache, and marked the resulting local price as estimated.
- Migrated legacy cached weekly-average and daily-sales values into the corrected Steam median and volume fields when no real sale history is available.

### Fixed
- Fixed repeated Steam Market HTTP 429 failures preventing prices from loading immediately after launch or after long retry intervals ([#8](https://github.com/nidea1/task-bar-trade-center/issues/8), [#9](https://github.com/nidea1/task-bar-trade-center/issues/9)).
- Fixed successful Steam HTTP 200 responses being rejected when Go's automatic gzip request selected a data-less cached page variant.
- Fixed Steam median and volume incorrectly affecting weekly average, daily sales, sell-through, confidence, and sell-score calculations.
- Fixed TBTC retaining an exited TaskBarHero process handle, which prevented automatic attachment to a relaunched game and caused `PlayerSaveData could not be resolved` refresh errors.

### Tests
- Added regression coverage for React `loaderData` parsing, SSR currency conversion, exact search fallback matching, legacy endpoint avoidance, cache migration, localized labels, overlay dimensions, and exited-process reattachment.
- Verified the listing flow against live Steam Market data and added transport coverage that prevents compressed shell-only responses.

### Acknowledgements
- Thanks to [@ctvjxrf143](https://github.com/ctvjxrf143) and [@shaiyumg](https://github.com/shaiyumg) for reporting the Steam Market 429 failures and providing diagnostic logs.

## [0.12.7] - 2026-07-10

### Changed
- Changed dynamic `PlayerSaveData` layout discovery to identify item, currency, hero, inventory, stash, and trade lists by their runtime IL2CPP element classes instead of relying only on data-shape heuristics.
- Bounded nested field discovery by each IL2CPP class instance size so adjacent objects cannot be mistaken for fields when layouts move.
- Preserved the fixed layout as a fast path while validating its runtime list classes before accepting it.

### Fixed
- Fixed false inventory layouts caused by larger lookalike lists such as `AttributeSaveData` being selected instead of `ItemSaveData` after game updates.
- Fixed dynamically discovered inventory slot fields being overwritten by legacy fixed offsets.
- Invalidated resolver caches created before class-aware layout validation.

### Tests
- Added regression coverage for class-aware list selection, misleading lookalike data, shifted nested fields, and instance-size-bounded field scanning.

## [0.12.6] - 2026-07-10

### Fixed
- Fixed dashboard inventory detection after a TaskBarHero update shifted the `PlayerSaveData` list offsets and the `HeroSaveData` equipped-items offset.
- Invalidated stale inventory resolver caches so layouts discovered against the previous game build are not reused.

### Tests
- Added regression coverage for the updated July 2026 save-data layout.

## [0.12.5] - 2026-07-06

### Changed
- Reworked the Best Items to Sell Now liquidity signal to score sell-through speed from the lowest-price sell queue divided by daily sales volume instead of raw daily sales thresholds.
- Replaced newly generated `high_daily_sales` sell reasons with `healthy_sell_through` and added a Sell-through metric to the Best Items cards.

### Fixed
- Fixed high daily sales tagging for heavily backlogged items, such as items with many low-price listings that would still take weeks to cycle through.

### Tests
- Added dashboard regression coverage for fast sell-through, long-queue Sapphire-style listings, and missing lowest-price queue data.

## [0.12.2] - 2026-07-04

### Added
- Added persistent caching of absolute memory pointers for inventory resolver results (`PlayerSaveData` layout and class addresses), resulting in instant (<1ms) startup times when the game remains active.
- Added automatic validation of cached pointers, with graceful fallback to a full memory scan and cache regeneration upon game process restarts or cache errors.

### Changed
- Changed "Box" notify source label to "Chest" in all 18 supported languages.

## [0.12.1] - 2026-07-04

### Fixed
- Fixed dashboard language and market scope not updating dynamically when modified from the system tray menu.

### Added
- Added inventory forecast screenshot to README.md.

## [0.12.0] - 2026-07-04

### Added
- Added chest automation rune parsing to detect "Rune of the Mainspring" levels and calculate chest auto-open rate reductions.
- Added live chest counts (Normal, Stage, and Total chests) read from game serialization memory (`PlayerSaveData.serializedBoxData`).
- Added inventory slot summary (total, used, and free slots) with fallback counting.
- Added inventory fill forecast dashboard metric that estimates the time to fill inventory, net slots per hour, and next chest drop times.
- Added localized translations for all new chest, rune, and fill forecast dashboard strings in all 18 supported languages.
- Added interactive update checking to the dashboard footer with active spinner animations during checks.

### Changed
- Reworked dashboard metrics layout to fit the new inventory fill forecast metric.
- Updated footer layout with new styles for light mode and active button hover states.

## [0.11.8] - 2026-07-03

### Fixed

* Fixed the overlay, tray menu, and dashboard becoming unreliable after long-running sessions by recovering stale overlay windows instead of leaving overlay update state stuck.
* Fixed an overlay window destroy event from terminating the shared Win32 message loop used by tray and hotkey handling.
* Fixed repeated dashboard icon application from loading new native icon handles on every dashboard show.

### Tests

* Added regression coverage to ensure overlay window destruction only clears overlay state.

## [0.11.7] - 2026-07-02

### Fixed

* Fixed dashboard settings appearing reset after application updates or first startup by waiting for the settings file to load before serving settings-related APIs.
* Fixed dashboard settings writes to use an atomic write path so interrupted saves do not leave a partially written `settings.json`.
* Fixed equipped item value totals being assigned to the wrong hero when game updates expose a shared legacy hero-key field before the actual hero class key.

### Tests

* Added regression coverage for dashboard settings API calls racing the settings load during startup.
* Added regression coverage for equipped items resolving to distinct hero keys when the old hero-key offset contains the same value for multiple heroes.

## [0.11.6] - 2026-07-02

### Changed

* Changed inventory snapshot resolution to keep the existing fixed `PlayerSaveData` layout as the fast path, then fall back to runtime layout discovery when game updates shift save-data field offsets.

### Fixed

* Fixed inventory snapshots failing with `PlayerSaveData could not be resolved` after TaskBarHero updates changed save-data object offsets.
* Fixed future save-data offset shifts from requiring a code update when item, slot, hero, currency, or trade-slot lists can still be identified from their runtime data shape.

### Tests

* Added regression coverage for shifted `PlayerSaveData` layouts, including moved item IDs, unique IDs, stash/inventory slots, equipped items, and gold fields.

## [0.11.5] - 2026-07-01

### Added

* Added an inventory resolver cache that stores resolved `GameAssembly.dll`-based class addresses and automatically invalidates them when the module path, size, or modification time changes.
* Added multi-pattern memory scanning for supported inventory resolver searches to avoid rescanning the same memory regions for each pattern.
* Added detailed `pricehistory` diagnostic logging with the minimal request URL, application ID, market hash name, HTTP status, and a response body limited to 1 KB.
* Added regression coverage for scoped market pricing, price history conversion, USD detail fallback, display-language migration, inventory resolver caching, and market endpoint fallback behavior.

### Changed

* Changed Steam `pricehistory` requests to use only `appid` and `market_hash_name`, without country or currency parameters.
* Reordered market data requests to prefer scoped listing and order book data, followed by scoped `priceoverview`, before using `pricehistory` as a final signal source.
* Changed market fallback behavior so valid local-scope prices can be displayed without waiting for a synchronous USD fallback.
* Changed USD fallback merging to preserve valid local prices while converting and filling only missing order book and history details.
* Normalized base price history values into the selected currency when a valid exchange rate is available.
* Preserved price history volume and trend signals while excluding unconverted history prices from suggested-price calculations when an exchange rate is unavailable.
* Changed initial inventory resolution to wait for up to 60 seconds before opening the dashboard, then continue resolving and publish the snapshot in the background.
* Reduced unnecessary overlay window repositioning, repainting, and tooltip-height logging when only minor rectangle changes are detected.
* Changed system-language preference handling to persist the resolved language code instead of storing `sys`.
* Improved frontend settings hydration to avoid overwriting persisted dashboard preferences during startup or application updates.

### Fixed

* Fixed Steam `pricehistory` HTTP 400 and other non-success responses blocking otherwise valid local market prices.
* Fixed unnecessary `pricehistory` requests when scoped listing, order book, or `priceoverview` data already provides a usable price.
* Fixed incomplete UK/GBP market analysis by merging missing USD market details after converting them into the selected currency.
* Fixed unconverted base-history prices affecting suggested-price calculations when no exchange rate is available.
* Fixed dashboard preferences being reset during frontend hydration, builds, or application updates.
* Fixed application startup remaining blocked indefinitely while the first inventory snapshot was being resolved.
* Fixed legacy settings containing `display_language: "sys"` by migrating them to the currently resolved display language.
* Fixed excessive overlay repositioning, repainting, and repetitive tooltip-height logging during small tooltip rectangle changes.

### Acknowledgements

* Special thanks to `raigoo_` for repeatedly testing development builds and providing detailed feedback that helped identify, reproduce, and verify several of these fixes.

## [0.11.4] - 2026-06-29

### Added
- Added market tooltip latency metrics that log tooltip cache state, in-flight fetch joins, Steam request limiter wait time, endpoint request duration, status, retry-after, and errors.

### Changed
- Optimized tooltip market price loading by showing stale cached prices immediately while refreshing in the background.
- Debounced hover-triggered Steam market fetches by 500 ms to avoid fetching prices for items the user only passes over briefly.
- Deduplicated in-flight market fetches for the same market scope and item so overlapping tooltip, inventory, and refresh requests share one Steam request chain.
- Prioritized active tooltip market requests over background inventory price refreshes.

## [0.11.3] - 2026-06-29

### Added
- Added an in-dashboard update prompt with "Update Now" and "Update Later" actions when a new application version is available.
- Localized the new dashboard update prompt across all 18 supported languages.

### Changed
- Updated Trade Ship cooldown detection to prefer live `TradingStashSlot` timestamps over stale save-slot timestamps, fixing early voyage-complete notifications.
- Opening an available update now brings the dashboard forward instead of relying only on the tray install action.

### Fixed
- Suppressed marketable-item notifications from cube decoration, engraving, inscription, and extraction logs.
- Suppressed stale Trade Ship completion notifications when an already-expired cooldown is first observed.
- Avoided briefly showing the "Waiting for TaskBarHero" status when the user chooses to close TBTC after TaskBarHero exits.

## [0.11.2] - 2026-06-29

### Fixed
- Fixed readability of the dashboard settings popover in light mode by adjusting its background, borders, shadow, dividers, checkbox active/hover states, and source group headers for better contrast.
- Fixed dropdown rarity selection options (unselected and hover states) in light mode to display their respective rarity colors (using a color-mixed darkened variant) instead of being overridden by general text color styles.

## [0.11.1] - 2026-06-28

### Fixed
- Postpone marketable inventory item notifications if price information is stale (older than 5 minutes) or if the item's icon is not yet downloaded, resolving it asynchronously.
- Added a fallback to the application's small icon when a custom item icon is unavailable, ensuring notifications are never displayed without an icon.

## [0.11.0] - 2026-06-28

### Added
- Added Game scale option setting (`Oyun Pencere Ölçeği`) to the settings popover with options: `1x`, `1.25x`, and `1.5x`.
- Localized the game scale setting name across all 18 supported languages.
- Added a customizable hotkey feature to let users reassign the system-wide shortcut key for toggling the dashboard (previously fixed to F2).
- Added a pulsing keyboard capturer UI inside settings to record custom key combos (e.g. `Ctrl + F2`, `Alt + Shift + K`) while ignoring standalone modifiers.
- Added Go Wails bindings `DisableDashboardHotkey` and `EnableDashboardHotkey` using Windows message dispatching (`WM_APP_HOTKEY_DISABLE`/`WM_APP_HOTKEY_ENABLE`) to temporarily unregister active hotkeys during UI keyboard capturing so key presses are not swallowed by the OS.
- Added `ts_type:"string"` tags to `time.Time` fields in Go structs to resolve Wails binding generation warnings.
- Completed all dashboard settings and hotkey control translations across all 18 supported language locale files.
- Added an update-available confirmation dialog so users can install immediately or defer and install later from the tray menu.
- Added localized update confirmation copy across all 18 supported language locale files.
- Added Chile/CLP Steam market currency and region support.
- Added Best Items to Sell Now filters for rarity, item type, equipped/unequipped state, search, and score/price sorting, with persisted dashboard settings and translations for all supported languages.
- Added Steam Trade Ship memory reading to parse active voyage slots and remaining cooldowns.
- Added a background cooldown monitor that sends Windows system tray notifications when a Trade Ship voyage finishes.
- Added Trade Ship notification translations for all 18 supported languages.
- Added detailed Steam API rate limit (`429 Too Many Requests`) logging to the refresh queue, capturing the specific item ID, market hash name, endpoint, status, remaining queue, `Retry-After` header, and backoff timestamp.

### Changed
- Redesigned the dashboard header layout by shifting all configurations (Theme, Language, Currency, Price Mode, Rarity Threshold, and Notification sources) into a clean, unified Settings Popover menu to prevent layout wrapping or overflow at smaller screen scales.
- Refactored the Theme selection and Rarity Notification selection to use consistent dropdown menus (`GameDropdown`) and removed redundant prefix labels inside the dropdown buttons to streamline the visual layout.
- Removed the 12-item cap from the dashboard's Best Items to Sell Now list so valuable equipped items no longer hide other sell candidates.
- Constrained the dashboard sell recommendation and missing-price lists to scroll internally when many items are present.
- Aligned side-by-side dashboard item panels to a shared height so Best Items, All Items, and Missing Prices look balanced while scrolling internally.
- Integrated async notification price fetches with the main refresh queue's backoff state to skip outbound Steam requests when a rate limit is active.
- Updated Steam API client to abort request chains immediately upon encountering a `429` status code.

### Fixed
- Fixed Best Items and All Items filter controls wrapping unevenly at intermediate dashboard widths.
- Suppressed the redundant "Waiting for TaskBarHero" notification when the user chooses to close TBTC from the TaskBarHero-closed prompt.

## [0.10.3] - 2026-06-28

### Added
- Added live item-synthesis (`CubeSynthesisLog`) detection. Read synthesis results directly from memory offset `0xB0` of log entries.
- Added a timing retry buffer for synthesis logs to wait (up to 3 retries / 6 seconds) for server-side response item ID writes.
- Added class filtering to the deep recursive memory searcher, excluding game state classes (Hero, Monster, etc.) and increasing the search budget to 1500 nodes for faster and reliable item ID discovery on payload misses.

### Fixed
- Resolved an issue in log notifications where items in box opens, crafting, and offering logs resolved to base template IDs instead of their graded IDs.
- Fixed a bug where disabling notification types in settings caused logs to stop advancing (`lastLogCount` stuck), leading to backlog notification spam when re-enabled.

## [0.10.2] - 2026-06-27

### Fixed
- Persisted dashboard preferences in the shared settings file so version updates no longer reset theme, price mode, filters, sort order, or the active marketable-items tab.
- Aligned the dashboard smart refresh and force refresh buttons with the dropdown controls so the header row keeps a consistent height.

## [0.10.1] - 2026-06-27

### Fixed
- Normalized the non-English locale files so all supported languages now include the latest refresh, time, and inventory-read strings with localized labels and tooltips.

## [0.10.0] - 2026-06-27

### Added
- Added live chest-open item detection through the game's `LogManager`, resolving `BoxOpenLog` entries and decoding `ItemName_<ItemID>` strings for immediate marketable-item notifications before the save file updates.
- Added event-driven inventory snapshot synchronization: the app now seeds inventory once when TaskBarHero attaches, then syncs after `StageClearLog` or `StageFailedLog` events instead of running periodic background inventory polling.
- Added counter-based notification deduplication between instant `BoxOpenLog` notifications and the later inventory snapshot scan, preventing duplicate alerts when the same chest item is observed through both paths.
- Added a persisted minimum rarity notification filter (`min_rarity_notify`) with backend rarity levels and a dashboard bell dropdown for selecting the minimum marketable item grade to notify.
- Added automatic smart inventory price refresh after stage-end inventory syncs, queueing missing, stale, or stale-icon marketable items through the rate-limited refresh queue.
- Added force-refresh inventory price actions for the dashboard and tray menu, allowing all marketable inventory items to be repriced regardless of cache freshness.
- Added global Steam market icon metadata caching and debounced cache writes so item icons can be reused across market scopes and cache writes are batched.
- Added regression coverage for rarity notification filtering, force refresh queueing, icon metadata reuse, and refresh queue priority behavior.

### Changed
- Reworked dashboard price refresh controls into separate Smart Refresh and Force Refresh actions with localized labels and tooltips.
- Changed the notification rarity selector to an icon-only bell control next to the theme toggle, with rarity-colored dropdown options and improved Common rarity contrast.
- Moved inventory sync timing out of the dashboard header and into the footer as `Last Sync` / `Son Senkronizasyon` time text.
- Changed inventory dashboard reads to return cached dashboard state without starting background snapshot refreshes, preserving the event-driven inventory polling model.
- Changed the inventory price refresh queue to use a 3-second base delay while keeping Steam rate-limit backoff protection.
- Improved notification icon preparation by asynchronously caching `.ico` files and falling back to global icon metadata when price cache entries are missing icon URLs.

### Fixed
- Fixed duplicate marketable-item notifications when instant chest-open detection and the later inventory save snapshot report the same item.
- Fixed dark dropdown readability for Common rarity options by using a lighter neutral rarity color.
- Fixed single-anchor scale calibration profiles being rejected during game layout parsing, allowing scale-specific X and Y offsets to apply before additional X anchors are configured.

## [0.9.7] - 2026-06-27

### Fixed
- Fixed missing and incorrect translations in the Best Items to Sell Now dashboard tab across all supported languages.

## [0.9.6] - 2026-06-27

### Fixed
- Fixed missing application icon in Windows toast/balloon notifications when running in standalone mode by embedding `icon.ico` directly into the binary and dynamically extracting it to the local AppData folder.

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
