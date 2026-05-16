# pokoinpos-peer

Permissioned Proof-of-Stake peer node for the PokoinPOS network.

## Image

- `newisdom/pokoinpos-peer:latest`
- Current published multi-arch digest after the latest runtime update:
  `sha256:3ae5947d7552ec06384a699b1383a6a3d0e21a37dbe765113416f85a52c72715`

## Deployment model

- A peer can run as a **seed node** (`JOIN_PORT=-1`) or as a **joining node** (`JOIN_HOST/JOIN_PORT` set).
- Each peer must use a **unique** `POKOINPOS_LISTEN_PORT`.
- The node includes:
  - automatic reconnect attempts to known peers, dynamic manifest peers, and static seed peers
  - persistent local state (chain, miner identity, last slot) mounted under `/data`
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

## Prerequisites

1. Docker Engine + Docker Compose.
2. A reachable seed node IP or DNS name.
3. Network rules allowing peer-to-peer TCP traffic.
4. Optional: a strong `POKOINPOS_OPERATOR_TOKEN` if you want guarded operator actions.

## Step 1: Create environment file

Use one env file per peer:

```bash
cat > peer2.env <<'EOF'
PEER_NAME=pokoinpos-peer2
POKOINPOS_LISTEN_PORT=43001
POKOINPOS_OPS_PORT=8081
POKOINPOS_BOOTSTRAP_MANIFEST_URL=https://pokoin.com/bootstrap-peers.json
POKOINPOS_BOOTSTRAP_REFRESH_INTERVAL_HOURS=24
POKOINPOS_BOOTSTRAP_PEERS=92.5.153.117:43000,130.162.242.213:43001
POKOINPOS_JOIN_HOST=92.5.153.117
POKOINPOS_JOIN_PORT=43000
POKOINPOS_OPERATOR_TOKEN=
POKOINPOS_REWARD_PAYOUT_ADDRESS=
POKOINPOS_SLOT_SECONDS=1
POKOINPOS_IDLE_SLOT_INTERVAL=30
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

```bash
docker compose --env-file peer2.env -f docker-compose.peer.yml up -d --build
```

Automatic rollout:

- This compose bundle includes a Watchtower updater service.
- When a newer `newisdom/pokoinpos-peer` image is published, peers auto-pull and restart.
- Poll interval is controlled by `POKOINPOS_UPDATE_INTERVAL_SECONDS`.
- For the full code-to-Docker-to-node update workflow, see
  `docs/blockchain-update-workflow.md`.

Adaptive mining:

- `POKOINPOS_SLOT_SECONDS` controls how often the node checks whether work is needed.
- `POKOINPOS_IDLE_SLOT_INTERVAL=30` means an idle node mines only a keepalive block every 30 slots.
- When transactions enter the mempool, the node mines immediately and scales attempts with `POKOINPOS_MINE_ATTEMPTS_PER_PENDING_TX`.
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
curl -fsS http://127.0.0.1:8081/chain/status
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

Nodes fetch `https://pokoin.com/bootstrap-peers.json` first, then fall back to
`POKOINPOS_BOOTSTRAP_PEERS` if the manifest cannot be reached. A candidate node
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
