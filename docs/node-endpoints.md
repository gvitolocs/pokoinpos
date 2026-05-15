# PokoinPoS Node Endpoints

This document lists every HTTP endpoint exposed by the PokoinPoS node runtime.

Default local base URL:

```text
http://127.0.0.1:8080
```

Inside Docker the node listens on `:8080`. `POKOINPOS_OPS_PORT` controls the host port, so examples may use `8081`, `4000`, or any other free local port.

## Public Endpoints

### `GET /health`

Compact liveness and node status endpoint for website health pages, uptime checks, and load balancer probes.

Authentication: none.

Response fields:

- `status`: node liveness status, currently `ok`.
- `currencySymbol`: native currency symbol, currently `PKN`.
- `peerCount`: number of connected peers.
- `chainHeight`: current best-chain height.
- `committedHeight`: height considered finalized after `finalityDepth`.
- `finalityDepth`: number of tentative tip blocks excluded from committed state.
- `mempoolDepth`: number of pending transactions.
- `validatorStake`: this node miner's current spendable PKN balance used for weighted mining and payout.
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

- `currencySymbol`: native currency symbol, currently `PKN`.
- `height`: current best-chain height.
- `committedHeight`: height considered finalized after `finalityDepth`.
- `finalityDepth`: number of tentative tip blocks excluded from committed state.
- `peerCount`: number of connected peers.
- `mempoolDepth`: number of pending transactions.
- `validatorStake`: this node miner's current spendable PKN balance used for weighted mining and payout.
- `txCount`: transaction count in the committed ledger.
- `acceptedBlocks`: total accepted blocks.
- `minedBlocks`: blocks mined by this node.
- `uptimeSeconds`: node runtime uptime in seconds.

Example:

```bash
curl http://127.0.0.1:8080/chain/status
```

### `GET /chain/validators`

Dynamic validator list derived from connected peers and advertised validator identity.

Authentication: none.

Response fields:

- `validators`: list of known peer validator identities.

Each validator entry includes:

- `peerId`: network peer ID, currently the listen port identifier.
- `validator`: advertised validator account/public key.
- `stake`: current best-chain spendable PKN balance for that validator.
- `authorized`: true when the peer has advertised a validator identity through P2P.
- `local`: whether the row is this node.
- `connected`: whether the row is a currently connected remote peer.

Example:

```bash
curl http://127.0.0.1:8080/chain/validators
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
- `pokoinpos_validator_stake`
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
- `currency`: native currency symbol, currently `PKN`.
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

### `GET /dashboard`

Local node-host dashboard for operators running a PokoinPoS node.

Authentication: none for read-only cards. Admin actions require:

```text
Authorization: Bearer <POKOINPOS_ADMIN_TOKEN>
```

The dashboard shows health, readiness, chain status, peer count, mempool depth, Prometheus metrics links, and guarded admin actions. The typo path `/deshboard` is also accepted for convenience.

Example:

```bash
open http://127.0.0.1:4000/dashboard
```

Use `http://127.0.0.1:8080/dashboard` when running with the default ops port. The `4000` URL is just an example if you set `POKOINPOS_OPS_PORT=4000`.

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
See `docs/explorer-api.md` for block explorer endpoints.

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

### `GET /admin/dashboard/status`

Protected capability check used by the local dashboard.

Authentication:

```text
Authorization: Bearer <POKOINPOS_ADMIN_TOKEN>
```

Response fields:

- `adminEnabled`: whether admin actions are available for the supplied token.
- `actions`: enabled dashboard admin actions.
- `chainId`: EVM-compatible chain ID.
- `networkId`: EVM-compatible network ID.

Example:

```bash
curl -H "Authorization: Bearer ${POKOINPOS_ADMIN_TOKEN}" \
  http://127.0.0.1:8080/admin/dashboard/status
```

### `POST /admin/withdraw`

Withdraws this validator node's spendable PKN balance to a payout EVM wallet.

Authentication:

```text
Authorization: Bearer <POKOINPOS_ADMIN_TOKEN>
```

Request body:

```json
{"to":"0x1111111111111111111111111111111111111111","amount":100}
```

Response fields:

- `hash`: native transaction hash/id.
- `to`: payout EVM wallet.
- `amount`: withdrawn PKN amount.

Validator authorization is separate from spendable balance. A validator can withdraw all spendable PKN and remain authorized as long as it continues participating as a P2P node.

## Website Health Page Integration

For a public site, prefer these calls:

- Use `/health` for a simple green/red health card.
- Use `/chain/status` for chain height, committed height, peer count, uptime, and currency.
- Use `/chain/validators` to show the dynamic peer/validator list and payout balances.
- Use `/endpoints` to render a live list of available node endpoints.

Do not expose `POKOINPOS_ADMIN_TOKEN` in frontend code. If your site needs admin actions, call them through a private backend route.

## Adaptive Mining Notes

The node does not need to mine a new block every slot when there are no transactions. `POKOINPOS_IDLE_SLOT_INTERVAL` controls idle keepalive blocks:

- `30` means mine one idle keepalive block every 30 slots.
- `0` disables idle backoff and preserves the old behavior.
- Pending transactions always bypass idle backoff and trigger mining immediately.

Validator eligibility is based on P2P participation with a valid miner identity. Spendable balance affects mining weight, but does not decide whether a validator can mine. Validators with positive balance share 97% of lottery weight proportionally to balance; all zero-balance validators share the remaining 3% cumulatively. Rewards can be withdrawn automatically each month to a configured payout `0x` wallet.
