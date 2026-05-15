---
title: "PokoinPoS Project Report"
author: "PokoinPoS"
date: "\\today"
documentclass: article
geometry:
  - margin=2.5cm
  - a4paper
fontsize: 11pt
numbersections: true
---

# Goal

This report describes the current PokoinPoS implementation: a permissioned, static Proof-of-Stake blockchain node that started from ADNO Exercise 16.2 and was extended into a production-style MVP for Oracle Cloud and Docker deployments.

The project now covers:

- tree-based blockchain ordering with the longest-chain rule,
- static Proof-of-Stake block production by slots,
- deterministic transaction validation and ledger reconstruction,
- ADNO11-style committed state with configurable finality depth,
- persistent node state and reconnect behavior,
- Docker deployment with automatic peer updates,
- operational APIs, metrics, and production runbooks,
- native currency display as `PK`.

# Blockchain Design

## Genesis and static stake

Genesis defines the permissioned validator set and initial balances:

- 10 initial accounts,
- 1,000,000 PK each,
- fixed `Hardness` and `Seed` values for deterministic lottery checks.

All peers use the same genesis metadata, so they start from the same state and can independently validate blocks.

## Slot-based Proof-of-Stake

Each slot has a configurable duration, with a default of 1 second.

For each validator and slot:

1. the node computes a deterministic draw from `(Seed, Slot, VerificationKey)`,
2. winning is checked against the validator stake from genesis,
3. a winning validator creates one candidate block,
4. the block is signed and flooded to connected peers.

This is permissioned PoS: validators are known from genesis instead of being discovered through an open staking market.

## Total order and ledger state

Blocks form a tree through `ParentHash`. Peers determine total transaction order by:

1. computing block depths in the known tree,
2. selecting the best leaf by longest branch,
3. replaying transactions from genesis to that branch,
4. applying rewards after valid transactions.

The resulting ledger is the deterministic state-machine output for the selected chain.

# ADNO11 Finality Model

The runtime supports `POKOINPOS_FINALITY_DEPTH`.

With `POKOINPOS_FINALITY_DEPTH=1`, the latest best-chain tip is treated as tentative. The committed ledger is reconstructed from the best chain minus one tip block. This follows the ADNO11 Chapter 14/16 idea that the replicated state machine should apply an agreed total order only after a block is considered committed, not merely observed.

Operational APIs expose both:

- `height`: current best-chain height,
- `committedHeight`: height used for committed state,
- `finalityDepth`: configured number of tentative blocks.

# Block Validation Rules

A peer accepts a block only if all validation checks pass:

1. parent exists in the known blockchain tree,
2. block signature is valid,
3. PoS lottery draw is valid for signer, slot, seed, and hardness,
4. transaction encoding is valid and block size limits are respected,
5. transactions are valid in sequence against parent state:
   - positive amount,
   - valid transaction signature,
   - no overdraft.

If one transaction is invalid, the whole block is rejected.

# Currency, Fees, and Rewards

The native currency symbol is `PK`.

Transaction accounting:

- sender pays the full amount,
- receiver gets `amount - 1`,
- the `1 PK` difference is the transaction fee.

Block rewards:

- `+10 PK` per accepted block,
- `+1 PK` per transaction included in that block.

The currency symbol is also returned by runtime status endpoints as `currencySymbol: "PK"`.

# Runtime and Operations

## Node mode

The production-style node runtime is selected with:

```bash
POKOINPOS_RUN_MODE=node
```

Main runtime features:

- environment-based configuration,
- persistent state under `POKOINPOS_STATE_DIR`,
- periodic state saves,
- miner identity persistence,
- automatic reconnect to the configured seed peer,
- structured JSON logs.

## Operational API

The node exposes:

- `GET /health`
- `GET /ready`
- `GET /chain/status`
- `GET /metrics`
- `POST /admin/mine?slot=<n>` with bearer-token authorization

The metrics endpoint emits Prometheus-style values for chain height, committed height, finality depth, peer count, mempool depth, accepted blocks, mined blocks, accepted transactions, ledger transactions, and uptime.

# Docker Deployment

The project includes a Docker image:

```bash
docker pull newisdom/pokoinpos-peer:latest
```

For peers, the recommended deployment path is Docker Compose:

```bash
deploy/scripts/docker-peer-up.sh deploy/env/peer.env
```

The compose stack runs:

- `pokoinpos-peer`: the blockchain node,
- `pokoinpos-updater`: Watchtower, which pulls new `latest` images and restarts the node automatically.

This means existing Docker peers update when a new image is published, without manually redeploying each node.

# Oracle Cloud Deployment

The MVP was deployed on Oracle Cloud Free Tier instances. The deployment uses:

- Dockerized peer runtime,
- persistent host-mounted state directory,
- P2P port exposure for peer connections,
- ops API port for health and metrics,
- hardened host/service scripts for the non-Docker path,
- backup and restore scripts for disaster recovery.

The current live Oracle peers were updated to the latest image and both report:

- `currencySymbol: "PK"`,
- `peerCount: 1`,
- `finalityDepth: 1`.

# Peer Disconnect Behavior

If a peer disconnects:

- the local node keeps its blockchain and miner identity in persistent state,
- the peer periodically retries connection to the configured seed,
- when connectivity returns, blocks and transactions can propagate again,
- Watchtower can still update the container if Docker Hub is reachable.

Home or lab deployments behind a router must forward the configured P2P port to the host. Dynamic DNS such as DuckDNS can be used when the public IP changes.

# Verification

Current local verification commands:

```bash
cd peer
go test ./...
go test -race ./...
```

Both standard tests and race tests pass after the latest updates.

# Generate the PDF

From the repository root:

```bash
pandoc reports/report.md -o reports/report.pdf --pdf-engine=pdflatex
```
