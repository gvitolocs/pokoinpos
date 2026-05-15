# pokoinpos-peer

Permissioned Proof-of-Stake peer node for the PokoinPOS network.

## Image

- `newisdom/pokoinpos-peer:latest`

## Deployment model

- A peer can run as a **seed node** (`JOIN_PORT=-1`) or as a **joining node** (`JOIN_HOST/JOIN_PORT` set).
- Each peer must use a **unique** `POKOINPOS_LISTEN_PORT`.
- The node includes:
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

If a peer runs at home, the only port that must be reachable from the internet is the **P2P port**:

- Forward **TCP `POKOINPOS_LISTEN_PORT`**
- Example seed node: forward **TCP `43000`**
- Example second peer: forward **TCP `43001`**
- Do **not** expose the ops port (`POKOINPOS_OPS_PORT`, usually `8080`) unless you intentionally want remote admin/monitoring access.

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
  newisdom/pokoinpos-peer:latest
```
