package dag

import (
	"fmt"
	"testing"
)

type testMemStore[ID comparable] struct{ m map[ID]DAGNode[ID] }

func NewTestMemStore[ID comparable]() *testMemStore[ID] {
	return &testMemStore[ID]{m: make(map[ID]DAGNode[ID])}
}

func (ms *testMemStore[ID]) Add(n DAGNode[ID])              { ms.m[n.ID()] = n }
func (ms *testMemStore[ID]) Node(id ID) (DAGNode[ID], bool) { n, ok := ms.m[id]; return n, ok }
func (ms *testMemStore[ID]) NodesAtDepth(depth uint64) (map[ID]struct{}, error) {
	result := make(map[ID]struct{})
	for id, node := range ms.m {
		if node.Depth() == depth {
			result[id] = struct{}{}
		}
	}
	return result, nil
}

// Test node implementation for testing
type testNode struct {
	id      string
	parents []string
	depth   uint64
}

func (n *testNode) ID() string        { return n.id }
func (n *testNode) Parents() []string { return n.parents }
func (n *testNode) Depth() uint64     { return n.depth }

// Test node implementation for int IDs
type intNode struct {
	id      int
	parents []int
	depth   uint64
}

func (n *intNode) ID() int        { return n.id }
func (n *intNode) Parents() []int { return n.parents }
func (n *intNode) Depth() uint64  { return n.depth }

// Test store that can simulate missing nodes
type testStore struct {
	nodes   map[string]*testNode
	missing map[string]bool // nodes that should return false for lookup
}

func newTestStore() *testStore {
	return &testStore{
		nodes:   make(map[string]*testNode),
		missing: make(map[string]bool),
	}
}

func (ts *testStore) add(id string, parents ...string) {
	ts.nodes[id] = &testNode{id: id, parents: parents, depth: 0}
	// Update depths after all nodes are added
	ts.updateDepths()
}

func (ts *testStore) updateDepths() {
	// Reset all depths
	for _, node := range ts.nodes {
		node.depth = 0
	}

	// Calculate depths iteratively - assumes DAG so will converge quickly
	// Add safety limit for test cases that violate DAG assumption
	changed := true
	iterations := 0
	maxIterations := len(ts.nodes) + 1 // Should converge in at most N iterations for DAG

	for changed && iterations < maxIterations {
		changed = false
		iterations++
		for _, node := range ts.nodes {
			maxParentDepth := uint64(0)
			hasParents := false
			for _, parentID := range node.parents {
				if parentNode, ok := ts.nodes[parentID]; ok {
					hasParents = true
					if parentNode.depth+1 > maxParentDepth {
						maxParentDepth = parentNode.depth + 1
					}
				}
			}
			if hasParents && maxParentDepth > node.depth {
				node.depth = maxParentDepth
				changed = true
			}
		}
	}

	// If we hit the limit, it means the dag violates DAG assumption
	// Just leave depths as they are - behavior is undefined for non-DAG input
}

func (ts *testStore) addWithDepth(id string, depth uint64, parents ...string) {
	ts.nodes[id] = &testNode{id: id, parents: parents, depth: depth}
}

func (ts *testStore) markMissing(id string) {
	ts.missing[id] = true
}

func (ts *testStore) Node(id string) (DAGNode[string], bool) {
	if ts.missing[id] {
		return nil, false
	}
	node, ok := ts.nodes[id]
	return node, ok
}

func (ts *testStore) NodesAtDepth(depth uint64) (map[string]struct{}, error) {
	result := make(map[string]struct{})
	for id, node := range ts.nodes {
		if !ts.missing[id] && node.depth == depth {
			result[id] = struct{}{}
		}
	}
	return result, nil
}

func TestID(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		testID   string
		expected bool
	}{
		{"exact match", "A", "A", true},
		{"no match", "A", "B", false},
		{"empty string match", "", "", true},
		{"empty vs non-empty", "", "A", false},
		{"unicode", "🚀", "🚀", true},
		{"unicode mismatch", "🚀", "🛸", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set := ID(tt.id)
			store := newTestStore()

			result := set.Contains(store, tt.testID)
			if result != tt.expected {
				t.Errorf("Contains(%q) = %v, want %v", tt.testID, result, tt.expected)
			}

			// Test evaluation
			eval, err := set.Eval(store)
			if err != nil {
				t.Errorf("Eval() error = %v", err)
			}
			_, exists := eval[tt.id]
			if !exists {
				t.Errorf("Eval() should contain ID %q", tt.id)
			}
			if len(eval) != 1 {
				t.Errorf("Eval() should contain exactly one element, got %d", len(eval))
			}
		})
	}
}

func TestUnion(t *testing.T) {
	tests := []struct {
		name      string
		sets      []NodeSet[string]
		testID    string
		expected  bool
		evalCount int
	}{
		{
			name:      "empty union",
			sets:      []NodeSet[string]{},
			testID:    "A",
			expected:  false,
			evalCount: 0,
		},
		{
			name:      "single set match",
			sets:      []NodeSet[string]{ID("A")},
			testID:    "A",
			expected:  true,
			evalCount: 1,
		},
		{
			name:      "single set no match",
			sets:      []NodeSet[string]{ID("A")},
			testID:    "B",
			expected:  false,
			evalCount: 1,
		},
		{
			name:      "multiple sets first match",
			sets:      []NodeSet[string]{ID("A"), ID("B")},
			testID:    "A",
			expected:  true,
			evalCount: 2,
		},
		{
			name:      "multiple sets second match",
			sets:      []NodeSet[string]{ID("A"), ID("B")},
			testID:    "B",
			expected:  true,
			evalCount: 2,
		},
		{
			name:      "multiple sets no match",
			sets:      []NodeSet[string]{ID("A"), ID("B")},
			testID:    "C",
			expected:  false,
			evalCount: 2,
		},
		{
			name:      "overlapping sets",
			sets:      []NodeSet[string]{ID("A"), ID("A")},
			testID:    "A",
			expected:  true,
			evalCount: 1, // should deduplicate in eval
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			union := Union(tt.sets...)
			store := newTestStore()

			result := union.Contains(store, tt.testID)
			if result != tt.expected {
				t.Errorf("Contains(%q) = %v, want %v", tt.testID, result, tt.expected)
			}

			// Test evaluation
			eval, err := union.Eval(store)
			if err != nil {
				t.Errorf("Eval() error = %v", err)
			}
			if len(eval) != tt.evalCount {
				t.Errorf("Eval() count = %d, want %d", len(eval), tt.evalCount)
			}
		})
	}
}

func TestUnionWithEnumerationDisabled(t *testing.T) {
	store := newTestStore()
	store.add("A")
	store.add("B")

	// Create a union with one normal set and one that can't enumerate
	ancestorSet := Ancestors(ID("A"))
	normalSet := ID("B")
	union := Union(normalSet, ancestorSet)

	// Should still work for contains
	if !union.Contains(store, "B") {
		t.Error("Union should contain B even with non-enumerable set")
	}

	// Eval should skip the non-enumerable set and return partial results
	eval, err := union.Eval(store)
	if err != nil {
		t.Errorf("Union.Eval() should not error with non-enumerable sets, got %v", err)
	}
	if _, ok := eval["B"]; !ok {
		t.Error("Union.Eval() should contain B from enumerable set")
	}
}

func TestIntersect(t *testing.T) {
	tests := []struct {
		name     string
		sets     []NodeSet[string]
		testID   string
		expected bool
	}{
		{
			name:     "empty intersection",
			sets:     []NodeSet[string]{},
			testID:   "A",
			expected: true, // vacuous truth
		},
		{
			name:     "single set match",
			sets:     []NodeSet[string]{ID("A")},
			testID:   "A",
			expected: true,
		},
		{
			name:     "single set no match",
			sets:     []NodeSet[string]{ID("A")},
			testID:   "B",
			expected: false,
		},
		{
			name:     "two sets both match",
			sets:     []NodeSet[string]{ID("A"), ID("A")},
			testID:   "A",
			expected: true,
		},
		{
			name:     "two sets one matches",
			sets:     []NodeSet[string]{ID("A"), ID("B")},
			testID:   "A",
			expected: false,
		},
		{
			name:     "two sets neither matches",
			sets:     []NodeSet[string]{ID("A"), ID("B")},
			testID:   "C",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intersect := Intersect(tt.sets...)
			store := newTestStore()

			result := intersect.Contains(store, tt.testID)
			if result != tt.expected {
				t.Errorf("Contains(%q) = %v, want %v", tt.testID, result, tt.expected)
			}
		})
	}
}

func TestIntersectEval(t *testing.T) {
	store := newTestStore()

	// Test with enumerable sets
	set1 := Union(ID("A"), ID("B"), ID("C"))
	set2 := Union(ID("B"), ID("C"), ID("D"))
	intersect := Intersect(set1, set2)

	eval, err := intersect.Eval(store)
	if err != nil {
		t.Errorf("Intersect.Eval() error = %v", err)
	}

	expected := map[string]struct{}{"B": {}, "C": {}}
	if len(eval) != len(expected) {
		t.Errorf("Intersect.Eval() count = %d, want %d", len(eval), len(expected))
	}
	for id := range expected {
		if _, ok := eval[id]; !ok {
			t.Errorf("Intersect.Eval() missing %q", id)
		}
	}
}

func TestIntersectEvalWithNonEnumerable(t *testing.T) {
	store := newTestStore()
	store.add("A")

	// All sets are non-enumerable
	ancestorSet1 := Ancestors(ID("A"))
	ancestorSet2 := Ancestors(ID("A"))
	intersect := Intersect(ancestorSet1, ancestorSet2)

	_, err := intersect.Eval(store)
	if err != ErrEnumerationDisabled {
		t.Errorf("Intersect.Eval() with all non-enumerable should return ErrEnumerationDisabled, got %v", err)
	}
}

func TestDiff(t *testing.T) {
	tests := []struct {
		name     string
		setA     NodeSet[string]
		setB     NodeSet[string]
		testID   string
		expected bool
	}{
		{
			name:     "A contains, B doesn't",
			setA:     ID("A"),
			setB:     ID("B"),
			testID:   "A",
			expected: true,
		},
		{
			name:     "A contains, B contains",
			setA:     ID("A"),
			setB:     ID("A"),
			testID:   "A",
			expected: false,
		},
		{
			name:     "A doesn't contain",
			setA:     ID("A"),
			setB:     ID("B"),
			testID:   "C",
			expected: false,
		},
		{
			name:     "complex diff",
			setA:     Union(ID("A"), ID("B"), ID("C")),
			setB:     Union(ID("B"), ID("D")),
			testID:   "A",
			expected: true,
		},
		{
			name:     "complex diff excluded",
			setA:     Union(ID("A"), ID("B"), ID("C")),
			setB:     Union(ID("B"), ID("D")),
			testID:   "B",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diff := Diff(tt.setA, tt.setB)
			store := newTestStore()

			result := diff.Contains(store, tt.testID)
			if result != tt.expected {
				t.Errorf("Contains(%q) = %v, want %v", tt.testID, result, tt.expected)
			}
		})
	}
}

func TestDiffEval(t *testing.T) {
	store := newTestStore()

	setA := Union(ID("A"), ID("B"), ID("C"))
	setB := Union(ID("B"), ID("D"))
	diff := Diff(setA, setB)

	eval, err := diff.Eval(store)
	if err != nil {
		t.Errorf("Diff.Eval() error = %v", err)
	}

	expected := map[string]struct{}{"A": {}, "C": {}}
	if len(eval) != len(expected) {
		t.Errorf("Diff.Eval() count = %d, want %d", len(eval), len(expected))
	}
	for id := range expected {
		if _, ok := eval[id]; !ok {
			t.Errorf("Diff.Eval() missing %q", id)
		}
	}
}

func TestNot(t *testing.T) {
	tests := []struct {
		name     string
		set      NodeSet[string]
		testID   string
		expected bool
	}{
		{
			name:     "not contains",
			set:      ID("A"),
			testID:   "A",
			expected: false,
		},
		{
			name:     "not doesn't contain",
			set:      ID("A"),
			testID:   "B",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			not := Not(tt.set)
			store := newTestStore()

			result := not.Contains(store, tt.testID)
			if result != tt.expected {
				t.Errorf("Contains(%q) = %v, want %v", tt.testID, result, tt.expected)
			}
		})
	}
}

func TestNotEval(t *testing.T) {
	store := newTestStore()
	not := Not(ID("A"))

	_, err := not.Eval(store)
	if err != ErrEnumerationDisabled {
		t.Errorf("Not.Eval() should return ErrEnumerationDisabled, got %v", err)
	}
}

func TestAncestors(t *testing.T) {
	store := newTestStore()
	// Create a simple DAG: A -> B -> C
	store.add("C", "B")
	store.add("B", "A")
	store.add("A")

	tests := []struct {
		name     string
		arg      NodeSet[string]
		testID   string
		expected bool
	}{
		{
			name:     "direct ancestor",
			arg:      ID("C"),
			testID:   "B",
			expected: true,
		},
		{
			name:     "indirect ancestor",
			arg:      ID("C"),
			testID:   "A",
			expected: true,
		},
		{
			name:     "self is NOT ancestor",
			arg:      ID("C"),
			testID:   "C",
			expected: false, // Per spec: A node is NOT its own ancestor
		},
		{
			name:     "not ancestor",
			arg:      ID("A"),
			testID:   "C",
			expected: false,
		},
		{
			name:     "non-existent target",
			arg:      ID("X"),
			testID:   "A",
			expected: false,
		},
		{
			name:     "non-existent candidate",
			arg:      ID("C"),
			testID:   "X",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ancestors := Ancestors(tt.arg)

			result := ancestors.Contains(store, tt.testID)
			if result != tt.expected {
				t.Errorf("Ancestors(%v).Contains(%q) = %v, want %v", tt.arg, tt.testID, result, tt.expected)
			}
		})
	}
}

func TestAncestorsEval(t *testing.T) {
	store := newTestStore()
	ancestors := Ancestors(ID("A"))

	_, err := ancestors.Eval(store)
	if err != ErrEnumerationDisabled {
		t.Errorf("Ancestors.Eval() should return ErrEnumerationDisabled, got %v", err)
	}
}

func TestDescendants(t *testing.T) {
	store := newTestStore()
	// Create a simple DAG: A -> B -> C
	store.add("C", "B")
	store.add("B", "A")
	store.add("A")

	tests := []struct {
		name     string
		arg      NodeSet[string]
		testID   string
		expected bool
	}{
		{
			name:     "direct descendant",
			arg:      ID("A"),
			testID:   "B",
			expected: true,
		},
		{
			name:     "indirect descendant",
			arg:      ID("A"),
			testID:   "C",
			expected: true,
		},
		{
			name:     "self is NOT descendant",
			arg:      ID("A"),
			testID:   "A",
			expected: false, // Per spec: A node is NOT its own descendant
		},
		{
			name:     "not descendant",
			arg:      ID("C"),
			testID:   "A",
			expected: false,
		},
		{
			name:     "non-existent candidate",
			arg:      ID("A"),
			testID:   "X",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			descendants := Descendants(tt.arg)

			result := descendants.Contains(store, tt.testID)
			if result != tt.expected {
				t.Errorf("Descendants(%v).Contains(%q) = %v, want %v", tt.arg, tt.testID, result, tt.expected)
			}
		})
	}
}

func TestParents(t *testing.T) {
	store := newTestStore()
	// Create a DAG: A, B -> C -> D
	store.add("D", "C")
	store.add("C", "A", "B")
	store.add("A")
	store.add("B")

	tests := []struct {
		name     string
		arg      NodeSet[string]
		testID   string
		expected bool
	}{
		{
			name:     "is parent",
			arg:      ID("C"),
			testID:   "A",
			expected: true,
		},
		{
			name:     "is other parent",
			arg:      ID("C"),
			testID:   "B",
			expected: true,
		},
		{
			name:     "not parent",
			arg:      ID("C"),
			testID:   "D",
			expected: false,
		},
		{
			name:     "self not parent",
			arg:      ID("C"),
			testID:   "C",
			expected: false,
		},
		{
			name:     "multiple targets",
			arg:      Union(ID("C"), ID("D")),
			testID:   "A",
			expected: true,
		},
		{
			name:     "non-existent target",
			arg:      ID("X"),
			testID:   "A",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parents := Parents(tt.arg)

			result := parents.Contains(store, tt.testID)
			if result != tt.expected {
				t.Errorf("Parents(%v).Contains(%q) = %v, want %v", tt.arg, tt.testID, result, tt.expected)
			}
		})
	}
}

func TestChildren(t *testing.T) {
	store := newTestStore()
	// Create a DAG: A, B -> C -> D
	store.add("D", "C")
	store.add("C", "A", "B")
	store.add("A")
	store.add("B")

	tests := []struct {
		name     string
		arg      NodeSet[string]
		testID   string
		expected bool
	}{
		{
			name:     "is child",
			arg:      ID("A"),
			testID:   "C",
			expected: true,
		},
		{
			name:     "is child of other parent",
			arg:      ID("B"),
			testID:   "C",
			expected: true,
		},
		{
			name:     "not child",
			arg:      ID("D"),
			testID:   "A",
			expected: false,
		},
		{
			name:     "self not child",
			arg:      ID("A"),
			testID:   "A",
			expected: false,
		},
		{
			name:     "non-existent candidate",
			arg:      ID("A"),
			testID:   "X",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			children := Children(tt.arg)

			result := children.Contains(store, tt.testID)
			if result != tt.expected {
				t.Errorf("Children(%v).Contains(%q) = %v, want %v", tt.arg, tt.testID, result, tt.expected)
			}
		})
	}
}

func TestPredicateEval(t *testing.T) {
	store := newTestStore()
	store.add("A")
	store.add("B", "A")
	store.add("C", "B")

	tests := []struct {
		name        string
		predicate   NodeSet[string]
		shouldError bool
		description string
	}{
		{
			name:        "Descendants_enumerable",
			predicate:   Descendants(ID("A")),
			shouldError: false,
			description: "Descendants should be enumerable when store supports NodesAtDepth",
		},
		{
			name:        "Parents_not_enumerable",
			predicate:   Parents(ID("B")),
			shouldError: true,
			description: "Parents should still return ErrEnumerationDisabled",
		},
		{
			name:        "Children_enumerable",
			predicate:   Children(ID("A")),
			shouldError: false,
			description: "Children should be enumerable when store supports NodesAtDepth",
		},
		{
			name:        "Ancestors_not_enumerable",
			predicate:   Ancestors(ID("C")),
			shouldError: true,
			description: "Ancestors should still return ErrEnumerationDisabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.predicate.Eval(store)
			if tt.shouldError {
				if err != ErrEnumerationDisabled {
					t.Errorf("%s: expected ErrEnumerationDisabled, got %v", tt.description, err)
				}
			} else {
				if err != nil {
					t.Errorf("%s: expected no error, got %v", tt.description, err)
				}
				if result == nil {
					t.Errorf("%s: expected result, got nil", tt.description)
				}
			}
		})
	}
}

func TestSlice(t *testing.T) {
	store := newTestStore()
	// Create a linear chain: A -> B -> C -> D
	store.add("D", "C")
	store.add("C", "B")
	store.add("B", "A")
	store.add("A")

	tests := []struct {
		name     string
		a        NodeSet[string]
		b        NodeSet[string]
		testID   string
		expected bool
	}{
		{
			name:     "slice middle",
			a:        ID("A"),
			b:        ID("D"),
			testID:   "B",
			expected: true,
		},
		{
			name:     "slice middle 2",
			a:        ID("A"),
			b:        ID("D"),
			testID:   "C",
			expected: true,
		},
		{
			name:     "slice includes endpoints",
			a:        ID("A"),
			b:        ID("D"),
			testID:   "A",
			expected: true,
		},
		{
			name:     "slice includes endpoints 2",
			a:        ID("A"),
			b:        ID("D"),
			testID:   "D",
			expected: true,
		},
		{
			name:     "outside slice",
			a:        ID("B"),
			b:        ID("C"),
			testID:   "A",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slice := Slice(tt.a, tt.b)

			result := slice.Contains(store, tt.testID)
			if result != tt.expected {
				t.Errorf("Slice(%v, %v).Contains(%q) = %v, want %v", tt.a, tt.b, tt.testID, result, tt.expected)
			}
		})
	}

	t.Run("slice enumeration", func(t *testing.T) {
		// Test that Slice can now be enumerated
		slice := Slice(ID("A"), ID("D"))
		sliceNodes, err := slice.Eval(store)
		if err != nil {
			t.Errorf("Slice should be enumerable when store supports NodesAtDepth, got error: %v", err)
			return
		}

		// Should contain all nodes in the slice A::D (A, B, C, D)
		expectedNodes := map[string]bool{"A": true, "B": true, "C": true, "D": true}
		if len(sliceNodes) != len(expectedNodes) {
			t.Errorf("Slice enumeration returned %d nodes, expected %d", len(sliceNodes), len(expectedNodes))
		}

		for nodeID := range sliceNodes {
			if !expectedNodes[nodeID] {
				t.Errorf("Slice enumeration contains unexpected node: %s", nodeID)
			}
		}

		for expectedNode := range expectedNodes {
			if _, found := sliceNodes[expectedNode]; !found {
				t.Errorf("Slice enumeration missing expected node: %s", expectedNode)
			}
		}
	})
}

func TestMemStore(t *testing.T) {
	store := NewTestMemStore[string]()

	// Test empty store
	_, ok := store.Node("A")
	if ok {
		t.Error("Empty store should not contain any nodes")
	}

	// Test adding and retrieving
	node := &testNode{id: "A", parents: []string{"B"}}
	store.Add(node)

	retrieved, ok := store.Node("A")
	if !ok {
		t.Error("Store should contain added node")
	}
	if retrieved.ID() != "A" {
		t.Errorf("Retrieved node ID = %q, want %q", retrieved.ID(), "A")
	}

	// Test overwriting
	newNode := &testNode{id: "A", parents: []string{"C"}}
	store.Add(newNode)

	retrieved, _ = store.Node("A")
	if len(retrieved.Parents()) != 1 || retrieved.Parents()[0] != "C" {
		t.Error("Store should overwrite existing nodes")
	}
}

func TestMemStoreWithDifferentTypes(t *testing.T) {
	// Test with int IDs
	intStore := NewTestMemStore[int]()

	node := &intNode{id: 42, parents: []int{1, 2}}
	intStore.Add(node)

	retrieved, ok := intStore.Node(42)
	if !ok || retrieved.ID() != 42 {
		t.Error("MemStore should work with different ID types")
	}
}

func TestComplexDAG(t *testing.T) {
	store := newTestStore()

	// Create a more complex DAG:
	//     A   B
	//     |\ /|
	//     | X |
	//     |/ \|
	//     C   D
	//      \ /
	//       E
	store.add("E", "C", "D")
	store.add("C", "A", "B")
	store.add("D", "A", "B")
	store.add("A")
	store.add("B")

	tests := []struct {
		name     string
		query    NodeSet[string]
		testID   string
		expected bool
	}{
		{
			name:     "ancestors of E includes all",
			query:    Ancestors(ID("E")),
			testID:   "A",
			expected: true,
		},
		{
			name:     "descendants of A includes E",
			query:    Descendants(ID("A")),
			testID:   "E",
			expected: true,
		},
		{
			name:     "slice A to E includes C",
			query:    Slice(ID("A"), ID("E")),
			testID:   "C",
			expected: true,
		},
		{
			name:     "parents of C and D",
			query:    Parents(Union(ID("C"), ID("D"))),
			testID:   "A",
			expected: true,
		},
		{
			name:     "children of A and B",
			query:    Children(Union(ID("A"), ID("B"))),
			testID:   "C",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.query.Contains(store, tt.testID)
			if result != tt.expected {
				t.Errorf("%s.Contains(%q) = %v, want %v", tt.name, tt.testID, result, tt.expected)
			}
		})
	}
}

func TestEdgeCases(t *testing.T) {
	t.Run("missing nodes in store", func(t *testing.T) {
		store := newTestStore()
		store.add("A", "B") // B doesn't exist

		descendants := Descendants(ID("A"))
		// Should handle missing parent gracefully
		result := descendants.Contains(store, "C")
		if result {
			t.Error("Descendants should handle missing nodes gracefully")
		}
	})

	t.Run("self-referencing node (DAG violation)", func(t *testing.T) {
		store := newTestStore()
		store.add("A", "A") // A is its own parent - violates DAG assumption

		ancestors := Ancestors(ID("A"))
		// With DAG assumption, this may not work correctly
		// but we test it doesn't crash
		_ = ancestors.Contains(store, "A")
		// Just ensure it doesn't crash - behavior is undefined for non-DAG input
	})

	t.Run("empty parents", func(t *testing.T) {
		store := newTestStore()
		store.add("A") // No parents

		parents := Parents(ID("A"))
		result := parents.Contains(store, "B")
		if result {
			t.Error("Node with no parents should not have any parents")
		}
	})

	t.Run("cache behavior", func(t *testing.T) {
		store := newTestStore()
		store.add("A", "B")
		store.add("B")

		ancestors := Ancestors(ID("A"))

		// First call
		result1 := ancestors.Contains(store, "B")
		// Second call should use cache
		result2 := ancestors.Contains(store, "B")

		if result1 != result2 {
			t.Error("Cache should provide consistent results")
		}
		if !result1 {
			t.Error("B should be ancestor of A")
		}
	})
}

func TestErrorPropagation(t *testing.T) {
	store := newTestStore()

	// Test that errors from nested sets are properly propagated
	nonExistentSet := ID("nonexistent")
	union := Union(nonExistentSet)

	// This should not error for Contains (lazy evaluation)
	result := union.Contains(store, "test")
	if result {
		t.Error("Union with non-existent set should return false for Contains")
	}

	// Eval should work fine too (just returns empty for non-existent)
	eval, err := union.Eval(store)
	if err != nil {
		t.Errorf("Union.Eval() should not error, got %v", err)
	}
	if len(eval) != 1 {
		t.Errorf("Union.Eval() should contain the ID set, got %d items", len(eval))
	}
}

// ────────────────────────────────────────────────────────────────────────────────
// Depth optimization tests
// ────────────────────────────────────────────────────────────────────────────────

func TestDepthOptimizations(t *testing.T) {
	store := newTestStore()

	// Create a chain: A(0) -> B(1) -> C(2) -> D(3)
	store.addWithDepth("A", 0)
	store.addWithDepth("B", 1, "A")
	store.addWithDepth("C", 2, "B")
	store.addWithDepth("D", 3, "C")

	t.Run("isAncestor depth optimization", func(t *testing.T) {
		// A should be ancestor of D
		if !isAncestor(store, "A", "D") {
			t.Error("A should be ancestor of D")
		}

		// D should NOT be ancestor of A (depth check should reject immediately)
		if isAncestor(store, "D", "A") {
			t.Error("D should not be ancestor of A")
		}

		// Same depth nodes should not be ancestors of each other
		store.addWithDepth("E", 2, "B") // E at same depth as C
		if isAncestor(store, "C", "E") {
			t.Error("C should not be ancestor of E (same depth)")
		}
	})

	t.Run("ancestors depth optimization", func(t *testing.T) {
		ancestorsOfD := Ancestors(ID("D"))

		// A, B, C should be ancestors of D
		if !ancestorsOfD.Contains(store, "A") {
			t.Error("A should be ancestor of D")
		}
		if !ancestorsOfD.Contains(store, "B") {
			t.Error("B should be ancestor of D")
		}
		if !ancestorsOfD.Contains(store, "C") {
			t.Error("C should be ancestor of D")
		}

		// Per spec: D should NOT be ancestor of itself
		if ancestorsOfD.Contains(store, "D") {
			t.Error("D should NOT be ancestor of itself (per documented spec)")
		}
	})

	t.Run("descendants depth optimization", func(t *testing.T) {
		descendantsOfA := Descendants(ID("A"))

		// B, C, D should be descendants of A
		if !descendantsOfA.Contains(store, "B") {
			t.Error("B should be descendant of A")
		}
		if !descendantsOfA.Contains(store, "C") {
			t.Error("C should be descendant of A")
		}
		if !descendantsOfA.Contains(store, "D") {
			t.Error("D should be descendant of A")
		}

		// Per spec: A node is NOT its own descendant
		if descendantsOfA.Contains(store, "A") {
			t.Error("A should NOT be descendant of itself (per documented spec)")
		}
	})
}

func TestDepthBoundedSlice(t *testing.T) {
	store := newTestStore()

	// Create a more complex DAG with branches
	store.addWithDepth("A", 0)
	store.addWithDepth("B", 1, "A")
	store.addWithDepth("C", 1, "A")
	store.addWithDepth("D", 2, "B")
	store.addWithDepth("E", 2, "C")
	store.addWithDepth("F", 3, "D", "E") // F has both D and E as parents

	t.Run("depth bounded slice basic", func(t *testing.T) {
		// Slice from A to F should include intermediate nodes
		slice := DepthBoundedSlice("A", "F")

		// A should be in slice (start node)
		if !slice.Contains(store, "A") {
			t.Error("A should be in slice A::F")
		}

		// F should be in slice (end node)
		if !slice.Contains(store, "F") {
			t.Error("F should be in slice A::F")
		}

		// B, C, D, E should be in slice (intermediate nodes)
		if !slice.Contains(store, "B") {
			t.Error("B should be in slice A::F")
		}
		if !slice.Contains(store, "C") {
			t.Error("C should be in slice A::F")
		}
		if !slice.Contains(store, "D") {
			t.Error("D should be in slice A::F")
		}
		if !slice.Contains(store, "E") {
			t.Error("E should be in slice A::F")
		}
	})

	t.Run("depth bounds rejection", func(t *testing.T) {
		// Add a node outside the depth range
		store.addWithDepth("G", 5, "F")

		slice := DepthBoundedSlice("A", "F")

		// G should not be in slice (too deep)
		if slice.Contains(store, "G") {
			t.Error("G should not be in slice A::F (depth too high)")
		}
	})

	t.Run("caching behavior", func(t *testing.T) {
		slice := DepthBoundedSlice("A", "F")

		// First call
		result1 := slice.Contains(store, "B")
		// Second call should use cache
		result2 := slice.Contains(store, "B")

		if result1 != result2 {
			t.Error("Cached results should be consistent")
		}
		if !result1 {
			t.Error("B should be in slice A::F")
		}
	})
}

func TestDepthPerformanceComparison(t *testing.T) {
	store := newTestStore()

	// Create a deep chain for performance testing
	const chainLength = 100

	store.addWithDepth("root", 0)
	prev := "root"
	for i := 1; i < chainLength; i++ {
		current := fmt.Sprintf("node%d", i)
		store.addWithDepth(current, uint64(i), prev)
		prev = current
	}

	lastNode := fmt.Sprintf("node%d", chainLength-1)

	t.Run("ancestor check performance", func(t *testing.T) {
		// This should be fast due to depth optimization
		// In the old implementation, this would traverse the entire chain
		// With depth optimization, it's bounded by the depth difference

		if !isAncestor(store, "root", lastNode) {
			t.Error("root should be ancestor of last node")
		}

		// Reverse check should be immediately rejected due to depth
		if isAncestor(store, lastNode, "root") {
			t.Error("last node should not be ancestor of root")
		}
	})

	t.Run("depth bounded slice performance", func(t *testing.T) {
		// Test slice from beginning to end
		slice := DepthBoundedSlice("root", lastNode)

		// Check a few nodes in the middle
		midNode := "node50"
		if !slice.Contains(store, midNode) {
			t.Error("Middle node should be in slice")
		}

		// Check depth bounds work
		store.addWithDepth("outsider", 200, lastNode)
		if slice.Contains(store, "outsider") {
			t.Error("Node outside depth bounds should not be in slice")
		}
	})
}
