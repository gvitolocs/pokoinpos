# PokoinPoS Public Network

Use these values to verify or add the public PokoinPoS network.

```text
Network name: PokoinPoS
Chain ID: 26062026
Network ID: 26062026
Native currency: PKN
Decimals: 18
RPC URL: https://rpc.pokoin.com/rpc
Explorer URL: https://explorer.pokoin.com
Website: https://pokoin.com
Docker image: newisdom/pokoinpos-peer:latest
```

## Hosting Boundary

`explorer.pokoin.com` is a public frontend/static metadata hostname. It should
be hosted on Vercel alongside `pokoin.com`, not on an Oracle node.

Frontend/static URLs hosted by Vercel:

- `https://pokoin.com`
- `https://pokoin.com/scan`
- `https://pokoin.com/health`
- `https://pokoin.com/wallet`
- `https://explorer.pokoin.com`
- `https://pokoin.com/bootstrap-peers.json`
- `https://explorer.pokoin.com/wpkn/logo.png`
- `https://explorer.pokoin.com/wpkn-reserve.json`

Chain/RPC URLs backed by node infrastructure:

- `https://rpc.pokoin.com/rpc`
- `https://rpc.pokoin.com/health`
- `https://rpc.pokoin.com/chain/status`
- `https://rpc.pokoin.com/chain/validators`
- `https://rpc.pokoin.com/explorer/blocks`
- `https://rpc.pokoin.com/explorer/tx/{hash}`
- `https://rpc.pokoin.com/explorer/address/{address}`

The public explorer UI reads from `rpc.pokoin.com`; it is not itself the node.

## Bootstrap Registry

Public nodes fetch the dynamic bootstrap list from:

```text
https://pokoin.com/bootstrap-peers.json
```

The manifest is generated from observed node health. New candidate nodes spend
14 days in vetting and must stay online for at least 95% of that window. After
vetting, a node remains a regular peer until it is 365 days old. Only then can
it enter the annual bootstrap list, and only if it has at least 94% uptime over
the previous 365 days. That uptime must be observed by at least 3 other peers;
self-observation does not count. The current Oracle nodes are grandfathered as
bootstrap peers and remain fallback peers through `POKOINPOS_BOOTSTRAP_PEERS`.

## MetaMask

Use `metadata/wallet-add-network.json` for a `wallet_addEthereumChain` payload, or add the network manually with the values above.

## Chain Registry Artifacts

The repository includes draft metadata for public network submission:

- `metadata/pokoinpos-chain.json`
- `metadata/wallet-add-network.json`
- `metadata/ethereum-lists/eip155-26062026.json`
- `metadata/ethereum-lists/icon-pokoinpos.json`

Before submitting to ethereum-lists/chains, replace the icon placeholder with an IPFS-hosted logo that satisfies their icon requirements.

## Submission Checklist

- Confirm `https://rpc.pokoin.com/rpc` responds to `eth_chainId`.
- Confirm `https://explorer.pokoin.com` resolves to the Vercel-hosted explorer UI.
- Confirm explorer UI links and search read API-backed data from `https://rpc.pokoin.com`.
- Upload the logo to IPFS and update `metadata/ethereum-lists/icon-pokoinpos.json`.
- Check that chain ID `26062026` is not already registered.
- Open an ethereum-lists/chains pull request with the chain JSON and icon JSON.
