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

If the VM only has legacy `docker-compose` and it conflicts with Docker 29,
remove the old container and start from the same env file with `docker run`.

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

Nodes fetch `https://pokoin.com/bootstrap-peers.json` first and fall back to
`POKOINPOS_BOOTSTRAP_PEERS` if the manifest is unavailable.

## Verify Rollout

Check Docker Hub and then each node health endpoint:

```bash
docker buildx imagetools inspect newisdom/pokoinpos-peer:latest
curl -fsS https://rpc.pokoin.com/health
curl -fsS https://rpc.pokoin.com/chain/status
curl -fsS https://rpc.pokoin.com/chain/bootstrap
```

On a VM:

```bash
docker ps
docker logs --tail=100 pokoinpos-peer
curl -fsS http://127.0.0.1:8080/health
curl -fsS http://127.0.0.1:8080/chain/status
curl -fsS http://127.0.0.1:8080/chain/bootstrap
```

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
