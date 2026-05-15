# Wallet Integration Guide

This document describes wallet connection patterns for customers, merchants, and backend verification.

## Supported wallet categories

- Browser extension wallets (for example MetaMask, Rabby).
- Mobile wallets through WalletConnect.
- Embedded custodial wallets for low-friction checkout.

## Connection flow

1. Client requests wallet connection and receives `address` + `chainId`.
2. Client calls `POST /wallets/nonce`.
3. Wallet signs nonce message.
4. Client calls `POST /wallets/verify` with `signature`.
5. Server verifies signature and creates authenticated wallet session.

## Sign-in message format

Use a clear, phishing-resistant message:

- Domain name
- Wallet address
- Nonce
- Issued-at timestamp
- Expiration timestamp
- Statement of purpose (for example "Sign in to PokoinPOS")

Example:

`pokoinpos.com wants you to sign in with your wallet. Nonce: <nonce>. Expires: <iso-timestamp>.`

## Network and chain handling

- Reject unknown `chainId` values.
- Detect network mismatch and prompt switch before payment starts.
- Keep chain-specific confirmations configurable per environment.

## Payment signing

- Prefer transaction request payloads generated server-side to reduce client tampering.
- Validate destination address, amount, token contract, and deadline on the backend before marking intent as paid.

## Security controls

- Nonce TTL: 5 minutes maximum.
- One-time nonce use only; invalidate immediately after verification.
- Enforce rate limits by IP + address.
- Store wallet linkage audit trail (`address`, `userId`, `verifiedAt`, `clientFingerprint`).

## Wallet UX recommendations

- Always show connected address and selected network in checkout UI.
- Display live gas estimate and fallback network option.
- Provide copyable payment reference for support.
- Show real-time status from `GET /payments/intents/{paymentIntentId}`.

## Custodial fallback mode (optional)

For users without external wallets:

- Generate hosted wallet account.
- Enforce strong auth (MFA and device confirmation).
- Provide export flow and key-ownership terms.

## Failure scenarios to handle

- User rejects signature.
- Wrong network selected.
- Quote expires before broadcast.
- Transaction dropped or replaced.
- Underpayment caused by manual amount edits.

## Test cases

- Verify nonce replay rejection.
- Verify signature cannot be reused across domains.
- Validate wallet reconnect after session expiration.
- Validate success/failure callbacks into POS interface.
