# Pokoin Logo Instructions

This folder is the local source of truth for Pokoin / PKN / wPKN logo work.

## Files

- `pokoin-logo-source.svg`: original SVG source. Start from this when
  regenerating assets.
- `pokoin-logo-source-preview.png`: original small preview generated with the
  first SVG export.
- `pokoin-logo-cutout.svg`: transparent cutout version derived from the source.
- `pokoin-logo-4096-cutout.png`: high-resolution transparent master.
- `pokoin-logo-1080-cutout.png`: 1080 transparent cutout export.
- `pokoin-logo-1080-black-outline.png`: 1080 export with black outline and
  preserved white eye pixels.
- `pokoin-logo-1080-original-white-bg.png`: older 1080 export with white
  background retained for comparison only.
- `pokoin-logo-200.png` and `pokoin-logo-200.jpg`: earlier 200px preview files.
- `wpkn-logo-32.png`: BscScan/token-list icon currently published at the
  stable public URL.
- `favicon-soft/`: website `favicon.ico` variant only. It may have very
  slightly rounded corners so browser tabs do not look like a hard-cut square.
- `favicon-google/`: Google Search PNG favicon exports. Prefer `96x96`, because
  it is an exact `32x3` nearest-neighbor scale from the approved token logo.

## Stable Public URLs

Do not change these links after submitting them to third parties:

- wPKN logo URL: `https://explorer.pokoin.com/wpkn/logo.png`
- wPKN reserve proof: `https://explorer.pokoin.com/wpkn-reserve.json`
- Website icon assets: `https://pokoin.com/pokoin-512.png`,
  `https://pokoin.com/pokoin-1024.png`, `https://pokoin.com/favicon.ico`

If an icon needs updating, replace the file served behind the existing URL.
Do not rename the public URL unless the third-party listing is also updated.

## Visual Rules

- Preserve the original pixel-art shape from `pokoin-logo-source.svg`.
- Preserve the two white eye pixels.
- Remove only the white/near-white background connected to the SVG outer edge.
- Do not remove interior white pixels.
- Keep the pixel-art / jagged border style.
- Use nearest-neighbor scaling. Do not blur or anti-alias the pixel art.
- The large website logo should be transparent/scontornato and not clipped.
- The small token icon must remain `32x32` when requested by BscScan/token
  lists, but should still be derived from the original SVG.

## Current BscScan Icon Rule

For `https://explorer.pokoin.com/wpkn/logo.png`:

- File name/path must stay `/wpkn/logo.png`.
- PNG dimensions: `32x32`.
- Transparent background.
- Original SVG-derived logo, not a manually redrawn badge.
- Eyes stay white.
- The circle should fit the square edge-to-edge: opaque bbox should be
  `(0,0)-(31,31)` while the four corners remain transparent.
- Use cache-busting only for testing, for example:
  `https://explorer.pokoin.com/wpkn/logo.png?v=<timestamp>`.

## Website Homepage Logo Rule

The CardVault/Pokoin homepage card should use a large asset, not the tiny
BscScan icon:

- Preferred URL: `https://pokoin.com/pokoin-1024.png`
- Do not wrap it in `ClipOval`.
- Do not crop the pixel-art border.
- Use a larger box around the image so the full silhouette is visible.
- In Flutter, prefer `FilterQuality.none` for the pixel-art look.

## Website Favicon Rule

The browser favicon can be a separate ICO-only variant:

- Source from the approved local `metadata/token-assets/wpkn/logo.png`.
- Do not modify the BscScan/token-list PNG when changing the favicon.
- Very slightly round the ICO corners only, just enough to avoid a hard square
  tab icon.
- Keep the main icon and eyes unchanged.
- Expose `/favicon-96x96.png` as the explicit PNG favicon for Google Search.
- Do not link `/favicon-48x48.png`; it is less clean than the exact `32x3`
  `96x96` version.
- The `96x96` PNG should be a clean `32x3` nearest-neighbor scale.

## Regeneration Notes

The safest generation process is:

1. Parse `pokoin-logo-source.svg`.
2. Flood-fill only white/near-white pixels connected to the image border.
3. Make that outside background transparent.
4. Preserve all non-background pixels, including the two white eye pixels.
5. Scale with nearest-neighbor to the target size.
6. For website assets, generate larger PNGs: `192`, `512`, `1024`, `4096`.
7. For BscScan/token-list assets, generate exactly `32x32`.

## Deployment Notes

The live BscScan/token-list logo is now served by the Vercel web project behind
`https://explorer.pokoin.com/wpkn/logo.png`. Keep the canonical local source in
this repository and copy the approved file into the web app before deploying:

```bash
cp metadata/token-assets/wpkn/logo.png \
  ../cardvault/pokemon_card_vault/web/wpkn/logo.png
```

Do not upload explorer icons to the Oracle node unless you are intentionally
maintaining an emergency fallback. The stable public URL must remain unchanged;
only the file behind it changes.

Verify:

```bash
python3 - <<'PY'
import urllib.request, struct, time
url = "https://explorer.pokoin.com/wpkn/logo.png?v=" + str(int(time.time()))
data = urllib.request.urlopen(url, timeout=15).read()
print(struct.unpack(">II", data[16:24]))
PY
```

Expected output for the BscScan icon:

```text
(32, 32)
```
