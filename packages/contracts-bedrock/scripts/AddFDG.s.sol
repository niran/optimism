// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import { console2 as console } from "forge-std/console2.sol";

import { Script } from "forge-std/Script.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";
import { IFaultDisputeGame } from "src/dispute/interfaces/IFaultDisputeGame.sol";

import { IDisputeGame } from "src/dispute/interfaces/IDisputeGame.sol";
import { IDisputeGameFactory } from "src/dispute/interfaces/IDisputeGameFactory.sol";

import { IDelayedWETH } from "src/dispute/interfaces/IDelayedWETH.sol";
import { IAnchorStateRegistry } from "src/dispute/interfaces/IAnchorStateRegistry.sol";
import { IBigStepper } from "src/dispute/interfaces/IBigStepper.sol";
import { GameType, Duration, OutputRoot, Claim, GameStatus, Hash } from "src/dispute/lib/Types.sol";

import { StorageSetter } from "src/universal/StorageSetter.sol";
import { EIP1967Helper } from "test/mocks/EIP1967Helper.sol";

import { ISuperchainConfig } from "src/L1/interfaces/ISuperchainConfig.sol";
import { IProxyAdmin } from "src/universal/interfaces/IProxyAdmin.sol";
import { IOptimismPortal2 } from "src/L1/interfaces/IOptimismPortal2.sol";
/**
 * @title AddFDG
 * @notice Script to add FDG (Fee Data Grantor) to the system
 */

contract AddFDG is Script {
    /// @dev The Foundry VM.
    // All addresses from here:
    // https://github.com/ethereum-optimism/devnets/blob/52d29f90fc506feb4a0ebbd26fdf157bbb14950f/betanets/aegir/aegir-1/chain.yaml#L1
    IProxyAdmin proxyAdmin = IProxyAdmin(0x6d283c3Ff5B2140032BF1A9C2fa20e4c73484666);
    ISuperchainConfig superchain = ISuperchainConfig(0xC2Be75506d5724086DEB7245bd260Cc9753911Be);
    StorageSetter storageSetter = StorageSetter(0x54F8076f4027e21A010b4B3900C86211Dd2C2DEB);
    IAnchorStateRegistry anchorStateRegistry = IAnchorStateRegistry(0xB4265083491C4Ff36d2a964A5D2228E9b3CFd89F);
    IDisputeGameFactory disputeGameFactory = IDisputeGameFactory(0x5c06aDb8f7e30A6b30Ef6C91612719E8061b3Ff5);
    IOptimismPortal2 optimismPortal = IOptimismPortal2(payable(0x1dd21367755166CfE041e5d307A081A8411C8921));

    /**
     * @notice Main function that will be executed when the script runs
     */
    function run() public {
        // Get the PDG address
        IDisputeGame pdg = disputeGameFactory.gameImpls(GameType.wrap(1));

        // Deploy the FDG
        IFaultDisputeGame fdg = _deployFDG(IFaultDisputeGame(address(pdg)));

        // Set the FDG implementation address on the dispute game factory
        vm.startBroadcast();
        disputeGameFactory.setImplementation(GameType.wrap(0), fdg);
        disputeGameFactory.setInitBond(GameType.wrap(0), 0.08 ether);
        vm.stopBroadcast();

        // Reinitialize the anchor state registry
        _reinitAnchorStateRegistry(anchorStateRegistry);

        // Set the respected game type on the portal
        _setRespectedGameType();
    }

    /**
     * @notice Sets the respected game type on the Optimism Portal
     */
    function _setRespectedGameType() internal {
        vm.startBroadcast();
        address optimismPortalImpl = EIP1967Helper.getImplementation(address(optimismPortal));
        IProxyAdmin(proxyAdmin).upgrade(payable(address(optimismPortal)), address(storageSetter));
        // Pack respectedGameTypeUpdatedAt (8 bytes) and respectedGameType (4 bytes) into a single 32-byte value
        // respectedGameTypeUpdatedAt is stored in the lower 8 bytes, respectedGameType in the next 4 bytes
        bytes32 packedData = bytes32(uint256(uint64(block.timestamp))) << 32 | bytes32(uint256(0));

        // Step 1: Set the respected game type to the FDG
        StorageSetter(address(optimismPortal)).setBytes32(bytes32(uint256(59)), packedData);
        IProxyAdmin(proxyAdmin).upgrade(payable(address(optimismPortal)), address(optimismPortalImpl));
        vm.stopBroadcast();

        require(GameType.unwrap(optimismPortal.respectedGameType()) == 0);
        require(optimismPortal.respectedGameTypeUpdatedAt() == block.timestamp);
    }

    /**
     * @notice Helper function for the main execution
     */
    function _deployFDG(IFaultDisputeGame _disputeGame) internal returns (IFaultDisputeGame) {
        // Provided by Zach:
        // https://oplabs-pbc.slack.com/archives/C0885T0HRCG/p1741022679358429?thread_ts=1740770336.922089&cid=C0885T0HRCG
        Claim absolutePrestate = Claim.wrap(0x03f206f043bb34f9e931a49716754b303e635e931b7f1294ff8ca45c969fc627);
        uint256 maxGameDepth = _disputeGame.maxGameDepth();
        uint256 splitDepth = _disputeGame.splitDepth();
        Duration clockExtension = _disputeGame.clockExtension();
        Duration maxClockDuration = _disputeGame.maxClockDuration();
        IBigStepper _vm = _disputeGame.vm();
        IDelayedWETH weth = _disputeGame.weth();
        uint256 l2ChainId = _disputeGame.l2ChainId();

        // sanity check
        require(_disputeGame.anchorStateRegistry() == anchorStateRegistry);

        // Read the constructor params from the game, and use them to deploy the FDG.
        vm.broadcast();
        return IFaultDisputeGame(
            DeployUtils.create1(
                "FaultDisputeGame",
                abi.encode(
                        GameType.wrap(0), // GameType
                        absolutePrestate, // Claim
                        maxGameDepth, // uint256
                        splitDepth, // uint256
                        clockExtension, // Duration
                        maxClockDuration, // Duration
                        _vm, // IBigStepper
                        weth, // IDelayedWETH
                        anchorStateRegistry, // IAnchorStateRegistry
                        l2ChainId // uint256
                )
            )
        );
    }

    function _reinitAnchorStateRegistry(IAnchorStateRegistry _anchorStateRegistry) internal {
        // Step 0: cache the original implementation address
        address asrImpl = EIP1967Helper.getImplementation(address(_anchorStateRegistry));

        // Step 1: Upgrade to the storage setter contract
        vm.startBroadcast();
        IProxyAdmin(proxyAdmin).upgrade(payable(address(_anchorStateRegistry)), address(storageSetter));

        // Step 2: Set the initialized slot (slot 0) to 0
        StorageSetter(address(_anchorStateRegistry)).setBytes32(bytes32(0), bytes32(0));

        // Step 3: Create new anchor roots for initialization
        IAnchorStateRegistry.StartingAnchorRoot[] memory startingRoots =
            new IAnchorStateRegistry.StartingAnchorRoot[](1);
        // from:
        // cast rpc --rpc-url https://aegir-1-opn-geth-a-rpc-0-op-node.primary.infra.dev.oplabs.cloud
        // optimism_syncStatus | jq .finalized_l2.number
        //   178565
        // cast rpc --rpc-url https://aegir-1-opn-geth-a-rpc-0-op-node.primary.infra.dev.oplabs.cloud
        // optimism_outputAtBlock $(cast 2h 178565) | jq .outputRoot
        //   "0xe50e5ae025f1d11b8862a170621c095c7d571463d6854f34f4352a445dd17f9f"
        startingRoots[0] = IAnchorStateRegistry.StartingAnchorRoot({
            gameType: GameType.wrap(0), // Cannon
            outputRoot: OutputRoot({
                l2BlockNumber: 0,
                root: Hash.wrap(0xe50e5ae025f1d11b8862a170621c095c7d571463d6854f34f4352a445dd17f9f)
            })
        });

        // Step 4: Upgrade back to original implementation and reinitialize with the new starting roots
        bytes memory initData = abi.encodeCall(IAnchorStateRegistry.initialize, (startingRoots, superchain));
        IProxyAdmin(proxyAdmin).upgradeAndCall(payable(address(_anchorStateRegistry)), asrImpl, initData);
        vm.stopBroadcast();

        // Check that the anchor state registry has been reinitialized
        (Hash root0, uint256 l2BlockNumber0) = anchorStateRegistry.anchors(GameType.wrap(0));
        require(keccak256(abi.encode(root0)) == keccak256(abi.encode(startingRoots[0].outputRoot.root)));
        require(l2BlockNumber0 == startingRoots[0].outputRoot.l2BlockNumber);
    }
}
