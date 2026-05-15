# Documentation

This project is a blockchain-powered POS platform. The documentation in this folder defines the API contract, integration patterns, and production requirements for a modern deployment.

## Contents

- `api/endpoints.md`: REST API endpoints, request/response models, and webhook contracts.
- `node-endpoints.md`: Live PokoinPoS node endpoints for health pages, monitoring, and admin smoke tests.
- `integrations/wallets.md`: Wallet connection patterns for browser, mobile, and server-side verification.
- `integrations/shop-pos.md`: Shop onboarding, POS payment flow, refunds, and settlement.
- `operations/security-compliance.md`: Security baseline, key management, fraud controls, and compliance requirements.
- `operations/modern-blockchain-checklist.md`: Feature checklist for chain support, observability, reliability, and scale.
- `operations/disaster-recovery.md`: Backup/restore procedures, RTO/RPO targets, and DR game-day workflow.

## Versioning

- API prefix: `/api/v1`
- Breaking changes require `/api/v2`
- Webhook schema changes must include a migration window and dual payload support
