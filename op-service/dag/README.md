# Blockchain DAG Framework

> A unified library for blockchain DAG operations that replaces manual chain traversal with efficient, cached operations.

## Problem: Manual Chain Traversal

Blockchain applications commonly require traversing parent-child relationships between blocks. The typical implementation looks like this:

```go
// Manual implementation
func isAncestorManual(ethClient *sources.EthClient, ancestorHash, descendantHash common.Hash) bool {
    ctx := context.Background()
    current := descendantHash

    for current != ancestorHash {
        block, err := ethClient.BlockByHash(ctx, current)
        if err != nil || block.ParentHash() == (common.Hash{}) {
            return false // Network call failed or reached genesis
        }
        current = block.ParentHash() // Another network call
    }
    return true
}
```

**Issues with this approach:**
- **Reliability**: Edge cases (genesis, missing blocks, network failures) handled inconsistently
- **Maintainability**: Traversal logic duplicated across codebase
- **Efficiency**: No caching of intermediate results
- **Correctness**: Susceptible to off-by-one errors and infinite loops

## Solution: Unified DAG Framework

This framework provides **two layers** of abstraction:

1. **Low-level DAG library**: Generic, set-based operations on any directed acyclic graph
2. **High-level blockchain DSL**: Blockchain-specific operations with elegant syntax


### Framework Implementation
```go
// Same operation using the framework
store := NewEthClientStore(ctx, ethClient, logger)
isAncestor := store.Query(descendantHash).Ancestors().Contains(ancestorHash)
```

The framework provides the same functionality with improved performance, comprehensive edge case handling, and reduced boilerplate.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    Blockchain DSL                           │
│  store.Query(hash).Ancestors().Contains(target)             │
│  store.Query(hash).LatestCommonAncestor(other)              │
│  store.Reorg(oldHead, newHead).IsReorg()                    │
└─────────────────────────────────────────────────────────────┘
                                │
┌─────────────────────────────────────────────────────────────┐
│                   Low-Level DAG Library                     │
│  dag.Ancestors(dag.ID(hash)).Contains(store, target)        │
│  dag.Intersect(dag.Ancestors(a), dag.Ancestors(b))          │
│  dag.Slice(start, end)                                      │
└─────────────────────────────────────────────────────────────┘
                                │
┌─────────────────────────────────────────────────────────────┐
│                      Store Layer                            │
│  EthClientStore: RPC-backed block resolution                │
│  HybridStore: Local index + RPC fallback                    │
│  TestStore: In-memory for testing                           │
└─────────────────────────────────────────────────────────────┘
```

## Layer 1: Low-Level DAG Operations

The foundation is a generic DAG library that works on any directed acyclic graph:

```go
store := NewEthClientStore(ctx, ethClient, logger)

// Set-based operations
ancestorSet := dag.Ancestors(dag.ID(blockHash))
parentSet := dag.Parents(dag.ID(blockHash))
childrenSet := dag.Children(dag.ID(blockHash))

// Check membership
isAncestor := ancestorSet.Contains(store, candidateHash)

// Set algebra
commonAncestors := dag.Intersect(
    dag.Ancestors(dag.ID(blockA)),
    dag.Ancestors(dag.ID(blockB)),
)

// Slice operations (Mercurial-style)
chainSlice := dag.Slice(dag.ID(startBlock), dag.ID(endBlock))
```

**Features:**
- **Lazy evaluation**: Operations computed only when needed
- **Caching**: Results memoized to avoid repeated work
- **Set algebra**: Union, intersection, difference operations
- **Depth optimization**: Uses block depth for early termination

## Layer 2: Blockchain-Specific DSL

The high-level DSL adds blockchain single-parent assumptions and cleaner syntax:

```go
store := NewEthClientStore(ctx, ethClient, logger)

// Fluent interface for common operations
query := store.Query(blockHash)

// Single parent (blockchain assumption)
parentHash, hasParent := query.Parent()

// Ancestry checking
isAncestor := query.Ancestors().Contains(candidateHash)

// Block number (depth from genesis)
blockNum, ok := query.BlockNumber()

// Chain distance
distance, valid := query.DistanceFrom(otherHash)

// Latest Common Ancestor (returns result directly)
lca, found := query.LatestCommonAncestor(otherHash)
```

## Advanced Features

### 1. Hybrid Store for Reorg Handling

The `HybridStore` combines RPC access with local indexing for reorged blocks:

```go
// Create hybrid store
hybridStore := NewHybridStore(ctx, ethClient, logger)

// Index reorged blocks that are no longer available via RPC
reorgedBlock := eth.L1BlockRef{
    Hash:       common.HexToHash("0xreorged..."),
    Number:     12345,
    ParentHash: common.HexToHash("0xparent..."),
    Time:       1234567890,
}
hybridStore.IndexBlockRef(reorgedBlock)

// Now you can query relationships involving reorged blocks
lca, found := hybridStore.Query(canonicalHash).LatestCommonAncestor(reorgedHash)
```

### 2. Manual Reorg Analysis

The framework provides manual reorg analysis capabilities:
```go
// Analyze reorgs between two chain heads
reorg := store.Reorg(oldHead, newHead)
if reorg.IsReorg() {
    depth := reorg.Depth()
    lca, _ := reorg.LCA()
    oldChain, _ := reorg.OldChain()
    newChain, _ := reorg.NewChain()

    // Process the reorg...
}
```



### 3. Builder Pattern for Complex Queries

For sophisticated set operations:

```go
complexSet := NewBlockBuilder().
    Add(blockA).                    // Include specific block
    AddAncestors(blockB).          // Include all ancestors
    AddSlice(blockC, blockD).      // Include chain segment
    AddLatestCommonAncestor(blockE, blockF). // Include LCA
    QueryWith(store)

// Check membership or enumerate
contains := complexSet.Contains(candidateHash)
allBlocks, err := complexSet.Eval()
```

## Performance Deep Dive

### Caching

The framework implements caching within individual query operations:

```go
// Caching within a single query object
ancestorsOfA := store.Query(blockA).Ancestors()
isAncestor1 := ancestorsOfA.Contains(blockB) // Network calls + caching
isAncestor2 := ancestorsOfA.Contains(blockC) // Uses cached traversal results
isAncestor3 := ancestorsOfA.Contains(blockD) // Uses cached traversal results

// Store-level caching for block lookups
block1, _ := store.Node(blockHash) // Network call
block2, _ := store.Node(blockHash) // Cached (same block)
```

### Depth-Bounded Traversals

Uses block depth for early termination during membership checks:

```go
ancestorSet := dag.Ancestors(dag.ID(laterBlock))

// Depth optimization: if candidate is later than target, immediately returns false
earlierCandidate := someEarlierBlockHash
isAncestor := ancestorSet.Contains(store, earlierCandidate) // Returns false without traversal

// Only traverses when candidate could potentially be an ancestor
laterCandidate := someLaterBlockHash
isAncestor2 := ancestorSet.Contains(store, laterCandidate) // May traverse upward from laterBlock
```

### Enumeration Support

Some operations can enumerate their full contents:

```go
// Descendants and Children are enumerable (when store supports NodesAtDepth)
descendants, err := dag.Descendants(dag.ID(blockHash)).Eval(store)
children, err := dag.Children(dag.ID(blockHash)).Eval(store)

// Ancestors and Parents return ErrEnumerationDisabled (expensive reverse lookup)
_, err := dag.Ancestors(dag.ID(blockHash)).Eval(store) // Returns ErrEnumerationDisabled
```

## Real-World Usage Patterns

### Pattern 1: Ancestry Checking
```go
// Question: "Is block X an ancestor of block Y?"
isAncestor := store.Query(descendantHash).Ancestors().Contains(ancestorHash)
```

### Pattern 2: Latest Common Ancestor
```go
// Question: "What's the most recent common ancestor?"
lca, found := store.Query(blockA).LatestCommonAncestor(blockB)
if found {
    // Use lca
}
```

### Pattern 3: Chain Segment Extraction
```go
// Question: "Give me all blocks between A and B"
segment, err := store.Query(endBlock).GetChainSegment(startBlock, maxBlocks)
```

### Pattern 4: Reorg Analysis
```go
// Question: "Did a reorg happen between these heads?"
reorg := store.Reorg(oldHead, newHead)
if reorg.IsReorg() {
    depth := reorg.Depth()
    affectedBlocks := reorg.OldChain()
}
```

## Testing Support

The framework includes comprehensive testing utilities:

```go
// In-memory store for tests
store := NewTestMemStore[common.Hash]()
store.Add(&Block{BlockRef: testBlock})

// All operations work the same way
isAncestor := store.Query(blockHash).Ancestors().Contains(ancestorHash)
```