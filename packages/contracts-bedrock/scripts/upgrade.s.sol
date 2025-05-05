// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { console2 as console } from "forge-std/console2.sol";
import { Script } from "forge-std/Script.sol";
import { IOPContractsManagerInteropMigrator, IOPContractsManager } from "interfaces/L1/IOPContractsManager.sol";
import { Duration, Proposal, Hash } from "src/dispute/lib/Types.sol";
import { stdJson } from "forge-std/StdJson.sol";
import { Vm } from "forge-std/Vm.sol";
import { ISystemConfig } from "interfaces/L1/ISystemConfig.sol";
import { IProxyAdmin } from "interfaces/universal/IProxyAdmin.sol";
import { Claim } from "src/dispute/lib/Types.sol";
import { GnosisSafe as Safe } from "safe-contracts/GnosisSafe.sol";
import { Enum } from "safe-contracts/common/Enum.sol";

contract Upgrade is Script {
    function run() external view {
        // Read the JSON file
        string memory jsonStr = vm.readFile("migrate_input.json");
        // Get the target contract address from environment
        address targetAddress = vm.envAddress("TARGET_ADDRESS");

        // Extract values from JSON
        bool usePermissionlessGame = stdJson.readBool(jsonStr, ".usePermissionlessGame");

        bytes32 root = stdJson.readBytes32(jsonStr, ".startingAnchorRoot.root");
        uint256 l2SequenceNumber = stdJson.readUint(jsonStr, ".startingAnchorRoot.l2SequenceNumber");

        address proposer = stdJson.readAddress(jsonStr, ".gameParameters.proposer");
        address challenger = stdJson.readAddress(jsonStr, ".gameParameters.challenger");
        uint256 maxGameDepth = stdJson.readUint(jsonStr, ".gameParameters.maxGameDepth");
        uint256 splitDepth = stdJson.readUint(jsonStr, ".gameParameters.splitDepth");
        uint256 initBond = stdJson.readUint(jsonStr, ".gameParameters.initBond");
        uint64 clockExtension = uint64(stdJson.readUint(jsonStr, ".gameParameters.clockExtension"));
        uint64 maxClockDuration = uint64(stdJson.readUint(jsonStr, ".gameParameters.maxClockDuration"));

        // Parse opChainConfigs array
        bytes memory configsBytes = vm.parseJson(jsonStr, ".opChainConfigs");
        IOPContractsManager.OpChainConfig[] memory opChainConfigs = abi.decode(configsBytes, (IOPContractsManager.OpChainConfig[]));

        IOPContractsManagerInteropMigrator.MigrateInput memory inputs = IOPContractsManagerInteropMigrator.MigrateInput({
            usePermissionlessGame: usePermissionlessGame,
            startingAnchorRoot: Proposal({
                root: Hash.wrap(root),
                l2SequenceNumber: l2SequenceNumber
            }),
            gameParameters: IOPContractsManagerInteropMigrator.GameParameters({
                proposer: proposer,
                challenger: challenger,
                maxGameDepth: maxGameDepth,
                splitDepth: splitDepth,
                initBond: initBond,
                clockExtension: Duration.wrap(clockExtension),
                maxClockDuration: Duration.wrap(maxClockDuration)
            }),
            opChainConfigs: opChainConfigs
        });

        bytes memory data = abi.encodeCall(IOPContractsManager.migrate, inputs);

        // Encode the Safe transaction calldata
        bytes memory safeCalldata = abi.encodeCall(
            Safe.execTransaction,
            (
                targetAddress,  // to
                0,             // value
                data,          // data
                Enum.Operation.DelegateCall,  // operation
                0,             // safeTxGas
                0,             // baseGas
                0,             // gasPrice
                address(0),    // gasToken
                payable(0),    // refundReceiver
                ""            // signatures (empty for now, needs to be signed by owners)
            )
        );

        console.log("Safe transaction calldata:");
        console.logBytes(safeCalldata);
    }
}
