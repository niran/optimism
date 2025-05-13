package activation

import (
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
)

// Check handles activation checks against a given dependency set.
type Check struct {
	depSet depset.DependencySet
	logger log.Logger
}

// NewCheck creates a new Check object with the provided dependency set.
func NewCheck(depSet depset.DependencySet, logger log.Logger) *Check {
	return &Check{
		depSet: depSet,
		logger: logger,
	}
}

// Check checks if interop is active for a given chain and timestamp.
func (c *Check) Check(chain eth.ChainID, timestamp uint64) bool {
	// If we have a nil pointer or no dependency set then interop is never active
	if c == nil || c.depSet == nil {
		return false
	}

	// Interop is active if the chain can execute at the given timestamp
	canExecute, err := c.depSet.CanExecuteAt(chain, timestamp)
	if err != nil {
		c.logger.Debug("Error checking interop activation", "chain", chain, "timestamp", timestamp, "err", err)
		return false
	}
	return canExecute
}

// IsActivationBlock checks if a block is the activation block for interop.
// The activation block is the first block with a timestamp that crosses the activation threshold.
// This function checks both the block and its parent to determine if this is the transition block.
func (c *Check) IsActivationBlock(block eth.BlockRef, prevBlock eth.BlockRef, chain eth.ChainID) bool {
	// If we have a nil pointer or no dependency set then interop is never active
	if c == nil || c.depSet == nil {
		return false
	}

	// Check if interop is active for this block
	isActive := c.Check(chain, block.Time)
	if !isActive {
		// If interop isn't active, this definitely isn't an activation block
		return false
	}

	// If we don't have a previous block reference (empty struct), consider this a potential
	// activation block if it's active. This is a special case for startup.
	if prevBlock == (eth.BlockRef{}) {
		c.logger.Info("No previous block to compare against for activation check",
			"chain", chain, "block", block)
		return true
	}

	// Check if interop was active for the previous block
	wasActive := c.Check(chain, prevBlock.Time)

	// This is the activation block if interop is active now but wasn't active for the previous block
	isActivation := isActive && !wasActive
	if isActivation {
		c.logger.Info("Detected interop activation block",
			"chain", chain,
			"block", block,
			"timestamp", block.Time,
			"previousBlock", prevBlock,
			"previousTimestamp", prevBlock.Time)
	}

	return isActivation
}
