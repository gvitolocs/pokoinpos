require("@nomicfoundation/hardhat-toolbox");
require("dotenv").config();

const privateKey = process.env.BNB_DEPLOYER_PRIVATE_KEY;

module.exports = {
  solidity: {
    version: "0.8.28",
    settings: {
      optimizer: {
        enabled: true,
        runs: 200
      }
    }
  },
  networks: {
    bnbMainnet: {
      url: process.env.BNB_MAINNET_RPC_URL || "https://bsc-dataseed.binance.org/",
      chainId: 56,
      accounts: privateKey ? [privateKey] : []
    }
  },
  etherscan: {
    apiKey: process.env.BSCSCAN_API_KEY || ""
  }
};
