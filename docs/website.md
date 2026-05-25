# Pokoin Website

`https://pokoin.com` is the canonical public website for the Pokoin/CardVault
ecosystem. The live website is a Flutter web app deployed from the separate
`cardvault/pokemon_card_vault` project, while this repository remains the source
of truth for the PokoinPoS chain, RPC runtime, public network metadata, and node
deployment documentation.

## Public Routes

- `https://pokoin.com/`: CardVault marketplace home, including Oracle-backed Best sellers and Featured sections.
- `https://pokoin.com/marketplace`: marketplace catalog and search entry point.
- `https://pokoin.com/marketplace/<locale>/cards/<card-slug>`: card detail pages with seller listings, expansion navigation, and related versions.
- `https://pokoin.com/wallet`: integrated Pokoin Wallet for PKN balances and browser-wallet transfers.
- `https://pokoin.com/scan`: public scan UI for chain-facing status and explorer links.
- `https://pokoin.com/health`: public website health surface.
- `https://pokoin.com/bootstrap-peers.json`: bootstrap peer manifest consumed by PokoinPoS nodes.

## Website Runtime

Vercel hosts the Flutter SPA and serverless API functions. Web routes are
rewritten to the Flutter `index.html`, while `/api/*.js` functions serve
marketplace and wallet-backed data.

Important marketplace APIs:

- `GET /api/marketplace-home`: home snapshot and carousel payloads.
- `GET /api/marketplace-cards`: catalog/product rows.
- `GET /api/marketplace-card-versions`: expansion-scoped card navigation and versions.
- `POST /api/marketplace-search-candidates`: Oracle-backed search candidates.
- `POST /api/marketplace-autocomplete`: CardTrader-style autocomplete ranking.
- `POST /api/marketplace-event`: bounded marketplace interaction analytics.
- `GET /api/marketplace-hot-blueprints?window=1h|24h|7d&limit=50`: rolling hot blueprint analytics.

## Data Ownership

- Oracle Postgres stores marketplace catalog, search projections, expansion
  versions, variation dimensions, Cardmarket parsing metadata, raw marketplace
  events, and hot blueprint rollups.
- Firebase stores identity, profiles, balances, orders, carts, withdraw requests,
  and mutable seller listings.
- PokoinPoS nodes serve the chain/RPC surfaces at `https://rpc.pokoin.com`.
- Vercel serves the website, static public metadata, and marketplace API layer.

## Search And Homepage Behavior

Search treats structured card variation tokens as first-class intent. Queries
such as `v`, `ex`, `gx`, `vmax`, `mega`, and `shining` should filter or rank
against real variant tokens, not loose substrings. Single `vstar` ranks real
VSTAR cards first and may also include exact `VSTAR` set-token matches such as
`VSTAR Universe`.

Homepage Best sellers and Featured sections must come from Oracle snapshot
section IDs, backed by `public.marketplace_hot_blueprints` and
`public.get_marketplace_home_snapshot(...)`. Do not hardcode a single expansion,
rarity, or curated list in the website client.

## Deployment Notes

Deploy the website from `cardvault/pokemon_card_vault` with its deploy script:

```bash
./deploy-pokoin-web.sh
```

Do not run plain `vercel deploy` from this repository for `pokoin.com`. The
website deploy script builds Flutter, copies API functions into `build/web/api`,
rewrites server helper imports for Vercel output, and deploys the prepared web
artifact.

After deployment, verify:

```bash
curl -I https://pokoin.com/
curl -I https://pokoin.com/wallet
curl -fsS 'https://pokoin.com/api/marketplace-home?locale=en&limit=12'
curl -fsS 'https://pokoin.com/api/marketplace-hot-blueprints?window=24h&limit=5'
```
