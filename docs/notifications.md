# Inventory warnings and item targets

Dashboard Settings contains the fill-warning threshold (default five minutes,
with an off setting). Catalog checks run automatically in the background; live
chest countdowns appear in the Inventory Fill tooltip. The separate companion
panel has been removed. Stored item targets remain compatible with existing
settings and continue to be evaluated, without a target editor in the dashboard.
Preferences persist with the existing dashboard settings. Legacy preferences
receive the default threshold and an empty target list.

A warning fires when a valid forecast crosses from above the selected threshold
to at or below it. The first observation establishes a baseline. Repeated polls
do not repeat the warning; the estimate must recover above the threshold plus
two minutes before another crossing can alert. Changing regions or the threshold
establishes a fresh baseline. Existing inventory-full notifications remain separate.

Targets count quantities across inventory, normal stash, material stash, and
equipped items. A unique ID is counted only once, so location changes and stale
duplicate references do not satisfy a target. Missing legacy quantities mean one
item; current stack quantities come from named save-data fields. Material-stash
entries are read from the current player's `materialStashSaveDatas` list.
Unreadable material-list data rejects the snapshot rather than treating a failed
read as an empty stash. Material-stash stacks do not consume inventory slots.

New or edited targets establish a baseline without announcing existing stock.
Notifications fire when stock crosses the target count; they can fire again
after stock falls below the target and is replenished. Older snapshots are ignored
by the alert tracker. Game reattachment resets its baselines. Target alerts are
independent of the existing minimum-rarity/source filters for acquisition alerts.

Alerts use Windows tray notifications while the app is running. Item/quantity
targets are local settings and are not transmitted to an external service.
