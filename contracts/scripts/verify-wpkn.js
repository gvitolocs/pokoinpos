const hre = require("hardhat");

async function main() {
  const contractAddress = process.env.WPKN_CONTRACT_ADDRESS;
  const owner = process.env.WPKN_OWNER;
  if (!contractAddress || !owner) {
    throw new Error("WPKN_CONTRACT_ADDRESS and WPKN_OWNER are required");
  }

  await hre.run("verify:verify", {
    address: contractAddress,
    constructorArguments: [owner]
  });
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
