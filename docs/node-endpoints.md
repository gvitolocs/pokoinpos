# PokoinPoS Node Endpoints

This document lists every HTTP endpoint exposed by the PokoinPoS node runtime.

Default local base URL:

```text
http://127.0.0.1:8080
```

Docker Compose examples may map the ops port to another host port, for example `8081`.

## Public Endpoints

### `GET /health`

Compact liveness and node status endpoint for website health pages, uptime checks, and load balancer probes.

Authentication: none.

Response fields:

- `status`: node liveness status, currently `ok`.
- `currencySymbol`: native currency symbol, currently `PK`.
- `peerCount`: number of connected peers.
- `chainHeight`: current best-chain height.
- `committedHeight`: height considered finalized after `finalityDepth`.
- `finalityDepth`: number of tentative tip blocks excluded from committed state.
- `mempoolDepth`: number of pending transactions.
- `acceptedBlocks`: total accepted blocks.
- `acceptedTxs`: total accepted transactions.

Example:

```bash
curl http://127.0.0.1:8080/health
```

### `GET /ready`

Readiness endpoint for container health checks and deployment gates.

Authentication: none.

Response fields:

- `status`: `ok` when ready, `initializing` otherwise.
- `ready`: boolean runtime readiness.

Returns `503 Service Unavailable` while the node is not fully initialized.

Example:

```bash
curl http://127.0.0.1:8080/ready
```

### `GET /chain/status`

Detailed chain status endpoint for a public health page, chain dashboard, or basic explorer status panel.

Authentication: none.

Response fields:

- `currencySymbol`: native currency symbol, currently `PK`.
- `height`: current best-chain height.
- `committedHeight`: height considered finalized after `finalityDepth`.
- `finalityDepth`: number of tentative tip blocks excluded from committed state.
- `peerCount`: number of connected peers.
- `mempoolDepth`: number of pending transactions.
- `txCount`: transaction count in the committed ledger.
- `acceptedBlocks`: total accepted blocks.
- `minedBlocks`: blocks mined by this node.
- `uptimeSeconds`: node runtime uptime in seconds.

Example:

```bash
curl http://127.0.0.1:8080/chain/status
```

### `GET /metrics`

Prometheus-compatible metrics endpoint for monitoring and alerting.

Authentication: none.

Content type:

```text
text/plain; version=0.0.4
```

Metrics:

- `pokoinpos_chain_height`
- `pokoinpos_committed_height`
- `pokoinpos_finality_depth`
- `pokoinpos_peer_count`
- `pokoinpos_mempool_depth`
- `pokoinpos_blocks_accepted_total`
- `pokoinpos_blocks_mined_total`
- `pokoinpos_transactions_accepted_total`
- `pokoinpos_ledger_transaction_count`
- `pokoinpos_uptime_seconds`

Example:

```bash
curl http://127.0.0.1:8080/metrics
```

### `GET /endpoints`

Machine-readable endpoint catalog intended for your website health page or documentation UI.

Authentication: none.

Response fields:

- `service`: service name, currently `pokoinpos-node`.
- `currency`: native currency symbol, currently `PK`.
- `endpoints`: list of endpoint metadata.

Each endpoint entry includes:

- `method`
- `path`
- `summary`
- `authentication`
- `contentType`
- `responseFields`
- `queryParams` when applicable
- `useCases`

Example:

```bash
curl http://127.0.0.1:8080/endpoints
```

### `POST /rpc`

EVM-style JSON-RPC compatibility endpoint for MetaMask-style wallets.

Authentication: none.

Supported wallet methods include:

- `web3_clientVersion`
- `net_version`
- `eth_chainId`
- `eth_blockNumber`
- `eth_syncing`
- `eth_accounts`
- `eth_requestAccounts`
- `eth_getBalance`
- `eth_getTransactionCount`
- `eth_getCode`
- `eth_getTransactionByHash`
- `eth_getTransactionReceipt`
- `eth_mining`
- `eth_hashrate`
- `eth_gasPrice`
- `eth_estimateGas`
- `eth_sendRawTransaction`

`eth_sendTransaction` returns a JSON-RPC error because PokoinPoS does not unlock node-held Ethereum accounts. Wallets should sign locally and submit through `eth_sendRawTransaction`.

- `eth_sendTransaction`

See `docs/wallet-compatibility.md` for MetaMask setup values.

Example:

```bash
curl -s http://127.0.0.1:8080/rpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

## Admin Endpoints

### `POST /admin/mine?slot=<n>`

Manually attempts mining for a specific slot. This endpoint is intended for controlled operations and smoke tests, not for public browser access.

Authentication:

```text
Authorization: Bearer <POKOINPOS_ADMIN_TOKEN>
```

Query parameters:

- `slot`: positive integer slot number.

Response fields:

- `slot`: requested slot number.
- `mined`: whether this node won and mined the slot.

Example:

```bash
curl -X POST \
  -H "Authorization: Bearer ${POKOINPOS_ADMIN_TOKEN}" \
  "http://127.0.0.1:8080/admin/mine?slot=1"
```

## Website Health Page Integration

For a public site, prefer these calls:

- Use `/health` for a simple green/red health card.
- Use `/chain/status` for chain height, committed height, peer count, uptime, and currency.
- Use `/endpoints` to render a live list of available node endpoints.

Do not expose `POKOINPOS_ADMIN_TOKEN` in frontend code. If your site needs admin actions, call them through a private backend route.
