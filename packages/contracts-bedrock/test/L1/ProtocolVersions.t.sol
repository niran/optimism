// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

// Testing
import { CommonTest } from "test/setup/CommonTest.sol";
import { EIP1967Helper } from "test/mocks/EIP1967Helper.sol";

// Interfaces
import { IProxy } from "interfaces/universal/IProxy.sol";
import { IProtocolVersions, ProtocolVersion } from "interfaces/L1/IProtocolVersions.sol";

/// @title ProtocolVersions Test Init
/// @notice Test initialization for ProtocolVersions tests.
contract ProtocolVersions_TestInit is CommonTest {
    event ConfigUpdate(uint256 indexed version, IProtocolVersions.UpdateType indexed updateType, bytes data);

    ProtocolVersion required;
    ProtocolVersion recommended;

    function setUp() public virtual override {
        super.setUp();
        required = ProtocolVersion.wrap(deploy.cfg().requiredProtocolVersion());
        recommended = ProtocolVersion.wrap(deploy.cfg().recommendedProtocolVersion());
    }
}

/// @title ProtocolVersions_Initialize_Test
/// @notice Test contract for ProtocolVersions `initialize` function.
contract ProtocolVersions_Initialize_Test is ProtocolVersions_TestInit {
    /// @notice Tests that initialization sets the correct values.
    function test_initialize_values_succeeds() external {
        skipIfForkTest(
            "ProtocolVersions_Initialize_Test: cannot test initialization on forked network against hardhat config"
        );
        IProtocolVersions protocolVersionsImpl = IProtocolVersions(artifacts.mustGetAddress("ProtocolVersionsImpl"));
        address owner = deploy.cfg().finalSystemOwner();

        assertEq(ProtocolVersion.unwrap(protocolVersions.required()), ProtocolVersion.unwrap(required));
        assertEq(ProtocolVersion.unwrap(protocolVersions.recommended()), ProtocolVersion.unwrap(recommended));
        assertEq(protocolVersions.owner(), owner);

        assertEq(ProtocolVersion.unwrap(protocolVersionsImpl.required()), 0);
        assertEq(ProtocolVersion.unwrap(protocolVersionsImpl.recommended()), 0);
        assertEq(protocolVersionsImpl.owner(), address(0));
    }

    /// @notice Ensures that the events are emitted during initialization.
    function test_initialize_events_succeeds() external {
        IProtocolVersions protocolVersionsImpl = IProtocolVersions(artifacts.mustGetAddress("ProtocolVersionsImpl"));

        // Wipe out the initialized slot so the proxy can be initialized again
        vm.store(address(protocolVersions), bytes32(0), bytes32(0));

        // The order depends here
        vm.expectEmit(true, true, true, true, address(protocolVersions));
        emit ConfigUpdate(0, IProtocolVersions.UpdateType.REQUIRED_PROTOCOL_VERSION, abi.encode(required));
        vm.expectEmit(true, true, true, true, address(protocolVersions));
        emit ConfigUpdate(0, IProtocolVersions.UpdateType.RECOMMENDED_PROTOCOL_VERSION, abi.encode(recommended));

        vm.prank(EIP1967Helper.getAdmin(address(protocolVersions)));
        IProxy(payable(address(protocolVersions))).upgradeToAndCall(
            address(protocolVersionsImpl),
            abi.encodeCall(
                IProtocolVersions.initialize,
                (
                    alice, // _owner
                    required, // _required
                    recommended // recommended
                )
            )
        );
    }
}

/// @title ProtocolVersions_VERSION_Test
/// @notice Test contract for ProtocolVersions `VERSION` constant.
contract ProtocolVersions_VERSION_Test is ProtocolVersions_TestInit {
    /// @notice Tests that VERSION constant returns the expected value.
    function test_VERSION_succeeds() external view {
        assertEq(protocolVersions.VERSION(), 0);
    }
}

/// @title ProtocolVersions_REQUIREDSLOT_Test
/// @notice Test contract for ProtocolVersions `REQUIRED_SLOT` constant.
contract ProtocolVersions_REQUIREDSLOT_Test is ProtocolVersions_TestInit {
    /// @notice Tests that REQUIRED_SLOT constant returns the expected value.
    function test_REQUIREDSLOT_succeeds() external view {
        bytes32 expectedSlot = bytes32(uint256(keccak256("protocolversion.required")) - 1);
        assertEq(protocolVersions.REQUIRED_SLOT(), expectedSlot);
    }
}

/// @title ProtocolVersions_RECOMMENDEDSLOT_Test
/// @notice Test contract for ProtocolVersions `RECOMMENDED_SLOT` constant.
contract ProtocolVersions_RECOMMENDEDSLOT_Test is ProtocolVersions_TestInit {
    /// @notice Tests that RECOMMENDED_SLOT constant returns the expected value.
    function test_RECOMMENDEDSLOT_succeeds() external view {
        bytes32 expectedSlot = bytes32(uint256(keccak256("protocolversion.recommended")) - 1);
        assertEq(protocolVersions.RECOMMENDED_SLOT(), expectedSlot);
    }
}

/// @title ProtocolVersions_Version_Test
/// @notice Test contract for ProtocolVersions `version` function.
contract ProtocolVersions_Version_Test is ProtocolVersions_TestInit {
    /// @notice Tests that version returns the expected semantic version string.
    function test_version_succeeds() external view {
        assertEq(protocolVersions.version(), "1.1.0");
    }
}

/// @title ProtocolVersions_Required_Test
/// @notice Test contract for ProtocolVersions `required` function.
contract ProtocolVersions_Required_Test is ProtocolVersions_TestInit {
    /// @notice Tests that required getter returns current value.
    function test_required_succeeds() external view {
        assertEq(ProtocolVersion.unwrap(protocolVersions.required()), ProtocolVersion.unwrap(required));
    }
}

/// @title ProtocolVersions_Recommended_Test
/// @notice Test contract for ProtocolVersions `recommended` function.
contract ProtocolVersions_Recommended_Test is ProtocolVersions_TestInit {
    /// @notice Tests that recommended getter returns current value.
    function test_recommended_succeeds() external view {
        assertEq(ProtocolVersion.unwrap(protocolVersions.recommended()), ProtocolVersion.unwrap(recommended));
    }
}

/// @title ProtocolVersions_SetRequired_Test
/// @notice Test contract for ProtocolVersions `setRequired` function.
contract ProtocolVersions_SetRequired_Test is ProtocolVersions_TestInit {
    /// @notice Tests that `setRequired` updates the required protocol version successfully.
    /// @param _version The protocol version to set as required.
    function testFuzz_setRequired_succeeds(uint256 _version) external {
        vm.expectEmit(true, true, true, true);
        emit ConfigUpdate(0, IProtocolVersions.UpdateType.REQUIRED_PROTOCOL_VERSION, abi.encode(_version));

        vm.prank(protocolVersions.owner());
        protocolVersions.setRequired(ProtocolVersion.wrap(_version));
        assertEq(ProtocolVersion.unwrap(protocolVersions.required()), _version);
    }

    /// @notice Tests that `setRequired` reverts if the caller is not the owner.
    /// @param _caller The address of the unauthorized caller.
    function testFuzz_setRequired_notOwner_reverts(address _caller) external {
        // Use vm.assume to exclude the actual owner
        vm.assume(_caller != protocolVersions.owner());

        vm.expectRevert("Ownable: caller is not the owner");
        vm.prank(_caller);
        protocolVersions.setRequired(ProtocolVersion.wrap(0));
    }
}

/// @title ProtocolVersions_SetRecommended_Test
/// @notice Test contract for ProtocolVersions `setRecommended` function.
contract ProtocolVersions_SetRecommended_Test is ProtocolVersions_TestInit {
    /// @notice Tests that `setRecommended` updates the recommended protocol version successfully.
    /// @param _version The protocol version to set as recommended.
    function testFuzz_setRecommended_succeeds(uint256 _version) external {
        vm.expectEmit(true, true, true, true);
        emit ConfigUpdate(0, IProtocolVersions.UpdateType.RECOMMENDED_PROTOCOL_VERSION, abi.encode(_version));

        vm.prank(protocolVersions.owner());
        protocolVersions.setRecommended(ProtocolVersion.wrap(_version));
        assertEq(ProtocolVersion.unwrap(protocolVersions.recommended()), _version);
    }

    /// @notice Tests that `setRecommended` reverts if the caller is not the owner.
    /// @param _caller The address of the unauthorized caller.
    function testFuzz_setRecommended_notOwner_reverts(address _caller) external {
        // Use vm.assume to exclude the actual owner
        vm.assume(_caller != protocolVersions.owner());

        vm.expectRevert("Ownable: caller is not the owner");
        vm.prank(_caller);
        protocolVersions.setRecommended(ProtocolVersion.wrap(0));
    }
}
