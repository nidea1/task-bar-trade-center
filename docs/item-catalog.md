# Item catalog

`internal/catalog/items.json` is the embedded fallback. The application also refreshes its catalog from the installed game on each connection, without requiring Python or REA on the user's machine.

## Automatic refresh

The game executable supplies the installation directory. The reader extracts the `ItemInfoData` TextAsset from `sharedassets0.assets`, then joins ItemTable shared IDs with the localized Unity string tables. Only bundles referenced by the active Addressables `catalog.bin` are included, so an obsolete bundle left behind after an update is ignored. Serialized asset versions 17–22 and UnityFS stored/LZ4 blocks are supported; other formats produce an explicit refresh failure.

SHA-256 fingerprints cover the item assets, Addressables catalog, shared name table, and active locale bundles. Unchanged installations reuse `cache/item-catalog.json`. Updates are validated and written atomically before immutable catalog maps are replaced. Missing files, invalid data, unsupported types/grades, missing new English names, or ambiguous new material listing names retain the previous catalog and expose a failure status in the dashboard.

Catalog cache version 2 preserves item levels. Version 1 caches omitted levels and are rebuilt from the installed game on connection; until then, startup uses the embedded catalog. Marketable equipment without a valid positive level is rejected.

Existing entries remain available if removed from the game's table. Existing verified Steam names survive display-name changes. Legacy marketable placeholders without a known English/Steam name are excluded from pricing. The embedded file is not rewritten; updating that fallback still requires a build. Future structural game format changes can require reader maintenance.

The 2026-10-23 refresh adds 1,087 records to the existing 5,944 records, including 182 new marketable items. Existing records are preserved. The new records contain 1,000 names translated into 16 languages and 87 box names supplied as English literals by the game.

## Source and verification

- REA's IL2CPP metadata inspection identified `TaskbarHero.Data.ItemInfoData` and the `EItemType`, `EGradeType`, and `EGearType` enums.
- Item rows came from the installed game's `ItemInfoData` TextAsset in `TaskBarHero_Data/sharedassets0.assets`. UnityPy read the Unity asset format; REA supplied the managed schema and enum definitions.
- Names came from the `ItemTable` shared key table and localized string tables in `StreamingAssets/aa/StandaloneWindows64`.
- A read-only scan of the running game's `ItemInfoData` instances found all 7,031 rows. IDs, name keys, type, grade, gear type, level, Steam-item, deleted, and exchange-marketability fields matched the asset rows without differences.
- New `marketable` flags follow the observed `IsCanExchangeMarketable` value. Existing marketability/deletion annotations are retained, including legacy items temporarily blocked from exchange by the game.

The inspected files have these SHA-256 hashes:

| File | SHA-256 |
| --- | --- |
| `global-metadata.dat` | `85bf394366ce9c6c6969be9731131d5bc87c80bd94fbed3e54b8f50ca785e622` |
| `GameAssembly.dll` | `f09a3eb6dd3fcb14f9e9f612514d66b3b6d9007d4b90962436493f3f18c3089c` |

## Steam names

`name` stores the game's localized display names. Optional `market_hash_name` stores an exact Steam listing name when it differs from the normal naming convention. `market.BuildHashName` uses that override for both price requests and listing links; otherwise it derives the listing name from the item's English name, grade, and equipment variant.

Under the currently observed Steam naming rule, equipment at level 90 or above uses variant `C`; lower-level equipment uses `A`. For example, level-90 item `516193` uses `Eternal Armor (Beyond) C`, while level-80 item `516173` uses `Dimensional Armor (Beyond) A`. The choice follows the item's level, independently of its ID. The old Eternal Armor `A` listing belongs to a deleted item and retains historical sales, so it must not supply prices for the current equipment. The variant also forms part of price-cache keys and is preserved when game display names change. A future change to Steam's naming rule requires updating this cutoff.

The five Plague Fruit grades and ten grades each of Contaminated Reliquary and Contaminated Relic use a rarity suffix, for example `Plague Fruit (Rare)` and `Contaminated Relic (Arcana)`. Steam's public market search confirmed this convention for listed grades. These are materials, so they do not use the equipment ` A` suffix. Grades without a current listing use the same family naming convention; the catalog does not imply that sell orders exist.

Validate catalog changes with `go test ./internal/catalog ./internal/market` and the project checks.
