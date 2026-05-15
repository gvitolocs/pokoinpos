# Shop and POS Integration

This guide defines how physical and online shops should integrate with PokoinPOS.

## Onboarding flow

1. Merchant account created through `POST /merchants`.
2. Shop entity created through `POST /shops`.
3. Terminal devices provisioned through `POST /shops/{shopId}/terminals`.
4. Operator roles assigned (admin, cashier, finance).
5. Settlement and webhook destination configured.

## POS checkout flow

1. Cashier enters cart amount in fiat.
2. POS requests quote from `POST /payments/quote`.
3. POS creates payment intent via `POST /payments/intents`.
4. Customer scans QR or taps wallet deeplink.
5. POS polls payment intent status.
6. POS prints receipt after `confirmed`.
7. Event is delivered to merchant webhook.

## Ecommerce checkout flow

1. Backend creates payment quote and intent.
2. Frontend renders wallet button and network details.
3. Customer signs and sends payment.
4. Webhook updates order status from `pending` to `paid`.
5. Merchant ERP/OMS syncs on `payment.confirmed`.

## Required POS fields

- `orderReference`
- `terminalId`
- `shopId`
- `cashierId` (for accountability)
- `amountFiat`, `currency`
- `cartChecksum` (optional but recommended)

## Receipts

Receipt should include:

- Payment intent id
- Chain and asset used
- Transaction hash
- Fiat amount and exchange rate snapshot
- Date/time and terminal metadata

## Refund flow in shop

1. Operator selects historical payment.
2. POS creates refund request (`POST /refunds`).
3. Backoffice policy engine approves or auto-approves based on rules.
4. POS tracks status until `completed`.

## Settlement models

- Real-time forwarding to merchant wallet.
- Daily net settlement by merchant and currency.
- Hybrid model (stablecoins in real-time, volatile assets batched).

## Failure handling for retail

- If RPC degraded, allow "cash or card fallback" prompt.
- If quote expires, regenerate in one tap.
- If payment pending too long, move to "manual review" state.
- If duplicate payment detected, flag for automatic refund suggestion.

## Shop operations checklist

- Daily reconciliation report exported.
- Terminal clock sync checked.
- Staff wallet-risk training completed.
- Incident playbook accessible from POS admin screen.
