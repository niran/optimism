# Smart Contract Pull Request Review Policy

This document provides comprehensive guidelines for reviewing pull requests to smart contracts in the Optimism codebase. These guidelines are designed to ensure code quality, maintain backwards compatibility, and enable AI-assisted code review tools to effectively assess changes.

## Review Checklist

### Style Guide Compliance

All smart contract changes MUST adhere to the [Smart Contract Style Guide](../contributing/style-guide.md).

### ABI Compatibility

When ABI snapshots are modified for any contract, reviewers MUST verify:

1. **Backwards Compatibility Analysis**:
   - Compare the old and new ABI snapshots located in `snapshots/abi/`
   - Ensure no existing function signatures are removed or modified unless strictly necessary
   - Verify that new functions do not conflict with existing ones
   - Check that event signatures remain unchanged unless explicitly intended

2. **Breaking Change Documentation**:
   - If backwards compatibility is broken, the author MUST explicitly call out these changes in the PR description
   - The author MUST provide a migration plan or explanation of how breaking changes will be addressed
   - Consider if the change requires a major version bump per semver rules

### Storage Layout Compatibility

When storage layout snapshots are modified for any contract, reviewers MUST verify:

1. **Layout Preservation**:
    - Compare old and new storage layout snapshots in `snapshots/storageLayout/`
    - Ensure existing storage slots are not modified, reordered, or removed unless strictly necessary
    - Verify that new storage variables are appended at the end of the layout
    - Check that spacer variables maintain their positions and sizes
    - If a storage variable is removed, it MUST be replaced with an appropriately sized spacer variable
    - If no spacer is added when removing a variable, the author MUST explicitly confirm that the contract will be redeployed rather than upgraded

2. **Proxy Compatibility**:
   - For upgradeable contracts, ensure storage layout changes maintain proxy compatibility
   - Verify that inheritance hierarchy changes do not affect storage layout

3. **Breaking Change Mitigation**:
   - If storage layout compatibility is broken, the author MUST document the incompatibility
   - The author MUST specify whether a new contract deployment is required
   - Migration strategies for existing deployed contracts MUST be provided
