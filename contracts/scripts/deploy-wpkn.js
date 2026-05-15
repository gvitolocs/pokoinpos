const fs = require("node:fs");
const path = require("node:path");
const { ethers } = require("hardhat");

const INITIAL_MINT = process.env.WPKN_INITIAL_MINT || "2000000";

function requireAddress(name, fallback) {
  const value = process.env[name] || fallback;
  if (!value || !ethers.isAddress(value)) {
    throw new Error(`${name} must be a valid EVM address`);
  }
  return ethers.getAddress(value);
}

async function main() {
  const [deployer] = await ethers.getSigners();
  const owner = requireAddress("WPKN_OWNER", deployer.address);
  const recipient = requireAddress("WPKN_INITIAL_RECIPIENT", owner);
  const initialMint = ethers.parseEther(INITIAL_MINT);

  const WrappedPKN = await ethers.getContractFactory("WrappedPKN");
  const wpkn = await WrappedPKN.deploy(owner);
  await wpkn.waitForDeployment();

  const address = await wpkn.getAddress();
  const deploymentTx = wpkn.deploymentTransaction();
  if (initialMint > 0n) {
    if (owner !== deployer.address) {
      throw new Error("WPKN_OWNER must be the deployer when WPKN_INITIAL_MINT is greater than zero");
    }
    const mintTx = await wpkn.mint(recipient, initialMint);
    await mintTx.wait();
  }

  const artifact = {
    chainId: 56,
    network: "bnb-mainnet",
    contractName: "WrappedPKN",
    name: "Wrapped PKN",
    symbol: "wPKN",
    decimals: 18,
    address,
    owner,
    initialRecipient: recipient,
    initialMint: INITIAL_MINT,
    maxBackedSupply: "2000000",
    deployer: deployer.address,
    deploymentTxHash: deploymentTx ? deploymentTx.hash : null,
    deployedAt: new Date().toISOString()
  };

  const outputPath = path.resolve("../metadata/wpkn-bnb-mainnet.json");
  fs.mkdirSync(path.dirname(outputPath), { recursive: true });
  fs.writeFileSync(outputPath, `${JSON.stringify(artifact, null, 2)}\n`);

  console.log(`wPKN deployed to ${address}`);
  console.log(`Deployment artifact written to ${outputPath}`);
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
