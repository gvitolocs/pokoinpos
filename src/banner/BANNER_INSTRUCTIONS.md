# Pokoin Banner Instructions

This folder stores the canonical Pokoin project banner assets.

## Files

- `pokoin-project-banner-1360x430-original.png`: original banner before the
  monster-coin replacement.
- `pokoin-project-banner-1360x430.png`: current banner with the approved
  monster-coin artwork.

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

## Logo Replacement

The banner logo has been replaced with the approved monster-coin artwork from
`src/logo/assets/pokoin.svg`. Do not restore the earlier Pikachu-like artwork.

The current banner keeps the original composition and patches the left logo
area by scaling the approved 32x32 SVG into 5px pixel blocks:

```text
banner origin: x=90, y=135
block size:    5
logo size:     160 x 160
```

## Regeneration Rule

If the banner is regenerated from scratch, use the current logo rules from:

```text
src/logo/LOGO_INSTRUCTIONS.md
```

Important:

- use the approved monster-coin SVG from `src/logo/assets/pokoin.svg`,
- preserve pixel-art edges,
- avoid blur/anti-aliasing on the logo,
- keep the same final banner size unless a new target platform requires another
  aspect ratio.
