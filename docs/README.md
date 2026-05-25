# PokoinPoS Documentation

This folder documents the PokoinPoS node, public RPC surface, wallet compatibility, deployment runbooks, and operating requirements for the Pokoin/CardVault ecosystem.

The public web experience now lives in the separate `cardvault` repository as a single Flutter web app:

- `https://pokoin.com/` for CardVault
- `https://pokoin.com/wallet` for the integrated Pokoin Wallet

This repository documents the chain side:

- public website route ownership and deployment boundaries,
- public RPC and health endpoints,
- MetaMask-compatible JSON-RPC behavior,
- validator/mining rules,
- Docker and Oracle Free Tier deployment,
- disaster recovery and security runbooks.

## Contents

- `website.md`: Public `pokoin.com` route map, marketplace API surface, data ownership, search/homepage behavior, and website deployment notes.
- `api/endpoints.md`: REST API endpoints, request/response models, and webhook contracts.
- `node-endpoints.md`: Live PokoinPoS node endpoints for health pages, monitoring, and admin smoke tests.
- `public-network.md`: Public chain metadata, explorer/RPC URLs, and wallet-facing network values.
- `wallet-compatibility.md`: MetaMask-style wallet setup, JSON-RPC compatibility, and next steps for full transaction support.
- `docker-hub-overview.md`: Docker image publishing, peer deployment, and auto-update workflow.
- `integrations/wallets.md`: Wallet connection patterns for browser, mobile, and server-side verification.
- `integrations/shop-pos.md`: Shop onboarding, POS payment flow, refunds, and settlement.
- `operations/security-compliance.md`: Security baseline, key management, fraud controls, and compliance requirements.
- `operations/modern-blockchain-checklist.md`: Feature checklist for chain support, observability, reliability, and scale.
- `operations/disaster-recovery.md`: Backup/restore procedures, RTO/RPO targets, and DR game-day workflow.
- `operations/postgres-peer3-fallback.md`: Marketplace Postgres streaming replication, peer3 failover, and rebuild workflow.

## Versioning

- API prefix: `/api/v1`
- Breaking changes require `/api/v2`
- Webhook schema changes must include a migration window and dual payload support
