# Security and Compliance Baseline

This baseline captures minimum controls for a modern blockchain payment platform.

## Key management

- Never store private keys in application code or plaintext config.
- Use HSM/KMS-backed signing where possible.
- Rotate API keys and webhook secrets regularly.
- Enforce strict separation between signing keys and read-only keys.

## API and app security

- Mandatory TLS everywhere.
- JWT expiration and refresh flow with revocation support.
- Role-based access control (merchant admin, cashier, finance, support).
- Idempotency protections on all payment and refund writes.
- Request signing for high-risk server-to-server operations.

## Wallet and transaction safety

- Validate destination addresses against policy (allowlists, risk scores).
- Detect underpayment/overpayment and handle by explicit policy.
- Confirm chain finality threshold before marking payment as settled.
- Store immutable audit logs for every state transition.

## Fraud and risk controls

- Velocity limits by wallet, device, and merchant.
- Suspicious pattern alerts (rapid retries, multi-terminal anomalies).
- Manual review queue for large or risky transactions.
- Optional sanctions and AML screening pipeline depending on jurisdiction.

## Compliance considerations

- Maintain KYC/KYB workflows where required.
- Support tax-ready exports and accounting evidence trails.
- Retain records according to regional legal requirements.
- Provide data deletion/anonymization flows where privacy laws require it.

## Reliability and incident response

- Multi-RPC provider failover per supported chain.
- Queue-based webhook delivery with retries and dead-letter handling.
- Runbook for chain congestion and reorg scenarios.
- Backfill/reconciliation jobs for missed events.

## Monitoring and alerting

Track and alert on:

- Quote-to-payment conversion rate
- Payment confirmation latency by chain
- Settlement lag and failed payout rate
- Webhook delivery failure rate
- RPC error rate and latency

## Security testing

- Automated dependency and container scanning.
- Periodic penetration tests.
- Signature verification fuzz tests.
- Replay-attack and nonce-reuse tests in CI.
