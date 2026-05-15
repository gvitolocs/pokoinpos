# pokoinpos-peer

Permissioned Proof-of-Stake peer node for the PokoinPOS network.

## Image

- `newisdom/pokoinpos-peer:latest`
- `newisdom/pokoinpos-peer:v0.1.1` (recommended)
- `newisdom/pokoinpos-peer:v0.1.0` (legacy)

## Deployment model

- A peer can run as a **seed node** (`JOIN_PORT=-1`) or as a **joining node** (`JOIN_HOST/JOIN_PORT` set).
- Each peer must use a **unique** `POKOINPOS_LISTEN_PORT`.
- The node (v0.1.1+) includes:
  - automatic reconnect attempts to seed peer
  - persistent local state (chain, miner identity, last slot) mounted under `/data`

## Prerequisites

1. Docker Engine + Docker Compose.
2. A reachable seed node IP or DNS name.
3. Network rules allowing peer-to-peer TCP traffic.
4. A strong `POKOINPOS_ADMIN_TOKEN`.

## Step 1: Create environment file

Use one env file per peer:

```bash
cat > peer2.env <<'EOF'
PEER_NAME=pokoinpos-peer2
POKOINPOS_LISTEN_PORT=43001
POKOINPOS_OPS_PORT=8081
POKOINPOS_JOIN_HOST=92.5.153.117
POKOINPOS_JOIN_PORT=43000
POKOINPOS_ADMIN_TOKEN=replace-with-long-random-token
POKOINPOS_SLOT_SECONDS=1
POKOINPOS_GENESIS_HARDNESS=10000
POKOINPOS_GENESIS_SEED=42
POKOINPOS_INITIAL_BALANCE=1000000
POKOINPOS_STATE_HOST_PATH=./.pokoinpos-peer2-state
POKOINPOS_STATE_SAVE_INTERVAL_SECONDS=15
POKOINPOS_RECONNECT_INTERVAL_SECONDS=5
EOF
```

## Step 2: Launch with one command

```bash
docker compose --env-file peer2.env -f docker-compose.peer.yml up -d --build
```

## Step 3: Verify health

```bash
docker compose --env-file peer2.env -f docker-compose.peer.yml ps
curl -fsS http://127.0.0.1:8081/health
curl -fsS http://127.0.0.1:8081/chain/status
```

## Network requirements (critical)

### Cloud VM (Oracle/AWS/GCP/Azure)

Open inbound on the VM host and cloud firewall:

- `POKOINPOS_LISTEN_PORT` (p2p, required)
- `POKOINPOS_OPS_PORT` (optional externally; recommended internal-only)

Recommended source restrictions:

- P2P port: allow only known peer CIDRs/IPs.
- Ops port: allow only operator IPs or private network.

### Home/lab router deployments

If running behind NAT router:

1. Set static LAN IP for host.
2. Configure router port forwarding:
   - WAN `POKOINPOS_LISTEN_PORT` -> host `POKOINPOS_LISTEN_PORT`
3. Use your public IP (or dynamic DNS) as `POKOINPOS_JOIN_HOST`.
4. If ISP uses CGNAT, inbound forwarding may fail; use a VPS/Cloud seed node.

Dynamic DNS option (recommended for home IP changes):

- DuckDNS Docker guide: [LinuxServer DuckDNS docs](https://docs.linuxserver.io/images/docker-duckdns/)
- DuckDNS Docker image: [linuxserver/duckdns on Docker Hub](https://hub.docker.com/r/linuxserver/duckdns/)

## Security checklist

- Set a strong `POKOINPOS_ADMIN_TOKEN`; rotate it when leaked, shared, or after operator changes.
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

## Minimal seed node example

```bash
docker run -d --name pokoin-seed \
  -e POKOINPOS_RUN_MODE=node \
  -e POKOINPOS_LISTEN_PORT=43000 \
  -e POKOINPOS_JOIN_PORT=-1 \
  -e POKOINPOS_ADMIN_TOKEN=replace-with-long-random-token \
  -v "$(pwd)/.pokoin-seed-state:/data" \
  -p 43000:43000 \
  -p 8080:8080 \
  newisdom/pokoinpos-peer:v0.1.1
```
