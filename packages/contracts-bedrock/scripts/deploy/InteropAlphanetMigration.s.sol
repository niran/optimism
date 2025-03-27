// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Script } from "forge-std/Script.sol";
import { BaseDeployIO } from "scripts/deploy/BaseDeployIO.sol";
import { IProxyAdmin } from "interfaces/universal/IProxyAdmin.sol";
import { ISystemConfig } from "interfaces/L1/ISystemConfig.sol";
import { IOPContractsManagerInteropMigrator, IOPContractsManager } from "interfaces/L1/IOPContractsManager.sol";
import { Claim, Duration, Proposal, Hash } from "src/dispute/lib/Types.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
import { IOptimismPortal2 as IOptimismPortal } from "interfaces/L1/IOptimismPortal2.sol";
import { console2 as console } from "forge-std/console2.sol";

contract InteropAlphanetMigration is Script {
    function run() public {
        IOPContractsManager opcm = IOPContractsManager(0xEB32e20EbDE266A769a5683CC80976f05D9e6e7B);
        bytes32 absolutePrestate = hex"0387beeb10e2139751e069ad40f0d1f0fa91b6076fa6f2d5dd488d453a46eec6";
        bool usePermissionlessGame = true;
        Proposal memory startingAnchorRoot = Proposal({ root: Hash.wrap(hex"dd69e5f8f65f27ed413cb31f80070ec961b3dd5ca8898269cade08699d9303f6"), l2SequenceNumber: 1743027458 });
        address proposer = 0x4d522194aa103df731F2e6eB74cF2005FD6C48F5;
        address challenger = 0x544078E6C0A7dFC220E096026E99ee87773d1624;
        uint64 maxGameDepth = 73;
        uint64 splitDepth = 30;
        uint256 initBond = 0.08 ether;
        Duration clockExtension = Duration.wrap(10800);
        Duration maxClockDuration = Duration.wrap(302400);

        IOPContractsManager.OpChainConfig[] memory opChainConfigs = new IOPContractsManager.OpChainConfig[](2);
        opChainConfigs[0] = IOPContractsManager.OpChainConfig({
            systemConfigProxy: ISystemConfig(0x38CFB302cdA19FD376bE2237D220D35C404A36bA),
            proxyAdmin: IProxyAdmin(0x8a2dF05608B2AE0Eb75809b210527dd1d2705E31),
            absolutePrestate: Claim.wrap(absolutePrestate)
        });
        opChainConfigs[1] = IOPContractsManager.OpChainConfig({
            systemConfigProxy: ISystemConfig(0xCE1da8571d67d139A8040EBa35BEF8cfd34a0F2f),
            proxyAdmin: IProxyAdmin(0x6220fd818ea70803B65DfABA7DEb8E72C8Fb396E),
            absolutePrestate: Claim.wrap(absolutePrestate)
        });

        IOPContractsManagerInteropMigrator.MigrateInput memory inputs = IOPContractsManagerInteropMigrator.MigrateInput({
            usePermissionlessGame: usePermissionlessGame,
            startingAnchorRoot: startingAnchorRoot,
            gameParameters: IOPContractsManagerInteropMigrator.GameParameters({
                proposer: proposer,
                challenger: challenger,
                maxGameDepth: maxGameDepth,
                splitDepth: splitDepth,
                initBond: initBond,
                clockExtension: clockExtension,
                maxClockDuration: maxClockDuration
            }),
            opChainConfigs: opChainConfigs
        });

        bytes memory cd = abi.encodeWithSelector(IOPContractsManager.migrate.selector, inputs);
        console.log("calldata: ");
        console.logBytes(cd);

        //vm.broadcast(msg.sender);
        //opcm.migrate(inputs);
    }
}
