package reverseproxy

import (
	"sort"
	"sync"

	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy/memento"
)

// WeightedConsistentEngine manages the weighted assignment of buckets to nodes
// using an underlying Memento engine. It coordinates between the hashing logic
// of MementoEngine and the mapping logic of WeightedIndirection.
type WeightedConsistentEngine struct {
	memento     *memento.MementoEngine // The underlying consistent hashing engine
	indirection *WeightedIndirection   // Manages the mapping of buckets to nodes

	// Mutex to protect concurrent access
	mu sync.RWMutex
}

// NewWeightedConsistentEngine creates a new weighted consistent hashing engine.
func NewWeightedConsistentEngine() *WeightedConsistentEngine {
	return &WeightedConsistentEngine{
		memento:     memento.NewMementoEngine(0),
		indirection: NewWeightedIndirection(),
	}
}

// --- Utility Implementations ---

// No longer needed here, moved to WeightedIndirection

// --- Main Operation Implementations ---

// InitCluster initializes the cluster with a set of upstreams and their weights.
func (w *WeightedConsistentEngine) InitCluster(nodesWithWeights map[*Upstream]int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	totalBuckets := 0
	upstreams := make([]*Upstream, 0, len(nodesWithWeights))

	for upstream, weight := range nodesWithWeights {
		totalBuckets += weight
		upstreams = append(upstreams, upstream)
	}

	// Re-initialize the engine with the correct total size
	w.memento = memento.NewMementoEngine(totalBuckets)

	// Sort upstreams for deterministic bucket assignment
	sort.Slice(upstreams, func(i, j int) bool {
		return upstreams[i].String() < upstreams[j].String()
	})

	// Initialize data structures for each node in the indirection layer
	for _, upstream := range upstreams {
		weight := nodesWithWeights[upstream]
		w.indirection.InitNode(upstream, weight)
	}

	// Interleaved (weighted round-robin) bucket assignment for better distribution
	b := 0
	tempWeights := make(map[*Upstream]int)
	for upstream, weight := range nodesWithWeights {
		tempWeights[upstream] = weight
	}

	for b < totalBuckets {
		for _, upstream := range upstreams {
			if tempWeights[upstream] > 0 {
				w.indirection.AttachBucket(b, upstream)
				tempWeights[upstream]--
				b++
			}
		}
	}
}

// Lookup finds the node that owns a key.
func (w *WeightedConsistentEngine) Lookup(key string) (*Upstream, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if w.memento.Size() == 0 {
		return nil, false
	}

	bucketID := w.memento.GetBucket(key)
	upstream, ok := w.indirection.GetNodeID(bucketID)
	return upstream, ok
}

// AddNode adds a new node with a given weight.
func (w *WeightedConsistentEngine) AddNode(upstream *Upstream, weight int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.indirection.HasNode(upstream) {
		return // Node already exists
	}

	w.indirection.InitNode(upstream, weight)

	for i := 0; i < weight; i++ {
		bucketID := w.memento.AddBucket()
		w.indirection.AttachBucket(bucketID, upstream)
	}
}

// RemoveNode removes a node from the cluster.
func (w *WeightedConsistentEngine) RemoveNode(upstream *Upstream) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.indirection.HasNode(upstream) {
		return // Node does not exist
	}

	// Get the buckets to remove from the indirection layer first
	bucketsToRemove := w.indirection.GetBucketsForNode(upstream)

	// Remove each bucket from the memento engine
	for _, bucketID := range bucketsToRemove {
		w.memento.RemoveBucket(bucketID)
	}

	// Clean up all metadata for the removed node in the indirection layer
	w.indirection.RemoveNode(upstream)
}

// UpdateWeight updates the weight of an existing node.
func (w *WeightedConsistentEngine) UpdateWeight(upstream *Upstream, newWeight int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if newWeight < 0 {
		newWeight = 0
	}

	oldWeight, exists := w.indirection.GetWeight(upstream)

	if !exists {
		// If the node doesn't exist, treat it as an AddNode operation
		w.indirection.InitNode(upstream, newWeight)
		for i := 0; i < newWeight; i++ {
			bucketID := w.memento.AddBucket()
			w.indirection.AttachBucket(bucketID, upstream)
		}
		return
	}

	w.indirection.UpdateWeight(upstream, newWeight)
	delta := newWeight - oldWeight

	if delta > 0 { // Weight increased
		for i := 0; i < delta; i++ {
			bucketID := w.memento.AddBucket()
			w.indirection.AttachBucket(bucketID, upstream)
		}
	} else if delta < 0 { // Weight decreased
		numToRemove := -delta
		bucketsOwned := w.indirection.GetBucketsForNode(upstream)

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
