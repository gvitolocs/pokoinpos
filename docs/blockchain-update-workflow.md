# PokoinPoS Blockchain Update Workflow

This is the normal workflow for updating the live PokoinPoS blockchain runtime.
Nodes should not be edited one by one for normal code changes. Update the repo,
publish a new Docker image, and let the nodes auto-update from Docker Hub.

## What Changes Where

- Chain/runtime code lives in `peer/`.
- Docker packaging lives in `Dockerfile` and `docker-compose.peer.yml`.
- Per-node runtime values live in `deploy/env/*.env`.
- Public bootstrap registry files live in `deploy/bootstrap/`.
- Oracle nodes run `newisdom/pokoinpos-peer:latest` and are restarted by
  Watchtower when a newer image is pushed.

## Normal Runtime Update

Use this flow when changing Go node code, RPC behavior, dashboard behavior,
bootstrap fetching, peer discovery, mining rules, or ops endpoints.

```bash
make test
make vet
make lint
```

Build and publish the multi-architecture image:

```bash
make docker-push
```

After the image is pushed, running nodes with Watchtower enabled pull the new
image and restart automatically. The polling interval is controlled by:

```env
POKOINPOS_AUTO_UPDATE=true
POKOINPOS_UPDATE_INTERVAL_SECONDS=60
```

## When Env Files Must Change

Publishing a new Docker image is enough for code-only updates. Update
`deploy/env/*.env` only when adding or changing runtime variables, ports,
bootstrap peers, reward settings, or chain identity values.

After changing env files, copy the relevant env file to the node and recreate the
container once. Future code-only updates can go back to the Docker Hub flow.

```bash
scp deploy/env/peer3.env ubuntu@<node-ip>:/home/ubuntu/peer3.env
ssh ubuntu@<node-ip> 'docker compose --env-file /home/ubuntu/peer3.env -f /home/ubuntu/docker-compose.peer.yml up -d --remove-orphans'
```

Some older VMs only have legacy Compose v1:

```bash
docker-compose --version
```

If `docker compose ...` fails with `unknown shorthand flag: 'f' in -f`, use
`docker-compose` instead. If Compose v1 fails with `KeyError:
'ContainerConfig'` or a container-name conflict, remove only the stopped peer
container and recreate it from the same env file. Do not delete the state
directory unless you are intentionally resetting the node as a late joiner.

```bash
sudo docker rm -f pokoinpos-peer3
sudo docker-compose -f docker-compose.peer.yml --env-file peer3.env up -d pokoinpos-peer
```

## Consensus Rule Updates

Treat consensus changes differently from ordinary RPC/dashboard updates. Examples
include block validation, lottery draws, hardness, fork choice, ledger replay,
transaction validity, rewards, genesis interpretation, and state persistence.

Before rollout:

```bash
make test
make vet
make lint
```

Then deploy the new image to all validators in the same maintenance window. Do
not leave some validators on old consensus code and others on new consensus code
for long periods. If the change intentionally preserves backward compatibility,
verify both old and new block formats in tests before pushing.

Current consensus note, 2026-05-21:

- Legacy blocks without `LotteryProof` remain valid and use the old deterministic
  draw rule `hash(seed, slot, validatorPublicKey)` so the existing chain can keep
  syncing.
- New blocks carry `LotteryProof`, a validator signature over `(seed, slot)`.
  The lottery draw is derived from that proof signature and the proof is verified
  against the block validator public key.
- New block signatures bind `LotteryProof`, so a proof cannot be swapped after
  the block is signed.
- This fixes the predictability issue where anyone could precompute lottery
  results for every peer from public data.

Post-rollout checks:

```bash
curl -fsS http://127.0.0.1:<ops-port>/chain/status
curl -fsS http://127.0.0.1:<ops-port>/explorer/blocks?limit=5
curl -fsS http://127.0.0.1:<ops-port>/metrics | egrep "chain_height|mempool|lottery"
```

Expected result: all peers converge to the same height/hash, old historical
blocks still import, and newly mined blocks include `LotteryProof`.

## Bootstrap Registry Update

Use this when the bootstrap candidate list, observer policy, or generated
manifest changes.

```bash
python3 deploy/scripts/bootstrap-monitor.py \
  --observer-id <observer-peer-id> \
  --candidates deploy/bootstrap/candidates.json \
  --history deploy/bootstrap/uptime-history.json

python3 deploy/scripts/bootstrap-rotate.py \
  --candidates deploy/bootstrap/candidates.json \
  --history deploy/bootstrap/uptime-history.json \
  --output deploy/bootstrap/bootstrap-peers.json
```

Then copy the generated manifest into the web app static folder before deploying
the site:

```bash
cp deploy/bootstrap/bootstrap-peers.json ../cardvault/pokemon_card_vault/web/bootstrap-peers.json
```

Nodes fetch `https://pokoin.com/bootstrap-peers.json` first. The manifest carries
fallback peers, the default join peer, refresh interval, EVM chain ID, and EVM
network ID; local env values are only needed when intentionally overriding those
network defaults.

## Verify Rollout

Check Docker Hub and then each node health endpoint:

```bash
docker buildx imagetools inspect newisdom/pokoinpos-peer:latest
curl -fsS https://rpc.pokoin.com/health
curl -fsS https://rpc.pokoin.com/chain/status
curl -fsS https://rpc.pokoin.com/chain/bootstrap
curl -fsS -X POST https://rpc.pokoin.com/rpc \
  -H 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"eth_getCode","params":["0x0000000000000000000000000000000000000000","latest"]}'
```

On a VM:

```bash
docker ps
docker logs --tail=100 pokoinpos-peer
curl -fsS http://127.0.0.1:8080/health
curl -fsS http://127.0.0.1:8080/chain/status
curl -fsS http://127.0.0.1:8080/chain/bootstrap
curl -fsS -X POST http://127.0.0.1:8080/rpc \
  -H 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"eth_estimateGas","params":[{"to":"0x0000000000000000000000000000000000000000","data":"0x"}]}'
curl -fsS http://127.0.0.1:8080/admin/mempool \
  -H "Authorization: Bearer $POKOINPOS_OPERATOR_TOKEN"
```

For a multi-peer health check, verify the same invariants on every node, not
only that the process is running:

```bash
curl -fsS http://127.0.0.1:<ops-port>/chain/status
curl -fsS http://127.0.0.1:<ops-port>/chain/supply/total.txt
```

Expected fields should match across the network:

- `height` should converge, allowing for a short delay while a node imports new
  blocks.
- `txCount` should match after sync.
- `totalSupply` should match after sync.
- EVM JSON-RPC should answer contract-related methods such as `eth_getCode`,
  `eth_call`, `eth_getStorageAt`, `eth_getLogs`, and `eth_estimateGas`.
- If `mempoolDepth` differs between nodes, inspect only through the
  operator-protected local diagnostics endpoint `/admin/mempool`; do not expose
  mempool contents publicly.
- Nodes automatically evict pending mempool transactions that no longer apply to
  the current best ledger after blocks are accepted and before candidate mining.
- Persisted genesis metadata must match, especially:
  - `genesis.Hardness`
  - `genesis.Seed`
  - number and contents of `genesis.InitialBalances`

## High CPU / Mining Loop Diagnosis

If an Oracle CPU chart shows one or more PokoinPoS VMs pinned near or above
`100%`, first identify whether the hot process is `/app/peer`:

```bash
ssh ubuntu@<node-ip> 'uptime; ps -eo pid,ppid,comm,%cpu,%mem,etime,args --sort=-%cpu | sed -n "1,12p"; sudo docker stats --no-stream'
```

Then compare chain status and metrics:

```bash
curl -fsS http://127.0.0.1:<ops-port>/chain/status
curl -fsS http://127.0.0.1:<ops-port>/metrics | egrep "mempool|lottery|blocks_mined|chain_height"
```

Observed incident, 2026-05-20:

- `pokoinpos-peer2` and `pokoinpos-peer3` were using about one full CPU core.
- Both had non-empty mempools containing old external asset credit transactions
  for `BTC`, `ETH`, and `BNB`.
- `lotteryAttempts` was increasing rapidly while `lotteryWins` stayed at `0`.
- `peer4` had an empty mempool and low peer CPU.

Root cause: pending mempool transactions made the runtime mine every slot and
the runtime defaults amplified that into many attempts per second
(`POKOINPOS_MINE_ATTEMPTS_PER_PENDING_TX=100`, capped at
`POKOINPOS_MAX_MINE_ATTEMPTS_PER_TICK=100`). Each attempt also called
`ValidatorStake()`, which rebuilt the best ledger through `BestBalanceOf()`, and
older `MineOneSlot` code could replay the best ledger and select candidate
transactions before confirming that the local validator's lottery draw could
win.

Fixes:

- `MineOneSlot` now performs a cheap optimistic lottery precheck before replaying
  the ledger and selecting candidate transactions. Losing slots return
  immediately; winning candidates still re-check exact hardness after valid
  transaction selection before block creation.
- `ValidatorStake()` uses the already-maintained peer ledger instead of
  reconstructing the best-chain ledger on every lottery attempt.
- Production mining defaults are bounded to
  `POKOINPOS_MINE_ATTEMPTS_PER_PENDING_TX=1` and
  `POKOINPOS_MAX_MINE_ATTEMPTS_PER_TICK=5`, so a stale non-empty mempool cannot
  spin thousands of lottery attempts per minute.

Expected healthy pattern after rollout:

- Non-empty mempool may still exist until a validator wins and includes the
  pending transactions.
- `lotteryAttempts` can continue increasing, but should climb slowly, not by
  thousands within the first minute after restart.
- `docker stats` should show the peer container near idle except during actual
  block validation/import, real transaction pressure, or short startup bursts.

If CPU remains high after the fixed image rolls out, inspect the guarded mempool:

```bash
curl -fsS http://127.0.0.1:<ops-port>/admin/mempool \
  -H "Authorization: Bearer $POKOINPOS_OPERATOR_TOKEN"
```

Do not manually delete state or reset a node just to clear mempool CPU unless the
node is divergent. Prefer a code/runtime fix and normal Docker rollout.

When checking persisted state directly:

```bash
sudo python3 - <<'PY'
import json, pathlib
p = pathlib.Path('/path/to/node-state.json')
s = json.loads(p.read_text())
print('blocks', len(s['blocks']), 'lastSlot', s.get('lastSlot'))
print('genesisHardness', s['genesis'].get('Hardness'))
print('genesisSeed', s['genesis'].get('Seed'))
print('genesisBalances', len(s['genesis'].get('InitialBalances', {})))
PY
```

For the current public chain, the canonical genesis uses `genesisHardness
10000`, `genesisSeed 42`, and `genesisBalances 2`. A node with a different
genesis must be treated as divergent even if it has a similar block height.

## Historical Sync and Divergent Nodes

Nodes now announce chain metadata during handshake:

- `genesisHash`
- `height`
- `bestHash`

If a joining node has no meaningful local history, it can adopt the canonical
chain from a bootstrap peer. If a node already has local history and the genesis
metadata differs, sync is refused and the node logs a critical
`chain_sync_*_genesis_mismatch` event. This is intentional: do not merge chains
with incompatible genesis metadata.

Important implementation detail: `genesisHash` must be based on the genesis
metadata content, not only the old block hash. The old genesis block hash did not
include metadata and could make different genesis configurations look identical.

Use this safe recovery flow for a divergent node:

1. Confirm the reference node is healthy.
2. Stop the divergent node.
3. Back up its current state file with a timestamp.
4. Remove only the active state file.
5. Restart the node from the latest Docker image and correct env file.
6. Verify it imports from bootstrap and that `height`, `txCount`, `totalSupply`,
   and genesis metadata match the reference.

Example for a node in `/home/ubuntu` using legacy Compose v1:

```bash
cd /home/ubuntu
ts=$(date +%Y%m%d%H%M%S)
sudo docker stop pokoinpos-peer3 || true
sudo cp .pokoinpos-peer3-state/node-43000-state.json \
  .pokoinpos-peer3-state/node-43000-state.divergent-$ts.json
sudo rm -f .pokoinpos-peer3-state/node-43000-state.json
sudo docker rm -f pokoinpos-peer3 || true
sudo docker-compose -f docker-compose.peer.yml --env-file peer3.env pull pokoinpos-peer
sudo docker-compose -f docker-compose.peer.yml --env-file peer3.env up -d pokoinpos-peer
sleep 30
curl -fsS http://127.0.0.1:8080/chain/status
curl -fsS http://127.0.0.1:8080/chain/supply/total.txt
```

Example for a node in `/opt/pokoinpos-docker` using Compose v2:

```bash
cd /opt/pokoinpos-docker
ts=$(date +%Y%m%d%H%M%S)
sudo docker compose -f docker-compose.peer.yml --env-file peer2.env stop pokoinpos-peer
sudo cp state-peer2/node-43001-state.json \
  state-peer2/node-43001-state.divergent-$ts.json
sudo rm -f state-peer2/node-43001-state.json
sudo docker compose -f docker-compose.peer.yml --env-file peer2.env pull pokoinpos-peer
sudo docker compose -f docker-compose.peer.yml --env-file peer2.env up -d pokoinpos-peer
sleep 30
curl -fsS http://127.0.0.1:8080/chain/status
curl -fsS http://127.0.0.1:8080/chain/supply/total.txt
```

Never delete backup files after recovery unless a separate archival backup has
been made.

## Bootstrap and Self-Connection Checks

A node must not use itself as its only bootstrap target. The runtime filters
self-bootstrap entries by `POKOINPOS_ADVERTISE_HOST` and
`POKOINPOS_LISTEN_PORT`, and logs `bootstrap_self_peer_skipped` when it skips its
own address.

When validating a node, inspect:

```bash
sudo docker logs --tail=150 <container-name> | grep -E 'bootstrap|chain_sync|critical'
curl -fsS http://127.0.0.1:<ops-port>/chain/bootstrap
```

Common signals:

- `bootstrap_self_peer_skipped`: normal if the manifest includes this node.
- `bootstrap_join_succeeded`: the node connected to a bootstrap peer.
- `chain_sync_suffix_parent_mismatch`: the node asked for an incremental suffix
  but the remote branch did not attach to the local tip; the node may request a
  full chain and accept only if there is a safe common ancestor.
- `chain_sync_full_replacement_refused`: the node refused to replace local
  history. Check genesis metadata and decide whether to reset the node as a
  late joiner.
- `chain_sync_*_genesis_mismatch`: genesis metadata differs. Back up and reset
  the divergent node; do not force-merge it.

## Code Is Not Live Until Docker Is Published

The repository can contain fixes that are not yet running on Oracle. Live nodes
run the Docker image currently published as:

```text
newisdom/pokoinpos-peer:latest
```

If you change Go code in `peer/`, the Oracle nodes do not see that change until
you run:

```bash
make docker-push
```

Watchtower then pulls the new image and restarts the containers. Until that
happens, a node may still show old behavior even though the repo is already
fixed.

Example: if a new peer starts and exposes its P2P port but
`http://127.0.0.1:8080/health` does not answer, check whether the current Docker
Hub image includes the latest runtime changes. Push the image, wait for
Watchtower, then verify `/health`, `/chain/status`, and `/chain/bootstrap`
again.

## Safe Rule

For normal blockchain updates: change repo files, test, build and push Docker,
then let nodes update themselves. Only touch Oracle nodes directly for first
setup, env/port changes, emergency rollback, or infrastructure maintenance.
