# PokoinSwap Native AMM

PokoinSwap is a native AMM inside PokoinPoS. It is not Uniswap contracts and does not require a full EVM. Validators replay pool creation, liquidity, internal asset accounting, and swaps directly from PokoinPoS blocks.

## Current Scope

PokoinSwap has two related flows:

1. native PokoinPoS AMM swaps, starting with `PKN-WPKN`;
2. CardVault crypto-to-PKN purchases, where external crypto deposits are verified
   by the web backend and credited to the user's site PKN balance;
3. CardVault PKN-to-external-crypto sales, where a native PokoinPoS PKN
   transfer is confirmed first and outbound crypto payout is gated by liquidity.

The first AMM pool target is `PKN-WPKN`:

- `PKN` uses the existing native account balance ledger.
- `wPKN` is an internal accounting asset used for bridge/reserve workflows.
- Operators create pools, credit/debit internal assets, and add liquidity.
- Public users swap with wallet-signed `eth_sendRawTransaction` transactions.

## Web Wallet UI

The public PokoinSwap screen is implemented in the CardVault web app at
`lib/wallet/main.dart` and is served from `https://pokoin.com/wallet`. Keep the
screen as a focused exchange surface:

- Default state: `Sell ETH` and `Buy PKN`.
- A compact swap card with `Sell` and `Buy` amount panels.
- Token selector pills and a central flip action for changing direction.
- Quote details for rate, fee, pool, and reserves.
- One primary signed-swap action.

When a live native AMM pool exists, the wallet uses the PokoinPoS quote API and
submits to the native router. When no native pool exists but the selected asset
is an approved external EVM asset, the wallet uses the CardVault
crypto-to-PKN purchase API instead. For the reverse direction, `Sell PKN` /
`Buy BTC`, `ETH`, `BNB`, or supported tokens can be quoted and recorded before
pool liquidity exists, but automatic payout must stay disabled until the
settlement wallets or pools are funded.

Approved external bridge assets such as `BTC`, `ETH`, `BNB`, and supported
ERC-20s use market-priced bridge quotes, not experimental AMM pools, unless the
asset is the native PokoinSwap target such as `WPKN`.

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

## Crypto-To-PKN Purchase Flow

CardVault supports external crypto deposits for buying PKN into the user's site
balance. This is not a Uniswap/PancakeSwap fork; it borrows the familiar swap
interaction while settlement is handled by authenticated backend verification.

Reference price:

```text
1 PKN = 0.005 USDT
```

Supported auto-verified assets:

- BTC on Bitcoin.
- Native ETH on Ethereum.
- Native BNB on BNB Chain.
- ERC-20 `USDT`, `USDC`, `DAI`, `CAKE` on BNB Chain.
- ERC-20 `LINK`, `UNI` on Ethereum.

The wallet flow is:

1. user opens `https://pokoin.com/wallet`;
2. swap card defaults to `Sell ETH` / `Buy PKN`;
3. wallet requests an authenticated quote from CardVault:
   `POST /api/crypto-pkn-purchase/quote`;
4. backend stores the quote with expiry and returns settlement details;
5. wallet asks MetaMask to send EVM native/ERC-20 assets to the configured
   settlement wallet, or shows the Bitcoin settlement address for BTC deposits;
6. wallet submits the resulting deposit tx hash to:
   `POST /api/crypto-pkn-purchase/request`;
7. backend verifies chain, sender, recipient, amount, quote ownership, quote
   expiry, and tx reuse;
8. backend credits `balances/{uid}.availablePkn` and writes ledger/deposit
   records.

The tx hash is the idempotency key. A deposit transaction can only be used once.

## PKN-To-Crypto Sale Flow

CardVault can also support the reverse bridge route:

```text
Sell PKN -> Buy BTC/ETH/BNB/USDT/USDC/DAI/LINK/UNI/CAKE
```

The wallet sends native PKN on PokoinPoS to the treasury first. The backend
verifies that transaction and, when payout signing is configured, broadcasts the
target crypto automatically. EVM assets use an EVM private key. BTC uses a
dedicated Bitcoin WIF hot-wallet key and Blockstream-compatible UTXO/broadcast
APIs.

Flow:

1. user selects `Sell PKN` and a target external crypto;
2. wallet requests `POST /api/crypto-pkn-sale/quote`;
3. backend prices PKN at `1 PKN = 0.005 USDT`, prices the target asset in USD,
   applies sell-side fees, checks payout wallet liquidity, and stores an
   expiring quote;
4. user provides a payout address for the target chain;
5. wallet switches to PokoinPoS and sends native PKN to the Pokoin treasury;
6. wallet calls `POST /api/crypto-pkn-sale/request` with the PKN transaction
   hash;
7. backend verifies quote ownership, expiry, linked wallet sender, treasury
   recipient, PKN amount, payout address, and idempotency;
8. when `CRYPTO_PKN_AUTO_PAYOUT_ENABLED` is true and a funded payout wallet is
   configured, backend broadcasts the outbound crypto transaction and records
   its tx hash;
9. if the signing key or liquidity is unavailable, the request stays
   `pending_liquidity` or `manual_settlement`.

Feature gates:

```text
CRYPTO_PKN_SELL_ENABLED=true
CRYPTO_PKN_AUTO_PAYOUT_ENABLED=true
CRYPTO_PKN_EVM_PAYOUT_PRIVATE_KEY=
CRYPTO_PKN_SELL_FEE_BPS=100
BITCOIN_PAYOUT_ADDRESS=
BITCOIN_PAYOUT_PRIVATE_KEY_WIF=
BITCOIN_NETWORK=mainnet
```

Recommended request states are `quoted`, `pending_liquidity`,
`manual_settlement`, `payout_submitted`, `completed`, and `failed`.

## Crypto-To-PKN API

Quote:

```bash
curl -X POST https://pokoin.com/api/crypto-pkn-purchase/quote \
  -H "Authorization: Bearer <FIREBASE_ID_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"asset":"ETH","amountIn":0.01}'
```

Request credit after the wallet sends the deposit:

```bash
curl -X POST https://pokoin.com/api/crypto-pkn-purchase/request \
  -H "Authorization: Bearer <FIREBASE_ID_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"quoteId":"<QUOTE_ID>","depositTxHash":"0x..."}'
```

Status:

```bash
curl https://pokoin.com/api/crypto-pkn-purchase/status \
  -H "Authorization: Bearer <FIREBASE_ID_TOKEN>"
```

## Crypto-To-PKN Environment

Required or recommended CardVault env vars:

```text
CRYPTO_PKN_SETTLEMENT_ADDRESS=0x74466c3a204429B22CE8558F3F18f3C59F67fCB3
CRYPTO_PKN_USDT_PRICE=0.005
CRYPTO_PKN_FEE_BPS=100
CRYPTO_PKN_SELL_ENABLED=true
CRYPTO_PKN_SELL_FEE_BPS=100
CRYPTO_PKN_AUTO_PAYOUT_ENABLED=true
CRYPTO_PKN_EVM_PAYOUT_PRIVATE_KEY=
CRYPTO_PKN_QUOTE_TTL_MS=60000
ETHEREUM_RPC_URL=https://ethereum.publicnode.com
BNB_RPC_URL=https://bsc-dataseed.binance.org
USDT_BNB_CONTRACT_ADDRESS=0x55d398326f99059fF775485246999027B3197955
USDC_BNB_CONTRACT_ADDRESS=0x8ac76a51cc950d9822d68b83fe1ad97b32cd580d
DAI_BNB_CONTRACT_ADDRESS=0x1af3f329e8be154074d8769d1ffa4ee058b1dbc3
LINK_ETH_CONTRACT_ADDRESS=0x514910771af9ca656af840dff83e8264ecf986ca
UNI_ETH_CONTRACT_ADDRESS=0x1f9840a85d5af5bf1d1762f925bdaddc4201f984
CAKE_BNB_CONTRACT_ADDRESS=0x0e09fabb73bd3ade0a17ecc321fd13a19e81ce82
BITCOIN_SETTLEMENT_ADDRESS=bc1q253wlm72m9s346y0jj4pcjey9xyn5wz9yxp8uf
BITCOIN_PAYOUT_ADDRESS=
BITCOIN_PAYOUT_PRIVATE_KEY_WIF=
BITCOIN_NETWORK=mainnet
BITCOIN_MIN_PAYOUT_BTC=0.00001
BITCOIN_MAX_PAYOUT_BTC=0.01
BITCOIN_FEE_RATE_SATS_PER_VBYTE=
BITCOIN_EXPLORER_API_URL=https://blockstream.info/api
BITCOIN_MIN_CONFIRMATIONS=1
```

Optional price override env vars can replace live market lookups:

```text
CRYPTO_PKN_ETH_USD_PRICE=
CRYPTO_PKN_BNB_USD_PRICE=
CRYPTO_PKN_BTC_USD_PRICE=
CRYPTO_PKN_LINK_USD_PRICE=
CRYPTO_PKN_UNI_USD_PRICE=
CRYPTO_PKN_CAKE_USD_PRICE=
```

## Implementation Files

CardVault:

- `api/crypto-pkn-purchase.js`: quote/request/status route.
- `api/_crypto_pkn_purchase.js`: pricing and deposit verification helper.
- `lib/wallet/main.dart`: swap UI and submission flow.
- `lib/wallet/auth_service.dart`: authenticated API client methods.
- `lib/wallet/wallet_bridge*.dart` and `web/index.html`: MetaMask native and
  ERC-20 transfer bridge.

PokoinPoS:

- native AMM runtime remains in the chain/node codebase;
- native swaps still use the PokoinSwap router address and `pokoinswap:v1:`
  calldata.
