// Package adapters provides a fluent DSL (Domain Specific Language) for blockchain DAG operations.
// This file contains syntactic sugar that makes DAG queries more readable and intuitive for blockchain use cases.
//
// BLOCKCHAIN ASSUMPTIONS:
//   - Each block has exactly one parent (except genesis which has no parent)
//   - Blocks form a tree structure, not a general DAG
//   - Block relationships follow blockchain semantics (parent/child, ancestor/descendant)
//
// Instead of writing:
//
//	dag.Descendants(dag.ID(blockHash))
//
// You can write:
//
//	block.Descendants()
//
// Or with a store:
//
//	store.Query(blockHash).Descendants().Contains(otherHash)
package adapters

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-service/dag"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
)

// BlockQuery provides a fluent interface for DAG operations on blockchain blocks.
// It wraps a block hash and provides methods that return NodeSet queries.
type BlockQuery struct {
	hash common.Hash
}

// From creates a fluent BlockQuery wrapper around the given block hash.
// This is the entry point for the fluent blockchain DSL.
//
// Example:
//
//	ancestors := adapters.From(blockHash).Ancestors()
//	descendants := adapters.From(blockHash).Descendants()
func From(hash common.Hash) *BlockQuery {
	return &BlockQuery{hash: hash}
}

// FromBlock creates a fluent BlockQuery from a Block instance.
func FromBlock(block *Block) *BlockQuery {
	return &BlockQuery{hash: block.ID()}
}

// FromBlockRef creates a fluent BlockQuery from an eth.BlockRef.
func FromBlockRef(ref eth.BlockRef) *BlockQuery {
	return &BlockQuery{hash: ref.Hash}
}

// Hash returns the underlying block hash.
func (bq *BlockQuery) Hash() common.Hash {
	return bq.hash
}

// Ancestors returns a NodeSet of all ancestors of this block.
// Note: A block is NOT its own ancestor.
func (bq *BlockQuery) Ancestors() dag.NodeSet[common.Hash] {
	return dag.Ancestors(dag.ID(bq.hash))
}

// Descendants returns a NodeSet of all descendants of this block.
// Note: A block is NOT its own descendant.
func (bq *BlockQuery) Descendants() dag.NodeSet[common.Hash] {
	return dag.Descendants(dag.ID(bq.hash))
}

// Parent returns the single parent block hash, or zero hash if this is genesis.
// This is a blockchain-specific convenience method assuming single parent.
func (bq *BlockQuery) Parent(store *EthClientStore) (common.Hash, bool) {
	if node, ok := store.Node(bq.hash); ok {
		if block, ok := node.(*Block); ok {
			parents := block.Parents()
			if len(parents) > 0 {
				return parents[0], true
			}
		}
	}
	return common.Hash{}, false
}

// Children returns a NodeSet of all direct children of this block.
// Note: A block is NOT its own child.
// In blockchain context: returns all blocks that have this block as their parent.
func (bq *BlockQuery) Children() dag.NodeSet[common.Hash] {
	return dag.Children(dag.ID(bq.hash))
}

// SliceTo returns a NodeSet representing a Mercurial-style slice from this block to the target block.
// The slice includes both endpoints (this block and the target block).
func (bq *BlockQuery) SliceTo(target common.Hash) dag.NodeSet[common.Hash] {
	return dag.Slice(dag.ID(bq.hash), dag.ID(target))
}

// SliceFrom returns a NodeSet representing a Mercurial-style slice from the source block to this block.
// The slice includes both endpoints (the source block and this block).
func (bq *BlockQuery) SliceFrom(source common.Hash) dag.NodeSet[common.Hash] {
	return dag.Slice(dag.ID(source), dag.ID(bq.hash))
}

// LatestCommonAncestor returns the latest common ancestor of this block and the others.
// Returns the LCA hash and true if found, or zero hash and false if no common ancestor exists.
func (bq *BlockQuery) LatestCommonAncestor(store *EthClientStore, others ...common.Hash) (common.Hash, bool) {
	hashes := make([]common.Hash, len(others)+1)
	hashes[0] = bq.hash
	copy(hashes[1:], others)
	return findBlockchainLCA(store, hashes)
}

// UnionWith returns a NodeSet that is the union of this block with other blocks.
func (bq *BlockQuery) UnionWith(others ...common.Hash) dag.NodeSet[common.Hash] {
	sets := make([]dag.NodeSet[common.Hash], len(others)+1)
	sets[0] = dag.ID(bq.hash)
	for i, other := range others {
		sets[i+1] = dag.ID(other)
	}
	return dag.Union(sets...)
}

// StoreQuery provides a fluent interface for DAG operations with an EthClientStore context.
// It combines an EthClientStore and a block hash to provide immediate query execution.
type StoreQuery struct {
	store *EthClientStore
	hash  common.Hash
}

// Query creates a fluent StoreQuery that combines an EthClientStore and block hash.
// This allows for immediate execution of DAG queries.
//
// Example:
//
//	isAncestor := store.Query(blockHash).Ancestors().Contains(otherHash)
//	descendants := store.Query(blockHash).Descendants().Eval()
func (s *EthClientStore) Query(hash common.Hash) *StoreQuery {
	return &StoreQuery{store: s, hash: hash}
}

// Hash returns the block hash being queried.
func (sq *StoreQuery) Hash() common.Hash {
	return sq.hash
}

// Store returns the underlying EthClientStore.
func (sq *StoreQuery) Store() *EthClientStore {
	return sq.store
}

// Ancestors returns a StoreQuerySet for all ancestors of this block.
func (sq *StoreQuery) Ancestors() *StoreQuerySet {
	return &StoreQuerySet{
		store:   sq.store,
		nodeSet: dag.Ancestors(dag.ID(sq.hash)),
	}
}

// Descendants returns a StoreQuerySet for all descendants of this block.
func (sq *StoreQuery) Descendants() *StoreQuerySet {
	return &StoreQuerySet{
		store:   sq.store,
		nodeSet: dag.Descendants(dag.ID(sq.hash)),
	}
}

// Parents returns a StoreQuerySet for all direct parents of this block.
// In blockchain context: returns the single parent block (or empty for genesis).
func (sq *StoreQuery) Parents() *StoreQuerySet {
	return &StoreQuerySet{
		store:   sq.store,
		nodeSet: dag.Parents(dag.ID(sq.hash)),
	}
}

// Parent returns the single parent block hash, or zero hash if this is genesis.
// This is a blockchain-specific convenience method assuming single parent.
func (sq *StoreQuery) Parent() (common.Hash, bool) {
	if node, ok := sq.store.Node(sq.hash); ok {
		if block, ok := node.(*Block); ok {
			parents := block.Parents()
			if len(parents) > 0 {
				return parents[0], true
			}
		}
	}
	return common.Hash{}, false
}

// Children returns a StoreQuerySet for all direct children of this block.
// In blockchain context: returns all blocks that have this block as their parent.
func (sq *StoreQuery) Children() *StoreQuerySet {
	return &StoreQuerySet{
		store:   sq.store,
		nodeSet: dag.Children(dag.ID(sq.hash)),
	}
}

// SliceTo returns a StoreQuerySet for a slice from this block to the target.
func (sq *StoreQuery) SliceTo(target common.Hash) *StoreQuerySet {
	return &StoreQuerySet{
		store:   sq.store,
		nodeSet: dag.Slice(dag.ID(sq.hash), dag.ID(target)),
	}
}

// SliceFrom returns a StoreQuerySet for a slice from the source to this block.
func (sq *StoreQuery) SliceFrom(source common.Hash) *StoreQuerySet {
	return &StoreQuerySet{
		store:   sq.store,
		nodeSet: dag.Slice(dag.ID(source), dag.ID(sq.hash)),
	}
}

// LatestCommonAncestor returns the latest common ancestor of this block and the others.
// Returns the LCA hash and true if found, or zero hash and false if no common ancestor exists.
func (sq *StoreQuery) LatestCommonAncestor(others ...common.Hash) (common.Hash, bool) {
	hashes := make([]common.Hash, len(others)+1)
	hashes[0] = sq.hash
	copy(hashes[1:], others)
	return findBlockchainLCA(sq.store, hashes)
}

// Blockchain-specific traversal methods

// IsGenesis checks if this block is the genesis block (has no parent).
func (sq *StoreQuery) IsGenesis() bool {
	_, hasParent := sq.Parent()
	return !hasParent
}

// BlockNumber returns the block number (depth) of this block.
func (sq *StoreQuery) BlockNumber() (uint64, bool) {
	if node, ok := sq.store.Node(sq.hash); ok {
		if block, ok := node.(*Block); ok {
			return block.BlockRef.Number, true
		}
	}
	return 0, false
}

// IsOnChain checks if this block is an ancestor of the given head block, or is the head itself.
// This is useful for determining if a block is on a particular chain branch.
func (sq *StoreQuery) IsOnChain(tipHash common.Hash) bool {
	// A block is on the main chain if it's the tip itself or an ancestor of the tip
	return sq.hash == tipHash || sq.store.Query(tipHash).Ancestors().Contains(sq.hash)
}

// DistanceFrom calculates the block distance between this block and another.
// Returns the number of blocks between them, or -1 if they're not on the same chain.
func (sq *StoreQuery) DistanceFrom(other common.Hash) (int, bool) {
	thisNum, thisOk := sq.BlockNumber()
	if !thisOk {
		return -1, false
	}

	otherQuery := sq.store.Query(other)
	otherNum, otherOk := otherQuery.BlockNumber()
	if !otherOk {
		return -1, false
	}

	// Check if they're on the same chain
	if thisNum > otherNum {
		// Check if other is ancestor of this
		if sq.Ancestors().Contains(other) {
			return int(thisNum - otherNum), true
		}
	} else if otherNum > thisNum {
		// Check if this is ancestor of other
		if otherQuery.Ancestors().Contains(sq.hash) {
			return int(otherNum - thisNum), true
		}
	} else {
		// Same block number - only same if same hash
		if sq.hash == other {
			return 0, true
		}
	}

	return -1, false
}

// GetChainSegment returns a slice of block hashes from this block back to the ancestor.
// This assumes a linear chain (blockchain) and returns blocks in reverse chronological order.
func (sq *StoreQuery) GetChainSegment(ancestorHash common.Hash, maxBlocks int) ([]common.Hash, error) {
	// TODO: We should not have hardcoded limits like these.
	if maxBlocks <= 0 {
		maxBlocks = 1000 // Default reasonable limit
	}

	var chain []common.Hash
	currentHash := sq.hash

	for len(chain) < maxBlocks {
		chain = append(chain, currentHash)

		// Stop if we reached the target ancestor
		if currentHash == ancestorHash {
			return chain, nil
		}

		// Get parent
		currentQuery := sq.store.Query(currentHash)
		parentHash, hasParent := currentQuery.Parent()
		if !hasParent {
			// Reached genesis without finding ancestor
			break
		}

		currentHash = parentHash
	}

	// If we didn't find the ancestor, check if it's actually an ancestor
	if !sq.Ancestors().Contains(ancestorHash) {
		return nil, fmt.Errorf("block %s is not an ancestor of %s", ancestorHash, sq.hash)
	}

	return chain, nil
}

// StoreQuerySet provides immediate execution methods for NodeSet operations with an EthClientStore.
// It wraps a NodeSet with an EthClientStore to enable direct Contains and Eval calls.
type StoreQuerySet struct {
	store   *EthClientStore
	nodeSet dag.NodeSet[common.Hash]
}

// Contains checks if the given block hash is a member of this query set.
func (sqs *StoreQuerySet) Contains(hash common.Hash) bool {
	return sqs.nodeSet.Contains(sqs.store, hash)
}

// Eval attempts to enumerate all block hashes in this query set.
// Returns dag.ErrEnumerationDisabled if the set refuses to enumerate.
func (sqs *StoreQuerySet) Eval() (map[common.Hash]struct{}, error) {
	return sqs.nodeSet.Eval(sqs.store)
}

// NodeSet returns the underlying NodeSet for further composition.
func (sqs *StoreQuerySet) NodeSet() dag.NodeSet[common.Hash] {
	return sqs.nodeSet
}

// Store returns the underlying EthClientStore.
func (sqs *StoreQuerySet) Store() *EthClientStore {
	return sqs.store
}

// UnionWith combines this query set with other NodeSets.
func (sqs *StoreQuerySet) UnionWith(others ...dag.NodeSet[common.Hash]) *StoreQuerySet {
	sets := make([]dag.NodeSet[common.Hash], len(others)+1)
	sets[0] = sqs.nodeSet
	copy(sets[1:], others)
	return &StoreQuerySet{
		store:   sqs.store,
		nodeSet: dag.Union(sets...),
	}
}

// IntersectWith finds the intersection of this query set with other NodeSets.
func (sqs *StoreQuerySet) IntersectWith(others ...dag.NodeSet[common.Hash]) *StoreQuerySet {
	sets := make([]dag.NodeSet[common.Hash], len(others)+1)
	sets[0] = sqs.nodeSet
	copy(sets[1:], others)
	return &StoreQuerySet{
		store:   sqs.store,
		nodeSet: dag.Intersect(sets...),
	}
}

// Except returns a query set with the given NodeSets removed.
func (sqs *StoreQuerySet) Except(others ...dag.NodeSet[common.Hash]) *StoreQuerySet {
	result := sqs.nodeSet
	for _, other := range others {
		result = dag.Diff(result, other)
	}
	return &StoreQuerySet{
		store:   sqs.store,
		nodeSet: result,
	}
}

// BlockBuilder provides a fluent interface for constructing complex blockchain DAG queries.
// It accumulates multiple NodeSets and can combine them with Union or Intersect operations.
type BlockBuilder struct {
	sets []dag.NodeSet[common.Hash]
}

// NewBlockBuilder creates a new empty BlockBuilder.
func NewBlockBuilder() *BlockBuilder {
	return &BlockBuilder{
		sets: make([]dag.NodeSet[common.Hash], 0),
	}
}

// Add includes a single block in the builder.
func (bb *BlockBuilder) Add(hash common.Hash) *BlockBuilder {
	bb.sets = append(bb.sets, dag.ID(hash))
	return bb
}

// AddBlock includes a single Block in the builder.
func (bb *BlockBuilder) AddBlock(block *Block) *BlockBuilder {
	bb.sets = append(bb.sets, dag.ID(block.ID()))
	return bb
}

// AddBlockRef includes a single eth.BlockRef in the builder.
func (bb *BlockBuilder) AddBlockRef(ref eth.BlockRef) *BlockBuilder {
	bb.sets = append(bb.sets, dag.ID(ref.Hash))
	return bb
}

// AddAncestors includes all ancestors of the given block.
func (bb *BlockBuilder) AddAncestors(hash common.Hash) *BlockBuilder {
	bb.sets = append(bb.sets, dag.Ancestors(dag.ID(hash)))
	return bb
}

// AddDescendants includes all descendants of the given block.
func (bb *BlockBuilder) AddDescendants(hash common.Hash) *BlockBuilder {
	bb.sets = append(bb.sets, dag.Descendants(dag.ID(hash)))
	return bb
}

// AddSlice includes a Mercurial-style slice between two blocks.
func (bb *BlockBuilder) AddSlice(start, end common.Hash) *BlockBuilder {
	bb.sets = append(bb.sets, dag.Slice(dag.ID(start), dag.ID(end)))
	return bb
}

// AddLatestCommonAncestor adds a placeholder for LCA computation.
// Note: The actual LCA computation is deferred until QueryWith() is called with a store.
// Without a store, this method cannot compute the actual LCA and will include all input hashes.
func (bb *BlockBuilder) AddLatestCommonAncestor(hashes ...common.Hash) *BlockBuilder {
	if len(hashes) > 0 {
		// Store the LCA request to be processed when QueryWith is called
		bb.sets = append(bb.sets, &lcaPlaceholder{hashes: hashes})
	}
	return bb
}

// lcaPlaceholder represents a deferred LCA computation
type lcaPlaceholder struct {
	hashes []common.Hash
}

func (lp *lcaPlaceholder) Contains(store dag.Store[common.Hash], id common.Hash) bool {
	// When evaluated with a store, compute the actual LCA
	if ethStore, ok := store.(*EthClientStore); ok {
		lca, found := findBlockchainLCA(ethStore, lp.hashes)
		return found && lca == id
	}
	// Fallback: check if id is one of the input hashes
	for _, hash := range lp.hashes {
		if hash == id {
			return true
		}
	}
	return false
}

func (lp *lcaPlaceholder) Eval(store dag.Store[common.Hash]) (map[common.Hash]struct{}, error) {
	// When evaluated with a store, compute the actual LCA
	if ethStore, ok := store.(*EthClientStore); ok {
		lca, found := findBlockchainLCA(ethStore, lp.hashes)
		if found {
			return map[common.Hash]struct{}{lca: {}}, nil
		}
		return map[common.Hash]struct{}{}, nil
	}
	// Fallback: return all input hashes
	result := make(map[common.Hash]struct{})
	for _, hash := range lp.hashes {
		result[hash] = struct{}{}
	}
	return result, nil
}

// Union combines all accumulated sets with a union operation.
func (bb *BlockBuilder) Union() dag.NodeSet[common.Hash] {
	if len(bb.sets) == 0 {
		return dag.Union[common.Hash]()
	}
	return dag.Union(bb.sets...)
}

// Intersect combines all accumulated sets with an intersection operation.
func (bb *BlockBuilder) Intersect() dag.NodeSet[common.Hash] {
	if len(bb.sets) == 0 {
		return dag.Union[common.Hash]() // Empty set for intersection
	}
	return dag.Intersect(bb.sets...)
}

// QueryWith creates a StoreQuerySet using the given store and the builder's result.
func (bb *BlockBuilder) QueryWith(store *EthClientStore) *StoreQuerySet {
	return &StoreQuerySet{
		store:   store,
		nodeSet: bb.Union(),
	}
}

// ReorgQuery provides specialized methods for analyzing blockchain reorganizations.
type ReorgQuery struct {
	store  *EthClientStore
	oldTip common.Hash
	newTip common.Hash
	lca    *common.Hash // Cached LCA
	depth  *int         // Cached depth
}

// Reorg creates a new ReorgQuery for analyzing a potential reorganization between two chain tips.
func (s *EthClientStore) Reorg(oldTip, newTip common.Hash) *ReorgQuery {
	return &ReorgQuery{
		store:  s,
		oldTip: oldTip,
		newTip: newTip,
	}
}

// IsReorg checks if this represents an actual reorganization (not just a simple extension).
func (rq *ReorgQuery) IsReorg() bool {
	// If both tips are the same, there's no reorganization
	if rq.oldTip == rq.newTip {
		return false
	}

	// If the old tip is an ancestor of the new tip, it's just a chain extension
	return !rq.store.Query(rq.newTip).Ancestors().Contains(rq.oldTip)
}

// LCA returns the latest common ancestor of the old and new tips.
func (rq *ReorgQuery) LCA() (common.Hash, bool) {
	if rq.lca != nil {
		return *rq.lca, *rq.lca != (common.Hash{})
	}

	// Use the efficient blockchain-specific LCA implementation
	lca, found := findBlockchainLCA(rq.store, []common.Hash{rq.oldTip, rq.newTip})
	rq.lca = &lca
	return lca, found
}

// Depth returns the depth of the reorganization (number of blocks to revert).
func (rq *ReorgQuery) Depth() int {
	if rq.depth != nil {
		return *rq.depth
	}

	lca, ok := rq.LCA()
	if !ok {
		depth := -1
		rq.depth = &depth
		return -1
	}

	distance, ok := rq.store.Query(rq.oldTip).DistanceFrom(lca)
	if !ok {
		depth := -1
		rq.depth = &depth
		return -1
	}

	rq.depth = &distance
	return distance
}

// OldChain returns the blocks that need to be reverted (from old tip back to LCA, exclusive).
func (rq *ReorgQuery) OldChain() ([]common.Hash, error) {
	lca, ok := rq.LCA()
	if !ok {
		return nil, fmt.Errorf("no common ancestor found between %s and %s", rq.oldTip, rq.newTip)
	}

	chain, err := rq.store.Query(rq.oldTip).GetChainSegment(lca, 1000)
	if err != nil {
		return nil, err
	}

	// Remove the LCA from the result (exclusive)
	if len(chain) > 0 && chain[len(chain)-1] == lca {
		chain = chain[:len(chain)-1]
	}

	return chain, nil
}

// NewChain returns the blocks that need to be applied (from LCA to new tip, exclusive).
func (rq *ReorgQuery) NewChain() ([]common.Hash, error) {
	lca, ok := rq.LCA()
	if !ok {
		return nil, fmt.Errorf("no common ancestor found between %s and %s", rq.oldTip, rq.newTip)
	}

	chain, err := rq.store.Query(rq.newTip).GetChainSegment(lca, 1000)
	if err != nil {
		return nil, err
	}

	// Remove the LCA from the result (exclusive)
	if len(chain) > 0 && chain[len(chain)-1] == lca {
		chain = chain[:len(chain)-1]
	}

	return chain, nil
}

// Summary returns a human-readable summary of the reorganization.
func (rq *ReorgQuery) Summary() string {
	if !rq.IsReorg() {
		return "No reorganization - simple chain extension"
	}

	depth := rq.Depth()
	if depth < 0 {
		return "Invalid reorganization - no common ancestor"
	}

	lca, _ := rq.LCA()
	return fmt.Sprintf("Reorganization detected: depth=%d, LCA=%s", depth, lca.Hex()[:10])
}
