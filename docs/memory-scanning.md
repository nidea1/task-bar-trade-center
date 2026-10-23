# Memory scanning

All game access remains read-only. The scanner uses `VirtualQueryEx` to enumerate
committed, readable regions and `ReadProcessMemory` to inspect them. No game
methods are invoked and no code is injected.

## Discovery and live reads

IL2CPP class names needed by inventory, runes, region data, chest timers and
trade slots are discovered together. One pass locates the names; another locates
and validates their class metadata. A third pass gathers initial object
references for the known classes together. Consumers validate these candidates
using their existing named-field and structure checks. Unused discovery results
are consumed once, rather than treated as permanently live objects.

Class pointers imported from the session cache are checked against their exact
name, element class and cast class before use. Replacing a save-data instance
does not discard valid class metadata. The existing process/build cache matching
still controls persistent address reuse.

Trade-slot discovery retains object addresses, not cooldown values. Subsequent
reads validate the Unity native pointer, class, index, cache and text references,
then read the current deadline. They do not scan the process while the expected
active/unlocked slot indexes remain available. Locked backing save slots do not
require UI objects. Destroyed objects allow rediscovery at most once every five
seconds. An absent optional trade UI retries after thirty seconds instead of
repeatedly scanning on every poll; save-backed data remains the fallback.

The current game's trade UI uses named `Index`, `m_coolTimeText`, `bkkz` and `bklc`
fields, validated against the expected cache class and plausible dates. REA and
live FieldInfo inspection showed these moved to 0xA8, 0xF8, 0x100 and 0x118 in
1.3.0. Unknown named layouts are rejected. The current
`RemakeTradingStashSaveData.ItemUniqueId` is never decoded as a legacy timestamp;
it contains an item ID, so a valid live UI deadline is required for that format.

The shared scanner allocates one chunk/overlap buffer per scan rather than new
buffers for every memory chunk and pattern. It supports overlapping matches,
mixed pattern lengths and matches spanning chunk boundaries. Unreadable chunks
break the overlap so bytes on either side of a gap cannot form a false match.
Result limits are positive and apply independently to each nonempty pattern.

## Local measurements

Read-only measurements on 2026-10-23, TaskBarHero 1.3.0, on one Ryzen 7 9800X3D
Windows machine while the same game session was running:

| Measurement | Before | After |
| --- | --- | --- |
| Cold inventory core resolution | 15.27 s | 4.17–4.48 s |
| Cold full-process scan passes | 13 | 3 |
| Bytes read by cold scans | 25.21 GB | 5.77–5.80 GB |
| First full snapshot after core resolution | 4.92 s | 19 ms |
| Subsequent full snapshots | 685–696 ms, one scan each | 17–19 ms, no scans |

Restarting the application with a valid same-game-session class cache took
about 0.61–0.63 seconds for inventory core resolution, using one combined object
scan.

These measure inventory resolution, not total application startup or network
requests. Item counts changed from 65 to 68 during the measurements because the
game continued progressing; region, slot capacity, ten trade slots and live
normal-chest observations remained consistent. Other computers and game states
will have different timings.

Verification covers batch discovery, duplicate and invalid class candidates,
imported-class validation, updated cooldown values without rescanning, destroyed
Unity component recovery, chunk boundaries, unreadable gaps and randomized
pattern/limit comparisons against a direct search. Run `go test ./...` and
`go test ./internal/tbhmem -run '^$' -bench BenchmarkScanMemoryRegion -benchmem`
on Windows.
