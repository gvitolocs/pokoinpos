# PokoinSwap Native AMM

PokoinSwap is a native AMM inside PokoinPoS. It is not Uniswap contracts and does not require a full EVM. Validators replay pool creation, liquidity, internal asset accounting, and swaps directly from PokoinPoS blocks.

## Current Scope

The first pool target is `PKN-WPKN`:

- `PKN` uses the existing native account balance ledger.
- `wPKN` is an internal accounting asset used for bridge/reserve workflows.
- Operators create pools, credit/debit internal assets, and add liquidity.
- Public users swap with wallet-signed `eth_sendRawTransaction` transactions.

## Web Wallet UI

The public PokoinSwap screen is implemented in the CardVault web app at
`lib/wallet/main.dart` and is served from `https://pokoin.com/wallet`. Keep the
screen as a focused exchange surface:

- A compact swap card with `Sell` and `Buy` amount panels.
- Token selector pills and a central flip action for changing direction.
- Quote details for rate, fee, pool, and reserves.
- One primary signed-swap action.

Do not add marketing copy, decorative charts, or feature explainer cards to the
swap form. Those belong on landing/documentation pages, not inside the exchange
interaction.

## AMM Math

Quotes use deterministic integer constant-product math:

```text
amountInAfterFee = amountIn * (10000 - feeBps) / 10000
amountOut = reserveOut * amountInAfterFee / (reserveIn + amountInAfterFee)
```

The initial fee is `30` bps. Swaps are rejected if the quote is zero, if reserves would be drained, or if `amountOut < minAmountOut`.

## Public APIs

```bash
curl http://127.0.0.1:8080/chain/swap/pools
curl http://127.0.0.1:8080/chain/swap/pools/PKN-WPKN
curl http://127.0.0.1:8080/chain/swap/balances/0x1111111111111111111111111111111111111111
curl "http://127.0.0.1:8080/chain/swap/quote?pool=PKN-WPKN&assetIn=PKN&amountIn=100"
```

## Operator APIs

All operator endpoints require:

```text
Authorization: Bearer <POKOINPOS_OPERATOR_TOKEN>
```

Create pool:

```bash
curl -X POST http://127.0.0.1:8080/admin/swap/pools \
  -H "Authorization: Bearer ${POKOINPOS_OPERATOR_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"assetA":"PKN","assetB":"wPKN"}'
```

Credit internal `wPKN`:

```bash
curl -X POST http://127.0.0.1:8080/admin/assets/credit \
  -H "Authorization: Bearer ${POKOINPOS_OPERATOR_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"asset":"wPKN","account":"0x1111111111111111111111111111111111111111","amount":1000}'
```

Add liquidity:

```bash
curl -X POST http://127.0.0.1:8080/admin/swap/liquidity/add \
  -H "Authorization: Bearer ${POKOINPOS_OPERATOR_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"poolId":"PKN-WPKN","amountA":1000,"amountB":1000}'
```

## Wallet-Signed Swaps

Wallet-signed swaps use the existing JSON-RPC endpoint:

```text
POST /rpc
eth_sendRawTransaction
```

The raw Ethereum transaction must target the native PokoinSwap router address:

```text
0x0000000000000000000000000000000000002606
```

Its calldata is a PokoinPoS-native payload with prefix:

```text
pokoinswap:v1:
```

followed by JSON fields such as:

```json
{"action":"amm_swap","poolId":"PKN-WPKN","assetIn":"PKN","assetOut":"WPKN","amountIn":100,"minAmountOut":90}
```

The node recovers the signer, validates chain ID and nonce, then validators replay the AMM swap from block data.
