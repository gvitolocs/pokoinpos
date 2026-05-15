# API Endpoints

Base URL examples:

- Production: `https://api.pokoinpos.com/api/v1`
- Staging: `https://staging-api.pokoinpos.com/api/v1`

## Authentication

- Use `Authorization: Bearer <jwt>` for merchant and operator actions.
- Use API keys only for server-to-server integrations.
- Every write endpoint must enforce idempotency through `Idempotency-Key`.

## Health and metadata

### `GET /health`

Purpose:

- Liveness/readiness checks for load balancers and monitoring.

Response:

- `status`: `ok | degraded | down`
- `version`
- `chainStatus`: summary of RPC and indexer availability

### `GET /chains`

Purpose:

- Return supported chains, native currency, confirmation policy, and explorer links.

Response fields:

- `chainId`
- `name`
- `currencySymbol`
- `minConfirmations`
- `finalityType` (`probabilistic` or `instant`)

## Merchant and shop management

### `POST /merchants`

Purpose:

- Create a merchant account with legal profile and settlement defaults.

Body fields:

- `businessName`
- `country`
- `taxId` (optional, region dependent)
- `defaultSettlementCurrency` (fiat or stablecoin)

### `POST /shops`

Purpose:

- Register a physical or online shop under a merchant.

Body fields:

- `merchantId`
- `displayName`
- `timezone`
- `location` (optional for ecommerce)

### `POST /shops/{shopId}/terminals`

Purpose:

- Provision POS terminals and issue terminal credentials.

Body fields:

- `name`
- `deviceType`
- `allowedNetworks`

## Wallet management

### `POST /wallets/nonce`

Purpose:

- Create a one-time challenge nonce for wallet-signature authentication.

Body fields:

- `address`
- `chainId`

### `POST /wallets/verify`

Purpose:

- Verify signed nonce and bind wallet ownership to user or merchant profile.

Body fields:

- `address`
- `chainId`
- `nonce`
- `signature`

### `GET /wallets/{address}/balances`

Purpose:

- Return token balances and fiat conversion snapshot.

Query fields:

- `chainId`
- `includeTokens=true|false`

## Checkout and payments

### `POST /payments/quote`

Purpose:

- Generate a quote for cart amount, including conversion, network fee estimate, and expiry.

Body fields:

- `shopId`
- `terminalId`
- `amountFiat`
- `fiatCurrency`
- `targetAsset` (for example `USDC`)
- `chainId`

Response fields:

- `quoteId`
- `amountCrypto`
- `networkFeeEstimate`
- `expiresAt`

### `POST /payments/intents`

Purpose:

- Create a payment intent and reserve quote terms during TTL.

Body fields:

- `quoteId`
- `orderReference`
- `customerContext` (optional)

Response fields:

- `paymentIntentId`
- `paymentAddress` or `paymentRequest` (depending on flow)
- `expiresAt`

### `GET /payments/intents/{paymentIntentId}`

Purpose:

- Poll current payment state for POS screen updates.

States:

- `created`
- `awaiting_funds`
- `pending_confirmations`
- `confirmed`
- `failed`
- `expired`

### `POST /payments/intents/{paymentIntentId}/cancel`

Purpose:

- Cancel unpaid payment intent before expiration.

## Refunds and reversals

### `POST /refunds`

Purpose:

- Request full or partial refund for settled payments.

Body fields:

- `paymentIntentId`
- `amount` (optional for full refund)
- `reason`
- `destinationAddress`

### `GET /refunds/{refundId}`

Purpose:

- Track refund status and chain transaction metadata.

## Settlement and reconciliation

### `POST /settlements/run`

Purpose:

- Trigger merchant settlement batch (manual mode).

Body fields:

- `merchantId`
- `periodStart`
- `periodEnd`

### `GET /settlements/{settlementId}`

Purpose:

- Retrieve batch totals, fees, payout tx hash, and reconciliation links.

### `GET /reconciliation/transactions`

Purpose:

- Export transaction-level ledger for accounting systems.

Query fields:

- `merchantId`
- `from`
- `to`
- `format` (`json | csv`)

## Webhooks

### `POST /webhooks/merchant`

Delivery guarantees:

- At-least-once delivery
- HMAC signature header required (`X-Pokoinpos-Signature`)
- Retry policy with exponential backoff

Event types:

- `payment.confirmed`
- `payment.failed`
- `payment.expired`
- `refund.completed`
- `settlement.completed`

Minimum payload fields:

- `id` (event id)
- `type`
- `createdAt`
- `resourceId`
- `resourceType`
- `data` (event-specific body)

## Error model

Use a consistent error body:

- `error.code` (machine-readable)
- `error.message` (operator readable)
- `error.requestId`
- `error.details` (optional array)

Common codes:

- `AUTH_INVALID_TOKEN`
- `PAYMENT_QUOTE_EXPIRED`
- `PAYMENT_UNDERPAID`
- `CHAIN_RPC_UNAVAILABLE`
- `SETTLEMENT_WINDOW_LOCKED`
