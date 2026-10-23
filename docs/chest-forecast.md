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

The 2026-10-23 rune update also resolves PlagueLands unlocks and reductions from
the current player's `RuneSaveData` list. When that list is resolved, account
totals must match its unlocks, base times, and reductions before they are used.
If a matching account object is unavailable, the verified rune values supply
the forecast and the diagnostic records that source. Totals and individual
runes are never added together.

| PlagueLands chest | Unlock rune / base time | Reduction rune IDs | Seconds per level | Maximum total reduction |
| --- | --- | --- | --- | --- |
| Normal | `1163` / 600 s | `11631`, `11632`, `11633` | 4 s, up to 5 levels each | 60 s |
| Blue/stage | `1165` / 1200 s | `11651`, `11652`, `11653` | 8 s, up to 5 levels each | 120 s |

These values came from `RuneInfoData` and `RuneLevelInfoData` TextAssets; REA
metadata identified the managed fields and account status enums. All 48
PlagueLands rune-level rows, including act-boss rows, matched the running game's
`RuneLevelInfoData` objects. Act-boss unlock `1167` and reductions
`11671`/`11672`/`11673` affect act-boss chests; they do not change the normal/blue
periodic-drop forecast. Automatic opening requires the corresponding unlock
rune. Drop intervals remain 240/360 seconds; with all reduction runes at level 5,
opening intervals are 540/1080 seconds.

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

## Live opening progress

The snapshot reader also observes the current region's `StageBox` components.
In 1.3.0, `bkmu` is the base opening duration, `bkmv` the effective duration,
`bkmx` elapsed opening time, and `bkmw` the automatic-opening toggle. Read-only
native inspection and live samples confirmed that the slider advances as
`elapsed/effectiveDuration` (31.15 → 34.15 → 37.15 seconds over two three-second
samples). The reader checks finite values, native component validity, region,
duration bounds, and agreement with the slider. Conflicting instances are rejected.

The current `countText` value, such as `10/40`, supplies the waiting chest count.
Its `TMP_Text.m_text` field is discovered through the class hierarchy. This
avoids relying on the old `PlayerSaveData + 0x20` box structure, which changed
in 1.3.0. The event queue starts its first opening at the observed remaining time
and subsequent openings at the full effective interval. An empty queue still
waits for a drop; switching automatic opening off stops that stream.

Snapshots are refreshed at least every six seconds while the game is attached.
Cached enabled timers are aged between reads, and observations older than 15
seconds are discarded. The dashboard distinguishes live remaining time from
an estimated opening interval. Normal unlock runes `13002`/`15001` are checked
separately from their reduction runes, including an unlocked level-zero reduction.
