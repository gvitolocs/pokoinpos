# PokoinPoS Explorer API

Base URL:

```text
https://rpc.pokoin.com
```

The explorer API exposes canonical best-chain data for public explorers, indexers, and registry validation.

## Blocks

### `GET /explorer/blocks`

Query parameters:

- `limit`: number of blocks to return, between `1` and `100`.
- `cursor`: block number to start from. Defaults to latest.

Response fields:

- `blocks`: newest-first block summaries.
- `nextCursor`: next block number for pagination.
- `latestHeight`: current best-chain height.
- `finalizedHeight`: height finalized by the configured finality depth.

### `GET /explorer/blocks/{heightOrHash}`

Returns a full canonical block with transactions.

## Transactions

### `GET /explorer/tx/{hash}`

Returns canonical transaction details:

- `hash`
- `from`
- `to`
- `amount`
- `nonce`
- `blockHash`
- `blockNumber`
- `transactionIndex`
- `finalized`

## Addresses

### `GET /explorer/address/{address}`

Returns balance, nonce, transaction count, and recent transactions.

### `GET /explorer/address/{address}/txs`

Query parameters:

- `limit`: number of transactions to return, between `1` and `100`.
- `cursor`: zero-based transaction cursor.

## Search

### `GET /explorer/search?q={query}`

Searches by:

- block number,
- block hash,
- transaction hash,
- address.

## EVM JSON-RPC Explorer Methods

The `/rpc` endpoint also supports explorer-oriented methods:

- `eth_getBlockTransactionCountByNumber`
- `eth_getBlockTransactionCountByHash`
- `eth_getTransactionByBlockNumberAndIndex`
- `eth_getTransactionByBlockHashAndIndex`
