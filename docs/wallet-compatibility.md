# Wallet Compatibility

PokoinPoS now exposes an EVM-style JSON-RPC compatibility endpoint at:

```text
/rpc
```

This wallet-compatibility layer lets MetaMask-style wallets recognize the network, read chain metadata, read block height, check balances, and submit wallet-signed raw transactions.

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
7. inserts the self-verifying transaction into the PoS mempool,
8. includes it in the next valid PoS block.

The transaction remains self-verifying during block replay, so forks/restarts do not rely on the RPC server trusting a prior request.

## What Works Now

Wallets can:

- add PokoinPoS as a custom network,
- read `chainId`,
- read network ID,
- read block height,
- read balances for Ethereum-style `0x...` accounts,
- submit signed transfers through `eth_sendRawTransaction`,
- poll transaction objects and receipts.

This is enough for PKN value transfers from MetaMask-style wallets. Smart contract execution and ERC-20 contracts are not implemented yet.

## What Comes Next

To reach full EVM/dApp compatibility, implement:

1. Smart contract execution or an EVM runtime.
2. ERC-20/BEP-20 token contracts.
3. Full block and log query methods.
4. Mempool and receipt semantics closer to Ethereum.
5. Optional EIP-1559 fee handling.

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
