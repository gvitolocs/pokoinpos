# PKN Native Asset Self-Assessment

Date: 2026-05-16

## Important Disclaimer

This document is a project self-assessment prepared by the Pokoin team for
informational and compliance-intake purposes. It is not a legal opinion, is not
legal advice, and has not been prepared, reviewed, or signed by external legal
counsel. Any platform, exchange, on-ramp provider, or reviewer that requires a
formal legal opinion should request one from qualified legal counsel in the
relevant jurisdiction.

## Asset Summary

- Asset name: Pokoin
- Ticker symbol: PKN
- Asset type: native blockchain coin
- Native network: PokoinPoS
- Chain ID: 26062026
- Network ID: 26062026
- Decimals: 18
- Official website: https://pokoin.com
- Public explorer: https://explorer.pokoin.com
- Public RPC: https://rpc.pokoin.com/rpc
- Chain status endpoint: https://rpc.pokoin.com/chain/status
- Total supply endpoint: https://rpc.pokoin.com/chain/supply/total.txt
- Circulating supply endpoint: https://rpc.pokoin.com/chain/supply/circulating.txt

PKN is the native asset of the PokoinPoS blockchain. It is not issued as an
ERC-20, BEP-20, or other smart-contract token on a third-party chain. Because it
is a native coin, PKN does not have a token contract address and does not have a
token-contract owner that can be renounced.

## Current Public Network Data

At the time this document was prepared, public endpoints reported:

- Total supply: 2,003,004,050 PKN
- Circulating supply: 2,001,004,050 PKN
- Public chain height: 395
- Public transaction count: 13

These values are live network values and may change as additional validator
rewards or transactions are included in future blocks. Reviewers should use the
public supply endpoints above for the latest numerical values.

## Intended Purpose and Utility

PKN is intended to function as the native utility asset of PokoinPoS. Its
current and intended uses include:

- native value transfers on PokoinPoS;
- validator reward accounting;
- network settlement and balances;
- marketplace-related utility for the Pokoin/CardVault ecosystem;
- wallet transfers through native and EVM-style compatibility endpoints;
- public explorer and RPC indexing for transparent verification.

PKN is not designed to represent equity, debt, dividends, revenue share,
profit participation, voting rights in a legal entity, or ownership of project
assets.

## Native Coin, Not Token Contract

PKN is accounted for directly by the PokoinPoS ledger. This is analogous to ETH
on Ethereum or BNB on BNB Smart Chain: the native coin exists at protocol level
and does not require a token contract to track balances or transfers.

PokoinPoS exposes an EVM-style JSON-RPC compatibility layer so wallets can read
balances and submit signed native PKN transfers. This compatibility layer does
not make PKN an ERC-20/BEP-20 smart-contract token, and current PokoinPoS
documentation states that smart contract execution and ERC-20 token contracts
are not implemented for the native network at this stage.

## Privacy Characteristics

PKN is not designed as a privacy coin. PokoinPoS uses a public ledger where
blocks, balances, transfers, supply data, validator information, and explorer
records are publicly visible through the official explorer and RPC endpoints.
The project does not implement transaction mixing, shielded addresses, stealth
addresses, confidential transactions, or similar privacy-oriented features.

## Stablecoin and Security-Token Characteristics

PKN is not a stablecoin. It is not designed to maintain a fixed price relative
to fiat currency, commodities, securities, or another cryptoasset.

Based on the project's own self-assessment, PKN is intended as a native utility
coin for blockchain settlement and ecosystem access, not as a security token.
The project does not present PKN as a claim on equity, debt, profits, dividends,
interest, revenue share, or other regulated financial instruments.

This self-assessment is not a substitute for a formal legal opinion. If a
reviewer requires a legal classification under US, EU, UK, or other law, that
analysis should be performed by qualified legal counsel.

## Regulation and Jurisdictional Notes

The project is operated as an internet-native software and blockchain project.
Development and infrastructure are currently associated with the European Union
and cloud infrastructure providers used for public nodes. The asset is not
currently offered under a specific regulated jurisdictional framework.

The project is not aware of PKN being regulated as a security, stablecoin, or
other regulated financial instrument in any jurisdiction at the time of this
self-assessment.

## Related BNB Chain Representation

The project also maintains wPKN, a fixed-supply BNB Chain representation of
PKN for external DeFi visibility and liquidity discovery. wPKN is separate from
native PKN:

- Native asset: PKN on PokoinPoS
- BNB Chain representation: wPKN on BNB Smart Chain

wPKN contract ownership has been renounced and its supply is fixed at
2,000,000 wPKN. This does not change the status of native PKN, which remains
the protocol-level coin of PokoinPoS.

## Evidence Links

- Official website: https://pokoin.com
- Public explorer: https://explorer.pokoin.com
- Public RPC status: https://rpc.pokoin.com/chain/status
- Total supply: https://rpc.pokoin.com/chain/supply/total.txt
- Circulating supply: https://rpc.pokoin.com/chain/supply/circulating.txt
- Source code and documentation: https://github.com/newisdom/pokoinpos
- PokoinPoS chain metadata: https://github.com/newisdom/pokoinpos/blob/main/metadata/pokoinpos-chain.json

## Conclusion

This project self-assessment describes PKN as the native utility coin of the
PokoinPoS public ledger. PKN is not a stablecoin, not a privacy coin, and not a
third-party-chain smart-contract token. It is intended for native network
transfers, validator rewards, settlement, and marketplace-related utility.

This document is provided for informational review only and should not be
treated as a legal opinion.
