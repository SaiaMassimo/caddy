package memento

import (
	"sort"
	"sync"
)

// WeightedConsistentEngine manages the weighted assignment of buckets to nodes
// using an underlying Memento engine. It coordinates between the hashing logic
// of MementoEngine and the mapping logic of WeightedIndirection.
type WeightedConsistentEngine struct {
	memento     *MementoEngine       // The underlying consistent hashing engine
	indirection *WeightedIndirection // Manages the mapping of buckets to nodes

	// Mutex to protect concurrent access
	mu sync.RWMutex
}

// NewWeightedConsistentEngine creates a new weighted consistent hashing engine.
func NewWeightedConsistentEngine() *WeightedConsistentEngine {
	return &WeightedConsistentEngine{
		memento:     NewMementoEngine(0),
		indirection: NewWeightedIndirection(),
	}
}

// --- Utility Implementations ---

// No longer needed here, moved to WeightedIndirection

// --- Main Operation Implementations ---

// InitCluster initializes the cluster with a set of nodes and their weights.
func (w *WeightedConsistentEngine) InitCluster(nodesWithWeights map[string]int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	totalBuckets := 0
	nodeIDs := make([]string, 0, len(nodesWithWeights))

	for nodeID, weight := range nodesWithWeights {
		totalBuckets += weight
		nodeIDs = append(nodeIDs, nodeID)
	}

	// Re-initialize the engine with the correct total size
	w.memento = NewMementoEngine(totalBuckets)

	// Sort nodes for deterministic bucket assignment
	sort.Strings(nodeIDs)

	// Initialize data structures for each node in the indirection layer
	for _, nodeID := range nodeIDs {
		weight := nodesWithWeights[nodeID]
		w.indirection.InitNode(nodeID, weight)
	}

	// Interleaved (weighted round-robin) bucket assignment for better distribution
	b := 0
	tempWeights := make(map[string]int)
	for node, weight := range nodesWithWeights {
		tempWeights[node] = weight
	}

	for b < totalBuckets {
		for _, nodeID := range nodeIDs {
			if tempWeights[nodeID] > 0 {
				w.indirection.AttachBucket(b, nodeID)
				tempWeights[nodeID]--
				b++
			}
		}
	}
}

// Lookup finds the node that owns a key.
func (w *WeightedConsistentEngine) Lookup(key string) (string, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if w.memento.Size() == 0 {
		return "", false
	}

	bucketID := w.memento.GetBucket(key)
	nodeID, ok := w.indirection.GetNodeID(bucketID)
	return nodeID, ok
}

// AddNode adds a new node with a given weight.
func (w *WeightedConsistentEngine) AddNode(nodeID string, weight int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.indirection.HasNode(nodeID) {
		return // Node already exists
	}

	w.indirection.InitNode(nodeID, weight)

	for i := 0; i < weight; i++ {
		bucketID := w.memento.AddBucket()
		w.indirection.AttachBucket(bucketID, nodeID)
	}
}

// RemoveNode removes a node from the cluster.
func (w *WeightedConsistentEngine) RemoveNode(nodeID string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.indirection.HasNode(nodeID) {
		return // Node does not exist
	}

	// Get the buckets to remove from the indirection layer first
	bucketsToRemove := w.indirection.GetBucketsForNode(nodeID)

	// Remove each bucket from the memento engine
	for _, bucketID := range bucketsToRemove {
		w.memento.RemoveBucket(bucketID)
	}

	// Clean up all metadata for the removed node in the indirection layer
	w.indirection.RemoveNode(nodeID)
}

// UpdateWeight updates the weight of an existing node.
func (w *WeightedConsistentEngine) UpdateWeight(nodeID string, newWeight int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if newWeight < 0 {
		newWeight = 0
	}

	oldWeight, exists := w.indirection.GetWeight(nodeID)

	if !exists {
		// If the node doesn't exist, treat it as an AddNode operation
		w.indirection.InitNode(nodeID, newWeight)
		for i := 0; i < newWeight; i++ {
			bucketID := w.memento.AddBucket()
			w.indirection.AttachBucket(bucketID, nodeID)
		}
		return
	}

	w.indirection.UpdateWeight(nodeID, newWeight)
	delta := newWeight - oldWeight

	if delta > 0 { // Weight increased
		for i := 0; i < delta; i++ {
			bucketID := w.memento.AddBucket()
			w.indirection.AttachBucket(bucketID, nodeID)
		}
	} else if delta < 0 { // Weight decreased
		numToRemove := -delta
		bucketsOwned := w.indirection.GetBucketsForNode(nodeID)

		if numToRemove > len(bucketsOwned) {
			numToRemove = len(bucketsOwned)
		}

		// Remove the last 'numToRemove' buckets
		bucketsToRemove := append([]int{}, bucketsOwned[len(bucketsOwned)-numToRemove:]...)

		for _, bucketID := range bucketsToRemove {
			w.indirection.DetachBucket(bucketID)
			w.memento.RemoveBucket(bucketID)
		}
	}
}
