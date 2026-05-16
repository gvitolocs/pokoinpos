# Pokoin Banner Instructions

This folder stores the canonical Pokoin project banner assets.

## Files

- `pokoin-project-banner-1360x430-original.png`: original banner before the
  white-eye correction.
- `pokoin-project-banner-1360x430.png`: corrected banner with white eye pixels.

## Dimensions

Keep the banner dimensions unchanged:

```text
1360 x 430
```

These dimensions already fit the intended profile/header use case well.

## Visual Direction

The banner style should remain:

- dark blue / green tech background,
- pixel-art logo on the left,
- yellow pixel text `POKOIN`,
- subtitle `PKN & wPKN`,
- marketplace rails copy,
- small colored rail bars on the right.

## Logo Correction

The banner was generated with the older coin logo where the eye pixels were not
white. Do not redesign the whole banner unless explicitly requested.

For the current banner, only the two true eye highlight regions in the left coin
logo were changed from the banner's dark teal/greenish eye color to white.
The regions include the adjacent teal remnant line created by scaling:

```text
Left eye region:  x=155..160, y=190..194
Right eye region: x=175..179, y=190..194
Color:           RGBA(255, 255, 255, 255)
```

Those coordinates come from matching the original 32x32 SVG grid to the banner:

```text
banner origin: x=90, y=135
block size:    5
SVG eyes:      (13,11), (17,11)
```

In the original banner, the greenish remnants were detected at:

```text
Left remnant:  x=157..160, y=192..194
Right remnant: x=176..178, y=192..194
```

Earlier attempted origins/coordinates were wrong and should not be used:

- origin `100,145` with eyes `(12,13)`, `(20,13)`
- origin `100,145` with eyes `(13,11)`, `(17,11)`

## Regeneration Rule

If the banner is regenerated from scratch, use the current logo rules from:

```text
src/logo/LOGO_INSTRUCTIONS.md
```

Important:

- preserve white eyes,
- preserve pixel-art edges,
- avoid blur/anti-aliasing on the logo,
- keep the same final banner size unless a new target platform requires another
  aspect ratio.
