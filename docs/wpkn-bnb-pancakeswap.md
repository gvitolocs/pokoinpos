# wPKN on BNB Chain and PancakeSwap

`wPKN` is the BNB Chain wrapped representation of native `PKN`.

The first reserve allocation is `2,000,000 PKN`, backed 1:1 by native PKN held in the PokoinPoS treasury account recorded in `metadata/wpkn-reserve.json`.

## Contract

- Network: BNB Smart Chain mainnet
- Chain ID: `56`
- Token name: `Wrapped PKN`
- Token symbol: `wPKN`
- Decimals: `18`
- Max backed supply for this launch: `2,000,000 wPKN`
- Contract address: `0x91A17E2bddfF839078BD395482B38e4AC15276f4`
- PancakeSwap pair: `0x86294c008542C2707B9f67e3E4BA2d03B7bF7451`
- Contract artifact: `metadata/wpkn-bnb-mainnet.json`
- Reserve manifest: `metadata/wpkn-reserve.json`

## Operator Prerequisites

1. The deployer wallet must be the same EVM/MetaMask wallet selected for the PKN reserve workflow.
2. The deployer wallet needs BNB on BNB Chain mainnet for:
   - Contract deployment gas.
   - Contract verification transactions if needed.
   - Initial PancakeSwap `wPKN/BNB` liquidity.
3. The private key must be stored only on the Oracle VM in a local secret file, never committed and never pasted into chat.

Recommended secret location on Oracle:

```bash
sudo install -d -m 0700 /opt/pokoinpos-secrets
sudo nano /opt/pokoinpos-secrets/wpkn.env
sudo chmod 600 /opt/pokoinpos-secrets/wpkn.env
```

Example secret file:

```bash
BNB_MAINNET_RPC_URL=https://bsc-dataseed.binance.org/
BNB_DEPLOYER_PRIVATE_KEY=0x...
BSCSCAN_API_KEY=
WPKN_OWNER=0xYourDeployerAddress
WPKN_INITIAL_RECIPIENT=0xYourDeployerAddress
WPKN_INITIAL_MINT=2000000
```

## Deploy From Oracle

From the repository root on Oracle:

```bash
cd contracts
npm ci
set -a
. /opt/pokoinpos-secrets/wpkn.env
set +a
npm run build
npm run deploy:bnb
```

After deployment, `scripts/deploy-wpkn.js` writes the deployed contract details to:

```text
metadata/wpkn-bnb-mainnet.json
```

## Verify on BscScan

Set the deployed contract address in the same shell:

```bash
export WPKN_CONTRACT_ADDRESS=0xDeployedContract
npm run verify:bnb
```

Verification requires `BSCSCAN_API_KEY`.

## Add PancakeSwap Liquidity

Current launch liquidity:

```text
2,000 wPKN + 0.0015 BNB
```

Transactions:

- Approve: `0xc9ba216a3be3b90f82b14817a74e7e3e8eb322c69b8562fac3249fc8fa02eb1a`
- Add liquidity: `0xd4d1d40c31d1efff2be4b6e0d29d54877a5732709a95648c01860a663a15defb`

Use the PancakeSwap UI with the deployer wallet:

1. Open PancakeSwap on BNB Chain.
2. Import the deployed `wPKN` token contract.
3. Create or select the `wPKN/BNB` pair.
4. Deposit the chosen amount of `wPKN` and BNB.
5. Confirm the liquidity transaction.
6. Record the pool address in `metadata/wpkn-reserve.json`.

The initial market price is determined by the liquidity ratio:

```text
initial price in BNB = BNB deposited / wPKN deposited
```

Example:

```text
100,000 wPKN + 1 BNB => 1 wPKN = 0.00001 BNB
```

## Reserve Rule

Circulating `wPKN` must never exceed native PKN reserved for backing.

For this launch:

```text
max wPKN supply = 2,000,000
reserved native PKN = 2,000,000
```

If native PKN is removed from reserve, the matching amount of `wPKN` must be burned first.
