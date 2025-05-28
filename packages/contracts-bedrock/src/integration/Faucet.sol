// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/// @title Faucet
/// @notice Faucet is a util contract to airdrop Wei, primarily for efficiently funding many accounts in integration tests.
contract Faucet {
    /// @notice Sends Wei to recipients.
    /// @param _recipients List of recipients.
    /// @param _amount     Amount of Wei to send to each recipient.
    function fund(
        address[] memory _recipients,
        uint256 _amount
    ) external payable {
        uint256 totalAmount = _recipients.length * _amount;
        require(msg.value >= totalAmount, "Not enough ETH sent");

        for (uint256 i = 0; i < _recipients.length; i++) {
            payable(_recipients[i]).transfer(_amount);
        }

        // Refund any excess ETH back to the sender.
        uint256 refund = msg.value - totalAmount;
        if (refund > 0) {
            payable(msg.sender).transfer(refund);
        }
    }
}
