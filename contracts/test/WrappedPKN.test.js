const { expect } = require("chai");
const { ethers } = require("hardhat");

describe("WrappedPKN", function () {
  async function deployWrappedPKNFixture() {
    const [owner, reserve, other] = await ethers.getSigners();
    const WrappedPKN = await ethers.getContractFactory("WrappedPKN");
    const wpkn = await WrappedPKN.deploy(owner.address);
    await wpkn.waitForDeployment();
    return { wpkn, owner, reserve, other };
  }

  it("uses the expected metadata and max backed supply", async function () {
    const { wpkn } = await deployWrappedPKNFixture();

    expect(await wpkn.name()).to.equal("Wrapped PKN");
    expect(await wpkn.symbol()).to.equal("wPKN");
    expect(await wpkn.decimals()).to.equal(18n);
    expect(await wpkn.MAX_BACKED_SUPPLY()).to.equal(ethers.parseEther("2000000"));
  });

  it("allows the owner to mint backed supply", async function () {
    const { wpkn, reserve } = await deployWrappedPKNFixture();

    await expect(wpkn.mint(reserve.address, ethers.parseEther("2000000")))
      .to.emit(wpkn, "Transfer")
      .withArgs(ethers.ZeroAddress, reserve.address, ethers.parseEther("2000000"));

    expect(await wpkn.balanceOf(reserve.address)).to.equal(ethers.parseEther("2000000"));
    expect(await wpkn.totalSupply()).to.equal(ethers.parseEther("2000000"));
  });

  it("rejects minting above the reserve cap", async function () {
    const { wpkn, reserve } = await deployWrappedPKNFixture();
    const amount = ethers.parseEther("2000000.000000000000000001");

    await expect(wpkn.mint(reserve.address, amount))
      .to.be.revertedWithCustomError(wpkn, "MaxBackedSupplyExceeded")
      .withArgs(amount, ethers.parseEther("2000000"));
  });

  it("rejects non-owner minting", async function () {
    const { wpkn, reserve, other } = await deployWrappedPKNFixture();

    await expect(wpkn.connect(other).mint(reserve.address, 1n))
      .to.be.revertedWithCustomError(wpkn, "OwnableUnauthorizedAccount")
      .withArgs(other.address);
  });

  it("allows holders and the owner to burn supply", async function () {
    const { wpkn, reserve } = await deployWrappedPKNFixture();
    await wpkn.mint(reserve.address, ethers.parseEther("10"));

    await expect(wpkn.connect(reserve).burn(ethers.parseEther("3")))
      .to.emit(wpkn, "Transfer")
      .withArgs(reserve.address, ethers.ZeroAddress, ethers.parseEther("3"));
    expect(await wpkn.totalSupply()).to.equal(ethers.parseEther("7"));

    await wpkn.burnFrom(reserve.address, ethers.parseEther("2"));
    expect(await wpkn.totalSupply()).to.equal(ethers.parseEther("5"));
  });
});
