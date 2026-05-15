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
- Confirm `https://explorer.pokoin.com` is live and public.
- Confirm `https://explorer.pokoin.com/tx/{hash}` style pages or API-backed pages resolve transaction details.
- Upload the logo to IPFS and update `metadata/ethereum-lists/icon-pokoinpos.json`.
- Check that chain ID `26062026` is not already registered.
- Open an ethereum-lists/chains pull request with the chain JSON and icon JSON.
