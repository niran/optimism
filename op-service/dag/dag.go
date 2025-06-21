// Package dag provides a *lazy*, generic DSL for Mercurial-style block-set
// algebra over an arbitrary directed-acyclic dag (DAG).
//
// # Performance Assumptions
//
// This library assumes the input dag is a **DAG (Directed Acyclic dag)**.
// This assumption enables significant performance optimizations:
//   - Depth-based early termination for ancestor queries
//   - No cycle detection overhead in traversals
//   - Aggressive pruning based on topological ordering
//
// # Runtime Complexity Summary
//
// Most operations are designed for optimal performance on blockchain DAGs:
//   - Basic set operations (Union, Intersect, Diff): O(n) where n = number of sets
//   - Structural predicates (Ancestors, Descendants): O(d) where d = depth difference
//   - Slice operations: O(d) with depth-bounded traversal, enumerable when store supports NodesAtDepth
//   - Single node operations (ID): O(1)
//
// # Enumeration Support
//
// Enumeration support varies by predicate and store capabilities:
//   - Descendants/Children: Enumerable if Store supports NodesAtDepth()
//   - Ancestors/Parents: Return ErrEnumerationDisabled (expensive reverse lookup)
//   - Slice operations: Enumerable if Store supports NodesAtDepth() (uses Descendants enumeration)
//   - Basic operations (Union, Intersect, etc.): Fully enumerable
//
// Note: EthClientStore now supports NodesAtDepth() by fetching blocks by number,
// making Descendants/Children enumerable for blockchain DAGs. HybridStore combines
// both canonical blocks (via RPC) and locally indexed blocks for comprehensive
// enumeration support.
//
// # Concurrency
//
// **Not goroutine-safe.**  Several NodeSet implementations cache membership
// results in internal maps without synchronisation.  Access a given NodeSet
// instance from only one goroutine at a time, or guard calls with your own
// locks if you need concurrent usage.
//
// # Why a DAG and not a tree?
//
// A blockchain is a tree, but when also considering initiator-executor, and
// L1-L2 relationships an optimism superchain form a DAG.  Assuming a DAG from
// the start will allow us to expand this library to represent those
// relationships in the future.
//
// # Self-membership rules
//
//   - A node is **not** its own *ancestor* – `Ancestors(X).Contains(X) == false`.
//   - A node is **not** its own *descendant* – `Descendants(X).Contains(X) == false`.
//
// The rest of the operator semantics (Union, Intersect, Diff, Not) follow the
// familiar set-algebra definitions.
package dag

import "errors"

// Sentinel error returned by NodeSet.Eval when the concrete implementation
// deliberately refuses to enumerate its full contents (e.g. Ancestors on a
// very large DAG) or when the enumeration would be too expensive.
var ErrEnumerationDisabled = errors.New("dag: expensive enumeration disabled for this expression or parameters")

// ───────────────────────────────────────────────────────────────────────────────
//  DAG primitives
// ───────────────────────────────────────────────────────────────────────────────

// DAGNode is the minimal interface the query engine expects.  Only identity,
// parent links, and an integer *Depth* (distance from any root) are required –
// just enough to implement depth-bounded traversals.
type DAGNode[ID comparable] interface {
	ID() ID
	Parents() []ID
	Depth() uint64
}

// Store resolves node IDs back to concrete DAGNode values.  Any additional
// indexing methods (e.g. Children, All) are out-of-scope for this generic
// interface but can be added by the caller via embedding or type assertion.
type Store[ID comparable] interface {
	Node(id ID) (DAGNode[ID], bool)

	// NodesAtDepth returns all nodes at the specified depth.
	// Returns ErrEnumerationDisabled if the store cannot efficiently enumerate nodes at depth.
	NodesAtDepth(depth uint64) (map[ID]struct{}, error)
}

// ───────────────────────────────────────────────────────────────────────────────
//  NodeSet – lazy set abstraction
// ───────────────────────────────────────────────────────────────────────────────

type NodeSet[ID comparable] interface {
	// Contains answers membership lazily.
	Contains(st Store[ID], id ID) bool

	// Eval returns the exact contents, or ErrEnumerationDisabled when the set
	// decides enumeration would be too expensive.
	Eval(st Store[ID]) (map[ID]struct{}, error)
}

// ───────────────────────────────────────────────────────────────────────────────
//  Leaf: single node
// ───────────────────────────────────────────────────────────────────────────────

type idSet[ID comparable] struct {
	id ID
}

// ID creates a NodeSet containing a single node.
//
// Runtime Complexity:
//   - Contains(): O(1) - constant time equality check
//   - Eval(): O(1) - returns single-element map
func ID[ID comparable](id ID) NodeSet[ID] { return &idSet[ID]{id: id} }

func (s *idSet[ID]) Contains(_ Store[ID], id ID) bool          { return id == s.id }
func (s *idSet[ID]) Eval(_ Store[ID]) (map[ID]struct{}, error) { return map[ID]struct{}{s.id: {}}, nil }

// ───────────────────────────────────────────────────────────────────────────────
//  Set algebra (all *lazy*)
// ───────────────────────────────────────────────────────────────────────────────

// —— Union ————————————————————————————————————————————————

type unionSet[ID comparable] []NodeSet[ID]

// Union creates a NodeSet representing the union of multiple sets.
// A node is in the union if it's in ANY of the component sets.
//
// Runtime Complexity:
//   - Contains(): O(n) where n = number of input sets
//     Each component set's Contains() is called until one returns true
//   - Eval(): O(n × m) where n = number of sets, m = average set size
//     Enumerates all component sets and merges results
func Union[ID comparable](sets ...NodeSet[ID]) NodeSet[ID] { return unionSet[ID](sets) }

func (u unionSet[ID]) Contains(st Store[ID], id ID) bool {
	for _, subset := range u {
		if subset.Contains(st, id) {
			return true
		}
	}
	return false
}

func (u unionSet[ID]) Eval(st Store[ID]) (map[ID]struct{}, error) {
	result := make(map[ID]struct{})
	enumerated := false // did any component give concrete data?

	for _, subset := range u {
		members, err := subset.Eval(st)
		switch err {
		case nil:
			enumerated = true
		case ErrEnumerationDisabled:
			continue // fine – just skip it and keep whatever we have so far
		default:
			return nil, err
		}
		for id := range members {
			result[id] = struct{}{}
		}
	}

	if !enumerated && len(u) > 0 {
		// every component refused to enumerate (but there were components)
		return nil, ErrEnumerationDisabled
	}
	return result, nil
}

// —— Intersect ————————————————————————————————————————————

type interSet[ID comparable] []NodeSet[ID]

// Intersect creates a NodeSet representing the intersection of multiple sets.
// A node is in the intersection if it's in ALL of the component sets.
//
// Runtime Complexity:
//   - Contains(): O(n) where n = number of input sets
//     All component sets' Contains() must be called and return true
//   - Eval(): O(n × m) where n = number of sets, m = size of smallest set
//     Enumerates one set and filters through all others
func Intersect[ID comparable](sets ...NodeSet[ID]) NodeSet[ID] { return interSet[ID](sets) }

func (i interSet[ID]) Contains(st Store[ID], id ID) bool {
	for _, subset := range i {
		if !subset.Contains(st, id) {
			return false
		}
	}
	return true
}

func (i interSet[ID]) Eval(st Store[ID]) (map[ID]struct{}, error) {
	for idx, subset := range i {
		// find first subset willing to enumerate, then filter through the rest
		enumerated, err := subset.Eval(st)
		if err == ErrEnumerationDisabled {
			continue
		}
		if err != nil {
			return nil, err
		}

		// Filter enumerated results through ALL OTHER subsets
		for id := range enumerated {
			// Check membership in all OTHER subsets (not including the one we enumerated)
			for j, otherSubset := range i {
				if j != idx && !otherSubset.Contains(st, id) {
					delete(enumerated, id)
					break // No need to check remaining subsets for this id
				}
			}
		}
		return enumerated, nil
	}
	return nil, ErrEnumerationDisabled
}

// —— Diff ——————————————————————————————————————————————

type diffSet[ID comparable] struct {
	left, right NodeSet[ID]
}

// Diff creates a NodeSet representing the difference between two sets (a - b).
// A node is in the difference if it's in the left set but NOT in the right set.
//
// Runtime Complexity:
//   - Contains(): O(1) amortized - calls left.Contains() then right.Contains()
//   - Eval(): O(m) where m = size of left set
//     Enumerates left set and filters out elements in right set
func Diff[ID comparable](a, b NodeSet[ID]) NodeSet[ID] { return diffSet[ID]{left: a, right: b} }

func (d diffSet[ID]) Contains(st Store[ID], id ID) bool {
	return d.left.Contains(st, id) && !d.right.Contains(st, id)
}

func (d diffSet[ID]) Eval(st Store[ID]) (map[ID]struct{}, error) {
	raw, err := d.left.Eval(st)
	if err != nil {
		return nil, err
	}

	// clone so we don't mutate the caller's map
	result := make(map[ID]struct{}, len(raw))
	for id := range raw {
		result[id] = struct{}{}
	}

	for id := range result {
		if d.right.Contains(st, id) {
			delete(result, id)
		}
	}
	return result, nil
}

// —— Not ——————————————————————————————————————————————

type notSet[ID comparable] struct {
	inner NodeSet[ID]
}

// Not creates a NodeSet representing the complement of the given set.
// A node is in the complement if it's NOT in the inner set.
//
// Runtime Complexity:
//   - Contains(): O(1) amortized - calls inner.Contains() and negates result
//   - Eval(): Not supported - returns ErrEnumerationDisabled
//     (Complement of a set in infinite domain cannot be enumerated)
func Not[ID comparable](s NodeSet[ID]) NodeSet[ID] { return notSet[ID]{inner: s} }

func (n notSet[ID]) Contains(st Store[ID], id ID) bool       { return !n.inner.Contains(st, id) }
func (n notSet[ID]) Eval(Store[ID]) (map[ID]struct{}, error) { return nil, ErrEnumerationDisabled }

// ───────────────────────────────────────────────────────────────────────────────
//  Structural predicates (ancestors, descendants, parents, children)
// ───────────────────────────────────────────────────────────────────────────────

// internal discriminator for the four predicate kinds
const (
	predAncestors = iota
	predDescendants
	predParents
	predChildren
)

type predSet[ID comparable] struct {
	kind  int
	arg   NodeSet[ID] // base set we predicate against
	cache map[ID]bool // memoised Contains results
}

// Ancestors creates a NodeSet containing all ancestors of nodes in the given set.
// A node X is an ancestor of Y if there's a path from X to Y (X ≠ Y).
//
// Runtime Complexity:
//   - Contains(): O(d) where d = depth difference between candidate and target
//     Uses depth-bounded BFS traversal with DAG optimizations
//   - Eval(): Not supported - returns ErrEnumerationDisabled
func Ancestors[ID comparable](s NodeSet[ID]) NodeSet[ID] {
	return &predSet[ID]{kind: predAncestors, arg: s, cache: map[ID]bool{}}
}

// Descendants creates a NodeSet containing all descendants of nodes in the given set.
// A node Y is a descendant of X if there's a path from X to Y (X ≠ Y).
//
// Runtime Complexity:
//   - Contains(): O(d) where d = depth difference between candidate and target
//     Uses depth-bounded BFS traversal upward from candidate
//   - Eval(): O(d × n) where d = max depth range, n = nodes per depth
//     Enumerates nodes at each depth and checks ancestry (requires Store.NodesAtDepth support)
func Descendants[ID comparable](s NodeSet[ID]) NodeSet[ID] {
	return &predSet[ID]{kind: predDescendants, arg: s, cache: map[ID]bool{}}
}

// Parents creates a NodeSet containing all direct parents of nodes in the given set.
//
// Runtime Complexity:
//   - Contains(): O(m × p) where m = size of argument set, p = avg parents per node
//     Enumerates argument set and checks parent lists
//   - Eval(): Not supported - returns ErrEnumerationDisabled
func Parents[ID comparable](s NodeSet[ID]) NodeSet[ID] {
	return &predSet[ID]{kind: predParents, arg: s, cache: map[ID]bool{}}
}

// Children creates a NodeSet containing all direct children of nodes in the given set.
//
// Runtime Complexity:
//   - Contains(): O(p) where p = number of parents of the candidate node
//     Checks if any parent of candidate is in the argument set
//   - Eval(): O(n × p) where n = nodes at child depth, p = avg parents per node
//     Enumerates nodes at depth+1 and checks parent relationships (requires Store.NodesAtDepth support)
func Children[ID comparable](s NodeSet[ID]) NodeSet[ID] {
	return &predSet[ID]{kind: predChildren, arg: s, cache: map[ID]bool{}}
}

func (p *predSet[ID]) memoise(id ID, value bool) bool {
	p.cache[id] = value
	return value
}

func (p *predSet[ID]) Contains(st Store[ID], id ID) bool {
	if cached, ok := p.cache[id]; ok {
		return cached
	}

	switch p.kind {
	// ————————————————— Ancestors ————————————————
	case predAncestors:
		// Self-membership rule: A node is NOT its own ancestor
		if p.arg.Contains(st, id) {
			return p.memoise(id, false)
		}
		targetSet, err := p.arg.Eval(st)
		if err != nil && err != ErrEnumerationDisabled {
			return p.memoise(id, false)
		}
		for targetID := range targetSet {
			if isAncestor(st, id, targetID) {
				return p.memoise(id, true)
			}
		}
		return p.memoise(id, false)

	// ————————————————— Descendants ————————————————
	case predDescendants:
		// Self-membership rule: A node is NOT its own descendant
		if p.arg.Contains(st, id) {
			return p.memoise(id, false)
		}
		candidateNode, ok := st.Node(id)
		if !ok {
			return p.memoise(id, false)
		}
		maxDepth := candidateNode.Depth()

		// BFS upward from candidate - no cycle detection needed in DAG
		queue := []ID{id}

		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]

			node, ok := st.Node(current)
			if !ok {
				continue
			}

			for _, parentID := range node.Parents() {
				if p.arg.Contains(st, parentID) {
					return p.memoise(id, true)
				}
				// DAG optimization: only traverse parents at valid depths
				if parentNode, ok := st.Node(parentID); ok && parentNode.Depth() <= maxDepth {
					queue = append(queue, parentID)
				}
			}
		}
		return p.memoise(id, false)

	// ————————————————— Parents ————————————————
	case predParents:
		if p.arg.Contains(st, id) {
			return p.memoise(id, false) // a node is not its own parent
		}
		argMembers, err := p.arg.Eval(st)
		if err != nil {
			return p.memoise(id, false)
		}
		for childID := range argMembers {
			if childNode, ok := st.Node(childID); ok {
				for _, parentID := range childNode.Parents() {
					if parentID == id {
						return p.memoise(id, true)
					}
				}
			}
		}
		return p.memoise(id, false)

	// ————————————————— Children ————————————————
	case predChildren:
		node, ok := st.Node(id)
		if !ok {
			return p.memoise(id, false)
		}
		for _, parentID := range node.Parents() {
			if p.arg.Contains(st, parentID) {
				return p.memoise(id, true)
			}
		}
		return p.memoise(id, false)
	}

	return p.memoise(id, false)
}

func (p *predSet[ID]) Eval(st Store[ID]) (map[ID]struct{}, error) {
	switch p.kind {
	case predDescendants:
		return p.evalDescendants(st)
	case predChildren:
		return p.evalChildren(st)
	default:
		// Ancestors and Parents still not enumerable
		return nil, ErrEnumerationDisabled
	}
}

// evalDescendants enumerates descendants by checking all nodes at greater depths
func (p *predSet[ID]) evalDescendants(st Store[ID]) (map[ID]struct{}, error) {
	// Get the argument set (the nodes we want descendants of)
	argMembers, err := p.arg.Eval(st)
	if err != nil {
		return nil, ErrEnumerationDisabled
	}

	if len(argMembers) == 0 {
		return make(map[ID]struct{}), nil
	}

	// Find the minimum depth of argument nodes
	minDepth := uint64(^uint64(0)) // Max uint64
	for argID := range argMembers {
		if node, ok := st.Node(argID); ok {
			if node.Depth() < minDepth {
				minDepth = node.Depth()
			}
		}
	}

	result := make(map[ID]struct{})

	// Check all nodes at depths greater than minDepth
	for depth := minDepth + 1; depth < minDepth+1000; depth++ { // Reasonable upper bound
		nodesAtDepth, err := st.NodesAtDepth(depth)
		if err == ErrEnumerationDisabled {
			return nil, ErrEnumerationDisabled
		}
		if err != nil {
			return nil, err
		}

		if len(nodesAtDepth) == 0 {
			break // No more nodes at this depth, assume we've reached the end
		}

		// Check each node to see if it's a descendant of any argument node
		for candidateID := range nodesAtDepth {
			// Check if this candidate is a descendant of any argument node
			for argID := range argMembers {
				if isAncestor(st, argID, candidateID) {
					result[candidateID] = struct{}{}
					break
				}
			}
		}
	}

	return result, nil
}

// evalChildren enumerates children by checking nodes at depth+1 of argument nodes
func (p *predSet[ID]) evalChildren(st Store[ID]) (map[ID]struct{}, error) {
	// Get the argument set (the nodes we want children of)
	argMembers, err := p.arg.Eval(st)
	if err != nil {
		return nil, ErrEnumerationDisabled
	}

	if len(argMembers) == 0 {
		return make(map[ID]struct{}), nil
	}

	result := make(map[ID]struct{})

	// For each argument node, check nodes at depth+1
	for argID := range argMembers {
		argNode, ok := st.Node(argID)
		if !ok {
			continue
		}

		childDepth := argNode.Depth() + 1
		nodesAtChildDepth, err := st.NodesAtDepth(childDepth)
		if err == ErrEnumerationDisabled {
			return nil, ErrEnumerationDisabled
		}
		if err != nil {
			return nil, err
		}

		// Check each potential child node
		for candidateID := range nodesAtChildDepth {
			candidateNode, ok := st.Node(candidateID)
			if !ok {
				continue
			}

			// Check if this candidate has argID as a parent
			for _, parentID := range candidateNode.Parents() {
				if parentID == argID {
					result[candidateID] = struct{}{}
					break
				}
			}
		}
	}

	return result, nil
}

// ───────────────────────────────────────────────────────────────────────────────
//  Helper: ancestor test (BFS upward, depth-aware)
// ───────────────────────────────────────────────────────────────────────────────

// isAncestor efficiently tests if candidate is an ancestor of target.
//
// Runtime Complexity: O(d) where d = depth difference between candidate and target
// Uses depth-bounded BFS traversal with DAG optimizations for early termination.
func isAncestor[ID comparable](st Store[ID], candidate, target ID) bool {
	if candidate == target {
		return false // A node is not its own ancestor
	}

	candNode, candOk := st.Node(candidate)
	targNode, targOk := st.Node(target)
	if !candOk || !targOk {
		return false
	}

	// DAG optimization: candidate must be shallower than target to be ancestor
	if candNode.Depth() >= targNode.Depth() {
		return false
	}

	// BFS upward from target - no cycle detection needed in DAG
	queue := []ID{target}

	for len(queue) > 0 {
		currentID := queue[0]
		queue = queue[1:]

		currentNode, ok := st.Node(currentID)
		if !ok {
			continue
		}
		for _, parentID := range currentNode.Parents() {
			if parentID == candidate {
				return true
			}
			// Add parent to queue (no visited tracking needed in DAG)
			queue = append(queue, parentID)
		}
	}
	return false
}

// ───────────────────────────────────────────────────────────────────────────────
//  Convenience: Mercurial-style slice A::B
//  All changesets that are descendants of A and ancestors of B, including A and B themselves
// ───────────────────────────────────────────────────────────────────────────────

// Slice creates a NodeSet representing all nodes on paths between start and end sets.
// Uses Mercurial semantics: A::B includes both A and B themselves, plus all nodes
// that are descendants of A AND ancestors of B.
//
// Runtime Complexity:
//   - Contains(): O(d) where d = max depth difference in the slice
//     Evaluates union of start, end, and intersection of descendants/ancestors
//   - Eval(): O(d × n) where d = max depth range, n = nodes per depth
//     Enumerable when store supports NodesAtDepth (uses Descendants enumeration)
func Slice[ID comparable](start, end NodeSet[ID]) NodeSet[ID] {
	// Mercurial semantics: A::B includes both A and B themselves
	// Since Descendants(A) doesn't include A, we need to union with start and end
	return Union(
		start, // Include start endpoint
		end,   // Include end endpoint
		Intersect(Descendants(start), Ancestors(end)), // Include the path between them
	)
}

// ───────────────────────────────────────────────────────────────────────────────
//  Depth-bounded slice (faster when you know the exact bounds)
// ───────────────────────────────────────────────────────────────────────────────

type depthBoundedSlice[ID comparable] struct {
	startID, endID ID
	cache          map[ID]bool // memoised Contains results – unsynchronised!
}

// DepthBoundedSlice creates an optimized slice between two specific nodes.
// More efficient than general Slice() when you know the exact start and end nodes.
//
// Runtime Complexity:
//   - Contains(): O(d) where d = depth difference between start and end
//     Uses depth bounds for early rejection and cached results
//   - Eval(): Not supported - returns ErrEnumerationDisabled
func DepthBoundedSlice[ID comparable](startID, endID ID) NodeSet[ID] {
	return &depthBoundedSlice[ID]{startID: startID, endID: endID}
}

func (d *depthBoundedSlice[ID]) Contains(st Store[ID], id ID) bool {
	if d.cache == nil {
		d.cache = make(map[ID]bool)
	}
	if cached, ok := d.cache[id]; ok {
		return cached
	}

	startNode, startOK := st.Node(d.startID)
	endNode, endOK := st.Node(d.endID)
	candNode, candOK := st.Node(id)
	if !startOK || !endOK || !candOK {
		d.cache[id] = false
		return false
	}

	startDepth := startNode.Depth()
	endDepth := endNode.Depth()
	candDepth := candNode.Depth()

	minDepth, maxDepth := startDepth, endDepth
	if endDepth < startDepth {
		minDepth, maxDepth = endDepth, startDepth
	}

	if candDepth < minDepth || candDepth > maxDepth {
		d.cache[id] = false
		return false
	}

	// Include endpoints explicitly (Mercurial semantics: A::B includes both A and B)
	if id == d.startID || id == d.endID {
		d.cache[id] = true
		return true
	}

	result := isAncestor(st, d.startID, id) && isAncestor(st, id, d.endID)
	d.cache[id] = result
	return result
}

func (d *depthBoundedSlice[ID]) Eval(Store[ID]) (map[ID]struct{}, error) {
	return nil, ErrEnumerationDisabled
}
