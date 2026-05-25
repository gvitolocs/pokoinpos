---
title: "Pokoin Market Performance Data"
subtitle: "Trading volume reports, user engagement metrics, and supporting documents"
author: "Pokoin / PokoinPoS"
date: "2026-05-16"
geometry: margin=0.8in
fontsize: 10pt
documentclass: article
colorlinks: true
linkcolor: blue
urlcolor: blue
---

# Purpose

This document summarizes the currently available market performance and user
engagement data for Pokoin, PKN, and wPKN. It is prepared as a supporting
document for exchange, partner, and due-diligence discussions.

Pokoin is currently in an early infrastructure and market-formation stage.
Therefore, some metrics are live and verifiable, while broader marketplace
analytics such as sustained trading volume, recurring active users, and paid
analytics subscriptions are planned product metrics rather than mature historical
figures.

# Public Verification Links

| Area | Public URL |
| --- | --- |
| Website | `https://pokoin.com` |
| Marketplace | `https://pokoin.com/marketplace` |
| Wallet | `https://pokoin.com/wallet` |
| Explorer | `https://explorer.pokoin.com` |
| Scan UI | `https://pokoin.com/scan` |
| RPC endpoint | `https://rpc.pokoin.com/rpc` |
| Node health | `https://rpc.pokoin.com/health` |
| wPKN logo | `https://explorer.pokoin.com/wpkn/logo.png` |
| wPKN reserve manifest | `https://explorer.pokoin.com/wpkn-reserve.json` |

# Current Chain Performance Snapshot

Snapshot date: 2026-05-16.

| Metric | Current value | Source |
| --- | ---: | --- |
| Chain | PokoinPoS | Project metadata |
| Native asset | PKN | Project metadata |
| Chain ID | 26062026 | Public network config |
| Latest height | 378 | `rpc.pokoin.com/chain/status` |
| Finalized height | 377 | `rpc.pokoin.com/chain/status` |
| Finality depth | 1 block | `rpc.pokoin.com/chain/status` |
| Public transactions observed | 13 | `rpc.pokoin.com/chain/status` |
| Mempool depth | 0 | `rpc.pokoin.com/chain/status` |
| Authorized validators | 2 | `rpc.pokoin.com/chain/validators` |
| Connected peer count | 1 | `rpc.pokoin.com/chain/status` |
| Validator stake observed | 1,003,780 PKN | `rpc.pokoin.com/chain/status` |
| Mined blocks observed by current node | 5 | `rpc.pokoin.com/chain/status` |
| Accepted blocks observed by current node | 5 | `rpc.pokoin.com/chain/status` |

# wPKN Market and Liquidity Snapshot

wPKN is the wrapped representation of PKN on BNB Smart Chain.

| Metric | Current value |
| --- | --- |
| Wrapped token | Wrapped PKN |
| Symbol | wPKN |
| Chain | BNB Smart Chain |
| Contract | `0x91A17E2bddfF839078BD395482B38e4AC15276f4` |
| Total supply | 2,000,000 wPKN |
| Reserve model | 1 native PKN reserved for every 1 wPKN minted |
| Reserved native amount | 2,000,000 PKN |
| Native treasury | `0x74466c3a204429b22ce8558f3f18f3c59f67fcb3` |
| PancakeSwap pair | `0x86294c008542C2707B9f67e3E4BA2d03B7bF7451` |
| Initial pair | wPKN/BNB |
| Initial liquidity | 2,000 wPKN and 0.0015 BNB |
| Liquidity transaction | `0xd4d1d40c31d1efff2be4b6e0d29d54877a5732709a95648c01860a663a15defb` |

# Trading Volume Status

PKN and wPKN are in an early market-formation phase. Current confirmed market
activity includes:

- wPKN contract deployed on BNB Smart Chain.
- PancakeSwap wPKN/BNB pool created.
- Initial liquidity added to support public price discovery.
- Public reserve manifest available for verification.
- Public explorer and wallet surfaces available for user onboarding.
- Oracle-backed CardVault marketplace available through `pokoin.com`.

Historical trading volume is not yet presented as a mature recurring metric.
The project should begin publishing monthly trading-volume reports once the
market has sufficient activity and reliable data collection.

Recommended monthly trading report fields:

| Field | Purpose |
| --- | --- |
| Monthly wPKN trading volume | Measures external market activity |
| Monthly PKN native transfer count | Measures chain usage |
| Monthly unique active wallets | Measures user engagement |
| Marketplace card sales volume | Measures product-market usage |
| Tokenized card count | Measures core utility adoption |
| Average transaction size | Helps understand usage quality |
| Repeat buyer/seller count | Measures marketplace retention |
| Liquidity depth | Measures trading friction |
| Reserve coverage | Confirms wPKN backing discipline |

# User Engagement Metrics

Current engagement surfaces:

- Pokoin website.
- CardVault marketplace.
- Pokoin Wallet.
- PokoinScan / explorer.
- RPC and chain API.
- wPKN reserve proof.
- PancakeSwap pool.
- Oracle-backed marketplace flow with CardTrader catalog/search projections.

Current measurable engagement indicators:

| Indicator | Current status |
| --- | --- |
| Public explorer live | Yes |
| Wallet route live | Yes |
| RPC live | Yes |
| Validators live | Yes, 2 authorized validators |
| wPKN reserve manifest live | Yes |
| BNB Smart Chain token live | Yes |
| Marketplace analytics | Early interaction analytics live through Oracle hot-blueprint rollups |
| Instant Pokemon card tokenization | Planned / product direction |
| User-level retention cohorts | Not yet mature |
| Paid analytics subscriptions | Planned |

# Market Analytics Product Direction

Pokoin's analytics layer focuses on collectible-card markets, starting with
Pokemon cards. The goal is to provide better data than fragmented legacy sources
by making card events trackable and structured.

Current implemented analytics:

- Bounded marketplace event capture for card views, searches, result clicks,
  cart adds, reserves, and sales/reserve signals.
- Oracle `marketplace_card_events` storage for raw interaction events.
- Oracle `marketplace_hot_blueprints` rollups with 1h, 24h, and 7d scores.
- Homepage Best sellers and Featured sections backed by Oracle snapshot IDs.

Planned analytics:

- Tokenized card count by set, rarity, condition, and owner.
- Price history per card.
- Comparable sale references.
- Marketplace bid/ask spreads.
- Card liquidity scoring.
- Portfolio valuation for collectors.
- Ownership and transfer history.
- wPKN liquidity and reserve monitoring.
- Native PKN usage and wallet activity.

These metrics can become stronger over time as tokenized-card inventory and
marketplace transactions increase.

# Supporting Evidence

## Infrastructure evidence

Pokoin currently has the following public infrastructure:

- Public website and wallet.
- Public scan/explorer UI.
- Public RPC endpoint.
- Live node health endpoint.
- Validator visibility.
- wPKN contract and PancakeSwap pool.
- Reserve manifest for wrapped supply backing.

## Product evidence

The project has implemented or prepared:

- Native PKN chain.
- Public wallet route.
- Oracle-backed CardVault marketplace routes.
- Explorer/scan page with chain-facing metrics.
- wPKN reserve proof.
- Token logo and metadata assets.
- Dockerized peer deployment model.
- Documentation for node operators and public network metadata.
- Website documentation for route ownership, marketplace APIs, and deployment boundaries.

## Market evidence

Market evidence is early but concrete:

- wPKN exists on BNB Smart Chain.
- wPKN/BNB pair exists on PancakeSwap.
- Initial liquidity has been added.
- Public reserve amount is documented.
- Public URLs are stable for third-party token metadata.

# Next Reporting Milestones

Pokoin should produce recurring market-performance reports once sufficient data
is available.

| Period | Report focus |
| --- | --- |
| Month 1 | Baseline users, wallets, tokenized cards, transfers |
| Month 3 | Early cohort retention, card listings, marketplace tests |
| Month 6 | Trading volume, liquidity depth, active wallets, paid analytics interest |
| Month 12 | Marketplace GMV, tokenized-card coverage, partner integrations |

# Notes and Limitations

- Pokemon is a third-party brand. Pokoin should not imply official affiliation
  unless a license or partnership is obtained.
- Trading data is early and should not be overstated.
- Tokenomics and vesting are not finalized.
- No passive token-holder revenue or yield should be promised unless formally
  implemented and legally reviewed.
- Public-chain and marketplace data should be independently verified before
  external submission.

# Conclusion

Pokoin has live infrastructure and early market foundations: a native PKN chain,
public explorer, wallet, validator network, Oracle-backed CardVault marketplace,
wPKN on BNB Smart Chain, reserve manifest, and PancakeSwap pool. The next step is
to convert this foundation into measurable market performance through instant
card tokenization, settled marketplace transactions, active wallets, and
recurring monthly reporting.
