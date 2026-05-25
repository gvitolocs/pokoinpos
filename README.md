# PokoinPoS

PokoinPoS is the native Proof-of-Stake chain used by the Pokoin/CardVault ecosystem.
It started as an ADNO static PoS exercise and has been extended into a production-style node with public RPC, health endpoints, Docker deployment, peer auto-updates, and wallet compatibility.

## Current Network Surface

- Public website and wallet: `https://pokoin.com`
- Marketplace home/catalog: `https://pokoin.com/marketplace`
- Wallet route: `https://pokoin.com/wallet`
- Public scan/explorer UI: `https://pokoin.com/scan`
- Explorer/static metadata host: `https://explorer.pokoin.com`
- Public RPC: `https://rpc.pokoin.com/rpc`
- Public health page: `https://pokoin.com/health`
- Node health API: `https://rpc.pokoin.com/health`
- Chain ID: `26062026` (`0x18dacca`)
- Native currency: `PKN`

The CardVault website and Pokoin Wallet now live in a single Flutter web app in
the separate `cardvault` repository. This repository remains the source of truth
for the chain, node runtime, RPC behavior, deployment scripts, and blockchain
documentation. See `docs/website.md` for the website route map, marketplace API
surface, data ownership, and deployment boundaries.

## Hosting Architecture

Pokoin separates public web surfaces from node runtime services:

- **Vercel hosts web/static surfaces**: `pokoin.com`, `pokoin.com/scan`,
  `pokoin.com/health`, `pokoin.com/wallet`, `explorer.pokoin.com`,
  `/wpkn/logo.png`, `/wpkn-reserve.json`, banners, favicons, and token metadata
  assets. `pokoin.com` also serves the marketplace API functions under
  `/api/marketplace-*`.
- **Oracle/Docker nodes host chain services**: P2P, node-local ops APIs, and the
  public RPC gateway exposed at `rpc.pokoin.com`.
- **Cloudflare DNS points frontend hostnames to Vercel**. `explorer.pokoin.com`
  is a static/frontend hostname and should point to Vercel (`76.76.21.21`).
- **RPC hostnames point to node/gateway infrastructure**. `rpc.pokoin.com`
  should remain backed by one or more PokoinPoS nodes or a load-balanced RPC
  gateway.

This keeps the node focused on chain execution and RPC, while public websites,
icons, reserve manifests, and explorer UI deploy through the Vercel web pipeline.

## Consensus Model

The node uses slot-based Proof-of-Stake with deterministic lottery draws. Mining weight is based on validator balance, with a whale cap rule:

- validators with positive balance collectively compete for 97% of mining probability, proportional to stake;
- zero-balance validators collectively share the remaining 3%;
- a large balance can dominate the 97% pool, but cannot take the entire validator probability alone.

Core behavior includes:

- slot-based block production,
- longest-chain total ordering,
- block and transaction validation,
- miner rewards,
- throughput measurement,
- rollback observation.

## Academic Origin

Following the original exercise specification, the implementation uses:

- a genesis configuration with **10 initial accounts**, each with **1,000,000 PKN**,
- fixed `Hardness` and `Seed` for deterministic lottery checks,
- one-second slots,
- transaction validity rules (signature, positive amount, no overdraft),
- block rewards:
  - `+10 PKN` per accepted block.

## What was implemented

### 1) Blockchain core (`peer/account`)

- Tree-based blockchain data structure.
- Deterministic PoS lottery draw and win check.
- Full block validation pipeline:
  - parent existence,
  - block signature verification,
  - lottery correctness,
  - sequential transaction validity against parent state.
- Longest-chain traversal and ledger reconstruction from genesis.
- Rollback support by ignoring the last `k` blocks of the best branch.

### 2) Peer integration (`peer/peer.go`)

- PoS transaction propagation (`PoSTransaction`) into mempool.
- Slot mining (`MineOneSlot`) and block flooding.
- Block handling with re-validation and deterministic ledger rebuild from best chain.
- Deduplication and safer connection cleanup for concurrent network behavior.

### 3) Demo and validation (`peer/handin.go`, tests)

- End-to-end demo with 10 peers.
- Valid and invalid transaction scenarios.
- Throughput printing (valid tx/s on best chain).
- Rollback comparison (`rollback=0` vs `rollback=1`).
- Added/updated tests for blockchain behavior and convergence checks.

## Main engineering steps addressed

During implementation and debugging, the main issues addressed were:

- Non-deterministic genesis serialization (fixed to keep chain roots aligned across peers).
- Runtime blocking due to message/channel pressure in network handling.
- Longest-path and metadata traversal bugs in partially implemented blockchain code.
- Consistency issues between demo behavior and test expectations.

The final result is stable for `go test ./...` and for the handin demo run.

## Run the demo

```bash
cd peer
go run .
```

Expected output includes:

- final ledgers for each peer,
- mined block count,
- throughput metric,
- rollback comparison,
- completion message.

## Run tests

```bash
cd peer
go test ./...
```

## Run production-style node mode (single validator)

By default, `go run .` runs the hand-in demo.  
Set `POKOINPOS_RUN_MODE=node` to run long-lived node mode with ops endpoints.

```bash
cd peer
POKOINPOS_RUN_MODE=node \
POKOINPOS_LISTEN_PORT=43000 \
POKOINPOS_OPS_ADDR=:8080 \
POKOINPOS_FINALITY_DEPTH=1 \
POKOINPOS_OPERATOR_TOKEN= \
go run .
```

Operational endpoints:

- `GET /health`
- `GET /ready`
- `GET /chain/status`
- `GET /metrics`
- `GET /endpoints`
- `POST /rpc` (EVM-style JSON-RPC compatibility for MetaMask-style wallets)
- `POST /admin/mine?slot=<n>` (requires `Authorization: Bearer <token>`)

See `docs/node-endpoints.md` for the full endpoint catalog and website health page integration notes.
See `docs/wallet-compatibility.md` for MetaMask custom network setup.
See `docs/public-network.md` for public RPC, explorer, and chain registry metadata.

With `POKOINPOS_FINALITY_DEPTH=1`, state-machine execution is based on finalized blocks
(best chain minus one tentative tip), aligning total-order application with ADNO11 Chapter 14/16 style.

## One-command Docker deployment for future peers

1) Copy and edit the env template:

```bash
cp deploy/env/peer.env.example deploy/env/peer2.env
```

2) Launch with one command:

```bash
docker compose --env-file deploy/env/peer2.env -f docker-compose.peer.yml up -d --build
```

This command builds the image and starts a peer that joins an existing node using:

- `POKOINPOS_BOOTSTRAP_MANIFEST_URL`
- `POKOINPOS_ADVERTISE_HOST`

The public bootstrap manifest supplies the default join peer, fallback peers,
refresh interval, EVM chain ID, and EVM network ID. Keep local env values for
node identity, public advertise host, ports, state path, and optional operator
token unless you intentionally need a local override.

The container now includes:

- automatic seed and known-peer reconnect attempts when disconnected
- advertised public peer addresses so nodes can continue syncing even if the original seed goes offline
- persistent node state on disk (chain + miner identity + last slot)

Set `POKOINPOS_ADVERTISE_HOST` to the public IP or DNS name other peers can reach
on `POKOINPOS_LISTEN_PORT`; do not leave it as `127.0.0.1` for public peers.

By default, public nodes fetch the dynamic bootstrap manifest:

```env
POKOINPOS_BOOTSTRAP_MANIFEST_URL=https://pokoin.com/bootstrap-peers.json
```

New nodes try every manifest bootstrap peer and then continue with discovered
peers, so the network can still be found if one Oracle seed is offline.

Bootstrap promotion is intentionally slow. New public nodes first spend 14 days
in vetting and must stay online for at least `95%` of that vetting window. After
vetting they can operate as regular peers, but they do not become bootstrap
nodes until they are at least 365 days old and have at least `94%` observed
uptime over the last 365 days. Uptime is not self-reported: a node needs
observations from at least `3` other peers before it can qualify. The two current
Oracle nodes are grandfathered as bootstrap peers, and node-local dashboards
expose `/chain/bootstrap` so operators can see manifest refresh status, vetting
state, peer age, external observer count, and uptime ratios.

Set a unique state path per peer in env file:

- `POKOINPOS_STATE_HOST_PATH=./.pokoinpos-peer2-state`

Useful checks:

```bash
docker compose --env-file deploy/env/peer2.env -f docker-compose.peer.yml ps
curl http://127.0.0.1:8081/health
```

## Automatic peer updates (no manual redeploy on each node)

The compose file includes an updater service (`watchtower`) that polls Docker Hub and
auto-restarts the peer when a newer image is published.

Recommended workflow:

1) Publish new image tag to Docker Hub from admin/release pipeline.
2) Keep peers running with:

```bash
deploy/scripts/docker-peer-up.sh deploy/env/peer2.env
```

3) Peers auto-pull and restart within `POKOINPOS_UPDATE_INTERVAL_SECONDS`.

Controls in env file:

- `POKOINPOS_AUTO_UPDATE=true|false`
- `POKOINPOS_UPDATE_INTERVAL_SECONDS=60`

For a full production-style Docker deployment runbook (network/firewall/router guidance, reconnect behavior, and troubleshooting), see:

- `docs/blockchain-update-workflow.md`
- `docs/docker-hub-overview.md`

## Oracle Free Tier bootstrap (MVP)

From repository root:

```bash
sudo deploy/scripts/hardening.sh
sudo deploy/scripts/bootstrap-node.sh
sudo cp deploy/systemd/pokoinpos-node.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now pokoinpos-node
```

## Backup and restore

```bash
export BACKUP_PASSPHRASE="strong-passphrase"
deploy/scripts/backup.sh
deploy/scripts/restore.sh /var/backups/pokoinpos/<backup-file>.enc
```

See `docs/operations/disaster-recovery.md` for full DR procedure and RTO/RPO targets.

## Report

- PDF: `reports/report.pdf`
- Source: `reports/report.md`
