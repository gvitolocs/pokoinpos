// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import {ERC20} from "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";

contract WrappedPKN is ERC20, Ownable {
    uint256 public constant MAX_BACKED_SUPPLY = 2_000_000 ether;

    error MaxBackedSupplyExceeded(uint256 requestedSupply, uint256 maxBackedSupply);

    constructor(address initialOwner) ERC20("Wrapped PKN", "wPKN") Ownable(initialOwner) {}

    function mint(address to, uint256 amount) external onlyOwner {
        uint256 requestedSupply = totalSupply() + amount;
        if (requestedSupply > MAX_BACKED_SUPPLY) {
            revert MaxBackedSupplyExceeded(requestedSupply, MAX_BACKED_SUPPLY);
        }
        _mint(to, amount);
    }

    function burn(uint256 amount) external {
        _burn(msg.sender, amount);
    }

    function burnFrom(address account, uint256 amount) external onlyOwner {
        _burn(account, amount);
    }
}
