# Hand-in 10 - Exercise 16.2 (Static Proof-of-Stake Blockchain)

This repository contains the implementation of **Exercise 16.2** from the ADNO course material, focused on a simple but complete **tree-based blockchain with static Proof-of-Stake (PoS)**.

The project starts from the previous peer-to-peer flooding ledger and extends it into a PoS blockchain system with:

- slot-based block production,
- longest-chain total ordering,
- block and transaction validation,
- miner rewards,
- throughput measurement,
- rollback observation.

## Context from the exercise

Following the exercise specification, the implementation uses:

- a genesis configuration with **10 initial accounts**, each with **1,000,000 PK**,
- fixed `Hardness` and `Seed` for deterministic lottery checks,
- one-second slots,
- transaction validity rules (signature, positive amount, no overdraft),
- block rewards:
  - `+10 PK` per accepted block,
  - `+1 PK` per transaction included in that block.

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
POKOINPOS_ADMIN_TOKEN=change-me \
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

- `POKOINPOS_JOIN_HOST`
- `POKOINPOS_JOIN_PORT`

The container now includes:

- automatic seed reconnect attempts when disconnected
- persistent node state on disk (chain + miner identity + last slot)

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
