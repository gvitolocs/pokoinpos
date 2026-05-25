# PokoinPoS Docker Hub overview

`newisdom/pokoinpos-peer` is the Docker image for running a PokoinPoS peer node.
It is the operational package behind the public permissioned Proof-of-Stake
network: peer discovery, chain execution, validator participation, local ops
endpoints and RPC serving.

## Image

- `newisdom/pokoinpos-peer:latest`
- Current published multi-arch digest after the latest runtime update:
  `sha256:e255b36aa0c103f41e7d392dc78db83dc143b538c0f9d00e0927b0871752b8b6`
- Supported architecture model: multi-arch Docker Hub image for cloud VMs and
  Linux hosts.

## Network model

PokoinPoS has three operational layers:

- **Users** hold site balances, link EVM wallets and send PKN by username.
- **Peers** discover each other through the bootstrap manifest and exchange
  chain state.
- **Validators** are approved staking nodes that produce blocks inside the
  permissioned PoS network.

The public bootstrap manifest is the default source of network configuration:

```text
https://pokoin.com/bootstrap-peers.json
```

It supplies default join/fallback peers, refresh interval, EVM chain ID and EVM
network ID. New env files should normally keep only node-specific values such as
peer name, advertised host, P2P port, ops port and state path.

## Deployment model

- A peer joins the network using the bootstrap manifest unless explicit
  `POKOINPOS_JOIN_HOST` / `POKOINPOS_JOIN_PORT` overrides are set.
- Each peer must use a unique `POKOINPOS_LISTEN_PORT`.
- `POKOINPOS_ADVERTISE_HOST` must be reachable by other peers.
- The node includes:
  - automatic reconnect attempts to known peers, dynamic manifest peers, and static seed peers
  - persistent local state (chain, miner identity, last slot) mounted under `/data`
- The compose bundle includes Watchtower. When a newer Docker Hub image is
  published, peers can auto-pull and restart.
- Docker peers are responsible for chain execution, P2P, local ops endpoints, and
  RPC APIs. They are not responsible for hosting the public website, public
  explorer frontend, token icons, banners, or static metadata.

## Current Oracle node map

| Role | Host | P2P port | Ops host port | Status |
| --- | --- | ---: | ---: | --- |
| Grandfathered bootstrap | `92.5.153.117` | `43000` | `8080` | bootstrap |
| Grandfathered bootstrap | `130.162.242.213` | `43001` | `8080` | bootstrap |
| Vetting peer | `141.147.62.244` | `43000` | `8080` | vetting |
| Vetting peer | `92.5.23.133` | `43001` | `8081` | vetting |

The two bootstrap nodes are treated as already mature. New candidates must pass
the 14-day vetting period and then remain regular peers until the 365-day
bootstrap maturity rule is met.

## Public service architecture

Pokoin uses two hosting layers:

- **Vercel frontend/static layer**
  - `https://pokoin.com`
  - `https://pokoin.com/scan`
  - `https://pokoin.com/health`
  - `https://explorer.pokoin.com`
  - `https://explorer.pokoin.com/wpkn/logo.png`
  - `https://explorer.pokoin.com/wpkn-reserve.json`
- **Oracle/Docker node layer**
  - P2P listener, for example TCP `43000`
  - node-local ops/API port, for example `8080`
  - public RPC gateway: `https://rpc.pokoin.com/rpc`

`explorer.pokoin.com` should point to Vercel, not to the VM. Cloudflare can use
Vercel's apex A record target (`76.76.21.21`) or Vercel-managed CNAME records,
depending on the DNS zone shape. `rpc.pokoin.com` remains the node-backed
hostname because wallets and the explorer need live chain data.

## Marketplace database redundancy

The CardVault marketplace catalog uses a separate PostgreSQL database from the
peer blockchain state. Peer4 remains the writable marketplace Postgres primary,
and peer3 can run a hot standby replica for failover.

- Primary: peer4 `pokoin-marketplace-postgres`.
- Fallback: peer3 `pokoin-marketplace-postgres-replica`.
- Sync model: PostgreSQL physical WAL streaming from peer4 to peer3.
- Normal updates: run schema imports, marketplace refreshes, and API writes
  against peer4 only. Peer3 receives the same WAL stream automatically.
- Failure mode: promote peer3 with
  `deploy/scripts/postgres-failover-promote.sh`, update
  `MARKETPLACE_DATABASE_URL` to peer3, and re-seed peer4 before letting it serve
  database traffic again.

Streaming replication is not a backup replacement. Keep encrypted backups
enabled because bad migrations, accidental deletes, and corrupted writes also
replicate to peer3.

## Prerequisites

1. Docker Engine + Docker Compose.
2. A reachable public host/IP for your own peer advertisement.
3. Network rules allowing peer-to-peer TCP traffic.
4. Optional: a strong `POKOINPOS_OPERATOR_TOKEN` if you want guarded operator actions.

## Step 1: Create environment file

Use one env file per peer. Prefer copying the template:

```bash
cp deploy/env/peer.env.example deploy/env/my-peer.env
nano deploy/env/my-peer.env
```

Minimal example:

```bash
cat > peer2.env <<'EOF'
PEER_NAME=pokoinpos-peer2
POKOINPOS_LISTEN_PORT=43001
POKOINPOS_ADVERTISE_HOST=replace-with-your-public-ip-or-dns
POKOINPOS_OPS_PORT=8081
POKOINPOS_BOOTSTRAP_MANIFEST_URL=https://pokoin.com/bootstrap-peers.json
POKOINPOS_OPERATOR_TOKEN=
POKOINPOS_REWARD_PAYOUT_ADDRESS=
POKOINPOS_SLOT_SECONDS=1
POKOINPOS_IDLE_SLOT_INTERVAL=300
POKOINPOS_GENESIS_HARDNESS=10000
POKOINPOS_GENESIS_SEED=42
POKOINPOS_INITIAL_BALANCE=1000000
POKOINPOS_STATE_HOST_PATH=./.pokoinpos-peer2-state
POKOINPOS_STATE_SAVE_INTERVAL_SECONDS=15
POKOINPOS_RECONNECT_INTERVAL_SECONDS=5
POKOINPOS_AUTO_UPDATE=true
POKOINPOS_UPDATE_INTERVAL_SECONDS=60
EOF
```

## Step 2: Launch with one command

Recommended helper:

```bash
chmod +x deploy/scripts/docker-peer-up.sh
./deploy/scripts/docker-peer-up.sh deploy/env/my-peer.env
```

Equivalent Docker Compose command:

```bash
docker compose --env-file peer2.env -f docker-compose.peer.yml up -d --remove-orphans
```

Automatic rollout:

- This compose bundle includes a Watchtower updater service.
- When a newer `newisdom/pokoinpos-peer` image is published, peers auto-pull and restart.
- Poll interval is controlled by `POKOINPOS_UPDATE_INTERVAL_SECONDS`.
- For the full code-to-Docker-to-node update workflow, see
  `docs/blockchain-update-workflow.md`.
- For divergent chain recovery, late-joiner resets, genesis metadata checks, and
  legacy `docker-compose` handling, use the operational runbook in
  `docs/blockchain-update-workflow.md`.

Adaptive mining:

- `POKOINPOS_SLOT_SECONDS` controls how often the node checks whether work is needed.
- `POKOINPOS_IDLE_SLOT_INTERVAL=300` means an idle node mines only a keepalive block every 300 slots.
- When transactions enter the mempool, the node mines immediately and scales attempts with `POKOINPOS_MINE_ATTEMPTS_PER_PENDING_TX`. Keep this low in production; the default is `1`, with `POKOINPOS_MAX_MINE_ATTEMPTS_PER_TICK=5`, so a stale mempool cannot pin a CPU core.
- Set `POKOINPOS_IDLE_SLOT_INTERVAL=0` only if you want the old behavior of trying to mine every idle slot.
- Any node that joins the P2P network with a valid miner identity is a validator, even with zero spendable PKN balance.
- Validator balance still affects lottery weight: positive-balance validators share 97% proportionally, while all zero-balance validators share 3% cumulatively.
- The dynamic validator list is available at `/chain/validators`.
- `POKOINPOS_REWARD_PAYOUT_ADDRESS` is optional. If set to a `0x...` wallet, the node queues a monthly payout of its spendable validator rewards to that wallet.
- Leaving `POKOINPOS_REWARD_PAYOUT_ADDRESS` empty disables automatic payouts.
- Validator spendable rewards can also be withdrawn manually with `/admin/withdraw` or the local dashboard.

## Step 3: Verify health

```bash
docker compose --env-file peer2.env -f docker-compose.peer.yml ps
curl -fsS http://127.0.0.1:8081/health
curl -fsS http://127.0.0.1:8081/ready
curl -fsS http://127.0.0.1:8081/chain/status
curl -fsS http://127.0.0.1:8081/chain/bootstrap
```

If the P2P port is open but `/health` does not respond, the VM may still be
running an older Docker Hub image. Repository fixes are not live until
`newisdom/pokoinpos-peer:latest` is rebuilt and pushed. Run `make docker-push`,
wait for Watchtower to restart the node, then check `/health` again.

The local node-host dashboard is served from the same ops port:

```text
http://127.0.0.1:8081/dashboard
```

If you want the dashboard at `http://localhost:4000/dashboard`, set `POKOINPOS_OPS_PORT=4000` in your env file. The container still listens on `:8080`; `POKOINPOS_OPS_PORT` only controls the host port.

## Bootstrap registry policy

Nodes fetch `https://pokoin.com/bootstrap-peers.json` first. The manifest carries
the default join peer, fallback peers, refresh interval, EVM chain ID, and EVM
network ID, so new peer env files should only keep node-specific values unless
you intentionally need a local override. A candidate node
must complete a 14-day vetting stage with at least 95% uptime. After that it is
only a regular peer until it is 365 days old; then it can enter the annual public
bootstrap list if it also has at least 94% observed uptime over the last 365
days. Uptime must be observed by at least 3 other peers; a node cannot certify
itself. The current Oracle seeds are grandfathered bootstrap peers and remain
static fallback peers.

Operators can inspect local discovery health at:

```bash
curl -fsS http://127.0.0.1:8081/chain/bootstrap
```

## Wallet and RPC behavior

- Native PKN is served by PokoinPoS RPC at `https://rpc.pokoin.com/rpc`.
- The public explorer and wallet UI use Vercel, but live chain data comes from
  the node-backed RPC gateway.
- MetaMask can add PokoinPoS as an EVM-compatible network with chain data from
  the bootstrap/public configuration.
- Site account balances are separate from connected wallet balances. Username
  transfers are instant site-balance moves; linked wallets are used for native
  PKN visibility, top-ups and withdrawals.

## Network requirements (critical)

### Cloud VM (Oracle/AWS/GCP/Azure)

Open inbound on the VM host and cloud firewall:

- `POKOINPOS_LISTEN_PORT` (p2p, required)
- `POKOINPOS_OPS_PORT` (optional externally; recommended internal-only; default Compose host port is `8080`, but each peer should use a unique host port)

Recommended source restrictions:

- P2P port: allow only known peer CIDRs/IPs.
- Ops port: allow only operator IPs or private network.

### Home/lab router deployments

If a peer runs at home, the only port that must be reachable from the internet is the **P2P port**:

- Forward **TCP `POKOINPOS_LISTEN_PORT`**
- Example seed node: forward **TCP `43000`**
- Example second peer: forward **TCP `43001`**
- Do **not** expose the ops port (`POKOINPOS_OPS_PORT`, often `8080`, `8081`, or your chosen dashboard port) unless you intentionally want remote admin/monitoring access.

Step-by-step:

1. Give the Docker host a fixed LAN IP.
   - Example: `192.168.1.50`
2. Open your router admin page.
   - Usually `192.168.1.1` or `192.168.0.1`
3. Find **Port Forwarding**, **NAT**, **Virtual Server**, or **Applications & Gaming**.
4. Add one TCP rule:

| Field | Example value |
| --- | --- |
| Service name | `pokoinpos-peer` |
| Protocol | `TCP` |
| External/WAN port | `43000` |
| Internal/LAN IP | `192.168.1.50` |
| Internal/LAN port | `43000` |

5. Allow the same TCP port on the host firewall:

```bash
sudo ufw allow 43000/tcp
```

6. Other peers should join using your public IP or DNS name:

```env
POKOINPOS_JOIN_HOST=your-name.duckdns.org
POKOINPOS_JOIN_PORT=43000
```

7. Test from outside your home network:

```bash
nc -zv your-name.duckdns.org 43000
```

Dynamic DNS with DuckDNS:

1. Create a DuckDNS subdomain, for example `your-name.duckdns.org`.
2. Run a DuckDNS updater so the DNS name follows your changing home IP.
3. Put that DNS name in `POKOINPOS_JOIN_HOST`.

Simple references:

- Router port-forwarding basics: [PortForward.com guide](https://portforward.com/how-to-port-forward)
- DuckDNS official setup: [DuckDNS install page](https://www.duckdns.org/install.jsp)
- DuckDNS update API: [DuckDNS API/update spec](https://duckdns.org/spec.jsp)
- DuckDNS Docker container: [LinuxServer DuckDNS docs](https://docs.linuxserver.io/images/docker-duckdns/)
- DuckDNS image: [linuxserver/duckdns on Docker Hub](https://hub.docker.com/r/linuxserver/duckdns/)

If the test fails even after forwarding, your ISP may use CGNAT. In that case, use a VPS/cloud seed node instead of a home seed node.

## Security checklist

- Leave `POKOINPOS_OPERATOR_TOKEN` empty for ordinary public peers. If enabled,
  set a strong token and rotate it when leaked, shared, or after operator changes.
- Do not expose ops port publicly unless required.
- Keep Docker host patched.
- Use least-privilege cloud firewall rules.
- Store state volume on durable disk and back it up.

## What happens if a peer disconnects?

- The node keeps running and mining local slots.
- Reconnect loop retries seed connection every `POKOINPOS_RECONNECT_INTERVAL_SECONDS`.
- On restart, node restores chain state and miner identity from `/data`.
- If network is partitioned, nodes can diverge temporarily; they reconcile when connectivity returns.

## Troubleshooting

1. `peerCount` remains `0`:
   - verify `POKOINPOS_JOIN_HOST`/`POKOINPOS_JOIN_PORT`
   - test TCP reachability: `nc -zv <join_host> <join_port>`
2. Health endpoint timeout:
   - container not running or ops port not mapped
3. Repeated cold starts:
   - verify `POKOINPOS_STATE_HOST_PATH` exists and is writable
4. Public connectivity fails:
   - check cloud NSG/Security List + host firewall + router forwarding
5. `GET /chain/validators`, `/ready`, or `/metrics` returns `method_not_allowed` on `rpc.pokoin.com`:
   - check the reverse proxy is not rewriting non-`/rpc` paths into `/rpc`
   - proxy the whole host to the node ops port, for example with `deploy/caddy/Caddyfile.example`

## Minimal seed node example

```bash
docker run -d --name pokoin-seed \
  -e POKOINPOS_RUN_MODE=node \
  -e POKOINPOS_LISTEN_PORT=43000 \
  -e POKOINPOS_JOIN_PORT=-1 \
  -e POKOINPOS_OPERATOR_TOKEN= \
  -v "$(pwd)/.pokoin-seed-state:/data" \
  -p 43000:43000 \
  -p 8080:8080 \
  newisdom/pokoinpos-peer:latest
```
