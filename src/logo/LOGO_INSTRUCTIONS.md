# Pokoin Logo Instructions

This folder is the local source of truth for Pokoin / PKN / wPKN logo work.
The approved current mark is the 32x32 pixel-art monster coin from
`~/Downloads/Pokoin.svg`, replacing the earlier Pikachu-like artwork.

## Files

- `pokoin-logo-source.svg`: approved 32x32 SVG source. Start from this when
  regenerating assets.
- `pokoin-logo-cutout.svg`: transparent SVG copy derived from the source.
- `assets/`: local generated asset pack for the approved icon, including SVG,
  PNG resolutions `16`, `24`, `32`, `48`, `64`, `96`, `128`, `180`, `192`,
  `200`, `256`, `384`, `512`, `1024`, `2048`, `4096`, plus `favicon.ico`.
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

- Preserve the approved monster coin shape from `pokoin-logo-source.svg`.
- Preserve the white highlight/eye pixels.
- Keep the transparent background.
- Do not reintroduce the older Pikachu-like mascot silhouette.
- Keep the pixel-art / jagged border style.
- Use nearest-neighbor scaling. Do not blur or anti-alias the pixel art.
- The large website logo should be transparent/scontornato and not clipped.
- The small token icon must remain `32x32` when requested by BscScan/token
  lists, but should still be derived from the original SVG.

## Current BscScan Icon Rule

For `https://explorer.pokoin.com/wpkn/logo.png`:

- File name/path must stay `/wpkn/logo.png`.
- PNG dimensions: `200x200`.
- Transparent background.
- Original SVG-derived monster coin, not a manually redrawn badge.
- White highlights/eyes stay white.
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

- Source from the approved local `pokoin-logo-source.svg`.
- Keep the main icon and white highlights unchanged.
- Expose `/favicon-96x96.png` as the explicit PNG favicon for Google Search.
- Do not link `/favicon-48x48.png`; it is less clean than the exact `32x3`
  `96x96` version.
- The `96x96` PNG should be a clean `32x3` nearest-neighbor scale.

## Regeneration Notes

The safest generation process is:

1. Start from `pokoin-logo-source.svg`.
2. Scale with nearest-neighbor/crisp pixel rendering to the target size.
3. For website assets, generate `32`, `48`, `96`, `180`, `192`, `512`, and
   `1024` PNGs.
4. Generate `favicon.ico` from `16`, `32`, and `48` PNG entries.
5. Copy the website icon set into `../cardvault/pokemon_card_vault/web/`.
6. Copy `pokoin.svg`, `pokoin-192.png`, `pokoin-512.png`, and
   `pokoin-1024.png` into `../hypemeter/public/`.
7. Copy the token-list PNG to
   `../cardvault/pokemon_card_vault/web/wpkn/logo.png` and
   `metadata/token-assets/wpkn/logo.png`.
8. Copy extension icons to `/Users/giuseppe/pokemon-card-extension/icons/` and
   shared extension assets to `/Users/giuseppe/pokemon-card-extension/assets/`.
9. Update `../cardvault/pokemon_card_vault/web/bimi.svg`; the Cloudflare BIMI
   DNS record should stay `v=BIMI1; l=https://pokoin.com/bimi.svg;`.

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
