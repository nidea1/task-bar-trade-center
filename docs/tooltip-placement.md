# Tooltip placement investigation

Verified against TaskBarHero 1.3.0 on Windows x64, 2026-10-22.

## Evidence

The [REA IL2CPP reader](https://github.com/morluto/rea/blob/888b2afc0d848885d9a0e3bc7560eab5b1d0c7d6/docs/il2cpp-metadata.md)
was run locally against the installed `global-metadata.dat` and `GameAssembly.dll`.
The published REA 6.7.0 CLI did not expose this command, so the source provider
at commit `888b2afc0d848885d9a0e3bc7560eab5b1d0c7d6` was invoked with Node 24
and tsx. REA is an investigation tool, not an application runtime dependency.

| Artifact | SHA-256 |
| --- | --- |
| global-metadata.dat (version 31) | `85bf394366ce9c6c6969be9731131d5bc87c80bd94fbed3e54b8f50ca785e622` |
| GameAssembly.dll | `f09a3eb6dd3fcb14f9e9f612514d66b3b6d9007d4b90962436493f3f18c3089c` |

The metadata identifies `TaskbarHero.ItemTooltip` (type 790), its `rectTooltip`
field, and `UnityEngine.RectTransform.GetWorldCorners` (method 84162, native
RVA `0x4907c70`). These are metadata observations, not a decompilation of the
game's positioning implementation.

Read-only live memory inspection established the hierarchy:

```text
ItemTooltip
  rectTooltip (+0x28) -> managed RectTransform
    native pointer (+0x10) -> UI_ItemTooltip
      child array (+0x60), first child -> Window
        bg
        ExplandedBG
        item content panels
```

The outer `UI_ItemTooltip` has a fixed 480-unit height. Its `Window` child has
the content-dependent rectangle. Previously, negated local coordinates and
calibration tables approximated screen positions; those approximations could
align one item while drifting on another.

## Runtime calculation

The optional `tooltip.rect_transform_pointer_offsets` chain resolves the native
`Window` transform using `tooltip.height_pointer_base_aob`. Native offsets for
this build are:

| Structure | Offset | Value |
| --- | --- | --- |
| RectTransform | `0xA8` | local rectangle: x, y, width, height |
| Transform | `0x28`, `0x30` | hierarchy pointer, transform index |
| Hierarchy | `0x10` | transform count |
| Hierarchy | `0x18`, `0x20` | local TRS array, parent-index array |
| Local TRS | stride `0x30` | translation float4, quaternion float4, scale float4 |

The reader transforms all four local rectangle corners through their ancestors.
The game's screen-space overlay canvas uses client pixels with upward Y.
Converting Y with the current client height and adding `ClientToScreen`'s origin
produces the actual Windows screen rectangle, including window movement and UI
scale. No game functions are invoked and no game memory is written.

The HUD is horizontally centered and attached to the actual bottom edge. If it
does not fit in the tooltip monitor's work area, it is attached to the top edge.
There is no added gap or empirical X/Y correction on this path.

Legacy layouts without the optional transform chain retain their previous
coordinate/calibration behavior. The local launcher pins the updated layout so
an older remote layout cannot replace it during verification.
The dashboard and tray no longer expose a manual game scale selector. Stored
scale values still round-trip through settings for legacy layout compatibility;
they have no effect on native transform placement.

## Verification and limits

Tests cover different item heights, parent movement, 1x/1.25x/1.5x scaling,
quaternion rotation, invalid pointers and counts, cyclic parents, non-finite
values, screen edges, negative monitor coordinates, above placement and bypass
of legacy calibration. Live testing confirmed that changing items tracks their
actual bounds.

The native offsets and screen-space canvas assumption are specific to the
verified build. Future Unity/game changes may require rediscovery. This does
not implement a camera projection for world-space or camera-space canvases.
