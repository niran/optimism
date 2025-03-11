// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

// Forge
import { Script } from "forge-std/Script.sol";

// Scripts
import { BaseDeployIO } from "scripts/deploy/BaseDeployIO.sol";
import { DeployUtils } from "scripts/libraries/DeployUtils.sol";

// Interfaces
import { OPContractsManager } from "src/L1/OPContractsManager.sol";
import { ISystemConfig } from "interfaces/L1/ISystemConfig.sol";
import { IProxyAdmin } from "interfaces/universal/IProxyAdmin.sol";
import { IDelayedWETH } from "interfaces/dispute/IDelayedWETH.sol";
import { IBigStepper } from "interfaces/dispute/IBigStepper.sol";
import { GameType, Duration, Claim } from "src/dispute/lib/Types.sol";
import { IFaultDisputeGame } from "interfaces/dispute/IFaultDisputeGame.sol";

/// @title AddGameTypeInput
contract AddGameTypeInput is BaseDeployIO {
    // Address that will be used for the DummyCaller contract
    address internal _prank;
    // OPCM contract address
    OPContractsManager internal _opcm;
    // SystemConfig contract address
    ISystemConfig internal _systemConfig;
    // ProxyAdmin contract address
    IProxyAdmin internal _proxyAdmin;
    // DelayedWETH contract address (optional)
    IDelayedWETH internal _delayedWETH;
    // Game type to add
    GameType internal _disputeGameType;
    // Absolute prestate for the game
    Claim internal _disputeAbsolutePrestate;
    // Maximum game depth
    uint256 internal _disputeMaxGameDepth;
    // Split depth for the game
    uint256 internal _disputeSplitDepth;
    // Clock extension duration
    Duration internal _disputeClockExtension;
    // Maximum clock duration
    Duration internal _disputeMaxClockDuration;
    // Initial bond amount
    uint256 internal _initialBond;
    // VM contract address
    IBigStepper internal _vm;
    // Whether this is a permissioned game
    bool internal _permissioned;
    // Salt mixer for deterministic addresses
    string internal _saltMixer;

    function set(bytes4 _sel, address _value) public {
        if (_sel == this.prank.selector) {
            require(_value != address(0), "AddGameType: prank cannot be zero address");
            _prank = _value;
        } else if (_sel == this.opcm.selector) {
            require(_value != address(0), "AddGameType: opcm cannot be zero address");
            _opcm = OPContractsManager(_value);
        } else if (_sel == this.systemConfig.selector) {
            require(_value != address(0), "AddGameType: systemConfig cannot be zero address");
            _systemConfig = ISystemConfig(_value);
        } else if (_sel == this.proxyAdmin.selector) {
            require(_value != address(0), "AddGameType: proxyAdmin cannot be zero address");
            _proxyAdmin = IProxyAdmin(_value);
        } else if (_sel == this.delayedWETH.selector) {
            _delayedWETH = IDelayedWETH(payable(_value));
        } else if (_sel == this.proofVM.selector) {
            require(_value != address(0), "AddGameType: vm cannot be zero address");
            _vm = IBigStepper(_value);
        } else {
            revert("AddGameType: unknown selector");
        }
    }

    function set(bytes4 _sel, uint256 _value) public {
        if (_sel == this.disputeMaxGameDepth.selector) {
            require(_value > 0, "AddGameType: maxGameDepth must be greater than 0");
            _disputeMaxGameDepth = _value;
        } else if (_sel == this.disputeSplitDepth.selector) {
            _disputeSplitDepth = _value;
        } else if (_sel == this.initialBond.selector) {
            require(_value > 0, "AddGameType: initialBond must be greater than 0");
            _initialBond = _value;
        } else {
            revert("AddGameType: unknown selector");
        }
    }

    function set(bytes4 _sel, GameType _value) public {
        if (_sel == this.disputeGameType.selector) {
            _disputeGameType = _value;
        } else {
            revert("AddGameType: unknown selector");
        }
    }

    function set(bytes4 _sel, Claim _value) public {
        if (_sel == this.disputeAbsolutePrestate.selector) {
            _disputeAbsolutePrestate = _value;
        } else {
            revert("AddGameType: unknown selector");
        }
    }

    function set(bytes4 _sel, Duration _value) public {
        if (_sel == this.disputeClockExtension.selector) {
            _disputeClockExtension = _value;
        } else if (_sel == this.disputeMaxClockDuration.selector) {
            _disputeMaxClockDuration = _value;
        } else {
            revert("AddGameType: unknown selector");
        }
    }

    function set(bytes4 _sel, bool _value) public {
        if (_sel == this.permissioned.selector) {
            _permissioned = _value;
        } else {
            revert("AddGameType: unknown selector");
        }
    }

    function set(bytes4 _sel, string memory _value) public {
        if (_sel == this.saltMixer.selector) {
            _saltMixer = _value;
        } else {
            revert("AddGameType: unknown selector");
        }
    }

    function prank() public view returns (address) {
        require(_prank != address(0), "AddGameType: prank not set");
        return _prank;
    }

    function opcm() public view returns (OPContractsManager) {
        require(address(_opcm) != address(0), "AddGameType: opcm not set");
        return _opcm;
    }

    function systemConfig() public view returns (ISystemConfig) {
        require(address(_systemConfig) != address(0), "AddGameType: systemConfig not set");
        return _systemConfig;
    }

    function proxyAdmin() public view returns (IProxyAdmin) {
        require(address(_proxyAdmin) != address(0), "AddGameType: proxyAdmin not set");
        return _proxyAdmin;
    }

    function delayedWETH() public view returns (IDelayedWETH) {
        return _delayedWETH;
    }

    function disputeGameType() public view returns (GameType) {
        return _disputeGameType;
    }

    function disputeAbsolutePrestate() public view returns (Claim) {
        return _disputeAbsolutePrestate;
    }

    function disputeMaxGameDepth() public view returns (uint256) {
        require(_disputeMaxGameDepth > 0, "AddGameType: maxGameDepth not set");
        return _disputeMaxGameDepth;
    }

    function disputeSplitDepth() public view returns (uint256) {
        return _disputeSplitDepth;
    }

    function disputeClockExtension() public view returns (Duration) {
        return _disputeClockExtension;
    }

    function disputeMaxClockDuration() public view returns (Duration) {
        return _disputeMaxClockDuration;
    }

    function initialBond() public view returns (uint256) {
        require(_initialBond > 0, "AddGameType: initialBond not set");
        return _initialBond;
    }

    function proofVM() public view returns (IBigStepper) {
        require(address(_vm) != address(0), "AddGameType: vm not set");
        return _vm;
    }

    function permissioned() public view returns (bool) {
        return _permissioned;
    }

    function saltMixer() public view returns (string memory) {
        return _saltMixer;
    }
}

/// @title AddGameTypeOutput
contract AddGameTypeOutput is BaseDeployIO {
    IDelayedWETH internal _delayedWETH;
    IFaultDisputeGame internal _faultDisputeGame;

    function set(bytes4 _sel, address _value) public {
        require(_value != address(0), "AddGameType: address cannot be zero");
        if (_sel == this.delayedWETH.selector) {
            _delayedWETH = IDelayedWETH(payable(_value));
        } else if (_sel == this.faultDisputeGame.selector) {
            _faultDisputeGame = IFaultDisputeGame(_value);
        } else {
            revert("AddGameType: unknown selector");
        }
    }

    function delayedWETH() public view returns (IDelayedWETH) {
        DeployUtils.assertValidContractAddress(address(_delayedWETH));
        return _delayedWETH;
    }

    function faultDisputeGame() public view returns (IFaultDisputeGame) {
        DeployUtils.assertValidContractAddress(address(_faultDisputeGame));
        return _faultDisputeGame;
    }
}

/// @title AddGameType
contract AddGameType is Script {
    function run(AddGameTypeInput _agi, AddGameTypeOutput _ago) public {
        addGameType(_agi, _ago);
        checkOutput(_ago);
    }

    function addGameType(AddGameTypeInput _agi, AddGameTypeOutput _ago) internal {
        // Create the game input
        OPContractsManager.AddGameInput[] memory gameConfigs = new OPContractsManager.AddGameInput[](1);
        gameConfigs[0] = OPContractsManager.AddGameInput({
            saltMixer: _agi.saltMixer(),
            systemConfig: _agi.systemConfig(),
            proxyAdmin: _agi.proxyAdmin(),
            delayedWETH: _agi.delayedWETH(),
            disputeGameType: _agi.disputeGameType(),
            disputeAbsolutePrestate: _agi.disputeAbsolutePrestate(),
            disputeMaxGameDepth: _agi.disputeMaxGameDepth(),
            disputeSplitDepth: _agi.disputeSplitDepth(),
            disputeClockExtension: _agi.disputeClockExtension(),
            disputeMaxClockDuration: _agi.disputeMaxClockDuration(),
            initialBond: _agi.initialBond(),
            vm: _agi.proofVM(),
            permissioned: _agi.permissioned()
        });

        // Etch DummyCaller contract
        address prank = _agi.prank();
        bytes memory code = vm.getDeployedCode("AddGameType.s.sol:DummyCaller");
        vm.etch(prank, code);
        vm.store(prank, bytes32(0), bytes32(uint256(uint160(address(_agi.opcm())))));
        vm.label(prank, "DummyCaller");

        // Call into the DummyCaller to perform the delegatecall
        vm.broadcast(msg.sender);
        (bool success, bytes memory result) = DummyCaller(prank).addGameType(gameConfigs);
        require(success, "AddGameType: addGameType failed");

        // Decode the result and set it in the output
        OPContractsManager.AddGameOutput[] memory outputs = abi.decode(result, (OPContractsManager.AddGameOutput[]));
        require(outputs.length == 1, "AddGameType: unexpected number of outputs");
        _ago.set(_ago.delayedWETH.selector, address(outputs[0].delayedWETH));
        _ago.set(_ago.faultDisputeGame.selector, address(outputs[0].faultDisputeGame));
    }

    function checkOutput(AddGameTypeOutput _ago) internal view {
        DeployUtils.assertValidContractAddress(address(_ago.delayedWETH()));
        DeployUtils.assertValidContractAddress(address(_ago.faultDisputeGame()));
    }
}

/// @title DummyCaller
contract DummyCaller {
    address internal _opcmAddr;

    function addGameType(OPContractsManager.AddGameInput[] memory _gameConfigs) external returns (bool, bytes memory) {
        bytes memory data = abi.encodeCall(DummyCaller.addGameType, _gameConfigs);
        (bool success, bytes memory result) = _opcmAddr.delegatecall(data);
        return (success, result);
    }
}
