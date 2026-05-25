# Wallet Compatibility

PokoinPoS now exposes an EVM-style JSON-RPC compatibility endpoint at:

```text
/rpc
```

This wallet-compatibility layer lets MetaMask-style wallets recognize the network, read chain metadata, read block height, check balances, submit wallet-signed raw transactions, and interact with the first embedded EVM smart contract runtime.

Native Pokoin NFTs are exposed through node HTTP APIs, not ERC-721/ERC-1155 contracts. MetaMask can still be used for PKN balances and transfers, while NFT inventory should be shown in Pokoin-owned wallet, explorer, or Card Vault surfaces.

Native PokoinSwap support also uses wallet-signed transactions. The current production swap surface remains the deterministic native AMM, while the embedded EVM runtime now provides the foundation for deploying and testing Uniswap/Pancake-style contracts.

## MetaMask Custom Network

Use these values when adding the network manually:

- Network name: `PokoinPoS`
- RPC URL: `https://<your-node-domain>/rpc`
- Chain ID: `26062026`
- Currency symbol: `PKN`
- Block explorer URL: `https://explorer.pokoin.com`

For a local node:

```text
http://127.0.0.1:8080/rpc
```

For Docker Compose with `POKOINPOS_OPS_PORT=8081`:

```text
http://127.0.0.1:8081/rpc
```

## Supported JSON-RPC Methods

The current compatibility endpoint supports:

- `web3_clientVersion`
- `net_version`
- `eth_chainId`
- `eth_blockNumber`
- `eth_syncing`
- `eth_accounts`
- `eth_requestAccounts`
- `eth_getBalance`
- `eth_getTransactionCount`
- `eth_getCode`
- `eth_getStorageAt`
- `eth_call`
- `eth_getLogs`
- `eth_getTransactionByHash`
- `eth_getTransactionReceipt`
- `eth_getBlockTransactionCountByNumber`
- `eth_getBlockTransactionCountByHash`
- `eth_getTransactionByBlockNumberAndIndex`
- `eth_getTransactionByBlockHashAndIndex`
- `eth_mining`
- `eth_hashrate`
- `eth_gasPrice`
- `eth_estimateGas`
- `eth_sendRawTransaction`

`eth_sendTransaction` returns a clear JSON-RPC error because PokoinPoS does not unlock private keys inside the node. Wallets should sign locally and submit through `eth_sendRawTransaction`.

## Funding EVM Wallets

MetaMask accounts use Ethereum-style `0x...` addresses. To give an EVM wallet PKN at chain start, configure deterministic genesis allocation on every peer before first boot:

```env
POKOINPOS_EVM_GENESIS_ALLOC=0xabc...:1000000,0xdef...:500000
```

All peers must use the same allocation before creating their state files. If a node already has persisted state under `/data`, changing this value will not rewrite existing genesis state.

## Transaction Flow

MetaMask signs an Ethereum transaction locally. The node then:

1. decodes the raw Ethereum transaction,
2. recovers the secp256k1 signer address,
3. verifies the chain ID,
4. checks the sender nonce,
5. checks the sender PKN balance,
6. maps `value` to native PKN amount,
7. executes data-bearing transactions through the embedded EVM during block replay,
8. records contract code, storage, receipts, and logs in deterministic ledger state,
9. inserts the self-verifying transaction into the PoS mempool,
10. includes it in the next valid PoS block.

The transaction remains self-verifying during block replay, so forks/restarts do not rely on the RPC server trusting a prior request.

## What Works Now

Wallets can:

- add PokoinPoS as a custom network,
- read `chainId`,
- read network ID,
- read block height,
- read balances for Ethereum-style `0x...` accounts,
- submit signed transfers through `eth_sendRawTransaction`,
- submit signed native PokoinSwap transactions through `eth_sendRawTransaction`,
- deploy and call basic EVM smart contracts through signed raw transactions,
- read contract bytecode with `eth_getCode`,
- read contract storage with `eth_getStorageAt`,
- read execution logs with `eth_getLogs`,
- poll transaction objects and receipts.

This is enough for PKN value transfers, native PokoinSwap swaps, and initial smart-contract deployment tests from MetaMask/ethers-style tooling. Full Uniswap/Pancake-style dApp compatibility should still be verified iteratively against the actual deploy scripts because those tools may require additional Ethereum RPC edge cases.

## Native PokoinSwap APIs

PokoinSwap is a chain-native AMM for `PKN` against internal accounting assets such as `wPKN`. Public swap state and quotes are available from:

- `GET /chain/swap/pools`
- `GET /chain/swap/pools/{poolId}`
- `GET /chain/swap/balances/{address}`
- `GET /chain/swap/quote?pool=PKN-WPKN&assetIn=PKN&amountIn=100`

Public swaps should be wallet-signed and submitted through `eth_sendRawTransaction`. Operator-only pool creation, liquidity, and internal asset credit/debit endpoints require `Authorization: Bearer <POKOINPOS_OPERATOR_TOKEN>`.

## Native NFT APIs

PokoinPoS supports first-class native NFT ownership in the chain ledger. These endpoints expose the current best-chain NFT state:

- `GET /chain/nfts`: list all native NFTs.
- `GET /chain/nfts?owner=<account>`: list NFTs owned by a wallet/account.
- `GET /chain/nfts/owner/<account>`: owner inventory path form.
- `GET /chain/nfts/{collectionId}/{tokenId}`: lookup a single token.

Operator-only minting and custodial transfer endpoints require `Authorization: Bearer <POKOINPOS_OPERATOR_TOKEN>`:

- `POST /admin/nft/mint`
- `POST /admin/nft/transfer`

Native NFTs carry `collectionId`, `tokenId`, `owner`, `metadataUri`, `metadataHash`, `imageUri`, `mintTx`, and `lastTx`. Card Vault and the explorer should use these APIs for NFT display.

## What Comes Next

To reach full EVM/dApp compatibility, continue with:

1. Deploy and test ERC-20/BEP-20, factory, pair, router, and multicall contracts.
2. Fill any missing JSON-RPC methods discovered by Hardhat/ethers deployment.
3. Refine mempool, receipt, and log semantics closer to Ethereum.
4. Add optional EIP-1559 fee handling.

## Example Calls

Check chain ID:

```bash
curl -s http://127.0.0.1:8080/rpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

Check block number:

```bash
curl -s http://127.0.0.1:8080/rpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":2,"method":"eth_blockNumber","params":[]}'
```

Check balance:

```bash
curl -s http://127.0.0.1:8080/rpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":3,"method":"eth_getBalance","params":["0x0000000000000000000000000000000000000000","latest"]}'
```

Submit a signed raw transaction:

```bash
curl -s http://127.0.0.1:8080/rpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":4,"method":"eth_sendRawTransaction","params":["0x<signed-raw-transaction>"]}'
```
