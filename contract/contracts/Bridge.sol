// SPDX-License-Identifier: Apache-2.0

pragma solidity ^0.8.9;

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "@openzeppelin/contracts/security/ReentrancyGuard.sol";
import "./Sequence.sol";

/// @title A contract for cross-chain ERC20 transfers using a single trusted relayer
/// @author Fury Labs, LLC
contract Bridge is ReentrancyGuard, Sequence(0) {
    using SafeERC20 for IERC20;

    /// @notice The trusted relayer with the ability to unlock funds
    address private _relayer;

    /// @notice Represents an ERC20 token lock emitted during a lock call
    /// @param token The ERC20 token address
    /// @param sender The Ethereum address of the sender that locked the funds
    /// @param toFuryAddr The Fury address to send the locked funds to
    /// @param amount The amount that was locked
    /// @param lockSequence The unique lock sequence
    event Lock(
        address indexed token,
        address indexed sender,
        address indexed toFuryAddr,
        uint256 amount,
        uint256 lockSequence
    );

    /// @notice Represents an ERC20 token unlock emitted during an unlock call
    /// @param token The ERC20 token address
    /// @param toAddr The Ethereum address the funds were unlocked to
    /// @param amount The amount that was unlocked
    /// @param unlockSequence The unique unlock sequence
    event Unlock(
        address indexed token,
        address indexed toAddr,
        uint256 amount,
        uint256 unlockSequence
    );

    /// @notice Initialize with a relayer address with a starting sequence of 0
    /// @param relayer_ The Ethereum addres of the trusted relayer
    constructor(address relayer_) {
        _relayer = relayer_;
    }

    /// @notice The trusted relayer address for the bridge
    /// @return The Ethereum address of the relayer
    function relayer() public view returns (address) {
        return _relayer;
    }

    /// @notice Locks an ERC20 amount and emits a Lock event with the Fury address to mint funds to
    /// @param token The ERC20 token address
    /// @param toFuryAddr The Fury address to send the locked funds to
    /// @param amount The amount to lock
    /// @dev Emits a Lock event
    function lock(
        address token,
        address toFuryAddr,
        uint256 amount
    ) public nonReentrant {
        incrementSequence();
        IERC20(token).safeTransferFrom(msg.sender, address(this), amount);

        emit Lock(token, msg.sender, toFuryAddr, amount, getSequence());
    }

    /// @notice Unlocks an ERC20 amount and emits an Unlock event
    /// @param token The ERC20 token address
    /// @param toAddr The Ethereum address to send to unlocks funds to
    /// @param amount The amount to unlock
    /// @dev Emits an Unlock event
    /// @dev May only be called by the relayer
    function unlock(
        address token,
        address toAddr,
        uint256 amount,
        uint256 unlockSequence
    ) public nonReentrant {
        require(msg.sender == _relayer, "Bridge: untrusted address");

        IERC20(token).safeTransfer(toAddr, amount);

        emit Unlock(token, toAddr, amount, unlockSequence);
    }
}
