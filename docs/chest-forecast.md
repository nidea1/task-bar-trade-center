# Region-aware chest forecasts

The snapshot reader resolves `PlayerSaveData.commonSaveData.currentStageKey`
using runtime IL2CPP field names, then looks up the corresponding
`StageInfoData.StageKey` and `STAGETYPE`. Stage data objects are cached and
validated on reuse. A stage change within one region keeps the drop countdown;
a region change resets both countdowns and clears the observed-drop flags.

| Stage types | Region | Normal chest | Blue/stage chest |
| --- | --- | --- | --- |
| `NORMAL` (0), `ACTBOSS` (1) | Normal | 180 seconds | 300 seconds |
| `PLAGUE` (2), `CONTAMINACTBOSS` (3) | PlagueLands | 240 seconds | 360 seconds |

The loaded game data confirmed `PLAGUE` for the first and last stages of
Nightmare (201201–201220), Hell (201301–201320), and Torment (201401–201420).
All 60 stage keys are covered by regression tests. Region selection uses
`STAGETYPE`, rather than hardcoding these ranges in the production reader.

The current region is included in the forecast payload. A missing or unsupported
stage type is displayed as an unavailable region; legacy layout/default timing
remains the fallback for snapshots without region information.

PlagueLands automation is independent of normal chest runes. Its base open times
and reductions use account status IDs 45/46 (normal chest) and 47/48 (stage chest),
verified from `EAccountStatus` metadata and the game's `OpenBoxStats` getter.
The reader only reads memory. It does not invoke game methods or change state.
An unresolved PlagueLands automation state does not inherit normal chest runes.

Runtime structures verified against TaskBarHero 1.3.0 on 2026-10-22:

- IL2CPP class field table at `+0x80`; `FieldInfo` entries have stride `0x20`,
  declaring class at `+0x10` and field offset at `+0x18`.
- `AccountStatus` integer dictionary at `+0x10`; dictionary entries at `+0x18`
  and count at `+0x20`. Inline entries have stride 28 and keys at `+8`.
- Inline `ObscuredInt` is 16 bytes. Its read-only value is
  `(uint32(hiddenValue) - uint32(currentCryptoKey)) ^ uint32(currentCryptoKey)`.

These native runtime layouts can require maintenance after structural game or
IL2CPP changes. Regression tests cover moved named fields, normal/PlagueLands
stage types, region transitions, separate automation, and invalid reads.
